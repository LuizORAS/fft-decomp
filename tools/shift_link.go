// shift_link.go is make build SHIFT=N: a test disc on which every module's
// image opens N bytes at its start (after MAIN's header) inside a RAM window
// that stays put, so every function, data row and table entry moves and
// everything that names them must follow. The disc goes to build/shift and is
// never the product: the identical disc stays make build's (and
// make patch-check's).
package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// shiftFillerWord fills the inserted bytes: a MIPS break, so code that still
// jumps to a module's old start traps instead of sliding into it as it would
// through zeros (which run as nops).
const shiftFillerWord = 0x0000000d

// shiftPadding is the zeros MAIN ends in on the shifted disc: a PS-X EXE
// loads whole sectors, so its file stays a whole number of them. Other
// modules take none.
func shiftPadding(m *moduleSpec, insert uint32) uint32 {
	if m.ID != mainModuleID {
		return 0
	}
	return (discDataSectorBytes - (uint32(m.Size)+insert)%discDataSectorBytes) % discDataSectorBytes
}

// shiftedConfig is the configuration of the shifted disc: each module's file
// grows by the insertion (and MAIN's padding), the disc layout is fitted to
// the new sizes as config-fmt would fit it, and every module's disc words take
// their values from that layout. It fails when a file whose sector or length
// the code names (include/fft/disc.h) would change, since the code compiles
// with the old number; it returns the files that move.
func (p project) shiftedConfig(config *projectConfig, insert uint32) (*projectConfig, []string, error) {
	if config.Disc == nil {
		return nil, nil, errors.New("make build SHIFT=N needs target/disc.yaml")
	}
	shifted := *config
	shifted.Modules = make([]*moduleSpec, len(config.Modules))
	shifted.byID = make(map[string]*moduleSpec, len(config.Modules))
	for index, m := range config.Modules {
		grown := *m
		grown.Size += int(insert + shiftPadding(m, insert))
		shifted.Modules[index], shifted.byID[m.ID] = &grown, &grown
	}
	layout, problems := config.Disc.fitted(&shifted)
	if len(problems) > 0 {
		return nil, nil, fmt.Errorf("the shifted disc: %s", problems[0])
	}
	shifted.Disc = layout
	shifted.discFiles = layout.files(&shifted)
	names, err := discCodeNames(p.root)
	if err != nil {
		return nil, nil, err
	}
	files := make([]string, 0, len(shifted.discFiles))
	for file := range shifted.discFiles {
		files = append(files, file)
	}
	sort.Strings(files)
	var moved []string
	for _, file := range files {
		after, before := shifted.discFiles[file], config.discFiles[file]
		name := discName(file)
		if names[discSectorPrefix+name] && after.lba != before.lba || names[discCountPrefix+name] && after.sectors != before.sectors {
			return nil, nil, fmt.Errorf("%s would move or grow on the shifted disc, but the code names its sector (include/fft/disc.h); a smaller SHIFT keeps it in place", file)
		}
		if after.lba != before.lba {
			moved = append(moved, file)
		}
	}
	for _, m := range shifted.Modules {
		m.LBA = shifted.discFiles[m.File].lba
	}
	// The grown modules share the disc words with the originals the link
	// writes from.
	for _, m := range config.Modules {
		for index := range m.DiscWords {
			m.DiscWords[index].values = shifted.discWordValues(m.DiscWords[index])
		}
	}
	return &shifted, moved, nil
}

// finishShiftedLink checks every module of the shifted disc against its
// original (shiftCheck), prints the room each window has left with the grown
// images, and writes the disc to build/shift/disc.
func (p project) finishShiftedLink(resolver symbolResolver, config, shifted *projectConfig, results map[string]linkResult, built map[string]string, failed int) error {
	fmt.Fprintf(p.stdout(), "build: %d of %d modules linked whole, %d bytes up\n", len(results), len(results)+failed, p.linkInsert)
	if failed > 0 {
		return fmt.Errorf("build: %d modules failed", failed)
	}
	report, err := p.shiftCheck(resolver, config, results)
	if err != nil {
		return err
	}
	fmt.Fprintf(p.stdout(), "shift: %d modules checked against the original: %d instructions, %d .rodata words and %d data words moved with what they name, %d disc words from the shifted layout; %d unexplained\n",
		report.modules, report.code, report.rodata, report.data, report.disc, len(report.unexplained))
	for _, line := range report.unexplained[:min(len(report.unexplained), 10)] {
		fmt.Fprintf(p.stdout(), "  %s\n", line)
	}
	if len(report.unexplained) > 0 {
		return fmt.Errorf("shift: %d words changed unexplained", len(report.unexplained))
	}
	if config.RAM != nil {
		fmt.Fprintln(p.stdout(), "ram: the room each window has left after its largest module, shifted")
		for _, line := range ramRoom(config.RAM, shifted, shifted.Modules) {
			fmt.Fprintln(p.stdout(), line)
		}
	}
	return p.writeDisc(shifted, built, filepath.Join(p.root, "build", "shift", "disc"), false)
}

