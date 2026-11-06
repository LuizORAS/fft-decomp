// codebase_map.go writes the codebase map (`make map`): an Obsidian vault of
// generated pages for every function, type, global, header, code subsystem
// and module, rebuilt from target/, src/ and include/ on each run. The output
// is deterministic, so two runs on one tree write identical files.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const mapDefaultOut = "build/map"

func (p project) mapCommand(args []string) error {
	flags := flag.NewFlagSet("map", flag.ContinueOnError)
	out := flags.String("out", mapDefaultOut, "vault directory, inside build/")
	pull := flags.Bool("pull-docs", false, "first copy hand-written pages edited inside the vault back to the repository")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("usage: tools map [--out=build/DIR] [--pull-docs]")
	}
	target, err := p.mapOutputDir(*out)
	if err != nil {
		return err
	}
	pulled, err := pullMapDocs(p.root, target, *pull)
	if err != nil {
		return err
	}
	for _, path := range pulled {
		fmt.Fprintf(p.stdout(), "map: pulled %s from the vault\n", path)
	}
	config, err := p.loadProjectConfig()
	if err != nil {
		return err
	}
	// The rename table's committed copy lives in docs/, so it is written
	// before docs/ is read and copied into the vault.
	renames, upstreamAvailable, upstreamErr := upstreamRenames(p.root, config)
	if upstreamErr != nil {
		fmt.Fprintf(p.stdout(), "map: %s not read, left as is: %v\n", mapUpstreamTable, upstreamErr)
	}
	if upstreamAvailable && upstreamErr == nil {
		changed, err := writeUpstreamRenames(p.root, renames)
		if err != nil {
			return err
		}
		if changed {
			fmt.Fprintf(p.stdout(), "map: %s updated\n", mapUpstreamTable)
		}
	}
	docs, err := loadMapDocs(p.root)
	if err != nil {
		return err
	}
	index, err := buildCodebaseIndex(p, config)
	if err != nil {
		return err
	}
	vault := newMapVault(index)
	if err := vault.addDocs(docs); err != nil {
		return err
	}
	vault.renames, vault.upstreamAvailable, vault.upstreamErr = renames, upstreamAvailable, upstreamErr
	pages := vault.render()
	if err := writeMapVault(target, pages); err != nil {
		return err
	}
	if _, err := writeSymbolMaps(p.root, config); err != nil {
		return err
	}
	relative, _ := filepath.Rel(p.root, target)
	fmt.Fprintf(p.stdout(), "map: %d pages (%d functions, %d types, %d globals, %d headers, %d modules) in %s\n",
		len(pages), len(index.functions), len(index.types), len(index.globals), len(index.headers),
		len(config.Modules), filepath.ToSlash(relative))
	quirks := 0
	for _, section := range index.quirks {
		quirks += len(section.entries)
	}
	fmt.Fprintf(p.stdout(), "map: %d QUIRKS.md entries, %d debt comments, %d stale mentions\n", quirks, len(index.debts), len(index.stale))
	if vault.upstreamAvailable && vault.upstreamErr == nil {
		fmt.Fprintf(p.stdout(), "map: %d names differ from upstream\n", len(vault.renames))
	}
	fmt.Fprintf(p.stdout(), "map: %d hand-written files (%d mechanics, %d guides), %d functions in a mechanic scope\n",
		len(docs), vault.docCount("mechanic"), vault.docCount("guide"), len(vault.mechanic))
	fmt.Fprintf(p.stdout(), "map: PCSX-Redux symbol maps in build/symbols\n")
	return nil
}

// mapOutputDir resolves the vault directory and refuses anything outside
// build/, because each run replaces the directory wholesale.
func (p project) mapOutputDir(out string) (string, error) {
	target := out
	if !filepath.IsAbs(target) {
		target = filepath.Join(p.root, target)
	}
	target = filepath.Clean(target)
	build := filepath.Join(p.root, "build")
	relative, err := filepath.Rel(build, target)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("map output %q must be a directory inside build/", out)
	}
	return target, nil
}

