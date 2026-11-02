package main

import (
	"debug/elf"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFlowLayoutMovesWhatFollowsAChangedFunction grows, shrinks and empties
// the changed code of the small test module and reads the layout, the address
// map, the script and the filler the link writes.
func TestFlowLayoutMovesWhatFollowsAChangedFunction(t *testing.T) {
	m := linkTestModule()
	m.ramEnd = 0x80100080
	flow := func(sizes map[string]objectSizes) linkLayout {
		t.Helper()
		layout, err := moduleLinkLayout(m)
		if err != nil {
			t.Fatal(err)
		}
		if layout, err = flowLayout(m, layout, "dc", sizes); err != nil {
			t.Fatal(err)
		}
		return layout
	}
	// test_second's 8 bytes grow to 12: its slot takes 24, and everything
	// after it moves 16 bytes.
	layout := flow(map[string]objectSizes{"test_second": {text: 12, rodata: 4, rodataAlign: 4}})
	var deltas []uint32
	for _, piece := range layout.pieces {
		deltas = append(deltas, piece.delta)
	}
	if e := layout.pieces[2].edition; layout.growth != 16 || e == nil || e.used != 12 || e.fillAt != 12 || e.fill != 12 || layout.pieces[2].slot() != 24 ||
		len(deltas) != 6 || deltas[1] != 0 || deltas[2] != 0 || deltas[3] != 16 || deltas[5] != 16 {
		t.Fatalf("growth %d, deltas %v, edition %+v", layout.growth, deltas, e)
	}
	moved := newAddressMap(m, layout)
	for before, after := range map[uint32]uint32{0x80100010: 0x80100010, 0x80100018: 0x80100028, 0x80100030: 0x80100040, 0x80100040: 0x80100050, 0x80100100: 0x80100100, 0x800ffff0: 0x800ffff0} {
		if got := moved.at(before); got != after {
			t.Fatalf("map 0x%08x -> 0x%08x, want 0x%08x", before, got, after)
		}
	}
	if (addressMap{}).at(0x80100030) != 0x80100030 {
		t.Fatal("the zero map moves an address")
	}
	script := linkScript(layout, "/w/data.o", nil, "", 0)
	for _, line := range []string{
		"  .text_80100010 0x80100010 : SUBALIGN(4) { *(.text_80100010) }\n",
		"  .fill_80100010 0x8010001c : SUBALIGN(1) { *(.fill_80100010) }\n",
		"  .bytes_80100018 0x80100028 : SUBALIGN(1) { *(.bytes_80100018) }\n",
		"  .rodata_80100020 0x80100030 : SUBALIGN(4) { *(.rodata_80100020) }\n",
		"ASSERT(test_second == 0x80100010, \"test_second must begin at 0x80100010\");\n",
		"ASSERT(g_test_tail == 0x80100040, \"g_test_tail must be at 0x80100040\");\n",
		"ASSERT(ADDR(.bytes_80100024) + SIZEOF(.bytes_80100024) <= 0x80100080, \"the image must end by its RAM window's end 0x80100080\");\n",
	} {
		if !strings.Contains(script, line) {
			t.Fatalf("script lacks %q:\n%s", line, script)
		}
	}
	if data := linkDataAssembly(m, layout, ""); !strings.Contains(data, "\t.section .fill_80100010, \"a\", @progbits\n\t.rept 3\n\t.byte 0x0d, 0x00, 0x00, 0x00\n\t.endr\n") {
		t.Fatalf("filler:\n%s", data)
	}
	// Shrunk to 4 bytes, it keeps its slot and nothing moves.
	if layout = flow(map[string]objectSizes{"test_second": {text: 4, rodata: 4, rodataAlign: 4}}); layout.growth != 0 || layout.pieces[3].delta != 0 || layout.pieces[2].edition.fill != 4 {
		t.Fatalf("shrunk: growth %d, edition %+v", layout.growth, layout.pieces[2].edition)
	}
	// A .rodata the new code no longer has: its piece is all filler.
	layout = flow(map[string]objectSizes{"test_second": {text: 8}})
	script = linkScript(layout, "/w/data.o", nil, "", 0)
	if strings.Contains(script, "  .rodata_80100020 0x") || !strings.Contains(script, "  .fill_80100020 0x80100020 : SUBALIGN(1) { *(.fill_80100020) }\n") {
		t.Fatalf("dropped .rodata:\n%s", script)
	}
}

// TestFlowLayoutPlacesANewRodataAfterItsText gives test_first a .rodata no
// range declares: it follows the .text at its own alignment, the filler
// follows it, and the object's .rodata takes the new section's name.
func TestFlowLayoutPlacesANewRodataAfterItsText(t *testing.T) {
	m := linkTestModule()
	m.Data = m.Data[:2]
	layout, err := moduleLinkLayout(m)
	if err != nil {
		t.Fatal(err)
	}
	if layout, err = flowLayout(m, layout, "dc", map[string]objectSizes{"test_first": {text: 8, rodata: 6, rodataAlign: 8}}); err != nil {
		t.Fatal(err)
	}
	if e := layout.pieces[1].edition; layout.growth != 16 || e.rodata != 6 || e.rodataAt != 8 || e.fillAt != 14 || e.fill != 10 {
		t.Fatalf("growth %d, edition %+v", layout.growth, e)
	}
	script := linkScript(layout, "/w/data.o", nil, "", 0)
	for _, line := range []string{
		"  .text_80100008 0x80100008 : SUBALIGN(4) { *(.text_80100008) }\n",
		"  .rodnew_80100008 0x80100010 : SUBALIGN(4) { *(.rodnew_80100008) }\n",
		"  .fill_80100008 0x80100016 : SUBALIGN(1) { *(.fill_80100008) }\n",
		"  .text_80100010 0x80100020 : SUBALIGN(4) { *(.text_80100010) }\n",
	} {
		if !strings.Contains(script, line) {
			t.Fatalf("script lacks %q:\n%s", line, script)
		}
	}
	if data := linkDataAssembly(m, layout, ""); !strings.Contains(data, "\t.section .fill_80100008, \"a\", @progbits\n\t.space 2\n\t.rept 2\n\t.byte 0x0d, 0x00, 0x00, 0x00\n\t.endr\n") {
		t.Fatalf("filler:\n%s", data)
	}
	if want := []byte{0, 0, 0x0d, 0, 0, 0, 0x0d, 0, 0, 0}; string(fillerBytes(0x80100016, 10)) != string(want) {
		t.Fatalf("filler bytes %x", fillerBytes(0x80100016, 10))
	}
	sections, err := linkObjectSections(layout)
	if err != nil || sections["test_first"][".rodata"] != ".rodnew_80100008" || sections["test_first"][".text"] != ".text_80100008" {
		t.Fatalf("sections %v %v", sections, err)
	}
	// MAIN pads its grown file to whole sectors.
	main := &moduleSpec{ID: mainModuleID, File: "SCUS_942.21", Load: 0x8000f800, Size: 0x1000, SourceDir: "src/main",
		Functions: []functionSpec{{Addr: 0x80010000, Size: 8, Name: "__SN_ENTRY_POINT"}}}
	if layout, err = moduleLinkLayout(main); err != nil {
		t.Fatal(err)
	}
	if layout, err = flowLayout(main, layout, "dc", map[string]objectSizes{"__SN_ENTRY_POINT": {text: 12}}); err != nil || layout.padding != 2032 {
		t.Fatalf("MAIN padding %d: %v", layout.padding, err)
	}
	if script = linkScript(layout, "/w/data.o", nil, "", 0); !strings.Contains(script, "  .padding 0x80010810 : SUBALIGN(1) { *(.padding) }\n") ||
		!strings.Contains(script, "__exe_text_size = ADDR(.padding) + SIZEOF(.padding) - __exe_text_start;\n") {
		t.Fatalf("MAIN's script:\n%s", script)
	}
}

// TestFlowLayoutLocks fails on what the moved code cannot carry, saying why.
func TestFlowLayoutLocks(t *testing.T) {
	for name, test := range map[string]struct {
		edit  func(*moduleSpec)
		sizes map[string]objectSizes
		want  string
	}{
		"a data row inside changed code": {
			sizes: map[string]objectSizes{"test_first": {text: 8}},
			want:  "test_first changes in edition dc, but the data row g_test_inside names a place inside its .text (+0x4)",
		},
		"a row bound inside changed code": {
			edit:  func(m *moduleSpec) { m.Data = append(m.Data, symbolSpec{Addr: 0x80100014, Name: "g_battle_inside"}) },
			sizes: map[string]objectSizes{"test_second": {text: 8, rodata: 4}},
			want:  "the data row g_battle_inside is bound at 0x80100014 inside test_second's .text, which edition dc moves or changes",
		},
		"a row bound inside code that moves": {
			edit: func(m *moduleSpec) {
				m.Data = append(m.Data[:2], symbolSpec{Addr: 0x80100014, Name: "g_battle_inside"})
			},
			sizes: map[string]objectSizes{"test_first": {text: 12}},
			want:  "the data row g_battle_inside is bound at 0x80100014 inside test_second's .text",
		},
		"a shared literal": {
			edit: func(m *moduleSpec) {
				m.Size = 0x48
				m.Functions = append(m.Functions, functionSpec{Addr: 0x80100040, Size: 8, Name: "test_third", Rodata: []rodataSpec{{Addr: 0x80100020, Size: 4}}})
			},
			sizes: map[string]objectSizes{"test_third": {text: 8, rodata: 4}},
			want:  "share the literal at 0x80100020, which a change to either in edition dc cannot rewrite for both",
		},
		"an image past its window": {
			sizes: map[string]objectSizes{"test_second": {text: 0x80, rodata: 4}},
			want:  "its image ends at 0x801000c0 in edition dc, 64 bytes past its RAM window's end 0x80100080",
		},
	} {
		t.Run(name, func(t *testing.T) {
			m := linkTestModule()
			m.ramEnd = 0x80100080
			if test.edit != nil {
				test.edit(m)
			}
			layout, err := moduleLinkLayout(m)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := flowLayout(m, layout, "dc", test.sizes); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("lock: %v", err)
			}
		})
	}
	// A second name for a changed function's entry moves with it.
	m := linkTestModule()
	m.Data = append(m.Data, symbolSpec{Addr: 0x80100010, Name: "g_test_entry"})
	layout, err := moduleLinkLayout(m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := flowLayout(m, layout, "dc", map[string]objectSizes{"test_second": {text: 12, rodata: 4}}); err != nil {
		t.Fatal(err)
	}
}

// editionTestConfig is the test module beside a BATTLE whose names it links,
// two of them at addresses inside the module's own image.
func editionTestConfig() (*projectConfig, *moduleSpec) {
	m := linkTestModule()
	m.Data = m.Data[:2]
	m.Data = append(m.Data, symbolSpec{Addr: 0x80100030, Name: "g_test_tail"})
	m.Links = []string{"battle"}
	m.Overrides = []overrideSpec{
		{Function: "test_second", Name: "g_battle_pages", Addr: 0x80100030},
		{Function: "test_second", Name: "battle_dead_call", Addr: 0x80100038},
	}
	battle := &moduleSpec{ID: "battle", File: "BATTLE.BIN", Load: 0x80067000, Size: 0x100000,
		Functions: []functionSpec{{Addr: 0x80100038, Size: 4, Name: "battle_dead_call"}},
		Data:      []symbolSpec{{Addr: 0x80100030, Name: "g_battle_pages"}}}
	return &projectConfig{Modules: []*moduleSpec{m, battle}, byID: map[string]*moduleSpec{m.ID: m, "battle": battle}}, m
}

// TestEditionOverrideLocks fails on an override that binds BATTLE's name in
// place over the module's own row once the edition moves it, and leaves a
// dead call to where the module keeps nothing.
func TestEditionOverrideLocks(t *testing.T) {
	config, m := editionTestConfig()
	resolver := newSymbolResolver(config)
	layout, err := moduleLinkLayout(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := editionOverrideLocks(resolver, m, newAddressMap(m, layout), "dc"); err != nil {
		t.Fatalf("in place: %v", err)
	}
	if layout, err = flowLayout(m, layout, "dc", map[string]objectSizes{"test_first": {text: 12}}); err != nil {
		t.Fatal(err)
	}
	err = editionOverrideLocks(resolver, m, newAddressMap(m, layout), "dc")
	if err == nil || !strings.Contains(err.Error(), "test_second's override binds g_battle_pages in place at 0x80100030, where the module keeps g_test_tail, which edition dc moves by 16 bytes") ||
		strings.Contains(err.Error(), "battle_dead_call") {
		t.Fatalf("moved: %v", err)
	}
}

// TestCompareSectionExplainsEachRelocation checks a moved function's words: a
// call that stays, a high and low half that follow a moved row, a low half
// whose high half is a number, a word a name bound in place keeps, and what
// nothing explains.
func TestCompareSectionExplainsEachRelocation(t *testing.T) {
	config, m := editionTestConfig()
	m.Data = append(m.Data, symbolSpec{Addr: 0x80100034, Name: "g_test_page_row", Page: 0x80100000})
	resolver := newSymbolResolver(config)
	layout, err := moduleLinkLayout(m)
	if err != nil {
		t.Fatal(err)
	}
	if layout, err = flowLayout(m, layout, "dc", map[string]objectSizes{"test_first": {text: 12}}); err != nil {
		t.Fatal(err)
	}
	words := func(values ...uint32) []byte {
		out := make([]byte, 4*len(values))
		for index, value := range values {
			binary.LittleEndian.PutUint32(out[4*index:], value)
		}
		return out
	}
	reference := words(
		0x0c040002, // jal test_first
		0x3c028010, // lui v0, %hi(g_test_tail)
		0x8c420030, // lw v0, %lo(g_test_tail)(v0)
		0x8c430034, // lw v1, %lo(g_test_page_row)(v0): its page is a number
		0x80100038, // .word battle_dead_call, bound in place
		0x00000000,
	)
	linked := words(0x0c040002, 0x3c028010, 0x8c420040, 0x8c430044, 0x80100038, 0)
	relocations := []objectRelocation{
		{offset: 0, kind: elf.R_MIPS_26, symbol: 1, name: "test_first"},
		{offset: 4, kind: elf.R_MIPS_HI16, symbol: 2, name: "g_test_tail"},
		{offset: 8, kind: elf.R_MIPS_LO16, symbol: 2, name: "g_test_tail"},
		{offset: 12, kind: elf.R_MIPS_LO16, symbol: 3, name: "g_test_page_row"},
		{offset: 16, kind: elf.R_MIPS_32, symbol: 4, name: "battle_dead_call"},
	}
	check := func(linked []byte) editionReport {
		var report editionReport
		w := &editionWords{m: m, resolver: resolver, maps: map[string]addressMap{m.ID: newAddressMap(m, layout)}, places: map[string]editionSection{},
			symbols:  map[string]uint32{"g_test_tail": 0x80100040, "g_test_page_row": 0x80100044, "battle_dead_call": 0x80100038},
			bindings: map[string]uint32{"battle_dead_call": 0x80100038}, targets: map[string]uint32{"battle_dead_call": 0x80100038}, report: &report}
		w.compareSection(editionSection{name: ".text_80100010", kind: "text", owner: "test_second", before: 0x80100010, after: 0x80100020, reference: reference, linked: linked}, relocations)
		return report
	}
	if report := check(linked); report.code != 2 || len(report.unexplained) != 0 {
		t.Fatalf("report %+v", report)
	}
	for name, edit := range map[string]func([]byte){
		"a word no relocation names": func(image []byte) { image[20] = 1 },
		"a low half left behind":     func(image []byte) { image[8] = 0x30 },
		"a call moved":               func(image []byte) { image[0] = 0x06 },
		"a bound word moved":         func(image []byte) { image[16] = 0x48 },
		"a register changed":         func(image []byte) { image[10] = 0x43 },
	} {
		t.Run(name, func(t *testing.T) {
			image := append([]byte(nil), linked...)
			edit(image)
			if report := check(image); len(report.unexplained) != 1 {
				t.Fatalf("unexplained %q", report.unexplained)
			}
		})
	}
}

// TestObjectRelocationsPairsTheHalves compiles a call and an indexed global
// and reads the object's relocations and sizes.
func TestObjectRelocationsPairsTheHalves(t *testing.T) {
	requirePinnedToolchain(t)
	t.Setenv("TOOLS_WORK_DIR", "")
	p := validTestProject(t)
	source := filepath.Join(p.root, "src", "battle", "battle_helper.c")
	writeTestFile(t, source, []byte("extern int g_battle_value[];\nextern void battle_other(void);\nint battle_helper(void) {\n    battle_other();\n    return g_battle_value[2];\n}\n"))
	profile, _ := compilerProfileNamed("gcc-2.6.3_O2_aspsx-2.34")
	object, err := p.compileHistoricalObject(source, filepath.Join(p.root, "build", "work", "relocations"), profile)
	if err != nil {
		t.Fatal(err)
	}
	relocations, err := objectRelocations(object)
	if err != nil {
		t.Fatal(err)
	}
	text := relocations[".text"]
	var kinds []string
	high := -1
	for index, r := range text {
		kinds = append(kinds, r.kind.String()+" "+r.name)
		if r.kind == elf.R_MIPS_HI16 {
			high = index
		}
	}
	if strings.Join(kinds, ", ") != "R_MIPS_26 battle_other, R_MIPS_HI16 g_battle_value, R_MIPS_LO16 g_battle_value" {
		t.Fatalf("relocations %q", kinds)
	}
	if pair, ok := pairedHalf(text, high); !ok || text[pair].kind != elf.R_MIPS_LO16 {
		t.Fatalf("pair %d %v", pair, ok)
	}
	if sizes, err := objectSizesOf(object); err != nil || sizes.text == 0 || sizes.rodata != 0 {
		t.Fatalf("sizes %+v %v", sizes, err)
	}
}

// TestGrownConfigForAnEdition fits the disc to an edition's grown BATTLE and
// refuses a move the code names.
func TestGrownConfigForAnEdition(t *testing.T) {
	p := validTestProject(t)
	load := func() *projectConfig {
		config, err := p.loadProjectConfig()
		if err != nil {
			t.Fatal(err)
		}
		last := config.Disc.Entries[len(config.Disc.Entries)-1]
		end := last.LBA + last.sectors(discModuleFiles(config))
		config.Disc.Entries = append(config.Disc.Entries, discEntry{LBA: end, Dummy: 10, Form: 2, Free: true})
		return config
	}
	grow := func(m *moduleSpec) uint32 {
		if m.ID == "battle" {
			return 16
		}
		return 0
	}
	grown, moved, err := p.grownConfig(load(), grow, "edition dc's disc", "the edition cannot give the code a new number yet")
	if err != nil {
		t.Fatal(err)
	}
	if battle, _ := grown.module("battle"); strings.Join(moved, " ") != "BATTLE.BIN" || battle.Size != 4096+16 || grown.discFiles["SCUS_942.21"].sectors != 2 {
		t.Fatalf("moved %q, battle %d bytes, main %+v", moved, battle.Size, grown.discFiles["SCUS_942.21"])
	}
	if err := os.WriteFile(filepath.Join(p.root, "src", "battle", "battle_helper.c"), []byte("void battle_helper(void) { read(DISC_SECTOR_BATTLE_BIN); }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.grownConfig(load(), grow, "edition dc's disc", "the edition cannot give the code a new number yet"); err == nil ||
		!strings.Contains(err.Error(), "BATTLE.BIN would move or grow on edition dc's disc, but the code names its sector (include/fft/disc.h); the edition cannot give the code a new number yet") {
		t.Fatalf("a code-named move: %v", err)
	}
}
