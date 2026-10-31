# AGENTS.md

A byte-exact matching decompilation of the USA PlayStation release of *Final
Fantasy Tactics* (`SCUS-94221`). Every game function is C that compiles to the
original bytes, except the routines the original programmers wrote in
assembly, which are `.s` files that assemble to them. The work now is making
that C better (names, types, structure) without changing a single byte.

## Linked libraries

Linked Psy-Q, runtime/startup and Suzuki library reconstructions are normal
repository source. Keep public SDK entry-point names, use `include/psx/`, and
put one reconstructed function per file under `src/lib/psyq/<library>/`.
Register it in the original module's `target/` configuration with an explicit
`source:` path. A routine or global with no established SDK name is
`psyq_<lib>_…` or `g_psyq_<lib>_…`. LIBGS (WORLD) is `src/lib/psyq/libgs/` and
LIBPRESS (OPENING) `src/lib/psyq/libpress/`. The Suzuki sound driver, its four
heap routines included, is `src/lib/suzuki/` under `suzuki_` names, declared in
`include/psx/suzuki.h`.

Library C has the same byte-exact requirements as game C. Every completed
function must match all original bytes; unmatched drafts stay in ignored
`build/`. Generate hashes only from original bytes through `make checksums`.
Never weaken checks, accept mismatches, or change a hash to bless edited code.
Use `make library-diff ARGS="--name=NAME --source=build/.../NAME.c
--addr=0xADDRESS --size=BYTES"` for scratch candidates before integration; it
uses the normal historical compiler pipeline without modifying target metadata.

Library headers and sources must never include `fft/` game headers. Move
shared library declarations to the library that logically owns them; game
headers may consume those declarations. Startup declares the program's `main`
as an external runtime contract. Keep ordinary declarations and driver
internals together; isolate or localize architectural register views so an
ordinary include cannot reserve CPU registers in unrelated callers.

Keep useful behavior and matching constraints in source comments and
`QUIRKS.md`, not research diaries or duplicate inventories. Retain a register
pin or empty assembly barrier only when it implements a real architectural ABI
or its removal changes the original bytes. Re-measure speculative workarounds
and remove those that prove unnecessary. Document uncertainty honestly.

These files implement the original PS1 behavior. BIOS firmware remains
external; linked BIOS veneers and hardware helpers are reconstructed here.
Game media, SDK archives/objects, extracted binaries and copied table dumps
remain excluded. Never modify `meta/`.

## Target

- Input: `scus-94221.bin` at the repo root (raw Mode 2/2352, 541,315,152
  bytes, SHA-256 `29e63bb6d2e7b842fe87f7c0ee03b7076681c34bcc47095cca27ec53236bf4f9`).
- Output: `build/disc/output-scus-94221.bin` + `.cue`, byte-identical to the
  input. These constants live in `tools/target.go`.
- Never add, stage or embed game media, extracted files, build output,
  proprietary SDK material or text copied from outside references (wikis,
  guides). `meta/` is the user's private reference folder: never modify it.

## Layout

| Path | Contents |
|---|---|
| `src/<module>/` | One C file per function, named after it (a `.s` file for an original hand-written routine) |
| `src/shared/<subsystem>/` | Proven shared implementations: one source compiled at every original entry that runs it, bound per module with `source:` and `source_symbol` in `target/` |
| `src/lib/psyq/<library>/`, `src/lib/suzuki/` | Byte-exact library reconstructions, registered with explicit `source:` paths in their original module |
| `include/fft/` | Module headers (`main.h`, `world.h`, `wldcore.h`, `open.h`, `effect.h`, `event_<overlay>.h`), BATTLE's subsystem headers (`battle_<subsystem>.h`, all included by `battle.h`) and shared type headers (`data.h`, `gfx.h`, `map.h`, `memory.h`, `menu.h`, `menu_script.h`, `script.h`, `sound.h`, `thread.h`, `unit.h`), plus `disc.h` and `ram.h`, generated from `target/disc.yaml` and `target/ram.yaml` |
| `include/psx/` | Psy-Q SDK, library and PS1 hardware declarations (keep SDK names and signatures) |
| `target/*.yaml` | Every disc module: functions with hashes, data names, libraries, regions |
| `target/disc.yaml` | The disc's file system: every directory, file and dummy at its sector, with sizes and dates, never bytes |
| `target/ram.yaml` | The RAM the modules load into: each window with its modules and what bounds it, and the windows resident together |
| `tools/` | Go tooling (its own module), rebuilt from the working tree on every `make` call, and `make smoke`'s emulator script and runner (`smoke.lua`, `smoke.sh`) |
| `QUIRKS.md` | Index of retail bugs and code that looks wrong on purpose |
| `CODEBASE.md` | Architecture guide: disc modules, boot and game loop, engine services, how to find things |
| `docs/` | Hand-written mechanic pages and guides; `make map` copies them into the map vault |
| `build/` | Ignored: extracted disc, caches, outputs, scratch work |

