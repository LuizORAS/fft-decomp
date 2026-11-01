# Hand-written documentation

The pages the code cannot generate: what each game mechanic does and where it
lives, and guides for common changes. With [`CODEBASE.md`](../CODEBASE.md)
they are the hand-written half of the codebase map. `make map` generates the
other half (a page for every function, type, global, header, code subsystem
and module) into `build/map/`, an Obsidian vault.

| Path | Contents |
|---|---|
| `mechanics/` | One page per game mechanic (movement, formulas, AI...) with the `scope` of code it covers |
| `guides/` | Walkthroughs of a common change ("where to change X") |
| `templates/` | The skeleton of each kind of page |
| `upstream-renames.tsv` | Each name the decompilation this project started from ([adamrt/fft_decomp](https://github.com/adamrt/fft_decomp), at its last merge, `f59f939d`) gave where ours differs, by module and address. Those names are frozen; `make map` keeps the last column on our current names |

Pages link to generated pages with wikilinks such as `[[name]]`. A generated
page is named after its function, type or global, so on GitHub a link still
reads as the exact name to search for.

## What `make map` does with these pages

It copies `docs/` and `CODEBASE.md` into the vault at the same paths. For each
page under `mechanics/` it reads two frontmatter properties:

- `tier`: the priority tier. 1 is the battle rules and the engine core, 2
  units, items and jobs, 3 menus, the world map and the rest of the game, 4
  effects and libraries.
- `scope`: name prefixes (`battle_move` matches `battle_move` and every
  `battle_move_*` function), exact function names written with `()`
  (`main()` matches `main` only) or source folders ending in `/`
  (`src/lib/psyq/` matches every function whose file is under it). Every matched
  function gets the `mechanic` and `tier` properties, and the vault gains a
  `<page> scope` page that lists them. A function that several entries match
  belongs to an exact entry, or else to the longest prefix; a folder takes
  only what no name entry matches.

The vault's Metrics page counts mechanic pages, guides and the functions in
a scope. Its Stale mentions page lists scope entries that match no function,
code spans that name a path, project symbol or type the code no longer has,
and wikilinks to pages that do not exist.

## Editing

Edit the pages here, in the repository. Editing the vault's copy also works:
the next `make map` notices the change (it keeps each copy's hash in
`build/map/.docs.sha256`) and stops, and `make map ARGS=--pull-docs` copies
the edit back here before regenerating. A new page created under the vault's
`docs/` folder is pulled the same way; pages created anywhere else in the
vault are lost on the next run. When the repository's page changed too, the
run stops and the two versions are merged by hand.

On Windows, Obsidian cannot open a vault inside WSL (it cannot watch that
folder), so export one to a Windows drive with
`make map MAP_EXPORT=/mnt/d/fft-map` and open that folder instead. Every run
with the same `MAP_EXPORT` updates it, and its `docs/` pages have the same
protection.

To start a page in Obsidian, use the core Templates plugin with
`docs/templates` as its folder (a new vault is set up that way). Elsewhere,
copy the template file.
