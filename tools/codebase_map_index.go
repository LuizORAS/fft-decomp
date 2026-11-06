// codebase_map_index.go collects what the codebase map shows: every function
// with its definition, summary comment and body references, and the types,
// globals, prototypes, macros and subsystem labels declared in include/.
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type mapInstance struct {
	module  string
	addr    uint32
	size    int
	profile string
	hash    string
	change  []string // the options an edition changes it under
}

type mapFunction struct {
	name      string
	symbol    string // C definition name: the source_symbol of a shared source
	source    string // repo-relative path
	asm       string // why the function is standalone assembly
	instances []mapInstance
	text      string // the whole source file
	signature string // definition header, whitespace collapsed
	summary   string // comment directly above the definition, markers removed
	line      int    // definition line
	subsystem string // module prefix plus the next name word, e.g. battle_formula
	prototype mapSite

	calls, references, globals, types []string
	calledBy, referencedBy            []string

	locals []string    // parameter and block-level local names
	quirks []*mapQuirk // QUIRKS.md entries that name it
	debts  []mapDebt   // "Type debt" / "Port debt" comments in its source
}

type mapSite struct {
	header string // repo-relative header path
	line   int
	label  string // subsystem label of the header block
}

type mapType struct {
	name  string
	kind  string // struct, union, enum, function pointer or typedef
	text  string // the declaration as written
	site  mapSite
	users []string
}

type mapAddress struct {
	module string
	addr   uint32
}

type mapGlobal struct {
	name        string
	declaration string // rendered type with the name, e.g. `s16 g_x []`
	site        mapSite
	addresses   []mapAddress
	users       []string
}

type mapMacro struct {
	identifiers []mapMacroIdentifier
}

type mapMacroIdentifier struct {
	name string
	call bool // followed by `(` in the macro body
}

type mapHeader struct {
	path       string
	labels     []mapLabel
	types      []string
	globals    []string
	prototypes []string
}

type mapLabel struct {
	line    int
	name    string
	purpose bool // a block comment follows the label line
}

// codebaseIndex is everything the map pages are rendered from.
type codebaseIndex struct {
	config      *projectConfig
	functions   map[string]*mapFunction
	types       map[string]*mapType
	globals     map[string]*mapGlobal
	headers     map[string]*mapHeader
	prototypes  map[string]mapSite
	enumerators map[string]string // enumerator -> its named enum type, when it has one
	macros      map[string]mapMacro

	interior          map[string]string     // data name inside a function's bytes -> that function
	regions           []*mapRegion          // target/ regions kept as original bytes, but interiors
	regionByName      map[string]*mapRegion // data names starting or inside a routine region with a page
	identifiers       map[string]bool       // every identifier the code spells, outside comments
	sortedIdentifiers []string              // identifiers, sorted on first use for prefix searches
	files             map[string]bool       // repo-relative paths under src/ and include/
	fileNames         map[string]bool       // base names of those files
	functionBySource  map[string]string     // source path -> function
	headerText        map[string]string
	commentsSeen      map[string]bool // sources whose comments were collected

	comments   []mapComment
	quirksText string
	quirks     []*mapQuirkSection
	debts      []mapDebt
	stale      []mapStale
}

// mapBasicTypesHeader declares s8..u64, too common to list among the types
// a function uses.
const mapBasicTypesHeader = "include/psx/types.h"

// mapLabelPattern matches a header subsystem label such as `/* ai */`.
var mapLabelPattern = regexp.MustCompile(`(?m)^/\* ([a-z][a-z0-9 _/-]*) \*/$`)

var mapDefinePattern = regexp.MustCompile(`^\s*#\s*define\s+([A-Za-z_][A-Za-z0-9_]*)(\([^)]*\))?(.*)$`)

