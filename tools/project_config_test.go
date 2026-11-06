package main

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// testProject writes target files and sources into a temporary project. A
// project with module files and no target/disc.yaml or target/ram.yaml gets
// them from testDiscYAML and testRAMYAML.
func testProject(t *testing.T, files map[string]string) project {
	t.Helper()
	root := t.TempDir()
	if _, ok := files[discLayoutPath]; !ok {
		if disc := testDiscYAML(files); disc != "" {
			files = maps.Clone(files)
			files[discLayoutPath] = disc
		}
	}
	if _, ok := files[ramMapPath]; !ok {
		if ram := testRAMYAML(files); ram != "" {
			files = maps.Clone(files)
			files[ramMapPath] = ram
		}
	}
	for path, content := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return project{root: root, filesRoot: filepath.Join(root, "build", "extracted", "files")}
}

const testDiscHeaderYAML = `license: license_data.dat
volume: {system: PLAYSTATION, application: PLAYSTATION, volume: TEST, volume_set: TEST, publisher: TEST, data_preparer: TEST, creation_date: "1997101718372100+36", modification_date: "1997101718372100+36"}
attributes: {gmt_offs: 36, xa_attrib: 56, xa_perm: 1, xa_gid: 10752, xa_uid: 10752, hidden: 1}
root: {lba: 22, xa_perm: 1365, xa_gid: 0, xa_uid: 0, hidden: 0}
`

// testDiscYAML lays out the module files the target fixtures name one after
// another from sector 23 in path order, each directory before its first file.
func testDiscYAML(files map[string]string) string {
	type moduleFile struct {
		File string `yaml:"file"`
		Size int    `yaml:"size"`
	}
	var modules []moduleFile
	for name, content := range files {
		if !strings.HasPrefix(name, modulesConfigDir+"/") {
			continue
		}
		for _, document := range splitYAMLDocuments([]byte(content)) {
			var header moduleFile
			if yaml.Unmarshal(document, &header) == nil && header.File != "" {
				modules = append(modules, header)
			}
		}
	}
	if len(modules) == 0 {
		return ""
	}
	sort.Slice(modules, func(i, j int) bool { return modules[i].File < modules[j].File })
	var out strings.Builder
	out.WriteString(testDiscHeaderYAML + "\nentries:\n")
	lba, dirs := 23, map[string]bool{}
	for _, m := range modules {
		if dir := parentDiscPath(m.File); dir != "" && !dirs[dir] {
			fmt.Fprintf(&out, "  - {lba: %d, dir: %s, date: 19971017183721}\n", lba, dir)
			lba, dirs[dir] = lba+1, true
		}
		fmt.Fprintf(&out, "  - {lba: %d, file: %s, date: 19971017183704}\n", lba, m.File)
		lba += max(1, (m.Size+discDataSectorBytes-1)/discDataSectorBytes)
	}
	return out.String()
}

// testRAMYAML gives each module the target fixtures name a window of its own,
// from its RAM start to its end, in a state of its own.
func testRAMYAML(files map[string]string) string {
	type moduleHeader struct {
		Module string `yaml:"module"`
		Size   int    `yaml:"size"`
		Load   uint32 `yaml:"load"`
	}
	ram := &ramMap{}
	for name, content := range files {
		if !strings.HasPrefix(name, modulesConfigDir+"/") || name == discLayoutPath {
			continue
		}
		for _, document := range splitYAMLDocuments([]byte(content)) {
			var header moduleHeader
			if yaml.Unmarshal(document, &header) != nil || header.Module == "" {
				continue
			}
			start, end := moduleRAM(&moduleSpec{ID: header.Module, Load: header.Load, Size: header.Size})
			ram.Windows = append(ram.Windows, ramWindow{
				Name: strings.ReplaceAll(header.Module, "-", "_"), Addr: start, End: max(end, start+4),
				Modules: []string{header.Module}, Why: "the test",
			})
		}
	}
	if len(ram.Windows) == 0 {
		return ""
	}
	sort.Slice(ram.Windows, func(i, j int) bool { return ram.Windows[i].Name < ram.Windows[j].Name })
	for _, window := range ram.Windows {
		ram.States = append(ram.States, ramState{Name: window.Name, Windows: []string{window.Name}})
	}
	return string(renderRAMMap(ram))
}

