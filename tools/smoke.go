// smoke.go prepares and reports `make smoke`, the run of tools/smoke.lua in
// PCSX-Redux on the original disc and on the shifted test disc that
// `make build SHIFT=N` writes, or with an edition on that edition's disc
// alone. The emulator runs on the host, so tools/emulator.sh launches it
// between `smoke prepare` and `smoke report`.
package main

import (
	"debug/elf"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// smokeDisc is one disc a run boots: the original BIN with the names of the
// identical link, the shifted test disc or an edition's disc with the names
// of its own link.
type smokeDisc struct {
	name string
	link string // build/<link>/<module>/<module>.elf
	cue  string // the cue sheet, relative to the repo root; empty: the BIN, linked beside its names
	hint string // the command that writes the link and the disc
}

var smokeDiscs = []smokeDisc{
	{name: "original", link: "link", hint: "make build"},
	{name: "shift", link: "shift", cue: filepath.Join("build", "shift", "disc", targetOutput+".cue"), hint: "make build SHIFT=16"},
}

// editionDisc is the disc an edition's make build writes, with its link.
func (p project) editionDisc() smokeDisc {
	return smokeDisc{
		name: p.edition,
		link: filepath.Join(p.edition, "link"),
		cue:  filepath.Join("build", p.edition, "disc", targetOutput+".cue"),
		hint: "make build EDITION=" + p.edition,
	}
}

// smokeSessions are the boots of each disc, in order: a new game through the
// first battle to the save screen, which writes the session's memory card, and
// Continue from that card through the Gariland battle to the world map's
// formation screen.
var smokeSessions = []string{"new-game", "world-map"}

// smokeSettings is the emulator's portable configuration for a run: the
// interpreter and the debugger for breakpoints, the BIOS fallback (OpenBIOS
// when none is set), the software renderer (it needs no OpenGL 3 driver), no
// network servers, unthrottled and muted, and a pause on a break instruction
// besides the debugger's default first-chance exceptions (address and bus
// errors, reserved instructions, coprocessor and overflow).
const smokeSettings = `{
  "emulator": {
    "AutoUpdate": false,
    "ShownAutoUpdateConfig": true,
    "Dynarec": false,
    "HardwareRenderer": false,
    "FastBoot": false,
    "Mcd1Inserted": true,
    "Mcd2Inserted": true,
    "Debug": {
      "Debug": true,
      "FirstChanceException": 7920,
      "GdbServer": false,
      "WebServer": false
    }
  },
  "SPU": {
    "Mute": true,
    "Speed": 0
  }
}
`

func (p project) smokeCommand(args []string) error {
	const usage = "usage: tools smoke prepare|report [--edition=E]"
	if len(args) == 0 {
		return errors.New(usage)
	}
	flags := flag.NewFlagSet("smoke "+args[0], flag.ContinueOnError)
	edition := flags.String("edition", "", "edition of target/editions.yaml whose disc boots alone (default: the original and the shifted disc)")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New(usage)
	}
	config, err := p.loadProjectConfig()
	if err != nil {
		return err
	}
	if p, err = p.withEdition(config, *edition); err != nil {
		return err
	}
	switch args[0] {
	case "prepare":
		return p.smokePrepare(config)
	case "report":
		return p.smokeReport()
	}
	return errors.New(usage)
}

// smokeRoot is build/smoke, or build/<edition>/smoke.
func (p project) smokeRoot() string { return p.outputDir("smoke") }

// smokePrepare writes each disc's names.tsv from its link, links the
// original BIN with a cue sheet, gives every session a fresh directory with
// its settings and lists the runs in the smoke root's runs.txt for
// tools/emulator.sh. An edition's one disc has its sessions in the root.
func (p project) smokePrepare(config *projectConfig) error {
	discs := smokeDiscs
	if p.edition != "" {
		discs = []smokeDisc{p.editionDisc()}
	} else if info, err := os.Stat(filepath.Join(p.root, targetInput)); err != nil || info.Size() != targetInputBytes {
		return fmt.Errorf("make smoke needs %s at the repo root", targetInput)
	}
	root := p.smokeRoot()
	var runs []string
	for _, disc := range discs {
		if disc.cue != "" {
			if _, err := os.Stat(filepath.Join(p.root, disc.cue)); err != nil {
				return fmt.Errorf("no %s disc: run %s first", disc.name, disc.hint)
			}
		}
		dir := root
		if p.edition == "" {
			dir = filepath.Join(root, disc.name)
		}
		cue, count, err := p.prepareRunDisc(config, disc, dir)
		if err != nil {
			return err
		}
		fmt.Fprintf(p.stdout(), "smoke: %s disc %s, %d names from build/%s\n", disc.name, cue, count, disc.link)
		card := "-"
		for _, session := range smokeSessions {
			sessionDir := filepath.Join(dir, session)
			if err := freshRunDir(sessionDir); err != nil {
				return err
			}
			rel, err := filepath.Rel(p.root, sessionDir)
			if err != nil {
				return err
			}
			runs = append(runs, fmt.Sprintf("smoke %s %s %s %s", rel, cue, card, session))
			// world-map continues from the save new-game writes.
			card = filepath.Join(rel, "memcard1.mcd")
		}
	}
	return os.WriteFile(filepath.Join(root, "runs.txt"), []byte(strings.Join(runs, "\n")+"\n"), 0o644)
}

