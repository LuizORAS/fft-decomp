// codebase_map_docs.go copies the hand-written pages (docs/ and CODEBASE.md)
// into the vault at their repository paths and reads the tier and scope of
// each mechanic page. The copies are protected: a page edited or created
// inside the vault stops the next run until --pull-docs copies it back.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	mapDocsDir      = "docs"
	mapCodebaseDoc  = "CODEBASE.md"
	mapDocsManifest = ".docs.sha256" // each copy's hash, for the next run's edit check
)

// mapDoc is one hand-written file. Markdown pages get a title (the name
// wikilinks use) and, under docs/mechanics/ or docs/guides/, a kind.
type mapDoc struct {
	path    string
	content string
	title   string
	kind    string
	tier    int
	scope   []mapScopeEntry
}

// mapScopeEntry is a name prefix (battle_move matches battle_move_*) or one
// function name, with its line in the page.
type mapScopeEntry struct {
	name string
	line int
}

var (
	mapWikilinkPattern = regexp.MustCompile(`\[\[([^\]|#^]+)`)
	mapCodeSpanPattern = regexp.MustCompile("`[^`]*`")
)

// loadMapDocs reads CODEBASE.md and every file under docs/, sorted by path.
func loadMapDocs(root string) ([]*mapDoc, error) {
	paths, err := listMapFiles(root, mapDocsDir)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(root, mapCodebaseDoc)); err == nil {
		paths = append(paths, mapCodebaseDoc)
	}
	sort.Strings(paths)
	docs := make([]*mapDoc, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return nil, err
		}
		doc := &mapDoc{path: path, content: string(data)}
		if strings.HasSuffix(path, ".md") {
			doc.title = strings.TrimSuffix(filepath.Base(path), ".md")
			switch {
			case strings.HasPrefix(path, mapDocsDir+"/mechanics/"):
				doc.kind = "mechanic"
			case strings.HasPrefix(path, mapDocsDir+"/guides/"):
				doc.kind = "guide"
			}
			if err := doc.readFrontmatter(); err != nil {
				return nil, err
			}
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

// listMapFiles lists the files below root/dir as root-relative slash paths,
// skipping dot files and folders (editor and Obsidian state).
func listMapFiles(root, dir string) ([]string, error) {
	base := filepath.Join(root, filepath.FromSlash(dir))
	if _, err := os.Stat(base); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	var files []string
	err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != base && strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.IsDir() {
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(relative))
		}
		return nil
	})
	return files, err
}

// readFrontmatter reads the tier and scope properties. The scope may be a
// YAML block list or a flow list ([a, b]).
func (doc *mapDoc) readFrontmatter() error {
	lines := strings.Split(doc.content, "\n")
	if strings.TrimSpace(lines[0]) != "---" {
		return nil
	}
	inScope := false
	for i := 1; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			return nil
		}
		if inScope && strings.HasPrefix(trimmed, "- ") {
			doc.addScope(trimmed[2:], i+1)
			continue
		}
		inScope = false
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "tier":
			if value == "" {
				continue
			}
			tier, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("%s:%d: tier %q is not a number", doc.path, i+1, value)
			}
			doc.tier = tier
		case "scope":
			if strings.HasPrefix(value, "[") {
				for _, item := range strings.Split(strings.Trim(value, "[]"), ",") {
					doc.addScope(item, i+1)
				}
			} else {
				inScope = value == ""
			}
		}
	}
	return fmt.Errorf("%s: the frontmatter has no closing ---", doc.path)
}

// mapScopeRank orders the scope entries that match one function: an exact
// entry (name()) wins, then the longest prefix, and a source folder (src/x/)
// takes only what no name entry matches.
func mapScopeRank(entry string) int {
	switch {
	case entry == "":
		return 0
	case strings.HasSuffix(entry, "()"):
		return 1 << 16
	case strings.HasSuffix(entry, "/"):
		return 1
	}
	return len(entry) + 1
}

func (doc *mapDoc) addScope(item string, line int) {
	if item = strings.Trim(strings.TrimSpace(item), `"'`); item != "" {
		doc.scope = append(doc.scope, mapScopeEntry{item, line})
	}
}