// writeMapVault writes the pages into a fresh directory and swaps it in, so
// pages of renamed or removed symbols disappear. Obsidian's own settings
// (.obsidian/) survive the swap.
func writeMapVault(target string, pages map[string]string) error {
	staging := target + ".new"
	if err := os.RemoveAll(staging); err != nil {
		return err
	}
	for _, path := range sortedKeys(pages) {
		full := filepath.Join(staging, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, []byte(pages[path]), 0o644); err != nil {
			return err
		}
	}
	settings := filepath.Join(target, ".obsidian")
	if _, err := os.Stat(settings); err == nil {
		if err := os.Rename(settings, filepath.Join(staging, ".obsidian")); err != nil {
			return err
		}
	} else {
		// A new vault starts from the core-feature configuration.
		for _, name := range sortedKeys(mapObsidianSeed) {
			full := filepath.Join(staging, ".obsidian", name)
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(full, []byte(mapObsidianSeed[name]), 0o644); err != nil {
				return err
			}
		}
	}
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	return os.Rename(staging, target)
}

// mapVault assigns every page a unique file name and renders the pages.
// Obsidian resolves [[name]] by file name, so names must be unique, also
// case-insensitively for Windows and macOS.
type mapVault struct {
	index *codebaseIndex

	renames           []mapRename // upstream names that differ from ours
	upstreamAvailable bool
	upstreamErr       error

	docs          []*mapDoc          // hand-written pages, sorted by path
	mechanic      map[string]*mapDoc // function name -> the mechanic page whose scope matches it
	mechanicEntry map[string]string  // function name -> the scope entry that matched

	names map[string]string // page key -> file name without .md
	paths map[string]string // page key -> vault-relative path
	taken map[string]bool   // lower-cased file names in use
}

func newMapVault(index *codebaseIndex) *mapVault {
	vault := &mapVault{index: index, names: map[string]string{}, paths: map[string]string{}, taken: map[string]bool{}}
	// Functions claim plain names first; other kinds take a suffix on a clash.
	for _, name := range sortedKeys(index.functions) {
		f := index.functions[name]
		folder := "functions/" + filepath.ToSlash(filepath.Dir(strings.TrimPrefix(f.source, "src/")))
		vault.assign("function:"+name, name, "function", folder)
	}
	for _, name := range sortedKeys(index.types) {
		vault.assign("type:"+name, name, "type", "types/"+mapHeaderFolder(index.types[name].site.header))
	}
	for _, name := range sortedKeys(index.globals) {
		vault.assign("global:"+name, name, "global", "globals/"+mapHeaderFolder(index.globals[name].site.header))
	}
	for _, path := range sortedKeys(index.headers) {
		name := filepath.Base(path)
		if strings.HasPrefix(path, "include/psx/") {
			name += " (psx)"
		}
		vault.assign("header:"+path, name, "header", "headers")
	}
	for _, subsystem := range vault.subsystems() {
		vault.assign("subsystem:"+subsystem, subsystem+" (subsystem)", "subsystem", "subsystems")
	}
	for _, m := range index.config.Modules {
		vault.assign("module:"+m.ID, m.ID+" (module)", "module", "modules")
	}
	for _, region := range index.regions {
		if key := region.key(); key != "" {
			vault.assign(key, region.name, "region", "regions/"+region.module)
		}
	}
	for _, section := range index.quirks {
		vault.assign("quirks:"+section.title, section.title, "quirks", "quirks")
	}
	vault.assign("report:backlog", mapBacklogPage, "report", "reports")
	vault.assign("report:stale", mapStalePage, "report", "reports")
	vault.assign("report:metrics", mapMetricsPage, "report", "reports")
	vault.assign("report:upstream", mapUpstreamPage, "report", "reports")
	return vault
}

