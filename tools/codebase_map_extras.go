// codebase_map_extras.go writes the by-products of `make map` that are not
// vault pages: PCSX-Redux symbol maps in build/symbols/, and the frozen table
// that translates the names of the decompilation this project started from
// into ours by address (a vault page and its committed copy).
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
)

const (
	mapUpstreamPage   = "Upstream renames"
	mapUpstreamTable  = "docs/upstream-renames.tsv" // the frozen names, with ours kept current
	mapUpstreamHeader = "module\taddress\tupstream\tours"
)

// mapSymbolScenes are the module sets resident together; overlays share
// addresses, so each PCSX-Redux map covers one scene.
var mapSymbolScenes = []struct {
	file    string
	modules []string
}{
	{"fft_battle.map", []string{"main", "battle"}},
	{"fft_world.map", []string{"main", "wldcore", "world"}},
	{"fft_opening.map", []string{"main", "opening"}},
}

// writeSymbolMaps writes "address name" maps for PCSX-Redux. At an address
// with several names the function, then the first data or import row, wins.
func writeSymbolMaps(root string, config *projectConfig) (int, error) {
	out := filepath.Join(root, "build", "symbols")
	if err := os.MkdirAll(out, 0o755); err != nil {
		return 0, err
	}
	for _, scene := range mapSymbolScenes {
		names := map[uint32]string{}
		for _, id := range scene.modules {
			m, ok := config.module(id)
			if !ok {
				continue
			}
			add := func(addr uint32, name string) {
				if _, seen := names[addr]; !seen && addr >= 0x80000000 {
					names[addr] = name
				}
			}
			for _, f := range m.Functions {
				add(f.Addr, f.Name)
			}
			for _, data := range m.Data {
				add(data.Addr, data.Name)
			}
			for _, imported := range m.Imports {
				add(imported.Addr, imported.Name)
			}
		}
		addresses := make([]uint32, 0, len(names))
		for addr := range names {
			addresses = append(addresses, addr)
		}
		sort.Slice(addresses, func(i, j int) bool { return addresses[i] < addresses[j] })
		var b strings.Builder
		for _, addr := range addresses {
			fmt.Fprintf(&b, "%08x %s\n", addr, names[addr])
		}
		if err := atomicWrite(filepath.Join(out, scene.file), []byte(b.String())); err != nil {
			return 0, err
		}
	}
	return len(mapSymbolScenes), nil
}

type mapRename struct {
	module   string
	addr     uint32
	upstream string
	ours     string
}

// upstreamRenames reads the rename table (mapUpstreamTable): each name the
// decompilation this project started from (adamrt/fft_decomp) gave a function
// or data row at its last merge, f59f939d, where ours differs. Those names are
// frozen. Each row takes our current name at its module and address (the one
// it held while the address still carries it); a row drops out when its
// address carries no name of ours, or the upstream name again. ok is false
// when the checkout has no table.
func upstreamRenames(root string, ours *projectConfig) (renames []mapRename, ok bool, err error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(mapUpstreamTable)))
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, true, err
	}
	names := map[string]map[uint32][]string{}
	for _, m := range ours.Modules {
		byAddr := map[uint32][]string{}
		for _, f := range m.Functions {
			byAddr[f.Addr] = append(byAddr[f.Addr], f.Name)
		}
		for _, data := range m.Data {
			byAddr[data.Addr] = append(byAddr[data.Addr], data.Name)
		}
		names[m.ID] = byAddr
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if lines[0] != mapUpstreamHeader {
		return nil, true, fmt.Errorf("%s does not start with its header %q", mapUpstreamTable, mapUpstreamHeader)
	}
	for index, line := range lines[1:] {
		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			return nil, true, fmt.Errorf("%s:%d: %d fields, not 4", mapUpstreamTable, index+2, len(fields))
		}
		addr, parseErr := strconv.ParseUint(strings.TrimPrefix(fields[1], "0x"), 16, 32)
		if parseErr != nil {
			return nil, true, fmt.Errorf("%s:%d: address %q: %v", mapUpstreamTable, index+2, fields[1], parseErr)
		}
		candidates := names[fields[0]][uint32(addr)]
		if len(candidates) == 0 || slices.Contains(candidates, fields[2]) {
			continue
		}
		current := candidates[0]
		if slices.Contains(candidates, fields[3]) {
			current = fields[3]
		}
		renames = append(renames, mapRename{module: fields[0], addr: uint32(addr), upstream: fields[2], ours: current})
	}
	sort.SliceStable(renames, func(i, j int) bool {
		if renames[i].module != renames[j].module {
			return renames[i].module < renames[j].module
		}
		if renames[i].addr != renames[j].addr {
			return renames[i].addr < renames[j].addr
		}
		return renames[i].upstream < renames[j].upstream
	})
	return renames, true, nil
}

// writeUpstreamRenames writes the rename table with our current names, a TSV
// that GitHub shows as a table, and reports whether its content changed.
func writeUpstreamRenames(root string, renames []mapRename) (bool, error) {
	var b strings.Builder
	b.WriteString(mapUpstreamHeader + "\n")
	for _, rename := range renames {
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\n", rename.module, hex32(rename.addr), rename.upstream, rename.ours)
	}
	path := filepath.Join(root, filepath.FromSlash(mapUpstreamTable))
	if current, err := os.ReadFile(path); err == nil && string(current) == b.String() {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, atomicWrite(path, []byte(b.String()))
}

func (vault *mapVault) upstreamPage(renames []mapRename, available bool, readErr error) string {
	var b strings.Builder
	b.WriteString(frontmatter(mapProperty{"type", "report"}, mapProperty{"renames", len(renames)}))
	fmt.Fprintf(&b, "# %s\n\n", mapUpstreamPage)
	b.WriteString("Function and data names that the decompilation this project started from ([adamrt/fft_decomp](https://github.com/adamrt/fft_decomp)) gave at its last merge, `f59f939d`, and that differ from ours at the same module and address: the table to translate one of its commits into this tree. ")
	fmt.Fprintf(&b, "`%s` holds those names, frozen; `make map` writes our current name at each address into it.\n\n", mapUpstreamTable)
	switch {
	case readErr != nil:
		fmt.Fprintf(&b, "The table could not be read: %v\n", readErr)
		return b.String()
	case !available:
		fmt.Fprintf(&b, "This checkout has no `%s`.\n", mapUpstreamTable)
		return b.String()
	case len(renames) == 0:
		b.WriteString("No names differ.\n")
		return b.String()
	}
	fmt.Fprintf(&b, "%d names differ.\n\n| Module | Address | Upstream | Ours |\n|---|---|---|---|\n", len(renames))
	for _, rename := range renames {
		ours := "`" + rename.ours + "`"
		if _, isFunction := vault.index.functions[rename.ours]; isFunction {
			ours = vault.functionLink(rename.ours)
		} else if _, isGlobal := vault.index.globals[rename.ours]; isGlobal {
			ours = vault.globalLink(rename.ours)
		}
		b.WriteString(tableRow(rename.module, "`"+hex32(rename.addr)+"`", "`"+rename.upstream+"`", ours))
	}
	return b.String()
}