// pullMapDocs compares the vault's copies with the hashes the last run wrote.
// A copy edited in the vault, or a new file under its docs/, stops the run;
// with pull, it is first copied back to the repository, unless the
// repository's file changed too. A copy deleted in the vault is written again.
func pullMapDocs(root, target string, pull bool) ([]string, error) {
	manifest, err := readMapManifest(filepath.Join(target, mapDocsManifest))
	if err != nil || manifest == nil {
		return nil, err
	}
	var edited []string
	for _, path := range sortedKeys(manifest) {
		hash, err := mapFileHash(filepath.Join(target, filepath.FromSlash(path)))
		if err != nil {
			return nil, err
		}
		if hash != "" && hash != manifest[path] {
			edited = append(edited, path)
		}
	}
	created, err := listMapFiles(target, mapDocsDir)
	if err != nil {
		return nil, err
	}
	for _, path := range created {
		if _, ok := manifest[path]; !ok {
			edited = append(edited, path)
		}
	}
	sort.Strings(edited)
	if len(edited) == 0 {
		return nil, nil
	}
	if !pull {
		return nil, fmt.Errorf("%d hand-written page(s) changed inside the vault: %s; run `make map ARGS=--pull-docs` to copy them back to the repository first, or delete them from the vault to drop the edits",
			len(edited), strings.Join(edited, ", "))
	}
	var pulled []string
	for _, path := range edited {
		source := filepath.Join(target, filepath.FromSlash(path))
		destination := filepath.Join(root, filepath.FromSlash(path))
		data, err := os.ReadFile(source)
		if err != nil {
			return nil, err
		}
		current, err := mapFileHash(destination)
		if err != nil {
			return nil, err
		}
		if current == mapHash(data) {
			continue
		}
		if current != "" && current != manifest[path] {
			return nil, fmt.Errorf("%s changed both inside the vault and in the repository; merge the two by hand, then delete the vault's copy", path)
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(destination, data, 0o644); err != nil {
			return nil, err
		}
		pulled = append(pulled, path)
	}
	return pulled, nil
}

// readMapManifest returns nil when the vault has no manifest yet.
func readMapManifest(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	manifest := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		if hash, path, ok := strings.Cut(line, "  "); ok {
			manifest[path] = hash
		}
	}
	return manifest, nil
}

// mapFileHash returns "" for a missing file.
func mapFileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return mapHash(data), nil
}

func mapHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// addDocs names the hand-written pages. They keep their repository paths, so
// a name a generated page already took is an error. It then resolves each
// mechanic's scope (a function belongs to its longest matching entry) and
// reports scope entries that match nothing, code spans that name code that
// does not exist (the QUIRKS.md rule) and wikilinks to missing pages as stale
// mentions.
func (vault *mapVault) addDocs(docs []*mapDoc) error {
	vault.docs = docs
	vault.mechanic = map[string]*mapDoc{}
	vault.mechanicEntry = map[string]string{}
	for _, doc := range docs {
		if doc.title == "" {
			continue
		}
		if vault.taken[strings.ToLower(doc.title)] {
			return fmt.Errorf("%s: the page name %q is already taken by a generated or another hand-written page", doc.path, doc.title)
		}
		vault.taken[strings.ToLower(doc.title)] = true
		vault.names["doc:"+doc.path] = doc.title
		vault.paths["doc:"+doc.path] = doc.path
	}
	names := sortedKeys(vault.index.functions)
	for _, doc := range docs {
		if doc.kind != "mechanic" {
			continue
		}
		vault.assign("scope:"+doc.path, doc.title+" scope", "scope", "scopes")
		for _, entry := range doc.scope {
			exact := strings.HasSuffix(entry.name, "()")
			folder := strings.HasSuffix(entry.name, "/")
			key := strings.TrimSuffix(entry.name, "()")
			matched := false
			for _, name := range names {
				if folder {
					if !strings.HasPrefix(vault.index.functions[name].source, key) {
						continue
					}
				} else if name != key && (exact || !strings.HasPrefix(name, key+"_")) {
					continue
				}
				matched = true
				if mapScopeRank(entry.name) > mapScopeRank(vault.mechanicEntry[name]) {
					vault.mechanic[name] = doc
					vault.mechanicEntry[name] = entry.name
				}
			}
			if !matched {
				vault.index.stale = append(vault.index.stale, mapStale{doc.path, entry.line, entry.name, "scope entry that matches no function"})
			}
		}
	}
	known := map[string]bool{"home": true, "functions.tsv": true}
	for name := range vault.taken {
		known[name] = true
	}
	for name := range mapBases {
		known[strings.ToLower(name)] = true
	}
	for _, doc := range docs {
		known[strings.ToLower(filepath.Base(doc.path))] = true
	}
	for _, doc := range docs {
		if doc.title == "" {
			continue
		}
		fenced := false
		for i, line := range strings.Split(doc.content, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				fenced = !fenced
				continue
			}
			if fenced {
				continue
			}
			for _, match := range mapBacktickPattern.FindAllStringSubmatch(line, -1) {
				// A name prefix, as in a scope entry, names its family.
				family := vault.index.hasIdentifierPrefix(match[1] + "_")
				if key, exists, ok := vault.index.resolveMention(match[1]); ok && key == "" && !exists && !family && mapCheckedMention(match[1]) {
					vault.index.stale = append(vault.index.stale, mapStale{doc.path, i + 1, match[1], "a code span names it, but nothing in the code does"})
				}
			}
			for _, match := range mapWikilinkPattern.FindAllStringSubmatch(mapCodeSpanPattern.ReplaceAllString(line, ""), -1) {
				// A table cell escapes the alias pipe: [[name\|alias]].
				name := strings.TrimSuffix(strings.TrimSpace(match[1]), "\\")
				target := strings.ToLower(name[strings.LastIndex(name, "/")+1:])
				if strings.Contains(target, "{{") || known[target] || known[strings.TrimSuffix(target, ".md")] {
					continue
				}
				vault.index.stale = append(vault.index.stale, mapStale{doc.path, i + 1, name, "wikilink to a page that does not exist"})
			}
		}
	}
	return nil
}

