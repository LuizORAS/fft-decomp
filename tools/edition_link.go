// edition_link.go is the link of an edition whose code changes size. Each
// module's pieces flow: every piece after a changed function moves by what
// that function grew, rounded up to 16 bytes so that every alignment up to 16
// holds, and a function that shrinks keeps its slot. The rest of a slot is
// filler, a break in each word as at the start of the shifted disc's images,
// and a changed function's new .rodata follows its .text. Each name another
// module places takes the address that module's map gives it, and the disc
// layout is fitted to the grown files. editionCheck then compares every word
// with the original: equal, or a relocation of the objects that names,
// through the address maps, what it named before; each changed function also
// matches its dc_hash, compiled alone at its original address. What the
// flowing layout cannot carry fails before the link with a lock that says why.
package main

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// objectSizes is what a changed function's object holds: its .text, and its
// .rodata with that section's alignment.
type objectSizes struct {
	text, rodata, rodataAlign int
}

// pieceEdition is how an edition's flowing layout fills a piece of changed
// code: the bytes its own section takes (none when the new code drops its
// .rodata), a changed function's new .rodata after its .text, and the filler
// from the end of that content to the end of the piece's slot.
type pieceEdition struct {
	used             int
	rodata, rodataAt int
	fill, fillAt     int
}

// slot is the bytes a piece takes in the link: its size, or in an edition's
// flowing layout what a changed piece grew to.
func (piece linkPiece) slot() int {
	if piece.edition == nil {
		return piece.size
	}
	return piece.edition.fillAt + piece.edition.fill
}

func linkNewRodataName(piece linkPiece) string {
	return fmt.Sprintf(".rodnew_%08x", piece.addr)
}

func linkFillName(piece linkPiece) string {
	return fmt.Sprintf(".fill_%08x", piece.addr)
}

func alignUp(value, align uint32) uint32 {
	if align < 2 {
		return value
	}
	return (value + align - 1) &^ (align - 1)
}

// pieceHolding is the index of the piece that holds addr, or -1.
func pieceHolding(pieces []linkPiece, addr uint32) int {
	index := sort.Search(len(pieces), func(i int) bool { return pieces[i].addr+uint32(pieces[i].size) > addr })
	if index < len(pieces) && pieces[index].addr <= addr {
		return index
	}
	return -1
}

func pieceWhat(piece linkPiece) string {
	if piece.function == nil {
		return linkSectionName(piece)
	}
	return piece.function.Name + "'s ." + piece.kind
}

