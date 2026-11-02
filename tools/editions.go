// editions.go loads, checks and canonically writes target/editions.yaml: the
// build options a source may test and the editions that combine them. The
// build without an edition is the original disc; a command given an edition
// preprocesses every source with -D<OPTION> for each of its options and
// writes its outputs under build/<edition>/. check-config also checks every
// conditional directive, so that a misspelled option fails instead of quietly
// compiling the original code.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const editionsPath = modulesConfigDir + "/editions.yaml"

// editionsFile is target/editions.yaml.
type editionsFile struct {
	Comment  string
	Options  []buildOption
	Editions []buildEdition
}

// buildOption is a name a source tests with #ifdef; Why says what code
// behind it changes.
type buildOption struct {
	Name string `yaml:"name"`
	Why  string `yaml:"why"`
}

// buildEdition is a set of options built together, under build/<Name>/.
type buildEdition struct {
	Name    string   `yaml:"name"`
	Title   string   `yaml:"title"`
	Options []string `yaml:"options"`
}

type editionsYAML struct {
	Options  []buildOption  `yaml:"options"`
	Editions []buildEdition `yaml:"editions"`
}

var (
	optionNamePattern  = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	editionNamePattern = regexp.MustCompile(`^[a-z][a-z0-9]*$`)
	// reservedEditionNames are the build/ directories of the original disc's
	// commands, which an edition's outputs must not share.
	reservedEditionNames = map[string]bool{
		"analysis": true, "bin": true, "cache": true, "diff": true, "disc": true,
		"extracted": true, "library-diff": true, "link": true, "map": true,
		"patch": true, "permute": true, "reloc-check": true, "scratch": true,
		"shift": true, "smoke": true, "symbols": true, "trial": true,
		"units": true, "warnings": true, "work": true,
	}
)

func parseEditions(data []byte) (*editionsFile, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var file editionsYAML
	if err := decoder.Decode(&file); err != nil {
		return nil, err
	}
	return &editionsFile{Comment: leadingComment(data), Options: file.Options, Editions: file.Editions}, nil
}

// renderEditions writes the canonical form: the options, then the editions,
// each in the order written, one flow mapping per line.
func renderEditions(editions *editionsFile) []byte {
	var out bytes.Buffer
	if editions.Comment != "" {
		out.WriteString(editions.Comment + "\n")
	}
	out.WriteString("options:\n")
	for _, option := range editions.Options {
		fmt.Fprintf(&out, "  - {name: %s, why: %s}\n", yamlText(option.Name), yamlText(option.Why))
	}
	out.WriteString("\neditions:\n")
	for _, edition := range editions.Editions {
		names := make([]string, len(edition.Options))
		for index, name := range edition.Options {
			names[index] = yamlText(name)
		}
		fmt.Fprintf(&out, "  - {name: %s, title: %s, options: [%s]}\n",
			yamlText(edition.Name), yamlText(edition.Title), strings.Join(names, ", "))
	}
	return out.Bytes()
}

func (editions *editionsFile) edition(name string) *buildEdition {
	for index := range editions.Editions {
		if editions.Editions[index].Name == name {
			return &editions.Editions[index]
		}
	}
	return nil
}

func (editions *editionsFile) names() []string {
	names := make([]string, len(editions.Editions))
	for index, edition := range editions.Editions {
		names[index] = edition.Name
	}
	return names
}

// validateEditions checks the names, that every option says why, and that
// each edition combines known options, each once.
func validateEditions(editions *editionsFile, fail func(string, ...any)) {
	options := map[string]bool{}
	for _, option := range editions.Options {
		switch {
		case !optionNamePattern.MatchString(option.Name):
			fail("%s: option %q is not an UPPER_CASE name", editionsPath, option.Name)
		case options[option.Name]:
			fail("%s: option %s appears twice", editionsPath, option.Name)
		case strings.TrimSpace(option.Why) == "":
			fail("%s: option %s does not say why", editionsPath, option.Name)
		}
		options[option.Name] = true
	}
	editionNames := map[string]bool{}
	for _, edition := range editions.Editions {
		switch {
		case !editionNamePattern.MatchString(edition.Name) || reservedEditionNames[edition.Name]:
			fail("%s: edition %q needs a lowercase name that no build/ directory of the original disc uses", editionsPath, edition.Name)
		case editionNames[edition.Name]:
			fail("%s: edition %s appears twice", editionsPath, edition.Name)
		case strings.TrimSpace(edition.Title) == "":
			fail("%s: edition %s has no title", editionsPath, edition.Name)
		case len(edition.Options) == 0:
			fail("%s: edition %s has no options; the build without an edition is the original disc", editionsPath, edition.Name)
		}
		editionNames[edition.Name] = true
		used := map[string]bool{}
		for _, name := range edition.Options {
			if !options[name] {
				fail("%s: edition %s names the unknown option %s", editionsPath, edition.Name, name)
			}
			if used[name] {
				fail("%s: edition %s names option %s twice", editionsPath, edition.Name, name)
			}
			used[name] = true
		}
	}
}

