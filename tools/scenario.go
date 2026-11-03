// scenario.go prepares and reports `make scenario`, the run of one scenario
// (tools/scenarios/NAME.lua, through tools/scenario.lua) in PCSX-Redux on the
// original disc and on the Developer's Cut disc. The emulator runs on the
// host, so tools/emulator.sh launches it between `scenario prepare` and
// `scenario report`.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// scenarioSave matches the line of a scenario that names its memory card
// save; scenarioSaveField finds any line that sets the field.
var (
	scenarioSave      = regexp.MustCompile(`(?m)^[ \t]*save[ \t]*=[ \t]*\{[ \t]*file[ \t]*=[ \t]*'([^'/\\]+)'[ \t]*,[ \t]*sha256[ \t]*=[ \t]*'([0-9a-f]{64})'[ \t]*\},?[ \t]*$`)
	scenarioSaveField = regexp.MustCompile(`(?m)^[ \t]*save[ \t]*=`)
	scenarioName      = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
)

// scenarioSaveOf returns the save file a scenario names and its sha256.
func scenarioSaveOf(text []byte) (string, string, error) {
	matches := scenarioSave.FindAllSubmatch(text, -1)
	if len(matches) != 1 || len(scenarioSaveField.FindAll(text, -1)) != 1 {
		return "", "", errors.New("a scenario names its save once, on one line: save = { file = 'NAME.mcd', sha256 = '<64 hex digits>' },")
	}
	return string(matches[0][1]), string(matches[0][2]), nil
}

// scenarioKey names the situation a run saves: the same disc, scenario,
// scripts, save and emulator give the same state.
func scenarioKey(parts map[string]string) string {
	h := sha256.New()
	for _, name := range []string{"disc", "scenario", "emulator.lua", "scenario.lua", "save", "emulator"} {
		fmt.Fprintf(h, "%s %s\n", name, parts[name])
	}
	return hex.EncodeToString(h.Sum(nil))[:32]
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (p project) scenarioCommand(args []string) error {
	const usage = "usage: tools scenario prepare --name=NAME --saves=DIR --emulator=SHA256|report"
	if len(args) == 0 {
		return errors.New(usage)
	}
	flags := flag.NewFlagSet("scenario "+args[0], flag.ContinueOnError)
	name := flags.String("name", "", "the scenario, tools/scenarios/NAME.lua")
	saves := flags.String("saves", "", "the directory that holds the memory card saves")
	emulator := flags.String("emulator", "", "the sha256 of the emulator's executable")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New(usage)
	}
	switch args[0] {
	case "prepare":
		if *name == "" || *saves == "" || *emulator == "" {
			return errors.New(usage)
		}
		return p.scenarioPrepare(*name, *saves, *emulator)
	case "report":
		return p.scenarioReport()
	}
	return errors.New(usage)
}

func (p project) scenarioRoot() string { return filepath.Join(p.root, "build", "scenarios") }