// flowLayout places m's pieces for an edition whose changed functions'
// objects take sizes (by function name). Every piece after a changed piece
// moves by what that piece grew, rounded up to 16 bytes; a piece that shrinks
// keeps its slot. MAIN pads to whole sectors, as on the shifted disc. It
// fails, naming each case, on what the moved code cannot carry: a data row
// inside a changed function (a second name for its entry aside), a literal a
// changed function shares with another, a row bound at its address inside
// code that moves, and an image that passes its RAM window's end.
func flowLayout(m *moduleSpec, layout linkLayout, edition string, sizes map[string]objectSizes) (linkLayout, error) {
	var locks []string
	lock := func(format string, args ...any) {
		locks = append(locks, m.ID+": "+fmt.Sprintf(format, args...))
	}
	changed := func(f *functionSpec) bool {
		if f == nil {
			return false
		}
		_, ok := sizes[f.Name]
		return ok
	}
	pieces := slices.Clone(layout.pieces)
	var delta uint32
	for index := range pieces {
		piece := &pieces[index]
		piece.delta = delta
		if !changed(piece.function) {
			continue
		}
		f, object := piece.function, sizes[piece.function.Name]
		e := &pieceEdition{used: object.rodata}
		if piece.kind == "text" {
			e.used = object.text
		}
		e.fillAt = e.used
		if piece.kind == "text" && len(f.Rodata) == 0 && object.rodata > 0 {
			e.rodata = object.rodata
			e.rodataAt = int(alignUp(piece.addr+uint32(object.text), uint32(object.rodataAlign)) - piece.addr)
			e.fillAt = e.rodataAt + object.rodata
		}
		slot := piece.size
		if e.fillAt > slot {
			slot += (e.fillAt - slot + 15) &^ 15
		}
		e.fill = slot - e.fillAt
		piece.edition = e
		delta += uint32(slot - piece.size)
		for _, alias := range piece.aliases {
			if piece.kind != "text" || alias.Addr != piece.addr {
				lock("%s changes in edition %s, but the data row %s names a place inside its .%s (+0x%x), which the new code does not keep",
					f.Name, edition, alias.Name, piece.kind, alias.Addr-piece.addr)
			}
		}
	}
	shadows := slices.Clone(layout.shadows)
	for index := range shadows {
		shadow := &shadows[index]
		holder := pieceHolding(pieces, shadow.addr)
		if holder < 0 {
			continue
		}
		shadow.delta = pieces[holder].delta
		if changed(shadow.function) || changed(pieces[holder].function) {
			lock("%s and %s share the literal at 0x%08x, which a change to either in edition %s cannot rewrite for both",
				pieces[holder].function.Name, shadow.function.Name, shadow.addr, edition)
		}
	}
	for _, symbol := range layout.absolute {
		if index := pieceHolding(pieces, symbol.Addr); index >= 0 && (pieces[index].delta != 0 || pieces[index].edition != nil) {
			lock("the data row %s is bound at 0x%08x inside %s, which edition %s moves or changes; the row would stay behind",
				symbol.Name, symbol.Addr, pieceWhat(pieces[index]), edition)
		}
	}
	layout.pieces, layout.shadows, layout.growth = pieces, shadows, delta
	if m.ID == mainModuleID {
		layout.padding = shiftPadding(m, delta)
	}
	if end := m.Load + uint32(m.Size) + delta + layout.padding; layout.ramEnd != 0 && end > layout.ramEnd {
		lock("its image ends at 0x%08x in edition %s, %d bytes past its RAM window's end 0x%08x",
			end, edition, end-layout.ramEnd, layout.ramEnd)
	}
	if len(locks) > 0 {
		return layout, errors.New(strings.Join(locks, "\n"))
	}
	return layout, nil
}

// addressMap takes an address of a module's original image to where an
// edition's flowing layout puts what was there: past the start of the piece
// that held it, by as much as that piece moved; the image's end by all the
// module grew. Every other address, and every address of a module the edition
// leaves alone (the zero map), keeps its place.
type addressMap struct {
	starts, deltas []uint32
	end, growth    uint32
}

func newAddressMap(m *moduleSpec, layout linkLayout) addressMap {
	moved := addressMap{end: uint32(m.end()), growth: layout.growth}
	for _, piece := range layout.pieces {
		moved.starts = append(moved.starts, piece.addr)
		moved.deltas = append(moved.deltas, piece.delta)
	}
	return moved
}

func (moved addressMap) at(addr uint32) uint32 {
	switch {
	case len(moved.starts) == 0 || addr < moved.starts[0] || addr > moved.end:
		return addr
	case addr == moved.end:
		return addr + moved.growth
	}
	index := sort.Search(len(moved.starts), func(i int) bool { return moved.starts[i] > addr }) - 1
	return addr + moved.deltas[index]
}

// objectSizesOf reads what a function's object holds: its .text, and its
// .rodata (or .rdata) with the section's alignment.
func objectSizesOf(path string) (objectSizes, error) {
	object, err := elf.Open(path)
	if err != nil {
		return objectSizes{}, err
	}
	defer object.Close()
	var sizes objectSizes
	for _, section := range object.Sections {
		switch section.Name {
		case ".text":
			sizes.text = int(section.Size)
		case ".rodata", ".rdata":
			if section.Size == 0 {
				continue
			}
			if sizes.rodata != 0 {
				return sizes, fmt.Errorf("%s holds both .rodata and .rdata", path)
			}
			sizes.rodata, sizes.rodataAlign = int(section.Size), int(section.Addralign)
		}
	}
	return sizes, nil
}