const testMainYAML = `# The main executable.
module: main
file: SCUS_942.21
size: 4096
sha256: 00000000000000000000000000000000000000000000000000000000000000aa
load: 0x8000f800
source_dir: src/main
profile: gcc-2.6.3_O2_aspsx-2.34

functions:
  - {addr: 0x80010000, size: 8, name: main_first, hash: 0123456789abcdef}
  - {addr: 0x80010008, size: 4, name: main_second, hash: 123456789012345a, profile: gcc-2.6.3_O1_aspsx-2.34}

data:
  - {addr: 0x80010100, name: g_main_value}
  - {addr: 0x80010100, name: g_main_value_alias}
  - {addr: 0x80010200, name: memset}
`

const testEventYAML = `module: event-attack
file: EVENT/ATTACK.OUT
size: 256
sha256: 11111111111111111111111111111111111111111111111111111111111111bb
load: 0x801bf000
source_dir: src/event
profile: gcc-2.6.3_O2_aspsx-2.34
links: [battle]

functions:
  - {addr: 0x801bf000, size: 4, name: attack_entry, hash: 00000000000000aa}

imports:
  - {addr: 0x80020000, name: g_main_value}

overrides:
  - {function: attack_entry, name: battle_helper, addr: 0x80070000}
---
module: event-card
file: EVENT/CARD.OUT
size: 256
sha256: 22222222222222222222222222222222222222222222222222222222222222cc
load: 0x801bf000
source_dir: src/event
profile: gcc-2.6.3_O0_aspsx-2.34

functions:
  - {addr: 0x801bf000, size: 4, name: card_entry, hash: 00000000000000bb}
`

const testBattleYAML = `module: battle
file: BATTLE.BIN
size: 4096
sha256: 33333333333333333333333333333333333333333333333333333333333333dd
load: 0x80067000
source_dir: src/battle
profile: gcc-2.6.3_O2_aspsx-2.34

functions:
  - {addr: 0x80067000, size: 4, name: battle_helper, hash: 00000000000000cc}

data:
  - {addr: 0x80067100, name: g_battle_value}
  - {addr: 0x80067104, name: g_main_value}
`

func validTestProject(t *testing.T) project {
	return testProject(t, map[string]string{
		"target/main.yaml":           testMainYAML,
		"target/event.yaml":          testEventYAML,
		"target/battle.yaml":         testBattleYAML,
		"src/main/main_first.c":      "void main_first(void) {}\n",
		"src/main/main_second.c":     "void main_second(void) {}\n",
		"src/event/attack_entry.c":   "void attack_entry(void) { g_main_value; g_battle_value; }\n",
		"src/event/card_entry.c":     "void card_entry(void) {}\n",
		"src/battle/battle_helper.c": "void battle_helper(void) {}\n",
	})
}

func TestProjectConfigLoadsMultiDocumentFiles(t *testing.T) {
	p := validTestProject(t)
	config, err := p.loadProjectConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Modules) != 4 {
		t.Fatalf("modules = %d", len(config.Modules))
	}
	card, ok := config.module("event-card")
	if !ok || card.Profile != "gcc-2.6.3_O0_aspsx-2.34" || card.path != "target/event.yaml" {
		t.Fatalf("event-card = %+v", card)
	}
	main, _ := config.module("main")
	if main.Comment != "# The main executable." || main.offset(0x80010000) != 0x800 {
		t.Fatalf("main comment/offset: %q %#x", main.Comment, main.offset(0x80010000))
	}
	if got := main.profileOf(&main.Functions[1]); got != "gcc-2.6.3_O1_aspsx-2.34" {
		t.Fatalf("function profile = %s", got)
	}
	if got := main.sourceOf(&main.Functions[0]); got != "src/main/main_first.c" {
		t.Fatalf("default source = %s", got)
	}
	if err := p.validateProjectConfig(config); err != nil {
		t.Fatal(err)
	}
}

