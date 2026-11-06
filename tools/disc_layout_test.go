package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testDiscLayoutYAML = `# The test disc.
` + testDiscHeaderYAML + `
entries:
  - {lba: 23, file: SYSTEM.CNF, size: 68, date: 19970912162300}
  - {lba: 24, file: SCUS_942.21, date: 19971017183704}
  - {lba: 26, file: EMPTY.BIN, size: 0, date: 19971017183704}
  - {lba: 27, dummy: 3, form: 2}
  - {lba: 30, dir: MAP, date: 19971017183721, fixed: "a GNS row stores each file's sector"}
  - {lba: 31, file: MAP/MAP001.GNS, size: 2049, date: 19971017183721}
  - {lba: 33, dir: OPEN, date: 19971017183721}
  - {lba: 34, dummy: 2, form: 2}
  - {lba: 36, file: OPEN/FFTST.STR, size: 4672, type: mixed, date: 19971014194710}
  - {lba: 38, dummy: 10, form: 1, free: true}
`

func discTestProject(t *testing.T, disc string) project {
	return discTestProjectWith(t, testMainYAML, disc)
}

func discTestProjectWith(t *testing.T, main, disc string) project {
	return testProject(t, map[string]string{
		"target/main.yaml":       main,
		discLayoutPath:           disc,
		"src/main/main_first.c":  "void main_first(void) {}\n",
		"src/main/main_second.c": "void main_second(void) {}\n",
	})
}

func TestDiscLayoutRoundTripsAndPlacesModules(t *testing.T) {
	p := discTestProject(t, testDiscLayoutYAML)
	config, err := p.loadProjectConfig()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.validateProjectConfig(config); err != nil {
		t.Fatal(err)
	}
	if got := string(renderDiscLayout(config.Disc)); got != testDiscLayoutYAML {
		t.Fatalf("not canonical:\n%s", got)
	}
	if main, _ := config.module("main"); main.LBA != 24 {
		t.Fatalf("main at sector %d", main.LBA)
	}
}

func TestDiscLayoutRejectsInvalidEntries(t *testing.T) {
	for name, edit := range map[string][2]string{
		"gap after a dummy":     {"dummy: 3,", "dummy: 2,"},
		"file overlaps":         {"size: 68,", "size: 2049,"},
		"module size":           {"file: SCUS_942.21,", "file: SCUS_942.21, size: 4096,"},
		"data file needs size":  {"file: SYSTEM.CNF, size: 68,", "file: SYSTEM.CNF,"},
		"undeclared directory":  {"file: OPEN/FFTST.STR", "file: MOVIE/FFTST.STR"},
		"mixed sector size":     {"size: 4672,", "size: 4673,"},
		"dummy form":            {"form: 1,", "form: 3,"},
		"two free dummies":      {"dummy: 3, form: 2}", "dummy: 3, form: 2, free: true}"},
		"free file":             {"date: 19970912162300}", "date: 19970912162300, free: true}"},
		"before the root":       {"{lba: 23,", "{lba: 21,"},
		"two kinds in one row":  {"dummy: 3, form: 2}", "dummy: 3, form: 2, file: X.BIN}"},
		"fixed file":            {"size: 0,", "size: 0, fixed: no,"},
		"missing module's file": {"  - {lba: 24, file: SCUS_942.21, date: 19971017183704}\n", "  - {lba: 24, dummy: 2, form: 2}\n"},
	} {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(testDiscLayoutYAML, edit[0]) {
				t.Fatalf("edit %q does not apply", edit[0])
			}
			p := discTestProject(t, strings.Replace(testDiscLayoutYAML, edit[0], edit[1], 1))
			config, err := p.loadProjectConfig()
			if err == nil {
				err = p.validateProjectConfig(config)
			}
			if err == nil || !strings.Contains(err.Error(), discLayoutPath) {
				t.Fatalf("invalid layout accepted: %v", err)
			}
		})
	}
	p := discTestProject(t, testDiscLayoutYAML)
	if err := os.Remove(filepath.Join(p.root, filepath.FromSlash(discLayoutPath))); err != nil {
		t.Fatal(err)
	}
	config, err := p.loadProjectConfig()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.validateProjectConfig(config); err == nil || !strings.Contains(err.Error(), "disc.yaml is missing") {
		t.Fatalf("missing layout: %v", err)
	}
}