// editionLayouts is the first step of an edition's build: the data edits of
// its options (editionEdits); each function the edition changes, in every
// module, compiled to its object; each module's
// layout flowed around what those objects take, with its locks; and the disc
// fitted to the grown files. It returns the project the links run with (the
// sizes and the address maps) and the grown configuration.
func (p project) editionLayouts(config *projectConfig, resolver symbolResolver) (project, *projectConfig, error) {
	p, err := p.editionEdits(config, resolver)
	if err != nil {
		return p, nil, err
	}
	p.editionSizes = map[string]map[string]objectSizes{}
	p.editionMaps = map[string]addressMap{}
	growth := map[string]uint32{}
	var problems []error
	var grew []string
	for _, m := range config.Modules {
		sizes := map[string]objectSizes{}
		for index := range m.Functions {
			f := &m.Functions[index]
			if !p.changes(f) {
				continue
			}
			profile, ok := compilerProfileNamed(m.profileOf(f))
			if !ok {
				return p, nil, fmt.Errorf("%s %s: unknown compiler profile %q", m.ID, f.Name, m.profileOf(f))
			}
			source := filepath.Join(p.root, filepath.FromSlash(m.sourceOf(f)))
			object, err := p.compileHistoricalObject(source, filepath.Join(p.workDir("link"), m.ID, f.Name), profile)
			if err != nil {
				return p, nil, fmt.Errorf("%s %s: %w", m.ID, f.Name, err)
			}
			if err := objectHoldsOnlyCode(f.Name, object); err != nil {
				problems = append(problems, fmt.Errorf("%s: %w; a fix that adds a global variable needs a place edition %s does not give yet", m.ID, err, p.edition))
				continue
			}
			if sizes[f.Name], err = objectSizesOf(object); err != nil {
				return p, nil, err
			}
		}
		if len(sizes) == 0 {
			continue
		}
		layout, err := moduleLinkLayout(m)
		if err != nil {
			return p, nil, err
		}
		if layout, err = flowLayout(m, layout, p.edition, sizes); err != nil {
			problems = append(problems, err)
			continue
		}
		p.editionSizes[m.ID], p.editionMaps[m.ID] = sizes, newAddressMap(m, layout)
		if growth[m.ID] = layout.growth + layout.padding; growth[m.ID] > 0 {
			grew = append(grew, fmt.Sprintf("%s +%d", m.ID, growth[m.ID]))
		}
	}
	if err := errors.Join(problems...); err != nil {
		return p, nil, fmt.Errorf("edition %s cannot place its changed code:\n%w", p.edition, err)
	}
	grown, moved, err := p.grownConfig(config, func(m *moduleSpec) uint32 { return growth[m.ID] },
		"edition "+p.edition+"'s disc", "the edition cannot give the code a new number yet")
	if err != nil {
		return p, nil, err
	}
	if len(grew) > 0 {
		if len(moved) == 0 {
			moved = []string{"none"}
		}
		fmt.Fprintf(p.stdout(), "edition %s: images that grow (bytes): %s; files that move on the disc: %s\n", p.edition, strings.Join(grew, ", "), strings.Join(moved, " "))
	}
	return p, grown, nil
}

// finishEditionLink checks every module an edition linked against its
// original (editionCheck), prints the room each window has left with the
// grown images, against the original's, and, with every module and the BIN,
// writes the edition's disc.
func (p project) finishEditionLink(resolver symbolResolver, config, grown *projectConfig, results map[string]linkResult, built map[string]string, failed int, linked []*moduleSpec) error {
	compared := "not checked: a module failed"
	if failed == 0 {
		compared = "identical to the original"
		report, err := p.editionCheck(config, resolver, results)
		if err != nil {
			return err
		}
		fmt.Fprintf(p.stdout(), "edition %s: %d modules checked against the original (%d identical): %d instructions, %d .rodata words and %d data words moved with what they name, %d disc words from the fitted layout, %d changed functions at their dc_hash, %d module data edits (%d bytes); %d unexplained\n",
			p.edition, report.modules, report.identical, report.code, report.rodata, report.data, report.disc, report.changed, report.edits, report.edited, len(report.unexplained))
		for _, line := range report.unexplained[:min(len(report.unexplained), 10)] {
			fmt.Fprintf(p.stdout(), "  %s\n", line)
		}
		if len(report.unexplained) > 0 {
			return fmt.Errorf("edition %s: %d words changed unexplained", p.edition, len(report.unexplained))
		}
		switch {
		case !p.validExtraction():
			compared = "each word against the functions compiled alone (no BIN: the data bytes are zeros)"
		case report.identical != report.modules && report.edits > 0:
			compared = "each word the original's, an edit's or moved with what it names"
		case report.identical != report.modules:
			compared = "each word the original's or moved with what it names"
		}
	}
	fmt.Fprintf(p.stdout(), "build: %d of %d modules linked whole for edition %s, %s\n", len(results), len(results)+failed, p.edition, compared)
	if config.RAM != nil && len(linked) > 0 {
		var modules []*moduleSpec
		for _, m := range linked {
			modules = append(modules, grown.byID[m.ID])
		}
		fmt.Fprintln(p.stdout(), "ram: the room each window has left after its largest module, against the original")
		for _, line := range ramRoom(config.RAM, grown, config, modules) {
			fmt.Fprintln(p.stdout(), line)
		}
	}
	switch {
	case failed > 0:
		return fmt.Errorf("build: %d modules failed", failed)
	case len(built) != len(config.Modules):
		return nil
	case !p.validExtraction():
		fmt.Fprintln(p.stdout(), "build: no disc without the BIN (the data bytes were zeros)")
		return nil
	}
	return p.writeEditionDisc(grown, built)
}