func (vault *mapVault) assign(key, name, kind, folder string) {
	base := name
	if vault.taken[strings.ToLower(base)] {
		base = name + " (" + kind + ")"
	}
	candidate := base
	for suffix := 2; vault.taken[strings.ToLower(candidate)]; suffix++ {
		candidate = fmt.Sprintf("%s %d", base, suffix)
	}
	vault.taken[strings.ToLower(candidate)] = true
	vault.names[key] = candidate
	vault.paths[key] = folder + "/" + candidate + ".md"
}

func mapHeaderFolder(header string) string {
	if header == "" {
		return "undeclared"
	}
	return strings.TrimSuffix(strings.TrimPrefix(header, "include/"), ".h")
}

// link renders a wikilink to a page, or the plain text when it has none.
func (vault *mapVault) link(key, text string) string {
	name, ok := vault.names[key]
	if !ok {
		return "`" + text + "`"
	}
	if name == text {
		return "[[" + name + "]]"
	}
	return "[[" + name + "|" + text + "]]"
}

// tableRow renders one Markdown table row. Pipes inside a cell, including a
// wikilink's alias separator, are escaped as Obsidian requires.
func tableRow(cells ...string) string {
	escaped := make([]string, len(cells))
	for i, text := range cells {
		escaped[i] = strings.ReplaceAll(text, "|", `\|`)
	}
	return "| " + strings.Join(escaped, " | ") + " |\n"
}

func (vault *mapVault) functionLink(name string) string { return vault.link("function:"+name, name) }
func (vault *mapVault) typeLink(name string) string     { return vault.link("type:"+name, name) }
func (vault *mapVault) globalLink(name string) string   { return vault.link("global:"+name, name) }
func (vault *mapVault) moduleLink(id string) string     { return vault.link("module:"+id, id) }
func (vault *mapVault) subsystemLink(s string) string   { return vault.link("subsystem:"+s, s) }

func (vault *mapVault) headerLink(path string) string {
	return vault.link("header:"+path, strings.TrimPrefix(path, "include/"))
}

func (vault *mapVault) subsystems() []string {
	seen := map[string]bool{}
	for _, f := range vault.index.functions {
		seen[f.subsystem] = true
	}
	return sortedKeys(seen)
}

func (vault *mapVault) render() map[string]string {
	pages := map[string]string{}
	index := vault.index
	for _, name := range sortedKeys(index.functions) {
		pages[vault.paths["function:"+name]] = vault.functionPage(index.functions[name])
	}
	for _, name := range sortedKeys(index.types) {
		pages[vault.paths["type:"+name]] = vault.typePage(index.types[name])
	}
	for _, name := range sortedKeys(index.globals) {
		pages[vault.paths["global:"+name]] = vault.globalPage(index.globals[name])
	}
	for _, path := range sortedKeys(index.headers) {
		pages[vault.paths["header:"+path]] = vault.headerPage(index.headers[path])
	}
	for _, subsystem := range vault.subsystems() {
		pages[vault.paths["subsystem:"+subsystem]] = vault.subsystemPage(subsystem)
	}
	for _, m := range index.config.Modules {
		pages[vault.paths["module:"+m.ID]] = vault.modulePage(m)
	}
	for _, region := range index.regions {
		if key := region.key(); key != "" {
			pages[vault.paths[key]] = vault.regionPage(region)
		}
	}
	for _, section := range index.quirks {
		pages[vault.paths["quirks:"+section.title]] = vault.quirkSectionPage(section)
	}
	pages[vault.paths["report:backlog"]] = vault.backlogPage()
	pages[vault.paths["report:stale"]] = vault.stalePage()
	pages[vault.paths["report:metrics"]] = vault.metricsPage()
	pages[vault.paths["report:upstream"]] = vault.upstreamPage(vault.renames, vault.upstreamAvailable, vault.upstreamErr)
	for name, content := range mapBases {
		pages["bases/"+name] = content
	}
	pages["functions.tsv"] = vault.functionsTSV()
	vault.docsPages(pages)
	pages["Home.md"] = vault.homePage()
	return pages
}

// frontmatter renders Obsidian properties in the given order. Strings are
// quoted; ints and bools stay bare so Bases can sort and filter them.
type mapProperty struct {
	key   string
	value any
}