// shiftReport counts the words that moved on the shifted disc and lists the
// ones nothing explains.
type shiftReport struct {
	modules, code, rodata, data, disc int
	unexplained                       []string
}

// shiftCheck compares every module of the shifted disc with its original file,
// word for word past the insertion. An instruction may change only as a
// reference moved by the insertion does (a jump, a high half with the low
// half's carry, a low half), a .rodata word only by the insertion, and a data
// word only into what the link now gives it: a relocation's symbol plus offset,
// that symbol having moved by the insertion or not at all, a disc word's
// value from the fitted layout, a RAM word's from the map. MAIN's header may
// change only in its entry point and text size. Every other byte, the filler
// and MAIN's padding hold what they must.
func (p project) shiftCheck(resolver symbolResolver, config *projectConfig, results map[string]linkResult) (shiftReport, error) {
	var report shiftReport
	for _, m := range config.Modules {
		result, ok := results[m.ID]
		if !ok {
			continue
		}
		original, err := p.readModuleImage(m)
		if err != nil {
			return report, err
		}
		linked, err := os.ReadFile(result.output)
		if err != nil {
			return report, err
		}
		symbols, err := elfSymbolValues(result.elf)
		if err != nil {
			return report, err
		}
		shiftCompareModule(&report, m, result.layout, original, linked, symbols, func(name string) (uint32, bool) { return resolver.resolve(m, name) })
	}
	return report, nil
}

// shiftCompareModule is shiftCheck for one module: original is its file,
// linked its shifted image, symbols the shifted link's and from where each
// name was before.
func shiftCompareModule(report *shiftReport, m *moduleSpec, layout linkLayout, original, linked []byte, symbols map[string]uint32, from func(string) (uint32, bool)) {
	insert := layout.insert
	fail := func(format string, args ...any) {
		report.unexplained = append(report.unexplained, m.ID+": "+fmt.Sprintf(format, args...))
	}
	expected := map[uint32]uint32{}
	disc := map[uint32]bool{}
	word := func(image []byte, offset int) uint32 { return binary.LittleEndian.Uint32(image[offset:]) }
	for _, relocation := range m.Relocations {
		to, placed := symbols[relocation.target()]
		was, known := from(relocation.target())
		if delta := to - was; !placed || !known || delta != 0 && delta != insert {
			fail("relocation 0x%08x: %s moved from 0x%08x to 0x%08x", relocation.Addr, relocation.target(), was, to)
			continue
		}
		before := word(original, m.offset(relocation.Addr))
		expected[relocation.Addr] = before + to - was
	}
	for _, w := range m.DiscWords {
		for index, value := range w.values {
			expected[w.Addr+uint32(4*index)], disc[w.Addr+uint32(4*index)] = value, true
		}
	}
	for _, w := range m.RAMWords {
		expected[w.Addr] = w.value
	}
	headerBytes := 0
	if layout.pieces[0].kind == "header" {
		headerBytes = psxEXEHeaderSize
		expected[m.Load+0x10] = symbols[mainEntryPoint]
		expected[m.Load+0x18] = m.Load + psxEXEHeaderSize
		expected[m.Load+0x1c] = uint32(m.Size-psxEXEHeaderSize) + insert + layout.padding
	}
	for offset := headerBytes; offset < headerBytes+int(insert); offset += 4 {
		if word(linked, offset) != shiftFillerWord {
			fail("the filler at file offset 0x%x is not a break", offset)
			break
		}
	}
	for offset := len(linked) - int(layout.padding); offset < len(linked); offset++ {
		if linked[offset] != 0 {
			fail("MAIN's padding at file offset 0x%x is not zero", offset)
			break
		}
	}
	for _, piece := range layout.pieces {
		moved := 0
		if piece.kind != "header" {
			moved = int(insert)
		}
		end := piece.addr + uint32(piece.size)
		for addr := piece.addr; addr < end; {
			at := m.offset(addr)
			if addr%4 != 0 || addr+4 > end {
				if original[at] != linked[at+moved] {
					fail("byte 0x%08x changed", addr)
				}
				addr++
				continue
			}
			before, after := word(original, at), word(linked, at+moved)
			addr += 4
			want, written := expected[addr-4]
			switch {
			case written && (piece.kind == "bytes" || piece.kind == "header"):
				if after != want {
					fail("0x%08x holds 0x%08x, but the link gives 0x%08x", addr-4, after, want)
				} else if after != before && disc[addr-4] {
					report.disc++
				} else if after != before {
					report.data++
				}
			case before == after:
			case piece.kind == "text" && shiftedInstructionExplained(before, after, insert):
				report.code++
			case piece.kind == "rodata" && after-before == insert:
				report.rodata++
			default:
				fail("%s word 0x%08x changed from 0x%08x to 0x%08x", piece.kind, addr-4, before, after)
			}
		}
	}
	report.modules++
}