// scenarioPrepare checks the scenario's save and writes, for the original
// disc and the Developer's Cut disc, the disc's names.tsv and a fresh run
// directory: the emulator's settings, the scenario, the save in slot 1, the
// situation's key and the situation a passing run saved for that key
// (cached.state; TOOLS_CACHE=0 leaves it out). The runs go to runs.txt.
func (p project) scenarioPrepare(name, saves, emulator string) error {
	if !scenarioName.MatchString(name) {
		return fmt.Errorf("scenario %q: a name is lowercase letters, digits and dashes", name)
	}
	source := filepath.Join("tools", "scenarios", name+".lua")
	text, err := os.ReadFile(filepath.Join(p.root, source))
	if err != nil {
		return fmt.Errorf("no scenario %s", source)
	}
	file, sum, err := scenarioSaveOf(text)
	if err != nil {
		return fmt.Errorf("%s: %w", source, err)
	}
	save, err := os.ReadFile(filepath.Join(saves, file))
	if err != nil {
		return fmt.Errorf("%s: no save %s in SAVES (make scenario's SAVES=dir, default ~/fft-saves)", source, file)
	}
	if got := sha256Hex(save); got != sum {
		return fmt.Errorf("%s: SAVES/%s has sha256 %s, not the scenario's %s", source, file, got, sum)
	}
	config, err := p.loadProjectConfig()
	if err != nil {
		return err
	}
	if info, err := os.Stat(filepath.Join(p.root, targetInput)); err != nil || info.Size() != targetInputBytes {
		return fmt.Errorf("make scenario needs %s at the repo root", targetInput)
	}
	dc, err := p.withEdition(config, developersCut)
	if err != nil {
		return err
	}
	discs := []smokeDisc{smokeDiscs[0], dc.editionDisc()}
	images := []string{targetInput, filepath.Join("build", developersCut, "disc", targetOutput+".bin")}
	parts := map[string]string{"scenario": sha256Hex(text), "save": sum, "emulator": emulator}
	for _, script := range []string{"emulator.lua", "scenario.lua"} {
		data, err := os.ReadFile(filepath.Join(p.root, "tools", script))
		if err != nil {
			return err
		}
		parts[script] = sha256Hex(data)
	}
	root := p.scenarioRoot()
	cache := filepath.Join(root, "cache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return err
	}
	fmt.Fprintf(p.stdout(), "scenario %s: save %s, sha256 checked\n", name, file)
	var runs []string
	for i, disc := range discs {
		if disc.cue != "" {
			if _, err := os.Stat(filepath.Join(p.root, disc.cue)); err != nil {
				return fmt.Errorf("no %s disc: run %s first", disc.name, disc.hint)
			}
		}
		if parts["disc"], err = sha256File(filepath.Join(p.root, images[i])); err != nil {
			return err
		}
		discDir := filepath.Join(root, disc.name)
		cue, _, err := p.prepareRunDisc(config, disc, discDir)
		if err != nil {
			return err
		}
		dir := filepath.Join(discDir, name)
		if err := freshRunDir(dir); err != nil {
			return err
		}
		key := scenarioKey(parts)
		files := map[string][]byte{"scenario.lua": text, "memcard1.mcd": save, "key.txt": []byte(key + "\n")}
		for file, data := range files {
			if err := os.WriteFile(filepath.Join(dir, file), data, 0o644); err != nil {
				return err
			}
		}
		situation := "the setup runs to it"
		cached := filepath.Join(cache, key+".state")
		if _, err := os.Stat(cached); err == nil && os.Getenv("TOOLS_CACHE") != "0" {
			if err := os.Link(cached, filepath.Join(dir, "cached.state")); err != nil {
				return err
			}
			situation = "it loads from the cache"
		}
		rel, err := filepath.Rel(p.root, dir)
		if err != nil {
			return err
		}
		runs = append(runs, fmt.Sprintf("scenario %s %s - %s", rel, cue, disc.name))
		fmt.Fprintf(p.stdout(), "scenario %s: %s disc %s (sha256 %.16s), situation %.16s: %s\n", name, disc.name, cue, parts["disc"], key, situation)
	}
	return os.WriteFile(filepath.Join(root, "runs.txt"), []byte(strings.Join(runs, "\n")+"\n"), 0o644)
}

// quitWhileLoading reports a run whose log ends at the load of a savestate:
// the emulator went away inside it.
func quitWhileLoading(dir string) bool {
	text, err := os.ReadFile(filepath.Join(dir, "log.txt"))
	if err != nil {
		return false
	}
	lines := strings.Split(strings.TrimSpace(string(text)), "\n")
	return strings.Contains(lines[len(lines)-1], "loading the savestate")
}

// scenarioReport prints each disc's result and values, and moves the
// situation a passing run saved into the cache under its key.
func (p project) scenarioReport() error {
	root := p.scenarioRoot()
	runs, err := readEmulatorRuns(filepath.Join(root, "runs.txt"))
	if err != nil {
		return errors.New("no build/scenarios/runs.txt: run make scenario")
	}
	failed := 0
	for _, run := range runs {
		result, shots, err := p.runResult(run.dir)
		if err != nil {
			return err
		}
		dir := filepath.Join(p.root, run.dir)
		fmt.Fprintf(p.stdout(), "%-8s %s (%d screenshots in %s)\n", run.argument, result, shots, run.dir)
		if values, err := os.ReadFile(filepath.Join(dir, "values.txt")); err == nil {
			for _, line := range strings.Split(strings.TrimSpace(string(values)), "\n") {
				if fields := strings.Split(line, "\t"); len(fields) == 3 {
					fmt.Fprintf(p.stdout(), "    %s = %s (expected %s)\n", fields[0], fields[1], fields[2])
				}
			}
		}
		if result != "pass" {
			failed++
			if quitWhileLoading(dir) {
				fmt.Fprintf(p.stdout(), "    the emulator quit loading the situation: PCSX-Redux builds older than 30177 (2026-11-02) crash loading a state made past the title\n")
			}
			continue
		}
		situation := filepath.Join(dir, "situation.state")
		if _, err := os.Stat(situation); err != nil {
			fmt.Fprintf(p.stdout(), "    situation from the cache\n")
			continue
		}
		key, err := os.ReadFile(filepath.Join(dir, "key.txt"))
		if err != nil {
			return err
		}
		cached := filepath.Join(root, "cache", strings.TrimSpace(string(key))+".state")
		if err := os.Rename(situation, cached); err != nil {
			return err
		}
		fmt.Fprintf(p.stdout(), "    situation saved for the next run: build/scenarios/cache/%s\n", filepath.Base(cached))
	}
	if failed > 0 {
		return fmt.Errorf("scenario: %d of %d discs did not pass (log.txt in each run's directory)", failed, len(runs))
	}
	return nil
}
