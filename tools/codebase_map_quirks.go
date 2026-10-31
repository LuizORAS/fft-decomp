// codebase_map_quirks.go connects the codebase map to QUIRKS.md and to the
// comments in src/ and include/: each QUIRKS entry is linked to the symbols it
// names, "Type debt" and "Port debt" comments are collected for the build-3
// backlog, and names that no longer exist in the code are reported as stale.
package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const mapQuirksFile = "QUIRKS.md"

type mapQuirkSection struct {
	title   string
	intro   string
	entries []*mapQuirk
}

type mapQuirk struct {
	section  string
	line     int
	text     string   // the bullet with its continuation lines, as written
	keys     []string // pages of the symbols and files it names
	missing  []string // names that exist nowhere in the code
	function []string // functions it names, for their pages
}

type mapComment struct {
	path      string
	line      int
	text      string // markers removed, whitespace collapsed
	following string // the next non-blank line, which a comment usually describes
	function  string // the function whose source holds it, if any
}

type mapStale struct {
	path string
	line int
	name string
	why  string
}

// mapQuirkSwitches names the build-3 compile switch each QUIRKS section feeds.
var mapQuirkSwitches = map[string]string{
	"Retail bugs the source reproduces":   "FIX_BUGS",
	"Calls that disagree with the callee": "PORT, NON_MATCHING",
	"Declaration leads":                   "NON_MATCHING (type debt)",
	"Pointers held in 32-bit integers":    "PORT (64-bit)",
	"Duplicated code":                     "NON_MATCHING (refactor)",
	"Other surprises":                     "review",
	quirksAssemblySection:                 "PORT (C twin)",
}

var (
	mapBacktickPattern = regexp.MustCompile("`([^`]+)`")
	mapDebtPattern     = regexp.MustCompile(`\b(Type|Port) debt\b`)
	// mapSymbolPattern finds project symbol names written in prose.
	mapSymbolPattern = regexp.MustCompile(`\b(?:g_|D_)?(?:battle|world|wldcore|main|open|effect|attack|bunit|card|debugchr|equip|etc|helpmenu|jobstts|option|require|small)_[a-z0-9_]*[a-z0-9]\b(?:\.[ch]\b)?`)
	// mapFamilyPattern finds a family of names written as a prefix and `*`.
	mapFamilyPattern  = regexp.MustCompile(`\b(?:g_|D_)?(?:battle|world|wldcore|main|open|effect|attack|bunit|card|debugchr|equip|etc|helpmenu|jobstts|option|require|small)_[a-z0-9_]*\*`)
	mapIdentifierText = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	mapWordPattern    = regexp.MustCompile(`\b(?:[A-Za-z_][A-Za-z0-9_]{3,}|0x[0-9a-fA-F]{2,})\b`)
)

// scanComments returns the block comments of a C file with their lines.
func scanComments(path, source string) []mapComment {
	var comments []mapComment
	lines := newLineIndex(source)
	for index := 0; index < len(source); index++ {
		switch character := source[index]; {
		case character == '"' || character == '\'':
			for index++; index < len(source) && source[index] != character && source[index] != '\n'; index++ {
				if source[index] == '\\' {
					index++
				}
			}
		case strings.HasPrefix(source[index:], "//"):
			for index < len(source) && source[index] != '\n' {
				index++
			}
		case strings.HasPrefix(source[index:], "/*"):
			end := strings.Index(source[index+2:], "*/")
			if end < 0 {
				return comments
			}
			body := source[index+2 : index+2+end]
			body = strings.ReplaceAll(body, "\n *", "\n")
			following := ""
			for _, line := range strings.Split(source[index+end+4:], "\n") {
				if trimmed := strings.TrimSpace(line); trimmed != "" {
					following = trimmed
					break
				}
			}
			comments = append(comments, mapComment{path: path, line: lines.line(index), text: strings.Join(strings.Fields(body), " "), following: following})
			index += end + 3
		}
	}
	return comments
}

