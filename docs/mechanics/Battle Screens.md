---
type: mechanic
tier: 3
scope:
  - bunit
  - equip
  - jobstts
  - option
  - helpmenu
  - debugchr
  - require
  - attack
  - battle_deployment
  - deployment_skip_thread()
  - etc
  - small
  - gfx_clip_portrait_poly_from_left()
  - gfx_clip_portrait_poly_from_right()
  - gfx_load_image_and_wait()
  - gfx_move_image_and_wait()
  - gfx_set_clut_rect_from_id()
  - gfx_set_formation_icon_uv()
  - gfx_set_scaled_poly_ft4_4bpp()
  - gfx_set_scaled_poly_ft4_8bpp()
  - gfx_store_image_and_wait()
  - menu_ability_get_ct_display_value()
  - menu_ability_get_jp_cost_display_value()
  - menu_ability_get_mp_cost_display_value()
  - menu_ability_is_action()
  - menu_ability_is_action_tab()
  - menu_ability_is_learned()
  - menu_ability_is_movement_tab()
  - menu_ability_is_non_action_learned()
  - menu_ability_is_non_action_unlearned()
  - menu_ability_is_reaction_tab()
  - menu_ability_is_support_tab()
  - menu_build_item_icon_source()
  - menu_build_scaled_draw_area_packets()
  - menu_build_status_group_primitives()
  - menu_build_unflagged_portrait_poly()
  - menu_build_unit_portrait_transition()
  - menu_build_zodiac_primitives()
  - menu_draw_thread_status_indicators()
  - menu_get_event_speed()
  - menu_gfx_enqueue_draw_area()
  - menu_gfx_enqueue_draw_mode()
  - menu_gfx_enqueue_draw_move()
  - menu_gfx_enqueue_oriented_textured_quad()
  - menu_gfx_enqueue_textured_quad()
  - menu_gfx_enqueue_textured_quad_list()
  - menu_gfx_get_draw_color()
  - menu_icon_list_thread()
  - menu_icon_strip_thread()
  - menu_init_scaled_draw_area_packets()
  - menu_init_status_panel_primitives()
  - menu_input_update_controller()
  - menu_run_status_panel_thread()
  - menu_script_draw_window_frame()
  - menu_script_if_else_group()
  - menu_script_if_group()
  - menu_script_set_clut()
  - menu_script_set_draw_color()
  - menu_script_set_fixed_clut()
  - menu_script_set_texture_page()
  - menu_script_skip_group()
  - menu_select_status_primitive_color_palette()
  - menu_set_status_panel_primitive_colors()
  - menu_set_vsync_mode_and_event_speed()
  - menu_text_entry_wait_thread()
  - menu_text_find_entry_by_index()
  - menu_text_render_decimal_entry_list()
  - menu_text_render_decimal_value()
  - menu_text_render_encoded_ids_to_image()
  - menu_text_render_glyph_to_4bpp_image()
  - menu_text_render_signed_decimal_entries()
  - menu_thread_is_running()
  - menu_update_and_draw_animated_cursor()
  - menu_update_wrapped_horizontal_selection()
---

# Battle Screens

## What it does in the game

BATTLE.BIN holds the battle itself. Most of its full screens, and a few
loaders, are overlays in the EVENT folder that it reads on demand
([[battle_companion_overlay_e]]). Each loads at the companion overlay address,
0x801bf000, or above it at the game heap's base, 0x801df000, once the heap
cells there are reserved.

