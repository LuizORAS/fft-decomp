// edition_edits.go reads, checks and canonically writes target/edits.yaml: the
// bytes an option changes in place, in a module's data between functions or
// in a disc file that is not a module. The repository keeps only the new
// bytes and the hash of the original bytes they replace, which only make
// checksums writes, from the BIN. An edition's make build writes the edits of
// its options: a module's in its link (linkDataAssembly), a file's in a copy
// under build/<edition>/files that its disc takes.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const editsPath = modulesConfigDir + "/edits.yaml"

// editsFile is target/edits.yaml.
type editsFile struct {
	Comment string
	Edits   []dataEdit
}

// dataEdit replaces len(Bytes) bytes in place: at Addr in Module's image, or
// at Offset in File. Hash is the hash of the original bytes it replaces.
type dataEdit struct {
	Module string
	Addr   uint32
	File   string
	Offset uint32
	Bytes  []byte
	Option string
	Hash   string
	Why    string
}

type editYAML struct {
	Module string `yaml:"module"`
	Addr   uint32 `yaml:"addr"`
	File   string `yaml:"file"`
	Offset uint32 `yaml:"offset"`
	Bytes  string `yaml:"bytes"`
	Option string `yaml:"option"`
	Hash   string `yaml:"hash"`
	Why    string `yaml:"why"`
}

type editsYAML struct {
	Edits []editYAML `yaml:"edits"`
}

// start and end are the edit's range: addresses in a module, offsets in a
// file.
func (e dataEdit) start() uint32 {
	if e.Module != "" {
		return e.Addr
	}
	return e.Offset
}

func (e dataEdit) end() uint32 { return e.start() + uint32(len(e.Bytes)) }

func (e dataEdit) where() string {
	if e.Module != "" {
		return fmt.Sprintf("%s 0x%08x", e.Module, e.Addr)
	}
	return fmt.Sprintf("%s+0x%x", e.File, e.Offset)
}

func parseEdits(data []byte) (*editsFile, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var file editsYAML
	if err := decoder.Decode(&file); err != nil {
		return nil, err
	}
	edits := &editsFile{Comment: leadingComment(data)}
	for index, item := range file.Edits {
		values, err := parseEditBytes(item.Bytes)
		if err != nil {
			return nil, fmt.Errorf("edit %d: %w", index+1, err)
		}
		edits.Edits = append(edits.Edits, dataEdit{Module: item.Module, Addr: item.Addr, File: item.File, Offset: item.Offset,
			Bytes: values, Option: item.Option, Hash: item.Hash, Why: item.Why})
	}
	return edits, nil
}

// parseEditBytes reads bytes written as two hex digits each, separated by
// spaces ("2a 00 ff").
func parseEditBytes(text string) ([]byte, error) {
	fields := strings.Fields(text)
	values := make([]byte, len(fields))
	for index, field := range fields {
		value, err := strconv.ParseUint(field, 16, 8)
		if err != nil || len(field) != 2 {
			return nil, fmt.Errorf("bytes %q: write each byte as two hex digits", text)
		}
		values[index] = byte(value)
	}
	return values, nil
}

func editBytesText(values []byte) string {
	fields := make([]string, len(values))
	for index, value := range values {
		fields[index] = fmt.Sprintf("%02x", value)
	}
	return strings.Join(fields, " ")
}

// sortEdits orders a module's edits by module and address first, then a
// file's by path and offset.
func sortEdits(edits []dataEdit) {
	sort.SliceStable(edits, func(i, j int) bool {
		a, b := edits[i], edits[j]
		switch {
		case (a.Module != "") != (b.Module != ""):
			return a.Module != ""
		case a.Module != b.Module:
			return a.Module < b.Module
		case a.File != b.File:
			return a.File < b.File
		}
		return a.start() < b.start()
	})
}

// renderEdits writes the canonical form, one flow mapping per line in
// sortEdits order, the bytes always quoted.
func renderEdits(edits *editsFile) []byte {
	sortEdits(edits.Edits)
	var out bytes.Buffer
	if edits.Comment != "" {
		out.WriteString(edits.Comment + "\n")
	}
	if len(edits.Edits) == 0 {
		out.WriteString("edits: []\n")
		return out.Bytes()
	}
	out.WriteString("edits:\n")
	for _, e := range edits.Edits {
		place := fmt.Sprintf("file: %s, offset: 0x%x", yamlText(e.File), e.Offset)
		if e.Module != "" {
			place = fmt.Sprintf("module: %s, addr: 0x%08x", yamlText(e.Module), e.Addr)
		}
		fields := []string{place, "bytes: " + strconv.Quote(editBytesText(e.Bytes)), "option: " + yamlText(e.Option)}
		if e.Hash != "" {
			fields = append(fields, "hash: "+yamlText(e.Hash))
		}
		fields = append(fields, "why: "+yamlText(e.Why))
		fmt.Fprintf(&out, "  - {%s}\n", strings.Join(fields, ", "))
	}
	return out.Bytes()
}

