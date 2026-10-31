package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRenameConfigReferencesFollowsRelocations(t *testing.T) {
	m := &moduleSpec{ID: "battle", Data: []symbolSpec{{Addr: 0x80100000, Name: "g_old"}},
		Relocations: []relocationSpec{{Addr: 0x80100010, To: "g_old", Offset: 4}, {Addr: 0x80100014, To: "g_other"}}}
	config := &projectConfig{Modules: []*moduleSpec{m}}
	if err := renameConfigReferences(config, "g_old", "g_new"); err != nil {
		t.Fatal(err)
	}
	if m.Data[0].Name != "g_new" || m.Relocations[0].To != "g_new" || m.Relocations[0].Offset != 4 || m.Relocations[1].To != "g_other" {
		t.Fatalf("after the rename: %+v %+v", m.Data, m.Relocations)
	}
}

func writeSymbolRenameFixture(t *testing.T, root, relative, contents string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o640); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSymbolFileTransactionRollsBackPublishedFiles(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "first"), filepath.Join(root, "second")
	if err := os.WriteFile(first, []byte("first-before"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("second-before"), 0o600); err != nil {
		t.Fatal(err)
	}
	transaction := newSymbolFileTransaction()
	if err := addExistingFile(transaction, first, []byte("first-after")); err != nil {
		t.Fatal(err)
	}
	if err := addExistingFile(transaction, second, []byte("second-after")); err != nil {
		t.Fatal(err)
	}
	calls := 0
	err := transaction.commitWithRename(func(oldPath, newPath string) error {
		calls++
		if calls == 3 {
			return errors.New("injected publication failure")
		}
		return os.Rename(oldPath, newPath)
	})
	if err == nil || !strings.Contains(err.Error(), "injected publication failure") {
		t.Fatalf("unexpected error: %v", err)
	}
	for path, want := range map[string]string{first: "first-before", second: "second-before"} {
		got, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if string(got) != want {
			t.Fatalf("%s contains %q, want %q", path, got, want)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("transaction left temporary files: %#v", entries)
	}
}

func TestSymbolFileTransactionRejectsStaleInputWithoutWrites(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "first"), filepath.Join(root, "second")
	os.WriteFile(first, []byte("first-before"), 0o600)
	os.WriteFile(second, []byte("second-before"), 0o600)
	transaction := newSymbolFileTransaction()
	if err := addExistingFile(transaction, first, []byte("first-after")); err != nil {
		t.Fatal(err)
	}
	if err := addExistingFile(transaction, second, []byte("second-after")); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(second, []byte("concurrent"), 0o600)
	if err := transaction.commit(); err == nil || !strings.Contains(err.Error(), "changed on disk") {
		t.Fatalf("unexpected error: %v", err)
	}
	firstBody, _ := os.ReadFile(first)
	secondBody, _ := os.ReadFile(second)
	if string(firstBody) != "first-before" || string(secondBody) != "concurrent" {
		t.Fatalf("preflight made partial writes: %q, %q", firstBody, secondBody)
	}
}

