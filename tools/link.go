// link.go links each module whole, as the original linker did: every
// function's relocatable object and the original bytes between them, in
// address order, with an ASSERT that each name lands at its original address
// (`make build`). The data bytes come from the extracted disc; without it they
// are zeros of the right size, which still proves the code and the layout.
// When every module links equal to its original file, the disc is built from
// them under build/disc; `make patch-check` builds it again from copies of
// the original images.
package main

import (
	"bytes"
	"crypto/sha256"
	"debug/elf"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// linkPiece is one contiguous range of a module image: a function's .text,
// its .rodata, or the original bytes between them with the data rows inside.
type linkPiece struct {
	addr     uint32
	size     int
	kind     string // "text", "rodata", "bytes" or MAIN's "header"
	function *functionSpec
	labels   []symbolSpec
	aliases  []symbolSpec  // data rows inside a text or rodata piece, defined from its address
	words    []linkWord    // words of a bytes piece the link writes
	delta    uint32        // how far an edition's flowing layout moves it
	edition  *pieceEdition // how that layout fills a piece of changed code
}

// linkWord is a word of the bytes the link writes instead of copying it: a
// relocation's symbol (plus offset), a disc word's number from the layout or
// a RAM window's size or start from the RAM map.
type linkWord struct {
	addr uint32
	expr string
}

// linkLayout is a module's pieces in address order; the .rodata copies that
// lie inside another function's (a literal several functions share, which
// each object carries); and the data rows the link binds to their absolute
// addresses: those past the file, and those inside a function that name
// another module's object (see linkRowIsForeign); and the end of the module's
// RAM window, which the image must not pass (0 for none). On the shifted test
// disc (make build SHIFT=N), insert bytes open at the image's start (after
// MAIN's header) and MAIN ends in padding up to a whole sector. In an
// edition's flowing layout (flowLayout), growth is what the changed code adds
// and MAIN pads the same way.
type linkLayout struct {
	pieces   []linkPiece
	shadows  []linkPiece
	absolute []symbolSpec
	ramEnd   uint32
	insert   uint32
	growth   uint32
	padding  uint32
	edits    []dataEdit // an edition's data edits in the bytes pieces
}

// The resident executable starts with its PS-X EXE header (psxEXEHeaderSize
// bytes); the link writes the fields that follow from the layout (the entry
// point, the text address and size) and keeps the rest of the original header.
const mainEntryPoint = "__SN_ENTRY_POINT"

// moduleLinkLayout splits [load, load+size) into the functions' .text and
// .rodata ranges and the bytes between them; MAIN's header is a piece of its
// own.
func moduleLinkLayout(m *moduleSpec) (linkLayout, error) {
	var pieces []linkPiece
	for index := range m.Functions {
		f := &m.Functions[index]
		pieces = append(pieces, linkPiece{addr: f.Addr, size: f.Size, kind: "text", function: f})
		for _, rodata := range f.Rodata {
			pieces = append(pieces, linkPiece{addr: rodata.Addr, size: rodata.Size, kind: "rodata", function: f})
		}
	}
	// The largest of the ranges that start together comes first, so a shared
	// literal's other copies fall inside it.
	sort.Slice(pieces, func(a, b int) bool {
		if pieces[a].addr != pieces[b].addr {
			return pieces[a].addr < pieces[b].addr
		}
		return pieces[a].size > pieces[b].size
	})
	layout := linkLayout{ramEnd: m.ramEnd}
	cursor, end := m.Load, uint32(m.end())
	for _, piece := range pieces {
		if piece.addr < cursor {
			last := layout.pieces[len(layout.pieces)-1]
			if piece.kind == "rodata" && last.kind == "rodata" && piece.addr+uint32(piece.size) <= last.addr+uint32(last.size) {
				layout.shadows = append(layout.shadows, piece)
				continue
			}
			return layout, fmt.Errorf("%s: %s %s at 0x%08x overlaps the range before it", m.ID, piece.function.Name, piece.kind, piece.addr)
		}
		if piece.addr > cursor {
			layout.pieces = append(layout.pieces, linkPiece{addr: cursor, size: int(piece.addr - cursor), kind: "bytes"})
		}
		layout.pieces = append(layout.pieces, piece)
		cursor = piece.addr + uint32(piece.size)
	}
	if cursor > end {
		return layout, fmt.Errorf("%s: its last range ends at 0x%08x, past the file end 0x%08x", m.ID, cursor, end)
	}
	if cursor < end {
		layout.pieces = append(layout.pieces, linkPiece{addr: cursor, size: int(end - cursor), kind: "bytes"})
	}
	if m.ID == mainModuleID {
		first := layout.pieces[0]
		if first.kind != "bytes" || first.addr != m.Load || first.size < psxEXEHeaderSize {
			return layout, fmt.Errorf("%s: no room for the %d-byte PS-X EXE header before its first function", m.ID, psxEXEHeaderSize)
		}
		header := linkPiece{addr: m.Load, size: psxEXEHeaderSize, kind: "header"}
		if first.size == psxEXEHeaderSize {
			layout.pieces[0] = header
		} else {
			rest := linkPiece{addr: m.Load + psxEXEHeaderSize, size: first.size - psxEXEHeaderSize, kind: "bytes"}
			layout.pieces = append([]linkPiece{header, rest}, layout.pieces[1:]...)
		}
	}
	data := append([]symbolSpec(nil), m.Data...)
	sort.Slice(data, func(a, b int) bool { return data[a].Addr < data[b].Addr })
	for _, symbol := range data {
		index := sort.Search(len(layout.pieces), func(i int) bool {
			return layout.pieces[i].addr+uint32(layout.pieces[i].size) > symbol.Addr
		})
		if index < len(layout.pieces) && layout.pieces[index].addr <= symbol.Addr {
			piece := &layout.pieces[index]
			switch {
			case piece.kind == "bytes":
				piece.labels = append(piece.labels, symbol)
				continue
			case (piece.kind == "text" || piece.kind == "rodata") && !linkRowIsForeign(m, symbol.Name):
				piece.aliases = append(piece.aliases, symbol)
				continue
			}
		}
		layout.absolute = append(layout.absolute, symbol)
	}
	var words []linkWord
	for _, relocation := range m.Relocations {
		expr := relocation.target()
		if relocation.Offset != 0 {
			expr += fmt.Sprintf("+0x%x", relocation.Offset)
		}
		words = append(words, linkWord{relocation.Addr, expr})
	}
	for _, word := range m.DiscWords {
		if len(word.values) == 0 {
			return layout, fmt.Errorf("%s: disc word 0x%08x has no %s in the disc layout", m.ID, word.Addr, word.File)
		}
		for index, value := range word.values {
			words = append(words, linkWord{word.Addr + uint32(4*index), fmt.Sprintf("%d", value)})
		}
	}
	for _, word := range m.RAMWords {
		if !word.found {
			return layout, fmt.Errorf("%s: ram word 0x%08x has no value in the RAM map", m.ID, word.Addr)
		}
		words = append(words, linkWord{word.Addr, fmt.Sprintf("0x%x", word.value)})
	}
	sort.Slice(words, func(a, b int) bool { return words[a].addr < words[b].addr })
	for _, word := range words {
		index := sort.Search(len(layout.pieces), func(i int) bool {
			return layout.pieces[i].addr+uint32(layout.pieces[i].size) > word.addr
		})
		piece := &layout.pieces[min(index, len(layout.pieces)-1)]
		if index == len(layout.pieces) || piece.kind != "bytes" || piece.addr > word.addr || word.addr+4 > piece.addr+uint32(piece.size) {
			return layout, fmt.Errorf("%s: data word 0x%08x is not a word of the bytes between functions", m.ID, word.addr)
		}
		piece.words = append(piece.words, word)
	}
	return layout, nil
}

// A data row inside one of the module's functions or literals names a place
// in that code: a second name for a function's entry, the end of a patch
// template, an immediate the program rewrites, a literal another function
// loads. The link defines it from its piece, so it moves with the code. A row
// whose name carries another module's token (WORLD's names for OPEN's buffers
// in the same RAM) names that module's object at a fixed address instead, and
// stays absolute.
func linkRowIsForeign(m *moduleSpec, name string) bool {
	token, _, _ := strings.Cut(strings.TrimPrefix(name, "g_"), "_")
	own := m.ID
	switch {
	case m.ID == "opening":
		own = "open"
	case strings.HasPrefix(m.ID, "event-"):
		own = strings.TrimPrefix(m.ID, "event-")
	case strings.HasPrefix(m.ID, "effect-"):
		own = "effect"
	}
	return moduleTokens[token] && token != own
}

func linkSectionName(piece linkPiece) string {
	return fmt.Sprintf(".%s_%08x", piece.kind, piece.addr)
}

func linkShadowName(piece linkPiece) string {
	return fmt.Sprintf(".shadow_%08x_%s", piece.addr, piece.function.Name)
}

// linkObjectSections gives, per function, the name its object's .text and
// .rodata take: their piece's section name, so the script matches every piece
// by section name alone. GNU ld 2.40 matches each input statement that names
// a file against every input file, and one such statement per piece made
// BATTLE's link take half a minute instead of a fraction of a second.
func linkObjectSections(layout linkLayout) (map[string]map[string]string, error) {
	sections := map[string]map[string]string{}
	take := func(f *functionSpec, name string, kinds ...string) error {
		names := sections[f.Name]
		if names == nil {
			names = map[string]string{}
			sections[f.Name] = names
		}
		for _, kind := range kinds {
			if other, ok := names[kind]; ok {
				return fmt.Errorf("%s: its %s is both %s and %s", f.Name, kind, other, name)
			}
			names[kind] = name
		}
		return nil
	}
	for _, piece := range layout.pieces {
		var err error
		switch piece.kind {
		case "text":
			err = take(piece.function, linkSectionName(piece), ".text")
			if err == nil && piece.edition != nil && piece.edition.rodata > 0 {
				err = take(piece.function, linkNewRodataName(piece), ".rodata", ".rdata")
			}
		case "rodata":
			err = take(piece.function, linkSectionName(piece), ".rodata", ".rdata")
		}
		if err != nil {
			return nil, err
		}
	}
	for _, piece := range layout.shadows {
		if err := take(piece.function, linkShadowName(piece), ".rodata", ".rdata"); err != nil {
			return nil, err
		}
	}
	return sections, nil
}

// linkObjectRenames lists, per function, the symbols its object takes under
// other names in the module link: the definition of a source compiled for
// several entries takes the row's name, and each override names a symbol the
// module resolves to the override's address. An override at an address no
// name resolves to gets a D_<address> label when the address lies in the
// module's bytes (it then moves with them); anywhere else the D_ name is an
// absolute binding, returned with its address.
func linkObjectRenames(resolver symbolResolver, m *moduleSpec, layout *linkLayout) (map[string]map[string]string, map[string]uint32, error) {
	named := map[uint32][]string{}
	addNames := func(names map[string]uint32, keep func(string, uint32) bool) {
		var sorted []string
		for name, addr := range names {
			if keep(name, addr) {
				sorted = append(sorted, name)
			}
		}
		sort.Strings(sorted)
		for _, name := range sorted {
			named[names[name]] = append(named[names[name]], name)
		}
	}
	addNames(resolver.own[m.ID], func(string, uint32) bool { return true })
	for _, module := range append(append([]string(nil), m.Links...), mainModuleID) {
		addNames(resolver.exported[module], func(name string, addr uint32) bool {
			at, ok := resolver.resolve(m, name)
			return ok && at == addr
		})
	}
	generated := map[uint32]string{}
	absolute := map[string]uint32{}
	targetOf := func(addr uint32) string {
		if names := named[addr]; len(names) > 0 {
			return names[0]
		}
		if name, ok := generated[addr]; ok {
			return name
		}
		name := fmt.Sprintf("D_%08x", addr)
		generated[addr] = name
		for index := range layout.pieces {
			piece := &layout.pieces[index]
			if piece.kind == "bytes" && piece.addr <= addr && addr < piece.addr+uint32(piece.size) {
				piece.labels = append(piece.labels, symbolSpec{Addr: addr, Name: name})
				sort.SliceStable(piece.labels, func(a, b int) bool { return piece.labels[a].Addr < piece.labels[b].Addr })
				return name
			}
		}
		absolute[name] = addr
		return name
	}
	renames := map[string]map[string]string{}
	for index := range m.Functions {
		f := &m.Functions[index]
		names := map[string]string{}
		if symbol := f.sourceSymbol(); symbol != f.Name {
			names[symbol] = f.Name
		}
		renames[f.Name] = names
	}
	for _, override := range m.Overrides {
		names := renames[override.Function]
		if names == nil {
			return nil, nil, fmt.Errorf("%s: override of %s names no function %s", m.ID, override.Name, override.Function)
		}
		if at, ok := resolver.resolve(m, override.Name); ok && at == override.Addr {
			continue
		}
		if _, defined := names[override.Name]; defined {
			// A body that names itself by its shared definition already
			// reaches its own entry through the rename.
			if f := m.function(override.Function); f != nil && f.Addr == override.Addr {
				continue
			}
			return nil, nil, fmt.Errorf("%s: %s overrides %s, the name its own definition takes", m.ID, override.Function, override.Name)
		}
		names[override.Name] = targetOf(override.Addr)
	}
	return renames, absolute, nil
}

// linkDataAssembly writes every bytes piece as its own section: the original
// bytes (`.incbin` of the extracted file) or, without it, zeros, and the bytes
// of each data edit of an edition, with a global label at each data row, each
// relocation's word from its symbol and each disc word from the disc layout.
func linkDataAssembly(m *moduleSpec, layout linkLayout, image string) string {
	var out strings.Builder
	fmt.Fprintf(&out, "# %s: the bytes between functions, in address order.\n", m.ID)
	if layout.insert > 0 {
		fmt.Fprintf(&out, "\t.section .insert, \"a\", @progbits\n\t.rept %d\n\t.word 0x%08x\n\t.endr\n", layout.insert/4, shiftFillerWord)
	}
	if layout.padding > 0 {
		fmt.Fprintf(&out, "\t.section .padding, \"a\", @progbits\n\t.space %d\n", layout.padding)
	}
	// An edition's filler after changed code: a break in each whole word,
	// zeros around them (fillerBytes).
	for _, piece := range layout.pieces {
		if piece.edition == nil || piece.edition.fill == 0 {
			continue
		}
		at, size := piece.addr+uint32(piece.edition.fillAt), piece.edition.fill
		lead := min(int(alignUp(at, 4)-at), size)
		words := (size - lead) / 4
		fmt.Fprintf(&out, "\t.section %s, \"a\", @progbits\n", linkFillName(piece))
		if lead > 0 {
			fmt.Fprintf(&out, "\t.space %d\n", lead)
		}
		if words > 0 {
			fmt.Fprintf(&out, "\t.rept %d\n\t.byte 0x0d, 0x00, 0x00, 0x00\n\t.endr\n", words)
		}
		if rest := size - lead - 4*words; rest > 0 {
			fmt.Fprintf(&out, "\t.space %d\n", rest)
		}
	}
	for _, piece := range layout.pieces {
		if piece.kind != "bytes" && piece.kind != "header" {
			continue
		}
		fmt.Fprintf(&out, "\t.section %s, \"a\", @progbits\n", linkSectionName(piece))
		at := piece.addr
		emit := func(until uint32) {
			for at < until {
				next, edit := until, -1
				for index, e := range layout.edits {
					if e.Addr <= at && at < e.end() {
						next, edit = min(e.end(), until), index
						break
					}
					if at < e.Addr && e.Addr < next {
						next = e.Addr
					}
				}
				switch {
				case edit >= 0:
					e := layout.edits[edit]
					fmt.Fprintf(&out, "\t.byte %s\n", linkByteList(e.Bytes[at-e.Addr:next-e.Addr]))
				case image == "":
					fmt.Fprintf(&out, "\t.space %d\n", next-at)
				default:
					fmt.Fprintf(&out, "\t.incbin \"%s\", %d, %d\n", image, m.offset(at), next-at)
				}
				at = next
			}
		}
		if piece.kind == "header" {
			// "PS-X EXE", the entry point at 0x10 and the text address and
			// size at 0x18; the other fields and the region text are original.
			fmt.Fprintf(&out, "\t.ascii \"PS-X EXE\"\n")
			at += 8
			emit(piece.addr + 0x10)
			fmt.Fprintf(&out, "\t.word %s\n", mainEntryPoint)
			at += 4
			emit(piece.addr + 0x18)
			fmt.Fprintf(&out, "\t.word __exe_text_start\n\t.word __exe_text_size\n")
			at += 8
		}
		// A relocated word comes from its symbol, so it follows the symbol
		// wherever the link puts it; a disc word follows its file's place
		// and size in the disc layout.
		word := func(written linkWord) {
			emit(written.addr)
			fmt.Fprintf(&out, "\t.word %s\n", written.expr)
			at += 4
		}
		// A data row inside a relocated word (an array base biased into a
		// pointer's last bytes) is defined back from the word's end.
		label := func(symbol symbolSpec) {
			if symbol.Addr < at {
				fmt.Fprintf(&out, "\t.globl %s\n\t.set %s, . - %d\n", symbol.Name, symbol.Name, at-symbol.Addr)
				return
			}
			emit(symbol.Addr)
			fmt.Fprintf(&out, "\t.globl %s\n%s:\n", symbol.Name, symbol.Name)
		}
		labels := piece.labels
		for _, written := range piece.words {
			for len(labels) > 0 && labels[0].Addr <= written.addr {
				label(labels[0])
				labels = labels[1:]
			}
			word(written)
			for len(labels) > 0 && labels[0].Addr < at {
				label(labels[0])
				labels = labels[1:]
			}
		}
		for _, symbol := range labels {
			label(symbol)
		}
		emit(piece.addr + uint32(piece.size))
	}
	return out.String()
}

// linkScript places each piece at its address (every input section carries
// its piece's name, see linkObjectSections), binds the names the objects
// leave undefined that this module cannot define (its links, MAIN, imports,
// the absolute data rows), defines each data row inside its own code from its
// piece, and asserts every function, label and such row, and that the image
// ends inside its RAM window.
// A module with `$gp`-relative code sets `_gp` to the label gp names. A
// nonzero shift moves every piece and label by that many bytes, and the RAM
// window with them (reloc-check's shifted links); the bound names stay where
// they are. The layout's insert moves every piece but MAIN's header inside a
// window that stays put, after the filler section (.insert), and MAIN's
// padding section follows its last piece. In an edition's flowing layout each
// piece moves by its delta, and a piece of changed code places its own
// section, a changed function's new .rodata and its filler.
func linkScript(layout linkLayout, dataObject string, bindings map[string]uint32, gp string, shift uint32) string {
	var script strings.Builder
	script.WriteString("OUTPUT_ARCH(mips)\n")
	if gp != "" {
		fmt.Fprintf(&script, "_gp = %s;\n", gp)
	}
	names := make([]string, 0, len(bindings))
	for name := range bindings {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(&script, "PROVIDE(%s = 0x%08x);\n", name, bindings[name])
	}
	at := func(piece linkPiece, addr uint32) uint32 {
		if piece.kind == "header" {
			return addr + shift
		}
		return addr + shift + layout.insert + piece.delta
	}
	// The assembler aligns .text to 16 bytes; SUBALIGN keeps each input at its
	// piece's address, as the per-function link does.
	script.WriteString("SECTIONS {\n")
	first, last := layout.pieces[0], layout.pieces[len(layout.pieces)-1]
	if first.kind == "header" && len(layout.pieces) > 1 {
		first = layout.pieces[1]
	}
	if layout.insert > 0 {
		fmt.Fprintf(&script, "  .insert 0x%08x : SUBALIGN(4) { *(.insert) }\n", first.addr+shift)
	}
	end := ""
	section := func(name string, addr uint32, align int) {
		fmt.Fprintf(&script, "  %s 0x%08x : SUBALIGN(%d) { *(%s) }\n", name, addr, align, name)
		end = name
	}
	for _, piece := range layout.pieces {
		align := 4
		if piece.kind != "text" && piece.kind != "rodata" {
			align = 1
		}
		e := piece.edition
		if e == nil || e.used > 0 {
			section(linkSectionName(piece), at(piece, piece.addr), align)
		}
		if e != nil && e.rodata > 0 {
			section(linkNewRodataName(piece), at(piece, piece.addr+uint32(e.rodataAt)), 4)
		}
		if e != nil && e.fill > 0 {
			section(linkFillName(piece), at(piece, piece.addr+uint32(e.fillAt)), 1)
		}
	}
	if layout.padding > 0 {
		section(".padding", at(last, last.addr+uint32(last.slot())), 1)
	}
	// Another copy of a shared literal keeps its address for its object's
	// references and loads nothing: the loaded copy holds the same bytes.
	for _, piece := range layout.shadows {
		fmt.Fprintf(&script, "  %s 0x%08x (NOLOAD) : SUBALIGN(4) { *(%s) }\n", linkShadowName(piece), at(piece, piece.addr), linkShadowName(piece))
	}
	// Every object's .data, .bss and small-data sections are empty
	// (objectHoldsOnlyCode), as is the data object's .text; the debug
	// sections never load.
	fmt.Fprintf(&script, "  /DISCARD/ : { \"%s\"(.text) *(.data) *(.bss) *(.sdata) *(.sbss) *(.reginfo) *(.pdr) *(.comment) *(.gnu.attributes) *(.MIPS.abiflags) *(.debug*) *(.mdebug*) }\n}\n", dataObject)
	for _, piece := range layout.pieces {
		for _, alias := range piece.aliases {
			fmt.Fprintf(&script, "%s = ADDR(%s) + 0x%x;\n", alias.Name, linkSectionName(piece), alias.Addr-piece.addr)
		}
	}
	if header := layout.pieces[0]; header.kind == "header" && len(layout.pieces) > 1 {
		start := linkSectionName(first)
		if layout.insert > 0 {
			start = ".insert"
		}
		fmt.Fprintf(&script, "__exe_text_start = ADDR(%s);\n", start)
		fmt.Fprintf(&script, "__exe_text_size = ADDR(%s) + SIZEOF(%s) - __exe_text_start;\n", end, end)
	}
	for _, piece := range layout.pieces {
		if piece.kind == "text" {
			fmt.Fprintf(&script, "ASSERT(%s == 0x%08x, \"%s must begin at 0x%08x\");\n", piece.function.Name, at(piece, piece.addr), piece.function.Name, at(piece, piece.addr))
		}
		for _, label := range append(append([]symbolSpec(nil), piece.labels...), piece.aliases...) {
			fmt.Fprintf(&script, "ASSERT(%s == 0x%08x, \"%s must be at 0x%08x\");\n", label.Name, at(piece, label.Addr), label.Name, at(piece, label.Addr))
			// Code loads the row's page as a number, so the row must stay in
			// it. The shifted links move every row on purpose; reloc-check
			// lists the code that loads the page instead.
			if label.Page != 0 && shift == 0 {
				fmt.Fprintf(&script, "ASSERT(((%s + 0x8000) & 0xffff0000) == 0x%08x, \"%s must stay in page 0x%08x, which code loads as a number\");\n",
					label.Name, label.Page, label.Name, label.Page)
			}
		}
	}
	if layout.ramEnd != 0 {
		fmt.Fprintf(&script, "ASSERT(ADDR(%s) + SIZEOF(%s) <= 0x%08x, \"the image must end by its RAM window's end 0x%08x\");\n",
			end, end, layout.ramEnd+shift, layout.ramEnd+shift)
	}
	return script.String()
}

// compileHistoricalObject returns the relocatable object of one source under
// profile in buildDir, from the cache when the same input compiled before.
func (p project) compileHistoricalObject(sourcePath, buildDir string, profile compilerProfile) (string, error) {
	preprocessed, language, err := p.historicalInput(sourcePath, buildDir, profile)
	if err != nil {
		return "", err
	}
	return p.historicalObject(sourcePath, preprocessed, language, buildDir, profile)
}

// historicalObject is compileHistoricalObject's object for the input
// historicalInput wrote to preprocessed.
func (p project) historicalObject(sourcePath, preprocessed, language, buildDir string, profile compilerProfile) (string, error) {
	key, err := compilationCacheKey(preprocessed, "", profile, language)
	if err != nil {
		return "", err
	}
	object := filepath.Join(buildDir, profile.name+".o")
	cachePath := filepath.Join(p.root, "build", "cache", "tools", key+".o")
	useCache := os.Getenv("TOOLS_CACHE") != "0"
	if useCache {
		if data, err := os.ReadFile(cachePath); err == nil {
			return object, atomicWrite(object, data)
		}
	}
	if object, err = p.assembleHistoricalObject(sourcePath, preprocessed, buildDir, profile, language); err != nil {
		return "", err
	}
	if useCache {
		data, err := os.ReadFile(object)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
			return "", err
		}
		if err := atomicWrite(cachePath, data); err != nil {
			return "", err
		}
	}
	return object, nil
}