// editionReport counts what an edition's check compared: the modules, those
// identical to the original, the words that moved with what they name (in
// code, in .rodata, in the data between functions), the disc words the
// fitted layout changed, the changed functions at their dc_hash and the data
// edits with their bytes; and it lists what nothing explains.
type editionReport struct {
	modules, identical, code, rodata, data, disc, changed, edits, edited int
	unexplained                                                          []string
}

// editionCheck compares every module an edition linked with its original,
// word by word (editionCompareModule).
func (p project) editionCheck(config *projectConfig, resolver symbolResolver, results map[string]linkResult) (editionReport, error) {
	var report editionReport
	for _, m := range config.Modules {
		if result, ok := results[m.ID]; ok {
			if err := p.editionCompareModule(&report, config, resolver, m, result); err != nil {
				return report, err
			}
		}
	}
	return report, nil
}

// objectRelocation is one relocation of a loaded section of an object: its
// offset in the section, its type, the symbol's index (which pairs a HI16
// with its LO16), the symbol's name and, for a symbol the object defines, the
// section it lies in.
type objectRelocation struct {
	offset  uint32
	kind    elf.R_MIPS
	symbol  uint32
	name    string
	section string
}

// objectRelocations returns an object's relocations by the name of the loaded
// section they patch, in the object's order (the order that pairs the halves).
func objectRelocations(path string) (map[string][]objectRelocation, error) {
	object, err := elf.Open(path)
	if err != nil {
		return nil, err
	}
	defer object.Close()
	symbols, err := object.Symbols()
	if err != nil && !errors.Is(err, elf.ErrNoSymbols) {
		return nil, err
	}
	relocations := map[string][]objectRelocation{}
	for _, section := range object.Sections {
		if section.Type == elf.SHT_RELA {
			return nil, fmt.Errorf("%s: unexpected RELA relocations in %s", path, section.Name)
		}
		if section.Type != elf.SHT_REL || int(section.Info) >= len(object.Sections) || object.Sections[section.Info].Flags&elf.SHF_ALLOC == 0 {
			continue
		}
		target := object.Sections[section.Info].Name
		data, err := section.Data()
		if err != nil {
			return nil, err
		}
		for offset := 0; offset+8 <= len(data); offset += 8 {
			info := binary.LittleEndian.Uint32(data[offset+4:])
			r := objectRelocation{offset: binary.LittleEndian.Uint32(data[offset:]), kind: elf.R_MIPS(info & 0xff), symbol: info >> 8}
			// A relocation's symbol index counts the null symbol Symbols
			// leaves out.
			if index := int(r.symbol); index > 0 && index <= len(symbols) {
				symbol := symbols[index-1]
				r.name = symbol.Name
				if symbol.Section != elf.SHN_UNDEF && int(symbol.Section) < len(object.Sections) {
					r.section = object.Sections[symbol.Section].Name
				}
			}
			relocations[target] = append(relocations[target], r)
		}
	}
	return relocations, nil
}