| Module | File | Load | Role |
|---|---|---|---|
| main | `SCUS_942.21` | `0x80010000` | Resident executable (`load: 0x8000f800` covers the 0x800-byte header) |
| battle | `BATTLE.BIN` | `0x80067000` | Battle engine |
| wldcore | `WORLD/WLDCORE.BIN` | `0x80067000` | World core, runs with WORLD |
| world | `WORLD/WORLD.BIN` | `0x800e0000` | World map, menus, event scripts |
| opening | `OPEN/OPEN.BIN` | `0x80067000` | Opening and title |
| event-* | `EVENT/*.OUT` | `0x801bf000`… | Menu screens run with BATTLE (`target/event.yaml`) |
| effect-* | `EFFECT/E*.BIN` | `0x801c2500` | 110 ability effects run with BATTLE (`target/effect.yaml`) |

File offset = address − `load`. Linked library ranges are declared under
`libraries:`. The linked Psy-Q and startup functions, LIBGS (WORLD), LIBPRESS
(OPENING), and Suzuki sound driver are reconstructed C. This includes the four
Suzuki heap routines at `0x8001423c`–`0x8001442c`, which use the original
`-G8` profile and `$gp` binding. Original padding, tables and executable
patch templates remain data in their original modules, except a template
that holds an address split over two instructions, which is a `.s` so the
link moves the address (LIBCARD's two). `target/*.yaml` is
authoritative for each function's source, address, size, profile and hash.

## Commands

```sh
make bootstrap             # build the Docker image (extracts the BIN if present)
make check [MODULE=x]      # compile and compare with target/ hashes; no BIN needed
make build [MODULE=x]      # link each module whole from its objects and the bytes between them, taking each name another module places from that module's link (a second pass); compare with the original file (function hashes without the BIN, the data bytes then zeros); fail on an image past its RAM window and print each window's room; every module with the BIN also builds the disc in build/disc and checks its sha256
make build SHIFT=16        # a test disc in build/shift: every module's image opens 16 bytes (break instructions) at its start, MAIN pads to whole sectors and a file that outgrows its sectors moves; fail on a word that changed unexplained (needs the BIN)
make patch-check [MODULE=x] # cross-check make build the way the disc was built before the link: each function compiled alone and written over a copy of its original module image, every copy identical, and every module's copy makes the disc in build/patch/disc, sha256 checked (needs the BIN)
make extract               # extract the BIN (build/checksums/diff do it on demand)
make checksums             # refresh hashes from the ORIGINAL bytes only
make check-config          # validate target/, sources and declarations
make config-fmt            # rewrite target/*.yaml in canonical form
make decl-check            # fail on calls to undeclared functions (the build's -w hides them)
make declarations          # report conflicting declarations in detail
make symbols ACTION=rename-function|rename-global ARGS="--old A --new B"
make share-audit [MODULE=x] # report shared C implementation candidates as JSON
make twins [MODULE=x]      # group functions identical under renames across modules (needs the BIN)
make trial TWINS=t0001     # compile each twin's source at the others' entries (or MODULE ENTRY SOURCE)
make unit-check [MODULE=x] # compile each original translation unit whole from its members' sources
make smoke                 # boot the original and the shifted test disc in PCSX-Redux (PCSX_REDUX=path) from tools/smoke.lua: a new game through Orbonne (ATTACK.OUT, an effect file's code) to the save screen, then Continue through Gariland to the world map's formation screen, each checkpoint checking the code in RAM; logs and screenshots in build/smoke, about 50 minutes (after make build and make build SHIFT=16)
make reloc-check [MODULE=x] # fail on a data word that looks like a RAM address or a disc sector and is neither relocation, disc or ram word nor value, and on a RAM address split over two instructions outside code; link each module twice shifted to check every changed word, and list the findings in build/reloc-check (needs the BIN)
make diff FUNC=f           # compare one function with the original bytes (needs the BIN)
make library-diff ARGS="..." # compare a scratch library candidate without registering it
make permute FUNC=f        # run decomp-permuter on it [DURATION=300 JOBS=4]
make test                  # vet and test the Go tooling
make map                   # write the codebase map (Obsidian vault) to build/map and the upstream rename table to docs/upstream-renames.tsv; ARGS=--pull-docs copies docs edited in the vault back
make warnings              # list cc1 -Wall warnings per function in build/warnings [MODULE=x]
make verify                # fmt-check, check-config, decl-check and check
make fmt                   # clang-format src/ and include/ in place
make fmt-check             # fail on any formatting drift
make shell                 # interactive shell in the image
make run                   # build and launch PCSX-Redux (PCSX_REDUX=/path on Linux)
make compile-commands      # editor compile_commands.json (host clang)
```

`MODULE` takes an id (`battle`, `event-attack`) or an alias (`attack`,
`bunit`, `equip`, `jobstts`); for `check`, `build` and `patch-check` it may
be a list, and `event` or `effect` selects every such module. `diff` and
`permute` need it only when several modules define `FUNC`. Both compile
exactly as `check` does. Every change ends with `make check` passing. Before
handing off, also run `make build` and `make patch-check` (disc exact both
ways), `make build SHIFT=16` (nothing unexplained), `make reloc-check`,
`make check-config`, `make decl-check`, `make fmt-check` and `make test`.

Every target runs in a fresh `docker run --rm` container of the pinned
`Dockerfile` image as the host user, with the checkout mounted at `/work`;
the image is tagged by a checksum of its inputs, built on first use (its last
layer proves every tool runs) and rebuilt by `make bootstrap`. It holds the
old GCC `cc1` binaries (native on x86-64, through `qemu-i386` elsewhere), maspsx, mipsel binutils and
`cpp`, mkpsxiso, asm-differ (`make diff` listings), decomp-permuter, Go and
clang-format 22.1.8. CI (`.github/workflows/check.yml`) runs `fmt-check`,
`check-config`, `test`, `decl-check` and `map` in one job and `check` and
`build` in four module shards, without the BIN; `check-config` fails unless
the shards' module lists cover every module once. Optional overrides: `JOBS`,
`CACHE=0`, `WORK_DIR`, `REPORT_ALL=1`, `DURATION` (see the Makefile).

## target/*.yaml

One YAML document per module, plus `disc.yaml` and `ram.yaml` (below); `event.yaml` and
`effect.yaml` hold many, separated by `---`. `make config-fmt` defines the canonical form (unquoted hex
addresses, decimal sizes, one flow mapping per line, sorted rows) and
`make check-config` rejects anything else.

- Header: `module`, `file` (its sector is in `disc.yaml`), `size`, `sha256`,
  `load`, `source_dir`, `profile` (module default), `links` (modules
  resident alongside).
- `functions`: `{addr, size, name, hash}`, plus `profile`, `rodata`, `asm`
  (why the routine is assembly; its source is then `<name>.s` in
  `src/<module>/`, or in its library's directory with `source`), `source` (a
  path other than the default) or
  `source_symbol` (the neutral symbol a `src/shared/` source defines) only
  when needed. `hash` is the first 16 hex digits of the SHA-256 of the
  original bytes; only `make checksums` writes it.
- `data`: named addresses with no reconstruction (globals, library entry
  points). A data row inside a function names that place in the code and
  moves with it; one named with another module's prefix is that module's
  object at a fixed address. A row or import outside the image that names
  another module's object gives that module in `in`; `make build` takes its
  address from that module's link while the two are in RAM together
  (`ram.yaml` states). A row outside every image with no `in` is a fixed area
  and keeps its address. A row's `page` is the high half of its address
  that code loads as a number where no symbol spelling matches the bytes;
  `make build` asserts the row stays in that page. `imports`: names from
  overlays that linking cannot otherwise reach. `overrides`: per-function
  bindings. `regions`: bytes deliberately left original, with the reason.
- `relocations`: data words that hold an address, `{addr, to}` plus
  `offset` when the word points past the start of `to` (any name the module
  resolves, or with `in` a function or data row that module places);
  `make build` writes each word from its symbol, so it follows the
  symbol. `values`: words that look like RAM addresses or disc sectors but
  are not, `{addr, why}` for one word or `{addr, end, why}` for a range.
  Each word lies outside code and is classified once; `make reloc-check`
  fails on a word in the game's RAM that is neither, nor a `ram` word, and on
  a RAM address split over a `lui` and a use of its register outside code,
  which no data word can relocate.
- `disc`: data words that hold a file's place on the disc, `{addr, sector:
  FILE}` for its first sector (plus `offset` in sectors past it),
  `{addr, size: FILE}` for its size rounded up to whole sectors (0 when
  empty) and `{addr, extent: FILE}` for the two words in a row. `make build`
  writes each from `disc.yaml`, so it follows the file. `make reloc-check`
  checks each against the original and fails on a word that reads as a
  file's first sector followed by its size, or as one of three or more first
  sectors in a row, unless it is a `disc` word or a value.
- `ram`: data words that hold a RAM window's size, `{addr, size: WINDOW}`,
  its start, `{addr, start: WINDOW}` (a list of the windows that start there
  when several do), or its start past another window's,
  `{addr, start: WINDOW, from: OTHER}`; `make build` writes each from
  `ram.yaml`, and `make reloc-check` checks each against the original.
- `units`: the original translation units, `{addr, end, name}` plus `profile`
  when the file was not built with the module default, matched by address
  from upstream's unit tables and named with its labels. The functions in
  `[addr, end)` were one source file; each keeps its own source, and
  `make unit-check` rebuilds the unit from them.
- Linking one function binds each name it uses from its own module, then
  each `links` module in order, then main. `make build` then binds each name
  another module places at the address that module's link gave it, when the
  two are in RAM together; a name from a module never resident with it
  (WORLD's copies of BATTLE and EVENT code) keeps its target address.

`disc.yaml` is the disc: the volume, then every directory record, file and
dummy (sectors the original mastering left empty) at its first sector
(`lba`), in sector order. A file row gives its byte `size` (a `mixed` file,
an STR movie or XA audio, has 2336-byte sectors), except a module's file,
which takes its size from its module; each file or dummy ends where the next
row starts. `make build` and `make patch-check` write the mkpsxiso project from it.
A `fixed` directory holds files whose sectors other disc data stores (MAP:
the GNS rows), so they stay where they are. Every dummy is the gap between
two rows, and `make config-fmt` writes them: a file that outgrows the
sectors up to the next directory or file moves to the start of the `free`
dummy (outside a `fixed` directory), and its sectors and sizes in code and
data follow it. A file's sector or length in
code is a name `make config-fmt` writes to `include/fft/disc.h` for each one
the code uses: `DISC_SECTOR_<FILE>` is its first sector and
`DISC_SECTOR_COUNT_<FILE>` its length in sectors, `<FILE>` being its path
with `/` and `.` as `_`; `make check-config` fails when the header differs.

`ram.yaml` is the RAM the modules load into. A window runs from its modules'
load address (`addr`) to where the next thing resident with them starts
(`end`, the reason in `why`); each module, listed by id or by family
(`effect`), is in one window, starts at its start and ends by its end. A
state lists the windows resident at one time, which never overlap; two
modules are in RAM together when one state lists both their windows (two
modules of one window never are). `make build` fails on an image that passes its window's end and prints the
room each window has left. A window's size in code is
`RAM_WINDOW_BYTES_<WINDOW>` and its start past another's
`RAM_WINDOW_START_<WINDOW>_FROM_<OTHER>`, which `make config-fmt` writes to
`include/fft/ram.h` for each one the code uses.

To add a function: write `src/<module>/<name>.c`, add its `functions` row and
any new `data` rows, then `make config-fmt`, `make checksums`,
`make check MODULE=…`, `make build`. Rename with `make symbols` so sources,
headers, comments, `target/`, the file name and the Markdown documents change
together.

## Compiler profiles

Default: GCC 2.6.3 `-O2 -G0`, soft-float R3000, ASPSX 2.34 via maspsx. A
module's `profile` is the toolchain its translation units were built with; a
function carries its own `profile` only where it differs:

- WLDCORE and OPENING are GCC 2.7.2 with ASPSX 2.21 and checked division
  (`gcc-2.7.2_O2_aspsx-2.21_divcheck`): a separate `mfhi` in constant
  divisions, no delay-slot fill from past an if-body join, `lui/addiu/addu`
  for an indexed global, and the zero/overflow `break` checks around each
  division. No function there needs another profile. A load through a
  `const` pointee is never reloaded after a store under this compiler, so a
  parameter or table retail reloads is not `const`.
- `-O1` and `-O0`: parts of WORLD and the CARD screen.
- ASPSX 2.21 and `_divcheck`: a few functions in BATTLE, WORLD and MAIN.
- `gcc-2.6.3_O2_G8_aspsx-2.34`: the four Suzuki CPU heap routines require
  `$gp`-relative small data with `$gp = 0x800329bc`. The measured assembler
  load-delay correction applies only to this profile; the default remains G0.

Translation units were contiguous, so profiles come in address runs. Move a
function to another profile only when it then matches with fewer levers (or
none), and prefer the profile of its neighbors.

## Code rules

- Byte-exact always. Never weaken a check or edit a hash to accept a mismatch.
- Represent only what the original game did: no abstraction, helper layer or
  scaffolding the game did not have. A macro, header or helper exists only
  when it expresses something the original contained.
- C, except the routines the original programmers wrote in assembly. Each is
  one `.s` file per routine and module, named after it. Its opening comment
  says which original range it reproduces and why it cannot be C (it reads or
  replaces `$sp`, saves `k0`/`k1`/`$gp` in a context switch, keeps trapping
  `addi`, runs code after its own `jr ra`, or another measured reason), and its
  `functions` row carries that reason in `asm`. Only a range no C spelling
  under the pinned compilers reproduces becomes `.s`: a `handwritten` region,
  or a `blocked` one, whose comment then says it was probably C. A game routine
  that reads or replaces `$sp` is `.s` as well, since C reaches `$sp` only
  through a register pin. Addresses in a `.s` are symbols, never numbers. Each
  routine has a Port debt entry under "Hand-written assembly" in `QUIRKS.md`,
  because a native build needs a portable C twin. `make check-config` checks
  each of these and fails on a region left over a routine that has its source
  and on a handwritten or blocked range no function covers.
  Instruction-emitting inline asm remains in the hand-written thread/copy
  helpers and the PS1 hardware/ABI helpers in `include/psx/`.
  Library algorithms stay C; each hardware instruction or register constraint
  must have a measured PS1 dependency or byte-exact ABI reason.
- Typed access: structs and named fields, never raw offsets such as
  `*(s16*)((u8*)x + 0x218)`. Add a missing field to its shared header with a
  provisional name. A raw offset, cast, register pin or empty asm barrier stays
  only when the clean spelling provably changes the bytes, with a one-line
  comment saying so. A RAM address is never a number in code: code names its
  symbol, and a fixed area outside the module images is a `#define` in its
  module header's memory group; `make check-config` fails on any other.