// editableRegionKinds are the regions of data an edit may change; the other
// kinds are code.
var editableRegionKinds = map[string]bool{"data": true, "padding": true}

// validateEdits checks each edit without the disc: its place, that it touches
// no code and no word the build writes itself, that no whole word it writes
// reads as a RAM address, that no two overlap, and its option, hash and why.
func validateEdits(config *projectConfig, fail func(string, ...any)) {
	if config.Edits == nil {
		return
	}
	declared, built := map[string]bool{}, map[string]bool{}
	if config.Editions != nil {
		for _, option := range config.Editions.Options {
			declared[option.Name] = true
		}
		for _, edition := range config.Editions.Editions {
			for _, option := range edition.Options {
				built[option] = true
			}
		}
	}
	edits := slices.Clone(config.Edits.Edits)
	sortEdits(edits)
	for index, e := range edits {
		where := editsPath + ": the edit at " + e.where()
		switch {
		case len(e.Bytes) == 0:
			fail("%s writes no bytes", where)
		case !declared[e.Option]:
			fail("%s names %q, which is no option of %s", where, e.Option, editionsPath)
		case !built[e.Option]:
			fail("%s names %s, which no edition of %s builds", where, e.Option, editionsPath)
		case !validHash(e.Hash):
			fail("%s has no valid hash of the bytes it replaces; run `make checksums`", where)
		case strings.TrimSpace(e.Why) == "":
			fail("%s does not say why", where)
		}
		if index > 0 {
			previous := edits[index-1]
			if previous.Module == e.Module && previous.File == e.File && e.start() < previous.end() {
				fail("%s overlaps the edit at %s", where, previous.where())
			}
		}
		switch {
		case (e.Module != "") == (e.File != ""):
			fail("%s names a module and an addr, or a file and an offset", where)
		case e.Module != "":
			validateModuleEdit(config, e, where, fail)
		default:
			validateFileEdit(config, e, where, fail)
		}
	}
}

func validateModuleEdit(config *projectConfig, e dataEdit, where string, fail func(string, ...any)) {
	m := config.byID[e.Module]
	if m == nil {
		fail("%s names no module of %s", where, modulesConfigDir)
		return
	}
	overlaps := func(addr, end uint32) bool { return e.Addr < end && addr < e.end() }
	switch {
	case e.Offset != 0:
		fail("%s gives an offset; a module's edit gives its addr", where)
	case !m.contains(e.Addr, len(e.Bytes)):
		fail("%s lies past the bytes of %s; an edit replaces bytes in place, and a table that grows is not an edit yet", where, m.File)
	case m.ID == mainModuleID && overlaps(m.Load, m.Load+psxEXEHeaderSize):
		fail("%s lies in MAIN's PS-X EXE header, which the link writes", where)
	}
	for _, f := range m.Functions {
		if overlaps(f.Addr, f.Addr+uint32(f.Size)) {
			fail("%s lies in %s's code; code changes in its C under #ifdef", where, f.Name)
		}
		for _, rodata := range f.Rodata {
			if overlaps(rodata.Addr, rodata.Addr+uint32(rodata.Size)) {
				fail("%s lies in %s's .rodata, which its C compiles", where, f.Name)
			}
		}
	}
	for _, region := range m.Regions {
		if !editableRegionKinds[region.Kind] && overlaps(region.Addr, region.End) {
			fail("%s lies in the %s region at 0x%08x, which is code", where, region.Kind, region.Addr)
		}
	}
	for _, relocation := range m.Relocations {
		if overlaps(relocation.Addr, relocation.Addr+4) {
			fail("%s covers the relocation 0x%08x (to %s), which the build writes from its symbol", where, relocation.Addr, relocation.To)
		}
	}
	for _, word := range m.DiscWords {
		if overlaps(word.Addr, word.Addr+word.size()) {
			fail("%s covers the disc word 0x%08x (%s of %s), which the build writes from the disc layout", where, word.Addr, word.Kind, word.File)
		}
	}
	for _, word := range m.RAMWords {
		if overlaps(word.Addr, word.Addr+4) {
			fail("%s covers the ram word 0x%08x, which the build writes from the RAM map", where, word.Addr)
		}
	}
	for addr := (e.Addr + 3) &^ 3; addr+4 <= e.end(); addr += 4 {
		if value := binary.LittleEndian.Uint32(e.Bytes[addr-e.Addr:]); relocScopeLow <= value && value < relocScopeHigh {
			fail("%s writes 0x%08x at 0x%08x, which reads as a RAM address; a pointer is a relocations row", where, value, addr)
		}
	}
}

