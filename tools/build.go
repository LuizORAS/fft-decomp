// build.go compiles configured functions and verifies them: against the
// configured hashes without the disc (`check`), and written over copies of
// the original module images (`patch-check`, the cross-check of the linked
// `make build`); it records those hashes from the original bytes
// (`checksums`) and writes the disc from built modules (writeDisc).
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// compileSlots, when non-nil, bounds concurrent compiles across modules
// that check and patch-check compile in parallel.
var compileSlots chan struct{}

// toolJobs is the compile parallelism: TOOLS_JOBS, or one per CPU.
func toolJobs() (int, error) {
	jobs := runtime.NumCPU()
	if value := os.Getenv("TOOLS_JOBS"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 {
			return 0, fmt.Errorf("invalid TOOLS_JOBS %q", value)
		}
		jobs = parsed
	}
	return jobs, nil
}

// shortHash is the configured checksum: the first 16 hex digits of SHA-256.
func shortHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:hashDigits]
}

var effectAlias = regexp.MustCompile(`^effect([0-9]{3})$`)

// resolveModuleName accepts a module id, an event overlay's short name
// (attack) or an effect number (effect259).
func resolveModuleName(config *projectConfig, name string) (*moduleSpec, error) {
	candidates := []string{name, "event-" + name}
	if match := effectAlias.FindStringSubmatch(name); match != nil {
		candidates = append(candidates, "effect-e"+match[1])
	}
	for _, id := range candidates {
		if m, ok := config.module(id); ok {
			return m, nil
		}
	}
	return nil, fmt.Errorf("unknown module %q; use a module id such as battle, event-attack or effect-e259, or an event short name such as attack", name)
}

// resolveModuleSelection resolves a space- or comma-separated list of module
// names; `event` and `effect` select every event-* or effect-* module. The
// result keeps configuration order without duplicates.
func resolveModuleSelection(config *projectConfig, selection string) ([]*moduleSpec, error) {
	selected := make(map[*moduleSpec]bool)
	for _, name := range strings.FieldsFunc(selection, func(r rune) bool { return r == ',' || r == ' ' }) {
		if name == "event" || name == "effect" {
			for _, m := range config.Modules {
				if strings.HasPrefix(m.ID, name+"-") {
					selected[m] = true
				}
			}
			continue
		}
		m, err := resolveModuleName(config, name)
		if err != nil {
			return nil, err
		}
		selected[m] = true
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no modules selected by %q", selection)
	}
	var modules []*moduleSpec
	for _, m := range config.Modules {
		if selected[m] {
			modules = append(modules, m)
		}
	}
	return modules, nil
}

// compileProgress prints a line each time another tenth of a command's
// functions has compiled, so a cold-cache build or check is not silent.
type compileProgress struct {
	mu                sync.Mutex
	label             string
	total, done, next int
}

func newCompileProgress(label string, modules []*moduleSpec) *compileProgress {
	progress := &compileProgress{label: label, next: 1}
	for _, m := range modules {
		progress.total += len(m.Functions)
	}
	return progress
}

func (c *compileProgress) step() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.done++
	if tenths := c.done * 10 / c.total; tenths >= c.next {
		fmt.Printf("%s: %d%% (%d/%d)\n", c.label, tenths*10, c.done, c.total)
		c.next = tenths + 1
	}
}

type compiledFunction struct {
	sections map[string][]byte
	err      error
}

// compileModule compiles the selected functions of one module (all when
// only is nil) through the historical toolchain, each linked at its target
// address, and returns their allocatable sections.
func (p project) compileModule(config *projectConfig, resolver symbolResolver, m *moduleSpec, only map[string]bool) ([]compiledFunction, error) {
	results := make([]compiledFunction, len(m.Functions))
	jobs, err := toolJobs()
	if err != nil {
		return nil, err
	}
	// Per-function intermediates go to the container-local work directory:
	// thousands of small files are much faster there than on the host bind
	// mount. They are namespaced by module because many effect overlays share
	// a helper name and modules compile concurrently.
	workDir := filepath.Join(p.workDir("modules"), m.ID)
	work := make(chan int)
	var workers sync.WaitGroup
	for range min(jobs, len(m.Functions)) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range work {
				// A check of every module runs them all at once; compileSlots caps
				// the total number of concurrent compiles across all of them.
				if compileSlots != nil {
					compileSlots <- struct{}{}
				}
				results[index].sections, results[index].err = p.compileFunction(resolver, m, &m.Functions[index], workDir)
				if compileSlots != nil {
					<-compileSlots
				}
				p.progress.step()
			}
		}()
	}
	for index := range m.Functions {
		if only == nil || only[m.Functions[index].Name] {
			work <- index
		}
	}
	close(work)
	workers.Wait()
	return results, nil
}