func buildCodebaseIndex(p project, config *projectConfig) (*codebaseIndex, error) {
	index := &codebaseIndex{
		config:      config,
		functions:   map[string]*mapFunction{},
		types:       map[string]*mapType{},
		globals:     map[string]*mapGlobal{},
		headers:     map[string]*mapHeader{},
		prototypes:  map[string]mapSite{},
		enumerators: map[string]string{},
		macros:      map[string]mapMacro{},

		interior:         map[string]string{},
		regionByName:     map[string]*mapRegion{},
		identifiers:      map[string]bool{},
		files:            map[string]bool{},
		fileNames:        map[string]bool{},
		functionBySource: map[string]string{},
		headerText:       map[string]string{},
		commentsSeen:     map[string]bool{},
	}
	if err := index.scanFiles(p.root); err != nil {
		return nil, err
	}
	if err := index.scanHeaders(p.root); err != nil {
		return nil, err
	}
	index.addConfigFunctions()
	index.addDataAddresses()
	index.scanRegions()
	for _, name := range sortedKeys(index.functions) {
		if err := index.scanFunction(p.root, index.functions[name]); err != nil {
			return nil, err
		}
	}
	index.linkUsers()
	if err := index.scanQuirksAndComments(p.root); err != nil {
		return nil, err
	}
	return index, nil
}

