// smoke.go prepares and reports `make smoke`, the run of tools/smoke.lua in
// PCSX-Redux on the original disc and on the shifted test disc that
// `make build SHIFT=N` writes. The emulator runs on the host, so tools/smoke.sh
// launches it between `smoke prepare` and `smoke report`.
package main

import (
	"debug/elf"
	"encoding/binary"
	"errors"
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

// smokeDisc is one disc the run boots: the original BIN with the names of the
// identical link, or the shifted test disc with the names of its own link.
type smokeDisc struct {
	name string
	link string // build/<link>/<module>/<module>.elf
}

var smokeDiscs = []smokeDisc{{name: "original", link: "link"}, {name: "shift", link: "shift"}}

// smokeSessions are the boots of each disc, in order: a new game through the
// first battle to the save screen, which writes the session's memory card, and
// Continue from that card through the Gariland battle to the world map's
// formation screen.
var smokeSessions = []string{"new-game", "world-map"}

// smokeSettings is the emulator's portable configuration for a session: the
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
	if len(args) == 1 {
		switch args[0] {
		case "prepare":
			return p.smokePrepare()
		case "report":
			return p.smokeReport()
		}
	}
	return errors.New("usage: tools smoke prepare|report")
}

func (p project) smokeRoot() string { return filepath.Join(p.root, "build", "smoke") }

// smokePrepare writes build/smoke/<disc>/names.tsv from each disc's link,
// links the original BIN with a cue sheet, gives every session a fresh
// directory with its settings and lists the runs in build/smoke/runs.txt:
// disc, session and cue sheet.
func (p project) smokePrepare() error {
	config, err := p.loadProjectConfig()
	if err != nil {
		return err
	}
	input := filepath.Join(p.root, targetInput)
	if info, err := os.Stat(input); err != nil || info.Size() != targetInputBytes {
		return fmt.Errorf("make smoke needs %s at the repo root", targetInput)
	}
	shiftCue := filepath.Join("build", "shift", "disc", targetOutput+".cue")
	if _, err := os.Stat(filepath.Join(p.root, shiftCue)); err != nil {
		return errors.New("no shifted disc: run make build SHIFT=16 first")
	}
	root := p.smokeRoot()
	var runs []string
	for _, disc := range smokeDiscs {
		dir := filepath.Join(root, disc.name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		count, err := p.writeSmokeNames(config, disc, filepath.Join(dir, "names.tsv"))
		if err != nil {
			return err
		}
		cue := shiftCue
		if disc.name == "original" {
			if cue, err = p.linkSmokeOriginal(dir); err != nil {
				return err
			}
		}
		fmt.Fprintf(p.stdout(), "smoke: %s disc %s, %d names from build/%s\n", disc.name, cue, count, disc.link)
		for _, session := range smokeSessions {
			sessionDir := filepath.Join(dir, session)
			if err := os.RemoveAll(sessionDir); err != nil {
				return err
			}
			if err := os.MkdirAll(sessionDir, 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(sessionDir, "pcsx.json"), []byte(smokeSettings), 0o644); err != nil {
				return err
			}
			runs = append(runs, fmt.Sprintf("%s %s %s", disc.name, session, cue))
		}
	}
	return os.WriteFile(filepath.Join(root, "runs.txt"), []byte(strings.Join(runs, "\n")+"\n"), 0o644)
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
			return 0, fmt.Errorf("%s: %w (run make build and make build SHIFT=16 first)", disc.name, err)
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

// smokeShotName is a screenshot the run wrote raw: its order, label, size and
// pixel depth.
var smokeShotName = regexp.MustCompile(`^(.+)\.(\d+)x(\d+)x(16|24)\.raw$`)

// smokeReport turns each session's screenshots into PNG files and prints its
// result: the first line of result.txt, which the run writes when it ends.
func (p project) smokeReport() error {
	root := p.smokeRoot()
	data, err := os.ReadFile(filepath.Join(root, "runs.txt"))
	if err != nil {
		return errors.New("no build/smoke/runs.txt: run make smoke")
	}
	failed := 0
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		dir := filepath.Join(root, fields[0], fields[1])
		shots, err := convertSmokeShots(dir)
		if err != nil {
			return err
		}
		result := "no result: the emulator ended before the script did"
		if text, err := os.ReadFile(filepath.Join(dir, "result.txt")); err == nil {
			result = strings.TrimSpace(strings.SplitN(string(text), "\n", 2)[0])
		}
		if result != "pass" {
			failed++
		}
		fmt.Fprintf(p.stdout(), "%-8s %-9s %s (%d screenshots in build/smoke/%s/%s)\n", fields[0], fields[1], result, shots, fields[0], fields[1])
		if checkpoints, err := os.ReadFile(filepath.Join(dir, "checkpoints.txt")); err == nil {
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