func TestDiscLayoutFitMovesWhatOutgrowsItsPlace(t *testing.T) {
	free := testDiscLayoutYAML
	// fit applies each edit to the layout or to main's YAML and runs
	// config-fmt; it returns the layout written, or config-fmt's error,
	// which check-config reports too.
	fit := func(t *testing.T, disc string, edits ...[2]string) (string, *projectConfig, error) {
		t.Helper()
		main := testMainYAML
		for _, edit := range edits {
			switch {
			case strings.Contains(disc, edit[0]):
				disc = strings.Replace(disc, edit[0], edit[1], 1)
			case strings.Contains(main, edit[0]):
				main = strings.Replace(main, edit[0], edit[1], 1)
			default:
				t.Fatalf("edit %q does not apply", edit[0])
			}
		}
		p := discTestProjectWith(t, main, disc)
		if err := p.configFormat(); err != nil {
			config, loadErr := p.loadProjectConfig()
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			if invalid := p.validateProjectConfig(config); invalid == nil || !strings.Contains(invalid.Error(), err.Error()) {
				t.Fatalf("check-config does not report %q: %v", err, invalid)
			}
			return "", nil, err
		}
		got, err := os.ReadFile(filepath.Join(p.root, filepath.FromSlash(discLayoutPath)))
		if err != nil {
			t.Fatal(err)
		}
		config, err := p.loadProjectConfig()
		if err != nil {
			t.Fatal(err)
		}
		if err := p.validateProjectConfig(config); err != nil {
			t.Fatalf("fitted layout: %v", err)
		}
		return string(got), config, nil
	}
	replace := func(text string, pairs ...string) string {
		for index := 0; index < len(pairs); index += 2 {
			if !strings.Contains(text, pairs[index]) {
				t.Fatalf("%q is not in the layout", pairs[index])
			}
			text = strings.Replace(text, pairs[index], pairs[index+1], 1)
		}
		return text
	}
	for _, test := range []struct {
		name  string
		edits [][2]string
		want  string
		main  int // main's sector after the fit
	}{
		{"a module that outgrows its place moves to the free run", [][2]string{{"size: 4096\n", "size: 6000\n"}}, replace(free,
			"  - {lba: 24, file: SCUS_942.21, date: 19971017183704}\n", "  - {lba: 24, dummy: 2, form: 2}\n",
			"  - {lba: 38, dummy: 10, form: 1, free: true}\n", "  - {lba: 38, file: SCUS_942.21, date: 19971017183704}\n  - {lba: 41, dummy: 7, form: 1, free: true}\n"), 38},
		{"a file grows into the dummy after it", [][2]string{{"size: 0,", "size: 4096,"}}, replace(free,
			"size: 0,", "size: 4096,", "{lba: 27, dummy: 3,", "{lba: 28, dummy: 2,"), 24},
		{"a fixed file shrinks in place", [][2]string{{"size: 2049,", "size: 100,"}}, replace(free,
			"size: 2049,", "size: 100,", "  - {lba: 33, dir: OPEN", "  - {lba: 32, dummy: 1, form: 2}\n  - {lba: 33, dir: OPEN"), 24},
		{"a file placed by hand in the free run", [][2]string{{"{lba: 26, file: EMPTY.BIN", "{lba: 44, file: EMPTY.BIN"}}, replace(free,
			"  - {lba: 26, file: EMPTY.BIN, size: 0, date: 19971017183704}\n", "",
			"{lba: 27, dummy: 3,", "{lba: 26, dummy: 4,",
			"  - {lba: 38, dummy: 10, form: 1, free: true}\n", "  - {lba: 38, dummy: 6, form: 1}\n  - {lba: 44, file: EMPTY.BIN, size: 0, date: 19971017183704}\n  - {lba: 45, dummy: 3, form: 1, free: true}\n"), 24},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, config, err := fit(t, free, test.edits...)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("layout:\n%s", got)
			}
			if main, _ := config.module("main"); main.LBA != test.main {
				t.Fatalf("main at sector %d", main.LBA)
			}
		})
	}
	for name, test := range map[string]struct {
		disc  string
		edit  [2]string
		error string
	}{
		"fixed directory": {free, [2]string{"size: 2049,", "size: 4097,"}, "MAP/MAP001.GNS needs the sectors up to 34, but OPEN/ starts at 33 and its directory is fixed"},
		"free run full":   {free, [2]string{"size: 4096\n", "size: 30000\n"}, "SCUS_942.21 needs the sectors up to 53, past the free run's end at 48"},
		"no free run":     {strings.Replace(free, ", free: true}", "}", 1), [2]string{"size: 4096\n", "size: 6000\n"}, "SCUS_942.21 outgrows its place, and no dummy is free"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := fit(t, test.disc, test.edit); err == nil || err.Error() != discLayoutPath+": "+test.error {
				t.Fatalf("config-fmt: %v", err)
			}
		})
	}
}

