// disc_layout.go loads, checks and canonically writes target/disc.yaml, the
// disc's ISO 9660 file system without its bytes, writes the mkpsxiso project
// that builds the disc from it with every entry forced to the sector the
// layout gives it, and generates include/fft/disc.h, the sectors and lengths
// the code compiles in.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const discLayoutPath = modulesConfigDir + "/disc.yaml"

// A data file fills 2048-byte Mode 2 Form 1 sectors; a mixed file (an STR
// movie or XA audio) carries 2336 bytes a sector, subheaders included.
const (
	discDataSectorBytes  = 2048
	discMixedSectorBytes = 2336
)

// discLayout is target/disc.yaml: the volume, mkpsxiso's default attributes,
// the root directory and every entry in sector order.
type discLayout struct {
	Comment    string
	License    string // extracted file holding the 16 system-area sectors
	Volume     discVolume
	Attributes discAttributes
	Root       discRoot
	Entries    []discEntry
}

type discVolume struct {
	System           string `yaml:"system"`
	Application      string `yaml:"application"`
	Volume           string `yaml:"volume"`
	VolumeSet        string `yaml:"volume_set"`
	Publisher        string `yaml:"publisher"`
	DataPreparer     string `yaml:"data_preparer"`
	CreationDate     string `yaml:"creation_date"`
	ModificationDate string `yaml:"modification_date"`
}

// discAttributes are the attributes of every entry that does not set its own.
type discAttributes struct {
	GMTOffset int `yaml:"gmt_offs"`
	XAAttrib  int `yaml:"xa_attrib"`
	XAPerm    int `yaml:"xa_perm"`
	XAGID     int `yaml:"xa_gid"`
	XAUID     int `yaml:"xa_uid"`
	Hidden    int `yaml:"hidden"`
}

// discRoot is the root directory record; every directory has its XA
// attributes.
type discRoot struct {
	LBA    int `yaml:"lba"`
	XAPerm int `yaml:"xa_perm"`
	XAGID  int `yaml:"xa_gid"`
	XAUID  int `yaml:"xa_uid"`
	Hidden int `yaml:"hidden"`
}

// discEntry is one row starting at sector LBA: a directory record (Dir), a
// file (File) or a dummy run of Dummy empty sectors in Form.
type discEntry struct {
	LBA   int    `yaml:"lba"`
	Dir   string `yaml:"dir"`
	File  string `yaml:"file"`
	Size  *int   `yaml:"size"` // nil for a module's file, sized by its module
	Type  string `yaml:"type"` // "" for data, or "mixed"
	Date  string `yaml:"date"`
	Fixed string `yaml:"fixed"` // why a directory's files stay at their sectors
	Dummy int    `yaml:"dummy"`
	Form  int    `yaml:"form"`
	Free  bool   `yaml:"free"` // the run a file that outgrows its place moves to
}

type discLayoutYAML struct {
	License    string         `yaml:"license"`
	Volume     discVolume     `yaml:"volume"`
	Attributes discAttributes `yaml:"attributes"`
	Root       discRoot       `yaml:"root"`
	Entries    []discEntry    `yaml:"entries"`
}

var discDatePattern = regexp.MustCompile(`^[0-9]{14}$`)

func parseDiscLayout(data []byte) (*discLayout, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var file discLayoutYAML
	if err := decoder.Decode(&file); err != nil {
		return nil, err
	}
	return &discLayout{
		Comment: leadingComment(data), License: file.License, Volume: file.Volume,
		Attributes: file.Attributes, Root: file.Root, Entries: file.Entries,
	}, nil
}