// scanFiles records the files under src/ and include/, so a file named in a
// comment or in QUIRKS.md can be checked.
func (index *codebaseIndex) scanFiles(root string) error {
	for _, dir := range []string{"src", "include"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			index.files[filepath.ToSlash(relative)] = true
			index.fileNames[entry.Name()] = true
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// addIdentifiers records the identifiers of a token stream.
func (index *codebaseIndex) addIdentifiers(tokens []cToken) {
	for _, token := range tokens {
		if token.kind == cIdent {
			index.identifiers[token.text] = true
		}
	}
}

func (index *codebaseIndex) addConfigFunctions() {
	for _, m := range index.config.Modules {
		for position := range m.Functions {
			f := &m.Functions[position]
			entry := index.functions[f.Name]
			if entry == nil {
				entry = &mapFunction{name: f.Name, symbol: f.sourceSymbol(), source: m.sourceOf(f), asm: f.Asm}
				index.functions[f.Name] = entry
				if _, seen := index.functionBySource[entry.source]; !seen {
					index.functionBySource[entry.source] = f.Name
				}
			}
			index.identifiers[f.Name] = true
			entry.instances = append(entry.instances, mapInstance{
				module: m.ID, addr: f.Addr, size: f.Size, profile: m.profileOf(f), hash: f.Hash, change: f.Change,
			})
		}
	}
	for _, f := range index.functions {
		sort.Slice(f.instances, func(i, j int) bool {
			if f.instances[i].module != f.instances[j].module {
				return f.instances[i].module < f.instances[j].module
			}
			return f.instances[i].addr < f.instances[j].addr
		})
		f.subsystem = mapSubsystem(f.name, f.source)
		f.prototype = index.prototypes[f.name]
	}
}

// addDataAddresses attaches data-row addresses to header globals. Data names
// that no header declares get no page: they are tables and labels that C
// never names.
func (index *codebaseIndex) addDataAddresses() {
	for _, m := range index.config.Modules {
		functions := append([]functionSpec(nil), m.Functions...)
		sort.Slice(functions, func(i, j int) bool { return functions[i].Addr < functions[j].Addr })
		for _, data := range m.Data {
			// A data name inside a function's bytes is an interior entry
			// point of that function.
			position := sort.Search(len(functions), func(i int) bool { return functions[i].Addr > data.Addr }) - 1
			if position >= 0 {
				f := functions[position]
				if data.Addr > f.Addr && data.Addr < f.Addr+uint32(f.Size) && index.functions[data.Name] == nil {
					if _, seen := index.interior[data.Name]; !seen {
						index.interior[data.Name] = f.Name
					}
				}
			}
			index.identifiers[data.Name] = true
			if global := index.globals[data.Name]; global != nil {
				global.addresses = append(global.addresses, mapAddress{module: m.ID, addr: data.Addr})
			}
		}
		for _, imported := range m.Imports {
			index.identifiers[imported.Name] = true
		}
	}
	for _, global := range index.globals {
		sort.Slice(global.addresses, func(i, j int) bool {
			if global.addresses[i].module != global.addresses[j].module {
				return global.addresses[i].module < global.addresses[j].module
			}
			return global.addresses[i].addr < global.addresses[j].addr
		})
	}
}

// mapSubsystem names a function's code subsystem from its name: the module
// prefix and the next word (battle_formula), the library directory for
// linked SDK code, and one group for the effect overlays.
func mapSubsystem(name, source string) string {
	if strings.HasPrefix(source, "src/psyq/") {
		parts := strings.Split(source, "/")
		if len(parts) > 3 {
			return "psyq_" + parts[2]
		}
	}
	if strings.HasPrefix(source, "src/effect/") {
		return "effect_overlays"
	}
	words := strings.Split(name, "_")
	if len(words) >= 2 && words[0] != "" && words[0] == strings.ToLower(words[0]) {
		return words[0] + "_" + words[1]
	}
	module := strings.Split(strings.TrimPrefix(source, "src/"), "/")[0]
	return module + "_other"
}

func (index *codebaseIndex) scanHeaders(root string) error {
	includeRoot := filepath.Join(root, "include")
	err := filepath.WalkDir(includeRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".h" {
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
		relative = filepath.ToSlash(relative)
		if err := index.scanHeader(relative, string(data)); err != nil {
			return fmt.Errorf("%s: %w", relative, err)
		}
		return nil
	})
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (index *codebaseIndex) scanHeader(path, source string) error {
	header := &mapHeader{path: path}
	index.headers[path] = header
	index.headerText[path] = source
	lines := newLineIndex(source)
	for _, match := range mapLabelPattern.FindAllStringSubmatchIndex(source, -1) {
		header.labels = append(header.labels, mapLabel{
			line: lines.line(match[0]), name: source[match[2]:match[3]], purpose: blockPurposeComment(source[match[1]:]),
		})
	}
	index.addMacros(scanMacros(source))
	tokens, err := tokenizeC(source)
	if err != nil {
		return err
	}
	index.addIdentifiers(tokens)
	site := func(offset int) mapSite {
		line := lines.line(offset)
		return mapSite{header: path, line: line, label: header.labelAt(line)}
	}
	for position := 0; position < len(tokens); {
		next, statement, definition, err := declarationStatement(tokens, position)
		if err != nil {
			return fmt.Errorf("line %d: %w", lines.line(tokens[position].start), err)
		}
		whole := tokens[position:next]
		position = next
		if definition || len(whole) == 0 {
			continue
		}
		if whole[0].text == "typedef" || isTagKeyword(whole[0].text) {
			index.recordType(source, whole, site)
			continue
		}
		for _, declaration := range parseDeclaratorList(statement, false) {
			at := site(declaration.line)
			if functionDeclarationSignature(declaration.signature) {
				if _, seen := index.prototypes[declaration.symbol]; !seen {
					index.prototypes[declaration.symbol] = at
					header.prototypes = append(header.prototypes, declaration.symbol)
				}
				continue
			}
			if _, seen := index.globals[declaration.symbol]; !seen {
				index.globals[declaration.symbol] = &mapGlobal{
					name:        declaration.symbol,
					declaration: renderSignature(declaration.signature, declaration.symbol),
					site:        at,
				}
				header.globals = append(header.globals, declaration.symbol)
			}
		}
	}
	return nil
}

// recordType indexes a typedef or a named struct/union/enum body, with the
// enumerators of an enum body. Array-size assertions are not types.
func (index *codebaseIndex) recordType(source string, statement []cToken, site func(int) mapSite) {
	typedef := statement[0].text == "typedef"
	body := -1
	for position, token := range statement {
		if token.text == "{" {
			body = position
			break
		}
	}
	kind, name := "", ""
	tagged := statement
	if typedef {
		tagged = statement[1:]
	}
	if len(tagged) > 0 && isTagKeyword(tagged[0].text) {
		kind = tagged[0].text
	}
	switch {
	case typedef && body >= 0:
		close := matchingToken(statement, body)
		if close < 0 {
			return
		}
		if bound := boundIdentifier(statement[close+1:]); bound >= 0 {
			name = statement[close+1+bound].text
		}
	case typedef:
		_, begin, _ := parseDeclarationSpecifier(statement[1:])
		declarator := statement[1+begin:]
		if bound := boundIdentifier(declarator); bound >= 0 {
			name = declarator[bound].text
		}
		if kind == "" {
			kind = "typedef"
			for position := 0; position+1 < len(declarator); position++ {
				if declarator[position].text == "(" && declarator[position+1].text == "*" {
					kind = "function pointer"
					break
				}
			}
		}
	case body >= 0 && len(statement) > 1 && statement[1].kind == cIdent:
		// A named tag body without a typedef, used for its enumerators.
		name = statement[0].text + " " + statement[1].text
	}
	if body >= 0 && kind == "enum" {
		close := matchingToken(statement, body)
		for position := body + 1; position < close; position++ {
			token := statement[position]
			previous := statement[position-1].text
			if token.kind == cIdent && (previous == "{" || previous == ",") {
				index.enumerators[token.text] = name
			}
		}
	}
	if name == "" || strings.Contains(name, "_must_be_") {
		return
	}
	if _, seen := index.types[name]; seen {
		return
	}
	end := statement[len(statement)-1].end
	if end < len(source) && source[end] == ';' {
		end++
	}
	at := site(statement[0].start)
	index.types[name] = &mapType{name: name, kind: kind, text: source[statement[0].start:end], site: at}
	index.headers[at.header].types = append(index.headers[at.header].types, name)
}

// blockPurposeComment reports whether the text after a label line opens with
// a standalone block comment (followed by a blank line) rather than the
// comment of the block's first declaration.
func blockPurposeComment(after string) bool {
	after = strings.TrimLeft(after, " \t\r\n")
	if !strings.HasPrefix(after, "/*") {
		return false
	}
	end := strings.Index(after, "*/")
	if end < 0 {
		return false
	}
	rest := strings.TrimLeft(after[end+2:], " \t\r")
	return strings.HasPrefix(rest, "\n\n") || strings.HasPrefix(rest, "\n\r\n")
}

// addMacros indexes header macros; their names and body identifiers count as
// names the code spells.
func (index *codebaseIndex) addMacros(macros map[string]mapMacro) {
	for name, macro := range macros {
		index.macros[name] = macro
		index.identifiers[name] = true
		for _, identifier := range macro.identifiers {
			index.identifiers[identifier.name] = true
		}
	}
}

func (header *mapHeader) labelAt(line int) string {
	label := ""
	for _, candidate := range header.labels {
		if candidate.line > line {
			break
		}
		label = candidate.name
	}
	return label
}

// scanMacros reads `#define` lines, joining continuations, and records the
// identifiers each body names other than its parameters.
func scanMacros(source string) map[string]mapMacro {
	macros := map[string]mapMacro{}
	lines := strings.Split(source, "\n")
	for position := 0; position < len(lines); position++ {
		line := lines[position]
		for strings.HasSuffix(strings.TrimRight(line, " \t\r"), "\\") && position+1 < len(lines) {
			line = strings.TrimSuffix(strings.TrimRight(line, " \t\r"), "\\") + " " + lines[position+1]
			position++
		}
		match := mapDefinePattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		parameters := map[string]bool{}
		for _, parameter := range strings.Split(strings.Trim(match[2], "()"), ",") {
			parameters[strings.TrimSpace(parameter)] = true
		}
		tokens, err := tokenizeC(match[3])
		if err != nil {
			continue
		}
		var macro mapMacro
		for position, token := range tokens {
			if token.kind != cIdent || parameters[token.text] {
				continue
			}
			if position > 0 && (tokens[position-1].text == "." || tokens[position-1].text == "->") {
				continue
			}
			call := position+1 < len(tokens) && tokens[position+1].text == "("
			macro.identifiers = append(macro.identifiers, mapMacroIdentifier{name: token.text, call: call})
		}
		macros[match[1]] = macro
	}
	return macros
}

// scanFunction reads the function's source: its definition, the comment
// directly above it, and what its header and body name.
func (index *codebaseIndex) scanFunction(root string, f *mapFunction) error {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f.source)))
	if err != nil {
		return err
	}
	f.text = string(data)
	if f.asm != "" {
		f.summary = asmSourceSummary(f.text)
		return nil
	}
	tokens, err := tokenizeC(f.text)
	if err != nil {
		return fmt.Errorf("%s: %w", f.source, err)
	}
	index.addIdentifiers(tokens)
	lines := newLineIndex(f.text)
	localMacros := scanMacros(f.text)
	for name, macro := range localMacros {
		index.identifiers[name] = true
		for _, identifier := range macro.identifiers {
			index.identifiers[identifier.name] = true
		}
	}
	for start := 0; start < len(tokens); {
		next, statement, definition, err := sourceDefinitionStatement(tokens, start)
		if err != nil {
			return fmt.Errorf("%s: %w", f.source, err)
		}
		if !definition || !definesSymbol(statement, f.definitionName()) {
			start = next
			continue
		}
		previousEnd := 0
		if start > 0 {
			previousEnd = tokens[start-1].end
		}
		f.summary = definitionComment(f.text[previousEnd:tokens[start].start])
		f.line = lines.line(tokens[start].start)
		headerEnd := tokens[start+len(statement)-1].end
		f.signature = strings.Join(strings.Fields(f.text[tokens[start].start:headerEnd]), " ")
		index.classify(f, tokens[start:next], localMacros)
		f.locals = index.declaredNames(f, tokens[start:next])
		return nil
	}
	return fmt.Errorf("%s: no definition of %s", f.source, f.definitionName())
}

