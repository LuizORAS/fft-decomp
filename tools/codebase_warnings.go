// codebase_warnings.go lists what the historical cc1 reports with -Wall for
// each function (`make warnings`). The byte-exact build compiles with -w, so
// a cleanup can add a warning without failing; listing before and after a
// change and diffing the two shows any new one. Each module gets
// build/warnings/<module>.txt (build/<edition>/warnings/ for an edition) of
// sorted "function: warning" lines, without line numbers, so an edit that
// only moves lines does not show up.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// mapWarningLine matches a cc1 diagnostic: "file:line: warning: text".
var mapWarningLine = regexp.MustCompile(`^[^:]*:[0-9]+: (warning: .*)$`)

type warningTask struct {
	module   string
	function string
	source   string
	profile  compilerProfile
}

func (p project) warningsCommand(args []string) error {
	flags := flag.NewFlagSet("warnings", flag.ContinueOnError)
	selection := flags.String("module", "", "modules to check (default: all)")
	edition := flags.String("edition", "", "edition of target/editions.yaml to compile (default: the original disc)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("usage: tools warnings [--module=M] [--edition=E]")
	}
	config, err := p.loadProjectConfig()
	if err != nil {
		return err
	}
	if p, err = p.withEdition(config, *edition); err != nil {
		return err
	}
	modules := config.Modules
	if *selection != "" {
		if modules, err = resolveModuleSelection(config, *selection); err != nil {
			return err
		}
	}
	var tasks []warningTask
	for _, m := range modules {
		for position := range m.Functions {
			f := &m.Functions[position]
			if f.Asm != "" {
				continue
			}
			profile, ok := compilerProfiles[m.profileOf(f)]
			if !ok {
				return fmt.Errorf("%s: unknown profile %q", f.Name, m.profileOf(f))
			}
			tasks = append(tasks, warningTask{module: m.ID, function: f.Name, source: m.sourceOf(f), profile: profile})
		}
	}
	jobs, err := toolJobs()
	if err != nil {
		return err
	}
	work, err := os.MkdirTemp("", "fft-warnings-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

	results := make([][]string, len(tasks))
	errs := make([]error, len(tasks))
	next := make(chan int)
	var wait sync.WaitGroup
	for worker := 0; worker < jobs; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for position := range next {
				results[position], errs[position] = p.functionWarnings(tasks[position], filepath.Join(work, fmt.Sprintf("%d.i", position)))
			}
		}()
	}
	for position := range tasks {
		next <- position
	}
	close(next)
	wait.Wait()
	if err := errors.Join(errs...); err != nil {
		return err
	}

	byModule := map[string][]string{}
	total, functions := 0, 0
	for position, task := range tasks {
		if len(results[position]) > 0 {
			functions++
		}
		for _, warning := range results[position] {
			byModule[task.module] = append(byModule[task.module], task.function+": "+warning)
			total++
		}
	}
	out := p.outputDir("warnings")
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	for _, m := range modules {
		lines := byModule[m.ID]
		sort.Strings(lines)
		text := strings.Join(lines, "\n")
		if text != "" {
			text += "\n"
		}
		if err := atomicWrite(filepath.Join(out, m.ID+".txt"), []byte(text)); err != nil {
			return err
		}
	}
	relative, _ := filepath.Rel(p.root, out)
	fmt.Fprintf(p.stdout(), "warnings: %d in %d of %d functions (%d modules) in %s\n", total, functions, len(tasks), len(modules), filepath.ToSlash(relative))
	return nil
}

// functionWarnings preprocesses and compiles one function as validate does,
// with -Wall in place of -w, and returns its warnings.
func (p project) functionWarnings(task warningTask, preprocessed string) ([]string, error) {
	command := p.cppCommand(task.source, preprocessed)
	cpp := exec.Command(command[0], command[1:]...)
	cpp.Dir = p.root
	if output, err := cpp.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("%s: preprocessing: %w\n%s", task.function, err, output)
	}
	arguments := append(cc1Command(task.profile.compilerPath), preprocessed, "-o", os.DevNull,
		fmt.Sprintf("-G%d", task.profile.smallDataLimit), "-Wall", "-funsigned-char", "-fpeephole", "-ffunction-cse",
		"-fpcc-struct-return", "-fcommon", "-msoft-float", "-quiet", "-mcpu=3000", "-fgnu-linker", "-mgas",
		task.profile.optimization)
	cc1 := exec.Command(arguments[0], arguments[1:]...)
	cc1.Dir = p.root
	output, err := cc1.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s: cc1: %w\n%s", task.function, err, output)
	}
	var warnings []string
	for _, line := range strings.Split(string(output), "\n") {
		if match := mapWarningLine.FindStringSubmatch(line); match != nil {
			warnings = append(warnings, match[1])
		}
	}
	return warnings, nil
}