- Ordinary C: named fields, direct assignments, conventional loops. Remove a
  redundant local unless a named temporary reads better (a long call or index
  expression, a value used more than once, a step that deserves a name).
- Names carry the module prefix (`battle_`, `world_`, `wldcore_`, `open_`,
  `main_`, `<overlay>_`; globals `g_battle_`, …). Unknowns stay
  `func_ADDRESS` or `D_ADDRESS` until the code supports a name, and a name
  that rests on thin evidence says Provisional in its comment. Unnamed
  struct members are `_unknown_XX` (code accesses it, meaning unknown),
  `_unused_XX` (no source accesses it by name) or `_padding_XX` (alignment
  or size filler, with an inline comment saying which), where XX is the
  hex offset in the member's own struct, zero-padded to the same width
  throughout the struct. `make check-config` fails on any access to an
  `_unused_` or `_padding_` member. snake_case; `_t` struct and `_e` enum
  typedefs whose tag repeats the name without the suffix; a size assert of a
  project type is named after it (`battle_stats_size_must_be_448`); uppercase
  enum values; `_init_`, not `_initialize_`. Decimal for counts and ordinary enum values,
  hex for flags and encoded values. A `src/shared/` source defines the neutral
  name its rows' `source_symbol` gives; each module's row keeps its module name
  where the module names that entry. `include/psx/` keeps SDK names. Header guards
  use descriptive header/path names without an `FFT_` prefix. A library
  header groups its SDK interface and reconstructed driver internals together;
  register-reserving ABI helpers stay in focused separate headers.
  In partial SDK/BIOS views, a field absent from this game's accesses remains
  `_unknown_XX`; use `_unused_XX` only when broader library/BIOS evidence proves
  it unused. Proven register spacing and alignment remain `_padding_XX`.
