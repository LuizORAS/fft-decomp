package main

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const testMapMainHeader = `#ifndef MAIN_H
#define MAIN_H

/* system */
typedef struct main_record {
    s32 value;
} main_record_t;

typedef enum main_mode {
    MAIN_MODE_IDLE = 0,
    MAIN_MODE_RUN = 1,
} main_mode_e;

typedef char main_record_size_must_be_4[(sizeof(main_record_t) == 4) ? 1 : -1];

extern main_record_t g_main_value;
void main_first(void);

#define MAIN_CALL_SECOND() main_second()

/* sound */
void main_second(void);

#endif
`

const testMapBattleHeader = `/* unit */
extern s32 g_battle_value;
void battle_helper(void);
`

func testMapProject(t *testing.T) project {
	t.Helper()
	p := testProject(t, map[string]string{
		"target/main.yaml":           testMainYAML,
		"target/event.yaml":          testEventYAML,
		"target/battle.yaml":         testBattleYAML + testMapBattleRegion,
		"include/fft/main.h":         testMapMainHeader,
		"include/fft/battle.h":       testMapBattleHeader,
		"include/psx/types.h":        "typedef int s32;\n",
		"src/main/main_first.c":      "#include \"fft/main.h\"\n\n/* Runs the second routine through a macro\n * and reads the record. */\nvoid main_first(void) {\n    g_main_value.value = MAIN_MODE_RUN;\n    MAIN_CALL_SECOND();\n}\n",
		"src/main/main_second.c":     "#include \"fft/main.h\"\n\n/* Not a summary: a define follows. */\n#define LOCAL_STEP 1\nvoid main_second(void) { battle_helper(); }\n",
		"src/event/attack_entry.c":   "void attack_entry(void) {\n    void (*handler)(void) = main_second;\n    s32 value = g_battle_value;\n    handler();\n}\n",
		"src/event/card_entry.c":     "/* Port debt (QUIRKS.md): the old card_old_name\n * read a stale register (see main_fi* and card_gone_*). */\nvoid card_entry(void) { battle_asm_routine(); }\n",
		"src/battle/battle_helper.c": "/* main 0x80067000; layout note */\nvoid battle_helper(void) {}\n",
		"QUIRKS.md":                  testMapQuirks,
	})
	p.out = io.Discard
	return p
}

// testMapBattleRegion appends a handwritten routine, kept as bytes, to the
// battle module's data rows.
const testMapBattleRegion = `  - {addr: 0x80067200, name: battle_asm_routine}

regions:
  - {addr: 0x80067200, end: 0x80067240, kind: handwritten, why: "handwritten test routine"}
`

func TestCodebaseMapRegions(t *testing.T) {
	p := testMapProject(t)
	if err := p.mapCommand(nil); err != nil {
		t.Fatal(err)
	}
	pages := readTree(t, filepath.Join(p.root, "build", "map"))
	region := pages["regions/battle/battle_asm_routine.md"]
	for _, want := range []string{"kind: \"handwritten\"", "Original handwritten code kept as bytes", "## Called by (1)\n\n[[card_entry]]"} {
		if !strings.Contains(region, want) {
			t.Fatalf("region page lacks %q:\n%s", want, region)
		}
	}
	if !strings.Contains(pages["functions/event/card_entry.md"], "## Calls (1)\n\n[[battle_asm_routine]]") {
		t.Fatalf("card_entry page:\n%s", pages["functions/event/card_entry.md"])
	}
	if !strings.Contains(pages["modules/battle (module).md"], "| `0x80067200`–`0x80067240` | handwritten | [[battle_asm_routine]] | handwritten test routine |") {
		t.Fatalf("battle module page:\n%s", pages["modules/battle (module).md"])
	}
}

func TestMapAsmSourceSummary(t *testing.T) {
	if got := asmSourceSummary("/* Restore sp,\n * then jump.\n *\n * Assembly. */\n\t.text\n"); got != "Restore sp, then jump.\n\nAssembly." {
		t.Fatalf("summary %q", got)
	}
	if got := asmSourceSummary("\t.text\n/* Not at the top. */\n"); got != "" {
		t.Fatalf("summary %q for a file that opens with code", got)
	}
}