func TestProjectConfigRendersCanonicalFormUnchanged(t *testing.T) {
	p := validTestProject(t)
	config, err := p.loadProjectConfig()
	if err != nil {
		t.Fatal(err)
	}
	for path, data := range renderConfigFiles(config) {
		if string(data) != string(config.files[path]) {
			t.Fatalf("%s is not stable:\n%s", path, data)
		}
	}
}

func TestProjectConfigRejectsNonCanonicalAndInvalidFiles(t *testing.T) {
	for name, edit := range map[string]func(string) string{
		"quoted address": func(s string) string {
			return strings.Replace(s, "addr: 0x80010000,", `addr: "0x80010000",`, 1)
		},
		"unsorted functions": func(s string) string {
			first := "  - {addr: 0x80010000, size: 8, name: main_first, hash: 0123456789abcdef}\n"
			return strings.Replace(strings.Replace(s, first, "", 1), "\ndata:", first+"\ndata:", 1)
		},
		"missing hash": func(s string) string {
			return strings.Replace(s, ", hash: 0123456789abcdef", "", 1)
		},
		"overlap": func(s string) string {
			return strings.Replace(s, "size: 8, name: main_first", "size: 12, name: main_first", 1)
		},
		"default profile spelled out": func(s string) string {
			return strings.Replace(s, "hash: 0123456789abcdef}", "hash: 0123456789abcdef, profile: gcc-2.6.3_O2_aspsx-2.34}", 1)
		},
	} {
		t.Run(name, func(t *testing.T) {
			p := validTestProject(t)
			path := filepath.Join(p.root, "target", "main.yaml")
			if err := os.WriteFile(path, []byte(edit(testMainYAML)), 0o644); err != nil {
				t.Fatal(err)
			}
			config, err := p.loadProjectConfig()
			if err == nil {
				err = p.validateProjectConfig(config)
			}
			if err == nil {
				t.Fatal("invalid configuration was accepted")
			}
		})
	}
}

