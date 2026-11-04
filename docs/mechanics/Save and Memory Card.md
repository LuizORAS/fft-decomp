---
type: mechanic
tier: 3
scope:
  - card
  - world_card
  - open_card
  - main_card
  - main_save
  - menu_bits_init_secondary_reader()
  - menu_bits_read_secondary()
  - menu_set_palette_mode()
  - menu_text_render_id_rows_to_vram()
---

# Save and Memory Card

## What it does in the game

A game is saved to a memory card in one of 15 slots, each a one-block file.
Three places reach the card:

| Where | What the player sees | Code |
|---|---|---|
| The world map | The world menu's save row opens the memory card screen: choose the card, then save or load | [[world_card_open_menu]], [[world_card_update_menu_request]], [[world_card_run_menu_screen]] (WORLD.BIN) |
| The title | Continue opens the same screen to load; New Game first checks that a card answers, and warns when none does | [[open_title_update_menu]], [[open_card_check_slots]], [[open_title_start_new_game]] |
| After some events | The event script offers to save (CallFunction OPEN_SAVE_MENU): CARD.OUT's save screen, run through companion overlay state CARD_SAVE | [[card_save_run_menu]] ([[Battle Screens]]) |

### The screens

WORLD's screen steps through [[world_card_menu_step_e]] and CARD's through
[[card_save_menu_state_e]]; the steps match:

1. **The card.** Slot 1 or 2.
2. **The scan** ([[card_save_update_slot_scan]], [[world_card_update_slot_scan]]):
   open the card (a new card asks to be formatted), list its save files until
   two listings agree, count the free blocks, then read each of the 15 files'
   summary, one a frame. A slot is occupied, available while free blocks last,
   or unavailable ([[card_save_slot_state_e]]). A file whose summary slot reads
   0x10 or more, which marks a write that never finished, is deleted.
3. **The slot list** ([[card_save_update_and_draw_slot_list]]): four rows show
   at a time, each the slot's leader name, job, level, date, location and play
   time ([[card_save_build_slot_description]]). Saving over an occupied slot
   asks first.
4. **Saving** ([[card_save_update_slot_write]], [[world_card_run_save_step]]):
   build the save image and write it to the slot's file.
5. **Loading** (WORLD only, [[world_card_run_load_step]]):
   [[world_card_load_globals_from_save_image]] checks the image and restores
   the game from it; the world map then restarts the game loop
   (`MAIN_SYSTEM_FLAG_RESTART`).

Slots read in one visit stay known to the next
(`g_main_card_save_slot_metadata`, `g_main_card_save_last_used_slot`), so the
scan is skipped while the card still answers.

### The save image

[[card_save_build_buffer]] and WORLD's copy
[[world_card_build_save_image]] fill a 0x1e00-byte image
([[card_save_buffer_t]]) from the game's globals; the loader reads it back in
the same order.

| Offset | Contents |
|---|---|
| 0x000 | The memory card header ([[card_save_header_t]]): 'SC', one icon frame, one block, the Shift-JIS title with the slot number and play time, the icon ([[card_save_build_file_header]]) |
| 0x100 | The slot line ([[card_save_summary_t]]): the slot (0xff until the write is done), the leader's name, job and level, the month, day and location, and format version 4 |
| 0x118 | One parity bit per 128-byte block |
| 0x120 | The play time in seconds |
| 0x124 | The world state: treasure, land and proposition dates, proposition states and progress, the sound novels' reading states, the Brave Story's character ages |
| 0x484 | The 20 party records, 0xe0 bytes each |
| 0x1604 | Item and poached item quantities, then the Move-Find item flags |
| 0x1884 | The 256 script variables |
| 0x1c84 | The game options, then the item type and page orders |

A load is refused when the slot byte still reads 0xff, the format version is
not 4, or a block's parity disagrees. The tutorial's partial load skips the
parity check and keeps the play time.

### Card input and output

- MAIN opens eight BIOS card events at boot ([[main_card_init_events]]); the
  CARD, WORLD and OPEN pollers test them (done, error, timeout, new card) and
  report a [[card_io_result_e]].
- Every operation retries: _card_info, _card_load and _card_clear, and the
  file calls open, seek, read, write and delete up to ten times, a short
  transfer seeking back and trying again. A new card is acknowledged with
  _card_clear; formatting calls format() three times.

### A new game

[[main_save_init_state_and_options]] and [[main_save_init_data_tables]] reset
what a save holds: the script and saved words, the calendar, the options and
the data tables.

## Data and structures

| Record | What it holds |
|---|---|
| [[card_save_buffer_t]] | The save image |
| [[card_save_header_t]] | The memory card header |
| [[card_save_summary_t]] | A slot's line on the screen |
| [[card_save_slot_state_e]] | Unscanned, unavailable, occupied, available |
| [[card_io_result_e]] | A card operation's outcome |
| [[world_card_menu_step_e]], [[card_save_menu_state_e]] | The two screens' steps |
| `g_main_card_save_slot_metadata`, `g_main_card_save_last_used_slot` | The slot lines kept between visits, and the last slot loaded or written |

## Main flow

1. **Open.** The world menu, the title's Continue or an event opens the
   screen.
2. **Scan.** The chosen card is opened and its 15 slots read.
3. **Save or load.** A slot is chosen; the image is built and written, or read
   and restored.
4. **Leave.** The screen fades out; after a load the game restarts from the
   loaded state.

## Where to change

| To change | Edit |
|---|---|
| What a save holds | [[card_save_buffer_t]], and both writers ([[card_save_build_buffer]], [[world_card_build_save_image]]) with the loader ([[world_card_load_globals_from_save_image]]) |
| When a save is accepted | [[world_card_load_globals_from_save_image]] |
| A slot's line | [[card_save_build_slot_description]], [[world_card_build_save_slot_description]] |
| The card header and icon | [[card_save_build_file_header]], [[world_card_build_save_file_header]] |
| Retries and timing | The `card_io_*` and `card_file_*` routines and their `world_card_*` copies |
| A larger save | The image fills one 8 KB card block; more needs a second block, a change to the save format that waits for the maintainer's decision (`AGENTS.md`, "Changes under an option") |

## Quirks and debts

- A save interrupted before its slot byte is written is deleted by the next
  scan.
- Both screens end by asking the backdrop thread to stop through the wrong
  parameter; CARD also stops a slot-14 thread and suspends slot 15, which it
  never starts.
- [[world_card_build_save_slot_description]] reads a job-name pointer that a
  void call leaves in `$v0`.
- [[world_card_run_menu_screen]]'s stack frame of packet pools reaches into
  the top cells of the game heap.
- [[world_card_read_file_chunked]] marks a short last read and still returns
  the total.

## Functions in scope

![[Save and Memory Card scope]]