type linkResult struct {
	output    string
	identical bool   // the whole image equals the original file
	checked   string // what was compared

	// What linking the module again takes (reloc-check's shifted links).
	elf      string
	layout   linkLayout
	inputs   []string // the data object, then each function's object
	bindings map[string]uint32
	targets  map[string]uint32 // the bindings as target/ gives them
	gp       string
}

// linkObjects runs the linker on inputs (the data object first) with script.
func linkObjects(script, elfPath string, inputs []string) error {
	arguments := append([]string{"-EL", "--orphan-handling=error", "-T", script, "-o", elfPath}, inputs...)
	if output, err := exec.Command("mipsel-linux-gnu-ld", arguments...).CombinedOutput(); err != nil {
		return fmt.Errorf("mipsel-linux-gnu-ld: %w\n%s", err, bytes.TrimSpace(output))
	}
	return nil
}

// linkModule compiles every function of m to an object, writes the data
// assembly and the linker script under build/link/<module>/, links them and
// compares the image with the original file (or, without it, every function
// and rodata range with its hash). Each name another module defines is bound
// where target/ puts it, or at the address in values (another module's link
// output, buildCommand's second pass).
func (p project) linkModule(resolver symbolResolver, m *moduleSpec, values map[string]uint32) (linkResult, error) {
	var result linkResult
	layout, err := moduleLinkLayout(m)
	if err != nil {
		return result, err
	}
	want, imageErr := p.readModuleImage(m)
	if imageErr != nil && !errors.Is(imageErr, errNotExtracted) {
		return result, imageErr
	}
	image := ""
	if imageErr == nil {
		image = filepath.Join(p.filesRoot, filepath.FromSlash(m.File))
	}
	workDir := filepath.Join(p.workDir("link"), m.ID)
	objects := make(map[string]string, len(m.Functions))
	errs := make([]error, len(m.Functions))
	jobs, err := toolJobs()
	if err != nil {
		return result, err
	}
	var lock sync.Mutex
	next := make(chan int)
	var wait sync.WaitGroup
	for range min(jobs, len(m.Functions)) {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for index := range next {
				f := &m.Functions[index]
				profile, _ := compilerProfileNamed(m.profileOf(f))
				source := filepath.Join(p.root, filepath.FromSlash(m.sourceOf(f)))
				object, err := p.compileHistoricalObject(source, filepath.Join(workDir, f.Name), profile)
				lock.Lock()
				objects[f.Name], errs[index] = object, err
				lock.Unlock()
			}
		}()
	}
	for index := range m.Functions {
		next <- index
	}
	close(next)
	wait.Wait()
	if err := errors.Join(errs...); err != nil {
		return result, err
	}

	outDir := p.outputDir("link", m.ID)
	if p.linkInsert > 0 {
		outDir = filepath.Join(p.root, "build", "shift", m.ID)
		layout.insert, layout.padding = p.linkInsert, shiftPadding(m, p.linkInsert)
	}
	if sizes := p.editionSizes[m.ID]; len(sizes) > 0 {
		// The edition's flowing layout, whose locks editionLayouts checked.
		if layout, err = flowLayout(m, layout, p.edition, sizes); err != nil {
			return result, err
		}
	}
	layout.edits = p.editionModuleEdits[m.ID]
	if err := os.MkdirAll(filepath.Join(outDir, "objects"), 0o755); err != nil {
		return result, err
	}
	renames, absolute, err := linkObjectRenames(resolver, m, &layout)
	if err != nil {
		return result, err
	}
	sections, err := linkObjectSections(layout)
	if err != nil {
		return result, err
	}
	for name, object := range objects {
		if err := objectHoldsOnlyCode(name, object); err != nil {
			return result, err
		}
		renamed := filepath.Join(outDir, "objects", name+".o")
		arguments := []string{}
		olds := make([]string, 0, len(renames[name]))
		for old := range renames[name] {
			olds = append(olds, old)
		}
		sort.Strings(olds)
		for _, old := range olds {
			arguments = append(arguments, "--redefine-sym", old+"="+renames[name][old])
		}
		kinds := make([]string, 0, len(sections[name]))
		for kind := range sections[name] {
			kinds = append(kinds, kind)
		}
		sort.Strings(kinds)
		for _, kind := range kinds {
			arguments = append(arguments, "--rename-section", kind+"="+sections[name][kind])
		}
		if err := runInDir(p.root, append(append([]string{"mipsel-linux-gnu-objcopy"}, arguments...), object, renamed)...); err != nil {
			return result, err
		}
		objects[name] = renamed
	}
	dataSource := filepath.Join(outDir, "data.s")
	dataObject := filepath.Join(outDir, "data.o")
	if err := atomicWrite(dataSource, []byte(linkDataAssembly(m, layout, image))); err != nil {
		return result, err
	}
	if err := runInDir(p.root, "mipsel-linux-gnu-as", "-EL", "-march=r3000", "-no-pad-sections", "-o", dataObject, dataSource); err != nil {
		return result, err
	}
	defined := map[string]bool{}
	for _, f := range m.Functions {
		defined[f.Name] = true
	}
	for _, piece := range layout.pieces {
		for _, label := range append(append([]symbolSpec(nil), piece.labels...), piece.aliases...) {
			defined[label.Name] = true
		}
	}
	bindings := map[string]uint32{}
	for name, addr := range absolute {
		bindings[name] = addr
	}
	// The data object names the symbols its relocated words hold.
	users := []string{dataObject}
	for _, object := range objects {
		users = append(users, object)
	}
	for _, object := range users {
		undefined, err := undefinedObjectIdentifiers(object)
		if err != nil {
			return result, err
		}
		for name := range undefined {
			if addr, ok := resolver.resolve(m, name); ok && !defined[name] {
				bindings[name] = addr
			}
		}
	}
	result.targets = maps.Clone(bindings)
	if p.edition != "" {
		// Each name another module's link places takes the address that
		// module's flowing layout gives it (editionLayouts).
		for name, addr := range bindings {
			if owner, ok := resolver.owner(m, name); ok {
				bindings[name] = p.editionMaps[owner.module].at(addr)
			}
		}
	}
	for name, value := range values {
		if _, ok := bindings[name]; ok {
			bindings[name] = value
		}
	}
	script := filepath.Join(outDir, "link.ld")
	gp, err := linkGlobalPointer(m, layout)
	if err != nil {
		return result, err
	}
	if err := atomicWrite(script, []byte(linkScript(layout, dataObject, bindings, gp, 0))); err != nil {
		return result, err
	}
	elfPath := filepath.Join(outDir, m.ID+".elf")
	inputs := []string{dataObject}
	for _, f := range m.Functions {
		inputs = append(inputs, objects[f.Name])
	}
	if err := linkObjects(script, elfPath, inputs); err != nil {
		if p.edition != "" && strings.Contains(err.Error(), "undefined reference") {
			err = fmt.Errorf("%w\n(a changed function that calls a new function or names a new global needs a place edition %s does not give yet)", err, p.edition)
		}
		return result, err
	}
	result.elf, result.layout, result.inputs, result.bindings, result.gp = elfPath, layout, inputs, bindings, gp
	if err := p.checkLinkedSizes(elfPath, layout); err != nil {
		return result, err
	}
	result.output = filepath.Join(outDir, filepath.Base(filepath.FromSlash(m.File)))
	if err := runInDir(p.root, "mipsel-linux-gnu-objcopy", "-O", "binary", elfPath, result.output); err != nil {
		return result, err
	}
	got, err := os.ReadFile(result.output)
	if err != nil {
		return result, err
	}
	if size := m.Size + int(layout.insert+layout.growth+layout.padding); len(got) != size {
		return result, fmt.Errorf("%s: linked image has %d bytes, not %d", m.ID, len(got), size)
	}
	if layout.insert > 0 {
		result.checked = fmt.Sprintf("%d bytes up; checked after the second pass", layout.insert)
		return result, nil
	}
	if p.edition != "" {
		// editionCheck compares the image once every module has linked.
		result.checked = "checked after the second pass"
		if grew := layout.growth + layout.padding; grew > 0 {
			result.checked = fmt.Sprintf("%d bytes more; checked after the second pass", grew)
		}
		return result, nil
	}
	if want != nil {
		sum := sha256.Sum256(got)
		if offset, differs := mismatch(want, got); differs {
			return result, fmt.Errorf("%s: linked image differs at file offset 0x%x (0x%08x)", m.ID, offset, m.Load+uint32(offset))
		}
		result.identical, result.checked = true, "sha256 "+hex.EncodeToString(sum[:8])
		return result, nil
	}
	checked := 0
	for _, f := range m.Functions {
		if shortHash(got[m.offset(f.Addr):m.offset(f.Addr)+f.Size]) != f.Hash {
			return result, fmt.Errorf("%s: %s differs from its hash", m.ID, f.Name)
		}
		for _, rodata := range f.Rodata {
			if shortHash(got[m.offset(rodata.Addr):m.offset(rodata.Addr)+rodata.Size]) != rodata.Hash {
				return result, fmt.Errorf("%s: %s rodata 0x%08x differs from its hash", m.ID, f.Name, rodata.Addr)
			}
		}
		checked++
	}
	result.checked = fmt.Sprintf("%d function hashes (no disc: data zeroed)", checked)
	return result, nil
}