// scanQuirksAndComments reads QUIRKS.md and the comments of every function
// source and header. It runs after scanFunction, which collects identifiers.
func (index *codebaseIndex) scanQuirksAndComments(root string) error {
	for _, name := range sortedKeys(index.functions) {
		f := index.functions[name]
		if index.commentsSeen[f.source] {
			continue
		}
		index.commentsSeen[f.source] = true
		for _, comment := range scanComments(f.source, f.text) {
			comment.function = name
			index.comments = append(index.comments, comment)
		}
	}
	for _, path := range sortedKeys(index.headers) {
		index.comments = append(index.comments, scanComments(path, index.headerText[path])...)
	}
	data, err := os.ReadFile(filepath.Join(root, mapQuirksFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	index.quirksText = string(data)
	index.quirks = parseQuirks(index.quirksText)
	for _, section := range index.quirks {
		for _, quirk := range section.entries {
			index.resolveQuirk(quirk)
		}
	}
	index.collectDebtsAndStale()
	return nil
}

// parseQuirks splits QUIRKS.md into `## ` sections of top-level bullets; a
// bullet keeps its indented continuation and nested lines.
func parseQuirks(source string) []*mapQuirkSection {
	var sections []*mapQuirkSection
	var section *mapQuirkSection
	var quirk *mapQuirk
	for number, line := range strings.Split(source, "\n") {
		switch {
		case strings.HasPrefix(line, "## "):
			section = &mapQuirkSection{title: strings.TrimSpace(line[3:])}
			sections = append(sections, section)
			quirk = nil
		case section == nil:
		case strings.HasPrefix(line, "- "):
			quirk = &mapQuirk{section: section.title, line: number + 1, text: line}
			section.entries = append(section.entries, quirk)
		case quirk != nil && strings.HasPrefix(line, " ") && strings.TrimSpace(line) != "":
			quirk.text += "\n" + line
		case strings.TrimSpace(line) == "":
			quirk = nil
		case quirk == nil:
			if section.intro != "" {
				section.intro += " "
			}
			section.intro += strings.TrimSpace(line)
		}
	}
	return sections
}

// resolveMention maps a name written in QUIRKS.md, a document or a comment to
// its page key, and reports whether the name exists in the code at all. ok is
// false for text that is not a symbol mention (expressions, wildcards,
// shorthand, <placeholder> paths).
func (index *codebaseIndex) resolveMention(text string) (key string, exists bool, ok bool) {
	text = strings.TrimSuffix(strings.TrimSpace(text), "()")
	if strings.ContainsAny(text, "*{}< ") || strings.Contains(text, "NNN") || strings.HasSuffix(text, "_") {
		return "", false, false
	}
	switch {
	case (strings.HasPrefix(text, "src/") || strings.HasPrefix(text, "include/")) && strings.HasSuffix(text, "/"):
		for path := range index.files {
			if strings.HasPrefix(path, text) {
				return "", true, true
			}
		}
		return "", false, true
	case strings.HasSuffix(text, ".c") || strings.HasSuffix(text, ".s") || strings.HasSuffix(text, ".h"):
		if strings.Contains(text, "/") {
			if name, found := index.functionBySource[text]; found {
				return "function:" + name, true, true
			}
			if _, found := index.headers[text]; found {
				return "header:" + text, true, true
			}
			return "", index.files[text], true
		}
		stem := strings.TrimSuffix(text, filepath.Ext(text))
		if !strings.HasSuffix(text, ".h") && index.functions[stem] != nil {
			return "function:" + stem, true, true
		}
		for path := range index.headers {
			if filepath.Base(path) == text {
				return "header:" + path, true, true
			}
		}
		return "", index.fileNames[text], true
	case strings.Contains(text, "."):
		head, field, _ := strings.Cut(text, ".")
		if !mapIdentifierText.MatchString(head) || !mapIdentifierText.MatchString(field) {
			return "", false, false
		}
		key, exists, _ := index.resolveMention(head)
		return key, exists && index.identifiers[field], true
	case !mapIdentifierText.MatchString(text):
		return "", false, false
	}
	switch {
	case index.functions[text] != nil:
		return "function:" + text, true, true
	case index.globals[text] != nil:
		return "global:" + text, true, true
	case index.types[text] != nil:
		return "type:" + text, true, true
	case index.interior[text] != "":
		return "function:" + index.interior[text], true, true
	case index.regionByName[text] != nil:
		return index.regionByName[text].key(), true, true
	}
	_, enumerator := index.enumerators[text]
	exists = index.identifiers[text] || enumerator
	if strings.HasPrefix(text, "_") && !exists {
		// Shorthand such as `_ambient_light` for a family of names.
		return "", false, false
	}
	return "", exists, true
}

func (index *codebaseIndex) resolveQuirk(quirk *mapQuirk) {
	seen := map[string]bool{}
	mentions := []string{}
	for _, match := range mapBacktickPattern.FindAllStringSubmatch(quirk.text, -1) {
		mentions = append(mentions, match[1])
	}
	mentions = append(mentions, symbolMentions(mapBacktickPattern.ReplaceAllString(quirk.text, " "))...)
	for _, mention := range mentions {
		if seen[mention] {
			continue
		}
		seen[mention] = true
		key, exists, ok := index.resolveMention(mention)
		switch {
		case !ok:
		case key != "":
			quirk.keys = appendUnique(quirk.keys, key)
			if name, isFunction := strings.CutPrefix(key, "function:"); isFunction {
				quirk.function = appendUnique(quirk.function, name)
			}
		case !exists && mapCheckedMention(mention):
			quirk.missing = appendUnique(quirk.missing, mention)
		}
	}
	for _, name := range quirk.function {
		index.functions[name].quirks = append(index.functions[name].quirks, quirk)
	}
}

// collectDebtsAndStale gathers the debt comments, the names in comments that
// exist nowhere in the code, and comments citing QUIRKS.md about something
// QUIRKS.md does not mention.
func (index *codebaseIndex) collectDebtsAndStale() {
	for _, section := range index.quirks {
		for _, quirk := range section.entries {
			for _, name := range quirk.missing {
				index.stale = append(index.stale, mapStale{path: mapQuirksFile, line: quirk.line, name: name, why: "QUIRKS.md names it, but nothing in the code does"})
			}
			for _, family := range mapFamilyPattern.FindAllString(quirk.text, -1) {
				if !strings.Contains(family, "NNN") && !index.hasIdentifierPrefix(strings.TrimSuffix(family, "*")) {
					index.stale = append(index.stale, mapStale{path: mapQuirksFile, line: quirk.line, name: family, why: "QUIRKS.md names this family, but no name in the code starts that way"})
				}
			}
		}
	}
	for _, comment := range index.comments {
		if match := mapDebtPattern.FindStringSubmatch(comment.text); match != nil {
			index.debts = append(index.debts, mapDebt{kind: strings.ToLower(match[1]), comment: comment})
			if comment.function != "" {
				f := index.functions[comment.function]
				f.debts = append(f.debts, mapDebt{kind: strings.ToLower(match[1]), comment: comment})
			}
		}
		for _, name := range symbolMentions(comment.text) {
			if _, exists, ok := index.resolveMention(name); ok && !exists {
				index.stale = append(index.stale, mapStale{path: comment.path, line: comment.line, name: name, why: "a comment names it, but nothing in the code does"})
			}
		}
		for _, family := range mapFamilyPattern.FindAllString(comment.text, -1) {
			if !strings.Contains(family, "NNN") && !index.hasIdentifierPrefix(strings.TrimSuffix(family, "*")) {
				index.stale = append(index.stale, mapStale{path: comment.path, line: comment.line, name: family, why: "a comment names this family, but no name in the code starts that way"})
			}
		}
		if strings.Contains(comment.text, "QUIRKS") && index.quirksText != "" && !index.quirksCovers(comment) {
			index.stale = append(index.stale, mapStale{path: comment.path, line: comment.line, name: mapCommentSubject(comment), why: "the comment cites QUIRKS.md, which does not mention its function or any name it gives"})
		}
	}
}

// mapCheckedMention reports whether a missing mention in backticks is a stale
// name rather than prose (an instruction, a disc file, a local variable or a
// shortened family name): only repository paths, project-prefixed symbols
// (called or not) and _t/_e types are checked.
func mapCheckedMention(mention string) bool {
	mention = strings.TrimSuffix(strings.TrimSpace(mention), "()")
	switch {
	case strings.HasPrefix(mention, "src/") || strings.HasPrefix(mention, "include/"):
		return true
	case strings.HasSuffix(mention, "_t") || strings.HasSuffix(mention, "_e"):
		return true
	}
	return mapSymbolPattern.FindString(mention) == mention
}

func (index *codebaseIndex) quirksCovers(comment mapComment) bool {
	if comment.function != "" && strings.Contains(index.quirksText, comment.function) {
		return true
	}
	if strings.Contains(index.quirksText, comment.path) || strings.Contains(index.quirksText, filepath.Base(comment.path)) {
		return true
	}
	// A comment usually describes the declaration below it, so the names and
	// hexadecimal values on that line count as well.
	for _, match := range mapWordPattern.FindAllString(comment.text+" "+comment.following, -1) {
		named := strings.Contains(match, "_") || strings.HasPrefix(match, "0x")
		if named && strings.Contains(index.quirksText, match) {
			return true
		}
	}
	return false
}

// symbolMentions returns the project symbol names written in prose, leaving
// out a prefix followed by `*`, which names a family (mapFamilyPattern).
func symbolMentions(text string) []string {
	var names []string
	for _, match := range mapSymbolPattern.FindAllStringIndex(text, -1) {
		if match[1] < len(text) && text[match[1]] == '*' {
			continue
		}
		names = append(names, text[match[0]:match[1]])
	}
	return names
}

// hasIdentifierPrefix reports whether any identifier the code spells starts
// with prefix.
func (index *codebaseIndex) hasIdentifierPrefix(prefix string) bool {
	if index.sortedIdentifiers == nil {
		index.sortedIdentifiers = sortedKeys(index.identifiers)
	}
	position := sort.SearchStrings(index.sortedIdentifiers, prefix)
	return position < len(index.sortedIdentifiers) && strings.HasPrefix(index.sortedIdentifiers[position], prefix)
}

func mapCommentSubject(comment mapComment) string {
	if comment.function != "" {
		return comment.function
	}
	return filepath.Base(comment.path)
}

func appendUnique(list []string, value string) []string {
	for _, existing := range list {
		if existing == value {
			return list
		}
	}
	return append(list, value)
}

type mapDebt struct {
	kind    string // "type" or "port"
	comment mapComment
}