// renderDiscLayout writes the canonical form: fixed key order and one flow
// mapping per entry, in sector order.
func renderDiscLayout(layout *discLayout) []byte {
	sort.SliceStable(layout.Entries, func(i, j int) bool { return layout.Entries[i].LBA < layout.Entries[j].LBA })
	var out bytes.Buffer
	if layout.Comment != "" {
		out.WriteString(layout.Comment + "\n")
	}
	v, a, r := layout.Volume, layout.Attributes, layout.Root
	fmt.Fprintf(&out, "license: %s\n", yamlText(layout.License))
	fmt.Fprintf(&out, "volume: {system: %s, application: %s, volume: %s, volume_set: %s, publisher: %s, data_preparer: %s, creation_date: %s, modification_date: %s}\n",
		yamlText(v.System), yamlText(v.Application), yamlText(v.Volume), yamlText(v.VolumeSet), yamlText(v.Publisher),
		yamlText(v.DataPreparer), yamlText(v.CreationDate), yamlText(v.ModificationDate))
	fmt.Fprintf(&out, "attributes: {gmt_offs: %d, xa_attrib: %d, xa_perm: %d, xa_gid: %d, xa_uid: %d, hidden: %d}\n",
		a.GMTOffset, a.XAAttrib, a.XAPerm, a.XAGID, a.XAUID, a.Hidden)
	fmt.Fprintf(&out, "root: {lba: %d, xa_perm: %d, xa_gid: %d, xa_uid: %d, hidden: %d}\n", r.LBA, r.XAPerm, r.XAGID, r.XAUID, r.Hidden)
	if len(layout.Entries) > 0 {
		out.WriteString("\nentries:\n")
	}
	// A date reads back into its string field as written, digits only.
	date := func(value string) string {
		if discDatePattern.MatchString(value) {
			return value
		}
		return yamlText(value)
	}
	for _, entry := range layout.Entries {
		fields := []string{fmt.Sprintf("lba: %d", entry.LBA)}
		switch {
		case entry.Dir != "":
			fields = append(fields, "dir: "+yamlText(entry.Dir), "date: "+date(entry.Date))
			if entry.Fixed != "" {
				fields = append(fields, "fixed: "+yamlText(entry.Fixed))
			}
		case entry.File != "":
			fields = append(fields, "file: "+yamlText(entry.File))
			if entry.Size != nil {
				fields = append(fields, fmt.Sprintf("size: %d", *entry.Size))
			}
			if entry.Type != "" {
				fields = append(fields, "type: "+yamlText(entry.Type))
			}
			fields = append(fields, "date: "+date(entry.Date))
		default:
			fields = append(fields, fmt.Sprintf("dummy: %d, form: %d", entry.Dummy, entry.Form))
			if entry.Free {
				fields = append(fields, "free: true")
			}
		}
		fmt.Fprintf(&out, "  - {%s}\n", strings.Join(fields, ", "))
	}
	return out.Bytes()
}

// sectors returns how many sectors a file or dummy row takes, sizing a
// module's file by its module; mkpsxiso gives an empty file one sector.
func (entry *discEntry) sectors(modules map[string]*moduleSpec) int {
	if entry.Dummy > 0 {
		return entry.Dummy
	}
	size := 0
	if entry.Size != nil {
		size = *entry.Size
	} else if m := modules[entry.File]; m != nil {
		size = m.Size
	}
	unit := discDataSectorBytes
	if entry.Type == "mixed" {
		unit = discMixedSectorBytes
	}
	return max(1, (size+unit-1)/unit)
}

// discFileFact is one file of the layout: its first sector, its byte size (a
// module's from its module) and the sectors it takes.
type discFileFact struct {
	lba, size, sectors int
}

// files lists the layout's files by path.
func (layout *discLayout) files(config *projectConfig) map[string]discFileFact {
	modules := discModuleFiles(config)
	files := make(map[string]discFileFact)
	for index := range layout.Entries {
		entry := &layout.Entries[index]
		if entry.File == "" {
			continue
		}
		size := 0
		if entry.Size != nil {
			size = *entry.Size
		} else if m := modules[entry.File]; m != nil {
			size = m.Size
		}
		files[entry.File] = discFileFact{entry.LBA, size, entry.sectors(modules)}
	}
	return files
}

