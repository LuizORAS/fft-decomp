package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testRAMMapYAML = `# The test RAM.
windows:
  - {name: main, addr: 0x80010000, end: 0x80067000, modules: [main], why: "the overlays load at 0x80067000"}
  - {name: battle, addr: 0x80067000, end: 0x801bf000, modules: [battle], why: "the companion overlays"}
  - {name: companion, addr: 0x801bf000, end: 0x801df000, modules: [event], why: "the game heap"}

states:
  - {name: battle, windows: [main, battle, companion]}
`

// ramTestProject is validTestProject's modules with ram as target/ram.yaml and
// main as its main module.
func ramTestProject(t *testing.T, main, ram string, more map[string]string) project {
	files := map[string]string{
		"target/main.yaml":           main,
		"target/event.yaml":          testEventYAML,
		"target/battle.yaml":         testBattleYAML,
		ramMapPath:                   ram,
		"src/main/main_first.c":      "void main_first(void) {}\n",
		"src/main/main_second.c":     "void main_second(void) {}\n",
		"src/event/attack_entry.c":   "void attack_entry(void) { g_main_value; g_battle_value; }\n",
		"src/event/card_entry.c":     "void card_entry(void) {}\n",
		"src/battle/battle_helper.c": "void battle_helper(void) {}\n",
	}
	for name, text := range more {
		files[name] = text
	}
	return testProject(t, files)
}

func loadRAMTest(t *testing.T, p project) (*projectConfig, error) {
	t.Helper()
	config, err := p.loadProjectConfig()
	if err != nil {
		t.Fatal(err)
	}
	return config, p.validateProjectConfig(config)
}

func TestRAMMapPlacesModulesInWindows(t *testing.T) {
	config, err := loadRAMTest(t, ramTestProject(t, testMainYAML, testRAMMapYAML, nil))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(renderRAMMap(config.RAM)); got != testRAMMapYAML {
		t.Fatalf("not canonical:\n%s", got)
	}
	if battle, _ := config.module("battle"); battle.ramEnd != 0x801bf000 {
		t.Fatalf("battle's window ends at 0x%08x", battle.ramEnd)
	}
	room := strings.Join(ramRoom(config.RAM, config, nil, config.Modules), "\n")
	for _, line := range []string{
		"  main               0x80010000-0x80067000  354304 bytes left after main",
		"  companion          0x801bf000-0x801df000  130816 bytes left after event-attack",
	} {
		if !strings.Contains(room, line) {
			t.Fatalf("room lacks %q:\n%s", line, room)
		}
	}
}

func TestRAMRoomComparesAnEditionWithTheOriginal(t *testing.T) {
	p := ramTestProject(t, testMainYAML, testRAMMapYAML, nil)
	original, err := loadRAMTest(t, p)
	if err != nil {
		t.Fatal(err)
	}
	grown, err := loadRAMTest(t, p)
	if err != nil {
		t.Fatal(err)
	}
	grown.byID["main"].Size += 16
	grown.byID["event-attack"].Size -= 16
	grown.byID["event-card"].Size -= 16
	room := ramRoom(grown.RAM, grown, original, grown.Modules)
	for _, want := range []string{
		"  main               0x80010000-0x80067000  354288 bytes left after main (16 fewer than the original)",
		"  companion          0x801bf000-0x801df000  130832 bytes left after event-attack (16 more than the original)",
		"  battle             0x80067000-0x801bf000 ",
	} {
		found := false
		for _, line := range room {
			found = found || strings.HasPrefix(line, want) && (strings.Contains(want, "original") || !strings.Contains(line, "original"))
		}
		if !found {
			t.Fatalf("room lacks %q:\n%s", want, strings.Join(room, "\n"))
		}
	}
}