// withEdition returns the project that builds edition name: its options as
// preprocessor definitions and its outputs under build/<name>/. An empty name
// is the original disc.
func (p project) withEdition(config *projectConfig, name string) (project, error) {
	if name == "" {
		return p, nil
	}
	if config.Editions == nil {
		return p, fmt.Errorf("edition %q: %s is missing", name, editionsPath)
	}
	edition := config.Editions.edition(name)
	if edition == nil {
		return p, fmt.Errorf("unknown edition %q; %s declares %s", name, editionsPath, strings.Join(config.Editions.names(), ", "))
	}
	p.edition = name
	p.defines = nil
	for _, option := range edition.Options {
		p.defines = append(p.defines, "-D"+option)
	}
	return p, nil
}

// cppCommand is the preprocessor command line of every compile: the project's
// include directory and, for an edition, its options.
func (p project) cppCommand(source, output string) []string {
	command := []string{"mipsel-linux-gnu-cpp", "-P", "-undef", "-nostdinc", "-Iinclude"}
	command = append(command, p.defines...)
	return append(command, source, "-o", output)
}

// outputDir is build/<parts...> for the original disc and
// build/<edition>/<parts...> for an edition.
func (p project) outputDir(parts ...string) string {
	if p.edition != "" {
		parts = append([]string{p.edition}, parts...)
	}
	return filepath.Join(append([]string{p.root, "build"}, parts...)...)
}

// workDir is where kind's per-function intermediates go (see toolWorkDir),
// apart for each edition.
func (p project) workDir(kind string) string {
	base := toolWorkDir()
	if base == "" {
		base = filepath.Join(p.root, "build", "work")
	}
	if p.edition != "" {
		base = filepath.Join(base, p.edition)
	}
	return filepath.Join(base, kind)
}

var (
	directiveDefine      = regexp.MustCompile(`^#[ \t]*define[ \t]+([A-Za-z_][A-Za-z0-9_]*)`)
	directiveConditional = regexp.MustCompile(`^#[ \t]*(ifdef|ifndef|if|elif)\b(.*)$`)
	directiveDefined     = regexp.MustCompile(`\bdefined[ \t]*(?:\([ \t]*([A-Za-z_][A-Za-z0-9_]*)[ \t]*\)|([A-Za-z_][A-Za-z0-9_]*))`)
	directiveIdentifier  = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
)

// validateConditionals checks every #if, #ifdef, #ifndef and #elif of the C
// sources and headers: each name it tests is one the code #defines (a header
// guard, NULL) or an option of target/editions.yaml, an option is tested only
// as defined or not (#ifdef, #ifndef, defined()), never as a value, and no
// code #defines an option.
func validateConditionals(root string, editions *editionsFile) error {
	type test struct {
		where string
		name  string
		value bool
	}
	defined := map[string]string{}
	var tests []test
	for _, dir := range []string{"src", "include"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			extension := filepath.Ext(path)
			if entry.IsDir() || (extension != ".c" && extension != ".h") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			source := string(data)
			for _, directive := range cDirectives(source) {
				text := strings.TrimSpace(strings.ReplaceAll(directive.text, "\\\n", " "))
				where := fmt.Sprintf("%s:%d", filepath.ToSlash(relative), lineOf(source, directive.start))
				if match := directiveDefine.FindStringSubmatch(text); match != nil {
					if _, seen := defined[match[1]]; !seen {
						defined[match[1]] = where
					}
					continue
				}
				match := directiveConditional.FindStringSubmatch(text)
				if match == nil {
					continue
				}
				if match[1] == "ifdef" || match[1] == "ifndef" {
					if name := directiveIdentifier.FindString(match[2]); name != "" {
						tests = append(tests, test{where, name, false})
					}
					continue
				}
				for _, found := range directiveDefined.FindAllStringSubmatch(match[2], -1) {
					tests = append(tests, test{where, found[1] + found[2], false})
				}
				for _, name := range directiveIdentifier.FindAllString(directiveDefined.ReplaceAllString(match[2], " "), -1) {
					tests = append(tests, test{where, name, true})
				}
			}
			return nil
		})
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	options := map[string]bool{}
	if editions != nil {
		for _, option := range editions.Options {
			options[option.Name] = true
			if where, ok := defined[option.Name]; ok {
				tests = append(tests, test{where, option.Name, false})
			}
		}
	}
	var problems []string
	for _, t := range tests {
		_, isDefined := defined[t.name]
		switch {
		case options[t.name] && isDefined:
			problems = append(problems, fmt.Sprintf("%s: %s is an option of %s, which only an edition defines", t.where, t.name, editionsPath))
		case options[t.name] && t.value:
			problems = append(problems, fmt.Sprintf("%s: option %s is tested as a value; test it with #ifdef, #ifndef or defined()", t.where, t.name))
		case !options[t.name] && !isDefined:
			problems = append(problems, fmt.Sprintf("%s: %s is neither a name the code #defines nor an option of %s", t.where, t.name, editionsPath))
		}
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("conditional directives:\n  %s", strings.Join(slices.Compact(problems), "\n  "))
}
