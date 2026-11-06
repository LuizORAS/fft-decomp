---
type: mechanic
tier: 1
scope:
  - main()
  - main_restore_game_loop_stack_pointer()
  - main_system
  - main_boot
  - main_overlay
  - main_file
  - main_heap
  - battle_heap
  - battle_thread
  - world_thread
  - suzuki_sound_disable_root_counter_2_event()
  - suzuki_sound_enable_root_counter_2_event()
  - thread_exit_current()
  - thread_get_current_global_pointer()
  - thread_get_current_task_id()
  - thread_is_running()
  - thread_reset_scheduler()
  - thread_resume()
  - thread_set_current_task_id()
  - thread_set_four_parameters()
  - thread_set_parameters()
  - thread_start()
  - thread_suspend()
  - thread_wait_for_unit_view_panels()
  - thread_wait_frames()
  - thread_wait_until_inactive()
---

# Engine core

## What it does in the game

What every scene stands on, though the player never sees it: booting the
console and showing the logos, the top-level loop that moves between the
title, the world map and battles, loading each part of the game from the
disc, sharing the 2 MB of RAM, and running many small tasks side by side every
frame (menu cursors, dialogue text, event actors).

## Data and structures

**Memory map.** MAIN stays resident; the overlays share fixed windows, which
`target/ram.yaml` describes. The words at the start of MAIN's data
(`0x80010000`) hold the load addresses; `make build` writes them from that
file and fails on an image that passes its window's end.

| Range | Holds |
|---|---|
| `0x80010000`– | MAIN (`SCUS_942.21`), with the SMD heap for sound files at `0x8004eb18` (16 cells of 2 KB) |
| `0x80067000`– | OPEN, WLDCORE or BATTLE, one at a time ([[g_main_heap_low_overlay_load_address]]) |
| `0x800e0000`– | WORLD, beside WLDCORE ([[g_main_heap_world_overlay_load_address]]) |
| `0x801bf000`– | EVENT and EFFECT overlays, beside BATTLE ([[g_main_heap_companion_overlay_load_address]]) |
| up to `0x801df000` | the battle heap, from the effect palette buffer ([[g_battle_heap_end_address]]) |
| `0x801df000`–`0x801ff000` | the game heap, 64 cells of 2 KB ([[g_main_heap_high_overlay_load_address]]); companion overlays load over it after reserving it |
| `0x801ff000`– | the main stack, which a deep WORLD frame can push into the heap's top cells |

**Records and globals.**
- Disc reads: [[main_file_load_descriptor_t]], one read in flight per
  descriptor; [[g_main_file_cd_state]] is the shared one, and
  [[g_main_file_still_loading]] is its state word under a second name.
- Heaps: [[g_main_heap_game_allocator_table]] and
  [[g_main_heap_smd_allocator_table]] hold one tag per cell (0 is free);
  the battle heap is a free list of [[battle_heap_node_t]] blocks from
  [[g_battle_heap_rover]], with one list of live blocks per owner.
- Threads: [[thread_t]] slots of `0x400` bytes reached through
  [[g_threads]] (BATTLE and WORLD each have one); the running one is
  [[g_thread_current_id]]. Task ids
  ([[thread_task_e]]) let threads find each other.
- The game loop: [[g_main_system_game_flow_state]] (2 resets, 3 plays the
  ending), [[g_main_system_go_straight_to_battle]] and
  [[g_main_system_frontend_world_result]] (5 returns to the title).

## Main flow

1. **Boot.** [[main]] runs [[main_boot_run_startup]] (callbacks, GPU, pads,
   SPU and CD reset, the SCEA and Squaresoft logos, memory card and sound
   setup), saves its stack pointer for resets, then enters
   [[main_system_run_game_loop]], which never returns.