func TestProjectConfigRejectsUnknownKeysAndMisplacedModules(t *testing.T) {
	p := validTestProject(t)
	path := filepath.Join(p.root, "target", "main.yaml")
	if err := os.WriteFile(path, []byte(strings.Replace(testMainYAML, "size: 4096", "size: 4096\nbytes: 4", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.loadProjectConfig(); err == nil || !strings.Contains(err.Error(), "bytes") {
		t.Fatalf("unknown key error = %v", err)
	}
	if err := os.WriteFile(path, []byte(testMainYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.root, "target", "battle.yaml"), []byte(strings.Replace(testBattleYAML, "module: battle", "module: event-x", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.loadProjectConfig(); err == nil || !strings.Contains(err.Error(), "target/event.yaml") {
		t.Fatalf("misplaced module error = %v", err)
	}
}

func TestProjectConfigValidatesRelocationsAndValues(t *testing.T) {
	valid := "\nrelocations:\n  - {addr: 0x80010108, to: g_main_value, offset: 0x4}\n  - {addr: 0x80010104, to: main_second}\n" +
		"\nvalues:\n  - {addr: 0x80010120, end: 0x80010130, why: \"a table of constants\"}\n  - {addr: 0x80010114, end: 0x80010118, why: \"one word\"}\n"
	canonical := "\nrelocations:\n  - {addr: 0x80010104, to: main_second}\n  - {addr: 0x80010108, to: g_main_value, offset: 0x4}\n" +
		"\nvalues:\n  - {addr: 0x80010114, why: \"one word\"}\n  - {addr: 0x80010120, end: 0x80010130, why: \"a table of constants\"}\n"
	for name, testCase := range map[string]struct {
		sections, want string
	}{
		"valid":             {valid, ""},
		"misaligned":        {"\nrelocations:\n  - {addr: 0x80010102, to: main_second}\n", "is not an aligned word"},
		"in a function":     {"\nrelocations:\n  - {addr: 0x80010004, to: main_second}\n", "lies in main_first"},
		"in the header":     {"\nrelocations:\n  - {addr: 0x8000f900, to: main_second}\n", "lies in the PS-X EXE header"},
		"unknown name":      {"\nrelocations:\n  - {addr: 0x80010104, to: g_nowhere}\n", "g_nowhere is no name this module resolves"},
		"twice":             {"\nrelocations:\n  - {addr: 0x80010104, to: main_second}\n\nvalues:\n  - {addr: 0x80010100, end: 0x80010110, why: \"x\"}\n", "overlaps a relocation at 0x80010104"},
		"value with no why": {"\nvalues:\n  - {addr: 0x80010110, why: \" \"}\n", "needs a why"},
		"empty range":       {"\nvalues:\n  - {addr: 0x80010120, end: 0x80010120, why: \"x\"}\n", "is not an aligned range"},
	} {
		t.Run(name, func(t *testing.T) {
			p := validTestProject(t)
			path := filepath.Join(p.root, "target", "main.yaml")
			if err := os.WriteFile(path, []byte(testMainYAML+testCase.sections), 0o644); err != nil {
				t.Fatal(err)
			}
			config, err := p.loadProjectConfig()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := p.writeProjectConfig(config); err != nil {
				t.Fatal(err)
			}
			if config, err = p.loadProjectConfig(); err != nil {
				t.Fatal(err)
			}
			err = p.validateProjectConfig(config)
			if testCase.want == "" {
				written, readErr := os.ReadFile(path)
				if err != nil || readErr != nil || !strings.HasSuffix(string(written), canonical) {
					t.Fatalf("valid sections: %v %v\n%s", err, readErr, written)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("error = %v, want %q", err, testCase.want)
			}
		})
	}
}

func TestProjectConfigValidatesDataPages(t *testing.T) {
	const row = "  - {addr: 0x80010100, name: g_main_value}\n"
	for name, testCase := range map[string]struct {
		row, want string
	}{
		"valid":            {"  - {addr: 0x80010100, name: g_main_value, page: 0x80010000}\n", ""},
		"another page":     {"  - {addr: 0x80010100, name: g_main_value, page: 0x80020000}\n", "data g_main_value: page 0x80020000 is not 0x80010000, its address's"},
		"outside the file": {row + "  - {addr: 0x80028000, name: g_main_far, page: 0x80030000}\n", "data g_main_far: only a row in the module's file takes a page"},
	} {
		t.Run(name, func(t *testing.T) {
			p := validTestProject(t)
			path := filepath.Join(p.root, "target", "main.yaml")
			if err := os.WriteFile(path, []byte(strings.Replace(testMainYAML, row, testCase.row, 1)), 0o644); err != nil {
				t.Fatal(err)
			}
			config, err := p.loadProjectConfig()
			if err != nil {
				t.Fatal(err)
			}
			err = p.validateProjectConfig(config)
			if testCase.want == "" {
				main, _ := config.module("main")
				if err != nil || !strings.Contains(string(renderModuleConfig(main)), testCase.row) {
					t.Fatalf("valid page: %v\n%s", err, renderModuleConfig(main))
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("error = %v, want %q", err, testCase.want)
			}
		})
	}
}

func TestProjectConfigValidatesRegionsSourcesAndAssembly(t *testing.T) {
	for name, testCase := range map[string]struct {
		edit func(string) string
		want string
	}{
		"blocked region over a function": {
			func(s string) string {
				return s + "\nregions:\n  - {addr: 0x80010000, end: 0x80010008, kind: blocked, why: \"stale\"}\n"
			},
			"overlaps matching function main_first",
		},
		"unknown region kind": {
			func(s string) string {
				return s + "\nregions:\n  - {addr: 0x80010100, end: 0x80010108, kind: mystery, why: \"x\"}\n"
			},
			"known kind",
		},
		"missing source": {
			func(s string) string {
				return strings.Replace(s, "name: main_first,", "name: main_missing,", 1)
			},
			"main_missing",
		},
	} {
		t.Run(name, func(t *testing.T) {
			p := validTestProject(t)
			if err := os.WriteFile(filepath.Join(p.root, "target", "main.yaml"), []byte(testCase.edit(testMainYAML)), 0o644); err != nil {
				t.Fatal(err)
			}
			config, err := p.loadProjectConfig()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := p.writeProjectConfig(config); err != nil {
				t.Fatal(err)
			}
			if config, err = p.loadProjectConfig(); err != nil {
				t.Fatal(err)
			}
			err = p.validateProjectConfig(config)
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("error = %v, want %q", err, testCase.want)
			}
		})
	}
}

func TestYAMLTextQuotesAmbiguousScalars(t *testing.T) {
	for value, want := range map[string]string{
		"battle_helper":    "battle_helper",
		"0123456789abcdef": "0123456789abcdef",
		"1234567890123456": `"1234567890123456"`,
		"12345678901234e5": `"12345678901234e5"`,
		"true":             `"true"`,
		"LIBGPU":           "LIBGPU",
		"two words":        `"two words"`,
		`say "a", b`:       `"say \"a\", b"`,
		"EVENT/ATTACK.OUT": "EVENT/ATTACK.OUT",
	} {
		if got := yamlText(value); got != want {
			t.Errorf("yamlText(%q) = %s, want %s", value, got, want)
		}
	}
}

func TestSymbolResolverSearchesOwnLinksThenMain(t *testing.T) {
	p := validTestProject(t)
	config, err := p.loadProjectConfig()
	if err != nil {
		t.Fatal(err)
	}
	resolver := newSymbolResolver(config)
	attack, _ := config.module("event-attack")
	card, _ := config.module("event-card")
	for _, testCase := range []struct {
		module *moduleSpec
		name   string
		want   uint32
		found  bool
	}{
		{attack, "g_main_value", 0x80020000, true},   // own import first
		{attack, "g_battle_value", 0x80067100, true}, // linked module
		{card, "g_main_value", 0x80010100, true},     // main last
		{card, "g_battle_value", 0, false},           // not linked
		{attack, "attack_entry", 0x801bf000, true},   // own function
		{attack, "main_first", 0x80010000, true},     // main function
	} {
		got, found := resolver.resolve(testCase.module, testCase.name)
		if found != testCase.found || got != testCase.want {
			t.Errorf("%s %s = %#x %v", testCase.module.ID, testCase.name, got, found)
		}
	}
	used := map[string]struct{}{"g_main_value": {}, "g_battle_value": {}, "not_a_symbol": {}}
	bindings := resolver.linkerBindings(attack, &attack.Functions[0], used)
	if len(bindings) != 3 || bindings["battle_helper"] != 0x80070000 || bindings["g_main_value"] != 0x80020000 {
		t.Fatalf("bindings = %v", bindings)
	}
	main, _ := config.module("main")
	bindings = resolver.linkerBindings(main, &main.Functions[0], map[string]struct{}{})
	if len(bindings) != 1 || bindings["memset"] != 0x80010200 {
		t.Fatalf("compiler-emitted bindings = %v", bindings)
	}
}

func TestLinkerScriptPlacesSections(t *testing.T) {
	f := &functionSpec{Addr: 0x80067010, Name: "f", Rodata: []rodataSpec{{Addr: 0x80067000, Size: 8}}}
	script, err := linkerScript(f, map[string]uint32{"b": 0x80010000, "a": 0x80020000}, false)
	if err != nil {
		t.Fatal(err)
	}
	want := "OUTPUT_ARCH(mips)\nPROVIDE(a = 0x80020000);\nPROVIDE(b = 0x80010000);\nSECTIONS {\n" +
		"  .rodata 0x80067000 : SUBALIGN(4) { *(.rodata) *(.rdata) }\n" +
		"  .text 0x80067010 : SUBALIGN(4) { *(.text) }\n" +
		"  /DISCARD/ : { *(.reginfo) *(.pdr) *(.comment) *(.gnu.attributes) }\n}\n"
	if script != want {
		t.Fatalf("script:\n%s", script)
	}
	f.Rodata = append(f.Rodata, rodataSpec{Addr: 0x80067008, Size: 4})
	if _, err := linkerScript(f, nil, false); err == nil {
		t.Fatal("two rodata ranges were accepted")
	}
}

func TestResolveModuleNameAcceptsShortNames(t *testing.T) {
	p := validTestProject(t)
	config, err := p.loadProjectConfig()
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"main": "main", "attack": "event-attack", "event-card": "event-card"} {
		m, err := resolveModuleName(config, name)
		if err != nil || m.ID != want {
			t.Errorf("%s = %v, %v", name, m, err)
		}
	}
	if _, err := resolveModuleName(config, "nope"); err == nil {
		t.Fatal("unknown module accepted")
	}
}

func TestShortHashIsSixteenHexDigits(t *testing.T) {
	if got := shortHash([]byte("abc")); got != "ba7816bf8f01cfea" || !validHash(got) {
		t.Fatalf("shortHash = %s", got)
	}
}

func TestCompilerProfiles(t *testing.T) {
	o0, ok := compilerProfileNamed("gcc-2.6.3_O0_aspsx-2.34")
	if !ok {
		t.Fatal("O0 profile is not registered")
	}
	o2, ok := compilerProfileNamed(defaultProfile)
	if !ok {
		t.Fatal("canonical O2 profile is not registered")
	}
	if o0.compilerPath != o2.compilerPath || o0.aspsxVersion != o2.aspsxVersion {
		t.Fatal("O0 profile must differ from O2 only by optimization")
	}
	if o0.optimization != "-O0" || o2.optimization != "-O2" {
		t.Fatalf("optimizations = %q, %q", o0.optimization, o2.optimization)
	}
	if _, ok := compilerProfileNamed("unknown"); ok {
		t.Fatal("unknown profile unexpectedly resolved")
	}

	divcheck, ok := compilerProfileNamed("gcc-2.6.3_O2_aspsx-2.34_divcheck")
	if !ok {
		t.Fatal("checked-division profile is not registered")
	}
	if divcheck.compilerPath != o2.compilerPath ||
		divcheck.aspsxVersion != o2.aspsxVersion ||
		divcheck.optimization != o2.optimization {
		t.Fatal("divcheck profile must differ from the canonical one only by expandDiv")
	}
	if !divcheck.expandDiv {
		t.Fatal("divcheck profile must request the checked division expansion")
	}
	legacyO2, ok := compilerProfileNamed("gcc-2.6.3_O2_aspsx-2.21")
	if !ok {
		t.Fatal("ASPSX 2.21 profile is not registered")
	}
	legacyDivcheck, ok := compilerProfileNamed("gcc-2.6.3_O2_aspsx-2.21_divcheck")
	if !ok {
		t.Fatal("ASPSX 2.21 checked-division profile is not registered")
	}
	if legacyDivcheck.compilerPath != legacyO2.compilerPath ||
		legacyDivcheck.aspsxVersion != legacyO2.aspsxVersion ||
		legacyDivcheck.optimization != legacyO2.optimization {
		t.Fatal("ASPSX 2.21 divcheck profile must differ only by expandDiv")
	}
	if !legacyDivcheck.expandDiv {
		t.Fatal("ASPSX 2.21 divcheck profile must request checked division")
	}
	// The target mixes checked and unchecked divisions: never the default.
	for _, profile := range []compilerProfile{o0, o2} {
		if profile.expandDiv {
			t.Fatalf("%s must not expand divisions", profile.name)
		}
	}
}

// The checked-division expansion changes emitted bytes, so it must take part in
// the compilation cache identity or a profile switch would reuse stale objects.
func TestCompilationCacheKeyCoversExpandDiv(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "input.i")
	if err := os.WriteFile(source, []byte("int main(void){return 0;}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	plain, ok := compilerProfileNamed(defaultProfile)
	if !ok {
		t.Fatal("canonical profile is not registered")
	}
	expanded := plain
	expanded.expandDiv = true

	first, err := compilationCacheKey(source, "", plain, "C")
	if err != nil {
		t.Skipf("cache key unavailable in this environment: %v", err)
	}
	second, err := compilationCacheKey(source, "", expanded, "C")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("expandDiv does not affect the compilation cache key")
	}
	// A profile without the flag keeps its existing cache identity.
	again, err := compilationCacheKey(source, "", plain, "C")
	if err != nil {
		t.Fatal(err)
	}
	if again != first {
		t.Fatal("cache key for an unchanged profile is not stable")
	}
}
