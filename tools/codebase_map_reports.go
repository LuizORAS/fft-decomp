// codebase_map_reports.go renders the QUIRKS.md section pages and the
// generated reports: the build-3 backlog and the stale-mention list.
package main

import (
	"fmt"
	"sort"
	"strings"
)

const (
	mapBacklogPage = "Build 3 backlog"
	mapStalePage   = "Stale mentions"
)

// quirkText renders a QUIRKS.md bullet with every name that has a page
// turned into a wikilink.
func (vault *mapVault) quirkText(text string) string {
	return mapBacktickPattern.ReplaceAllStringFunc(text, func(span string) string {
		inner := span[1 : len(span)-1]
		key, _, ok := vault.index.resolveMention(inner)
		if !ok || key == "" {
			return span
		}
		if _, has := vault.names[key]; !has {
			return span
		}
		return vault.link(key, inner)
	})
}

// quirkExcerpt is a bullet's first sentence as plain text, for overviews.
func quirkExcerpt(text string) string {
	plain := strings.TrimPrefix(strings.Join(strings.Fields(text), " "), "- ")
	plain = mapBacktickPattern.ReplaceAllString(plain, "$1")
	if end := strings.Index(plain, ". "); end >= 0 {
		plain = plain[:end+1]
	}
	if len(plain) > 180 {
		cut := strings.LastIndex(plain[:177], " ")
		if cut < 0 {
			cut = 177
		}
		plain = plain[:cut] + "..."
	}
	return plain
}

func (vault *mapVault) quirkLinks(quirk *mapQuirk) string {
	var links []string
	for _, key := range quirk.keys {
		if _, has := vault.names[key]; has {
			links = append(links, vault.link(key, strings.SplitN(key, ":", 2)[1]))
		}
	}
	return strings.Join(links, " · ")
}

func (vault *mapVault) quirkSectionLink(title string) string {
	return vault.link("quirks:"+title, title)
}

func (vault *mapVault) quirkSectionPage(section *mapQuirkSection) string {
	var b strings.Builder
	b.WriteString(frontmatter(
		mapProperty{"type", "quirks"},
		mapProperty{"entries", len(section.entries)},
		mapProperty{"switch", mapQuirkSwitches[section.title]},
	))
	fmt.Fprintf(&b, "# %s\n\n", section.title)
	b.WriteString("Generated from `QUIRKS.md`; edit that file, not this page.\n\n")
	if section.intro != "" {
		b.WriteString(section.intro + "\n\n")
	}
	if suggested := mapQuirkSwitches[section.title]; suggested != "" {
		fmt.Fprintf(&b, "Build 3 switch: **%s**. Part of the [[%s]].\n\n", suggested, mapBacklogPage)
	}
	for _, quirk := range section.entries {
		b.WriteString(vault.quirkText(quirk.text) + "\n")
	}
	return b.String()
}

// quirksFor lists the QUIRKS.md entries that name a page.
func (vault *mapVault) quirksFor(key string) []*mapQuirk {
	var quirks []*mapQuirk
	for _, section := range vault.index.quirks {
		for _, quirk := range section.entries {
			for _, named := range quirk.keys {
				if named == key {
					quirks = append(quirks, quirk)
					break
				}
			}
		}
	}
	return quirks
}

// quirksSection renders a page's QUIRKS.md entries in full.
func (vault *mapVault) quirksSection(quirks []*mapQuirk) string {
	if len(quirks) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n## Quirks (%d)\n", len(quirks))
	for _, quirk := range quirks {
		fmt.Fprintf(&b, "\n%s, `QUIRKS.md` line %d:\n\n%s\n", vault.quirkSectionLink(quirk.section), quirk.line, vault.quirkText(quirk.text))
	}
	return b.String()
}

func (vault *mapVault) debtWhere(comment mapComment) string {
	if comment.function != "" {
		return vault.functionLink(comment.function)
	}
	if _, isHeader := vault.index.headers[comment.path]; isHeader {
		return vault.headerLink(comment.path)
	}
	return "`" + comment.path + "`"
}