func frontmatter(properties ...mapProperty) string {
	var b strings.Builder
	b.WriteString("---\n")
	for _, property := range properties {
		switch value := property.value.(type) {
		case string:
			fmt.Fprintf(&b, "%s: %s\n", property.key, yamlQuote(value))
		case []string:
			quoted := make([]string, len(value))
			for i, item := range value {
				quoted[i] = yamlQuote(item)
			}
			fmt.Fprintf(&b, "%s: [%s]\n", property.key, strings.Join(quoted, ", "))
		default:
			fmt.Fprintf(&b, "%s: %v\n", property.key, value)
		}
	}
	b.WriteString("---\n")
	return b.String()
}

func yamlQuote(value string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `"`, `\"`) + `"`
}

// firstSentence shortens a summary for tables.
func firstSentence(summary string) string {
	text := strings.Join(strings.Fields(summary), " ")
	if end := strings.Index(text, ". "); end >= 0 {
		text = text[:end+1]
	}
	if len(text) > 200 {
		text = text[:197] + "..."
	}
	return text
}

func hex32(value uint32) string { return fmt.Sprintf("0x%08x", value) }

func joinLinks(names []string, link func(string) string) string {
	links := make([]string, len(names))
	for i, name := range names {
		links[i] = link(name)
	}
	return strings.Join(links, " · ")
}