// definitionName is the name its source defines: a shared source defines
// its source_symbol for every row that uses it.
func (f *mapFunction) definitionName() string {
	if f.symbol != "" {
		return f.symbol
	}
	return f.name
}

func definesSymbol(statement []cToken, name string) bool {
	withoutStatic := make([]cToken, 0, len(statement))
	for _, token := range statement {
		if token.text != "static" {
			withoutStatic = append(withoutStatic, token)
		}
	}
	for _, declaration := range parseDeclaratorList(withoutStatic, true) {
		if declaration.symbol == name {
			return true
		}
	}
	return false
}

// classify sorts the identifiers of a definition into calls, function
// references, globals and types. Member names after `.` or `->` are fields,
// and macros contribute the identifiers of their bodies.
func (index *codebaseIndex) classify(f *mapFunction, tokens []cToken, localMacros map[string]mapMacro) {
	calls, references, globals, types := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	var visit func(name string, call bool, depth int)
	visit = func(name string, call bool, depth int) {
		switch {
		case index.functions[name] != nil || index.regionByName[name] != nil:
			if call {
				calls[name] = true
			} else {
				references[name] = true
			}
		case index.globals[name] != nil:
			globals[name] = true
		case index.types[name] != nil:
			if index.types[name].site.header != mapBasicTypesHeader {
				types[name] = true
			}
		case index.enumerators[name] != "":
			if index.types[index.enumerators[name]] != nil {
				types[index.enumerators[name]] = true
			}
		default:
			macro, ok := localMacros[name]
			if !ok {
				macro, ok = index.macros[name]
			}
			if !ok || depth > 8 {
				return
			}
			for _, identifier := range macro.identifiers {
				if identifier.name != name {
					visit(identifier.name, identifier.call, depth+1)
				}
			}
		}
	}
	for position, token := range tokens {
		if token.kind != cIdent {
			continue
		}
		if position > 0 && (tokens[position-1].text == "." || tokens[position-1].text == "->") {
			continue
		}
		visit(token.text, position+1 < len(tokens) && tokens[position+1].text == "(", 0)
	}
	delete(calls, f.name)
	delete(references, f.name)
	for name := range calls {
		delete(references, name)
	}
	f.calls, f.references, f.globals, f.types = sortedKeys(calls), sortedKeys(references), sortedKeys(globals), sortedKeys(types)
}

