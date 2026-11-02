package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const testEditionsYAML = `# The build options and the editions that combine them.
options:
  - {name: FIX_BUGS, why: "retail bugs"}
  - {name: NON_MATCHING, why: "same behavior, clearer code"}

editions:
  - {name: dc, title: "Developer's Cut", options: [FIX_BUGS, NON_MATCHING]}
`

func TestEditionsRenderAndValidate(t *testing.T) {
	editions, err := parseEditions([]byte(testEditionsYAML))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(renderEditions(editions)); got != testEditionsYAML {
		t.Fatalf("canonical form:\n%s\nwant:\n%s", got, testEditionsYAML)
	}
	var problems []string
	validateEditions(editions, func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) })
	if len(problems) != 0 {
		t.Fatalf("valid editions: %v", problems)
	}
	for _, test := range []struct{ name, old, new, want string }{
		{"lowercase option", "name: FIX_BUGS,", "name: fix_bugs,", "UPPER_CASE"},
		{"option twice", "name: NON_MATCHING,", "name: FIX_BUGS,", "appears twice"},
		{"no why", "why: \"retail bugs\"", "why: \"\"", "does not say why"},
		{"reserved edition", "name: dc,", "name: shift,", "lowercase name"},
		{"unknown option", "options: [FIX_BUGS, NON_MATCHING]", "options: [FIX_BUGS, PORT]", "unknown option PORT"},
		{"option repeated", "options: [FIX_BUGS, NON_MATCHING]", "options: [FIX_BUGS, FIX_BUGS]", "twice"},
		{"no options", "options: [FIX_BUGS, NON_MATCHING]", "options: []", "has no options"},
	} {
		t.Run(test.name, func(t *testing.T) {
			editions, err := parseEditions([]byte(strings.Replace(testEditionsYAML, test.old, test.new, 1)))
			if err != nil {
				t.Fatal(err)
			}
			var problems []string
			validateEditions(editions, func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) })
			if !strings.Contains(strings.Join(problems, "\n"), test.want) {
				t.Fatalf("problems %v, want text %q", problems, test.want)
			}
		})
	}
}

func TestEditionDefinesAndOutputs(t *testing.T) {
	editions, err := parseEditions([]byte(testEditionsYAML))
	if err != nil {
		t.Fatal(err)
	}
	config := &projectConfig{Editions: editions}
	p := project{root: "/repo"}
	original, err := p.withEdition(config, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := original.outputDir("link", "main"); got != filepath.FromSlash("/repo/build/link/main") {
		t.Fatalf("original output %s", got)
	}
	want := []string{"mipsel-linux-gnu-cpp", "-P", "-undef", "-nostdinc", "-Iinclude", "a.c", "-o", "a.i"}
	if got := original.cppCommand("a.c", "a.i"); !slices.Equal(got, want) {
		t.Fatalf("original cpp %v", got)
	}
	dc, err := p.withEdition(config, "dc")
	if err != nil {
		t.Fatal(err)
	}
	if got := dc.outputDir("disc"); got != filepath.FromSlash("/repo/build/dc/disc") {
		t.Fatalf("edition output %s", got)
	}
	want = []string{"mipsel-linux-gnu-cpp", "-P", "-undef", "-nostdinc", "-Iinclude", "-DFIX_BUGS", "-DNON_MATCHING", "a.c", "-o", "a.i"}
	if got := dc.cppCommand("a.c", "a.i"); !slices.Equal(got, want) {
		t.Fatalf("edition cpp %v", got)
	}
	if _, err := p.withEdition(config, "ee"); err == nil || !strings.Contains(err.Error(), "unknown edition") {
		t.Fatalf("unknown edition: %v", err)
	}
	if _, err := p.withEdition(&projectConfig{}, "dc"); err == nil || !strings.Contains(err.Error(), "editions.yaml is missing") {
		t.Fatalf("no editions file: %v", err)
	}
}

func TestConditionalsTestDefinedNamesAndOptions(t *testing.T) {
	editions, err := parseEditions([]byte(testEditionsYAML))
	if err != nil {
		t.Fatal(err)
	}
	header := "#ifndef X_H\n#define X_H\n#define NULL 0\n#endif\n"
	valid := "#ifdef FIX_BUGS\nint a;\n#else\nint b;\n#endif\n" +
		"#if defined(FIX_BUGS) && !defined NON_MATCHING\n#endif\n#ifndef NULL\n#endif\n#if 0\n#endif\n"
	check := func(source string, editions *editionsFile) error {
		root := t.TempDir()
		writeTestFile(t, filepath.Join(root, "include", "x.h"), []byte(header))
		writeTestFile(t, filepath.Join(root, "src", "main", "a.c"), []byte(source))
		return validateConditionals(root, editions)
	}
	if err := check(valid, editions); err != nil {
		t.Fatalf("valid conditionals: %v", err)
	}
	for _, test := range []struct {
		name, source string
		editions     *editionsFile
		want         string
	}{
		{"misspelled option", "#ifdef FIX_BUG\n#endif\n", editions, "src/main/a.c:1: FIX_BUG is neither"},
		{"option as a value", "#if FIX_BUGS\n#endif\n", editions, "option FIX_BUGS is tested as a value"},
		{"option defined in code", "#define FIX_BUGS 1\n", editions, "which only an edition defines"},
		{"undeclared option", "#if defined(PORT)\n#endif\n", editions, "PORT is neither"},
		{"no editions file", "#ifdef FIX_BUGS\n#endif\n", nil, "FIX_BUGS is neither"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := check(test.source, test.editions)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want text %q", err, test.want)
			}
		})
	}
}