func (vault *mapVault) functionPage(f *mapFunction) string {
	var b strings.Builder
	modules := make([]string, 0, len(f.instances))
	for _, instance := range f.instances {
		if len(modules) == 0 || modules[len(modules)-1] != instance.module {
			modules = append(modules, instance.module)
		}
	}
	first := f.instances[0]
	debtKinds := []string{}
	for _, debt := range f.debts {
		debtKinds = appendUnique(debtKinds, debt.kind)
	}
	change := []string{}
	for _, instance := range f.instances {
		for _, option := range instance.change {
			change = appendUnique(change, option)
		}
	}
	_, numbered := vault.index.numberedFamily(f.name)
	weak := f.weakNames()
	b.WriteString(frontmatter(
		mapProperty{"type", "function"},
		mapProperty{"module", first.module},
		mapProperty{"modules", modules},
		mapProperty{"address", hex32(first.addr)},
		mapProperty{"size", first.size},
		mapProperty{"subsystem", f.subsystem},
		mapProperty{"mechanic", vault.mechanicLink(f.name)},
		mapProperty{"tier", vault.mechanicTier(f.name)},
		mapProperty{"header", f.prototype.header},
		mapProperty{"header_label", f.prototype.label},
		mapProperty{"source", f.source},
		mapProperty{"has_summary", f.summary != ""},
		mapProperty{"prose_summary", proseSummary(f.summary)},
		mapProperty{"calls", len(f.calls)},
		mapProperty{"called_by", len(f.calledBy)},
		mapProperty{"referenced_by", len(f.referencedBy)},
		mapProperty{"quirks", len(f.quirks)},
		mapProperty{"change", change},
		mapProperty{"debt", debtKinds},
		mapProperty{"address_name", mapAddressName(f.name)},
		mapProperty{"numeric_suffix", numbered},
		mapProperty{"suffix_justified", vault.index.suffixJustified(f)},
		mapProperty{"weak_locals", len(weak)},
	))
	fmt.Fprintf(&b, "# %s\n\n", f.name)
	switch {
	case f.asm != "":
		fmt.Fprintf(&b, "Standalone assembly: %s\n\n", f.asm)
	case f.summary != "":
		b.WriteString(f.summary + "\n\n")
	default:
		b.WriteString("*No summary comment yet.*\n\n")
	}
	b.WriteString("| Module | Address | Size | Profile |\n|---|---|---|---|\n")
	for _, instance := range f.instances {
		b.WriteString(tableRow(vault.moduleLink(instance.module), "`"+hex32(instance.addr)+"`", fmt.Sprint(instance.size), instance.profile))
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "- Subsystem: %s\n", vault.subsystemLink(f.subsystem))
	if f.prototype.header != "" {
		fmt.Fprintf(&b, "- Declared in %s line %d", vault.headerLink(f.prototype.header), f.prototype.line)
		if f.prototype.label != "" {
			fmt.Fprintf(&b, ", block `/* %s */`", f.prototype.label)
		}
		b.WriteString("\n")
	} else {
		b.WriteString("- No prototype in include/\n")
	}
	if f.line > 0 {
		fmt.Fprintf(&b, "- Source: `%s` line %d\n", f.source, f.line)
	} else {
		fmt.Fprintf(&b, "- Source: `%s`\n", f.source)
	}
	for _, region := range vault.index.regions {
		if region.function == f.name {
			fmt.Fprintf(&b, "- Original %s code (`target/` region): %s\n", region.kind, region.why)
		}
	}
	if f.signature != "" {
		fmt.Fprintf(&b, "\n## Signature\n\n```c\n%s\n```\n", f.signature)
	}
	section := func(title string, names []string, link func(string) string) {
		if len(names) > 0 {
			fmt.Fprintf(&b, "\n## %s (%d)\n\n%s\n", title, len(names), joinLinks(names, link))
		}
	}
	section("Calls", f.calls, vault.codeLink)
	section("Uses the address of", f.references, vault.codeLink)
	section("Called by", f.calledBy, vault.functionLink)
	section("Address used by", f.referencedBy, vault.functionLink)
	if f.asm == "" && len(f.calledBy) == 0 && len(f.referencedBy) == 0 {
		b.WriteString("\n## Called by\n\nNo C code calls this function or takes its address. It may be dispatched through a function-pointer table in the original data, or be unused.\n")
	}
	section("Globals", f.globals, vault.globalLink)
	section("Types", f.types, vault.typeLink)
	if len(weak) > 0 {
		fmt.Fprintf(&b, "\n## Weak names (%d)\n\n`%s`: names that say nothing about the value (see [[%s]]).\n", len(weak), strings.Join(weak, "`, `"), mapMetricsPage)
	}
	b.WriteString(vault.quirksSection(f.quirks))
	if len(f.debts) > 0 {
		fmt.Fprintf(&b, "\n## Debt comments (%d)\n\nListed in the [[%s]].\n\n", len(f.debts), mapBacklogPage)
		for _, debt := range f.debts {
			fmt.Fprintf(&b, "- **%s debt**, line %d: %s\n", strings.ToUpper(debt.kind[:1])+debt.kind[1:], debt.comment.line, debt.comment.text)
		}
	}
	language := "c"
	if f.asm != "" {
		language = "asm"
	}
	fmt.Fprintf(&b, "\n## Source\n\n```%s\n%s\n```\n", language, strings.TrimRight(f.text, "\n"))
	return b.String()
}

func (vault *mapVault) typePage(t *mapType) string {
	var b strings.Builder
	b.WriteString(frontmatter(
		mapProperty{"type", "type"},
		mapProperty{"kind", t.kind},
		mapProperty{"header", t.site.header},
		mapProperty{"header_label", t.site.label},
		mapProperty{"users", len(t.users)},
	))
	fmt.Fprintf(&b, "# %s\n\n", t.name)
	kind := t.kind
	if kind == "" {
		kind = "type"
	}
	fmt.Fprintf(&b, "%s declared in %s line %d", strings.ToUpper(kind[:1])+kind[1:], vault.headerLink(t.site.header), t.site.line)
	if t.site.label != "" {
		fmt.Fprintf(&b, ", block `/* %s */`", t.site.label)
	}
	fmt.Fprintf(&b, ".\n\n```c\n%s\n```\n", t.text)
	if len(t.users) > 0 {
		fmt.Fprintf(&b, "\n## Used by (%d)\n\n%s\n", len(t.users), joinLinks(t.users, vault.functionLink))
	} else {
		b.WriteString("\n## Used by\n\nNo function source names this type directly.\n")
	}
	b.WriteString(vault.quirksSection(vault.quirksFor("type:" + t.name)))
	return b.String()
}

