---
type: mechanic
tier: 3
scope:
  - open_system
  - open_controller
  - open_input
  - open_file
  - open_gfx
  - open_sound
  - open_movie
  - open_title
  - open_menu
  - open_prologue
  - open_birthday
  - open_text
---

# Opening and Title

## What it does in the game

OPEN.BIN runs everything before the first battle of a new game and the
ending's movie and credits. MAIN loads it at the start of every pass of the
game loop and once for the ending.

| Screen | What the player sees | Code |
|---|---|---|
| Title movie | FFTST.STR; Start skips the video, its audio goes on as the title music | [[open_movie_push_fftst_controller]], [[open_movie_update_fftst_controller]] |
| Title menu | New Game, Continue, Tutorial, Sound. Once the title music has stopped and 601 frames have passed without a cursor move, it plays a demo movie (FFTPRE, FFTPR, FFTUNIT in turn), then FFTST.STR again | [[open_title_push_menu_controller]], [[open_title_update_menu]], [[open_movie_update_demo_controller]] |
| Sound | Mono, stereo or wide, kept in the game options | [[open_menu_push_sound_type_controller]], [[open_menu_handle_sound_type_input]] |
| Memory card warning | New Game with no card: "You need a Memory card (with at least 1 block free) to save your current game"; Cross goes back, Circle goes on | [[open_title_start_new_game]], [[open_title_update_card_warning]] |
| Prologue | Alazlam's narration ("I am Alazlam, a scholar of ancient Ivalice history..."), line by line over crossfading pictures, ending with "please tell me your 'name' and 'birthday'" | [[open_prologue_start]], [[open_prologue_update]] |
| Name | WORLD's name keyboard as a screen of its own ([[Text]]) | [[open_title_step_new_game_start]], [[world_name_run_entry_screen]] |
| Birthday | Month and day, then a Yes/No confirmation; the day of the year and the zodiac sign go into Ramza's record | [[open_birthday_push_date_controller]], [[open_birthday_handle_confirmation_input]] |
| Opening movie | FFTOP.STR, then the first battle | [[open_movie_push_fftop_controller]], [[open_movie_update_fftop_controller]] |
| Continue | WORLD's memory card screen loads a game ([[Save and Memory Card]]) | [[open_title_update_menu]], [[world_card_run_menu_screen]] |
| Tutorial | Daravon's tutorial menu on the world map; leaving it returns to the title without the movie | [[open_title_update_menu]] |
| Ending | FFTEND.STR, then the credits script over the ending's pictures ([[Event Scripts]]) | [[open_system_run_ending_cutscene]], [[open_movie_update_fftend_controller]] |

The Music Test, a list of the 96 tracks with the composers' comments, is still
in the code but nothing in this release opens it (see the quirks).

### Controllers

Every screen is a controller: a handler ([[open_controller_handler_e]]) and a
100-byte record ([[open_controller_record_t]]) on a stack.
[[open_controller_dispatch_current]] runs the top one each frame; a screen
pushes the next one, replaces itself (index - 1, then a push) or pops.