// linkGlobalPointer names the data label `_gp` takes in a module whose code
// addresses small data relative to `$gp`: the one at the profile's global
// pointer (MAIN's g_psyq_crt_saved_ra, which the C start-up loads). It is ""
// when no function uses such a profile.
func linkGlobalPointer(m *moduleSpec, layout linkLayout) (string, error) {
	var gp uint32
	for index := range m.Functions {
		profile, ok := compilerProfileNamed(m.profileOf(&m.Functions[index]))
		if !ok || profile.smallDataLimit == 0 {
			continue
		}
		if gp != 0 && gp != profile.globalPointer {
			return "", fmt.Errorf("%s: its functions use two global pointers, 0x%08x and 0x%08x", m.ID, gp, profile.globalPointer)
		}
		gp = profile.globalPointer
	}
	if gp == 0 {
		return "", nil
	}
	for _, piece := range layout.pieces {
		for _, label := range piece.labels {
			if label.Addr == gp {
				return label.Name, nil
			}
		}
	}
	return "", fmt.Errorf("%s: no data row in its bytes names the global pointer 0x%08x", m.ID, gp)
}

// objectHoldsOnlyCode fails when a function's object defines data of its own
// (a loaded section other than .text and .rodata with bytes in it, or a
// common symbol): the module's data are its target rows, not C definitions.
func objectHoldsOnlyCode(name, path string) error {
	object, err := elf.Open(path)
	if err != nil {
		return err
	}
	defer object.Close()
	for _, section := range object.Sections {
		if section.Flags&elf.SHF_ALLOC == 0 || section.Size == 0 {
			continue
		}
		switch section.Name {
		case ".text", ".rodata", ".rdata", ".reginfo", ".MIPS.abiflags", ".pdr":
			continue
		}
		return fmt.Errorf("%s defines %d bytes of %s", name, section.Size, section.Name)
	}
	symbols, err := object.Symbols()
	if err != nil {
		return err
	}
	for _, symbol := range symbols {
		if symbol.Section == elf.SHN_COMMON {
			return fmt.Errorf("%s defines the common symbol %s", name, symbol.Name)
		}
	}
	return nil
}

