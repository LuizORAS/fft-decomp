// edition_guard.go is an edition's change guard and the `change` and
// `dc_hash` of a function row. A function that compiles differently with one
// of the edition's options lists exactly those options in its change: make
// check and make build with an edition run the guard first and fail, listing
// every function, on a change a row does not declare and on a declared option
// that changes nothing. The guard compiles each function as the original disc
// builds it and with each option alone, and compares the preprocessed text
// and, when the text differs, the relocatable objects: their loaded sections,
// their relocations by symbol name and their global symbols, without the
// line notes and the debug information. A function whose source and headers
// never name an option is not even preprocessed. A changed function's
// dc_hash, which only make checksums EDITION=dc writes, is the hash of the
// code and .rodata the Developer's Cut compiles at the function's address, so
// that make check EDITION=dc catches, without the BIN, a fix that changes
// again.
package main

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
)

// developersCut is the edition whose compiled code a changed function's
// dc_hash records.
const developersCut = "dc"

var directiveInclude = regexp.MustCompile(`^#[ \t]*include[ \t]*(?:"([^"]+)"|<([^>]+)>)`)

// changes reports whether f lists in its change an option p's edition builds.
func (p project) changes(f *functionSpec) bool {
	for _, option := range f.Change {
		if slices.Contains(p.options, option) {
			return true
		}
	}
	return false
}

// editionHash is a changed function's dc_hash: the first 16 hex digits of the
// SHA-256 of its code followed by its .rodata.
func editionHash(sections map[string][]byte) string {
	return shortHash(append(slices.Clone(sections[".text"]), sections[".rodata"]...))
}

// editionHashOf returns the hash p's edition must compile a changed f to.
func (p project) editionHashOf(f *functionSpec) (string, error) {
	if p.edition != developersCut {
		return "", fmt.Errorf("%s changes in edition %s, which keeps no hash of its changed functions (only %s has dc_hash)", f.Name, p.edition, developersCut)
	}
	return f.DCHash, nil
}

// validateChange checks a function row's change and dc_hash: each option is
// one an edition builds, listed once; a routine in assembly is the original
// code and never changes; dc_hash is there exactly when the Developer's Cut
// builds an option of the change.
func validateChange(editions *editionsFile, f *functionSpec, where string, fail func(string, ...any)) {
	if len(f.Change) == 0 && f.DCHash == "" {
		return
	}
	declared, built := map[string]bool{}, map[string]bool{}
	var developersCutOptions []string
	if editions != nil {
		for _, option := range editions.Options {
			declared[option.Name] = true
		}
		for _, edition := range editions.Editions {
			for _, option := range edition.Options {
				built[option] = true
			}
			if edition.Name == developersCut {
				developersCutOptions = edition.Options
			}
		}
	}
	if f.Asm != "" && len(f.Change) > 0 {
		fail("%s: a routine in assembly is the original code and takes no change", where)
	}
	inDevelopersCut := false
	for index, option := range f.Change {
		switch {
		case !declared[option]:
			fail("%s: change names %s, which is no option of %s", where, option, editionsPath)
		case !built[option]:
			fail("%s: change names %s, which no edition of %s builds", where, option, editionsPath)
		case slices.Contains(f.Change[:index], option):
			fail("%s: change names %s twice", where, option)
		}
		inDevelopersCut = inDevelopersCut || slices.Contains(developersCutOptions, option)
	}
	switch {
	case inDevelopersCut && !validHash(f.DCHash):
		fail("%s changes in edition %s but has no valid dc_hash; run `make checksums EDITION=%s`", where, developersCut, developersCut)
	case !inDevelopersCut && f.DCHash != "":
		fail("%s has a dc_hash but no change in edition %s", where, developersCut)
	}
}

// guardScan finds the options of an edition each source can see: those its
// text, or a header it includes at any depth, names anywhere (a comment too,
// which only costs a comparison). Every #include of the tree is a literal
// path, found as cpp -Iinclude finds it: next to the including file, then
// under include/.
type guardScan struct {
	p     project
	names *regexp.Regexp
	mu    sync.Mutex
	files map[string]guardFile
}

