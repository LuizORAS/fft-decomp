package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDeclCheckSourcesListEachCSourceOnce(t *testing.T) {
	p := sharedDefinitionProject(t, "void main_first(void) {}\n")
	config := checkedFixtureConfig(t, p)
	m, _ := config.module("main")
	m.Functions = append(m.Functions, functionSpec{Addr: 0x8001000c, Size: 4, Name: "main_switch", Asm: "replaces sp"})
	var got []string
	for _, task := range declCheckSources(config) {
		got = append(got, task.source+" ["+task.module+"]")
	}
	// main_second compiles main_first.c (listed once); main_switch is assembly.
	want := "src/battle/battle_helper.c [battle] src/event/attack_entry.c [event-attack] " +
		"src/event/card_entry.c [event-card] src/main/main_first.c [main]"
	if strings.Join(got, " ") != want {
		t.Fatalf("sources = %q", got)
	}
}

// declCheckFixture is the test project with every source compilable on its
// own, plus the given body for main_first.c.
func declCheckFixture(t *testing.T, mainFirst string) (project, *strings.Builder) {
	t.Helper()
	requirePinnedToolchain(t)
	p := validTestProject(t)
	writeTestFile(t, filepath.Join(p.root, "src/event/attack_entry.c"), []byte("void attack_entry(void) {}\n"))
	writeTestFile(t, filepath.Join(p.root, "src/main/main_first.c"), []byte(mainFirst))
	out := &strings.Builder{}
	p.out = out
	return p, out
}

func TestDeclCheckFailsOnAnUndeclaredCall(t *testing.T) {
	p, out := declCheckFixture(t, "void main_first(void) { main_helper(); }\n")
	err := p.declCheckCommand(nil)
	if err == nil || !strings.Contains(err.Error(), "1 implicit declarations") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(out.String(), "src/main/main_first.c [main]: implicit declaration of main_helper\n") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestDeclCheckPassesWhenEveryCallIsDeclared(t *testing.T) {
	p, out := declCheckFixture(t, "void main_helper(void);\nvoid main_first(void) { main_helper(); }\n")
	if err := p.declCheckCommand(nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "decl-check: 5 sources declare every function they call") {
		t.Fatalf("output = %q", out.String())
	}
}