// prepareRunDisc writes disc's names.tsv into dir and returns its cue sheet,
// relative to the repo root: the disc's own, or the BIN linked into dir.
func (p project) prepareRunDisc(config *projectConfig, disc smokeDisc, dir string) (string, int, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", 0, err
	}
	count, err := p.writeSmokeNames(config, disc, filepath.Join(dir, "names.tsv"))
	if err != nil {
		return "", 0, err
	}
	if disc.cue != "" {
		return disc.cue, count, nil
	}
	cue, err := p.linkSmokeOriginal(dir)
	return cue, count, err
}

// freshRunDir empties dir and writes the emulator's settings into it.
func freshRunDir(dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "pcsx.json"), []byte(smokeSettings), 0o644)
}

// linkSmokeOriginal hard-links the BIN into dir with a cue sheet beside it, so
// the original disc boots from a path no build rewrites.
func (p project) linkSmokeOriginal(dir string) (string, error) {
	bin := filepath.Join(dir, targetInput)
	if err := os.Remove(bin); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := os.Link(filepath.Join(p.root, targetInput), bin); err != nil {
		return "", err
	}
	cue := filepath.Join(dir, targetID+".cue")
	sheet := fmt.Sprintf("FILE %q BINARY\n  TRACK 01 MODE2/2352\n    INDEX 01 00:00:00\n", targetInput)
	if err := os.WriteFile(cue, []byte(sheet), 0o644); err != nil {
		return "", err
	}
	return filepath.Rel(p.root, cue)
}

// smokeName is one line of names.tsv: a symbol a module's link placed, with
// the first bytes of its code when it is code.
type smokeName struct {
	module, name string
	address      uint32
	code         []byte
}

// smokeCodeBytes is how much of a function's start a checkpoint compares
// with RAM, enough that another module's code at the address never matches.
const smokeCodeBytes = 32

// writeSmokeNames writes every symbol each module's link defines, in module
// and address order: the run looks its checkpoints up by module and name.
func (p project) writeSmokeNames(config *projectConfig, disc smokeDisc, path string) (int, error) {
	var names []smokeName
	for _, m := range config.Modules {
		elfPath := filepath.Join(p.root, "build", disc.link, m.ID, m.ID+".elf")
		found, err := readSmokeNames(m.ID, elfPath)
		if err != nil {
			return 0, fmt.Errorf("%s: %w (run %s first)", disc.name, err, disc.hint)
		}
		names = append(names, found...)
	}
	var b strings.Builder
	b.WriteString("# module\tname\taddress\tcode\n")
	for _, n := range names {
		code := "-"
		if len(n.code) > 0 {
			code = fmt.Sprintf("%x", n.code)
		}
		fmt.Fprintf(&b, "%s\t%s\t0x%08x\t%s\n", n.module, n.name, n.address, code)
	}
	return len(names), os.WriteFile(path, []byte(b.String()), 0o644)
}

// readSmokeNames lists the symbols an ELF defines in its own sections; names
// another module places are absolute there and stay out.
func readSmokeNames(module, path string) ([]smokeName, error) {
	file, err := elf.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	symbols, err := file.Symbols()
	if err != nil {
		return nil, err
	}
	var names []smokeName
	for _, symbol := range symbols {
		if symbol.Name == "" || strings.HasPrefix(symbol.Name, ".") || strings.HasPrefix(symbol.Name, "$") {
			continue
		}
		if symbol.Section == elf.SHN_UNDEF || symbol.Section >= elf.SHN_LORESERVE {
			continue
		}
		switch elf.ST_TYPE(symbol.Info) {
		case elf.STT_FUNC, elf.STT_OBJECT:
		case elf.STT_NOTYPE:
			// The compiler's local labels (LM1, gcc2_compiled.) are no names.
			if elf.ST_BIND(symbol.Info) == elf.STB_LOCAL {
				continue
			}
		default:
			continue
		}
		n := smokeName{module: module, name: symbol.Name, address: uint32(symbol.Value)}
		section := file.Sections[symbol.Section]
		end := section.Addr + section.Size
		if section.Flags&elf.SHF_EXECINSTR != 0 && section.Type != elf.SHT_NOBITS && symbol.Value >= section.Addr && symbol.Value < end {
			size := uint64(smokeCodeBytes)
			if symbol.Size > 0 && symbol.Size < size {
				size = symbol.Size
			}
			if symbol.Value+size > end {
				size = end - symbol.Value
			}
			n.code = make([]byte, size)
			if _, err := section.ReadAt(n.code, int64(symbol.Value-section.Addr)); err != nil {
				return nil, fmt.Errorf("%s: %s: %w", path, symbol.Name, err)
			}
		}
		names = append(names, n)
	}
	sort.SliceStable(names, func(i, j int) bool {
		if names[i].address != names[j].address {
			return names[i].address < names[j].address
		}
		return names[i].name < names[j].name
	})
	return names, nil
}