func TestRAMMapRejectsInvalidWindows(t *testing.T) {
	for name, edit := range map[string][3]string{
		"module in no window":   {"modules: [event]", "modules: [event-attack]", "module event-card is in no window"},
		"module in two windows": {"modules: [battle]", "modules: [battle, event-card]", "module event-card is in two windows"},
		"load differs":          {"addr: 0x801bf000", "addr: 0x801bf004", "module event-attack loads at 0x801bf000, but window companion starts at 0x801bf004"},
		"image past the end":    {"end: 0x801bf000, modules: [battle]", "end: 0x80067800, modules: [battle]", "module battle ends at 0x80068000, past window battle's end 0x80067800"},
		"overlap in a state":    {"end: 0x801bf000, modules: [battle]", "end: 0x801c0000, modules: [battle]", "state battle: windows battle and companion overlap"},
		"window in no state":    {"windows: [main, battle, companion]", "windows: [main, battle]", "window companion is in no state"},
		"unknown module":        {"modules: [main]", "modules: [main, nope]", "nope is no module"},
		"unknown window":        {"windows: [main, battle, companion]", "windows: [main, battle, companion, nope]", "state battle: nope is no window"},
		"end before start":      {"end: 0x80067000, modules: [main]", "end: 0x80010000, modules: [main]", "window main ends at 0x80010000, before it starts"},
		"two names alike":       {"{name: battle, addr", "{name: main, addr", "window main needs a new lowercase name"},
		"no why":                {`why: "the game heap"`, `why: ""`, "window companion needs its modules and a why"},
	} {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(testRAMMapYAML, edit[0]) {
				t.Fatalf("edit %q does not apply", edit[0])
			}
			_, err := loadRAMTest(t, ramTestProject(t, testMainYAML, strings.Replace(testRAMMapYAML, edit[0], edit[1], 1), nil))
			if err == nil || !strings.Contains(err.Error(), ramMapPath+": "+edit[2]) {
				t.Fatalf("invalid map accepted: %v", err)
			}
		})
	}
	p := ramTestProject(t, testMainYAML, testRAMMapYAML, nil)
	if err := os.Remove(filepath.Join(p.root, filepath.FromSlash(ramMapPath))); err != nil {
		t.Fatal(err)
	}
	if _, err := loadRAMTest(t, p); err == nil || !strings.Contains(err.Error(), "ram.yaml is missing") {
		t.Fatalf("missing map: %v", err)
	}
}

const testRAMWordsYAML = "\nram:\n  - {addr: 0x80010300, size: companion}\n  - {addr: 0x80010304, start: battle}\n" +
	"  - {addr: 0x80010308, start: companion, from: battle}\n"

func TestRAMWordsTakeTheirValuesFromTheMap(t *testing.T) {
	config, err := loadRAMTest(t, ramTestProject(t, testMainYAML+testRAMWordsYAML, testRAMMapYAML, nil))
	if err != nil {
		t.Fatal(err)
	}
	main, _ := config.module("main")
	var got []uint32
	for _, word := range main.RAMWords {
		if !word.found {
			t.Fatalf("ram word %+v", word)
		}
		got = append(got, word.value)
	}
	if fmt.Sprint(got) != fmt.Sprint([]uint32{0x20000, 0x80067000, 0x158000}) {
		t.Fatalf("ram word values %x", got)
	}
	if string(renderModuleConfig(main)) != testMainYAML+testRAMWordsYAML {
		t.Fatalf("not canonical:\n%s", renderModuleConfig(main))
	}
	for name, edit := range map[string][3]string{
		"unknown window":   {"size: companion}", "size: nope}", "nope is no window"},
		"in code":          {"0x80010300, size", "0x80010000, size", "lies in"},
		"misaligned":       {"0x80010300, size", "0x80010302, size", "not an aligned word"},
		"outside the file": {"0x80010300, size", "0x80020000, size", "not an aligned word"},
		"overlaps a value": {"\nram:", "\nvalues:\n  - {addr: 0x80010300, why: a count}\n\nram:", "repeats a value"},
		"size and start":   {"size: companion}", "size: companion, start: battle}", "takes a size or a start, not both"},
		"neither":          {"start: battle}", "from: battle}", "needs a size or a start"},
		"from after":       {"start: companion, from: battle}", "start: battle, from: companion}", "window companion starts after battle"},
	} {
		t.Run(name, func(t *testing.T) {
			text := testMainYAML + testRAMWordsYAML
			if !strings.Contains(text, edit[0]) {
				t.Fatalf("edit %q does not apply", edit[0])
			}
			_, err := loadRAMTest(t, ramTestProject(t, strings.Replace(text, edit[0], edit[1], 1), testRAMMapYAML, nil))
			if err == nil || !strings.Contains(err.Error(), "ram word") || !strings.Contains(err.Error(), edit[2]) {
				t.Fatalf("invalid ram word accepted: %v", err)
			}
		})
	}
}

