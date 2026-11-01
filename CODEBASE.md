# Codebase guide

How the decompiled *Final Fantasy Tactics* (`SCUS-94221`) fits together: what
is on the disc, how control moves between modules while the game runs, the
engine services every module uses, and where to look for a mechanic.

Related pages:
- [`AGENTS.md`](AGENTS.md): the working rules (layout, names, types, matching).
- [`QUIRKS.md`](QUIRKS.md): retail bugs and code that looks wrong on purpose.
- [`docs/`](docs/README.md): one page per game mechanic and guides for common
  changes.
- `make map`: generates a browsable page for every function, type and global
  (see [Finding things](#finding-things)).

## The disc

The game is one resident executable plus overlays that it loads from the CD
to fixed addresses. Each module has a configuration in [`target/`](target/)
and a source folder in [`src/`](src/). The configuration holds the address,
size and hash of every function, the names of its data, the data words that
hold an address, a disc sector or a RAM window's size, and the regions kept
as original bytes. Beside them, [`target/disc.yaml`](target/disc.yaml)
describes the disc's files and [`target/ram.yaml`](target/ram.yaml) the RAM
windows the modules load into.

| Module | Disc file | Loaded at | What it runs |
|---|---|---|---|
| `main` | `SCUS_942.21` | `0x80010000`, resident | Boot, the top-level game loop, disc reads, memory, sound, pads, memory card, the shared data tables and the linked Sony libraries |
| `opening` | `OPEN/OPEN.BIN` | `0x80067000` | Opening movie, title screen and the ending ([Opening and Title](docs/mechanics/Opening%20and%20Title.md)) |
| `wldcore` | `WORLD/WLDCORE.BIN` | `0x80067000` | The world map (its loop, drawing, travel, random battles and calendar), the menu stack, propositions and the location scripts' opcodes |
| `world` | `WORLD/WORLD.BIN` | `0x800e0000` | The menus over the world map (formation, shops, the location list, the memory card) and a copy of the event-script interpreter that never runs |
| `battle` | `BATTLE.BIN` | `0x80067000` | Battles and the story scenes played on battle maps |
| `event-*` | `EVENT/*.OUT` | `0x801bf000` | Screens and loaders that BATTLE runs on demand: unit, equipment and job menus (`BUNIT`, `EQUIP`, `JOBSTTS`), the scenario loader (`ATTACK`) and others ([Battle Screens](docs/mechanics/Battle%20Screens.md)) |
| `effect-*` | `EFFECT/E*.BIN` | `0x801c2500` | 110 ability effects, run with BATTLE ([Ability Effects](docs/mechanics/Ability%20Effects.md), [Effect Files](docs/mechanics/Effect%20Files.md)) |

Memory follows from these addresses:
- BATTLE, WLDCORE and OPEN share `0x80067000`, so only one of them is loaded
  at a time.
- WORLD sits above WLDCORE and runs with it.
- The event and effect overlays sit above BATTLE.

A function links against its own module, then the modules listed under
`links:` in its configuration, then MAIN. It never calls into a module that
is swapped out. A file offset is the address minus the module's `load`.

## Boot and the game loop

`main` ([`src/main/main.c`](src/main/main.c)) runs three steps:

1. **[`main_boot_run_startup`](src/main/main_boot_run_startup.c):**
   - clears the allocator table;
   - installs the VSync, DrawSync and CD callbacks;
   - resets the GPU, pads, SPU and CD;
   - draws the SCEA and Squaresoft logos;
   - opens the memory card events and the generic sound effects;
   - loads the zodiac frame and seeds `rand` with 1.
2. **Saves its stack pointer** with `main_system_store_stack_pointer.s`, which
   `main_restore_game_loop_stack_pointer.s` restores on a reset
   (`main_system_reset_game`) before jumping back to the top of the game loop.
   Both are original hand-written routines; `QUIRKS.md` lists them all under
   "Hand-written assembly".
3. **Enters [`main_system_run_game_loop`](src/main/main_system_run_game_loop.c)**,
   which never returns:

```
forever:
    OPEN.BIN: opening and title         main_overlay_exec_open_bin_main_loop
    repeat:
        WLDCORE + WORLD: world map      main_overlay_exec_wldcore_and_world_bin
        BATTLE.BIN: battle or scene     main_overlay_exec_battle_bin
                                        battle_state_run_game_loop
    until a reset (back to OPEN) or the ending
    ending: OPEN.BIN ending, one last BATTLE.BIN run, then reset
```

Three globals steer the loop:
- **`g_main_system_go_straight_to_battle`:** skips the world map after the
  title.
- **`g_main_system_frontend_world_result`:** a world result of 5 returns to
  OPEN without resetting the game state.
- **`g_main_system_game_flow_state`:** 2 resets the game, 3 plays the ending.

Each overlay then runs its own loop. BATTLE runs a state machine
(`battle_state_*`, about 100 functions) and a turn clock (`battle_turn_*`); the
[Battle flow](docs/mechanics/Battle%20flow.md) page walks through both. WLDCORE
starts at `wldcore_entrypoint` and returns the world result to MAIN; the
[World Map and Travel](docs/mechanics/World%20Map%20and%20Travel.md) page walks through it.

## Engine services

The [Engine core](docs/mechanics/Engine%20core.md) page covers these services
in more depth, with the memory map and where to change each one.

### Disc reads

[`main_file_load_data_from_disc`](src/main/main_file_load_data_from_disc.c)
works like this:
- it takes an absolute sector, a length in sectors and a destination;
- it advances an asynchronous CD read once per frame
  (`main_file_poll_load`, then `VSync`) and keeps the loading indicator
  current.

Each sector and length in the code is a name from
[`include/fft/disc.h`](include/fft/disc.h), which `make config-fmt` writes
from [`target/disc.yaml`](target/disc.yaml):
- `DISC_SECTOR_BATTLE_BIN` in `main_overlay_exec_battle_bin`;
- `DISC_SECTOR_WORLD_WLDCORE_BIN` and `DISC_SECTOR_WORLD_WORLD_BIN` in
  `main_overlay_exec_wldcore_and_world_bin`, which reads WORLD's whole RAM
  window (`RAM_WINDOW_BYTES_WORLD`) from that sector.

The data tables that hold sectors (the effect files', the event overlays',
the music's) are `disc` words, which `make build` writes from the same file.
So a file that moves on the disc takes its reads with it (see
[What can change today](#what-can-change-today)).

### Memory

Overlays load at `g_main_heap_low_overlay_load_address`: BATTLE.BIN gets
everything up to `g_main_heap_high_overlay_load_address`.

The game allocator ([`main_heap_alloc`](src/main/main_heap_alloc.c)) manages
the area above that, in 64 cells of 2 KB:
- it takes the first run of free cells that fits;
- it tags the cells with a new allocation id;
- it returns 0 when nothing fits.

The sound data has its own table (`main_heap_alloc_smd`), and the Suzuki
sound driver keeps a separate CPU heap.

### Threads

BATTLE and WORLD each schedule cooperative native threads:
- **Slots:** `0x400` bytes each, with about 900 bytes of stack
  (`thread_t` in [`include/fft/thread.h`](include/fft/thread.h)).
  Slot 0 is the overlay's main loop; BATTLE runs threads in slots 1–15 and
  WORLD in slots 1–16.
- **A frame:** the main loop yields once a frame
  (`battle_script_run_event_frame`, `world_script_run_event_frame`). Each
  module's hand-written `thread_yield` saves the thread's registers in its
  slot and resumes the next running slot, returning to slot 0 after the
  last. So every running thread advances once a frame,
  and `*_thread_wait_frames(n)` waits n frames.
- **Starting and identifying:** `*_thread_resolve_id` finds a free slot,
  `*_thread_set_parameters` passes the inputs, `thread_start` starts the
  function, and a `task_id` lets other threads find it.
- **Large calls:** a thread runs disc reads and overlay loads through
  `*_thread_call_on_main_stack`, on the main loop's stack.
- **What runs this way:** menu cursors, event unit moves and dialogue text.

### Data tables

MAIN holds the shared tables that both sides read:
- abilities, items, jobs, skillsets and statuses
  ([`include/fft/data.h`](include/fft/data.h));
- unit records ([`include/fft/unit.h`](include/fft/unit.h)).

The repository holds their types only. The bytes stay on the disc, because
game data is never committed.

### Event scripts

Story scenes are bytecode, run by the BATTLE interpreter
(`battle_script_*`). WORLD carries a copy (`world_script_*`) that nothing
starts, so it never runs.

WLDCORE runs two smaller script languages of its own: each location's
scripts on the world map (`wldcore_script_*`) and the sound novels
(`wldcore_sound_novel_*`). The [Event Scripts](docs/mechanics/Event%20Scripts.md)
page covers them all, with OPEN's credits script and the Formation tutorials.

[`include/fft/script.h`](include/fft/script.h) declares the bytecode and the
script variables that both interpreters share. The game loop itself writes
two of them before the ending.

### Sound, graphics and libraries

- **Sound:** MAIN's `main_sound_*` controls it, and the Suzuki driver
  (`suzuki_smd_*`, `Suzuki*`) plays it. The [Sound](docs/mechanics/Sound.md)
  page covers the music, the tunes and the sound effects, and the
  [Sound Driver](docs/mechanics/Sound%20Driver.md) page the driver.
- **Graphics:** they go through the Psy-Q GPU and GTE libraries
  ([`src/lib/psyq/`](src/lib/psyq/)), in fixed point where `ONE` (4096) is 1.0. The
  [Battle Map](docs/mechanics/Battle%20Map.md) and
  [Sprites and Graphics](docs/mechanics/Sprites%20and%20Graphics.md) pages
  cover how a battle is drawn.
- **Other libraries:** WORLD links LIBGS, and OPEN links LIBPRESS for the
  movies.

All of these libraries are reconstructed C, matched like the game code; the
[Sony Libraries](docs/mechanics/Sony%20Libraries.md) page lists what each one
does for the game.

## Source layout and names

- **Files:** one function per file, at `src/<module>/<name>.c`. Sony
  libraries live under `src/lib/psyq/<library>/`.
- **Headers:** one per module in [`include/fft/`](include/fft/), plus shared
  type headers. Each header groups its declarations under short subsystem
  labels such as `/* ai */`, each opened by a comment that says what its
  block holds.
- **Names:** a module prefix, a subsystem word, then the action:
  `battle_move_...`, `world_shop_...`. The prefix and subsystem word form
  the *code subsystem* that the map groups functions by. Unknowns stay
  `func_ADDRESS` and `D_ADDRESS`, and struct members of unknown meaning stay
  `_unknown_XX`.
- **Copies:** overlays repeat code. 115 groups of functions are byte for byte
  identical (same hash) under different names in different modules.

[`AGENTS.md`](AGENTS.md) has the full rules.

## Finding things

- **The vault:** `make map` writes an Obsidian vault to `build/map/`; open
  that folder as a vault. On Windows, Obsidian cannot open a vault inside
  WSL: add `MAP_EXPORT=/mnt/d/fft-map` and open that copy.
  - **Home** lists modules, code subsystems, mechanics and reports.
  - **Function pages:** each one shows the summary, the code, callers and
    callees, the globals and types it uses, and its `QUIRKS.md` entries.
  - **Bases views:** sort and filter every function.
  - **`build/map/functions.tsv`:** has one line per function, for grep
    (`grep -i formula build/map/functions.tsv`).
- **Mechanics and guides:** [`docs/mechanics/`](docs/README.md) pages give
  the purpose, data, flow and "where to change" of each mechanic, with the
  list of functions in its scope: [Battle flow](docs/mechanics/Battle%20flow.md),
  [Movement](docs/mechanics/Movement.md),
  [Targeting](docs/mechanics/Targeting.md),
  [Action](docs/mechanics/Action.md),
  [Formulas](docs/mechanics/Formulas.md),
  [Status](docs/mechanics/Status.md),
  [Abilities](docs/mechanics/Abilities.md),
  [AI](docs/mechanics/AI.md),
  [Units and Jobs](docs/mechanics/Units%20and%20Jobs.md),
  [Items and Shops](docs/mechanics/Items%20and%20Shops.md),
  [Roster and Formation](docs/mechanics/Roster%20and%20Formation.md),
  [World Map and Travel](docs/mechanics/World%20Map%20and%20Travel.md),
  [Propositions and the Bar](docs/mechanics/Propositions%20and%20the%20Bar.md),
  [Event Scripts](docs/mechanics/Event%20Scripts.md),
  [Menus](docs/mechanics/Menus.md),
  [World Map Menus](docs/mechanics/World%20Map%20Menus.md),
  [Battle Screens](docs/mechanics/Battle%20Screens.md),
  [Save and Memory Card](docs/mechanics/Save%20and%20Memory%20Card.md),
  [Text](docs/mechanics/Text.md),
  [Opening and Title](docs/mechanics/Opening%20and%20Title.md),
  [Camera](docs/mechanics/Camera.md),
  [Battle Map](docs/mechanics/Battle%20Map.md),
  [Sprites and Graphics](docs/mechanics/Sprites%20and%20Graphics.md),
  [Sound](docs/mechanics/Sound.md),
  [Ability Effects](docs/mechanics/Ability%20Effects.md),
  [Effect Files](docs/mechanics/Effect%20Files.md),
  [Sound Driver](docs/mechanics/Sound%20Driver.md),
  [Sony Libraries](docs/mechanics/Sony%20Libraries.md) and
  [Engine core](docs/mechanics/Engine%20core.md). The guides walk
  through common changes, such as the
  [Movement rules](docs/guides/Movement%20rules.md),
  [Changing a formula](docs/guides/Changing%20a%20formula.md),
  [Changing a status](docs/guides/Changing%20a%20status.md),
  [Changing an ability](docs/guides/Changing%20an%20ability.md),
  [Changing the AI's decisions](docs/guides/Changing%20the%20AI's%20decisions.md),
  [Changing jobs and growth](docs/guides/Changing%20jobs%20and%20growth.md),
  [Changing items and shops](docs/guides/Changing%20items%20and%20shops.md),
  [Changing the formation and roster](docs/guides/Changing%20the%20formation%20and%20roster.md),
  [Changing travel and random encounters](docs/guides/Changing%20travel%20and%20random%20encounters.md),
  [Changing propositions](docs/guides/Changing%20propositions.md),
  [Changing an event](docs/guides/Changing%20an%20event.md) and
  [Changing a menu](docs/guides/Changing%20a%20menu.md).
- **Calls through tables:** they do not appear in C, so about 1300 functions
  have no C caller. Their pages say so.
- **Debugging:** `make map` also writes PCSX-Redux symbol maps to
  `build/symbols/`, so the debugger shows these names. `make run` launches
  the emulator, and `make smoke` boots the original and the shifted test
  disc in it from a script that checks the code in RAM at each checkpoint.
- **Original names:** the vault's Upstream renames page translates the
  names of the decompilation this project started from
  ([adamrt/fft_decomp](https://github.com/adamrt/fft_decomp), as of its last
  merge, `f59f939d`) into this project's, by address.
  [`docs/upstream-renames.tsv`](docs/upstream-renames.tsv) holds that table,
  which GitHub shows as a searchable one; `make map` keeps its last column
  on the current names.

## What can change today

`make build` links each module whole, as the original linker did: every
function's object and, between them, the original data bytes read from the
BIN. A data word that holds an address is a relocation, a word that holds a
disc sector or a RAM window's size comes from `target/disc.yaml` or
`target/ram.yaml`, and a name another module places takes the address that
module's link gives it. So when code or data moves, what names it follows:
- `make build SHIFT=16` writes a test disc on which every module's image
  opens 16 bytes at its start, and checks that every changed word moved with
  what it names;
- `make smoke` boots that disc and the original in PCSX-Redux, from a new
  game through Orbonne and Gariland to the world map.

The product is still the original disc: every function must match its hash,
every module its original file and the disc its SHA-256 (`make patch-check`
builds it again the old way). So today the tree supports renames, types,
comments and documentation, and rewrites that keep the bytes.

A few addresses stay fixed on purpose: the RAM areas outside every module
image, the GTE matrix stack's page, the names WORLD's never-run copies of
BATTLE and EVENT code bind, and the MAP files, whose sectors the GNS files
store.

Gameplay changes wait for build 3, which accepts a disc that differs:
- code that grows or moves links where the layout puts it, up to its RAM
  window's end (`make build` prints each window's room), and
  `make config-fmt` moves a file that outgrows its sectors to the disc's free
  run;
- a table that grows needs its bytes in source, since the build reads data
  from the BIN at its original size;
- the `QUIRKS.md` entries and the "Type debt / Port debt" comments become
  optional fixes (the vault's Build 3 backlog page).