func (index *codebaseIndex) linkUsers() {
	for _, name := range sortedKeys(index.functions) {
		f := index.functions[name]
		for _, callee := range f.calls {
			if target := index.functions[callee]; target != nil {
				target.calledBy = append(target.calledBy, name)
			} else {
				region := index.regionByName[callee]
				region.calledBy = appendUnique(region.calledBy, name)
			}
		}
		for _, callee := range f.references {
			if target := index.functions[callee]; target != nil {
				target.referencedBy = append(target.referencedBy, name)
			} else {
				region := index.regionByName[callee]
				region.referencedBy = appendUnique(region.referencedBy, name)
			}
		}
		for _, global := range f.globals {
			index.globals[global].users = append(index.globals[global].users, name)
		}
		for _, typeName := range f.types {
			index.types[typeName].users = append(index.types[typeName].users, name)
		}
	}
}

// asmSourceSummary returns the block comment that opens an assembly source:
// the summary of a function written in assembly, which has no C definition.
func asmSourceSummary(text string) string {
	text = strings.TrimLeft(text, " \t\r\n")
	end := strings.Index(text, "*/")
	if !strings.HasPrefix(text, "/*") || end < 0 {
		return ""
	}
	return definitionComment(text[:end+2])
}

// definitionComment returns the text of the block comment that ends the gap
// before a definition, or "" when anything but whitespace follows it.
func definitionComment(gap string) string {
	trimmed := strings.TrimRight(gap, " \t\r\n")
	if !strings.HasSuffix(trimmed, "*/") {
		return ""
	}
	start := strings.LastIndex(trimmed, "/*")
	if start < 0 {
		return ""
	}
	body := trimmed[start+2 : len(trimmed)-2]
	var lines []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "*")
		lines = append(lines, strings.TrimSpace(line))
	}
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	// Join wrapped lines into paragraphs.
	var paragraphs []string
	current := ""
	for _, line := range lines {
		if line == "" {
			if current != "" {
				paragraphs = append(paragraphs, current)
			}
			current = ""
			continue
		}
		if current == "" {
			current = line
		} else {
			current += " " + line
		}
	}
	if current != "" {
		paragraphs = append(paragraphs, current)
	}
	return strings.Join(paragraphs, "\n\n")
}

// proseSummary reports whether a summary reads as a sentence rather than an
// address or layout note (`main 0x8001d578–0x8001d5a0; ...`).
func proseSummary(summary string) bool {
	if summary == "" {
		return false
	}
	first := strings.Fields(summary)[0]
	if strings.HasPrefix(first, "0x") || first[0] < 'A' || first[0] > 'Z' {
		return false
	}
	return true
}

type lineIndex []int

func newLineIndex(source string) lineIndex {
	starts := lineIndex{0}
	for position := 0; position < len(source); position++ {
		if source[position] == '\n' {
			starts = append(starts, position+1)
		}
	}
	return starts
}

// line returns the 1-based line holding offset.
func (starts lineIndex) line(offset int) int {
	return sort.Search(len(starts), func(i int) bool { return starts[i] > offset })
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
