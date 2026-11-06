// decl_check.go compiles every C source once with the historical cc1 in
// syntax-only mode and fails on calls to undeclared functions
// (`make decl-check`). The byte-exact build compiles with -w, so an implicit
// declaration, which still gives the right bytes for an int-returning call,
// would otherwise pass unnoticed.
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

// declCheckSource is one C source and the first module that compiles it.
type declCheckSource struct {
	source string
	module string
}

var implicitDeclaration = regexp.MustCompile("implicit declaration of function `([A-Za-z_][A-Za-z0-9_]*)'")

// declCheckSources lists each C source once, with the first module that
// compiles it; assembly rows have no C to check.
func declCheckSources(config *projectConfig) []declCheckSource {
	seen := map[string]bool{}
	var sources []declCheckSource
	for _, m := range config.Modules {
		for index := range m.Functions {
			f := &m.Functions[index]
			source := m.sourceOf(f)
			if f.Asm != "" || !strings.HasSuffix(source, ".c") || seen[source] {
				continue
			}
			seen[source] = true
			sources = append(sources, declCheckSource{source: source, module: m.ID})
		}
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].source < sources[j].source })
	return sources
}

// declCheckCommand is make decl-check; with --edition every source is
// preprocessed with that edition's options.
func (p project) declCheckCommand(args []string) error {
	flags := flag.NewFlagSet("decl-check", flag.ContinueOnError)
	edition := flags.String("edition", "", "edition of target/editions.yaml to preprocess with (default: the original disc)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("usage: tools decl-check [--edition=E]")
	}
	config, err := p.loadProjectConfig()
	if err != nil {
		return err
	}
	if p, err = p.withEdition(config, *edition); err != nil {
		return err
	}
	// Every module's preprocessed text is the same here, and both pinned
	// compilers report implicit declarations alike, so MAIN's profile serves.
	resident, ok := config.module("main")
	if !ok {
		return errors.New("decl-check: no main module")
	}
	profile, ok := compilerProfileNamed(resident.Profile)
	if !ok {
		return fmt.Errorf("decl-check: unknown compiler profile %q", resident.Profile)
	}
	sources := declCheckSources(config)
	jobs, err := toolJobs()
	if err != nil {
		return err
	}
	work, err := os.MkdirTemp("", "fft-decl-check-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

	names := make([][]string, len(sources))
	errs := make([]error, len(sources))
	next := make(chan int)
	var wait sync.WaitGroup
	for worker := 0; worker < jobs; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for position := range next {
				names[position], errs[position] = p.undeclaredCalls(sources[position], profile, filepath.Join(work, fmt.Sprintf("%d.i", position)))
			}
		}()
	}
	for position := range sources {
		next <- position
	}
	close(next)
	wait.Wait()

	count := 0
	for position, task := range sources {
		for _, name := range names[position] {
			fmt.Fprintf(p.stdout(), "%s [%s]: implicit declaration of %s\n", task.source, task.module, name)
			count++
		}
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("decl-check: %d implicit declarations", count)
	}
	fmt.Fprintf(p.stdout(), "decl-check: %d sources declare every function they call\n", len(sources))
	return nil
}

// undeclaredCalls preprocesses one source as the build does and returns the
// functions cc1 reports as implicitly declared.
func (p project) undeclaredCalls(task declCheckSource, profile compilerProfile, preprocessed string) ([]string, error) {
	cppDir, cppSource := p.preprocessInput(filepath.Join(p.root, task.source))
	if err := runInDir(cppDir, p.cppCommand(cppSource, preprocessed)...); err != nil {
		return nil, fmt.Errorf("%s: preprocessing: %w", task.source, err)
	}
	arguments := append(cc1Command(profile.compilerPath), "-quiet", "-fsyntax-only", "-Wimplicit",
		preprocessed, "-o", os.DevNull)
	cc1 := exec.Command(arguments[0], arguments[1:]...)
	cc1.Dir = p.root
	output, err := cc1.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s: cc1: %w\n%s", task.source, err, output)
	}
	var names []string
	for _, match := range implicitDeclaration.FindAllStringSubmatch(string(output), -1) {
		names = append(names, match[1])
	}
	return names, nil
}