func (vault *mapVault) globalPage(g *mapGlobal) string {
	var b strings.Builder
	modules := []string{}
	for _, address := range g.addresses {
		if len(modules) == 0 || modules[len(modules)-1] != address.module {
			modules = append(modules, address.module)
		}
	}
	b.WriteString(frontmatter(
		mapProperty{"type", "global"},
		mapProperty{"modules", modules},
		mapProperty{"header", g.site.header},
		mapProperty{"header_label", g.site.label},
		mapProperty{"users", len(g.users)},
	))
	fmt.Fprintf(&b, "# %s\n\n", g.name)
	fmt.Fprintf(&b, "Declared in %s line %d", vault.headerLink(g.site.header), g.site.line)
	if g.site.label != "" {
		fmt.Fprintf(&b, ", block `/* %s */`", g.site.label)
	}
	fmt.Fprintf(&b, ".\n\n```c\nextern %s;\n```\n", g.declaration)
	if len(g.addresses) > 0 {
		b.WriteString("\n| Module | Address |\n|---|---|\n")
		for _, address := range g.addresses {
			b.WriteString(tableRow(vault.moduleLink(address.module), "`"+hex32(address.addr)+"`"))
		}
	} else {
		b.WriteString("\nNo `data` row in target/ gives this name an address (it may be a library or linker symbol).\n")
	}
	if len(g.users) > 0 {
		fmt.Fprintf(&b, "\n## Used by (%d)\n\n%s\n", len(g.users), joinLinks(g.users, vault.functionLink))
	} else {
		b.WriteString("\n## Used by\n\nNo function source names this global directly.\n")
	}
	b.WriteString(vault.quirksSection(vault.quirksFor("global:" + g.name)))
	return b.String()
}

func (vault *mapVault) headerPage(h *mapHeader) string {
	var b strings.Builder
	b.WriteString(frontmatter(
		mapProperty{"type", "header"},
		mapProperty{"path", h.path},
		mapProperty{"labels", len(h.labels)},
		mapProperty{"types", len(h.types)},
		mapProperty{"globals", len(h.globals)},
		mapProperty{"prototypes", len(h.prototypes)},
	))
	fmt.Fprintf(&b, "# %s\n\n", strings.TrimPrefix(h.path, "include/"))
	type group struct{ types, globals, prototypes []string }
	groups := map[string]*group{}
	order := []string{""}
	for _, label := range h.labels {
		if _, seen := groups[label.name]; !seen {
			order = append(order, label.name)
		}
		groups[label.name] = &group{}
	}
	groups[""] = &group{}
	index := vault.index
	for _, name := range h.types {
		g := groups[index.types[name].site.label]
		g.types = append(g.types, name)
	}
	for _, name := range h.globals {
		g := groups[index.globals[name].site.label]
		g.globals = append(g.globals, name)
	}
	for _, name := range h.prototypes {
		g := groups[index.prototypes[name].label]
		g.prototypes = append(g.prototypes, name)
	}
	b.WriteString("| Block | Line | Types | Globals | Prototypes |\n|---|---|---|---|---|\n")
	for _, label := range h.labels {
		g := groups[label.name]
		fmt.Fprintf(&b, "| [[#%s]] | %d | %d | %d | %d |\n", label.name, label.line, len(g.types), len(g.globals), len(g.prototypes))
	}
	for _, label := range order {
		g := groups[label]
		if len(g.types)+len(g.globals)+len(g.prototypes) == 0 {
			continue
		}
		title := label
		if title == "" {
			title = "Before the first label"
		}
		fmt.Fprintf(&b, "\n## %s\n", title)
		if len(g.types) > 0 {
			fmt.Fprintf(&b, "\nTypes: %s\n", joinLinks(g.types, vault.typeLink))
		}
		if len(g.globals) > 0 {
			fmt.Fprintf(&b, "\nGlobals: %s\n", joinLinks(g.globals, vault.globalLink))
		}
		if len(g.prototypes) > 0 {
			fmt.Fprintf(&b, "\nFunctions: %s\n", joinLinks(g.prototypes, vault.functionLink))
		}
	}
	return b.String()
}