func (vault *mapVault) backlogPage() string {
	index := vault.index
	counts := map[string]int{}
	for _, debt := range index.debts {
		counts[debt.kind]++
	}
	fixed, fixedTotal := map[string]int{}, 0
	for _, section := range index.quirks {
		for _, quirk := range section.entries {
			if len(quirk.fixed) > 0 {
				fixed[section.title]++
				fixedTotal++
			}
		}
	}
	var b strings.Builder
	b.WriteString(frontmatter(
		mapProperty{"type", "report"},
		mapProperty{"type_debt_comments", counts["type"]},
		mapProperty{"port_debt_comments", counts["port"]},
		mapProperty{"fixed_quirks", fixedTotal},
	))
	fmt.Fprintf(&b, "# %s\n\n", mapBacklogPage)
	b.WriteString("Everything the byte-exact build must keep as it is and a later build can change, ")
	b.WriteString("collected from `QUIRKS.md` and from the \"Type debt\" / \"Port debt\" comments in the sources. ")
	b.WriteString("Nothing here is kept by hand: fix the entry or the comment, then run `make map`. ")
	b.WriteString("An entry an edition already changes says the option it is fixed under (its \"Fixed under\" marker).\n\n")
	b.WriteString("| Source | Entries | Fixed | Build 3 switch |\n|---|---|---|---|\n")
	for _, section := range index.quirks {
		b.WriteString(tableRow(vault.quirkSectionLink(section.title), fmt.Sprint(len(section.entries)), fmt.Sprint(fixed[section.title]), mapQuirkSwitches[section.title]))
	}
	b.WriteString(tableRow("\"Type debt\" comments", fmt.Sprint(counts["type"]), "", "NON_MATCHING (type debt)"))
	b.WriteString(tableRow("\"Port debt\" comments", fmt.Sprint(counts["port"]), "", "PORT"))
	for _, section := range index.quirks {
		fmt.Fprintf(&b, "\n## %s (%d)\n\n", section.title, len(section.entries))
		for _, quirk := range section.entries {
			fmt.Fprintf(&b, "- Line %d: %s", quirk.line, quirkExcerpt(quirk.text))
			if len(quirk.fixed) > 0 {
				fmt.Fprintf(&b, " **Fixed under %s.**", strings.Join(quirk.fixed, ", "))
			}
			if links := vault.quirkLinks(quirk); links != "" {
				fmt.Fprintf(&b, " → %s", links)
			}
			b.WriteString("\n")
		}
	}
	b.WriteString("\n## Debt comments\n\n")
	if len(index.debts) == 0 {
		b.WriteString("None.\n")
		return b.String()
	}
	debts := append([]mapDebt(nil), index.debts...)
	sort.SliceStable(debts, func(i, j int) bool {
		if debts[i].kind != debts[j].kind {
			return debts[i].kind > debts[j].kind
		}
		if debts[i].comment.path != debts[j].comment.path {
			return debts[i].comment.path < debts[j].comment.path
		}
		return debts[i].comment.line < debts[j].comment.line
	})
	b.WriteString("| Kind | Where | Line | Comment | Named in QUIRKS.md |\n|---|---|---|---|---|\n")
	for _, debt := range debts {
		named := "no"
		if index.quirksCovers(debt.comment) {
			named = "yes"
		}
		b.WriteString(tableRow(debt.kind, vault.debtWhere(debt.comment), fmt.Sprint(debt.comment.line), quirkExcerpt(debt.comment.text), named))
	}
	return b.String()
}

func (vault *mapVault) stalePage() string {
	stale := append([]mapStale(nil), vault.index.stale...)
	sort.SliceStable(stale, func(i, j int) bool {
		if stale[i].path != stale[j].path {
			return stale[i].path < stale[j].path
		}
		if stale[i].line != stale[j].line {
			return stale[i].line < stale[j].line
		}
		return stale[i].name < stale[j].name
	})
	var b strings.Builder
	b.WriteString(frontmatter(mapProperty{"type", "report"}, mapProperty{"stale", len(stale)}))
	fmt.Fprintf(&b, "# %s\n\n", mapStalePage)
	b.WriteString("Names that `QUIRKS.md` or a comment gives but that the code no longer spells, usually left behind by a rename ")
	b.WriteString("(`make symbols` does not edit comments or `QUIRKS.md`), and comments that cite `QUIRKS.md` about something it does not mention.\n\n")
	if len(stale) == 0 {
		b.WriteString("None.\n")
		return b.String()
	}
	b.WriteString("| File | Line | Name | Problem |\n|---|---|---|---|\n")
	for _, entry := range stale {
		where := "`" + entry.path + "`"
		if name, ok := vault.index.functionBySource[entry.path]; ok {
			where = vault.functionLink(name)
		} else if _, ok := vault.names["doc:"+entry.path]; ok {
			where = vault.link("doc:"+entry.path, entry.path)
		}
		b.WriteString(tableRow(where, fmt.Sprint(entry.line), "`"+entry.name+"`", entry.why))
	}
	return b.String()
}