| Overlay | What it shows or does | Opened by |
|---|---|---|
| BUNIT.OUT | Unit List: the units in a grid with their status banner, jobs and abilities; the unit picked gets the map cursor | the mini menu's Unit List row ([[battle_menu_run_bunit_transition_thread]]) |
| EQUIP.OUT | Equip Change: a unit's equipment, the items it can wear and the stat preview | Act's Equip Change ([[battle_menu_run_equip_thread]]) |
| JOBSTTS.OUT | A unit's jobs and each job's abilities | a row of the Status screen's list ([[battle_menu_run_jobstts_thread]]); REQUIRE's recruit prompt |
| OPTION.OUT | The game options, the AT list, and the crystal or chest a dead unit leaves | the mini menu's Option and AT rows, system function 0x1f ([[option_entrypoint]]) |
| HELPMENU.OUT | The help cursor | Select over the menus ([[battle_menu_run_helpmenu_thread]]) and over REQUIRE's panels |
| REQUIRE.OUT | Event opcode DisplayConditions: banners, Congratulations, gil and war trophies, Brave and Faith changes, desertions, recruits; event opcode DarkScreen | [[require_condition_dispatch]], [[require_bonus_run_dark_screen_burst]] |
| ATTACK.OUT | The start of a battle: the next scenario's setup, formation sprites and portraits, deployment; the map title | [[battle_menu_init_attack_resources_and_threads]], event opcodes LoadAttackGraphics and ShowMapTitle |
| ETC.OUT | The chapter titles, the game-over screen and the endings | event opcode ShowGraphic ([[etc_graphic_show_async]]) |
| SMALL.OUT | The unit and job names | the battle map's start ([[battle_menu_poll_small_load]]) |
| DEBUGCHR.OUT | A debug unit editor that no menu row opens, and the Status screen of a rider or a mount | [[battle_menu_open_debugchr]], [[battle_menu_open_rider_status_screen]] |
| CARD.OUT | The save screen an event opens | event CallFunction OPEN_SAVE_MENU |

### Loading and running

- **Loading.** A thread names the overlay in
  [[menu_request_open_companion_executable]], and the frame loop reads it
  on the main stack ([[battle_menu_open_companion_executable]]): the sector,
  read size and offset from 0x801bf000 come from
  `g_battle_menu_companion_executable_lbas`, `_sizes` and `_offsets`. BUNIT and
  EQUIP are read in two halves (indexes 2 and 15, 9 and 10), the second at
  0x801df000; JOBSTTS and HELPMENU load there whole. The opener reserves those
  heap cells first (`main_heap_reserve_at`) and frees them afterwards.
- **Inline overlays.** OPTION, HELPMENU, REQUIRE, ATTACK, ETC and DEBUGCHR
  run on the thread that loaded them, which calls their routines. SMALL only
  binds its names once it is read.
- **Frame overlays.** BUNIT, JOBSTTS, EQUIP and CARD run once a frame from
  [[battle_script_run_event_frame]]: the opener sets
  `g_battle_companion_overlay_state` ([[battle_companion_overlay_state_e]]) and
  waits until it is idle again. JOBSTTS and EQUIP draw into the frame's
  ordering table before the threads run, BUNIT and CARD after them.

### Display scripts and copied helpers

- The screens draw their windows with the menu display scripts
  ([[menu_script_opcode_e]]) of WORLD's menus. Each overlay has its own
  handlers, named `<overlay>_menu_script_<verb>` after WORLD's, and its own
  runner such as [[bunit_menu_run_display_script]]; BUNIT leaves opcodes 3 and
  7 empty, JOBSTTS 3, 6, 7 and 0x0b ([[Menus]]).
- Every overlay links its own copy of the helpers it needs: threads, bit
  readers, VRAM transfers, scrollable lists and their saved positions, the
  event speed. Their summaries name the WORLD or BATTLE twin.

### The unit status panels

BUNIT, EQUIP, DEBUGCHR, REQUIRE and ATTACK carry copies of the unit view's
status panels:

- the unit status banner, thread task 0x39
  (`<overlay>_panel_unit_status_banner_thread`; WORLD's
  [[world_menu_unit_status_banner_thread]] describes the frame loop): portrait,
  name, job, level, experience, HP, MP and CT, Brave, Faith and zodiac sign;
- the character status panel, task 0x3b
  (`<overlay>_panel_run_character_status_thread` in ATTACK, DEBUGCHR, EQUIP and
  REQUIRE), and the numeric editor (`<overlay>_editor_run_numeric_thread`);
- the billboards they read ([[battle_menu_unit_status_record_t]],
  [[battle_menu_active_unit_t]], [[world_item_stat_detail_t]]) and the
  palettes the panels switch while they are dimmed
  (`<overlay>_panel_set_primitive_colors`).

### Unit List (BUNIT)

[[bunit_entrypoint]] runs the screen: the grid of units with the status banner,
the help cursor that Select starts, the job and ability pages and a unit
browser. It returns the unit picked with the list menu's second row, or -1 when
Cross closes the screen, and BATTLE moves the map cursor to that unit.

### Equip Change (EQUIP)