- Fixed point is 1.3.12 / 20.12 with 1.0 = `ONE` (4096, `psx/libgte.h`), also
  one full rsin/rcos turn. Write `ONE` for 1.0; `>> 12` rescales after a
  multiply. A name that states a fixed-point format says `qN` for N fraction
  bits (`battle_effect_lerp_q8`, `menu_lerp_q12`).
- One declaration per symbol, in its header. `make check-config` fails on
  conflicting declarations. Globals and prototypes go in the header of their
  name's module prefix. A type goes in the most specific module header that
  every user can see: BATTLE's subsystem header (`battle_<subsystem>.h`) for
  BATTLE and its overlays (whose headers include `battle.h`), `world.h` for
  WORLD, WLDCORE and OPENING, and a shared type header only when both families
  use it. A subsystem header includes its own dependencies, not the module
  umbrella. Two exceptions: `main_` types stay
  in `main.h`, and MAIN-only table or unit records (item, ability, job,
  party, ENTD) sit with their siblings in `data.h` and `unit.h`. A `volatile`
  or otherwise one-function view of a global stays local to that source
  file, with its reason. A source file includes its own
  module header, plus another module's header when it calls into that module
  (MAIN and WORLD code calling BATTLE functions, for example). `main.h`
  includes the shared type headers. A header starts with its guard, then a
  short comment saying what it declares, then its includes. Each header groups its declarations under
  short lowercase subsystem labels (`/* ai */`), types first, then globals,
  then functions. A comment and a blank line after each label say what its
  block holds.