// checkLinkedSizes fails when an object's section is not the size its piece
// declares (or, for changed code in an edition, the size its object gave the
// flowing layout), which would shift or hide bytes after it.
func (p project) checkLinkedSizes(elfPath string, layout linkLayout) error {
	linked, err := elf.Open(elfPath)
	if err != nil {
		return err
	}
	defer linked.Close()
	for _, piece := range layout.pieces {
		section := linked.Section(linkSectionName(piece))
		want := piece.size
		if piece.edition != nil {
			want = piece.edition.used
		}
		switch {
		case want == 0 && (section == nil || section.Size == 0):
			// A changed function's .rodata its new code no longer has.
		case section == nil:
			return fmt.Errorf("the link produced no %s", linkSectionName(piece))
		case int(section.Size) != want:
			what := linkSectionName(piece)
			if piece.function != nil {
				what = piece.function.Name + " " + piece.kind
			}
			return fmt.Errorf("%s is %d bytes, the target %d", what, section.Size, want)
		}
		if e := piece.edition; e != nil {
			for _, part := range []struct {
				name string
				size int
			}{{linkNewRodataName(piece), e.rodata}, {linkFillName(piece), e.fill}} {
				if section := linked.Section(part.name); part.size > 0 && (section == nil || int(section.Size) != part.size) {
					return fmt.Errorf("%s's %s is not %d bytes", piece.function.Name, part.name, part.size)
				}
			}
		}
	}
	// A shadow loads nothing; `make check` compares each copy's bytes.
	for _, piece := range layout.shadows {
		section := linked.Section(linkShadowName(piece))
		if section == nil || int(section.Size) != piece.size {
			return fmt.Errorf("%s's copy of the literal at 0x%08x is not %d bytes", piece.function.Name, piece.addr, piece.size)
		}
	}
	return nil
}