// pairedHalf is the relocation that completes the HI16 or LO16 at index: the
// first LO16 of the same symbol after a HI16, the last HI16 of the same
// symbol before a LO16 (one high half serves the low halves after it).
func pairedHalf(relocations []objectRelocation, index int) (int, bool) {
	r := relocations[index]
	if r.kind == elf.R_MIPS_HI16 {
		for next := index + 1; next < len(relocations); next++ {
			if relocations[next].kind == elf.R_MIPS_LO16 && relocations[next].symbol == r.symbol {
				return next, true
			}
		}
		return 0, false
	}
	for previous := index - 1; previous >= 0; previous-- {
		if relocations[previous].kind == elf.R_MIPS_HI16 && relocations[previous].symbol == r.symbol {
			return previous, true
		}
	}
	return 0, false
}

// editionSection is one loaded section of an edition's image as the check
// compares it: the function it belongs to, where it was in the reference and
// where the edition put it, and its bytes in both.
type editionSection struct {
	name, kind, owner string
	before, after     uint32
	reference, linked []byte
}

// editionWords is one module's word check: the address maps (a module absent
// from them keeps every address), the module's sections by name (where a
// symbol an object defines lies), its link's symbols, its bindings as the
// edition's maps moved them and as target/ gives them, $gp in both, and
// whether its data reference is zeros (no BIN), whose words it then checks
// without counting them as moved.
type editionWords struct {
	m                 *moduleSpec
	resolver          symbolResolver
	maps              map[string]addressMap
	places            map[string]editionSection
	symbols           map[string]uint32
	bindings, targets map[string]uint32
	gpBefore, gpAfter uint32
	zeroed            bool
	report            *editionReport
}

func (w *editionWords) fail(format string, args ...any) {
	w.report.unexplained = append(w.report.unexplained, w.m.ID+": "+fmt.Sprintf(format, args...))
}

// moved is where the edition puts what the reference named at addr through a
// relocation's symbol. A symbol an object defines moves with its section
// while addr lies in it; a name bound to another module's object moves by
// that module's map, and one bound in place keeps its address; anything else
// moves by the module's own map.
func (w *editionWords) moved(section, name string, addr uint32) uint32 {
	if section != "" {
		if place, ok := w.places[section]; ok && place.before <= addr && addr <= place.before+uint32(len(place.reference)) {
			return addr - place.before + place.after
		}
		return w.maps[w.m.ID].at(addr)
	}
	if _, bound := w.bindings[name]; bound {
		if owner, ok := w.resolver.owner(w.m, name); ok {
			return w.maps[owner.module].at(addr)
		}
		return addr
	}
	return w.maps[w.m.ID].at(addr)
}

// symbolMove is how far the edition moved a relocation's symbol.
func (w *editionWords) symbolMove(r objectRelocation) uint32 {
	if r.section != "" {
		place := w.places[r.section]
		return place.after - place.before
	}
	before, ok := w.targets[r.name]
	if !ok {
		before, _ = w.resolver.resolve(w.m, r.name)
	}
	return w.symbols[r.name] - before
}

