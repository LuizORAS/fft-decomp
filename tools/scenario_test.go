package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestScenarioSaveLine(t *testing.T) {
	sum := strings.Repeat("0a", 32)
	file, got, err := scenarioSaveOf([]byte("return {\n    why = 'x',\n    save = { file = 'card.mcd', sha256 = '" + sum + "' },\n}\n"))
	if err != nil || file != "card.mcd" || got != sum {
		t.Fatalf("save line: %q %q %v", file, got, err)
	}
	for _, text := range []string{
		"return { why = 'x' }\n",
		"save = { file = 'a.mcd', sha256 = '" + sum + "' },\nsave = { file = 'b.mcd', sha256 = '" + sum + "' },\n",
		"save = {\n    file = 'a.mcd', sha256 = '" + sum + "' },\n",
		"save = { file = 'a.mcd', sha256 = '" + strings.ToUpper(sum) + "' },\n",
		"save = { file = '../a.mcd', sha256 = '" + sum + "' },\n",
		"save = { file = 'a.mcd', sha256 = '" + sum[:62] + "' },\n",
	} {
		if _, _, err := scenarioSaveOf([]byte(text)); err == nil {
			t.Errorf("accepted %q", text)
		}
	}
}

func TestScenarioKeyFollowsEveryInput(t *testing.T) {
	parts := map[string]string{"disc": "d", "scenario": "s", "emulator.lua": "e", "scenario.lua": "r", "save": "v", "emulator": "x"}
	key := scenarioKey(parts)
	if len(key) != 32 || scenarioKey(parts) != key {
		t.Fatalf("key %q", key)
	}
	for name := range parts {
		changed := map[string]string{}
		for k, v := range parts {
			changed[k] = v
		}
		changed[name] += "2"
		if scenarioKey(changed) == key {
			t.Errorf("the key ignores %s", name)
		}
	}
}

func TestEmulatorRunsRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runs.txt")
	writeTestFile(t, path, []byte("smoke build/smoke/original/new-game build/smoke/original/scus-94221.cue - new-game\n"+
		"scenario build/scenarios/dc/demo build/dc/disc/output-scus-94221.cue - dc\n"))
	runs, err := readEmulatorRuns(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 || runs[0].argument != "new-game" || runs[1].script != "scenario" || runs[1].card != "-" {
		t.Fatalf("runs %+v", runs)
	}
	writeTestFile(t, path, []byte("smoke build/smoke/original/new-game build/smoke/original/scus-94221.cue new-game\n"))
	if _, err := readEmulatorRuns(path); err == nil {
		t.Fatal("a line of four fields was read")
	}
}

func TestQuitWhileLoadingReadsTheLogsLastLine(t *testing.T) {
	dir := t.TempDir()
	if quitWhileLoading(dir) {
		t.Fatal("no log, yet a quit while loading")
	}
	writeTestFile(t, filepath.Join(dir, "log.txt"), []byte("     0 10507 names\n  2604 loading the savestate x/situation.state (21161959 bytes)\n"))
	if !quitWhileLoading(dir) {
		t.Fatal("a log that ends at the load")
	}
	writeTestFile(t, filepath.Join(dir, "log.txt"), []byte("  2604 loading the savestate x (1 bytes)\n  2605 savestate loaded\n"))
	if quitWhileLoading(dir) {
		t.Fatal("a log that goes on past the load")
	}
}

func TestEditionDiscIsItsOwnLink(t *testing.T) {
	p := project{root: "/repo", edition: "dc"}
	disc := p.editionDisc()
	if disc.name != "dc" || disc.link != filepath.Join("dc", "link") || disc.cue != filepath.Join("build", "dc", "disc", targetOutput+".cue") || disc.hint != "make build EDITION=dc" {
		t.Fatalf("edition disc %+v", disc)
	}
	if got := p.smokeRoot(); got != filepath.FromSlash("/repo/build/dc/smoke") {
		t.Fatalf("smoke root %s", got)
	}
}