// TestRAMWordValueSharesStarts checks the starts a list of windows shares and
// a start measured from another window's.
func TestRAMWordValueSharesStarts(t *testing.T) {
	ram := &ramMap{Windows: []ramWindow{
		{Name: "low", Addr: 0x80067000, End: 0x800c0000},
		{Name: "core", Addr: 0x80067000, End: 0x800e0000},
		{Name: "world", Addr: 0x800e0000, End: 0x801d0000},
	}}
	for _, test := range []struct {
		word    ramWordSpec
		value   uint32
		problem string
	}{
		{ramWordSpec{Start: ramWindowNames{"low", "core"}}, 0x80067000, ""},
		{ramWordSpec{Start: ramWindowNames{"world", "core"}}, 0, "windows world and core start apart"},
		{ramWordSpec{Start: ramWindowNames{"world"}, From: "core"}, 0x79000, ""},
		{ramWordSpec{Start: ramWindowNames{"world", "world"}, From: "core"}, 0, "measures one window's start from another's"},
		{ramWordSpec{Start: ramWindowNames{"world"}, From: "nope"}, 0, "nope is no window in " + ramMapPath},
	} {
		value, problem := ram.wordValue(test.word)
		if value != test.value || problem != test.problem {
			t.Fatalf("%+v: 0x%x %q", test.word, value, problem)
		}
	}
}

func TestRAMHeaderGivesTheSizesTheCodeUses(t *testing.T) {
	p := ramTestProject(t, testMainYAML, testRAMMapYAML, map[string]string{
		"src/main/main_first.c": "#define READ (RAM_WINDOW_BYTES_COMPANION / 2048)\n/* RAM_WINDOW_BYTES_BATTLE */\n" +
			"void main_first(void) { reserve(RAM_WINDOW_BYTES_MAIN, \"RAM_WINDOW_BYTES_NOPE\"); load(RAM_WINDOW_START_COMPANION_FROM_BATTLE); }\n",
	})
	if err := p.configFormat(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(p.root, filepath.FromSlash(ramHeaderPath)))
	if err != nil {
		t.Fatal(err)
	}
	enums := `
typedef enum ram_window_bytes {
    RAM_WINDOW_BYTES_MAIN = 0x57000,
    RAM_WINDOW_BYTES_COMPANION = 0x20000,
} ram_window_bytes_e;

typedef enum ram_window_start {
    RAM_WINDOW_START_COMPANION_FROM_BATTLE = 0x158000,
} ram_window_start_e;

#endif
`
	if !strings.HasPrefix(string(got), "#ifndef RAM_H\n") || !strings.HasSuffix(string(got), enums) {
		t.Fatalf("header:\n%s", got)
	}
	if _, err := loadRAMTest(t, p); err != nil {
		t.Fatal(err)
	}
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(p.root, filepath.FromSlash(name)), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(ramHeaderPath, strings.Replace(string(got), "= 0x20000,", "= 0x20004,", 1))
	if _, err := loadRAMTest(t, p); err == nil || !strings.Contains(err.Error(), "run `make config-fmt`") {
		t.Fatalf("edited header: %v", err)
	}
	write("src/main/main_second.c", "void main_second(void) { reserve(RAM_WINDOW_BYTES_NOPE); }\n")
	if _, err := loadRAMTest(t, p); err == nil || !strings.Contains(err.Error(), "RAM_WINDOW_BYTES_NOPE names no window") {
		t.Fatalf("unknown name: %v", err)
	}
	write("src/main/main_second.c", "void main_second(void) { load(RAM_WINDOW_START_BATTLE_FROM_COMPANION); }\n")
	if _, err := loadRAMTest(t, p); err == nil || !strings.Contains(err.Error(), "RAM_WINDOW_START_BATTLE_FROM_COMPANION names no window") {
		t.Fatalf("a start from a later window: %v", err)
	}
}