// compareSection compares one section word by word. A relocation's word must
// keep the bits outside its field and name, through moved, what its reference
// word named; every other byte must equal the reference's.
func (w *editionWords) compareSection(s editionSection, relocations []objectRelocation) {
	if len(s.linked) != len(s.reference) {
		w.fail("%s %s is %d bytes, its reference %d", s.owner, s.name, len(s.linked), len(s.reference))
		return
	}
	word := func(image []byte, offset uint32) uint32 { return binary.LittleEndian.Uint32(image[offset:]) }
	low := func(value uint32) uint32 { return uint32(int32(int16(value))) }
	covered := map[uint32]bool{}
	for index, r := range relocations {
		if r.offset%4 != 0 || int(r.offset)+4 > len(s.reference) {
			w.fail("%s %s+0x%x: a relocation outside the section's words", s.owner, s.name, r.offset)
			continue
		}
		covered[r.offset] = true
		before, after := word(s.reference, r.offset), word(s.linked, r.offset)
		siteBefore, siteAfter := s.before+r.offset, s.after+r.offset
		field := uint32(0xffff)
		var named, names uint32
		switch r.kind {
		case elf.R_MIPS_32:
			named, names, field = before, after, 0xffffffff
		case elf.R_MIPS_26:
			named = (siteBefore+4)&0xf0000000 | (before&0x3ffffff)<<2
			names = (siteAfter+4)&0xf0000000 | (after&0x3ffffff)<<2
			field = 0x3ffffff
		case elf.R_MIPS_PC16:
			named, names = siteBefore+4+low(before)<<2, siteAfter+4+low(after)<<2
		case elf.R_MIPS_GPREL16:
			named, names = w.gpBefore+low(before), w.gpAfter+low(after)
		case elf.R_MIPS_HI16, elf.R_MIPS_LO16:
			pair, ok := pairedHalf(relocations, index)
			if !ok || relocations[pair].offset%4 != 0 || int(relocations[pair].offset)+4 > len(s.reference) {
				// A low half whose high half is a number (a lui of the page a
				// data row declares, which the link asserts the row keeps):
				// only the low half moves, with its symbol.
				if r.kind != elf.R_MIPS_LO16 || after>>16 != before>>16 || (after-before-w.symbolMove(r))&0xffff != 0 {
					w.fail("%s %s+0x%x: the %s of %s changed from 0x%08x to 0x%08x, which its symbol's move does not explain", s.owner, s.name, r.offset, r.kind, r.name, before, after)
				} else if after != before {
					w.count(s)
				}
				continue
			}
			high, lowHalf := r.offset, relocations[pair].offset
			if r.kind == elf.R_MIPS_LO16 {
				high, lowHalf = lowHalf, high
			}
			named = word(s.reference, high)<<16 + low(word(s.reference, lowHalf))
		default:
			w.fail("%s %s+0x%x: a %s relocation, which the check does not decode", s.owner, s.name, r.offset, r.kind)
			continue
		}
		want := w.moved(r.section, r.name, named)
		// Each half holds its own part of the address (the high half with the
		// low half's carry), so a wrong half fails once, at its own word.
		switch r.kind {
		case elf.R_MIPS_HI16:
			names, want = after&0xffff, (want+0x8000)>>16&0xffff
		case elf.R_MIPS_LO16:
			names, want = after&0xffff, want&0xffff
		}
		switch {
		case after&^field != before&^field:
			w.fail("%s %s+0x%x: the %s changed outside its field, from 0x%08x to 0x%08x", s.owner, s.name, r.offset, r.kind, before, after)
		case names != want:
			w.fail("%s %s+0x%x: the %s of %s gives 0x%x for what was at 0x%08x, but the edition needs 0x%x", s.owner, s.name, r.offset, r.kind, r.name, names, named, want)
		case after != before:
			w.count(s)
		}
	}
	for offset := 0; offset < len(s.linked); {
		if covered[uint32(offset)] {
			offset += 4
			continue
		}
		if (s.before+uint32(offset))%4 == 0 && offset+4 <= len(s.linked) {
			if before, after := word(s.reference, uint32(offset)), word(s.linked, uint32(offset)); before != after {
				w.fail("%s %s word 0x%08x changed from 0x%08x to 0x%08x, and no relocation explains it", s.owner, s.name, s.before+uint32(offset), before, after)
			}
			offset += 4
			continue
		}
		if s.reference[offset] != s.linked[offset] {
			w.fail("%s %s byte 0x%08x changed", s.owner, s.name, s.before+uint32(offset))
		}
		offset++
	}
}

func (w *editionWords) count(s editionSection) {
	if s.kind == "text" {
		w.report.code++
	} else {
		w.report.rodata++
	}
}

// compareData compares a piece of the bytes between functions (or MAIN's
// header): each word the link writes must hold what the edition gives it
// (expected), and counts as moved or as a disc word when it changed; every
// other byte equals the reference's.
func (w *editionWords) compareData(piece linkPiece, reference, linked []byte, expected map[uint32]uint32, disc map[uint32]bool) {
	for offset := 0; offset < piece.size; {
		addr := piece.addr + uint32(offset)
		if addr%4 != 0 || offset+4 > piece.size {
			if reference[offset] != linked[offset] {
				w.fail("byte 0x%08x changed", addr)
			}
			offset++
			continue
		}
		before, after := binary.LittleEndian.Uint32(reference[offset:]), binary.LittleEndian.Uint32(linked[offset:])
		offset += 4
		want, written := expected[addr]
		switch {
		case !written && before != after:
			w.fail("data word 0x%08x changed from 0x%08x to 0x%08x, and no relocation explains it", addr, before, after)
		case written && after != want:
			w.fail("0x%08x holds 0x%08x, but the edition gives 0x%08x", addr, after, want)
		case w.zeroed:
		case written && after != before && disc[addr]:
			w.report.disc++
		case written && after != before:
			w.report.data++
		}
	}
}