// discWordValues returns the words a disc row holds: the file's first sector
// plus the row's offset, and its size rounded up to whole 2048-byte sectors
// (an empty file's is 0). It returns nil for a file the layout lacks.
func (c *projectConfig) discWordValues(word discWordSpec) []uint32 {
	file, ok := c.discFiles[word.File]
	if !ok {
		return nil
	}
	sector := uint32(file.lba + word.Offset)
	size := uint32((file.size + discDataSectorBytes - 1) / discDataSectorBytes * discDataSectorBytes)
	switch word.Kind {
	case "sector":
		return []uint32{sector}
	case "size":
		return []uint32{size}
	case "extent":
		return []uint32{sector, size}
	}
	return nil
}

// discModuleFiles maps each module's disc path to its module.
func discModuleFiles(config *projectConfig) map[string]*moduleSpec {
	files := make(map[string]*moduleSpec, len(config.Modules))
	for _, m := range config.Modules {
		files[m.File] = m
	}
	return files
}

// validateDiscLayout checks the layout without the disc: each row's shape,
// that every file sits in a declared directory, that a file or dummy ends
// where the next row starts (only a directory record's length is mkpsxiso's
// to compute), that each module's file has exactly one row, that one dummy
// at most is free and that every file fits.
func validateDiscLayout(layout *discLayout, config *projectConfig, fail func(string, ...any)) {
	label := discLayoutPath
	if layout.License == "" || strings.Contains(layout.License, "/") {
		fail("%s: license must name the extracted system-area file", label)
	}
	if layout.Volume.System == "" || layout.Volume.Volume == "" {
		fail("%s: volume needs a system and a volume identifier", label)
	}
	modules := discModuleFiles(config)
	dirs := map[string]bool{"": true}
	files := make(map[string]bool)
	free := 0
	for index, entry := range layout.Entries {
		row := fmt.Sprintf("%s: entry at lba %d", label, entry.LBA)
		kinds := 0
		for _, set := range []bool{entry.Dir != "", entry.File != "", entry.Dummy != 0} {
			if set {
				kinds++
			}
		}
		if kinds != 1 {
			fail("%s must be exactly one of dir, file or dummy", row)
			continue
		}
		if index == 0 && entry.LBA <= layout.Root.LBA || index > 0 && entry.LBA <= layout.Entries[index-1].LBA {
			fail("%s is out of sector order", row)
		}
		switch {
		case entry.Dir != "":
			if !discDatePattern.MatchString(entry.Date) || entry.Size != nil || entry.Type != "" || entry.Form != 0 || entry.Free {
				fail("%s: a directory takes only a date (YYYYMMDDHHMMSS) and fixed", row)
			}
			if path.Clean(entry.Dir) != entry.Dir || !dirs[parentDiscPath(entry.Dir)] || dirs[entry.Dir] {
				fail("%s: directory %q is not new in a declared directory", row, entry.Dir)
			}
			dirs[entry.Dir] = true
		case entry.File != "":
			if !discDatePattern.MatchString(entry.Date) || entry.Fixed != "" || entry.Form != 0 || entry.Free {
				fail("%s: a file takes a size, type and date (YYYYMMDDHHMMSS)", row)
			}
			if path.Clean(entry.File) != entry.File || !dirs[parentDiscPath(entry.File)] || files[entry.File] {
				fail("%s: file %q is not new in a declared directory", row, entry.File)
			}
			files[entry.File] = true
			if entry.Type != "" && entry.Type != "mixed" {
				fail("%s: type must be mixed or absent (data)", row)
			}
			_, module := modules[entry.File]
			switch {
			case module && entry.Size != nil:
				fail("%s: %s is a module's file; its module gives the size", row, entry.File)
			case !module && entry.Size == nil:
				fail("%s: %s needs a size", row, entry.File)
			case entry.Size != nil && (*entry.Size < 0 || entry.Type == "mixed" && *entry.Size%discMixedSectorBytes != 0):
				fail("%s: invalid size %d", row, *entry.Size)
			}
		default:
			if entry.Dummy < 0 || entry.Form != 1 && entry.Form != 2 || entry.Date != "" || entry.Size != nil || entry.Type != "" || entry.Fixed != "" {
				fail("%s: a dummy takes a sector count and form 1 or 2", row)
			}
			if entry.Free {
				free++
			}
		}
		if entry.Dir == "" && index+1 < len(layout.Entries) {
			if end, next := entry.LBA+entry.sectors(modules), layout.Entries[index+1].LBA; end != next {
				fail("%s ends at sector %d, but the next entry starts at %d", row, end, next)
			}
		}
	}
	if free > 1 {
		fail("%s: %d dummies are free; one run takes the files that move", label, free)
	}
	for _, m := range config.Modules {
		if !files[m.File] {
			fail("%s: module %s's file %s has no entry", label, m.ID, m.File)
		}
	}
	_, problems := layout.fitted(config)
	for _, problem := range problems {
		fail("%s: %s", label, problem)
	}
}