// guardFile is one scanned file: the options it names and the files it
// includes.
type guardFile struct {
	options  map[string]bool
	includes []string
}

func newGuardScan(p project) *guardScan {
	quoted := make([]string, len(p.options))
	for index, option := range p.options {
		quoted[index] = regexp.QuoteMeta(option)
	}
	return &guardScan{p: p, names: regexp.MustCompile(`\b(?:` + strings.Join(quoted, "|") + `)\b`), files: map[string]guardFile{}}
}

// exists reports whether path is a file, read from the input mirror when
// there is one.
func (s *guardScan) exists(path string) bool {
	_, input := s.p.preprocessInput(path)
	info, err := os.Stat(input)
	return err == nil && !info.IsDir()
}

// file scans path once.
func (s *guardScan) file(path string) (guardFile, error) {
	s.mu.Lock()
	scanned, ok := s.files[path]
	s.mu.Unlock()
	if ok {
		return scanned, nil
	}
	_, input := s.p.preprocessInput(path)
	data, err := os.ReadFile(input)
	if err != nil {
		return guardFile{}, err
	}
	text := string(data)
	scanned = guardFile{options: map[string]bool{}}
	for _, name := range s.names.FindAllString(text, -1) {
		scanned.options[name] = true
	}
	include := filepath.Join(s.p.root, "include")
	for _, directive := range cDirectives(text) {
		match := directiveInclude.FindStringSubmatch(strings.TrimSpace(directive.text))
		if match == nil {
			continue
		}
		candidates := []string{filepath.Join(include, filepath.FromSlash(match[2]))}
		if match[1] != "" {
			candidates = []string{filepath.Join(filepath.Dir(path), filepath.FromSlash(match[1])), filepath.Join(include, filepath.FromSlash(match[1]))}
		}
		found := ""
		for _, candidate := range candidates {
			if s.exists(candidate) {
				found = candidate
				break
			}
		}
		if found == "" {
			// cpp would not find it either; count it as naming every option.
			for _, option := range s.p.options {
				scanned.options[option] = true
			}
			continue
		}
		scanned.includes = append(scanned.includes, found)
	}
	s.mu.Lock()
	s.files[path] = scanned
	s.mu.Unlock()
	return scanned, nil
}

// visible returns, in the edition's order, the options source or a file it
// includes names.
func (s *guardScan) visible(source string) ([]string, error) {
	named, seen := map[string]bool{}, map[string]bool{}
	pending := []string{source}
	for len(pending) > 0 {
		path := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if seen[path] {
			continue
		}
		seen[path] = true
		scanned, err := s.file(path)
		if err != nil {
			return nil, err
		}
		for option := range scanned.options {
			named[option] = true
		}
		pending = append(pending, scanned.includes...)
	}
	var visible []string
	for _, option := range s.p.options {
		if named[option] {
			visible = append(visible, option)
		}
	}
	return visible, nil
}