func validateFileEdit(config *projectConfig, e dataEdit, where string, fail func(string, ...any)) {
	fact, ok := config.discFiles[e.File]
	if !ok {
		fail("%s names no file of %s", where, discLayoutPath)
		return
	}
	if m := discModuleFiles(config)[e.File]; m != nil {
		fail("%s edits module %s's file; give the module and an addr", where, m.ID)
		return
	}
	for _, entry := range config.Disc.Entries {
		if entry.File == e.File && entry.Type != "" {
			fail("%s edits a %s file, whose sectors an edit cannot rewrite yet", where, entry.Type)
			return
		}
	}
	switch {
	case e.Addr != 0:
		fail("%s gives an addr; a file's edit gives its offset", where)
	case uint64(e.end()) > uint64(fact.size):
		fail("%s runs past the end of %s (%d bytes); an edit replaces bytes in place", where, e.File, fact.size)
	}
}

// editionEdits selects the edits of p's edition's options, by module and by
// file. With the BIN it checks each against the original: the bytes it
// replaces must have its hash and differ from its own, and the edited image
// may give reloc-check no finding the original does not give (a word that
// reads as a RAM address, a disc sector or a split address).
func (p project) editionEdits(config *projectConfig, resolver symbolResolver) (project, error) {
	p.editionModuleEdits, p.editionFileEdits = map[string][]dataEdit{}, map[string][]dataEdit{}
	if config.Edits == nil {
		return p, nil
	}
	count, size := 0, 0
	for _, e := range config.Edits.Edits {
		if !slices.Contains(p.options, e.Option) {
			continue
		}
		count++
		size += len(e.Bytes)
		if e.Module != "" {
			p.editionModuleEdits[e.Module] = append(p.editionModuleEdits[e.Module], e)
		} else {
			p.editionFileEdits[e.File] = append(p.editionFileEdits[e.File], e)
		}
	}
	if count == 0 {
		return p, nil
	}
	checked := "without the BIN, only what target/ shows"
	if p.validExtraction() {
		checked = "each against the original bytes"
		var problems []string
		check := func(e dataEdit, replaced []byte) {
			switch {
			case shortHash(replaced) != e.Hash:
				problems = append(problems, fmt.Sprintf("the edit at %s replaces bytes whose hash is %s, not %s; it was written for other bytes", e.where(), shortHash(replaced), e.Hash))
			case bytes.Equal(replaced, e.Bytes):
				problems = append(problems, fmt.Sprintf("the edit at %s leaves every byte as the original has it", e.where()))
			}
		}
		names := newRelocNames(config)
		for _, m := range config.Modules {
			edits := p.editionModuleEdits[m.ID]
			if len(edits) == 0 {
				continue
			}
			original, err := p.readModuleImage(m)
			if err != nil {
				return p, err
			}
			edited := slices.Clone(original)
			for _, e := range edits {
				check(e, original[m.offset(e.Addr):m.offset(e.end())])
				copy(edited[m.offset(e.Addr):], e.Bytes)
			}
			layout, err := moduleLinkLayout(m)
			if err != nil {
				return p, err
			}
			before, err := relocClassify(resolver, names, m, layout, original)
			if err != nil {
				return p, err
			}
			after, err := relocClassify(resolver, names, m, layout, edited)
			if err != nil {
				return p, err
			}
			seen := map[relocFinding]bool{}
			for _, finding := range before.findings {
				seen[finding] = true
			}
			for _, finding := range after.findings {
				if !seen[finding] {
					problems = append(problems, fmt.Sprintf("the edits of %s make the word 0x%08x read as %s (0x%08x, %s), as make reloc-check reads it; a pointer is a relocations row",
						m.ID, finding.addr, editFindingKinds[finding.kind], finding.value, finding.what))
				}
			}
		}
		for _, path := range slices.Sorted(maps.Keys(p.editionFileEdits)) {
			original, err := os.ReadFile(filepath.Join(p.filesRoot, filepath.FromSlash(path)))
			if err != nil {
				return p, errNotExtracted
			}
			for _, e := range p.editionFileEdits[path] {
				check(e, original[e.Offset:e.end()])
			}
		}
		if len(problems) > 0 {
			return p, fmt.Errorf("edition %s's data edits (%s):\n  %s", p.edition, editsPath, strings.Join(problems, "\n  "))
		}
	}
	fmt.Fprintf(p.stdout(), "edits: %d of edition %s (%d bytes in %d modules and %d files), %s\n",
		count, p.edition, size, len(p.editionModuleEdits), len(p.editionFileEdits), checked)
	return p, nil
}