// fitted returns the layout `make config-fmt` writes, and why a file cannot
// fit. A file that outgrows the sectors up to the next directory or file
// moves to the start of the free run, unless its directory is fixed. Every
// dummy is then the gap between two rows, in the form of the dummy that held
// the gap's first sector (form 2 where a file was), and the gap that ends the
// free run stays free. A directory record keeps the sectors up to the row
// after it, as mkpsxiso wrote it.
func (layout *discLayout) fitted(config *projectConfig) (*discLayout, []string) {
	modules := discModuleFiles(config)
	type span struct {
		entry discEntry
		end   int // the sector after it
	}
	var rows []span
	var dummies []discEntry
	fixed := make(map[string]bool)
	entries := append([]discEntry(nil), layout.Entries...)
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].LBA < entries[j].LBA })
	start, discEnd, free := layout.Root.LBA+1, layout.Root.LBA+1, -1
	for index, entry := range entries {
		if index == 0 {
			start = entry.LBA
		}
		end := entry.LBA + entry.sectors(modules)
		switch {
		case entry.Dummy != 0:
			dummies = append(dummies, entry)
			if entry.Free {
				free = end
			}
		case entry.Dir != "":
			end = entry.LBA + 1
			if index+1 < len(entries) {
				end = entries[index+1].LBA
			}
			if entry.Fixed != "" {
				fixed[entry.Dir] = true
			}
		}
		discEnd = max(discEnd, end)
		if entry.Dummy == 0 {
			rows = append(rows, span{entry, end})
		}
	}
	name := func(entry discEntry) string {
		if entry.File != "" {
			return entry.File
		}
		return entry.Dir + "/"
	}
	var problems []string
	var placed, moving []span
	for index, row := range rows {
		if row.entry.File != "" && index+1 < len(rows) && row.end > rows[index+1].entry.LBA {
			next := rows[index+1].entry
			if !fixed[parentDiscPath(row.entry.File)] {
				moving = append(moving, row)
				continue
			}
			problems = append(problems, fmt.Sprintf("%s needs the sectors up to %d, but %s starts at %d and its directory is fixed",
				row.entry.File, row.end, name(next), next.LBA))
		}
		placed = append(placed, row)
	}
	if len(moving) > 0 && free < 0 {
		problems = append(problems, fmt.Sprintf("%s outgrows its place, and no dummy is free", moving[0].entry.File))
	} else if len(moving) > 0 {
		cursor := 0
		for _, row := range placed {
			if row.entry.LBA < free {
				cursor = max(cursor, row.end)
			}
		}
		for _, row := range moving {
			row.end, row.entry.LBA = cursor+row.end-row.entry.LBA, cursor
			if cursor = row.end; cursor > free {
				problems = append(problems, fmt.Sprintf("%s needs the sectors up to %d, past the free run's end at %d", row.entry.File, cursor, free))
			}
			placed = append(placed, row)
		}
		sort.SliceStable(placed, func(i, j int) bool { return placed[i].entry.LBA < placed[j].entry.LBA })
	}
	formAt := func(from, to int) int {
		for _, dummy := range dummies {
			if dummy.LBA <= from && from < dummy.LBA+dummy.Dummy {
				return dummy.Form
			}
		}
		for _, dummy := range dummies {
			if dummy.LBA < to && from < dummy.LBA+dummy.Dummy {
				return dummy.Form
			}
		}
		return 2
	}
	result := *layout
	result.Entries = nil
	gap := func(from, to int) {
		if from < to {
			result.Entries = append(result.Entries, discEntry{LBA: from, Dummy: to - from, Form: formAt(from, to), Free: from < free && free <= to})
		}
	}
	at := start
	for _, row := range placed {
		gap(at, row.entry.LBA)
		result.Entries = append(result.Entries, row.entry)
		at = max(at, row.end)
	}
	gap(at, discEnd)
	return &result, problems
}