[[equip_entrypoint]] runs one frame of the screen: the command menu, the Equip
and Remove modes (with the prompt that removes everything), the slot item
browser with the stat preview of the item under the cursor, and the item-type
order and rearrange modes. Once the screen closes it returns whether the
equipment changed, and the unit and its party record get the new loadout
([[equip_unit_commit_loadout_to_battle_stats]]).

### Job status (JOBSTTS)

[[jobstts_entrypoint]] runs one frame of the screen: the unit's job list (level,
JP, total JP, the next level's JP, mastered) and the chosen job's abilities by
category tab, with what is learned and each ability's costs.

### Options, the AT list and the crystal (OPTION)

- **Game options.** [[option_menu_run_options_thread]] lists the options and
  [[option_menu_run_option_choice_thread]] picks a value; the values are packed
  in `g_main_game_options`, and the last row restores the defaults
  (`g_option_reference_values`).
- **AT list.** [[option_menu_start_at_list_thread]] lists the turn order
  ([[option_menu_init_at_list]]) in the scrolling list thread
  ([[option_menu_run_list_thread]]); a row opens the unit view until BATTLE's
  mini menu state releases it ([[battle_menu_end_at_list_unit_view]]).
- **Crystal or chest.** [[option_menu_start_crystal_pickup_threads]] offers the
  crystal's abilities or a restore of HP and MP, or shows the chest's item, and
  returns the choice in `g_battle_crystal_pickup_action`.
- WORLD.BIN carries a copy (`world_option_*`) for the world menu's Options row
  ([[Menus]]).

### Help (HELPMENU)

[[helpmenu_menu_run_battle_help]] opens the help text for the running menu, or
moves the help cursor over the unit view's fields;
[[helpmenu_menu_run_require_help]] does the same over REQUIRE's panels. The help
binds its own text sections and restores BATTLE's when it closes
([[helpmenu_menu_close]]).

### After a battle, and story displays (REQUIRE)

Event opcode DisplayConditions runs [[require_condition_dispatch]] with a
display id ([[event_display_condition_e]]):

| Id | Display |
|---|---|
| 0 | A special banner ([[require_condition_show_special_banner_thread]]) |
| 1 | The hovered unit's stats for 60 frames ([[require_condition_show_hovered_unit_stats]]) |
| 2 | Congratulations ([[require_text_show_battle_congratulations]]) |
| 3 | The gil reward ([[require_reward_render_gil_ticker]]) |
| 4 | The war trophies ([[require_reward_render_war_trophies]]) |
| 5 | The permanent Brave and Faith changes: each unit keeps a quarter of the battle's change, and Brave 6-15 or Faith 85-94 brings a warning ([[require_party_report_brave_faith_changes]]) |
| 6 | The units with Brave below 6 or Faith of 95 and above leave ([[require_party_remove_low_brave_high_faith_units]]) |
| 7 | The party after the battle: units saved, recruits asked to join, the removal menus when the roster is full ([[require_party_apply_post_battle_unit_changes]]) |
| 8 and up | BONUS.BIN image (id - 8) as a banner ([[require_condition_show_bonus_banner]]) |

Event opcode DarkScreen breaks a BONUS.BIN image into a burst of tiles and
holds a dark flash until RemoveDarkScreen
([[require_bonus_run_dark_screen_burst]]).

### The start of a battle (ATTACK)

- [[attack_scenario_load_conditionals]] advances the story to the next event
  and copies the scenario's map, music, ENTD, squads and condition script into
  the script variables.
- [[attack_gfx_prepare_formation_sprites]] and
  [[attack_gfx_prepare_party_portrait_textures]] upload the roster's formation
  sprites and portraits.
- [[attack_deploy_run_screen]] runs the deployment screen for each squad: the
  player places roster units on the squad's 5x5 tiles, L1 and R1 stepping
  through the roster; then the deployed units' map coordinates are written,
  the deployment is marked finished ([[battle_deployment_mark_finished]]) for
  the battle's map setup, which waits for it, and the map title starts. A
  scenario without a first squad runs [[deployment_skip_thread]]
  instead, which marks it a frame later.
- Event opcode ShowMapTitle shows the map's title graphic
  ([[attack_map_load_title_graphic]], [[attack_map_show_title]]).

### Graphics and names (ETC, SMALL)