// functionLinker returns f's source path and the linker script that binds the
// names the source uses and places its sections at their target addresses.
func (p project) functionLinker(resolver symbolResolver, m *moduleSpec, f *functionSpec) (string, string, error) {
	source := filepath.Join(p.root, filepath.FromSlash(m.sourceOf(f)))
	used, err := sourceIdentifiers(source)
	if err != nil {
		return "", "", fmt.Errorf("read source: %w", err)
	}
	bindings := resolver.linkerBindings(m, f, used)
	if profile, ok := compilerProfileNamed(m.profileOf(f)); ok && profile.smallDataLimit != 0 {
		bindings["_gp"] = profile.globalPointer
	}
	script, err := linkerScript(f, bindings)
	return source, script, err
}

// compileFunction compiles one configured function with its profile, linked
// at its target address, under workDir/<name>, and returns its allocatable
// sections.
func (p project) compileFunction(resolver symbolResolver, m *moduleSpec, f *functionSpec, workDir string) (map[string][]byte, error) {
	source, script, err := p.functionLinker(resolver, m, f)
	if err != nil {
		return nil, err
	}
	linker, err := writeLinkerScript(workDir, f.Name, script)
	if err != nil {
		return nil, err
	}
	profile, ok := compilerProfileNamed(m.profileOf(f))
	if !ok {
		return nil, fmt.Errorf("unknown compiler profile %q", m.profileOf(f))
	}
	sections, err := p.compileHistoricalSections(source, linker, filepath.Join(workDir, f.Name), profile)
	if err == nil {
		return sections, nil
	}
	// Header macros can introduce names absent from the source scan. Bind only
	// actual undefined object symbols that resolve in this module's context;
	// unknown names still fail the link, and overrides keep their usual priority.
	object := filepath.Join(workDir, f.Name, profile.name+".o")
	if used, objectErr := undefinedObjectIdentifiers(object); objectErr == nil {
		sourceUsed, readErr := sourceIdentifiers(source)
		if readErr == nil {
			for name := range sourceUsed {
				used[name] = struct{}{}
			}
			bindings := resolver.linkerBindings(m, f, used)
			if profile.smallDataLimit != 0 {
				bindings["_gp"] = profile.globalPointer
			}
			if expanded, scriptErr := linkerScript(f, bindings); scriptErr == nil && expanded != script {
				if _, writeErr := writeLinkerScript(workDir, f.Name, expanded); writeErr == nil {
					return p.compileHistoricalSections(source, linker, filepath.Join(workDir, f.Name), profile)
				}
			}
		}
	}
	// A source read while an editor rewrites it can yield a script without
	// its bindings; when a fresh read differs, relink with that once.
	if _, fresh, freshErr := p.functionLinker(resolver, m, f); freshErr == nil && fresh != script {
		if _, writeErr := writeLinkerScript(workDir, f.Name, fresh); writeErr == nil {
			return p.compileHistoricalSections(source, linker, filepath.Join(workDir, f.Name), profile)
		}
	}
	return nil, err
}

// undeclaredSection names a compiled allocatable section the configuration
// does not account for.
func undeclaredSection(f *functionSpec, sections map[string][]byte) string {
	for name := range sections {
		if name != ".text" && (name != ".rodata" || len(f.Rodata) == 0) {
			return name
		}
	}
	return ""
}

func (p project) readModuleImage(m *moduleSpec) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(p.filesRoot, filepath.FromSlash(m.File)))
	if err != nil {
		return nil, errNotExtracted
	}
	if len(data) != m.Size {
		return nil, fmt.Errorf("%s: extracted %s has %d bytes, configured %d", m.ID, m.File, len(data), m.Size)
	}
	if sum := sha256.Sum256(data); hex.EncodeToString(sum[:]) != m.SHA256 {
		return nil, fmt.Errorf("%s: extracted %s does not match its configured sha256", m.ID, m.File)
	}
	return data, nil
}