| Id | Handler | Screen |
|---|---|---|
| 0 | [[open_movie_update_fftst_controller]] | FFTST.STR |
| 1 | [[open_movie_update_fftop_controller]] | FFTOP.STR |
| 2 | [[open_title_update_menu]] | Title menu |
| 3 | [[open_menu_handle_sound_type_input]] | Sound type |
| 4 | [[open_title_step_new_game_start]] | New Game: prologue, name, birthday |
| 5 | [[open_birthday_handle_menu_input]] | Birthday date |
| 6 | [[open_birthday_handle_confirmation_input]] | Birthday confirmation |
| 7 | [[open_menu_update_music_test]] | Music Test |
| 8 | [[open_controller_wait_text_message]] | A WORLD message box (the Music Test's comments) |
| 9 | [[open_movie_update_fftend_controller]] | FFTEND.STR |
| 10 | [[open_script_update_controller]] | The credits script |
| 11 | [[open_movie_update_demo_controller]] | A title demo movie |
| 12 | [[open_title_update_card_warning]] | Memory card warning |

### Files

| File | Holds | Read by |
|---|---|---|
| OPEN/OPNTEX.BIN | TIMs: the title's textures (sectors 0, 0x11, 0x22, with the menu words and the card warning), the prologue's two text pages (0x43, 0x54), the credits' glyph page (0x65) and 16 ending pictures (from 0x76) | [[open_file_load_main_menu_graphics]], [[open_file_load_prologue_text_pages]], [[open_file_load_end_movie_graphics]] |
| OPEN/OPNBK.BIN | Seven pictures: 0 the title (320x240), 1 the prologue and Music Test backdrop (256x240), 2-5 the prologue's pictures (210x180), 6 the ending's (512x240) | [[open_file_build_openbk_header]] |
| OPEN/FFTST.STR, FFTPRE.STR, FFTPR.STR, FFTUNIT.STR, FFTOP.STR, FFTEND.STR | The movies | [[open_movie_start_stream]] |
| OPEN/ENDING.XA | The credits music | [[open_script_update_xa_audio]] |
| EVENT/OPEN.LZW | The unit names, the birthday prompts and the Music Test's titles and comments, loaded by WORLD | [[world_menu_init_with_open_text]] |

## Data and structures

| Record | What it holds |
|---|---|
| [[open_controller_record_t]], [[open_controller_handler_e]] | A controller's record and handler id |
| [[open_system_runtime_flag_e]] | `g_open_system_runtime_flags`: running, input disabled, display blanked, screen width, WORLD loaded, the new-game party built |
| [[world_animated_render_record_t]], [[open_render_record_56_t]] | The two render records: animated sprites (menu words, cursor, warning) and window images (birthday windows) |
| [[open_overlay_fade_t]] | The full-screen fade to and from black |
| [[open_title_controller_t]] | The title menu: its records, cursor, idle timer and where the title music stops |
| [[open_prologue_t]], [[open_prologue_entry_t]] | The prologue's state and its 12 pages (`g_open_prologue_entries`) |
| [[open_birthday_date_state_t]], [[open_birthday_confirmation_state_t]] | The two birthday screens |
| [[open_movie_mdec_stream_state_t]] | The MDEC decoding buffers of a movie |

## Main flow

1. **Enter.** [[main_overlay_exec_open_bin_main_loop]] loads OPEN.BIN and
   runs [[open_system_run_main_loop]], which sets up the title sequence
   ([[open_system_init_title_sequence]]) and then, each frame, reads the disc,
   steps the movie, runs WORLD's menu frame when the Music Test needs it, runs
   the current controller, draws the render records and reads the pad.
2. **Title.** FFTST.STR leads into the title menu, unless MAIN asks to skip it
   (the Tutorial's return to the title).
3. **New Game.** The card check, the warning if no card answers, the
   prologue, the name, the birthday and FFTOP.STR; OPEN returns 0 and MAIN
   goes straight into BATTLE.
4. **Continue.** A loaded game returns 0 (into BATTLE) when it has a next
   scenario and no formation return event, else 1 (the world map).
5. **Tutorial.** OPEN returns 2 with `MAIN_SYSTEM_FLAG_TITLE_TUTORIAL`, and
   the world map opens Daravon's tutorial menu.
6. **Ending.** [[main_overlay_exec_open_bin_ending]] runs
   [[open_system_run_ending_cutscene]]: FFTEND.STR, then the credits script.

## Where to change

| To change | Edit |
|---|---|
| The title menu's entries | [[open_title_push_menu_controller]] (the records) and [[open_title_update_menu]] (what each does) |
| When and which demo movies play | [[open_title_update_menu]]: the 601-frame idle count, the three movies' sectors and mix levels |
| The prologue | `g_open_prologue_entries` (pages, lines, pictures) and the OPNTEX text pages, which are images |
| Ramza's starting party | [[open_title_init_new_game_party]], which calls [[main_party_init_new_game]] |
| The birthday's calendar | `g_open_birthday_month_lengths` and [[open_birthday_handle_menu_input]] |
| The soft reset | [[open_input_update_buttons_and_check_game_reset]] (L1 + R1 + Select + Start) |
| Opening the Music Test | Step 5 of [[open_title_step_new_game_start]], which nothing sets today |
| A movie's sector or length | The `open_movie_push_*` callers, through the `DISC_SECTOR_*` and `DISC_SECTOR_COUNT_*` names `make config-fmt` writes from `target/disc.yaml`, so the reads follow a moved file |

## Quirks and debts

- Nothing sets step 5 of the New Game controller, so the Music Test cannot be
  opened; community sources say the Japanese release opened it for a special
  name. Its titles and comments were left untranslated.
- February has 28 days in the birthday entry, so 29 February cannot be
  entered.
- Three runtime flags are tested but never set: the soft-reset lock, the
  cleared frame and the hidden render records.
- [[open_card_wait_for_hardware_event]], [[open_menu_init_render_record_fields]]
  and [[open_text_measure_menu_image]] have no caller.
- The crossfade's progress row is the second byte of its row flags under its
  own name ([[open_prologue_prepare_crossfade]]).

## Functions in scope

![[Opening and Title scope]]