func TestSymbolFileTransactionRollsBackMoveAndCreation(t *testing.T) {
	root := t.TempDir()
	config := writeSymbolRenameFixture(t, root, "a-config", "config-before")
	destination := filepath.Join(root, "b-destination")
	source := writeSymbolRenameFixture(t, root, "c-source", "source-before")
	transaction := newSymbolFileTransaction()
	if err := addExistingFile(transaction, config, []byte("config-after")); err != nil {
		t.Fatal(err)
	}
	if err := transaction.addCreation(destination, []byte("moved-source"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := addRemovedFile(transaction, source); err != nil {
		t.Fatal(err)
	}
	calls := 0
	err := transaction.commitWithRename(func(oldPath, newPath string) error {
		calls++
		if calls == 4 {
			return errors.New("injected move failure")
		}
		return os.Rename(oldPath, newPath)
	})
	if err == nil || !strings.Contains(err.Error(), "injected move failure") {
		t.Fatalf("unexpected error: %v", err)
	}
	for path, want := range map[string]string{config: "config-before", source: "source-before"} {
		body, readErr := os.ReadFile(path)
		if readErr != nil || string(body) != want {
			t.Fatalf("rollback left %s as %q (%v), want %q", path, body, readErr, want)
		}
	}
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rollback left destination behind: %v", err)
	}
}

func TestSourceRenameRejectsLinkerCollision(t *testing.T) {
	_, err := renameLinkerIdentifier([]byte("old = 0x10;\nnew = 0x20;\n"), "old", "new")
	if err == nil || !strings.Contains(err.Error(), "collision") {
		t.Fatalf("unexpected error: %v", err)
	}
	output, err := renameLinkerIdentifier([]byte("old = 0x10;\nnew = 0x10;\nuse = old;\n"), "old", "new")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(output, []byte("new = 0x10;")) != 1 || bytes.Contains(output, []byte("old")) {
		t.Fatalf("equal alias was not folded cleanly:\n%s", output)
	}
}

func TestPlanSourceIdentifierRenamesSkipsIncludesAndCoversAssembly(t *testing.T) {
	root := t.TempDir()
	for path, body := range map[string]string{
		"src/main/caller.c":   "#include \"fft/main_startup.h\"\nvoid caller(void) { main_startup(); }\n",
		"src/main/restore.s":  "\tj\tmain_startup\n",
		"include/fft/extra.h": "#include \"fft/main_startup.h\"\nvoid main_startup(void);\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	plan, err := planSourceIdentifierRenames(root, []sourceIdentifierRename{{"main_startup", "main_boot_run_startup"}})
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		"src/main/caller.c":   "#include \"fft/main_startup.h\"\nvoid caller(void) { main_boot_run_startup(); }\n",
		"src/main/restore.s":  "\tj\tmain_boot_run_startup\n",
		"include/fft/extra.h": "#include \"fft/main_startup.h\"\nvoid main_boot_run_startup(void);\n",
	} {
		change, ok := plan.changes[filepath.Join(root, filepath.FromSlash(path))]
		if !ok || string(change.replacement) != want {
			t.Fatalf("%s = %q, want %q", path, change.replacement, want)
		}
	}
}

func TestPlanSourceIdentifierRenamesFollowsProjectNamesIntoComments(t *testing.T) {
	root := t.TempDir()
	writeSymbolRenameFixture(t, root, "src/main/caller.c", "#include \"fft/main_startup.h\"\n/* main_startup runs once; load stays */\nchar *s = \"main_startup\";\nvoid caller(void) { main_startup(); load(); } // main_startup again\n")
	plan, err := planSourceIdentifierRenames(root, []sourceIdentifierRename{{"main_startup", "main_boot_run_startup"}, {"load", "main_file_load"}})
	if err != nil {
		t.Fatal(err)
	}
	want := "#include \"fft/main_startup.h\"\n/* main_boot_run_startup runs once; load stays */\nchar *s = \"main_startup\";\nvoid caller(void) { main_boot_run_startup(); main_file_load(); } // main_boot_run_startup again\n"
	if got := string(plan.changes[filepath.Join(root, "src", "main", "caller.c")].replacement); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenameDocumentIdentifierKeepsPlainWordsInProse(t *testing.T) {
	for _, test := range []struct{ oldName, newName, source, want string }{
		{"main_startup", "main_boot_run_startup",
			"`main_startup()` and [[main_startup|startup]] in `src/main/main_startup.c`; main_startup runs once, main_startup_extra stays.\n  - main_startup()\n",
			"`main_boot_run_startup()` and [[main_boot_run_startup|startup]] in `src/main/main_boot_run_startup.c`; main_boot_run_startup runs once, main_startup_extra stays.\n  - main_boot_run_startup()\n"},
		{"load", "main_file_load",
			"---\nscope:\n  - load\n  - load()\n  - loader\n---\n`load`, `load()`, `make load`, [[load]], [[load|the loader]], [[load#top]] and load in prose\n  - load\n",
			"---\nscope:\n  - main_file_load\n  - main_file_load()\n  - loader\n---\n`load`, `main_file_load()`, `make load`, [[main_file_load]], [[main_file_load|the loader]], [[main_file_load#top]] and load in prose\n  - load\n"},
		{"build", "main_file_build", "no front matter\n  - build\n", "no front matter\n  - build\n"},
	} {
		if got := string(renameDocumentIdentifier([]byte(test.source), test.oldName, test.newName)); got != test.want {
			t.Errorf("%s:\ngot:  %q\nwant: %q", test.oldName, got, test.want)
		}
	}
}

func TestPlanDocumentIdentifierRenamesReadsOnlyTheDocuments(t *testing.T) {
	root := t.TempDir()
	for path, body := range map[string]string{
		"QUIRKS.md":                 "- `g_old`: a quirk.\n",
		"docs/mechanics/Page.md":    "---\nscope:\n  - g_old\n---\nSee [[g_old]].\n",
		"docs/upstream-renames.tsv": "g_upstream\tg_old\n",
		"meta/notes.md":             "g_old\n",
		"build/map/Page.md":         "g_old\n",
		"CLAUDE-RULES.md":           "g_old\n",
	} {
		writeSymbolRenameFixture(t, root, path, body)
	}
	changes, err := planDocumentIdentifierRenames(root, []sourceIdentifierRename{{"g_old", "g_new"}})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"QUIRKS.md":              "- `g_new`: a quirk.\n",
		"docs/mechanics/Page.md": "---\nscope:\n  - g_new\n---\nSee [[g_new]].\n",
	}
	if len(changes) != len(want) {
		t.Fatalf("changed %d documents, want %d", len(changes), len(want))
	}
	for path, body := range want {
		change, ok := changes[filepath.Join(root, filepath.FromSlash(path))]
		if !ok || string(change.replacement) != body {
			t.Fatalf("%s = %q, want %q", path, change.replacement, body)
		}
	}
}

func TestReplaceIdentifierOutsideIncludesSkipsCommentsAndLiterals(t *testing.T) {
	source := []byte("#include \"fft/load.h\"\n/* the load-game overlay */\n// load here\nchar *s = \"load\";\nint load(void);\nload();\n")
	want := []byte("#include \"fft/load.h\"\n/* the load-game overlay */\n// load here\nchar *s = \"load\";\nint main_file_build_header_nnl(void);\nmain_file_build_header_nnl();\n")
	got := replaceIdentifierOutsideIncludes(regexp.MustCompile(`\bload\b`), source, []byte("main_file_build_header_nnl"))
	if !bytes.Equal(got, want) {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestSourceDefinedIdentifiersDistinguishesScopeAndStorage(t *testing.T) {
	source := []byte(`
/* int close; void open(void) {} */
typedef int read;
struct record;
extern int declared;
extern int callback_prototype(s32 (*)(), s32 (*)());
extern int initialized = 1, uninitialized;
int tentative, values[2] = {1, 2};
static int private_value;
int alias_value __asm__("linked_alias");
register u32 architectural_stack __asm__("$29");
register void* architectural_return __asm__("$31");
static void private_function(void) {}
void declared_function(void);
int (*function_pointer)(void);
int (*array_of_pointers[2])(void);
int *function_returning_pointer(void);
int (*function_returning_callback(void))(void);
const char* description = "int format;";
__asm__(".set noat");
DEFINE_PRIMITIVE_SETTER(SetTile16, 2, 0x78)
void caller(void) {
    int close = 1;
    int (*callback)(void);
    open();
    if (read()) { format(); }
}
static int legacy(read, callback) const char* read;
int (*callback)(void);
{ int close = 1; callback(); }
`)
	got, err := sourceDefinedIdentifiers(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"initialized", "tentative", "values", "private_value", "private_function", "alias_value", "linked_alias", "architectural_stack", "architectural_return", "function_pointer", "array_of_pointers", "description", "caller", "legacy"} {
		if !got[name] {
			t.Errorf("missing definition %s", name)
		}
	}
	for _, name := range []string{"close", "open", "read", "format", "callback", "$29", "$31", "declared", "callback_prototype", "uninitialized", "declared_function", "function_returning_pointer", "function_returning_callback", "SetTile16"} {
		if got[name] {
			t.Errorf("%s is not a definition", name)
		}
	}
}

func TestRenameFunctionAllowsSDKNamesInLocalsCommentsAndCalls(t *testing.T) {
	for _, name := range []string{"close", "open", "read", "format", "callback"} {
		t.Run(name, func(t *testing.T) {
			p := validTestProject(t)
			other := "/* int " + name + "; void " + name + "(void) {} */\n" +
				"register u32 architectural_stack __asm__(\"$29\");\n" +
				"void main_second(void) { int " + name + " = 0; }\n" +
				"void caller(void) { " + name + "(); main_first(); }\n"
			writeSymbolRenameFixture(t, p.root, "src/main/main_second.c", other)
			before := readTestFile(t, p, "target/main.yaml")
			if err := p.symbolsCommand([]string{"rename-function", "--old", "main_first", "--new", name, "--dry-run"}); err != nil {
				t.Fatalf("dry run rejected a local/comment/call: %v", err)
			}
			if readTestFile(t, p, "target/main.yaml") != before {
				t.Fatal("dry run changed the configuration")
			}
			if err := p.symbolsCommand([]string{"rename-function", "--old", "main_first", "--new", name}); err != nil {
				t.Fatalf("rename rejected a local/comment/call: %v", err)
			}
			if got := readTestFile(t, p, "src/main/"+name+".c"); got != "void "+name+"(void) {}\n" {
				t.Fatalf("renamed definition = %q", got)
			}
			if got, want := readTestFile(t, p, "src/main/main_second.c"), strings.ReplaceAll(other, "main_first()", name+"()"); got != want {
				t.Fatalf("caller = %q, want %q", got, want)
			}
		})
	}
}

func TestRenameFunctionRejectsRealSourceDefinitionsWithoutWrites(t *testing.T) {
	for _, declaration := range []string{
		"void close(void) {}",
		"static void close(void) {}",
		"int close;",
		"static int close;",
		"extern int close = 1;",
		"int other, close[2] = {1, 2};",
		"static int (*close)(void);",
		"register u32 close __asm__(\"$29\");",
	} {
		t.Run(declaration, func(t *testing.T) {
			p := validTestProject(t)
			writeSymbolRenameFixture(t, p.root, "src/main/main_second.c", declaration+"\nvoid main_second(void) {}\n")
			before := map[string]string{}
			for _, path := range []string{"target/main.yaml", "src/main/main_first.c", "src/main/main_second.c"} {
				before[path] = readTestFile(t, p, path)
			}
			if err := p.symbolsCommand([]string{"rename-function", "--old", "main_first", "--new", "close"}); err == nil || !strings.Contains(err.Error(), "collide") {
				t.Fatalf("definition collision was not rejected: %v", err)
			}
			for path, want := range before {
				if got := readTestFile(t, p, path); got != want {
					t.Fatalf("rejected rename changed %s", path)
				}
			}
			if _, err := os.Stat(filepath.Join(p.root, "src/main/close.c")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("rejected rename created a destination: %v", err)
			}
		})
	}
}

func TestRenameFunctionPropagatesSourceParseErrorsWithoutWrites(t *testing.T) {
	for _, malformed := range []string{
		"/* unterminated close comment",
		"const char* message = \"unterminated close",
		"void main_second(void) { close();",
		"int close",
		"int close + 1;",
		"int close,;",
		"static;",
		"}",
		"struct record { int value; } close;",
		"void legacy(close) int other; {}",
		"void legacy(close) int close;",
		"register u32 architectural_stack __asm__(\"$29\") = 0;",
		"register u32 architectural_stack __asm__(\"$32\");",
		"register u32 architectural_stack __asm__(\"$-1\");",
		"register u32 architectural_function(void) __asm__(\"$29\");",
	} {
		t.Run(malformed, func(t *testing.T) {
			p := validTestProject(t)
			writeSymbolRenameFixture(t, p.root, "src/main/main_second.c", malformed)
			before := readTestFile(t, p, "target/main.yaml")
			if err := p.symbolsCommand([]string{"rename-function", "--old", "main_first", "--new", "close"}); err == nil || !strings.Contains(err.Error(), "main_second.c") {
				t.Fatalf("missing source parse error: %v", err)
			}
			if readTestFile(t, p, "target/main.yaml") != before || readTestFile(t, p, "src/main/main_second.c") != malformed || readTestFile(t, p, "src/main/main_first.c") != "void main_first(void) {}\n" {
				t.Fatal("failed parse changed files")
			}
			if _, err := os.Stat(filepath.Join(p.root, "src/main/close.c")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("failed parse created a destination: %v", err)
			}
		})
	}
}

// addExistingFile and addRemovedFile snapshot a file as renameSymbols does.
func addExistingFile(transaction *symbolFileTransaction, path string, replacement []byte) error {
	info, original, err := readRegularSymbolFile(path)
	if err != nil {
		return err
	}
	return transaction.addExistingSnapshot(path, original, replacement, info.Mode())
}

func addRemovedFile(transaction *symbolFileTransaction, path string) error {
	info, original, err := readRegularSymbolFile(path)
	if err != nil {
		return err
	}
	return transaction.addRemovalSnapshot(path, original, info.Mode())
}