// patchModule replaces every configured function in a copy of the original
// image with its compiled bytes, failing on the first difference unless
// TOOLS_REPORT_ALL_MISMATCHES is set, and writes the copy under build/patch.
func (p project) patchModule(config *projectConfig, resolver symbolResolver, m *moduleSpec) (string, error) {
	want, err := p.readModuleImage(m)
	if err != nil {
		return "", err
	}
	results, err := p.compileModule(config, resolver, m, nil)
	if err != nil {
		return "", err
	}
	got := append([]byte(nil), want...)
	languageBytes := make(map[string]int)
	languageCounts := make(map[string]int)
	var mismatched []string
	for index := range m.Functions {
		f := &m.Functions[index]
		language := "C"
		if f.Asm != "" {
			language = "asm"
		}
		if results[index].err != nil {
			return "", fmt.Errorf("compile %s: %w", f.Name, results[index].err)
		}
		sections := results[index].sections
		start := m.offset(f.Addr)
		expected := want[start : start+f.Size]
		if shortHash(expected) != f.Hash {
			return "", fmt.Errorf("%s: %s hash does not match the original bytes; run `make checksums`", m.ID, f.Name)
		}
		if offset, differs := mismatch(expected, sections[".text"]); differs {
			line := fmt.Sprintf("%s mismatch at function offset 0x%x (file offset 0x%x)", f.Name, offset, start+offset)
			if os.Getenv("TOOLS_REPORT_ALL_MISMATCHES") == "" {
				return "", errors.New(line)
			}
			mismatched = append(mismatched, line)
			continue
		}
		copy(got[start:], sections[".text"])
		languageCounts[language]++
		languageBytes[language] += f.Size
		fmt.Fprintf(p.stdout(), "matching %s: %s (%d bytes)\n", language, f.Name, f.Size)
		for _, rodata := range f.Rodata {
			rodataStart := m.offset(rodata.Addr)
			expected := want[rodataStart : rodataStart+rodata.Size]
			if shortHash(expected) != rodata.Hash {
				return "", fmt.Errorf("%s: %s rodata 0x%08x hash does not match the original bytes; run `make checksums`", m.ID, f.Name, rodata.Addr)
			}
			compiled, ok := sections[".rodata"]
			if !ok {
				return "", fmt.Errorf("%s did not emit declared section .rodata", f.Name)
			}
			if offset, differs := mismatch(expected, compiled); differs {
				return "", fmt.Errorf("%s .rodata mismatch at section offset 0x%x (file offset 0x%x)", f.Name, offset, rodataStart+offset)
			}
			copy(got[rodataStart:], compiled)
			fmt.Fprintf(p.stdout(), "matching %s data: %s .rodata (%d bytes)\n", language, f.Name, len(compiled))
		}
		if section := undeclaredSection(f, sections); section != "" {
			return "", fmt.Errorf("%s emitted undeclared allocatable section %s", f.Name, section)
		}
	}
	if len(mismatched) > 0 {
		for _, line := range mismatched {
			fmt.Fprintf(os.Stderr, "MISMATCH: %s\n", line)
		}
		return "", fmt.Errorf("%d function(s) did not match", len(mismatched))
	}
	output := filepath.Join(p.root, "build", "patch", m.ID, filepath.Base(filepath.FromSlash(m.File)))
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(output, got, 0o644); err != nil {
		return "", err
	}
	for _, language := range []string{"C", "asm"} {
		if languageCounts[language] != 0 {
			fmt.Fprintf(p.stdout(), "matching %s total: %d bytes across %d functions\n", language, languageBytes[language], languageCounts[language])
		}
	}
	relative, err := filepath.Rel(p.root, output)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(p.stdout(), "byte-exact: %s (%d bytes)\n", filepath.ToSlash(relative), len(got))
	return output, nil
}

// loadCheckedConfig loads the configuration and runs check-config's checks.
func (p project) loadCheckedConfig() (*projectConfig, error) {
	config, err := p.loadProjectConfig()
	if err != nil {
		return nil, err
	}
	if err := p.validateProjectConfig(config); err != nil {
		return nil, err
	}
	if err := validateDeclarations(p.root); err != nil {
		return nil, err
	}
	if err := validatePlaceholderMembers(p.root); err != nil {
		return nil, err
	}
	if err := validateAddressLiterals(p.root); err != nil {
		return nil, err
	}
	if err := validateConditionals(p.root, config.Editions); err != nil {
		return nil, err
	}
	return config, nil
}