func TestMapSymbolMapsAndUpstreamRenames(t *testing.T) {
	p := testMapProject(t)
	table := filepath.Join(p.root, filepath.FromSlash(mapUpstreamTable))
	// Without an upstream export the committed copy is left alone.
	if err := p.mapCommand(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(table); !os.IsNotExist(err) {
		t.Fatalf("%s written without an upstream export: %v", mapUpstreamTable, err)
	}
	// Upstream still calls main_second "main_old_second" and g_battle_value
	// "g_battle_old_value".
	writeTestFile(t, filepath.Join(p.root, "build", "upstream", "target", "main.yaml"),
		[]byte(strings.Replace(testMainYAML, "name: main_second,", "name: main_old_second,", 1)))
	writeTestFile(t, filepath.Join(p.root, "build", "upstream", "target", "battle.yaml"),
		[]byte(strings.Replace(testBattleYAML, "name: g_battle_value}", "name: g_battle_old_value}", 1)))
	if err := p.mapCommand(nil); err != nil {
		t.Fatal(err)
	}
	read := func(path string) string {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	battle := read(filepath.Join(p.root, "build", "symbols", "fft_battle.map"))
	want := "80010000 main_first\n80010008 main_second\n80010100 g_main_value\n80010200 memset\n" +
		"80067000 battle_helper\n80067100 g_battle_value\n80067104 g_main_value\n80067200 battle_asm_routine\n"
	if battle != want {
		t.Fatalf("fft_battle.map:\n%s\nwant:\n%s", battle, want)
	}
	page := read(filepath.Join(p.root, "build", "map", "reports", "Upstream renames.md"))
	for _, row := range []string{
		"| battle | `0x80067100` | `g_battle_old_value` | [[g_battle_value]] |",
		"| main | `0x80010008` | `main_old_second` | [[main_second]] |",
	} {
		if !strings.Contains(page, row) {
			t.Fatalf("upstream page lacks %q:\n%s", row, page)
		}
	}
	wantTable := "module\taddress\tupstream\tours\n" +
		"battle\t0x80067100\tg_battle_old_value\tg_battle_value\n" +
		"main\t0x80010008\tmain_old_second\tmain_second\n"
	if got := read(table); got != wantTable {
		t.Fatalf("%s:\n%s\nwant:\n%s", mapUpstreamTable, got, wantTable)
	}
	if match := mapWarningLine.FindStringSubmatch("/tmp/x.i:12: warning: unused variable `t1'"); match == nil || match[1] != "warning: unused variable `t1'" {
		t.Fatalf("warning line parse: %v", match)
	}
}

const testMapQuirks = `# Target quirks

Facts about the retail code.

## Retail bugs the source reproduces

- ` + "`main_first`" + `: writes ` + "`g_main_value`" + ` in ` + "`src/main/main_first.c`" + `; the
  ` + "`div`" + ` result and ` + "`main_missing_name`" + ` are named here.
- ` + "`battle_helper`" + ` and its twin ` + "`main_second`" + `.

## Calls that disagree with the callee

- ` + "`card_entry`" + ` passes nothing.
`

func TestCodebaseMapQuirksDebtsAndStale(t *testing.T) {
	p := testMapProject(t)
	config, err := p.loadProjectConfig()
	if err != nil {
		t.Fatal(err)
	}
	index, err := buildCodebaseIndex(p, config)
	if err != nil {
		t.Fatal(err)
	}
	if len(index.quirks) != 2 || len(index.quirks[0].entries) != 2 || index.quirks[0].intro != "" {
		t.Fatalf("sections %+v", index.quirks)
	}
	first := index.quirks[0].entries[0]
	if first.line != 7 || !strings.Contains(first.text, "\n  `div`") {
		t.Fatalf("first entry at line %d: %q", first.line, first.text)
	}
	if !reflect.DeepEqual(first.missing, []string{"main_missing_name"}) {
		t.Fatalf("missing = %v (instructions such as div are not names)", first.missing)
	}
	for name, want := range map[string]int{"main_first": 1, "main_second": 1, "battle_helper": 1, "card_entry": 1, "attack_entry": 0} {
		if got := len(index.functions[name].quirks); got != want {
			t.Fatalf("%s has %d quirks, want %d", name, got, want)
		}
	}
	if len(index.debts) != 1 || index.debts[0].kind != "port" || index.debts[0].comment.function != "card_entry" {
		t.Fatalf("debts %+v", index.debts)
	}
	var stale []string
	for _, entry := range index.stale {
		stale = append(stale, entry.name)
	}
	if !reflect.DeepEqual(stale, []string{"main_missing_name", "card_old_name", "card_gone_*"}) {
		t.Fatalf("stale = %v", stale)
	}

	if err := p.mapCommand(nil); err != nil {
		t.Fatal(err)
	}
	pages := readTree(t, filepath.Join(p.root, "build", "map"))
	section := pages["quirks/Retail bugs the source reproduces.md"]
	if !strings.Contains(section, "- [[main_first]]: writes [[g_main_value]] in [[main_first|src/main/main_first.c]];") {
		t.Fatalf("section page:\n%s", section)
	}
	if !strings.Contains(pages["functions/event/card_entry.md"], "- **Port debt**, line 1: Port debt (QUIRKS.md): the old card_old_name read a stale register (see main_fi* and card_gone_*).") {
		t.Fatalf("card_entry page:\n%s", pages["functions/event/card_entry.md"])
	}
	if !strings.Contains(pages["globals/fft/main/g_main_value.md"], "## Quirks (1)") {
		t.Fatal("global page lacks its quirk")
	}
	backlog := pages["reports/Build 3 backlog.md"]
	for _, want := range []string{"port_debt_comments: 1", "| [[Retail bugs the source reproduces]] | 2 | FIX_BUGS |", "| port | [[card_entry]] | 1 |"} {
		if !strings.Contains(backlog, want) {
			t.Fatalf("backlog lacks %q:\n%s", want, backlog)
		}
	}
	if !strings.Contains(pages["reports/Stale mentions.md"], "| [[card_entry]] | 1 | `card_old_name` |") {
		t.Fatalf("stale page:\n%s", pages["reports/Stale mentions.md"])
	}
}

func TestCodebaseIndexReadsDefinitionsAndReferences(t *testing.T) {
	p := testMapProject(t)
	config, err := p.loadProjectConfig()
	if err != nil {
		t.Fatal(err)
	}
	index, err := buildCodebaseIndex(p, config)
	if err != nil {
		t.Fatal(err)
	}
	first := index.functions["main_first"]
	if first.summary != "Runs the second routine through a macro and reads the record." {
		t.Fatalf("summary = %q", first.summary)
	}
	if first.line != 5 || first.signature != "void main_first(void)" {
		t.Fatalf("definition at line %d as %q", first.line, first.signature)
	}
	check := func(name string, got, want []string) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s = %v, want %v", name, got, want)
		}
	}
	check("main_first calls", first.calls, []string{"main_second"})
	check("main_first globals", first.globals, []string{"g_main_value"})
	check("main_first types", first.types, []string{"main_mode_e"})

	second := index.functions["main_second"]
	if second.summary != "" {
		t.Fatalf("a comment followed by a define is not a summary: %q", second.summary)
	}
	check("main_second calls", second.calls, []string{"battle_helper"})
	check("main_second called by", second.calledBy, []string{"main_first"})
	check("main_second address used by", second.referencedBy, []string{"attack_entry"})
	check("attack_entry references", index.functions["attack_entry"].references, []string{"main_second"})
	check("attack_entry globals", index.functions["attack_entry"].globals, []string{"g_battle_value"})

	if got := first.prototype; got.header != "include/fft/main.h" || got.label != "system" {
		t.Fatalf("main_first prototype %+v", got)
	}
	if got := second.prototype.label; got != "sound" {
		t.Fatalf("main_second label %q", got)
	}
	if index.types["main_record_t"].kind != "struct" || index.types["main_mode_e"].kind != "enum" {
		t.Fatalf("type kinds %q %q", index.types["main_record_t"].kind, index.types["main_mode_e"].kind)
	}
	if _, ok := index.types["main_record_size_must_be_4"]; ok {
		t.Fatal("size assertion indexed as a type")
	}
	global := index.globals["g_main_value"]
	if len(global.addresses) != 2 || global.addresses[0].module != "battle" || global.addresses[1].module != "main" {
		t.Fatalf("g_main_value addresses %+v", global.addresses)
	}
	if proseSummary(index.functions["battle_helper"].summary) {
		t.Fatal("an address note is not a prose summary")
	}
	if got := mapSubsystem("battle_formula_calculate_hit", "src/battle/battle_formula_calculate_hit.c"); got != "battle_formula" {
		t.Fatalf("subsystem %q", got)
	}
	if got := mapSubsystem("RotTrans", "src/psyq/libgte/RotTrans.c"); got != "psyq_libgte" {
		t.Fatalf("library subsystem %q", got)
	}
}