- **ETC.** [[etc_graphic_show_async]] loads a graphic of `g_etc_graphics`
  (CHAPTER1-4.BIN, GAMEOVER.BIN, END1-5.BIN) through [[etc_graphic_open]], which
  fades the full-screen ones in and out itself, then shows a chapter title's
  wipe ([[etc_graphic_show_chapter_title]]) or the game over
  ([[etc_graphic_show_game_over]]) by the graphic's texture mode.
- **SMALL.** [[small_text_init_battle_pointers]] binds the roster names and the
  job names over the battle text sections when the map starts.

### Debug editor (DEBUGCHR)

[[debugchr_editor_run_session]] edits a battle unit's level, experience, HP, MP,
CT, Brave, Faith, zodiac sign, Move, Jump and Speed; no menu row opens it in
retail. The same overlay draws the Status screen of a rider or a mount
([[debugchr_editor_run_unit_thread]]).

## Data and structures

| Record | What it holds |
|---|---|
| [[battle_companion_overlay_e]] | The overlays by index |
| `g_battle_menu_companion_executable_lbas`, `_sizes`, `_offsets` | Each overlay's disc sector, read size and offset from 0x801bf000 |
| [[battle_companion_overlay_state_e]] | The overlay the event frame runs each frame |
| [[menu_script_opcode_e]] | The display script commands |
| [[battle_menu_unit_status_record_t]], [[battle_menu_active_unit_t]], [[world_item_stat_detail_t]] | The status panels' billboards and unit fields |
| [[event_display_condition_e]] | The displays of DisplayConditions |
| [[option_menu_type_e]] | OPTION's three menus |
| `g_etc_graphics` ([[etc_graphic_t]]) | ETC's graphics: file, sector, size, VRAM area, texture mode |

## Main flow

1. **Load.** A BATTLE thread reserves heap cells when the overlay needs them,
   names the overlay and waits while the frame loop reads it.
2. **Run.** It calls the overlay's routine, or sets the companion overlay state
   and waits while the event frame runs the overlay each frame.
3. **Return.** The overlay stops its threads; the opener frees the reservation,
   restarts BATTLE's menus and unit panels and fades the screen back in.

## Where to change

| To change | Edit |
|---|---|
| What a screen's windows show | Its display scripts and its `<overlay>_menu_script_*` handlers |
| The status banner | The five `<overlay>_panel_unit_status_banner_thread` copies and WORLD's |
| The game options' values and defaults | [[option_entrypoint]], `g_option_reference_values`, [[option_menu_run_option_choice_thread]] |
| The crystal's choices | [[option_menu_start_crystal_pickup_threads]], and BATTLE's [[battle_unit_generate_crystal_or_treasure]] |
| Brave and Faith after a battle | [[require_party_apply_permanent_brave_faith_changes_and_collect_warnings]], [[require_party_find_low_brave_high_faith_departures]] |
| Who joins after a battle | [[require_party_save_battle_units_and_find_join_candidates]], [[require_party_apply_post_battle_unit_changes]] |
| The war trophies | [[require_reward_collect_war_trophies_and_bonus_money]] |
| The next story event and the scenario setup | [[attack_scenario_load_conditionals]] |
| Deployment | [[attack_deploy_run_screen]], [[attack_deploy_prepare_valid_tiles]] |
| An overlay's size or load place | `g_battle_menu_companion_executable_*` (BATTLE.BIN data): `make build` writes each file's sectors and size from `target/disc.yaml` and each window's size and start from `target/ram.yaml`; in the Developer's Cut a file that grows takes its new sectors and size, unless the code names its sector or length (`include/fft/disc.h`, ATTACK.OUT among them) and that would change |

## Quirks and debts

- REQUIRE reserves 62 heap cells before loading JOBSTTS, which fills 64.
- DEBUGCHR's editor cannot keep job, equipment or ability edits.
- While a banner thread stops, BUNIT's status indicators (and WORLD's) read RAM
  address 0 as a display record; the other overlays' copies have no caller.
- EQUIP never fills the physical accessory evade slot of
  [[equip_unit_data_t]].
- The `*_text_render_decimal_value` copies always place the value as one digit.
- ATTACK fills deployment squad ids 0x35 and 0x36 and clears 0x37 and 0x38,
  although the screen handles four squads.
- Many copied helpers are never referenced in their overlay; QUIRKS.md lists
  them per overlay.

## Functions in scope

![[Battle Screens scope]]
