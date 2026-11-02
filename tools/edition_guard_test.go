package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestGuardProblems(t *testing.T) {
	for _, test := range []struct {
		name              string
		declared, visible []string
		changed           map[string]bool
		together          bool
		want              []string
	}{
		{"unchanged", nil, []string{"FIX_BUGS"}, map[string]bool{}, false, nil},
		{"declared change", []string{"FIX_BUGS"}, []string{"FIX_BUGS"}, map[string]bool{"FIX_BUGS": true}, true, nil},
		{"undeclared change", nil, []string{"FIX_BUGS", "NON_MATCHING"}, map[string]bool{"NON_MATCHING": true}, true,
			[]string{"changes under NON_MATCHING, which its change does not list"}},
		{"declared without change", []string{"FIX_BUGS"}, []string{"FIX_BUGS"}, map[string]bool{}, false,
			[]string{"declares FIX_BUGS, which does not change it"}},
		{"declared option out of sight", []string{"FIX_BUGS"}, nil, nil, false,
			[]string{"declares FIX_BUGS, which neither its source nor a header it includes names"}},
		{"another option than declared", []string{"FIX_BUGS"}, []string{"FIX_BUGS", "NON_MATCHING"}, map[string]bool{"NON_MATCHING": true}, true,
			[]string{"declares FIX_BUGS, which does not change it", "changes under NON_MATCHING, which its change does not list"}},
		{"options only together", nil, []string{"FIX_BUGS", "NON_MATCHING"}, map[string]bool{}, true,
			[]string{"compiles differently with FIX_BUGS and NON_MATCHING together than with each alone; give each option its own #ifdef block"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := guardProblems(test.declared, test.visible, test.changed, test.together); !slices.Equal(got, test.want) {
				t.Fatalf("problems %q, want %q", got, test.want)
			}
		})
	}
}

func TestGuardScanFollowsIncludes(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "include", "fft", "outer.h"), []byte("#include \"inner.h\"\n"))
	writeTestFile(t, filepath.Join(root, "include", "fft", "inner.h"), []byte("/* NON_MATCHING spelling */\n#ifdef FIX_BUGS\n#endif\n"))
	writeTestFile(t, filepath.Join(root, "src", "main", "a.c"), []byte("#include \"fft/outer.h\"\nint a;\n"))
	writeTestFile(t, filepath.Join(root, "src", "main", "b.c"), []byte("#include \"psx/types.h\"\nint FIX_BUGS_COUNT;\n"))
	writeTestFile(t, filepath.Join(root, "include", "psx", "types.h"), []byte("typedef int s32;\n"))
	writeTestFile(t, filepath.Join(root, "src", "main", "c.c"), []byte("#include \"missing.h\"\n"))
	scan := newGuardScan(project{root: root, options: []string{"FIX_BUGS", "IMPROVEMENTS", "NON_MATCHING"}})
	for source, want := range map[string][]string{
		"a.c": {"FIX_BUGS", "NON_MATCHING"},
		"b.c": nil,
		"c.c": {"FIX_BUGS", "IMPROVEMENTS", "NON_MATCHING"},
	} {
		got, err := scan.visible(filepath.Join(root, "src", "main", source))
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("%s sees %v (%v), want %v", source, got, err, want)
		}
	}
}

func TestChangeRowsRenderAndValidate(t *testing.T) {
	p := validTestProject(t)
	writeTestFile(t, filepath.Join(p.root, "target", "editions.yaml"), []byte(testEditionsYAML))
	main := strings.Replace(testMainYAML, "hash: 0123456789abcdef}", "hash: 0123456789abcdef, change: [FIX_BUGS, NON_MATCHING], dc_hash: 00000000000000ff}", 1)
	writeTestFile(t, filepath.Join(p.root, "target", "main.yaml"), []byte(main))
	config, err := p.loadProjectConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got := string(renderConfigFiles(config)["target/main.yaml"]); got != main {
		t.Fatalf("canonical form:\n%s", got)
	}
	if err := p.validateProjectConfig(config); err != nil {
		t.Fatal(err)
	}
	editions, err := parseEditions([]byte(strings.Replace(testEditionsYAML, "\neditions:", "  - {name: PORT, why: \"a port\"}\n\neditions:", 1)))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		f    functionSpec
		want string
	}{
		{"unknown option", functionSpec{Change: []string{"FIX_BUG"}, DCHash: "00000000000000ff"}, "FIX_BUG, which is no option"},
		{"option no edition builds", functionSpec{Change: []string{"PORT"}}, "PORT, which no edition"},
		{"option twice", functionSpec{Change: []string{"FIX_BUGS", "FIX_BUGS"}, DCHash: "00000000000000ff"}, "FIX_BUGS twice"},
		{"no dc_hash", functionSpec{Change: []string{"FIX_BUGS"}}, "no valid dc_hash; run `make checksums EDITION=dc`"},
		{"dc_hash without change", functionSpec{DCHash: "00000000000000ff"}, "has a dc_hash but no change"},
		{"assembly", functionSpec{Asm: "replaces sp", Change: []string{"FIX_BUGS"}, DCHash: "00000000000000ff"}, "takes no change"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var problems []string
			validateChange(editions, &test.f, "f", func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) })
			if !strings.Contains(strings.Join(problems, "\n"), test.want) {
				t.Fatalf("problems %q, want text %q", problems, test.want)
			}
		})
	}
}