func readTree(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(root, path)
		files[filepath.ToSlash(relative)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestMapCommandWritesDeterministicVault(t *testing.T) {
	p := testMapProject(t)
	if err := p.mapCommand(nil); err != nil {
		t.Fatal(err)
	}
	vault := filepath.Join(p.root, "build", "map")
	first := readTree(t, vault)
	page := first["functions/main/main_first.md"]
	for _, want := range []string{
		"type: \"function\"",
		"has_summary: true",
		"prose_summary: true",
		"header_label: \"system\"",
		"## Calls (1)\n\n[[main_second]]",
		"[[g_main_value]]",
		"[[main_mode_e]]",
		"| [[main (module)\\|main]] | `0x80010000` | 8 |",
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("main_first page lacks %q:\n%s", want, page)
		}
	}
	if !strings.Contains(first["functions/main/main_second.md"], "## Address used by (1)\n\n[[attack_entry]]") {
		t.Fatalf("main_second page:\n%s", first["functions/main/main_second.md"])
	}
	if !strings.Contains(first["functions/event/card_entry.md"], "No C code calls this function") {
		t.Fatal("an uncalled function does not say so")
	}
	for _, path := range []string{"Home.md", "modules/main (module).md", "subsystems/main_first (subsystem).md",
		"headers/main.h.md", "types/fft/main/main_record_t.md", "globals/fft/battle/g_battle_value.md"} {
		if _, ok := first[path]; !ok {
			t.Fatalf("missing %s; have %v", path, sortedKeys(first))
		}
	}

	if !strings.Contains(first[".obsidian/core-plugins.json"], `"bases": true`) {
		t.Fatal("a new vault is not seeded with the core-feature configuration")
	}
	for _, path := range []string{"reports/Metrics.md", "bases/Functions.base", "functions.tsv"} {
		if _, ok := first[path]; !ok {
			t.Fatalf("missing %s", path)
		}
	}

	// Obsidian settings survive a rebuild, and the pages come out identical.
	settings := filepath.Join(vault, ".obsidian", "app.json")
	writeTestFile(t, settings, []byte("{}"))
	if err := p.mapCommand(nil); err != nil {
		t.Fatal(err)
	}
	second := readTree(t, vault)
	if second[".obsidian/app.json"] != "{}" {
		t.Fatal(".obsidian was not kept")
	}
	for _, tree := range []map[string]string{first, second} {
		for path := range tree {
			if strings.HasPrefix(path, ".obsidian/") {
				delete(tree, path)
			}
		}
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("a second run wrote different pages")
	}
}

func TestDeclaredNamesAndWeakNames(t *testing.T) {
	source := "void f(s32 a0, u8* table, s32 count) {\n" +
		"    s32 i, t1;\n" +
		"    register s32 pinned __asm__(\"$5\");\n" +
		"    main_record_t* p = 0;\n" +
		"    for (i = 0; i < count; i++) {\n" +
		"        u8 temp;\n" +
		"        s16 unk_08 = table[i];\n" +
		"        local_t* q;\n" +
		"        g(a0, t1, temp, unk_08);\n" +
		"        *table = count * 2;\n" +
		"    }\n" +
		"}\n"
	tokens, err := tokenizeC(source)
	if err != nil {
		t.Fatal(err)
	}
	index := &codebaseIndex{types: map[string]*mapType{
		"main_record_t": {name: "main_record_t"}, "s32": {name: "s32"}, "u8": {name: "u8"}, "s16": {name: "s16"},
	}}
	f := &mapFunction{name: "f"}
	f.locals = index.declaredNames(f, tokens)
	want := []string{"a0", "table", "count", "i", "t1", "pinned", "p", "temp", "unk_08", "q"}
	if !reflect.DeepEqual(f.locals, want) {
		t.Fatalf("locals = %v, want %v", f.locals, want)
	}
	if got := f.weakNames(); !reflect.DeepEqual(got, []string{"a0", "t1", "p", "unk_08", "q"}) {
		t.Fatalf("weak = %v", got)
	}
	if base, ok := mapNumericSuffix("world_build_at_list_2"); !ok || base != "world_build_at_list" {
		t.Fatalf("suffix base %q %v", base, ok)
	}
	family := &codebaseIndex{functions: map[string]*mapFunction{
		"x_get_1": {}, "x_get_2": {}, "x_run": {}, "x_run_2": {}, "x_add_by_10": {},
	}}
	for name, want := range map[string]string{"x_get_1": "x_get", "x_run_2": "x_run", "x_add_by_10": ""} {
		if base, ok := family.numberedFamily(name); base != want || ok != (want != "") {
			t.Fatalf("numbered family of %s = %q %v", name, base, ok)
		}
	}
	if !mapAddressName("battle_return_zero_8008e6d0") || mapAddressName("battle_formula_calculate_hit") {
		t.Fatal("address-name detection")
	}
	if !blockPurposeComment("\n/* How the AI picks an action. */\n\ntypedef u8 a_t;") ||
		blockPurposeComment("\n/* The first type's own comment. */\ntypedef u8 a_t;") {
		t.Fatal("purpose-comment detection")
	}
	if got := (&mapFunction{locals: []string{"value", "data2", "values", "p"}}).genericNames(); !reflect.DeepEqual(got, []string{"value", "data2"}) {
		t.Fatalf("generic = %v", got)
	}
}

func TestMapCommandRefusesOutputOutsideBuild(t *testing.T) {
	p := testMapProject(t)
	for _, out := range []string{"src", "build", "build/..", "/tmp/elsewhere"} {
		if err := p.mapCommand([]string{"--out=" + out}); err == nil {
			t.Fatalf("--out=%s was accepted", out)
		}
	}
	if _, err := os.Stat(filepath.Join(p.root, "src", "main", "main_first.c")); err != nil {
		t.Fatal(err)
	}
}

func TestTableRowEscapesPipes(t *testing.T) {
	if got := tableRow("[[a (module)|a]]", "x | y"); got != "| [[a (module)\\|a]] | x \\| y |\n" {
		t.Fatalf("tableRow = %q", got)
	}
}

func TestMapDocsScopesAndEditProtection(t *testing.T) {
	p := testMapProject(t)
	write := func(root, path, content string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	vaultDir := filepath.Join(p.root, "build", "map")
	turns := "---\ntype: mechanic\ntier: 1\nscope:\n  - main\n  - main_first\n  - main_second()\n  - main_gone\n  - src/event/\n  - src/main/\n---\n\n# Turns\n\n[[main_first]], `[[in_code]]`, [[Turns scope]] and [[Missing page]].\n\n| [[main_first\\|first]] |\n\n`src/event/`, `src/gone/`, `src/<module>/`, `battle_asm`, `main_first()` and `main_missing_span()`.\n"
	write(p.root, "CODEBASE.md", "# Codebase\n")
	write(p.root, "docs/mechanics/Turns.md", turns)
	write(p.root, "docs/guides/Retarget.md", "---\ntype: guide\nscope: [battle]\n---\n\n[[Turns]] and ![[{{title}} scope]]\n")
	if err := p.mapCommand(nil); err != nil {
		t.Fatal(err)
	}
	pages := readTree(t, vaultDir)
	if pages["CODEBASE.md"] != "# Codebase\n" || pages["docs/mechanics/Turns.md"] != turns {
		t.Fatal("hand-written pages were not copied unchanged")
	}
	functionPage := func(name string) string {
		for path, content := range pages {
			if strings.HasPrefix(path, "functions/") && strings.HasSuffix(path, "/"+name+".md") {
				return content
			}
		}
		t.Fatalf("no page for %s", name)
		return ""
	}
	if page := functionPage("main_first"); !strings.Contains(page, `mechanic: "[[Turns]]"`) || !strings.Contains(page, "\ntier: 1\n") {
		t.Fatalf("main_first page:\n%s", page)
	}
	if strings.Contains(functionPage("battle_helper"), "[[Turns]]") {
		t.Fatal("a guide's scope assigned a mechanic")
	}
	if page := functionPage("card_entry"); !strings.Contains(page, `mechanic: "[[Turns]]"`) {
		t.Fatalf("a source folder entry did not match card_entry:\n%s", page)
	}
	for page, want := range map[string]string{
		"scopes/Turns scope.md":     "| `main_first` | 1 |\n| `main_second()` | 1 |\n| `main_gone` | 0 |\n| `src/event/` | 2 |\n| `src/main/` | 0 |\n",
		"reports/Stale mentions.md": "| [[Turns\\|docs/mechanics/Turns.md]] | 8 | `main_gone` |",
		"Home.md":                   "| [[Turns]] | 1 | [[Turns scope]]: ",
	} {
		if !strings.Contains(pages[page], want) {
			t.Fatalf("%s lacks %q:\n%s", page, want, pages[page])
		}
	}
	stale := pages["reports/Stale mentions.md"]
	if !strings.Contains(stale, "`Missing page`") || strings.Contains(stale, "in_code") || strings.Contains(stale, "{{title}}") ||
		strings.Contains(stale, "main_first") {
		t.Fatalf("stale page:\n%s", stale)
	}
	if !strings.Contains(stale, "`src/gone/`") || !strings.Contains(stale, "`main_missing_span()`") ||
		strings.Contains(stale, "<module>") || strings.Contains(stale, "`src/event/`") || strings.Contains(stale, "`battle_asm`") {
		t.Fatalf("code spans in the stale page:\n%s", stale)
	}

	write(vaultDir, "docs/mechanics/Turns.md", turns+"Edited in the vault.\n")
	write(vaultDir, "docs/guides/New.md", "# New\n")
	if err := p.mapCommand(nil); err == nil || !strings.Contains(err.Error(), "--pull-docs") {
		t.Fatalf("an edited copy did not stop the run: %v", err)
	}
	if err := p.mapCommand([]string{"--pull-docs"}); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{"docs/mechanics/Turns.md": turns + "Edited in the vault.\n", "docs/guides/New.md": "# New\n"} {
		data, err := os.ReadFile(filepath.Join(p.root, filepath.FromSlash(path)))
		if err != nil || string(data) != want {
			t.Fatalf("%s was not pulled: %q %v", path, data, err)
		}
	}
	write(p.root, "docs/guides/New.md", "# Repository\n")
	write(vaultDir, "docs/guides/New.md", "# Vault\n")
	if err := p.mapCommand([]string{"--pull-docs"}); err == nil || !strings.Contains(err.Error(), "both") {
		t.Fatalf("a page changed on both sides was overwritten: %v", err)
	}
}

// A shared source defines its source_symbol, not the name of each row that
// uses it.
func TestMapSharedSourceDefinition(t *testing.T) {
	root := t.TempDir()
	source := "/* Reports whether the unit is deployed. */\ns32 shared_is_deployed(s32 id) { return id; }\n"
	if err := os.WriteFile(filepath.Join(root, "a.c"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	index := &codebaseIndex{identifiers: map[string]bool{}}
	f := &mapFunction{name: "module_has_id", symbol: "shared_is_deployed", source: "a.c"}
	if err := index.scanFunction(root, f); err != nil {
		t.Fatal(err)
	}
	if f.line != 2 || f.summary != "Reports whether the unit is deployed." {
		t.Fatalf("line %d, summary %q", f.line, f.summary)
	}
}