// patchCheckCommand is make patch-check, make build's cross-check the way
// the disc was built before the link: each selected module's functions
// compiled alone at their addresses and written over a copy of its original
// image, which must stay identical, and with every module the disc from the
// copies in build/patch/disc.
func (p project) patchCheckCommand(args []string) error {
	flags := flag.NewFlagSet("patch-check", flag.ContinueOnError)
	selection := flags.String("module", "", "modules to check (default: all, and the disc)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("usage: tools patch-check [--module=M]")
	}
	config, err := p.loadCheckedConfig()
	if err != nil {
		return err
	}
	modules := config.Modules
	if *selection != "" {
		if modules, err = resolveModuleSelection(config, *selection); err != nil {
			return err
		}
	}
	p, err = p.withInputMirror()
	if err != nil {
		return err
	}
	p.progress = newCompileProgress("patch-check", modules)
	resolver := newSymbolResolver(config)
	// Patch every module concurrently (most overlays hold only a few
	// functions); compileSlots caps the total concurrent compiles at toolJobs.
	jobs, err := toolJobs()
	if err != nil {
		return err
	}
	compileSlots = make(chan struct{}, jobs)
	defer func() { compileSlots = nil }()
	type moduleResult struct {
		output string
		err    error
		log    bytes.Buffer
	}
	results := make([]*moduleResult, len(modules))
	var moduleWorkers sync.WaitGroup
	for index, m := range modules {
		if len(m.Functions) == 0 {
			continue
		}
		result := &moduleResult{}
		results[index] = result
		moduleWorkers.Add(1)
		go func(m *moduleSpec, result *moduleResult) {
			defer moduleWorkers.Done()
			modulePass := p
			modulePass.out = &result.log
			result.output, result.err = modulePass.patchModule(config, resolver, m)
		}(m, result)
	}
	moduleWorkers.Wait()
	built := make(map[string]string)
	for index, result := range results {
		if result == nil {
			continue
		}
		os.Stdout.Write(result.log.Bytes())
		if result.err != nil {
			return result.err
		}
		built[modules[index].ID] = result.output
	}
	if *selection != "" {
		return nil
	}
	return p.writeDisc(config, built, filepath.Join(p.root, "build", "patch", "disc"), true)
}

// writeDisc runs mkpsxiso on the project target/disc.yaml describes, with
// each module in built (id to rebuilt file) in place of its extracted file,
// writes the disc under buildDir and checks every rebuilt module at its LBA
// and, for the identical disc (exact), the disc's SHA-256.
func (p project) writeDisc(config *projectConfig, built map[string]string, buildDir string, exact bool) error {
	if !p.validExtraction() {
		return errNotExtracted
	}
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return err
	}
	outputs := make(map[string]string, len(built))
	for id, output := range built {
		outputs[id] = filepath.ToSlash(output)
	}
	buildXML := filepath.Join(buildDir, "disc.xml")
	if err := os.WriteFile(buildXML, config.Disc.mkpsxisoProject(config, filepath.ToSlash(p.filesRoot), outputs), 0o644); err != nil {
		return err
	}
	output := filepath.Join(buildDir, targetOutput+".bin")
	cue := filepath.Join(buildDir, targetOutput+".cue")
	// mkpsxiso writes into an existing image in place, and a blank Form 2
	// sector clears only its first 2048 data bytes: the rest of an earlier
	// disc's sector would stay. Each disc starts from a new file.
	if err := os.Remove(output); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := runInDir(p.root, "mkpsxiso", "-q", "-y", "-o", output, "-c", cue, buildXML); err != nil {
		return err
	}
	for _, m := range config.Modules {
		path, ok := built[m.ID]
		if !ok {
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		got, err := readMode2File(output, m.LBA, len(want))
		if err != nil {
			return err
		}
		if offset, differs := mismatch(want, got); differs {
			return fmt.Errorf("rebuilt disc %s mismatch at file offset 0x%x", m.File, offset)
		}
		fmt.Printf("disc module exact: %s at LBA %d (%d bytes)\n", m.File, m.LBA, len(got))
	}
	discSHA256, err := sha256File(output)
	if err != nil {
		return err
	}
	switch {
	case !exact && p.edition != "":
		fmt.Printf("disc image: sha256 %s (edition %s)\n", discSHA256, p.edition)
	case !exact:
		fmt.Printf("disc image: sha256 %s (a test disc, not the original)\n", discSHA256)
	case discSHA256 != targetInputSHA256:
		return fmt.Errorf("rebuilt disc SHA-256 mismatch: got %s, expected %s", discSHA256, targetInputSHA256)
	default:
		fmt.Printf("disc image exact: sha256 %s\n", discSHA256)
	}
	relative, err := filepath.Rel(p.root, cue)
	if err != nil {
		return err
	}
	fmt.Printf("built disc: %s\n", filepath.ToSlash(relative))
	return nil
}