// editFindingKinds say what a reloc-check finding at an edited word reads as.
var editFindingKinds = map[string]string{
	"pending": "a RAM address", "orphan": "an address inside its own file",
	"sector": "a disc sector", "split": "the high half of a split RAM address",
}

// writeEditedFiles writes each file p's edition edits, the extracted copy with
// its edits, under build/<edition>/files, and returns the copies by disc path.
func (p project) writeEditedFiles() (map[string]string, error) {
	written := map[string]string{}
	for path, edits := range p.editionFileEdits {
		data, err := os.ReadFile(filepath.Join(p.filesRoot, filepath.FromSlash(path)))
		if err != nil {
			return nil, errNotExtracted
		}
		for _, e := range edits {
			copy(data[e.Offset:], e.Bytes)
		}
		output := p.outputDir("files", filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
			return nil, err
		}
		if err := atomicWrite(output, data); err != nil {
			return nil, err
		}
		written[path] = output
	}
	return written, nil
}

// checkEditedFiles checks each file p's edition edits on the disc image at
// output, at its sector in config's layout.
func (p project) checkEditedFiles(config *projectConfig, output string) error {
	for _, path := range slices.Sorted(maps.Keys(p.editedFiles)) {
		want, err := os.ReadFile(p.editedFiles[path])
		if err != nil {
			return err
		}
		lba := config.discFiles[path].lba
		got, err := readMode2File(output, lba, len(want))
		if err != nil {
			return err
		}
		if offset, differs := mismatch(want, got); differs {
			return fmt.Errorf("rebuilt disc %s mismatch at file offset 0x%x", path, offset)
		}
		fmt.Printf("disc file edited: %s at LBA %d (%d bytes)\n", path, lba, len(got))
	}
	return nil
}

// editHashes sets each data edit's hash from the original bytes it replaces
// (make checksums) and counts the hashes it changed.
func (p project) editHashes(config *projectConfig) (changed, total int, err error) {
	if config.Edits == nil {
		return 0, 0, nil
	}
	for index := range config.Edits.Edits {
		e := &config.Edits.Edits[index]
		var original []byte
		start := int(e.Offset)
		if m := config.byID[e.Module]; m != nil {
			if original, err = p.readModuleImage(m); err != nil {
				return changed, total, err
			}
			if !m.contains(e.Addr, len(e.Bytes)) {
				return changed, total, fmt.Errorf("%s: the edit at %s lies outside %s", editsPath, e.where(), m.File)
			}
			start = m.offset(e.Addr)
		} else {
			if _, ok := config.discFiles[e.File]; !ok {
				return changed, total, fmt.Errorf("%s: the edit at %s names no module or disc file", editsPath, e.where())
			}
			if original, err = os.ReadFile(filepath.Join(p.filesRoot, filepath.FromSlash(e.File))); err != nil {
				return changed, total, errNotExtracted
			}
			if int(e.end()) > len(original) {
				return changed, total, fmt.Errorf("%s: the edit at %s runs past the end of %s", editsPath, e.where(), e.File)
			}
		}
		total++
		if value := shortHash(original[start : start+len(e.Bytes)]); e.Hash != value {
			e.Hash = value
			changed++
		}
	}
	return changed, total, nil
}

// linkByteList spells values for a .byte directive.
func linkByteList(values []byte) string {
	fields := make([]string, len(values))
	for index, value := range values {
		fields[index] = fmt.Sprintf("0x%02x", value)
	}
	return strings.Join(fields, ", ")
}