// linkCrossModule is make build's second pass. Each name another module's link
// places (symbolResolver.owner) takes the address that link gave it, and a
// module whose bindings change links again. Sizes and places never depend on
// those addresses, so the two passes settle every module; in the identical
// build nothing changes. It returns how many names took another module's
// address, how many stay where target/ puts them, and the modules linked
// again.
func (p project) linkCrossModule(resolver symbolResolver, modules []*moduleSpec, results map[string]linkResult) (int, int, []string, error) {
	exports := make(map[string]map[string]uint32, len(results))
	for id, result := range results {
		symbols, err := elfSymbolValues(result.elf)
		if err != nil {
			return 0, 0, nil, err
		}
		exports[id] = symbols
	}
	followed, kept := 0, 0
	var again []string
	for _, m := range modules {
		values, owned, absolute := crossModuleValues(resolver, m, results[m.ID].bindings, exports)
		followed, kept = followed+owned, kept+absolute
		if len(values) == 0 {
			continue
		}
		result, err := p.linkModule(resolver, m, values)
		if err != nil {
			return followed, kept, again, fmt.Errorf("%s with other modules' addresses: %w", m.ID, err)
		}
		results[m.ID] = result
		again = append(again, m.ID)
	}
	return followed, kept, again, nil
}

// crossModuleValues gives, of m's bindings, the ones whose owner's link
// (exports, by module) placed them elsewhere, with the new address; it counts
// the bindings an owner places and the ones that keep target/'s address.
func crossModuleValues(resolver symbolResolver, m *moduleSpec, bindings map[string]uint32, exports map[string]map[string]uint32) (map[string]uint32, int, int) {
	values := map[string]uint32{}
	owned, absolute := 0, 0
	for name, addr := range bindings {
		owner, ok := resolver.owner(m, name)
		if !ok {
			absolute++
			continue
		}
		owned++
		if value, ok := exports[owner.module][owner.symbol]; ok && value != addr {
			values[name] = value
		}
	}
	return values, owned, absolute
}