// discHeaderPath is the header of the layout's facts the code compiles in.
const discHeaderPath = "include/fft/disc.h"

const (
	discSectorPrefix = "DISC_SECTOR_"
	discCountPrefix  = "DISC_SECTOR_COUNT_"
	discSectorBytes  = "DISC_SECTOR_BYTES"
)

// discName spells a disc path the way the header's names spell it:
// EVENT/TEST.EVT is EVENT_TEST_EVT.
func discName(file string) string {
	return strings.ToUpper(strings.NewReplacer("/", "_", ".", "_").Replace(file))
}

// discCodeNames lists the DISC_SECTOR_ names the C sources and headers use in
// code or directives, apart from the generated header itself.
func discCodeNames(root string) (map[string]bool, error) {
	names, err := codeNamesWithPrefix(root, discSectorPrefix, discHeaderPath)
	delete(names, discSectorBytes)
	return names, err
}

// codeNamesWithPrefix lists the names starting with prefix that the C sources
// and headers use in code or directives, apart from the generated header that
// defines them.
func codeNamesWithPrefix(root, prefix, header string) (map[string]bool, error) {
	names := make(map[string]bool)
	for _, dir := range declarationScanDirs {
		err := filepath.WalkDir(filepath.Join(root, dir), func(file string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			extension := filepath.Ext(file)
			if entry.IsDir() || extension != ".c" && extension != ".h" {
				return nil
			}
			relative, err := filepath.Rel(root, file)
			if err != nil || filepath.ToSlash(relative) == header {
				return err
			}
			data, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			tokens, err := tokenizeC(string(data))
			if err != nil {
				return fmt.Errorf("%s: %w", filepath.ToSlash(relative), err)
			}
			for _, directive := range cDirectives(string(data)) {
				more, err := tokenizeC(strings.TrimPrefix(strings.TrimSpace(directive.text), "#"))
				if err != nil {
					return fmt.Errorf("%s: %w", filepath.ToSlash(relative), err)
				}
				tokens = append(tokens, more...)
			}
			for _, token := range tokens {
				if token.kind == cIdent && strings.HasPrefix(token.text, prefix) {
					names[token.text] = true
				}
			}
			return nil
		})
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	return names, nil
}

// discHeader writes the header for the names the code uses: DISC_SECTOR_<FILE>
// is the file's first sector and DISC_SECTOR_COUNT_<FILE> its length in
// sectors, each list in sector order. It returns nil when the code uses no
// name, and the names that spell no file.
func discHeader(layout *discLayout, config *projectConfig, names map[string]bool) ([]byte, []string) {
	if len(names) == 0 {
		return nil, nil
	}
	modules := discModuleFiles(config)
	var sectors, counts []string
	known := make(map[string]bool)
	for index := range layout.Entries {
		entry := &layout.Entries[index]
		if entry.File == "" {
			continue
		}
		name := discName(entry.File)
		if names[discSectorPrefix+name] {
			sectors = append(sectors, fmt.Sprintf("    %s%s = %d,\n", discSectorPrefix, name, entry.LBA))
			known[discSectorPrefix+name] = true
		}
		if names[discCountPrefix+name] {
			counts = append(counts, fmt.Sprintf("    %s%s = %d,\n", discCountPrefix, name, entry.sectors(modules)))
			known[discCountPrefix+name] = true
		}
	}
	var unknown []string
	for name := range names {
		if !known[name] {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(unknown)
	var out bytes.Buffer
	out.WriteString(`#ifndef DISC_H
#define DISC_H

/* Where the code finds disc files, generated from target/disc.yaml by ` + "`make config-fmt`" + `: edit the layout,
 * not this file. */

/* disc */
/* The first sector of each file the code reads at a fixed sector and the length in sectors of each file
 * it reads or streams whole, named as the disc directory names the file. The other files are found
 * through sector tables in the data. */

/* Bytes of data in a Mode 2 Form 1 sector. */
#define DISC_SECTOR_BYTES 2048
`)
	if len(sectors) > 0 {
		out.WriteString("\ntypedef enum disc_sector {\n" + strings.Join(sectors, "") + "} disc_sector_e;\n")
	}
	if len(counts) > 0 {
		out.WriteString("\ntypedef enum disc_sector_count {\n" + strings.Join(counts, "") + "} disc_sector_count_e;\n")
	}
	out.WriteString("\n#endif\n")
	return out.Bytes(), unknown
}

// validateDiscHeader fails when the header differs from what the layout gives
// for the names the code uses, or when a name spells no file.
func (p project) validateDiscHeader(config *projectConfig, fail func(string, ...any)) error {
	names, err := discCodeNames(p.root)
	if err != nil {
		return err
	}
	want, unknown := discHeader(config.Disc, config, names)
	for _, name := range unknown {
		fail("%s: %s names no file in %s", discHeaderPath, name, discLayoutPath)
	}
	if want == nil {
		return nil
	}
	if got, err := os.ReadFile(filepath.Join(p.root, filepath.FromSlash(discHeaderPath))); err != nil || !bytes.Equal(got, want) {
		fail("%s is not what %s gives; run `make config-fmt`", discHeaderPath, discLayoutPath)
	}
	return nil
}

// writeDiscHeader writes the header when it differs, and reports whether it
// did.
func (p project) writeDiscHeader(config *projectConfig) (bool, error) {
	if config.Disc == nil {
		return false, nil
	}
	names, err := discCodeNames(p.root)
	if err != nil {
		return false, err
	}
	want, _ := discHeader(config.Disc, config, names)
	full := filepath.Join(p.root, filepath.FromSlash(discHeaderPath))
	if got, err := os.ReadFile(full); want == nil || err == nil && bytes.Equal(got, want) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return false, err
	}
	return true, atomicWrite(full, want)
}

func parentDiscPath(name string) string {
	if index := strings.LastIndexByte(name, '/'); index >= 0 {
		return name[:index]
	}
	return ""
}

// mkpsxisoProject writes the mkpsxiso project of the layout. A file's source
// is its rebuilt module in built (module id to path), its edited copy in
// edited (disc path to path) or its extracted copy under filesRoot. A dummy goes in the directory of the row before it, as
// dumpsxiso writes it.
func (layout *discLayout) mkpsxisoProject(config *projectConfig, filesRoot string, built, edited map[string]string) []byte {
	type node struct {
		entry    *discEntry
		children []*node
	}
	root := &node{}
	dirs := map[string]*node{"": root}
	current := root
	for index := range layout.Entries {
		entry := &layout.Entries[index]
		item := &node{entry: entry}
		switch {
		case entry.Dir != "":
			dirs[parentDiscPath(entry.Dir)].children = append(dirs[parentDiscPath(entry.Dir)].children, item)
			dirs[entry.Dir] = item
			current = item
		case entry.File != "":
			current = dirs[parentDiscPath(entry.File)]
			current.children = append(current.children, item)
		default:
			current.children = append(current.children, item)
		}
	}
	modules := discModuleFiles(config)
	source := func(name string) string {
		if m := modules[name]; m != nil && built[m.ID] != "" {
			return xmlAttribute(built[m.ID])
		}
		if edited[name] != "" {
			return xmlAttribute(edited[name])
		}
		return xmlAttribute(filesRoot + "/" + name)
	}
	var out bytes.Buffer
	v, a, r := layout.Volume, layout.Attributes, layout.Root
	out.WriteString("<iso_project image_name=\"mkpsxiso.bin\" cue_sheet=\"mkpsxiso.cue\">\n    <track type=\"data\" cdvd_style=\"false\">\n")
	fmt.Fprintf(&out, "        <identifiers system=\"%s\" application=\"%s\" volume=\"%s\" volume_set=\"%s\" publisher=\"%s\" data_preparer=\"%s\" creation_date=\"%s\" modification_date=\"%s\"/>\n",
		xmlAttribute(v.System), xmlAttribute(v.Application), xmlAttribute(v.Volume), xmlAttribute(v.VolumeSet), xmlAttribute(v.Publisher),
		xmlAttribute(v.DataPreparer), xmlAttribute(v.CreationDate), xmlAttribute(v.ModificationDate))
	fmt.Fprintf(&out, "        <license file=\"%s\"/>\n", source(layout.License))
	fmt.Fprintf(&out, "        <default_attributes gmt_offs=\"%d\" xa_attrib=\"%d\" xa_perm=\"%d\" xa_gid=\"%d\" xa_uid=\"%d\" hidden=\"%d\"/>\n",
		a.GMTOffset, a.XAAttrib, a.XAPerm, a.XAGID, a.XAUID, a.Hidden)
	fmt.Fprintf(&out, "        <directory_tree xa_perm=\"%d\" xa_gid=\"%d\" xa_uid=\"%d\" hidden=\"%d\" offs=\"%d\">\n", r.XAPerm, r.XAGID, r.XAUID, r.Hidden, r.LBA)
	var write func(children []*node, depth int)
	write = func(children []*node, depth int) {
		indent := strings.Repeat("    ", depth)
		for _, child := range children {
			entry := child.entry
			switch {
			case entry.Dir != "":
				fmt.Fprintf(&out, "%s<dir name=\"%s\" source=\"%s\" date=\"%s\" xa_perm=\"%d\" xa_gid=\"%d\" xa_uid=\"%d\" offs=\"%d\">\n",
					indent, xmlAttribute(path.Base(entry.Dir)), source(entry.Dir), entry.Date, r.XAPerm, r.XAGID, r.XAUID, entry.LBA)
				write(child.children, depth+1)
				fmt.Fprintf(&out, "%s</dir>\n", indent)
			case entry.File != "":
				kind := "data"
				if entry.Type != "" {
					kind = entry.Type
				}
				fmt.Fprintf(&out, "%s<file name=\"%s\" source=\"%s\" type=\"%s\" date=\"%s\" offs=\"%d\"/>\n",
					indent, xmlAttribute(path.Base(entry.File)), source(entry.File), kind, entry.Date, entry.LBA)
			default:
				submode := 0
				if entry.Form == 2 {
					submode = 0x20
				}
				fmt.Fprintf(&out, "%s<dummy sectors=\"%d\" type=\"%d\" offs=\"%d\"/>\n", indent, entry.Dummy, submode, entry.LBA)
			}
		}
	}
	write(root.children, 3)
	out.WriteString("        </directory_tree>\n    </track>\n</iso_project>\n")
	return out.Bytes()
}