- Comments: a short summary above every function (what it does, in a line or
  two), then only the rationale needed to read the code. State facts, not
  history, in American spelling. Record retail bugs and caller/callee
  disagreements with one line in `QUIRKS.md`. Reference library material
  validates our interfaces; never copy its implementations or comments into
  the repository.
- Trace casts, shifts, masks and unions through their readers and writers
  before changing a type. Keep genuine encoded data and format-dispatch
  boundaries; do not invent overlays or split records without evidence.
  Prefer concrete records and named fields over raw union views when the
  accesses support them; keep genuine alternate layouts, and never move a
  union's uncertainty into casts or byte arithmetic.
- `const` only for genuinely read-only inputs, checked at every caller under
  its historical compiler profile (see WLDCORE's reload rule above).
- Shared-source consolidation (`src/shared/`) keeps every original address and
  call binding, with no runtime wrappers or invented alias machinery; each
  entry still compiles with its own profile and bindings.
- Run `make fmt` after editing C or headers, then `make check`: cc1 emits
  line notes, so formatting can change the bytes.

## Matching levers

When a cleanup changes the bytes, look here first.

- Stores ignore signedness; loads, compares, shifts and divisions don't. To
  get `lh` from a `u16` (or `lhu` from an `s16`), cast the lvalue
  (`*(s16*)&g`), not the value.
- A narrow prototype adds `andi` or `sll`/`sra` at callers; widening removes
  them. Give a parameter the type whose conversion the callers show
  (`sll`/`sra`: `s16` or `s8`; `andi`: `u16` or `u8`; none and a narrow read
  in the callee: the argument was already that narrow), in the prototype and
  the definition alike, and the casts at the call sites go. Where callers
  pass full words and the callee narrows, declare the parameter `s32` and
  narrow at its uses (one narrow local). A callee that loads a narrow stack
  parameter (`lhu 0x10(sp)`) is the first case with the right signedness, not
  a reason for an old-style definition: none is left in the tree.
- Callers and callees agree in the source. An argument every caller passes and
  the callee never reads is an `unused_` parameter; a parameter no caller
  passes is a local. A function-pointer cast at a call site is the last
  resort: try the plain call first (an "extra argument" is often only a
  register the cast forced), and state the retail fact in one line if the
  cast stays.
- A register read before any write is a local the source never assigned on
  that path: a `switch` without `default`, an early exit, a call with too few
  arguments. Declare it and leave it unset (`T x;`), with a `QUIRKS.md` line,
  rather than pinning a register or adding an `"=r"` barrier.
- A fresh `lui` for each neighboring global means separate symbols. A struct
  view gives GCC one base register and reshapes the body; keep the aliases.
- Raw offset versus field access changes scheduling (`MEM_IN_STRUCT_P`):
  re-measure after converting.
- Two single-bit tests on one `u16` (`(f & 0x100) && !(f & 0x10)`) merge into
  one word `lw`+`andi` test.
- One variable in two roles across a call forces a callee-saved register;
  split or merge variables before trying priority tricks. Read conflicts
  with `cc1 -da` (`.greg`).
- Single-set constants and temps schedule late (birthing boost); a multi-set
  variable of the same width moves them earlier.
- Any store through a pointer drops CSE of cached field loads; one store
  placed before a reload reproduces a target reload. To force a reload of
  one field, use a `volatile` lvalue on that field only.
- `count = obj->count;` before the loop is one instruction shorter than
  `i < obj->count`.
- One `volatile` access in the fall-through block stops reorg filling a delay
  slot from past the join.
- `if (x) continue; break;` gives `bnez x,next; j out`; `if (!x) break;` gives
  the inverted branch.
- A prologue that copies `$a2` before `$a1` means the loop walks a local copy
  (`p = param;`).
- A loop built from a backward `goto` has no loop notes, so loop.c neither
  hoists its constants and addresses nor strength-reduces it; a constant
  reloaded inside the target's loop body points to one. A run-once
  `do { } while (0)` emits nothing but weights its references one loop level
  deeper in register allocation.
- `switch` emits all compares (a tree under 5 cases, a jump table above), then
  the bodies; an `if` chain puts each body between compares. Write case bodies
  in the target's physical order.
- Two stores sharing one `la` base (`sw 0(v1)` / `sw -4(v1)`) point to `-O1`:
  `-O2`'s second CSE pass folds them back to absolute stores.
- A frame bigger than its spills means unused locals in the original;
  declaration order sets slot order. Eight untouched bytes are more often a
  compiler ghost: an `s16` local read both sign-extended and as 16 bits, or
  a `for` whose entry test folded away.

### Reading residue

A pin, barrier, `volatile` or `goto` usually stands for something the source
said plainly. What each has turned out to be, most common first:

- Narrower locals (`u8 mask`, `s16 code`, `u16 value`): a temp plus a copy
  plus a barrier, a pinned "copy in `$v1`", a reload after a store.
- One variable where the code has two, or two where it has one: two pins on
  one register are one variable reused (`i` holding an id before it counts);
  a `thread_id = id - 1` pinned in a callee-saved register is no variable.
  In a long function, one function-scope variable per role (one `unit`, one
  index) instead of one per block.
- Indexed access written out: `for (i = 0; a[i] != END; i++)`, `g[i].f`,
  the global named in every arm. Pointer walkers, "base + offset" cursors and
  reversed scans are strength reduction's output, not source.
- A global read directly in each argument or test instead of a local copy
  ("memory" barriers, reloads). A range written as two comparisons
  (`x < A || x > B`) reloads where a folded range test does not.
- Chained assignment `a.x = a.y = b.z = v;` for same-value stores that come
  out in reverse order or keep one base register; the same store written in
  both arms of an `if` where a fence kept it early; `p = base; p += n;` as
  two statements where a cursor keeps its own register.
- Statement and declaration order: a tied barrier after a call is the
  mask/or written before the call; exact allocation ties go to the earlier
  declaration.
- Return shapes: `if (a && b) return 1; return 0;` (a `$2` pin zeroed before
  the tests); a `?:` inside the returned expression (a temp pinned to `$2`);
  the failure `return` last; `if (c) { ...; return 0; } return 0;` (a
  constant load sitting last); a callee declared `int` that returns nothing
  (`li v0` in a delay slot after a call).
- Bit-field members and struct-typed globals: separate loads of one packed
  byte, a load kept after a struct store.
- SDK macros (`setXY4`, `setRGB0`, `setSemiTrans`, `getTPage`, `addPrim` in
  `psx/libgpu.h`): several stores on one source line.
- A `do { } while (0)` wrapper is itself a lever (a loop note doubles
  reference weights and fences scheduling); remove it with the pins it
  caused.
- `goto`: a shared `return` tail is `return` written at each place; a
  backward jump is a loop; a jump past the next block is the same condition
  tested again; bound tests to one label are case ranges; `default: common:`
  is code after the `switch`. The AI's resume dispatch
  (`switch (phase) { case N: goto stage_n; }`) is real.

When only registers differ, compute instead of permuting: `cc1 -da` writes
`.lreg`/`.greg`; local-alloc and global-alloc rank pseudos by
`floor_log2(refs) * refs / live_length` with references doubled per loop
depth, and loop.c hoists an invariant while `threshold * uses * lifetime >=
loop length` (threshold 26 with a call in the loop, 52 without, minus 3 per
value already moved).

Tolerated residue (zero-instruction only, each with a comment): the tied
operand barrier `__asm__("" : "=r"(x) : "0"(x))`, an empty `asm volatile`, a
`register … __asm__("$N")` pin, a dead local kept for frame size, and a
second link name for one address. Each is debt: try removing it, and keep it
only if removal changes the bytes.

## Improving the code

Useful work, each step verified with `make check`:

- Name the remaining `D_` globals, `func_` functions and `_unknown_`
  members from what the code does with them, and examine `_unused_` members
  (they may still be touched by whole-struct copies or another view).
- Remove register pins, asm barriers, casts and raw offsets where a natural
  spelling matches. Most pins stand for something the source said another
  way: an unset local, one variable where the code has two (or two where it
  has one), a narrower local, an unused parameter, another statement order.
- Replace artificial `goto` with the loops, `if`/`else` and `switch`
  the compiler actually produced.
- Fold adjacent globals the code proves are one record; keep separate
  symbols where the original reloads each address.
- Give `s32` handles and `void*` parameters their real types.

## Working with agents

For large tasks, split the work into disjoint files or modules and give each
worker its own scratch path under `build/`. One lead edits shared headers and
`target/*.yaml`, integrates results and runs the final `make build`. Workers
report the address range, match result and any proposed header change. Never
commit; the user commits.