func (vault *mapVault) docCount(kind string) int {
	count := 0
	for _, doc := range vault.docs {
		if doc.kind == kind {
			count++
		}
	}
	return count
}

func (vault *mapVault) mechanicLink(function string) string {
	if doc := vault.mechanic[function]; doc != nil {
		return "[[" + doc.title + "]]"
	}
	return ""
}

func (vault *mapVault) mechanicTier(function string) int {
	if doc := vault.mechanic[function]; doc != nil {
		return doc.tier
	}
	return 0
}

// docsPages adds the copies, each mechanic's scope page and the manifest of
// the copies' hashes that the next run checks.
func (vault *mapVault) docsPages(pages map[string]string) {
	var manifest strings.Builder
	for _, doc := range vault.docs {
		pages[doc.path] = doc.content
		fmt.Fprintf(&manifest, "%s  %s\n", mapHash([]byte(doc.content)), doc.path)
		if doc.kind == "mechanic" {
			pages[vault.paths["scope:"+doc.path]] = vault.scopePage(doc)
		}
	}
	pages[mapDocsManifest] = manifest.String()
}

func (vault *mapVault) scopePage(doc *mapDoc) string {
	var functions []*mapFunction
	perEntry := map[string]int{}
	summarized := 0
	for _, name := range sortedKeys(vault.index.functions) {
		if vault.mechanic[name] != doc {
			continue
		}
		f := vault.index.functions[name]
		functions = append(functions, f)
		perEntry[vault.mechanicEntry[name]]++
		if proseSummary(f.summary) {
			summarized++
		}
	}
	var b strings.Builder
	b.WriteString(frontmatter(
		mapProperty{"type", "scope"},
		mapProperty{"mechanic", "[[" + doc.title + "]]"},
		mapProperty{"tier", doc.tier},
		mapProperty{"functions", len(functions)},
		mapProperty{"prose_summaries", summarized},
	))
	fmt.Fprintf(&b, "# %s scope\n\n", doc.title)
	fmt.Fprintf(&b, "The functions that the `scope` of [[%s]] matches: %d, %d with a prose summary. A function that several entries match belongs to an exact entry (`name()`), or else to the longest prefix; a source folder (`src/x/`) takes only what no name entry matches.\n\n", doc.title, len(functions), summarized)
	b.WriteString("| Scope entry | Functions |\n|---|---|\n")
	for _, entry := range doc.scope {
		b.WriteString(tableRow("`"+entry.name+"`", fmt.Sprint(perEntry[entry.name])))
	}
	if len(functions) > 0 {
		b.WriteString("\n| Function | Module | Summary |\n|---|---|---|\n")
		for _, f := range functions {
			b.WriteString(tableRow(vault.functionLink(f.name), vault.moduleLink(f.instances[0].module), firstSentence(f.summary)))
		}
	}
	return b.String()
}

// docsSection lists the hand-written pages on the home page.
func (vault *mapVault) docsSection() string {
	var b strings.Builder
	b.WriteString("\n## Documentation\n\n")
	var general, guides []string
	var mechanics []*mapDoc
	for _, doc := range vault.docs {
		switch {
		case doc.title == "" || strings.HasPrefix(doc.path, mapDocsDir+"/templates/"):
		case doc.kind == "mechanic":
			mechanics = append(mechanics, doc)
		case doc.kind == "guide":
			guides = append(guides, "[["+doc.title+"]]")
		default:
			general = append(general, "[["+doc.title+"]]")
		}
	}
	if len(general)+len(mechanics)+len(guides) == 0 {
		b.WriteString("No hand-written pages yet (`docs/`, `CODEBASE.md`).\n")
		return b.String()
	}
	b.WriteString("Hand-written pages, copied from the repository on each run: edit them in `docs/`, or here and then run `make map ARGS=--pull-docs`.\n\n")
	if len(general) > 0 {
		b.WriteString("Overview: " + strings.Join(general, " · ") + "\n\n")
	}
	if len(mechanics) > 0 {
		sort.SliceStable(mechanics, func(i, j int) bool { return mechanics[i].tier < mechanics[j].tier })
		b.WriteString("| Mechanic | Tier | Functions in scope |\n|---|---|---|\n")
		for _, doc := range mechanics {
			count := 0
			for _, owner := range vault.mechanic {
				if owner == doc {
					count++
				}
			}
			b.WriteString(tableRow("[["+doc.title+"]]", fmt.Sprint(doc.tier), fmt.Sprintf("%s: %d", vault.link("scope:"+doc.path, doc.title+" scope"), count)))
		}
		b.WriteString("\n")
	}
	if len(guides) > 0 {
		b.WriteString("Guides: " + strings.Join(guides, " · ") + "\n")
	}
	return b.String()
}