// objectSignature describes what a relocatable object puts in memory: each
// loaded section's bytes (.reginfo and .MIPS.abiflags aside), its relocations
// by offset, type and symbol name, and the global symbols. Two objects with
// one signature link to the same bytes wherever they are placed.
func objectSignature(path string) (string, error) {
	object, err := elf.Open(path)
	if err != nil {
		return "", err
	}
	defer object.Close()
	symbols, err := object.Symbols()
	if err != nil {
		return "", err
	}
	// A relocation's symbol index counts the null symbol Symbols leaves out.
	symbolName := func(index uint32) string {
		if index == 0 || int(index) > len(symbols) {
			return ""
		}
		symbol := symbols[index-1]
		if elf.ST_TYPE(symbol.Info) == elf.STT_SECTION && int(symbol.Section) < len(object.Sections) {
			return object.Sections[symbol.Section].Name
		}
		return symbol.Name
	}
	var out strings.Builder
	loaded := map[uint32]bool{}
	for index, section := range object.Sections {
		if section.Flags&elf.SHF_ALLOC == 0 || section.Size == 0 || section.Name == ".reginfo" || section.Name == ".MIPS.abiflags" {
			continue
		}
		loaded[uint32(index)] = true
		fmt.Fprintf(&out, "section %s %d bytes", section.Name, section.Size)
		if section.Type != elf.SHT_NOBITS {
			data, err := section.Data()
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&out, " %x", data)
		}
		out.WriteString("\n")
	}
	for _, section := range object.Sections {
		if !loaded[section.Info] || (section.Type != elf.SHT_REL && section.Type != elf.SHT_RELA) {
			continue
		}
		if section.Type == elf.SHT_RELA {
			return "", fmt.Errorf("%s: unexpected RELA relocations in %s", path, section.Name)
		}
		data, err := section.Data()
		if err != nil {
			return "", err
		}
		for offset := 0; offset+8 <= len(data); offset += 8 {
			at := binary.LittleEndian.Uint32(data[offset:])
			info := binary.LittleEndian.Uint32(data[offset+4:])
			fmt.Fprintf(&out, "relocation %s+0x%x %s %s\n", object.Sections[section.Info].Name, at, elf.R_MIPS(info&0xff), symbolName(info>>8))
		}
	}
	var globals []string
	for _, symbol := range symbols {
		if elf.ST_BIND(symbol.Info) == elf.STB_LOCAL {
			continue
		}
		where := "undefined"
		switch {
		case symbol.Section == elf.SHN_UNDEF:
		case symbol.Section == elf.SHN_ABS:
			where = "absolute"
		case symbol.Section == elf.SHN_COMMON:
			where = "common"
		case int(symbol.Section) < len(object.Sections):
			where = object.Sections[symbol.Section].Name
		}
		globals = append(globals, fmt.Sprintf("symbol %s %s %s+0x%x %d\n", symbol.Name, elf.ST_TYPE(symbol.Info), where, symbol.Value, symbol.Size))
	}
	sort.Strings(globals)
	out.WriteString(strings.Join(globals, ""))
	return out.String(), nil
}

// guardFunction compiles f as the original disc builds it, with each visible
// option alone and, when there are several, with all of them. It returns the
// options that alone change f and whether all of them together do.
func (p project) guardFunction(m *moduleSpec, f *functionSpec, visible []string) (map[string]bool, bool, error) {
	profile, ok := compilerProfileNamed(m.profileOf(f))
	if !ok {
		return nil, false, fmt.Errorf("unknown compiler profile %q", m.profileOf(f))
	}
	source := filepath.Join(p.root, filepath.FromSlash(m.sourceOf(f)))
	workDir := filepath.Join(p.workDir("guard"), m.ID, f.Name)
	type build struct {
		project                     project
		preprocessed, language, dir string
		text                        []byte
	}
	compile := func(variant string, options []string) (build, error) {
		b := build{project: p, dir: filepath.Join(workDir, variant)}
		b.project.options = options
		var err error
		if b.preprocessed, b.language, err = b.project.historicalInput(source, b.dir, profile); err != nil {
			return b, err
		}
		b.text, err = os.ReadFile(b.preprocessed)
		return b, err
	}
	signature := func(b build) (string, error) {
		object, err := b.project.historicalObject(source, b.preprocessed, b.language, b.dir, profile)
		if err != nil {
			return "", err
		}
		return objectSignature(object)
	}
	original, err := compile("original", nil)
	if err != nil {
		return nil, false, err
	}
	originalSignature := ""
	differs := func(variant string, options []string) (bool, error) {
		b, err := compile(variant, options)
		if err != nil || bytes.Equal(b.text, original.text) {
			return false, err
		}
		if originalSignature == "" {
			if originalSignature, err = signature(original); err != nil {
				return false, err
			}
		}
		changed, err := signature(b)
		return changed != originalSignature, err
	}
	changed := map[string]bool{}
	together := false
	for _, option := range visible {
		if changed[option], err = differs(option, []string{option}); err != nil {
			return nil, false, err
		}
		together = together || changed[option]
	}
	if len(visible) > 1 {
		if together, err = differs("together", visible); err != nil {
			return nil, false, err
		}
	}
	return changed, together, nil
}