2. **The game loop.** Before each title, [[main_boot_reset_game_state]]
   starts a session. Then [[main_overlay_exec_open_bin_main_loop]] runs the
   title, and [[main_overlay_exec_wldcore_and_world_bin]] and
   [[main_overlay_exec_battle_bin]] alternate until a reset or the ending.
   Each loads its overlay from its file's first sector (a `DISC_SECTOR_*`
   name from `target/disc.yaml`) and calls its entry point; BATTLE's entry
   call lands on MAIN's 4 KB of nops (`main_overlay_battle_entry_stub`), and
   the game loop runs [[battle_state_run_game_loop]] itself.
3. **A frame inside an overlay.** The overlay's main loop is thread slot 0.
   Once a frame its event or menu frame function
   ([[battle_script_run_event_frame]], [[world_script_run_event_frame]]) yields:
   [[thread_yield]] saves its registers and resumes the next running
   slot, each thread runs until its own next yield, and after the last slot
   (15 in BATTLE, 16 in WORLD) control returns to slot 0. Every running
   thread therefore advances once a frame, and [[thread_wait_frames]]
   waits that many frames.
4. **Disc reads while playing.** [[main_file_request_read_bytes]] arms a read
   on the shared descriptor; the overlay's frame loop advances it with
   [[main_file_poll_load]] and tests [[main_file_is_still_loading]]. A thread
   makes such calls through [[thread_call_on_main_stack]], because its
   own stack is about 900 bytes. Waiting loaders
   ([[main_file_load_to_address]], [[main_file_alloc_and_load]]) poll in a
   loop of their own.
5. **Memory.** Files that are uploaded and freed (images, fonts) take game
   heap cells through [[main_heap_alloc]]; menus that open a companion
   overlay first claim the heap's cells with [[main_heap_reserve_at]]; the
   effect stage rebuilds the battle heap ([[battle_heap_init]]) for its own
   blocks.
6. **Soft reset.** [[main_system_reset_game]] stops the display and audio and
   jumps back to the top of the game loop on the stack saved at boot
   ([[main_system_store_stack_pointer]] and
   [[main_restore_game_loop_stack_pointer]]).

## Where to change

- **Order of title, world map and battles:** [[main_system_run_game_loop]]
  and its three globals.
- **Boot logos and startup:** [[main_boot_run_startup]],
  [[main_boot_show_sceap_logo]], [[main_boot_fade_in_squaresoft_logo]].
- **Load a file:** [[main_file_alloc_and_load]] for a temporary buffer,
  [[main_file_load_to_address]] for a fixed one, or
  [[main_file_request_read_bytes]] plus polling to keep the frame running.
  The callers name each file's sector and length with the `DISC_SECTOR_*`
  and `DISC_SECTOR_COUNT_*` names `make config-fmt` writes from
  `target/disc.yaml`, so a file that moves takes its reads with it.
- **Run a task beside the main loop:** take a slot from
  [[battle_thread_resolve_id]], pass inputs with
  [[thread_set_parameters]], start it with [[thread_start]],
  and yield every frame; stop with [[thread_exit_current]]. WORLD has
  the same functions under `world_thread_`.
- **Memory:** [[main_heap_alloc]] gives 2 KB cells (128 KB in all);
  [[battle_heap_alloc_block]] serves the battle effects.
- **Today's limits:** the original build keeps every byte; a change goes behind
  an option into the Developer's Cut, where code can grow and move, a table's
  bytes change in place, and everything that names them follows (`CODEBASE.md`,
  "What can change today"). The context switch and the main-stack bridge are
  hand-written MIPS, so a port rewrites them.

## Quirks and debts

- [[main_heap_alloc]] can give a block the tag of the block after it, and
  freeing the first then frees both (latent).
- [[battle_heap_alloc_block]] has no out-of-memory exit.
- [[battle_thread_resolve_id]] and its twins return a leftover `$v0` when no
  slot is free, which a main-loop caller receives.
- WORLD runs slot 16, which [[thread_reset_scheduler]] does not clear.
- Thread parameters are `s32` but often carry pointers (QUIRKS.md, "Pointers
  held in 32-bit integers").
- [[world_card_run_menu_screen]]'s stack frame reaches into the game heap's
  top cells.

## Functions in scope

![[Engine core scope]]
