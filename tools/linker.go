// linker.go resolves the names each function's source uses and writes the
// per-function linker script that places its sections at target addresses.
package main

import (
	"debug/elf"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// undefinedObjectIdentifiers includes references introduced by header macros,
// inline helpers and the compiler, which a scan of the source file cannot see.
func undefinedObjectIdentifiers(path string) (map[string]struct{}, error) {
	object, err := elf.Open(path)
	if err != nil {
		return nil, err
	}
	defer object.Close()
	symbols, err := object.Symbols()
	if err != nil {
		return nil, err
	}
	used := make(map[string]struct{})
	for _, symbol := range symbols {
		if symbol.Section == elf.SHN_UNDEF && symbol.Name != "" {
			used[symbol.Name] = struct{}{}
		}
	}
	return used, nil
}

// sourceIdentifiers returns the C identifiers that appear as whole tokens in a
// source file: an over-approximation of the names its object can reference. A
// missing binding fails the link rather than corrupting the output.
func sourceIdentifiers(path string) (map[string]struct{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	idents := make(map[string]struct{})
	start := -1
	isWord := func(c byte) bool {
		return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
	}
	for i := 0; i <= len(data); i++ {
		if i < len(data) && isWord(data[i]) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			idents[string(data[start:i])] = struct{}{}
			start = -1
		}
	}
	return idents, nil
}

// compilerEmitted reports whether GCC can reference a symbol the source never
// names: `__main` from main()'s prologue, libgcc helpers such as `__divsi3`,
// and `memcpy`/`memset` from block moves and clears.
func compilerEmitted(name string) bool {
	if strings.HasPrefix(name, "__") {
		return true
	}
	switch name {
	case "memcpy", "memset", "memmove":
		return true
	}
	return false
}

// symbolResolver answers which address a name has for a module: the module's
// own functions, data and imports, then each linked module's functions and
// data in order, then main's. A name qualified with a module
// (qualifiedName) is that module's function or data row.
type symbolResolver struct {
	config   *projectConfig
	own      map[string]map[string]uint32
	exported map[string]map[string]uint32
	placed   map[string]map[string]bool      // the functions and data rows a module's link places in its image
	owners   map[string]map[string]linkOwner // the owner of each row or import that names another module's object (In)
}

// linkOwner is the module whose link places a name, and the symbol it has
// there.
type linkOwner struct {
	module, symbol string
}

func newSymbolResolver(config *projectConfig) symbolResolver {
	resolver := symbolResolver{config: config, own: make(map[string]map[string]uint32), exported: make(map[string]map[string]uint32),
		placed: make(map[string]map[string]bool), owners: make(map[string]map[string]linkOwner)}
	for _, m := range config.Modules {
		own := make(map[string]uint32)
		exported := make(map[string]uint32)
		placed := make(map[string]bool)
		owners := make(map[string]linkOwner)
		for _, f := range m.Functions {
			own[f.Name], exported[f.Name], placed[f.Name] = f.Addr, f.Addr, true
		}
		for _, symbol := range m.Data {
			own[symbol.Name], exported[symbol.Name] = symbol.Addr, symbol.Addr
			placed[symbol.Name] = m.contains(symbol.Addr, 1)
		}
		for _, symbol := range m.Imports {
			own[symbol.Name] = symbol.Addr
		}
		for _, symbol := range append(append([]symbolSpec(nil), m.Data...), m.Imports...) {
			if other, ok := config.byID[symbol.In]; ok && symbol.In != m.ID {
				if name, ok := other.symbolStartingAt(symbol.Addr, symbol.Name); ok {
					owners[symbol.Name] = linkOwner{other.ID, name}
				}
			}
		}
		resolver.own[m.ID], resolver.exported[m.ID] = own, exported
		resolver.placed[m.ID], resolver.owners[m.ID] = placed, owners
	}
	return resolver
}

// qualifiedName is the link's name for module's symbol in another module's
// link: a relocation that names a module (In) binds it, so the same name in
// two modules stays two symbols. Module ids hold no underscore, so the dot
// and the underscores spell it back (splitQualifiedName).
func qualifiedName(module, symbol string) string {
	return strings.ReplaceAll(module, "-", "_") + "." + symbol
}

func splitQualifiedName(name string) (string, string, bool) {
	module, symbol, ok := strings.Cut(name, ".")
	return strings.ReplaceAll(module, "_", "-"), symbol, ok
}

// owner names the module and symbol whose link places name for m when that
// is another module resident with m (projectConfig.resident): m's own row or
// import that names another module's object (In), a name m resolves from a
// linked module or MAIN that its link places (or names with In), or a
// qualified name. A fixed area (a row outside every image), and a name from a
// module never resident with m, have none: they stay at the address target/
// gives.
func (resolver symbolResolver) owner(m *moduleSpec, name string) (linkOwner, bool) {
	if module, symbol, ok := splitQualifiedName(name); ok {
		return linkOwner{module, symbol}, resolver.config.resident(m.ID, module)
	}
	if _, ok := resolver.own[m.ID][name]; ok {
		owner, ok := resolver.owners[m.ID][name]
		return owner, ok && resolver.config.resident(m.ID, owner.module)
	}
	for _, link := range append(append([]string(nil), m.Links...), mainModuleID) {
		if _, ok := resolver.exported[link][name]; !ok {
			continue
		}
		owner, ok := resolver.owners[link][name]
		if !ok {
			owner, ok = linkOwner{link, name}, resolver.placed[link][name]
		}
		return owner, ok && resolver.config.resident(m.ID, owner.module)
	}
	return linkOwner{}, false
}

func (resolver symbolResolver) resolve(m *moduleSpec, name string) (uint32, bool) {
	if module, symbol, ok := splitQualifiedName(name); ok {
		addr, ok := resolver.exported[module][symbol]
		return addr, ok
	}
	if addr, ok := resolver.own[m.ID][name]; ok {
		return addr, true
	}
	for _, link := range m.Links {
		if addr, ok := resolver.exported[link][name]; ok {
			return addr, true
		}
	}
	addr, ok := resolver.exported[mainModuleID][name]
	return addr, ok
}

// linkerBindings lists the PROVIDE bindings one function's link needs: every
// name its source mentions that resolves, compiler-emitted helpers the module
// itself defines, and the function's overrides.
func (resolver symbolResolver) linkerBindings(m *moduleSpec, f *functionSpec, used map[string]struct{}) map[string]uint32 {
	bindings := make(map[string]uint32)
	for name, addr := range resolver.own[m.ID] {
		if compilerEmitted(name) {
			bindings[name] = addr
		}
	}
	for name := range used {
		if addr, ok := resolver.resolve(m, name); ok {
			bindings[name] = addr
		}
	}
	for _, override := range m.Overrides {
		if override.Function == f.Name {
			bindings[override.Name] = override.Addr
		}
	}
	// The shared C definition is this entry's self symbol, not another entry.
	if f.SourceSymbol != "" {
		bindings[f.sourceSymbol()] = f.Addr
	}
	return bindings
}

// linkerScript places .text (and .rodata when declared) at the target
// addresses so the compiled sections are final bytes. With newRodata (a
// function an edition changes), a .rodata no range declares follows the
// .text, where the edition's flowing layout puts it.
func linkerScript(f *functionSpec, bindings map[string]uint32, newRodata bool) (string, error) {
	names := make([]string, 0, len(bindings))
	for name := range bindings {
		names = append(names, name)
	}
	sort.Strings(names)
	var script strings.Builder
	script.WriteString("OUTPUT_ARCH(mips)\n")
	for _, name := range names {
		// The mapped definition must come from the object, never PROVIDE.
		if f.SourceSymbol != "" && name == f.sourceSymbol() {
			continue
		}
		if name == "_gp" {
			// GPREL relocations consult this linker symbol implicitly; PROVIDE
			// can omit it because the object has no explicit _gp reference.
			fmt.Fprintf(&script, "_gp = 0x%08x;\n", bindings[name])
			continue
		}
		fmt.Fprintf(&script, "PROVIDE(%s = 0x%08x);\n", name, bindings[name])
	}
	script.WriteString("SECTIONS {\n")
	switch len(f.Rodata) {
	case 0:
	case 1:
		fmt.Fprintf(&script, "  .rodata 0x%x : SUBALIGN(4) { *(.rodata) *(.rdata) }\n", f.Rodata[0].Addr)
	default:
		return "", fmt.Errorf("%s declares more than one .rodata range", f.Name)
	}
	fmt.Fprintf(&script, "  .text 0x%x : SUBALIGN(4) { *(.text) }\n", f.Addr)
	if newRodata && len(f.Rodata) == 0 {
		script.WriteString("  .rodata : { *(.rodata) *(.rdata) }\n")
	}
	script.WriteString("  /DISCARD/ : { *(.reginfo) *(.pdr) *(.comment) *(.gnu.attributes) }\n}\n")
	if f.SourceSymbol != "" {
		fmt.Fprintf(&script, "ASSERT(%s == 0x%08x, \"source definition must begin at original entry\");\n", f.sourceSymbol(), f.Addr)
	}
	return script.String(), nil
}

// writeLinkerScript stores a function's script under buildRoot/<name>.
func writeLinkerScript(buildRoot, name, script string) (string, error) {
	dir := filepath.Join(buildRoot, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "generated.ld")
	if existing, err := os.ReadFile(path); err == nil && string(existing) == script {
		return path, nil
	}
	return path, os.WriteFile(path, []byte(script), 0o644)
}