// checkCommand compiles functions and compares their bytes with the
// configured hashes. It never reads the disc or the extracted files.
func (p project) checkCommand(args []string) error {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	moduleName := flags.String("module", "", "modules to check (ids, short names, event or effect)")
	edition := flags.String("edition", "", "edition of target/editions.yaml to compile (default: the original disc)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("usage: tools check [--module=MODULES] [--edition=E]")
	}
	config, err := p.loadCheckedConfig()
	if err != nil {
		return err
	}
	modules := config.Modules
	if *moduleName != "" {
		if modules, err = resolveModuleSelection(config, *moduleName); err != nil {
			return err
		}
	}
	if p, err = p.withEdition(config, *edition); err != nil {
		return err
	}
	p, err = p.withInputMirror()
	if err != nil {
		return err
	}
	p.progress = newCompileProgress("check", modules)
	jobs, err := toolJobs()
	if err != nil {
		return err
	}
	compileSlots = make(chan struct{}, jobs)
	defer func() { compileSlots = nil }()
	resolver := newSymbolResolver(config)
	type moduleCheck struct {
		results []compiledFunction
		err     error
	}
	checks := make([]moduleCheck, len(modules))
	var workers sync.WaitGroup
	for index, m := range modules {
		workers.Add(1)
		go func(index int, m *moduleSpec) {
			defer workers.Done()
			checks[index].results, checks[index].err = p.compileModule(config, resolver, m, nil)
		}(index, m)
	}
	workers.Wait()
	var problems []string
	functions, bytesChecked := 0, 0
	for index, m := range modules {
		if checks[index].err != nil {
			return checks[index].err
		}
		for functionIndex := range m.Functions {
			f := &m.Functions[functionIndex]
			result := checks[index].results[functionIndex]
			where := fmt.Sprintf("%s %s 0x%08x", m.ID, f.Name, f.Addr)
			if result.err != nil {
				problems = append(problems, fmt.Sprintf("%s: compile failed: %v", where, result.err))
				continue
			}
			text := result.sections[".text"]
			if len(text) != f.Size || shortHash(text) != f.Hash {
				problems = append(problems, fmt.Sprintf("%s: .text is %d bytes with hash %s, expected %d bytes with hash %s", where, len(text), shortHash(text), f.Size, f.Hash))
			}
			for _, rodata := range f.Rodata {
				compiled := result.sections[".rodata"]
				if len(compiled) != rodata.Size || shortHash(compiled) != rodata.Hash {
					problems = append(problems, fmt.Sprintf("%s: .rodata 0x%08x is %d bytes with hash %s, expected %d bytes with hash %s", where, rodata.Addr, len(compiled), shortHash(compiled), rodata.Size, rodata.Hash))
				}
			}
			if section := undeclaredSection(f, result.sections); section != "" {
				problems = append(problems, fmt.Sprintf("%s: emitted undeclared allocatable section %s", where, section))
			}
			functions++
			bytesChecked += f.Size
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		for _, problem := range problems {
			fmt.Fprintf(os.Stderr, "MISMATCH %s\n", problem)
		}
		return fmt.Errorf("check: %d of %d functions differ from their configured hashes", len(problems), functions)
	}
	label := ""
	if p.edition != "" {
		label = ", edition " + p.edition
	}
	fmt.Printf("check: %d functions in %d modules match their hashes (%d bytes%s)\n", functions, len(modules), bytesChecked, label)
	return nil
}

// checksumsCommand records the hash of every function and .rodata range from
// the original module images, never from compiled output.
func (p project) checksumsCommand(args []string) error {
	if len(args) != 0 {
		return errors.New("usage: tools checksums")
	}
	return p.withTargetLock(func() error {
		config, err := p.loadProjectConfig()
		if err != nil {
			return err
		}
		changed, total := 0, 0
		for _, m := range config.Modules {
			if len(m.Functions) == 0 {
				continue
			}
			image, err := p.readModuleImage(m)
			if err != nil {
				return err
			}
			record := func(addr uint32, size int, hash *string) error {
				if !m.contains(addr, size) || size <= 0 {
					return fmt.Errorf("%s: range 0x%08x+%d lies outside %s", m.ID, addr, size, m.File)
				}
				start := m.offset(addr)
				value := shortHash(image[start : start+size])
				total++
				if *hash != value {
					*hash = value
					changed++
				}
				return nil
			}
			for index := range m.Functions {
				f := &m.Functions[index]
				if err := record(f.Addr, f.Size, &f.Hash); err != nil {
					return err
				}
				for rodataIndex := range f.Rodata {
					rodata := &f.Rodata[rodataIndex]
					if err := record(rodata.Addr, rodata.Size, &rodata.Hash); err != nil {
						return err
					}
				}
			}
		}
		files, err := p.writeProjectConfig(config)
		if err != nil {
			return err
		}
		fmt.Printf("checksums: %d of %d hashes updated from the original bytes (%d files written)\n", changed, total, files)
		return nil
	})
}
