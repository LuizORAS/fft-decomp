package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSuzukiSmallDataProfile(t *testing.T) {
	profile, ok := compilerProfileNamed("gcc-2.6.3_O2_G8_aspsx-2.34")
	if !ok || profile.smallDataLimit != 8 || profile.globalPointer != 0x800329bc {
		t.Fatalf("invalid Suzuki profile: %#v", profile)
	}
	standard, ok := compilerProfileNamed(defaultProfile)
	if !ok || standard.smallDataLimit != 0 || standard.globalPointer != 0 {
		t.Fatalf("standard profile changed: %#v", standard)
	}
}

func TestSmallDataLoadDelayHazards(t *testing.T) {
	for _, test := range []struct {
		name    string
		text    string
		wantNop bool
	}{
		{"extern-word", ".extern state,4\nlw $2,12($2)\n.loc 1 4\nLM4:\nsw $2,state\n", true},
		{"extern-halfword", ".extern state,2\nlhu $3,0($4)\nsh $3,state\n", true},
		{"large-extern", ".extern state,12\nlw $2,12($2)\nsw $2,state\n", false},
		{"unrelated-register", ".extern state,4\nlw $2,12($2)\nsw $3,state\n", false},
		{"existing-delay", ".extern state,4\nlw $2,12($2)\nnop\nsw $2,state\n", false},
		{"intervening-instruction", ".extern state,4\nlw $2,12($2)\naddu $4,$4,4\nsw $2,state\n", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := string(smallDataLoadDelays([]byte(test.text), 8))
			if inserted := strings.Contains(got, "nop # GPREL"); inserted != test.wantNop {
				t.Fatalf("inserted = %v, want %v: %s", inserted, test.wantNop, got)
			}
		})
	}
}

func TestSmallDataPointerStorePipeline(t *testing.T) {
	requirePinnedToolchain(t)
	root := t.TempDir()
	p := project{root: root}
	source := filepath.Join(root, "fixture.c")
	code := "struct item { int value; struct item *next; };\nextern struct item *fixture_small;\nvoid fixture(void) { fixture_small = fixture_small->next; }\n"
	if err := os.WriteFile(source, []byte(code), 0o600); err != nil {
		t.Fatal(err)
	}
	profile, _ := compilerProfileNamed("gcc-2.6.3_O2_G8_aspsx-2.34")
	script, err := linkerScript(&functionSpec{Name: "fixture", Addr: 0x8001423c}, map[string]uint32{
		"fixture_small": 0x800329f8, "_gp": profile.globalPointer,
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	linker := filepath.Join(root, "fixture.ld")
	if err := os.WriteFile(linker, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TOOLS_CACHE", "0")
	sections, err := p.compileHistoricalSections(source, linker, filepath.Join(root, "build"), profile)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{
		0x3c, 0, 0x82, 0x8f, 0, 0, 0, 0,
		4, 0, 0x42, 0x8c, 0, 0, 0, 0,
		0x3c, 0, 0x82, 0xaf, 8, 0, 0xe0, 3, 0, 0, 0, 0,
	}
	if !bytes.Equal(sections[".text"], want) {
		t.Fatalf("pointer-store bytes = %x, want %x", sections[".text"], want)
	}
}

func TestSmallDataRelocationPipeline(t *testing.T) {
	requirePinnedToolchain(t)
	root := t.TempDir()
	p := project{root: root}
	source := filepath.Join(root, "fixture.c")
	if err := os.WriteFile(source, []byte("extern int fixture_small;\nint fixture(void) { return fixture_small; }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	profile, _ := compilerProfileNamed("gcc-2.6.3_O2_G8_aspsx-2.34")
	script, err := linkerScript(&functionSpec{Name: "fixture", Addr: 0x8001423c}, map[string]uint32{
		"fixture_small": 0x800329f8, "_gp": profile.globalPointer,
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	linker := filepath.Join(root, "fixture.ld")
	if err := os.WriteFile(linker, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TOOLS_CACHE", "0")
	sections, err := p.compileHistoricalSections(source, linker, filepath.Join(root, "build"), profile)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0x3c, 0x00, 0x82, 0x8f, 0x08, 0x00, 0xe0, 0x03, 0, 0, 0, 0}
	if !bytes.Equal(sections[".text"], want) {
		t.Fatalf("small-data load bytes = %x, want %x", sections[".text"], want)
	}
}