// buildCommand is make build: every selected module linked whole and
// compared with its original, and with all of them identical the disc in
// build/disc; --shift=N links the test disc in build/shift instead.
func (p project) buildCommand(args []string) error {
	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	selection := flags.String("module", "", "modules to build (default: all, and the disc)")
	shift := flags.Uint("shift", 0, "bytes every module's image opens at its start, a multiple of 16: a test disc in build/shift (needs the BIN)")
	edition := flags.String("edition", "", "edition of target/editions.yaml to build, under build/<edition> (default: the original disc)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("usage: tools build [--module=M] [--shift=N] [--edition=E]")
	}
	if *shift != 0 && *edition != "" {
		return errors.New("make build takes SHIFT=N or EDITION=E, not both: the shifted test disc is the original's")
	}
	config, err := p.loadCheckedConfig()
	if err != nil {
		return err
	}
	if p, err = p.withEdition(config, *edition); err != nil {
		return err
	}
	modules := config.Modules
	if *selection != "" {
		if modules, err = resolveModuleSelection(config, *selection); err != nil {
			return err
		}
	}
	if p, err = p.withInputMirror(); err != nil {
		return err
	}
	if p.edition != "" {
		if err := p.editionGuard(modules); err != nil {
			return err
		}
	}
	var shifted *projectConfig
	if *shift != 0 {
		if *shift%16 != 0 || *shift >= 1<<16 || *selection != "" {
			return errors.New("make build SHIFT=N takes a multiple of 16 below 0x10000 and links every module")
		}
		if !p.validExtraction() {
			return errNotExtracted
		}
		var moved []string
		if shifted, moved, err = p.shiftedConfig(config, uint32(*shift)); err != nil {
			return err
		}
		p.linkInsert = uint32(*shift)
		fmt.Fprintf(p.stdout(), "shift: every module's image opens %d bytes at its start; files that move on the disc: %s\n", *shift, strings.Join(moved, " "))
	}
	resolver := newSymbolResolver(config)
	var grown *projectConfig
	if p.edition != "" {
		if p, grown, err = p.editionLayouts(config, resolver); err != nil {
			return err
		}
	}
	failed := 0
	results := make(map[string]linkResult, len(modules))
	var linkedModules []*moduleSpec
	for _, m := range modules {
		result, err := p.linkModule(resolver, m, nil)
		if err != nil {
			failed++
			fmt.Fprintf(p.stdout(), "FAIL %s: %s\n", m.ID, strings.ReplaceAll(firstLines(err.Error(), 8), "\n", "\n     "))
			continue
		}
		results[m.ID] = result
		linkedModules = append(linkedModules, m)
		fmt.Fprintf(p.stdout(), "ok   %s (%d bytes, %s)\n", m.ID, m.Size, result.checked)
	}
	followed, kept, again, err := p.linkCrossModule(resolver, linkedModules, results)
	if err != nil {
		return err
	}
	listed := ""
	if len(again) <= 8 {
		listed = strings.Join(append([]string{""}, again...), " ")
	}
	fmt.Fprintf(p.stdout(), "cross-module: %d names at the address another module's link gives, %d where target/ puts them (fixed areas, modules never resident); %d modules linked again%s\n",
		followed, kept, len(again), listed)
	built := make(map[string]string, len(results))
	original := true
	for id, result := range results {
		built[id] = result.output
		original = original && result.identical
	}
	if shifted != nil {
		return p.finishShiftedLink(resolver, config, shifted, results, built, failed)
	}
	if grown != nil {
		return p.finishEditionLink(resolver, config, grown, results, built, failed, linkedModules)
	}
	compared := "identical to the original"
	if !original {
		compared = "each function at its hash (no BIN: the data bytes are zeros)"
	}
	fmt.Fprintf(p.stdout(), "build: %d of %d modules linked whole, %s\n", len(results), len(results)+failed, compared)
	if config.RAM != nil && len(linkedModules) > 0 {
		fmt.Fprintln(p.stdout(), "ram: the room each window has left after its largest module")
		for _, line := range ramRoom(config.RAM, config, nil, linkedModules) {
			fmt.Fprintln(p.stdout(), line)
		}
	}
	if failed > 0 {
		return fmt.Errorf("build: %d modules failed", failed)
	}
	switch {
	case len(built) != len(config.Modules):
		return nil
	case !p.validExtraction():
		fmt.Fprintln(p.stdout(), "build: no disc without the BIN (the data bytes were zeros)")
		return nil
	}
	// Every module, each equal to its original file: the disc from them.
	return p.writeDisc(config, built, filepath.Join(p.root, "build", "disc"), true)
}

// writeEditionDisc writes the edition's disc to build/<edition>/disc, with the
// files its data edits change copied under build/<edition>/files, and says
// when it is still the original, byte for byte.
func (p project) writeEditionDisc(config *projectConfig, built map[string]string) error {
	dir := p.outputDir("disc")
	edited, err := p.writeEditedFiles()
	if err != nil {
		return err
	}
	p.editedFiles = edited
	if err := p.writeDisc(config, built, dir, false); err != nil {
		return err
	}
	sum, err := sha256File(filepath.Join(dir, targetOutput+".bin"))
	if err != nil {
		return err
	}
	if sum == targetInputSHA256 {
		fmt.Fprintf(p.stdout(), "build: edition %s gives the original disc byte for byte (no option changes it yet)\n", p.edition)
	}
	return nil
}