// fillerBytes is the filler of n bytes at addr: a break in each whole word,
// zeros around them (linkDataAssembly writes the same).
func fillerBytes(addr uint32, n int) []byte {
	out := make([]byte, n)
	lead := int(alignUp(addr, 4) - addr)
	for offset := lead; offset+4 <= n; offset += 4 {
		binary.LittleEndian.PutUint32(out[offset:], shiftFillerWord)
	}
	return out
}

// editionCompareModule is editionCheck for one module. The reference is the
// original image or, without it, zeros for the data and each function as it
// compiles alone at its address, which its hash proves; a module the edition
// neither moves nor rebinds already holds every unchanged function in place,
// and those bytes, once their hashes prove them, serve. A changed function's
// reference is its code compiled alone at its address, which its dc_hash
// proves, and the bytes a data edit covers are the edit's.
func (p project) editionCompareModule(report *editionReport, config *projectConfig, resolver symbolResolver, m *moduleSpec, result linkResult) error {
	linked, err := os.ReadFile(result.output)
	if err != nil {
		return err
	}
	original, err := p.readModuleImage(m)
	if err != nil && !errors.Is(err, errNotExtracted) {
		return err
	}
	layout := result.layout
	if size := m.Size + int(layout.growth+layout.padding); len(linked) != size {
		return fmt.Errorf("%s: the edition's image has %d bytes, not %d", m.ID, len(linked), size)
	}
	report.modules++
	changed := map[string]bool{}
	for index := range m.Functions {
		if p.changes(&m.Functions[index]) {
			changed[m.Functions[index].Name] = true
		}
	}
	if original != nil && len(changed) == 0 && bytes.Equal(original, linked) {
		report.identical++
		return nil
	}
	w := &editionWords{m: m, resolver: resolver, maps: p.editionMaps, places: map[string]editionSection{},
		bindings: result.bindings, targets: result.targets, zeroed: original == nil, report: report}
	if w.symbols, err = elfSymbolValues(result.elf); err != nil {
		return err
	}
	if result.gp != "" {
		w.gpBefore, _ = resolver.resolve(m, result.gp)
		w.gpAfter = w.symbols["_gp"]
	}
	moved := layout.growth != 0 || !maps.Equal(result.bindings, result.targets)
	reference, only := original, changed
	if reference == nil {
		reference = make([]byte, m.Size)
		if m.ID == mainModuleID {
			copy(reference, "PS-X EXE") // the only header bytes the link writes as text
		}
		if moved {
			only = nil
		}
	}
	alone := map[string]map[string][]byte{}
	if only == nil || len(only) > 0 {
		compiled, err := p.compileModule(config, resolver, m, only)
		if err != nil {
			return err
		}
		for index := range m.Functions {
			f := &m.Functions[index]
			if only != nil && !only[f.Name] {
				continue
			}
			if compiled[index].err != nil {
				return fmt.Errorf("%s %s: %w", m.ID, f.Name, compiled[index].err)
			}
			alone[f.Name] = compiled[index].sections
		}
	}
	for index := range m.Functions {
		f := &m.Functions[index]
		if changed[f.Name] {
			expected, err := p.editionHashOf(f)
			if err != nil {
				return err
			}
			if editionHash(alone[f.Name]) != expected {
				w.fail("%s differs from its dc_hash", f.Name)
			} else {
				report.changed++
			}
			continue
		}
		if original != nil {
			continue
		}
		text := linked[m.offset(f.Addr) : m.offset(f.Addr)+f.Size]
		var rodata []byte
		if len(f.Rodata) > 0 {
			rodata = linked[m.offset(f.Rodata[0].Addr) : m.offset(f.Rodata[0].Addr)+f.Rodata[0].Size]
		}
		if sections, ok := alone[f.Name]; ok {
			text, rodata = sections[".text"], sections[".rodata"]
		}
		if len(text) != f.Size || shortHash(text) != f.Hash {
			w.fail("%s differs from its hash", f.Name)
			continue
		}
		copy(reference[m.offset(f.Addr):], text)
		for _, literal := range f.Rodata {
			if len(rodata) != literal.Size || shortHash(rodata) != literal.Hash {
				w.fail("%s .rodata 0x%08x differs from its hash", f.Name, literal.Addr)
				continue
			}
			copy(reference[m.offset(literal.Addr):], rodata)
		}
	}
	for _, e := range p.editionModuleEdits[m.ID] {
		copy(reference[m.offset(e.Addr):], e.Bytes)
		report.edits++
		report.edited += len(e.Bytes)
	}
	expected, disc := map[uint32]uint32{}, map[uint32]bool{}
	for _, relocation := range m.Relocations {
		to, _ := resolver.resolve(m, relocation.target())
		expected[relocation.Addr] = w.moved("", relocation.target(), to+relocation.Offset)
	}
	for _, word := range m.DiscWords {
		for index, value := range word.values {
			expected[word.Addr+uint32(4*index)], disc[word.Addr+uint32(4*index)] = value, true
		}
	}
	for _, word := range m.RAMWords {
		expected[word.Addr] = word.value
	}
	if layout.pieces[0].kind == "header" {
		entry, _ := resolver.resolve(m, mainEntryPoint)
		expected[m.Load+0x10] = w.maps[m.ID].at(entry)
		expected[m.Load+0x18] = m.Load + psxEXEHeaderSize
		expected[m.Load+0x1c] = uint32(m.Size-psxEXEHeaderSize) + layout.growth + layout.padding
	}
	byOwner := map[string][]editionSection{}
	add := func(s editionSection) {
		w.places[s.name] = s
		byOwner[s.owner] = append(byOwner[s.owner], s)
	}
	for _, piece := range layout.pieces {
		at := m.offset(piece.addr) + int(piece.delta)
		after := piece.addr + piece.delta
		if piece.kind != "text" && piece.kind != "rodata" {
			w.compareData(piece, reference[m.offset(piece.addr):m.offset(piece.addr)+piece.size], linked[at:at+piece.size], expected, disc)
			continue
		}
		f := piece.function
		s := editionSection{name: linkSectionName(piece), kind: piece.kind, owner: f.Name, before: piece.addr, after: after}
		e := piece.edition
		if e == nil {
			s.reference, s.linked = reference[m.offset(piece.addr):m.offset(piece.addr)+piece.size], linked[at:at+piece.size]
			add(s)
			continue
		}
		s.reference, s.linked = alone[f.Name]["."+piece.kind], linked[at:at+e.used]
		if e.used > 0 || len(s.reference) > 0 {
			add(s)
		}
		if e.rodata > 0 {
			add(editionSection{name: linkNewRodataName(piece), kind: "rodata", owner: f.Name, before: piece.addr + uint32(e.rodataAt), after: after + uint32(e.rodataAt),
				reference: alone[f.Name][".rodata"], linked: linked[at+e.rodataAt : at+e.rodataAt+e.rodata]})
			if gap := linked[at+e.used : at+e.rodataAt]; !bytes.Equal(gap, make([]byte, len(gap))) {
				w.fail("%s: the bytes before its new .rodata are not zero", f.Name)
			}
		}
		if filler := linked[at+e.fillAt : at+e.fillAt+e.fill]; !bytes.Equal(filler, fillerBytes(piece.addr+uint32(e.fillAt), e.fill)) {
			w.fail("%s: the filler after its new %s is not breaks", f.Name, piece.kind)
		}
	}
	for _, shadow := range layout.shadows {
		w.places[linkShadowName(shadow)] = editionSection{name: linkShadowName(shadow), owner: shadow.function.Name, before: shadow.addr, after: shadow.addr + shadow.delta,
			reference: reference[m.offset(shadow.addr) : m.offset(shadow.addr)+shadow.size]}
	}
	for index := range m.Functions {
		f := &m.Functions[index]
		relocations, err := objectRelocations(result.inputs[1+index])
		if err != nil {
			return err
		}
		for _, s := range byOwner[f.Name] {
			w.compareSection(s, relocations[s.name])
		}
	}
	if padding := linked[len(linked)-int(layout.padding):]; !bytes.Equal(padding, make([]byte, len(padding))) {
		w.fail("MAIN's padding is not zero")
	}
	return nil
}