// emulatorRun is one line of a runs file: the script, the run's directory
// (relative to the repo root), the cue sheet, the card slot 1 takes from an
// earlier run (- for none) and the script's argument.
type emulatorRun struct {
	script, dir, cue, card, argument string
}

func readEmulatorRuns(path string) ([]emulatorRun, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var runs []emulatorRun
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 5 {
			return nil, fmt.Errorf("%s: %q is no run (script, directory, cue sheet, card, argument)", path, line)
		}
		runs = append(runs, emulatorRun{script: fields[0], dir: fields[1], cue: fields[2], card: fields[3], argument: fields[4]})
	}
	return runs, nil
}

// runResult converts a run's screenshots to PNG files and returns the first
// line of its result.txt, which the run writes when it ends.
func (p project) runResult(dir string) (string, int, error) {
	shots, err := convertSmokeShots(filepath.Join(p.root, dir))
	if err != nil {
		return "", 0, err
	}
	result := "no result: the emulator ended before the script did"
	if text, err := os.ReadFile(filepath.Join(p.root, dir, "result.txt")); err == nil {
		result = strings.TrimSpace(strings.SplitN(string(text), "\n", 2)[0])
	}
	return result, shots, nil
}

// smokeReport prints each run's result and checkpoints.
func (p project) smokeReport() error {
	runs, err := readEmulatorRuns(filepath.Join(p.smokeRoot(), "runs.txt"))
	if err != nil {
		return errors.New("no runs.txt in the smoke directory: run make smoke")
	}
	failed := 0
	for _, run := range runs {
		result, shots, err := p.runResult(run.dir)
		if err != nil {
			return err
		}
		if result != "pass" {
			failed++
		}
		fmt.Fprintf(p.stdout(), "%-30s %s (%d screenshots)\n", run.dir, result, shots)
		if checkpoints, err := os.ReadFile(filepath.Join(p.root, run.dir, "checkpoints.txt")); err == nil {
			for _, c := range strings.Split(strings.TrimSpace(string(checkpoints)), "\n") {
				if c != "" {
					fmt.Fprintf(p.stdout(), "    %s\n", c)
				}
			}
		}
	}
	if failed > 0 {
		return fmt.Errorf("smoke: %d runs did not pass (log.txt in each run's directory)", failed)
	}
	return nil
}

// smokeShotName is a screenshot the run wrote raw: its order, label, size and
// pixel depth.
var smokeShotName = regexp.MustCompile(`^(.+)\.(\d+)x(\d+)x(16|24)\.raw$`)

// convertSmokeShots writes a PNG for each raw screenshot in dir and removes
// the raw file.
func convertSmokeShots(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, nil
	}
	count := 0
	for _, entry := range entries {
		match := smokeShotName.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		width, _ := strconv.Atoi(match[2])
		height, _ := strconv.Atoi(match[3])
		depth, _ := strconv.Atoi(match[4])
		raw := filepath.Join(dir, entry.Name())
		pixels, err := os.ReadFile(raw)
		if err != nil {
			return count, err
		}
		picture, err := smokeShotImage(pixels, width, height, depth)
		if err != nil {
			return count, fmt.Errorf("%s: %w", raw, err)
		}
		out, err := os.Create(filepath.Join(dir, match[1]+".png"))
		if err != nil {
			return count, err
		}
		if err := png.Encode(out, picture); err != nil {
			out.Close()
			return count, err
		}
		if err := out.Close(); err != nil {
			return count, err
		}
		if err := os.Remove(raw); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// smokeShotImage decodes the emulator's screenshot: 16-bit pixels with red in
// the low five bits, or 24-bit red, green, blue bytes.
func smokeShotImage(pixels []byte, width, height, depth int) (image.Image, error) {
	stride := width * depth / 8
	if width <= 0 || height <= 0 || len(pixels) < stride*height {
		return nil, fmt.Errorf("%d bytes for %dx%d at %d bits", len(pixels), width, height, depth)
	}
	picture := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		row := pixels[y*stride:]
		for x := 0; x < width; x++ {
			var c color.RGBA
			if depth == 16 {
				v := binary.LittleEndian.Uint16(row[x*2:])
				c = color.RGBA{R: uint8(v&31) << 3, G: uint8(v>>5&31) << 3, B: uint8(v>>10&31) << 3, A: 255}
			} else {
				c = color.RGBA{R: row[x*3], G: row[x*3+1], B: row[x*3+2], A: 255}
			}
			picture.SetRGBA(x, y, c)
		}
	}
	return picture, nil
}