func (vault *mapVault) subsystemFunctions(subsystem string) []*mapFunction {
	var functions []*mapFunction
	for _, name := range sortedKeys(vault.index.functions) {
		if f := vault.index.functions[name]; f.subsystem == subsystem {
			functions = append(functions, f)
		}
	}
	sort.SliceStable(functions, func(i, j int) bool {
		a, b := functions[i].instances[0], functions[j].instances[0]
		if a.module != b.module {
			return a.module < b.module
		}
		return a.addr < b.addr
	})
	return functions
}

func (vault *mapVault) subsystemPage(subsystem string) string {
	functions := vault.subsystemFunctions(subsystem)
	summarized := 0
	labels := map[string]int{}
	for _, f := range functions {
		if proseSummary(f.summary) {
			summarized++
		}
		if f.prototype.header != "" {
			labels[strings.TrimPrefix(f.prototype.header, "include/")+" /* "+f.prototype.label+" */"]++
		}
	}
	var b strings.Builder
	b.WriteString(frontmatter(
		mapProperty{"type", "subsystem"},
		mapProperty{"functions", len(functions)},
		mapProperty{"summarized", summarized},
	))
	fmt.Fprintf(&b, "# %s\n\n", subsystem)
	fmt.Fprintf(&b, "Code subsystem from the function names: %d functions, %d with a summary.\n", len(functions), summarized)
	if len(labels) > 0 {
		b.WriteString("\nDeclared in:\n\n")
		for _, label := range sortedKeys(labels) {
			fmt.Fprintf(&b, "- `%s`: %d\n", label, labels[label])
		}
	}
	b.WriteString("\n| Function | Module | Address | Summary |\n|---|---|---|---|\n")
	for _, f := range functions {
		b.WriteString(tableRow(vault.functionLink(f.name), f.instances[0].module, "`"+hex32(f.instances[0].addr)+"`", firstSentence(f.summary)))
	}
	return b.String()
}

func (vault *mapVault) modulePage(m *moduleSpec) string {
	var functions []*mapFunction
	subsystems := map[string]int{}
	for position := range m.Functions {
		f := vault.index.functions[m.Functions[position].Name]
		functions = append(functions, f)
		subsystems[f.subsystem]++
	}
	var b strings.Builder
	b.WriteString(frontmatter(
		mapProperty{"type", "module"},
		mapProperty{"file", m.File},
		mapProperty{"load", hex32(m.Load)},
		mapProperty{"size", m.Size},
		mapProperty{"lba", m.LBA},
		mapProperty{"functions", len(m.Functions)},
		mapProperty{"links", m.Links},
	))
	fmt.Fprintf(&b, "# %s\n\n", m.ID)
	if comment := strings.TrimSpace(m.Comment); comment != "" {
		var lines []string
		for _, line := range strings.Split(comment, "\n") {
			lines = append(lines, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "#")))
		}
		b.WriteString(strings.Join(lines, " ") + "\n\n")
	}
	fmt.Fprintf(&b, "- Disc file `%s`, LBA %d, %d bytes\n", m.File, m.LBA, m.Size)
	fmt.Fprintf(&b, "- Loaded at `%s`–`%s`\n", hex32(m.Load), hex32(uint32(m.end())))
	fmt.Fprintf(&b, "- Sources in `%s`, default profile `%s`\n", m.SourceDir, m.Profile)
	if len(m.Links) > 0 {
		fmt.Fprintf(&b, "- Resident alongside: %s\n", joinLinks(m.Links, vault.moduleLink))
	}
	if len(m.Libraries) > 0 {
		b.WriteString("\n## Linked libraries\n\n| Library | Kind | Range |\n|---|---|---|\n")
		for _, library := range m.Libraries {
			fmt.Fprintf(&b, "| %s | %s | `%s`–`%s` |\n", library.Library, library.Kind, hex32(library.Addr), hex32(library.End))
		}
	}
	b.WriteString(vault.regionsSection(m.ID))
	b.WriteString("\n## Subsystems\n\n| Subsystem | Functions |\n|---|---|\n")
	for _, subsystem := range sortedKeys(subsystems) {
		b.WriteString(tableRow(vault.subsystemLink(subsystem), fmt.Sprint(subsystems[subsystem])))
	}
	b.WriteString("\n## Functions\n\n| Address | Function | Size | Summary |\n|---|---|---|---|\n")
	for position, f := range functions {
		spec := m.Functions[position]
		b.WriteString(tableRow("`"+hex32(spec.Addr)+"`", vault.functionLink(f.name), fmt.Sprint(spec.Size), firstSentence(f.summary)))
	}
	return b.String()
}

