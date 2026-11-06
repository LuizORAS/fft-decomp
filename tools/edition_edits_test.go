package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestEditsReadAndWriteCanonically(t *testing.T) {
	edits, err := parseEdits([]byte("# Data edits.\nedits:\n" +
		"  - {file: EVENT/TEST.EVT, offset: 0x10, bytes: \"01 00\", option: FIX_BUGS, hash: 0123456789abcdef, why: \"second\"}\n" +
		"  - {module: main, addr: 0x80010100, bytes: \"2A\", option: FIX_BUGS, why: first}\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := "# Data edits.\nedits:\n" +
		"  - {module: main, addr: 0x80010100, bytes: \"2a\", option: FIX_BUGS, why: first}\n" +
		"  - {file: EVENT/TEST.EVT, offset: 0x10, bytes: \"01 00\", option: FIX_BUGS, hash: 0123456789abcdef, why: second}\n"
	if got := string(renderEdits(edits)); got != want {
		t.Fatalf("canonical form:\n%s", got)
	}
	empty, err := parseEdits([]byte("edits: []\n"))
	if err != nil || string(renderEdits(empty)) != "edits: []\n" {
		t.Fatalf("empty: %v %q", err, renderEdits(empty))
	}
	for _, bad := range []string{"2g", "abc", "2a0", "0x2a"} {
		if _, err := parseEditBytes(bad); err == nil {
			t.Fatalf("bytes %q read", bad)
		}
	}
	if _, err := parseEdits([]byte("edits:\n  - {module: main, addr: 0x80010100, byte: \"2a\"}\n")); err == nil {
		t.Fatal("an unknown key read")
	}
}

// editsTestConfig is linkTestModule (functions at 0x80100008 and 0x80100010,
// .rodata at 0x80100020) with an interior region, a data region, a
// relocation, a disc word and a ram word, MAIN with its header, and three
// disc files, one of them mixed.
func editsTestConfig() *projectConfig {
	m := linkTestModule()
	m.Regions = []regionSpec{{Addr: 0x80100028, End: 0x8010002c, Kind: "interior"}, {Addr: 0x80100030, End: 0x80100038, Kind: "data"}}
	m.Relocations = []relocationSpec{{Addr: 0x80100004, To: "test_first"}}
	m.DiscWords = []discWordSpec{{Addr: 0x8010003c, Kind: "sector", File: "OPEN/FFTST.STR"}}
	m.RAMWords = []ramWordSpec{{Addr: 0x80100018, Size: "battle"}}
	exe := &moduleSpec{ID: mainModuleID, File: "SCUS_942.21", Load: 0x8000f800, Size: 0x900}
	return &projectConfig{
		Modules: []*moduleSpec{exe, m},
		byID:    map[string]*moduleSpec{exe.ID: exe, m.ID: m},
		discFiles: map[string]discFileFact{"SCUS_942.21": {lba: 24, size: 0x900}, "EVENT/TEST.OUT": {lba: 30, size: 0x40},
			"EVENT/TEST.EVT": {lba: 40, size: 0x100}, "OPEN/FFTST.STR": {lba: 50, size: 0x1000}},
		Disc: &discLayout{Entries: []discEntry{{File: "SCUS_942.21"}, {File: "EVENT/TEST.OUT"}, {File: "EVENT/TEST.EVT"},
			{File: "OPEN/FFTST.STR", Type: "mixed"}}},
		Editions: &editionsFile{Options: []buildOption{{Name: "FIX_BUGS", Why: "a"}, {Name: "LATER", Why: "b"}},
			Editions: []buildEdition{{Name: "dc", Title: "Developer's Cut", Options: []string{"FIX_BUGS"}}}},
	}
}

func TestValidateEditsLocks(t *testing.T) {
	at := func(addr uint32, values ...byte) dataEdit {
		return dataEdit{Module: "event-test", Addr: addr, Bytes: values, Option: "FIX_BUGS", Hash: "0123456789abcdef", Why: "test"}
	}
	in := func(file string, offset uint32, values ...byte) dataEdit {
		return dataEdit{File: file, Offset: offset, Bytes: values, Option: "FIX_BUGS", Hash: "0123456789abcdef", Why: "test"}
	}
	option := func(e dataEdit, name string) dataEdit { e.Option = name; return e }
	unhashed := at(0x80100030, 1)
	unhashed.Hash = ""
	both := at(0x80100030, 1)
	both.File = "EVENT/TEST.EVT"
	header := at(0x8000f810, 1)
	header.Module = mainModuleID
	tests := []struct {
		edits []dataEdit
		want  string // "" when the edits pass
	}{
		{[]dataEdit{at(0x80100030, 1, 2), at(0x80100000, 9), in("EVENT/TEST.EVT", 0xfe, 1, 2)}, ""},
		{[]dataEdit{at(0x8010000a, 1)}, "lies in test_first's code"},
		{[]dataEdit{at(0x80100021, 1)}, "lies in test_second's .rodata"},
		{[]dataEdit{at(0x80100029, 1)}, "lies in the interior region at 0x80100028, which is code"},
		{[]dataEdit{header}, "lies in MAIN's PS-X EXE header"},
		{[]dataEdit{at(0x8010003f+1, 1)}, "lies past the bytes of EVENT/TEST.OUT"},
		{[]dataEdit{at(0x80100006, 1)}, "covers the relocation 0x80100004 (to test_first)"},
		{[]dataEdit{at(0x8010003d, 1)}, "covers the disc word 0x8010003c (sector of OPEN/FFTST.STR)"},
		{[]dataEdit{at(0x80100019, 1)}, "covers the ram word 0x80100018"},
		{[]dataEdit{at(0x80100030, 0x00, 0x00, 0x10, 0x80)}, "writes 0x80100000 at 0x80100030, which reads as a RAM address"},
		{[]dataEdit{at(0x80100030, 1, 2), at(0x80100031, 3)}, "overlaps the edit at event-test 0x80100030"},
		{[]dataEdit{in("EVENT/TEST.OUT", 0, 1)}, "edits module event-test's file"},
		{[]dataEdit{in("OPEN/FFTST.STR", 0, 1)}, "edits a mixed file"},
		{[]dataEdit{in("EVENT/TEST.EVT", 0xff, 1, 2)}, "runs past the end of EVENT/TEST.EVT (256 bytes)"},
		{[]dataEdit{in("EVENT/NOPE.EVT", 0, 1)}, "names no file of target/disc.yaml"},
		{[]dataEdit{option(at(0x80100030, 1), "NOPE")}, "names \"NOPE\", which is no option"},
		{[]dataEdit{option(at(0x80100030, 1), "LATER")}, "names LATER, which no edition"},
		{[]dataEdit{unhashed}, "has no valid hash of the bytes it replaces; run `make checksums`"},
		{[]dataEdit{both}, "names a module and an addr, or a file and an offset"},
		{[]dataEdit{at(0x80100030)}, "writes no bytes"},
	}
	for _, test := range tests {
		config := editsTestConfig()
		config.Edits = &editsFile{Edits: test.edits}
		var problems []string
		validateEdits(config, func(format string, args ...any) {
			problems = append(problems, strings.TrimPrefix(fmt.Sprintf(format, args...), editsPath+": "))
		})
		got := strings.Join(problems, "\n")
		if (test.want == "") != (got == "") || !strings.Contains(got, test.want) {
			t.Fatalf("edits %v: want %q, got:\n%s", test.edits, test.want, got)
		}
	}
}

func TestLinkDataAssemblyWritesEdits(t *testing.T) {
	m := linkTestModule()
	layout, err := moduleLinkLayout(m)
	if err != nil {
		t.Fatal(err)
	}
	layout.edits = []dataEdit{{Module: m.ID, Addr: 0x80100002, Bytes: []byte{0x2a, 0x00, 0xff, 0x01}}, {Module: m.ID, Addr: 0x80100030, Bytes: []byte{0x07}}}
	data := linkDataAssembly(m, layout, "/disc/EVENT/TEST.OUT")
	for _, line := range []string{
		"\t.section .bytes_80100000, \"a\", @progbits\n\t.globl g_test_a\ng_test_a:\n\t.incbin \"/disc/EVENT/TEST.OUT\", 0, 2\n\t.byte 0x2a, 0x00\n" +
			"\t.globl g_test_b\ng_test_b:\n\t.byte 0xff, 0x01\n\t.incbin \"/disc/EVENT/TEST.OUT\", 6, 2\n",
		"\t.incbin \"/disc/EVENT/TEST.OUT\", 36, 12\n\t.globl g_test_tail\ng_test_tail:\n\t.byte 0x07\n\t.incbin \"/disc/EVENT/TEST.OUT\", 49, 15\n",
	} {
		if !strings.Contains(data, line) {
			t.Fatalf("data assembly lacks %q:\n%s", line, data)
		}
	}
	if zeroed := linkDataAssembly(m, layout, ""); !strings.Contains(zeroed, "g_test_a:\n\t.space 2\n\t.byte 0x2a, 0x00\n") {
		t.Fatalf("zeroed data assembly:\n%s", zeroed)
	}
}