const testDiscWordsYAML = "\ndisc:\n  - {addr: 0x80010300, extent: SYSTEM.CNF}\n  - {addr: 0x80010308, sector: MAP/MAP001.GNS, offset: 1}\n  - {addr: 0x8001030c, size: MAP/MAP001.GNS}\n"

func TestDiscWordsTakeTheirValuesFromTheLayout(t *testing.T) {
	load := func(main string) (*projectConfig, error) {
		p := testProject(t, map[string]string{
			"target/main.yaml":       main,
			discLayoutPath:           testDiscLayoutYAML,
			"src/main/main_first.c":  "void main_first(void) {}\n",
			"src/main/main_second.c": "void main_second(void) {}\n",
		})
		config, err := p.loadProjectConfig()
		if err != nil {
			t.Fatal(err)
		}
		return config, p.validateProjectConfig(config)
	}
	config, err := load(testMainYAML + testDiscWordsYAML)
	if err != nil {
		t.Fatal(err)
	}
	main, _ := config.module("main")
	var got []string
	for _, word := range main.DiscWords {
		got = append(got, fmt.Sprint(word.values))
	}
	if strings.Join(got, " ") != "[23 2048] [32] [4096]" {
		t.Fatalf("disc words %v", got)
	}
	if string(renderModuleConfig(main)) != testMainYAML+testDiscWordsYAML {
		t.Fatalf("not canonical:\n%s", renderModuleConfig(main))
	}
	for name, edit := range map[string][2]string{
		"unknown file":      {"extent: SYSTEM.CNF", "extent: NOPE.BIN"},
		"offset on extent":  {"extent: SYSTEM.CNF}", "extent: SYSTEM.CNF, offset: 1}"},
		"offset past file":  {"offset: 1}", "offset: 2}"},
		"in code":           {"0x80010300, extent", "0x80010000, extent"},
		"overlap":           {"0x80010308, sector", "0x80010304, sector"},
		"two kinds":         {"size: MAP/MAP001.GNS}", "size: MAP/MAP001.GNS, sector: SYSTEM.CNF}"},
		"overlaps a value":  {"\ndisc:", "\nvalues:\n  - {addr: 0x8001030c, why: a count}\n\ndisc:"},
		"misaligned":        {"0x8001030c, size", "0x8001030e, size"},
		"outside the file":  {"0x8001030c, size", "0x80020000, size"},
		"size with offsets": {"size: MAP/MAP001.GNS}", "size: MAP/MAP001.GNS, offset: 1}"},
	} {
		t.Run(name, func(t *testing.T) {
			text := testMainYAML + testDiscWordsYAML
			if !strings.Contains(text, edit[0]) {
				t.Fatalf("edit %q does not apply", edit[0])
			}
			if _, err := load(strings.Replace(text, edit[0], edit[1], 1)); err == nil || !strings.Contains(err.Error(), "disc word") {
				t.Fatalf("invalid disc word accepted: %v", err)
			}
		})
	}
}