func (vault *mapVault) homePage() string {
	index := vault.index
	summarized := 0
	for _, f := range index.functions {
		if proseSummary(f.summary) {
			summarized++
		}
	}
	var b strings.Builder
	b.WriteString(frontmatter(mapProperty{"type", "home"}))
	b.WriteString("# Final Fantasy Tactics codebase map\n\n")
	b.WriteString("Generated by `make map` from `target/`, `src/` and `include/`. Do not edit these pages: the next run replaces them. The pages under `docs/` and `CODEBASE.md` are copies of the repository's hand-written pages (see Documentation).\n\n")
	fmt.Fprintf(&b, "%d functions (%d with a summary), %d types, %d globals, %d headers, %d modules.\n",
		len(index.functions), summarized, len(index.types), len(index.globals), len(index.headers), len(index.config.Modules))
	groups := []struct {
		title string
		match func(string) bool
	}{
		{"Modules", func(id string) bool { return !strings.HasPrefix(id, "event-") && !strings.HasPrefix(id, "effect-") }},
		{"Event overlays (run with BATTLE)", func(id string) bool { return strings.HasPrefix(id, "event-") }},
		{"Effect overlays (run with BATTLE)", func(id string) bool { return strings.HasPrefix(id, "effect-") }},
	}
	for _, group := range groups {
		fmt.Fprintf(&b, "\n## %s\n\n", group.title)
		var links []string
		for _, m := range index.config.Modules {
			if group.match(m.ID) {
				links = append(links, fmt.Sprintf("%s (%d)", vault.moduleLink(m.ID), len(m.Functions)))
			}
		}
		b.WriteString(strings.Join(links, " · ") + "\n")
	}
	b.WriteString(vault.docsSection())
	b.WriteString("\n## Quirks and reports\n\n")
	var quirkLinks []string
	for _, section := range index.quirks {
		quirkLinks = append(quirkLinks, fmt.Sprintf("%s (%d)", vault.quirkSectionLink(section.title), len(section.entries)))
	}
	if len(quirkLinks) > 0 {
		b.WriteString("`QUIRKS.md`: " + strings.Join(quirkLinks, " · ") + "\n\n")
	}
	fmt.Fprintf(&b, "Reports: [[%s]] · [[%s]] (%d debt comments) · [[%s]] (%d) · [[%s]]\n", mapMetricsPage, mapBacklogPage, len(index.debts), mapStalePage, len(index.stale), mapUpstreamPage)
	b.WriteString("\nViews: [[Functions.base|Functions]] · [[Types.base|Types]] · [[Globals.base|Globals]] · `functions.tsv` (one line per function, for grep)\n")
	b.WriteString("\n## Headers\n\n")
	b.WriteString(joinLinks(sortedKeys(index.headers), vault.headerLink) + "\n")
	b.WriteString("\n## Code subsystems\n\n| Subsystem | Functions |\n|---|---|\n")
	for _, subsystem := range vault.subsystems() {
		b.WriteString(tableRow(vault.subsystemLink(subsystem), fmt.Sprint(len(vault.subsystemFunctions(subsystem)))))
	}
	return b.String()
}