// guardProblems compares what a function declares with what the guard found:
// declared holds the options of its change the edition builds, visible the
// options its source and headers name (the only ones that can change it),
// changed which of those alone change it, and together whether all of them
// at once do.
func guardProblems(declared, visible []string, changed map[string]bool, together bool) []string {
	var problems []string
	alone := false
	for _, option := range visible {
		alone = alone || changed[option]
		switch {
		case changed[option] && !slices.Contains(declared, option):
			problems = append(problems, fmt.Sprintf("changes under %s, which its change does not list", option))
		case !changed[option] && slices.Contains(declared, option):
			problems = append(problems, fmt.Sprintf("declares %s, which does not change it", option))
		}
	}
	for _, option := range declared {
		if !slices.Contains(visible, option) {
			problems = append(problems, fmt.Sprintf("declares %s, which neither its source nor a header it includes names", option))
		}
	}
	if together != alone {
		problems = append(problems, fmt.Sprintf("compiles differently with %s together than with each alone; give each option its own #ifdef block", strings.Join(visible, " and ")))
	}
	return problems
}

// editionGuard runs the change guard of p's edition over modules' C
// functions. It prints what it compared and fails, after listing every
// problem, when a function does not declare its changes as they are.
func (p project) editionGuard(modules []*moduleSpec) error {
	problems, seen, changed, err := p.guardModules(modules)
	if err != nil {
		return err
	}
	if len(problems) > 0 {
		for _, problem := range problems {
			fmt.Fprintf(os.Stderr, "GUARD %s\n", problem)
		}
		return fmt.Errorf("guard: %d problems in edition %s: a function that compiles differently lists in its change exactly the options that change it", len(problems), p.edition)
	}
	fmt.Fprintf(p.stdout(), "guard: %d functions see an option of edition %s in their source or headers; %d change, each under the options its change declares\n", seen, p.edition, changed)
	return nil
}

// guardModules compares every C function of modules and returns the sorted
// problems, how many functions see an option and how many change.
func (p project) guardModules(modules []*moduleSpec) ([]string, int, int, error) {
	scan := newGuardScan(p)
	type candidate struct {
		m        *moduleSpec
		f        *functionSpec
		declared []string
		visible  []string
		changed  map[string]bool
		together bool
		err      error
	}
	var problems []string
	report := func(m *moduleSpec, f *functionSpec, found []string) {
		for _, problem := range found {
			problems = append(problems, fmt.Sprintf("%s %s 0x%08x: %s", m.ID, f.Name, f.Addr, problem))
		}
	}
	var candidates []*candidate
	for _, m := range modules {
		for index := range m.Functions {
			f := &m.Functions[index]
			if f.Asm != "" {
				continue
			}
			c := &candidate{m: m, f: f}
			for _, option := range f.Change {
				if slices.Contains(p.options, option) {
					c.declared = append(c.declared, option)
				}
			}
			var err error
			if c.visible, err = scan.visible(filepath.Join(p.root, filepath.FromSlash(m.sourceOf(f)))); err != nil {
				return nil, 0, 0, err
			}
			if len(c.visible) == 0 {
				report(m, f, guardProblems(c.declared, nil, nil, false))
				continue
			}
			candidates = append(candidates, c)
		}
	}
	jobs, err := toolJobs()
	if err != nil {
		return nil, 0, 0, err
	}
	work := make(chan *candidate)
	var workers sync.WaitGroup
	for range min(jobs, len(candidates)) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for c := range work {
				c.changed, c.together, c.err = p.guardFunction(c.m, c.f, c.visible)
			}
		}()
	}
	for _, c := range candidates {
		work <- c
	}
	close(work)
	workers.Wait()
	changed := 0
	var errs []error
	for _, c := range candidates {
		if c.err != nil {
			errs = append(errs, fmt.Errorf("guard: %s %s: %w", c.m.ID, c.f.Name, c.err))
			continue
		}
		report(c.m, c.f, guardProblems(c.declared, c.visible, c.changed, c.together))
		if c.together {
			changed++
		}
	}
	if err := errors.Join(errs...); err != nil {
		return nil, 0, 0, err
	}
	sort.Strings(problems)
	return problems, len(candidates), changed, nil
}