func TestDiscHeaderGivesTheNamesTheCodeUses(t *testing.T) {
	p := discTestProject(t, testDiscLayoutYAML)
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(p.root, filepath.FromSlash(name)), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Code and a #define name sectors and a length; a comment, a string and
	// the sector size name no file.
	write("src/main/main_first.c", "/* DISC_SECTOR_SYSTEM_CNF */\n#define LAST (DISC_SECTOR_OPEN_FFTST_STR + DISC_SECTOR_COUNT_OPEN_FFTST_STR - 1)\n"+
		"void main_first(void) { load(DISC_SECTOR_SCUS_942_21, 4 * DISC_SECTOR_BYTES, \"DISC_SECTOR_NOPE\"); }\n")
	if err := p.configFormat(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(p.root, filepath.FromSlash(discHeaderPath)))
	if err != nil {
		t.Fatal(err)
	}
	enums := `
typedef enum disc_sector {
    DISC_SECTOR_SCUS_942_21 = 24,
    DISC_SECTOR_OPEN_FFTST_STR = 36,
} disc_sector_e;

typedef enum disc_sector_count {
    DISC_SECTOR_COUNT_OPEN_FFTST_STR = 2,
} disc_sector_count_e;

#endif
`
	if !strings.HasPrefix(string(got), "#ifndef DISC_H\n") || !strings.HasSuffix(string(got), "#define DISC_SECTOR_BYTES 2048\n"+enums) {
		t.Fatalf("header:\n%s", got)
	}
	validate := func() error {
		config, err := p.loadProjectConfig()
		if err != nil {
			t.Fatal(err)
		}
		return p.validateProjectConfig(config)
	}
	if err := validate(); err != nil {
		t.Fatal(err)
	}
	write(discHeaderPath, strings.Replace(string(got), "= 36,", "= 37,", 1))
	if err := validate(); err == nil || !strings.Contains(err.Error(), "run `make config-fmt`") {
		t.Fatalf("edited header: %v", err)
	}
	write("src/main/main_second.c", "void main_second(void) { load(DISC_SECTOR_NOPE_BIN); }\n")
	if err := validate(); err == nil || !strings.Contains(err.Error(), "DISC_SECTOR_NOPE_BIN names no file") {
		t.Fatalf("unknown name: %v", err)
	}
}

func TestDiscLayoutWritesTheMkpsxisoProject(t *testing.T) {
	p := discTestProject(t, testDiscLayoutYAML)
	config, err := p.loadProjectConfig()
	if err != nil {
		t.Fatal(err)
	}
	got := string(config.Disc.mkpsxisoProject(config, "/files", map[string]string{"main": "/out/SCUS_942.21"},
		map[string]string{"MAP/MAP001.GNS": "/dc/files/MAP/MAP001.GNS"}))
	want := `<iso_project image_name="mkpsxiso.bin" cue_sheet="mkpsxiso.cue">
    <track type="data" cdvd_style="false">
        <identifiers system="PLAYSTATION" application="PLAYSTATION" volume="TEST" volume_set="TEST" publisher="TEST" data_preparer="TEST" creation_date="1997101718372100+36" modification_date="1997101718372100+36"/>
        <license file="/files/license_data.dat"/>
        <default_attributes gmt_offs="36" xa_attrib="56" xa_perm="1" xa_gid="10752" xa_uid="10752" hidden="1"/>
        <directory_tree xa_perm="1365" xa_gid="0" xa_uid="0" hidden="0" offs="22">
            <file name="SYSTEM.CNF" source="/files/SYSTEM.CNF" type="data" date="19970912162300" offs="23"/>
            <file name="SCUS_942.21" source="/out/SCUS_942.21" type="data" date="19971017183704" offs="24"/>
            <file name="EMPTY.BIN" source="/files/EMPTY.BIN" type="data" date="19971017183704" offs="26"/>
            <dummy sectors="3" type="32" offs="27"/>
            <dir name="MAP" source="/files/MAP" date="19971017183721" xa_perm="1365" xa_gid="0" xa_uid="0" offs="30">
                <file name="MAP001.GNS" source="/dc/files/MAP/MAP001.GNS" type="data" date="19971017183721" offs="31"/>
            </dir>
            <dir name="OPEN" source="/files/OPEN" date="19971017183721" xa_perm="1365" xa_gid="0" xa_uid="0" offs="33">
                <dummy sectors="2" type="32" offs="34"/>
                <file name="FFTST.STR" source="/files/OPEN/FFTST.STR" type="mixed" date="19971014194710" offs="36"/>
                <dummy sectors="10" type="0" offs="38"/>
            </dir>
        </directory_tree>
    </track>
</iso_project>
`
	if got != want {
		t.Fatalf("project:\n%s", got)
	}
}