func TestEditionGuardComparesEachOption(t *testing.T) {
	requirePinnedToolchain(t)
	t.Setenv("TOOLS_WORK_DIR", "")
	p := validTestProject(t)
	writeTestFile(t, filepath.Join(p.root, "target", "editions.yaml"), []byte(testEditionsYAML))
	sources := map[string]string{
		// A fix under one option.
		"src/main/main_first.c": "int main_first(void) {\n#ifdef FIX_BUGS\n    return 1;\n#else\n    return 0;\n#endif\n}\n",
		// Other text, the same object.
		"src/main/main_second.c": "#ifdef FIX_BUGS\nextern int g_main_value;\n#endif\nvoid main_second(void) {}\n",
		// Two functions a header's macro changes.
		"include/fft/value.h":        "#ifdef NON_MATCHING\n#define VALUE 2\n#else\n#define VALUE 1\n#endif\n",
		"src/battle/battle_helper.c": "#include \"fft/value.h\"\nint battle_helper(void) { return VALUE; }\n",
		"src/event/card_entry.c":     "#include \"fft/value.h\"\nint card_entry(void) { return VALUE; }\n",
		// A change only the options together make.
		"src/event/attack_entry.c": "extern int g_main_value;\nvoid attack_entry(void) {\n#if defined(FIX_BUGS) && defined(NON_MATCHING)\n    g_main_value = 1;\n#endif\n}\n",
	}
	for path, text := range sources {
		writeTestFile(t, filepath.Join(p.root, filepath.FromSlash(path)), []byte(text))
	}
	config, err := p.loadProjectConfig()
	if err != nil {
		t.Fatal(err)
	}
	if p, err = p.withEdition(config, "dc"); err != nil {
		t.Fatal(err)
	}
	problems, seen, changed, err := p.guardModules(config.Modules)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"battle battle_helper 0x80067000: changes under NON_MATCHING, which its change does not list",
		"event-attack attack_entry 0x801bf000: compiles differently with FIX_BUGS and NON_MATCHING together than with each alone; give each option its own #ifdef block",
		"event-card card_entry 0x801bf000: changes under NON_MATCHING, which its change does not list",
		"main main_first 0x80010000: changes under FIX_BUGS, which its change does not list",
	}
	if !slices.Equal(problems, want) || seen != 5 || changed != 4 {
		t.Fatalf("undeclared: %d seen, %d changed, problems:\n%s", seen, changed, strings.Join(problems, "\n"))
	}
	declare := func(module, name string, options ...string) {
		m, _ := config.module(module)
		for index := range m.Functions {
			if m.Functions[index].Name == name {
				m.Functions[index].Change = options
			}
		}
	}
	declare("main", "main_first", "FIX_BUGS")
	declare("main", "main_second", "FIX_BUGS")
	declare("battle", "battle_helper", "NON_MATCHING")
	declare("event-card", "card_entry", "FIX_BUGS")
	writeTestFile(t, filepath.Join(p.root, "src", "event", "attack_entry.c"), []byte("void attack_entry(void) {}\n"))
	problems, _, changed, err = p.guardModules(config.Modules)
	if err != nil {
		t.Fatal(err)
	}
	want = []string{
		"event-card card_entry 0x801bf000: changes under NON_MATCHING, which its change does not list",
		"event-card card_entry 0x801bf000: declares FIX_BUGS, which neither its source nor a header it includes names",
		"main main_second 0x80010008: declares FIX_BUGS, which does not change it",
	}
	if !slices.Equal(problems, want) || changed != 3 {
		t.Fatalf("declared: %d changed, problems:\n%s", changed, strings.Join(problems, "\n"))
	}
	if _, err := os.Stat(filepath.Join(p.root, "build", "work", "dc", "guard", "main", "main_first", "FIX_BUGS")); err != nil {
		t.Fatalf("variant work directory: %v", err)
	}
}