// checkEditionImage compares an edition's linked image of m with the
// original: each changed function, code and .rodata, with its dc_hash; every
// other byte with the original file or, without the BIN, every other
// function with its hash. The edition's code keeps the original addresses,
// so a changed function keeps its size (the link fails otherwise).
func (p project) checkEditionImage(m *moduleSpec, want, got []byte) (bool, string, error) {
	restored := slices.Clone(got)
	changed := 0
	for index := range m.Functions {
		f := &m.Functions[index]
		ranges := [][2]int{{m.offset(f.Addr), m.offset(f.Addr) + f.Size}}
		hashes := []string{f.Hash}
		for _, rodata := range f.Rodata {
			ranges = append(ranges, [2]int{m.offset(rodata.Addr), m.offset(rodata.Addr) + rodata.Size})
			hashes = append(hashes, rodata.Hash)
		}
		if !p.changes(f) {
			if want == nil {
				for at, span := range ranges {
					if shortHash(got[span[0]:span[1]]) != hashes[at] {
						return false, "", fmt.Errorf("%s: %s differs from its hash", m.ID, f.Name)
					}
				}
			}
			continue
		}
		expected, err := p.editionHashOf(f)
		if err != nil {
			return false, "", err
		}
		var code []byte
		for _, span := range ranges {
			code = append(code, got[span[0]:span[1]]...)
			if want != nil {
				copy(restored[span[0]:span[1]], want[span[0]:span[1]])
			}
		}
		if shortHash(code) != expected {
			return false, "", fmt.Errorf("%s: %s differs from its dc_hash", m.ID, f.Name)
		}
		changed++
	}
	if want == nil {
		return false, fmt.Sprintf("%d function hashes, %d of them dc_hash (no disc: data zeroed)", len(m.Functions), changed), nil
	}
	if offset, differs := mismatch(want, restored); differs {
		return false, "", fmt.Errorf("%s: linked image differs from the original at file offset 0x%x (0x%08x), outside its changed functions", m.ID, offset, m.Load+uint32(offset))
	}
	sum := shortHash(got)
	if changed == 0 {
		return true, "sha256 " + sum, nil
	}
	return false, fmt.Sprintf("sha256 %s; %d changed functions at their dc_hash, every other byte the original's", sum, changed), nil
}

// editionChecksums is make checksums EDITION=dc: once the guard passes, each
// function whose change the edition builds gets the dc_hash of the code and
// .rodata the edition compiles at its address, and every other function
// loses a stale one. It reads no BIN.
func (p project) editionChecksums(config *projectConfig, name string) error {
	if name != developersCut {
		return fmt.Errorf("make checksums EDITION=%s: only edition %s keeps a hash of its changed functions (dc_hash)", name, developersCut)
	}
	p, err := p.withEdition(config, name)
	if err != nil {
		return err
	}
	if p, err = p.withInputMirror(); err != nil {
		return err
	}
	if err := p.editionGuard(config.Modules); err != nil {
		return err
	}
	resolver := newSymbolResolver(config)
	updated, total := 0, 0
	for _, m := range config.Modules {
		only := map[string]bool{}
		for index := range m.Functions {
			f := &m.Functions[index]
			if p.changes(f) {
				only[f.Name] = true
			} else if f.DCHash != "" {
				f.DCHash = ""
				updated++
			}
		}
		if len(only) == 0 {
			continue
		}
		results, err := p.compileModule(config, resolver, m, only)
		if err != nil {
			return err
		}
		for index := range m.Functions {
			f := &m.Functions[index]
			if !only[f.Name] {
				continue
			}
			if results[index].err != nil {
				return fmt.Errorf("%s %s: %w", m.ID, f.Name, results[index].err)
			}
			total++
			if value := editionHash(results[index].sections); f.DCHash != value {
				f.DCHash = value
				updated++
			}
		}
	}
	files, err := p.writeProjectConfig(config)
	if err != nil {
		return err
	}
	fmt.Printf("checksums: %d of %d dc_hash values updated from the code edition %s compiles (%d files written)\n", updated, total, p.edition, files)
	return nil
}
