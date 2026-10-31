# Target quirks

Non-obvious facts about the retail code. They explain source that looks wrong
and mark code that a cleanup must not "fix". Details live in the named file.

## Retail bugs the source reproduces

- `DrawOTagEnv`: the diagnostic string uses literal `&08x` for its second item, so it does not format the environment pointer.
- `StCdInterrupt`: without `CdlModeSize1`, it skips the local disc-position read but still copies the uninitialized four bytes into the ring descriptor.
- `sprintf`: unchecked precision and zero padding can overrun its 512-byte temporary segment buffer.
- `src/lib/psyq/libgpu/FntPrint.c`: unsupported conversions leave the field length undefined; the count limit is checked after writing, and oversized hexadecimal widths can overrun the 512-byte temporary buffer.
- `DecDCTvlc`: the pause path saves its Y predictor at `0x80073f34`, overwriting the first four bytes of the overlapping birthday month-length table. Nothing calls `DecDCTvlcSize`, so the output limit keeps its initial 0xffffff 16-bit values and no frame pauses.
- EFFECT native renderers: `battle_effect_step_emitter_timeline` sets phase 3 when a callback's keyframe window has two frames left, even if init never ran; the destroy path then reads its latch through the null work slot (kernel RAM).
- `src/battle/camera_store_state_to_script_variables.c`: all three zoom
  components are stored to 32-bit script variable `0x20`, so the first two are lost.
- `script_focus_speed` accumulates into an uninitialized `$s6`: the event
  dispatcher leaves `operand_3` there, while camera fusion leaves its key count.
  Those caller values affect the camera duration.
- `src/battle/battle_script_toggle_message_portrait_flip.c`,
  `src/world/world_script_toggle_message_portrait_flip.c`: the record index is never
  assigned; all six passes use whatever `$s0` held. Keep the local
  uninitialized.
- `src/wldcore/wldcore_calendar_advance_days.c`: an event's date advance keeps the
  elapsed days of an active proposition that does not finish within it, so those days
  never count toward it (the daily advance counts each one). It also misses a
  birthday that falls after December 31 inside the advance, and counts the year at
  December 31 where the daily advance counts it on March 21, with no cap at 99
  where the daily advance stops.
- `src/wldcore/wldcore_proposition_determine_success.c`: the preferred-job
  check resolves the party unit once, before its loop, from the scoring loop's
  leftover (past-the-end) index, so it tests that one unit every pass.
- `src/wldcore/wldcore_sound_novel_calculate_completion_percent.c` (no caller): counts 0x1000 bits
  from `g_main_sound_novel_section_bits`, which holds 1280 (40 words), so it also counts the
  Move-Find item flags and most of the poached item quantities stored after it.
- `src/battle/battle_map_calculate_slope_height.c`,
  `src/battle/battle_gfx_calculate_world_height_from_coords.c`: an off-map point
  (a unit staged outside the map, its world coordinates read unsigned) gets no
  tile, and the null tile is read from RAM address 0. The unit shadow, palette
  modulation and per-frame rotation code (`battle_gfx_draw_unit_shadow`,
  `battle_gfx_calculate_sprite_shadow_from_tile_slope`,
  `battle_gfx_apply_unit_palette_modulation`,
  `battle_gfx_update_all_unit_rotation_and_vectors`) do the same for units
  that chapter-end cutscenes walk off the map.
- `src/battle/battle_move_animate_fall_to_target_tile.c`: the event-state
  (`0x34`) path writes the tile-center depth into `real.vy` (not `real.vz`).
  The first frame adds the fall speed to it, which puts the unit below the
  ground, and the next frame snaps it to the ground: an event fall ends on its
  second frame.
- `src/battle/battle_move_update_walking_step_at_tile_edge.c`: adds the
  destination tile's water standing offset to the current edge height instead
  of the destination's, so for a unit that floats on, walks on or moves in
  water the step test sees a water destination lower than its surface, not
  higher: a step up onto deep water can take a plain walk where a climb hop or
  jump is due.
- `src/battle/battle_map_light_state_command.c`: several arms return an
  uninitialized pointer.
- `src/battle/battle_target_clear_panels_on_untargetable_tiles.c`: visits only the lower layer's 256
  panels, so an untargetable upper-layer tile stays in range of a single-tile or DIRECT_TARGETING
  ability (the weapon range's `battle_target_clear_range_on_untargetable_tiles` covers both layers).
- `src/battle/battle_target_mark_unit_panels_by_team.c`: its loop never advances the unit pointer,
  so it marks only the caster's tile; no retail ability sets the flags that call it
  (`ABILITY_SECONDARY_FLAG_1_ALLY_UNIT_PANELS`, `_ENEMY_UNIT_TILES`).
- `src/shared/menu/menu_preview_attack_caster_stats.c`: for an item the
  secondary-data pointer stays uninitialized, yet its `flags_3` is read before
  the item test.
- `src/battle/battle_map_load_mesh_variant.c`: scales the mesh header's color
  palette byte offset by four, placing the palette read past most mesh files.
  Its wait step tests `main_file_is_still_loading() != 0`, the reverse of the
  other loaders, so it applies the palette and frees the buffer while the read
  runs and then never leaves the wait. Its caller runs it only while
  `g_map_refresh_pending` is set, which nothing sets but a self-assignment in
  `battle_script_process_pending_requests`.
- `battle_map_queue_textured_triangles`, `battle_map_queue_textured_quads`
  and their untextured twins pack SXY with the full signed 32-bit MAC1 value; a
  negative X therefore forces the 16-bit screen Y to `0xffff`.
- `src/battle/battle_map_blend_ambient_light_color.c`,
  `src/battle/battle_map_blend_darkness_color.c`: modes `11` and above leave the
  target color uninitialized (no default case).
- `src/battle/battle_ai_load_ability_entry.c`: the CT adjustments test the
  Monster Skill (`0x80`) and Defend (`0x40`) support bits; Short Charge and
  Non-Charge were presumably intended.
- `src/battle/battle_ai_evaluate_status_cancellation.c`: the Blood Suck branch
  tests `BATTLE_AI_DECISION_JUMPING`, built from Jump (bit `0x04` of status
  byte 0; Blood Suck is bit `0x04` of byte 1), so Protect allies skips Blood
  Suck on a target that is not jumping.
- `src/battle/battle_ai_build_monster_skill_tile_mask.c`: the `-1` neighbor
  offsets are loaded unsigned (+255), so only `+x`/`+y` survive, and the level
  loop runs for level 0 only.
- `src/battle/battle_ai_build_targetable_tile_mask.c`: radii 8–15 index past
  the 8-entry template table into the following status weights. Jump Level 8
  can reach this path with a non-arcing weapon.
- `src/battle/battle_ai_evaluate_reflected_target_origins.c`: the resume entry
  skips the reflector pointer reload and depends on the caller's `$s2`.
- `src/battle/battle_ai_evaluate_math_targets.c`: the extra known-ability call
  in the special-ID loop discards its result.
- `src/battle/battle_formula_apply_ability_element.c`: Oil doubles XA against fire, but every
  caller has already stored XA * YA as the HP damage, so Oil never raises fire damage (it is still
  marked for removal). Players know this as Oil having no effect in the PlayStation version.
  The weapon-element path (`battle_formula_apply_weapon_element`) does not test Oil at all.
- `src/battle/battle_action_init_current_ability_strike_data.c`: the range-data
  guard `ability_id >= 0 || ability_id < ABILITY_ID_ITEM_FIRST` is always true
  (`&&` was presumably meant), so item ids read past the table; only the
  default-menu Item skillset `0xa1`, which no job or ENTD unit uses, holds them.
- `src/battle/battle_action_run_pre_formula_setup.c`: both range-data bounds are
  `ABILITY_ID_ITEM_FIRST + 1`, so Potion (`0x170`) reads its range data one record
  past the end of `g_main_ability_range_data`, from the bytes of
  `g_main_item_ability_item_ids`; the Item menu clears it for consumables. The
  other callers bound it at `ABILITY_ID_ITEM_FIRST`.
- `src/world/world_item_sort_id_list.c`,
  `src/event/equip_item_sort_list_by_criteria.c`: the evade sort keys treat
  `0x7a`..`0x8f` as shields and `0x90`..`0xef` as accessories and read
  `g_main_item_shield_data` and `g_main_item_accessory_data` by
  `secondary_data_id`, so throwables (122..127) read far past the 16 shield
  records and headgear and body armor (0..63) read the 32 accessory records and
  on past them; those items sort by unrelated bytes. A fix splits weapons at `ITEM_ID_SHIELD_FIRST` and keys
  headgear and body armor, which carry HP/MP bonuses instead of evade, as 0.
- `src/battle/battle_camera_step_real_coords_toward_target.c`: in the positive
  direction the Y (`vz`) step adds the vector component twice; X and Z add it
  once.
- `world_menu_open_entry_window` receives requested x/y origins but always
  opens at its fixed default origin.
- `src/battle/battle_gfx_build_next_action_result_display.c`: the CT-zero
  display case reads `y_shift` without setting it; only the HP/MP and
  stat-change cases assign it, so the sprite row depends on the leftover value.
- `src/main/main_unit_calculate_random_equipment.c`: throwing and bomb item
  types index past the four equipment-category bytes into the first innate
  status byte; that adjacent byte controls their eligibility.
- `src/shared/menu/menu_text_render_encoded_ids_to_image.c` (BUNIT, CARD,
  EQUIP and JOBSTTS): the encoded-text branch tests
  its glyph counter (`col`) against the line width but never assigns it; the
  stale register stays below the width, so text never wraps.
- `src/battle/battle_unit_init.c`: tints the new unit's palette
  before it clears `mount_state`, so a runtime slot last held by a rider looks up
  a mount that is gone and reads its world coordinates through NULL (kernel RAM).
- `src/battle/battle_menu_display_projected_action_effect.c`: a skipped
  preview leaves `g_battle_menu_preview_target_action` null, and its
  `attack_accuracy` is read (from kernel RAM) before the null test.
- `src/world/world_formation_rebuild_unit_list.c`: the Soldier Office (shop
  0x65) lists only monsters, yet `world_formation_init_menu_state` stages unit
  0 even when that list is empty; with no earlier list since WORLD loaded, the
  pointer is 0, so the "restore HP/MP" stores copy kernel RAM 0x10/0x16 to
  0x0c/0x12.
- `src/world/world_formation_get_unit_palette_index.c`: `job` is assigned only for sprite sets
  0x80 and up. Of the story identities below 0x80, 28 (Delita in chapters 2-4, Zalbag, Dycedarg,
  Larg, Goltana, Elmdor and other enemies) map to a portrait of 0x78 or more, which indexes past the
  20-entry `g_world_monster_formation_sprite_table`, and a result of 0x3e or more reads the unset
  `job`. The characters who join the party map below 0x3e. The monster branch also ORs the job's
  portrait palette in at bit 10 and masks it off with `& 0xFF` at once.
- `src/battle/battle_ai_choose_wait_facing.c`: when the target shares the
  acting unit's tile, `battle_ai_find_direction_of_target` returns
  `BATTLE_AI_DIRECTION_OVERLAP` (4), which indexes `viable_directions[4]`: the
  never-written `past_viable_directions` byte after the array holds whatever
  the stack held and decides whether 4 is returned as the facing.
- `battle_menu_run_scrolling_ability_list_thread` and its WORLD twin
  `world_menu_run_scrolling_ability_list_thread`: `SetSemiTrans(&frame, 1)` passes the
  address of the `frame` pointer, so the semi-transparency bit lands in a stack
  byte past the pointer and the sprite is unchanged.
- Unchecked divisions whose divisor can be zero; the R3000 `div` does not trap
  and leaves quotient -1 (1 for a negative dividend) and the dividend as the
  remainder: `100 / ct` in `menu_ability_get_ct_display_value` and
  `world_ability_get_ct_display_value` (CT 0 displays -1 + 1 = 0);
  `0x20000 / battle_effect_lerp_q8(...)` for the wave steps in
  `effect_update_wave_mesh` (E033, E035, E073, E079, E080, E230, E453, E456);
  and the direction-times-spread products over `scale` in
  `battle_effect_spawn_particle_motion`.
  More generally, GCC 2.6.3 turns a division by a constant into a multiply,
  so nearly every `div`/`divu` in the game divides by a variable: 1192 in
  game code, of which 84 carry the compiler's zero check (`break 7`) and 33
  follow a branch on the divisor. The other 1060, in 109 distinct functions
  (726 in five routines copied across the EFFECT overlays), rely on the
  divisor never being zero or on the R3000 result when it is. A native build
  must give each the R3000 result through a helper or guard it: x86 raises
  SIGFPE and ARM64 returns 0.
- Locals read before any assignment, so the value is whatever the caller left
  in a register: `battle_menu_icon_timed_thread` and `world_menu_icon_timed_thread`
  test `unset` (`$s6`) and, when it is nonnegative, lower `g_thread_current_id`
  around `battle_menu_select_icon_cluts` (`world_menu_select_icon_cluts`), as
  `battle_menu_icon_linked_entry_thread` does for its linked entry;
  `battle_effect_update_file_load` tests `result` (`$a0`) for a load
  state outside 0-3, which nothing sets; `battle_camera_get_input_direction`
  tests `input` (`$a1`) when `mode` or the controller mode is outside 0-2;
  `main_status_set_action_state` leaves `status` unset for an action state
  other than 0 and 5-8 (its `switch` has no default; no caller passes one);
  `battle_map_dispatch_gns_resource` returns a local no path assigns (`$s8`);
  `battle_menu_get_cursor_bob_offset` and `world_menu_get_cursor_bob_offset`
  walk an unset table pointer for a mode other than 0 or 1; `battle_effect_summon_charge_secondary_handler` returns
  `result` (`$a1`) unset when the phase is not 1-3, as does
  `battle_effect_linear_projectile_secondary_handler`;
  `bunit_unit_get_current_stat`, `bunit_unit_get_max_stat` and
  `world_formation_get_unit_stat_by_mode` return `value` (`$a1`) for a stat
  mode outside 0-2; `battle_move_start_unit_step` leaves `facing` (`$a2`, the
  destination tile its callers pass) unset for a direction outside 0-3;
  `battle_map_load_mesh_animation_record` returns `next` (`$a1`) for record type
  0; `world_formation_run_set_ability_menu` indexes the entry ids with
  `cursor` when the list is empty; `world_map_destination_list_thread`
  uses `selected` when no entry matches the current item;
  `helpmenu_menu_run_battle_help` and `world_menu_run_battle_help` read
  `node`, `x` and `y` in the cursor loop when the text thread is already
  running on the first frame, and `navigation` when the unit view mode is
  not 1-4; `battle_menu_init_system_function` and `world_menu_init_system_function`
  pass their action pointer on unset for SCENARIO_CHECK and CLOSE;
  `battle_gfx_init_numeric_sprite_display` leaves
  `display` unset for a section outside 0-2 (its caller passes 0-2);
  `battle_map_update_mesh_part_animations` indexes the instructions with
  `next`, assigned only when a keyframe expires in the same pass, for a part
  whose animation state is set and whose keyframe is 0;
  `battle_map_control_gte_background_color` returns `result` unset for DISABLE
  and RESTORE (every caller issues SET); `open_card_get_info_with_retries`,
  `card_io_get_info_with_retries` and `world_card_poll_info_with_retries`
  return `result` unset for a retry limit below 1 (their callers pass 2), and
  `open_card_poll_info_until_stable` for a nonpositive one (its callers pass 3);
  `battle_sound_stop_weather_sfx` returns an unset `result` no caller reads;
  `battle_effect_next_channel_sound` returns `sound` unset for a channel mode
  above 4.
- `world_card_read_file_chunked` sets `result` to -1 when the last read was
  short and then returns `total`; the test has no effect.
- `helpmenu_menu_run_require_help` and `world_menu_run_require_help` reach
  the shadow quads by indexing past the end of the local `cursor[2]`
  (`quad[2]`, `quads[buffer + 2]`), relying on `shadow[2]` following it on
  the stack; one four-entry array does not reproduce the two stack addresses.
  `entry`, `x` and `y` are assigned only on frames where text
  thread 0x10 is idle and are unset if it is already running on the first
  frame.
- The `menu_text_render_decimal_value` copies (WORLD's and the ATTACK, BUNIT, DEBUGCHR, EQUIP and REQUIRE ones) assign 1 on every rung
  of the minimum-width ladder, so the value is always placed as one digit, and
  read the digit count as the low nibble for alignment and as the low byte for
  the digit loop.
- `etc_graphic_build_game_over_polygons`: `texture_mode` is the constant 1,
  so the 4-bit CLUT arm never runs.
- `open_movie_init_mdec_stream_state` stores the slice width (24) into the
  global `g_open_movie_mdec_stream_state.output_rect.w`, not through its
  `state` parameter; the one caller passes that global.
- `main_unit_calculate_entd_data` copies ENTD bytes 0x21-0x27 with one
  seven-byte loop over the two-byte `ai_target_xy`, so it reads five bytes past
  it (0x23-0x27: the AI flags, the target unit, the two AI targeting flags and
  `ai_initial_enemy_flag`).
- EQUIP never fills the physical accessory evade slot of `equip_unit_data_t`
  (0x48); it stores the accessory's magic evade to 0x52.
- Event script, both interpreters:
  - Ten instruction values (0x14, 0x20, 0x34-0x36, 0x52, 0x67, 0xe1, 0xe2,
    0xe4) have operand lengths in the size table and no handler; the
    scanners step over them and the interpreter ends the script on them. No
    retail script uses them.
  - In BATTLE, AddGhostUnit runs an empty four-pass countdown and then looks
    for a free runtime id among ids 0-20 (`EVENT_UNIT_SLOT_COUNT`), but runtime
    ids are the 16 slot indices (`battle_unit_claim_slot`): with all 16 in use
    it queues absent id 16, and the install later finds no slot and is not
    retried. Its `continue` for no free id needs all 21 ids present, which
    allocation cannot produce.
  - BlockStart, AddUnitStart or CameraFusionStart with no matching End gets
    offset 0 from the finder and the script resumes at byte 1.
  - FocusSpeed and RelativeCamera patch the next instruction's operands
    without checking that it is Camera; Focus scans for the next Camera with
    no End check.
  - UnitAnimRotate tests the animation id's high byte as its flag, never
    reads its last operand byte, and has no absent-unit check, so runtime id
    2000 indexes the rotation table.
  - WaitUnitReady waits on `battle_return_one`, so it never waits.
  - Effect stores its x and y operands to two globals nothing reads.
  - DebugPause prints "PAUSE", yields one frame and sets a 16-bit value nothing reads
    (`script_print_debug_message`); instruction 0xdc stores 13 to
    another. Neither changes the script's course.
  - In WORLD, WalkTo is an empty arm outside a block, Arrow resolves both
    units and never sets the trajectory, and ChangeMap calls the empty
    `world_map_reset_script_light_transform` where BATTLE resets the script
    light transform. WORLD's interpreter never runs (see Other surprises).
  - FadeSound builds its request as `byte 2 << (byte 1 + 16)`;
    `battle_script_process_pending_requests` passes the low half, always 0, as
    the target volume of `main_sound_set_current_music_target`, so it only fades
    the music out, over byte 2 shifted left by byte 1, times four.
- `wldcore_menu_step_location_menu_level` opens the outfitter with shop id 0
  for every town; `world_shop_run_screen` then takes the id from the
  location variable. The fur shop and soldier office pass their own ids.
- `wldcore_proposition_step_result_level` clears the in-progress bit with
  `& 0xFFFD` on an `s32`, which also drops the variable's high half.
- OPENING's runtime flags 8, 0x20000 and 0x80000 (`open_system_runtime_flag_e`: RENDER_RECORDS_HIDDEN,
  CLEAR_FRAME and SOFT_RESET_DISABLED) are tested and never set.
- Suzuki driver: music status bits 0x20 and 0x2000, channel flag 0x10 and
  driver flag 0x2000 are tested and never set; channel flags 0x80 and 0x4000
  and voice-attribute bits 0x7000 are set and never tested;
  `suzuki_smd_set_flags_all_channels` and `suzuki_smd_dispatch_snapshot` have no
  caller.
- CARD save menu state 9 is compared and never stored. EQUIP refuses command
  row 1, so its screen state 3 is never entered. `g_etc_graphics` entries 0, 5
  and 6 all name CHAPTER1.BIN.
- `world_script_reset_event_variables_thread` does `g_main_system_flags += 3`
  on a 32-bit flags value. System flag bits 0x80, 0x400000 and 0x20000000 are
  tested and never set.
- `card_save_run_menu` and `world_card_run_menu_screen` end with
  `set_parameters(2, 0, -1, 0)`: the backdrop thread exits only when its
  third parameter is set, so that call does not stop it. CARD also tests and
  stops slot 14 and suspends slot 15, which nothing in the overlay starts.
- `wldcore_menu_step_map_path_level` plays sound id 0 when the path
  animation ends; that entry has no streams, so it only stops the channel
  pair.
- Scenario conditions 18 and 19 (casualties) compare with the record's 16-bit `id`, not `value` as the war-funds pair does
  (`script_check_scenario_condition` and its WORLD twin).
- `attack_scenario_load_conditionals` fills the deployment squad ids (script
  variables 0x35/0x36) and clears 0x37/0x38, although the deployment screen
  handles four squads.
- 32-bit variables 0xab-0xff of the script variable block are reachable by no variable
  id, yet variable 0xff gates the BATTLE call in
  `main_overlay_call_battle_return_zero`, which itself has no caller.
- Formation menu states 22 and 23 have entries in
  `g_world_formation_menu_handlers` and code paths, and no code stores them, so
  `world_formation_step_unit_portrait_slide` never runs. Menu input mode 1 is tested in four places and
  never stored.
- `battle_menu_wait_thread` and `world_menu_wait_thread` store -2 over the
  Wait entry's row action (7, `BATTLE_MENU_ID_WAIT_YES_NO`) before confirming,
  so the Yes/No entry of the menu table never opens. No code, row action or
  system function opens entries 0x09, 0x0d, 0x16, 0x17 (the only way to 0x18)
  or the four no-act turn menus (0x20, 0x21, 0x32, 0x33), yet the confirm-move
  threads reject Move for 0x21 and 0x33.
- `world_item_build_category_list` and `world_shop_build_item_list` treat
  filters 0 and 1 (the two hand slots) identically.
- The fur shop's "sell" list (`world_shop_run_fur_sell_list_step`) sells
  ordinary inventory at half price; its "buy" list
  (`world_shop_run_fur_buy_list_step`) is the poached stock.
- `main_unit_learn_job_abilities` sets the learned bit twice when the JP
  purchase succeeds: once in the purchase branch and again under
  `if (learned)`.
- `main_unit_apply_level_growth` calls `rand()` and discards the result; the
  call only advances the generator.
- `battle_gfx_load_unit_attack_image_into_evtchr_slot` searches
  `g_battle_gfx_vram_slots[0..1]` for a free load marker and decompresses into
  that slot's image data, but writes the owner, marker and state of the
  caller's slot.
- `require_party_save_battle_units_and_find_join_candidates` searches the formation sprite groups
  for the unit's row, remaps 0x49 to 0 and 0x47 to 0x48, and never uses the
  result.
- `require_overlay_open_jobstts` reserves 0x1f000 bytes (62 cells) at the game heap's base before
  loading JOBSTTS.OUT, but BATTLE's companion overlay 7 reads 0x20000 bytes there
  (`g_battle_menu_companion_executable_sizes`), so the top two cells (0x801fe000-0x801ff000) are
  overwritten without being claimed; JOBSTTS.OUT is 0x1b85c bytes, so they receive the disc bytes past
  its end. `battle_menu_run_jobstts_thread` reserves the full 0x20000.
- `require_overlay_run_helpmenu` reserves 0x10000 bytes (32 cells) at the game heap's base before
  loading HELPMENU.OUT, but BATTLE's companion overlay 3 reads 0x18800 bytes there (the file rounded
  up to a sector), so 17 cells (0x801ef000-0x801f7800) are overwritten without being claimed.
  `battle_menu_run_helpmenu_thread` reserves the whole high window.
- `helpmenu_menu_run_battle_help` and `world_menu_run_battle_help`: the
  `UNIT_VIEW_MODE_HOVERED_UNIT` arm and its navigation table cannot be reached
  (the first arm, which closes the help, takes modes 1 and 2); request
  kinds 4, 0x11, 0x12 and 0x17 call the close routine for an empty id and
  then start the text thread anyway, relying on close not returning.
- `main_heap_free`: the clearing loop stops at the first page byte that differs
  from the allocation id, not at the end of the 64-entry
  `g_main_heap_game_allocator_table`; an allocation that reaches the last page
  reads on into `g_main_gfx_draw_envs` and clears its bytes while they match.
- `battle_map_load_data`: the four texture quarter files load at
  `buffer + 0x8000 * (n + 1)`, while `battle_map_dispatch_gns_resource` uploads
  quarter n from `buffer + 0x8000 * n`; quarter 3 is read past the end of the
  0x20000-byte buffer.
- `battle_camera_toggle_tilt`: from tilt target 3 it starts a decrease toward
  target 4, which `battle_camera_update_tilt` has no branch to finish.
- `battle_reaction_prepare_next`: Counter Tackle performs Dash under the
  Black Magic skillset, the skillset Counter Magic passes.
- `src/wldcore/wldcore_proposition_determine_success.c`: the level score is
  read from the bravery scores (proposition data record 5, three rows of
  five) with a ten-entry stride, so it runs on into the faith scores (record
  6). Record 7, three rows of ten level scores, is fetched by nothing: with
  job list 1 a level 91-99 unit scores 5 where record 7 holds 50.
- `src/battle/battle_ai_add_usable_skillset_abilities.c`: the Faith test reads the AI flags of the
  ability considered last (`g_battle_ai_data_base.considered_ability`), not of the one being added, so
  for an innately Innocent unit (the Steel Giant job) it keeps or drops all of a skillset's default
  abilities at once, by whatever that stale ability was.
- `src/battle/battle_target_validate_weapon_target.c`: the weapon-range flag
  is read from `g_main_ability_range_data[(u8)ability_id]`, so abilities
  0x100 and up use the record of their id's low byte.
- `battle_unit_apply_level_up_down_ability`: a level-1 target of a level-down
  (the Degenerator trap; formula 0x59 misses at level 1) is clamped back to
  level 1 and still loses raw / (growth + 1) of every raw stat.
- `suzuki_sound_set_reverb_mode`: a negative mode is meant to keep the current
  mode, but the stored mode still carries the 0x100 work-area flag that LIBSPU
  strips from its own copy only. It never equals the SPU's mode, so there is
  no early return, and the reloaded value is treated as a mode of 10 or more.
  The 100 music files all hold mode 4.
- `g_suzuki_smd_opcode_lengths`: the 37 unassigned opcodes have length 0, so
  the look-ahead of `suzuki_smd_read_instructions` would never step past one,
  and 0x9D has three parameter bytes there while `suzuki_smd_stop_sfx` consumes
  two. No retail sequence uses any of them.
- `suzuki_smd_set_instrument`: the waveset kept on the Suzuki heap is the
  0xb30-byte header of WAVESET.WD (176 instruments), and most music channels
  end with Instrument 0xFF before End Bar, so entry 255 is read 0x4f0 bytes
  past the copy.
- `world_item_cash_out_excess_inventory` walks ids below `ITEM_ID_END - 1`, so
  Phoenix Downs (0xfd, the last item) over 99 are never cashed out; every
  other item walk runs to `ITEM_ID_END`.
- `battle_move_calculate_pathing` copies 256 terrain costs from row 0 of
  `g_main_terrain_movement_cost_tables` (64 a row) to the scratchpad cost
  block at 0x1f800100, on over the spread state at 0x1f800180. Its flag-0x100
  test (a WalkTo with use_terrain_costs clear) compares the cost-table index,
  not the cost, with 0xff, so blocked surfaces cost 1 too.
  `battle_move_store_unit_movement_to_scratchpad` copies 64 and keeps 0xff.
- `battle_formula_weapon_heal` (formula 0x07, the Healing Staff) applies
  zodiac compatibility twice, once itself and once inside
  `battle_formula_apply_physical_xa_modifiers`, whose Defense
  Up and Protect steps also cut the healing to two thirds.
- `battle_formula_raise_level_by_one_add_status_on_caster` (formula 0x57,
  Please Eat) stores a full HP and MP heal (max-HP damage for an undead
  target) and then overwrites `attack_type` with the level change's:
  `battle_action_apply_target_result` still applies the
  amounts, but `battle_gfx_prepare_post_action_display` shows no number.
- `main_sound_unload_scenario_mus` releases a slot's music record and SMD
  without clearing `g_main_sound_music.state` when that slot is the playing
  one. The shop, formation and both save screens unload their own music this
  way, so the next `main_sound_stop_current_music` hands the released record
  to `suzuki_smd_stop_music`.
- `battle_map_stop_clut_cycle` clears the active byte of slot `handle`,
  but `battle_map_start_clut_cycle` returns slot + 1: it stops the slot
  after the handle's own, and handle 8 clears the byte after the eight slots,
  the first of `g_battle_gfx_cycled_deployed_palettes`.
- `battle_gfx_append_unit_graphics_load_descriptor` writes its record before it
  tests the count, which stops at 16: a seventeenth append lands one past the
  16-entry queue, on the head of `g_main_zodiac_primitives`.
- `battle_effect_remove_secondary`: removing a record from the middle of the
  active chain leaves the record before it with `next_slot_id` naming the freed
  record. When that one is removed while a newer effect is still active, it
  patches the freed record's link (by then a free-chain link) and its real
  successor keeps naming it as the previous record.
- `main_heap_alloc` and `main_heap_alloc_smd` tag a new block one above the
  highest tag in the cells before it, so it can share a tag with the block
  right after it (allocate 62 cells, then 2; free the first; allocate 10, then
  52: the last two blocks are both tag 2). `main_heap_free` clears cells while
  the tag repeats, so freeing the 52-cell block frees the 2-cell one too,
  whose own free then returns 0. No retail sequence is known to do this.
  `main_heap_reserve_at` picks an unused tag but bounds-checks only the starting address, so the requested page range can extend beyond the heap.
- `main_unit_calculate_entd_equipment`: the random body-armor roll tests
  `BATTLE_SUPPORT_SET_1_EQUIP_SHIELD` before asking for `ITEM_TYPE_ARMOR`;
  Equip Armor is tested only for the helmet.
- `battle_unit_store_coordinate_mount_animation_data` and
  `battle_unit_restore_facing_after_attack`: for a mounted pair the second
  `battle_unit_set_tile_position` call names the unit's own battle record again
  (the first function finds its "mount" by the unit's own `unit_id`), so the
  partner's battle stats take no tile or facing here. The second function also
  writes the partner record's facing with no null test.
- `battle_gfx_release_reserved_vram_slot` tests owner bit 0x80
  (`BATTLE_GFX_VRAM_SLOT_OWNER_ATTACK_IMAGE`), which the EVTCHR (0xfe) and free
  (0xff) markers also carry, so the event instruction releases a saved EVTCHR
  slot too.
- `battle_effect_draw_projectile_model`, the Reflect shield: the depth is
  assigned from each corner in turn where the other models sum them, so only
  the last corner counts, and every triangle is linked at ordering-table entry
  0 whatever its depth.
- `battle_map_load_data`: the fourth selection pass does not exclude filler
  rows (type 0x31; 1,533 in the retail GNS files, all with variable 0x22, state
  1 and comparison equal). One whose comparison holds indexes entry 49 of the
  49-entry `g_battle_map_loaded_gns_records` and
  `g_battle_map_pending_gns_records`: it reads
  `g_battle_map_saved_light_matrix` and can overwrite the first 20 bytes of
  `g_battle_map_local_light_matrix`.
- `main_gfx_build_zodiac_primitives` and `menu_build_zodiac_primitives`:
  the round-to-nearest step of the sprite scale adds 1, not `ONE`, to the 4.12
  product before dividing, so it only rounds up a remainder of `ONE` - 1.
- Every copy of the ZODIAC backdrop builder (`main_gfx_build_zodiac_primitives`,
  `world_gfx_build_zodiac_primitives`, `attack_gfx_build_zodiac_primitives` and
  `menu_build_zodiac_primitives` in CARD, DEBUGCHR and REQUIRE) tags the dark quad
  `POLY_F4` but stores its corners at the `POLY_FT4` offsets (0x08, 0x10, 0x18,
  0x20), so the GPU takes the second and fourth corners from the words at 0x0c and
  0x14, which nothing here writes; only (0, 0) and (255, 0) are set. The unlinked
  path, which builds every sprite into one packet, never runs: no code stores
  `link_primitives`, and the WORLD, CARD and ATTACK contexts start at 1.
- `battle_state_handle_open_sp2_files_state` frees its 0x8000-byte buffer when
  `battle_gfx_open_sp2_file` returns 0, which means the read has started, and
  keeps it on 1 (loader busy). The next frame allocates again and has its
  second request refused, so the file arrives in the kept block only because
  the heap hands the freed pages back.
- `battle_effect_set_secondary` (fall dust, splash, level up, trap,
  zodiac poof, death smoke, teleport) builds its init record in a local of
  which `battle_effect_build_secondary_init` sets five fields;
  `battle_effect_init_secondary` copies the rest of the two ten-byte blocks and
  `used_weapon_id` as the stack left them.
- `battle_gfx_draw_unit_shadow` sets bit 0 of `shadow_dirty` before testing it,
  so the shadow quad is recalculated on every call and the dirty bit the
  movement code sets gates nothing.
- Tests that cannot succeed: `battle_unit_init` runs
  `battle_gfx_run_wep_eff_seq_script` for the new unit's sprite blocks only for a
  sheet id below `BATTLE_SPRITESHEET_ID_VELIUS` (60) and at least
  `BATTLE_SPRITESHEET_ID_BOY` (74); `wldcore_proposition_advance_daily_counters`
  caps a waiting day count above 32, and the only code that sets one,
  `wldcore_proposition_step_result_level`, stores 32 or 16; `battle_state_sync_frame` zeroes a
  `g_main_system_min_vsync_interval` of 1 under `wait < g_main_system_min_vsync_interval` with `wait` at least 2;
  `battle_state_run_battle_setup` loops until
  `battle_state_update_deployment_controller_input` returns -1, and it always
  returns 1; the first `switch` of `battle_move_apply_unit_step_velocity`
  repeats the layer test that the statement before it has just made false;
  `battle_turn_get_status_flags` cannot report Sleep, whose
  `g_main_status_effect_data` record carries the freeze-CT flag tested first;
  `battle_unit_set_animation_based_on_mounted_state` tests a depth masked to a
  byte for `< 0` and a nonzero animation for 0; `battle_menu_get_thread_activity`
  tests `i != 3` inside its scan of threads 4-9;
  `battle_camera_rotate_when_unit_tile_not_visible` adjusts a negative
  `direction` after masking it to 0-0xfff; selector 15 of
  `battle_map_set_tile_data_value` retries `battle_map_tile_data_15_stub`,
  which returns 0 at once, up to 1000 times before load exception 0x80.
  `battle_gfx_run_unit_seq_script`, `battle_gfx_run_wep_eff_seq_script` and
  `battle_gfx_load_shp_frame_parts` test sequence and frame pointers
  for -1, which the unpackers replace with the start of the data, and the last
  tests a three-bit part count plus one for `> 8`. The weapon interpreter's own
  bounds (`sequence_id > 0x100`, `frame > 0x200`) admit the index one past each
  table.

- `battle_unit_calculate_height_data`: the seven-byte result preserves an uninitialized elevation high bit; an unavailable unit leaves the other result fields unset.
  A non-Chocobo that shares its tile with another unit (a rider on its mount) takes its
  statuses, movement abilities and terrain from that unit, so it is unavailable when the
  other unit is Dead and never for its own Dead status.
- `battle_map_dispatch_gns_resource` passes the packed lighting colors as a
  `MATRIX` to `battle_map_set_light_matrices`, which also copies its 32-bit translation
  values from the following 16-bit direction components.
- `src/battle/battle_ai_evaluate_self_target_origins.c`: suspension during the
  first origin's target search marks the second-origin stage, so resuming skips
  the moved-origin pass.
- `battle_util_mul_div_s64` and `world_util_mul_div_s64` (hand-written) negate a
  negative divisor with the bit flip alone and add its 1 to the dividend's low
  word, and the carry of negating a negative product goes into the remainder
  instead of the product's high word: `(6264, -400920, -2359)` gives 1065039 where
  signed division gives 1064587. The overflow flag (`g_camera_cross_product_overflow`
  in BATTLE) is set only on the negative-result path; `battle_util_cross_product_q12`
  and `world_util_cross_product_q12` test bits 44-63 of the 64-bit difference for 0
  or -1 and keep bits 12-43, so an overflow into bit 43 escapes. Neither catches
  every signed 32-bit overflow.
- `battle_menu_clear_render_buffer` and `world_menu_clear_render_buffer`
  (hand-written) write `buffer & 3` single bytes before the word loop rather
  than 4 minus that, so a buffer at an odd address reaches the word stores at
  an address 2 mod 4 and faults. Every retail caller passes a word-aligned
  stack array or image global.
- `battle_gfx_step_queued_unit_graphics_load` replaces sheet ids from 0xa0 up with
  Ramza's chapter 1 sheet, but the table has `BATTLE_SPRITESHEET_ID_COUNT` (159)
  sheets, so id 0x9f would read one entry past it;
  `battle_gfx_append_unit_graphics_load_descriptor` already stops at the count, so
  0x9f never reaches the step.
- `battle_effect_spell_charge_secondary_handler` saves the inertia threshold and
  sets its own value but never restores it; the seven other secondary handlers
  that change it restore it.
- `battle_effect_reflect_secondary_handler` never sets the Y angle of the shield
  spin: it clears X and Z and later writes the frame angle to Z again, and
  `battle_effect_draw_projectile_model` passes all three angles to `RotMatrix`.
- `world_menu_icon_clamped_list_thread` (no caller): Up steps the cursor up, but the
  Down test is inverted, so the cursor moves down on every frame Down is not held
  until it reaches `max_row_index`.
- `battle_menu_build_and_upload_window_frame_image` and
  `world_menu_build_and_upload_window_frame_image`: the banded upload
  (`MENU_WINDOW_FRAME_UPLOAD`, an image of 0x3000 bytes or more) starts a band when
  `(i & 0x5f) == 0`, at rows 0x20, 0x80 and 0xa0 rather than every 0x60 rows, and its
  last-band test `i - height >= 0x60` is never true. After the band at row 0x20 the
  rows are written before the start of the buffer. No retail window is known to be
  that large.
- `main_unit_calculate_random_equipment`: an item with a higher required level than
  any before it restarts the candidate list, but items scanned after it with a lower
  level still join, so the pick is not limited to the highest level reached.
- `battle_status_queue_runtime_graphics_flag_change`: removing Crystal falls into the
  Chicken case, so it queues the Chicken display bit for removal and leaves the
  Crystal bit set.
- `battle_formula_apply_zodiac_compatibility` returns early when either unit is
  Serpentarius, before the floor of 1 that every other sign pair reaches, so an XA
  already cut to 0 stays 0 and the hit deals 0 damage.
- `open_movie_update_stream_frame` restarts a stream whose ring has fewer than 16
  free sectors through `open_movie_start_cd_stream_read`, which reads with mode
  0x1a0; the first read in `open_movie_start_stream` uses 0x1e0, so the restart drops
  `CdlModeRT` (0x40, XA-ADPCM playback).
- `battle_action_queue_unit_effect_messages` tests the 16-record capacity of
  `g_battle_action_post_effect_msgs` once, before it walks the unit's 11 reportable
  outcomes: with 15 records queued it can append ten more past the array, over
  `g_battle_gfx_compressed_nibble_phase` and the globals after it.
- `battle_ai_evaluate_self_target_origins` and
  `battle_ai_evaluate_reflected_target_origins` run
  `battle_ai_can_use_ability_at_coords` on the unit's current tile in both passes, so
  the starting tile also decides the move-then-act pass, which casts from the
  destination; `battle_ai_evaluate_before_and_after_move` tests the destination there.
- `battle_target_select_random_tile_for_random_fire_abilities` has no test for an
  empty candidate list, and `battle_target_mark_hit_tiles` calls it for any result but
  -1: with no candidate it adds 0x100 to the stale `mark` of
  `g_battle_target_panels[0]` and targets that upper-level tile. Whether a retail
  action gets there is unknown.
- `battle_effect_free_state_record` follows each freed work record's `next`, which
  `battle_effect_free_work_record` has just pointed at the old free-chain head. An
  empty list ends cleanly; with an empty free chain the walk stops after the first
  record and leaves the rest attached (clean only for a one-record list); otherwise
  it frees free-chain records again and then loops between two records forever.
- `battle_map_light_matrix_command`: the blend and delta steps (0x67, 0x69) start
  each translation from the start matrix's `m[i][3]` (the next row's first element,
  or the padding after `m[2][2]`) instead of `t[i]`. Nothing issues the start
  commands 0x66 and 0x68 that queue them, so they never run.
- `battle_camera_execute_position_command`, target-average source: it sums the world
  coordinates of the unit targets only but divides by the whole target count, so
  tile targets pull the average toward the map origin. The average is stored only
  with an offset, which all four callers pass.
- `battle_unit_place_in_new_location` (WarpUnit, and
  `battle_script_warp_unit_display_to_paired_unit` with its WORLD twin) passes
  `unit->facing`, the quadrant shifted left 10, as the `u8` facing of
  `battle_unit_set_tile_position`, where its other callers divide by 1024. The low
  byte is 0, so a warped unit's battle stats face south while its sprite faces as
  asked; facing evade reads the stats.
- `battle_move_animate_jump_start` and `battle_move_animate_jump_rise_with_sfx` size
  the rise as 0x120 minus the ground's (negative-up) y, the altitude of a point 0x120
  above the ground, but cover it from the unit's current height: a unit on ground h
  units up ends 0x120 + h above it. The second function's early exit compares the
  same value with the unit's absolute altitude.
- `battle_state_handle_confirm_action_state` stores answer 7 (a unit on the cursor
  tile) as the action's `targeting_type`, which otherwise holds
  `BATTLE_TARGETING_TYPE_TILE` (5) or `BATTLE_TARGETING_TYPE_UNIT` (6): the readers
  that test for UNIT then aim at the stored tile, and
  `battle_unit_face_towards_action_target` and
  `battle_target_move_cursor_to_action_target`, which test for TILE, at the unit. The
  7 comes from the fallback of `battle_menu_resolve_selection` for a menu outside its
  table; whether that happens here is unknown.
- `battle_move_calculate_pathing` (WalkTo): the destination filter repeats the water
  tests of `battle_move_set_reachable_tile_flags`, but refuses lava only when
  `move_type` is `BATTLE_MOVEMENT_CLASS_FLY`, where that function refuses it unless
  `can_pass_lava` is set. The walker always uses `BATTLE_MOVEMENT_CLASS_NORMAL`, so
  lava is never refused as a WalkTo destination.
- `script_block_start_thread` and `world_script_block_start_thread`: the block's
  WalkToAnim clears `g_unit_was_moving_latch[runtime_id]` outside its absent-unit
  test, so an absent unit (2000) zeroes the 16-bit value 4000 bytes on, the `w` of
  `g_menu_window_buffers[5].sprites[3]` in BATTLE and WORLD. The interpreters' own
  WalkToAnim keeps the store inside the test.
- `card_file_write_buffer`, `world_card_write_buffer_to_file` and
  `world_card_read_file_chunked` seek before each of their 40 attempts by the bytes
  transferred so far from the current position (`lseek` origin 1), not from the
  start: after a short first transfer of n bytes the next attempt works at file
  offset 2n with the data from offset n.
- `equip_item_get_ranking_value` and `world_item_get_ranking_value`: item 0 is a
  weapon to `equip_item_get_menu_category` and `world_item_get_menu_category`, but
  the weapon branch excludes it, so it falls into the helmet branch with index -144
  and reads the range byte of weapon record 96 as its HP bonus. Both sorts skip item
  0 first.
- `bunit_unit_build_list_index_map` maps a mount to its rider's list index
  (`bunit_unit_find_index_by_battle_id`), not the rider's grid cell, and the readers
  copy the entry straight into `g_bunit_unit_grid_selected_cell`, so browsing onto
  such a mount with L1/R1 puts the grid cursor on a later unit's cell (or past the
  last).
- `bunit_menu_script_draw_plain_number`, `equip_menu_script_draw_plain_number` and
  `world_menu_script_draw_plain_number` count their blank cells down to -1 in an `s16`
  with no lower bound: a value with more digits than the field starts below -1 and
  queues a blank quad about 65,535 times. Whether any retail field is too narrow is
  unknown.
- `world_gfx_update_fade_in_tile` does not clamp its level when the fade-in ends,
  unlike `world_gfx_update_fade_out_tile`: at a frame step of 2 the level goes from 16
  to -16, and the last subtractive tile takes the low byte, 240, darkening the screen
  for one frame. Whether a fade-in runs at step 2 is unknown.
- `wldcore_sound_novel_stop_music` clears `tracks[slot]`, but
  `wldcore_sound_change_scenario_music` and the audio queue keep slot n's track in
  `tracks[n - 1]`. The novel always uses slot 2, so the store hits `current_slot`
  (zeroed next) and `tracks[1]` keeps the unloaded id: a later queued play of that
  track switches to the empty slot without loading it.
- `wldcore_gfx_draw_animated_display_object`: a part with its own palette gets CLUT x
  `(clut_x + (word & 0xff0000)) >> 12`, shifting the cel's CLUT x origin out with the
  palette field; OPEN's `open_gfx_draw_animated_render_record` adds the origin after
  the shift. Only a cel with a nonzero CLUT x would show it.
- `wldcore_unit_get_ramza_roster_index` returns 16 (`PARTY_MEMBER_SLOT_COUNT`) when no
  formation slot holds Ramza, and the four MCBrave/MCFaith opcodes
  (`wldcore_script_check_ramza_brave_at_least`, `wldcore_script_check_ramza_brave_at_most`,
  `wldcore_script_check_ramza_faith_at_least`, `wldcore_script_check_ramza_faith_at_most`)
  use it unchecked, so they test roster slot 16.
- `main_sound_start_music_load_into_free_slot` allocates the SMD block before it
  requests the read; when `main_file_request_read_bytes` refuses (a read already in
  flight) it reports `MAIN_SYSTEM_ERROR_SMD_LOAD`, whose hook is empty, and returns 0
  without freeing the block. `main_sound_start_music_load_into_slot` (no caller) does
  the same.
- `battle_menu_alloc_memory` and `world_menu_alloc_memory` test the end of the arena
  before the size, so no allocation includes the last page. They mark pages before
  claiming one of the 16 records and leave them marked with no owner when all 16 are
  taken. Whether a normal game exhausts the records is unknown.
## Calls that disagree with the callee

Where callers pass arguments a callee never reads, the callee declares them as
`unused_` parameters and the calls are plain. The calls below cannot be
written that way.

- `StCdInterrupt`: emulated header/payload copies pass a fourth completion argument to the three-argument `mem2mem` helper, which ignores it.
- BATTLE's and WORLD's `g_thread_call_target` are the
  main-stack dispatch slots for callees with different signatures;
  assignments erase their function types.
- `world_text_bind_section_data` has no return statement;
  `world_card_build_save_slot_description` uses the job-name pointer it leaves in
  `$v0`, which a native build must return.
- `src/world/world_menu_resize_parent_entry_to_digits.c`: calls
  `text_count_decimal_digits` with no argument; the callee reads a stale
  `$a0`. Nothing calls the function itself.
- `src/battle/battle_move_animate_fall_to_target_tile.c` passes a coordinate
  buffer to the one-argument
  `battle_gfx_calculate_world_height_from_world_coords`, as it does to
  `battle_gfx_calculate_world_height_with_caller_coords`;
  `battle_move_animate_jump_arc_to_own_tile.c` does not.
- `g_suzuki_smd_modulator_waveforms` entries 8-15 call the void
  `suzuki_smd_modulator_deactivate`; `suzuki_smd_update_modulators` adds the
  leftover `$v0` (the cleared modulator flags) to the channel output.
- `main_sound_update_tunes` passes `current_music` to `suzuki_smd_is_music_playing`
  even when no scenario music is loaded; the null record reads the status
  16-bit value at address `0x10`.
- `src/shared/menu/menu_animate_window_quad_crop.c` passes an uninitialized
  source (a stale `$s5`) to `menu_zoom_cursor_frame`, which never reads it.
- `battle_map_step_setup_sequence` and
  `battle_map_step_init_sequence` call `main_sound_stop_sfx` without its sound
  id: step 0 passes `map_id`, still in `$a0`; step 11 passes whatever
  `main_sound_unload_current_scenario_music` left there.
- `battle_unit_get_world_coords_by_runtime_id` returns NULL for a missing
  unit, but its callers test for -1 and read through it. A teleport-out with
  removal (Wiegraf at the Fovoham windmill) fades the unit out while the
  teleport particles still read its world coordinates, from kernel RAM at address 0.
- `battle_map_get_tile_data_ptr_from_battle_id` and `battle_map_get_unit_tile_by_runtime_id` read the
  unit's map position without checking the lookup, so a unit with no runtime
  record (an arc-trajectory target in a Chapter 2 battle) is read from kernel
  RAM at address 0.
- `battle_menu_build_unit_portrait_poly` reads the speaker's battle record
  through `battle_unit_get_stats_from_battle_id`, which returns NULL for a
  speaker who is not on the field; the portrait test then reads kernel RAM.
  `menu_copy_unit_data_to_status_billboard` does the same while
  counting units, for a runtime record with no battle unit (the final battle).
  `script_process_inflict_status_commands` (BATTLE and WORLD) skips a battle id of
  2000, which `battle_unit_get_battle_id_by_runtime_id` never returns (it gives the
  `u8` battle id or -1); for a runtime unit with no battle stats the lookup gives
  NULL, and the Float and Jump tests read kernel RAM.
- `world_item_sort_id_list`: it counts each unit's five equipment ids into a
  256-byte local array, but the ids are 16-bit values and an item tried on in the
  shop's fitting room carries flag bits above the low byte, so sorting the
  list then increments a byte far past the array.
- `script_resolve_runtime_id`: for the active-turn selector 0x69 with no unit
  holding the turn, it falls back to ENTD units 1 and 2 but tests the finder's
  battle index, not the loop counter, against 3, so a unit found in battle slot 3
  counts as missing. With neither on the field it tests the Jump status of
  whatever the finder returned (NULL when there is none: kernel RAM at address
  0x58), and if that passes the lookup goes on with ENTD unit 3. During Game Over
  event 0x194 with no Ramza in the 21 records it returns -1, not
  `EVENT_RUNTIME_ID_NONE`, which is what its callers test for.
- `src/battle/battle_unit_init.c`: stores the spritesheet-slot
  claim (0xffff when all nine slots are taken) in the u8
  `spritesheet_vram_slot` and tests the stored 0xff against 0xffff, so the
  failure path never runs and a tenth sheet loads into slot 255.
- `src/battle/battle_ai_load_ability_entry.c`: a Jump ability's CT estimate
  for the acting unit reads `acting_unit`, which is NULL during the AI
  workspace setup at battle start (`acting_unit_id` is 0), so slot 0's Jump
  divides kernel-RAM bytes.
- `src/event/bunit_entrypoint.c` passes a second argument, 0, to the
  one-parameter `bunit_gfx_draw_falloff_shaded_rows`; its other caller passes
  one.
- `battle_menu_init_system_function` returns `$v0` as the void
  `battle_text_select_ability_announcement` left it.
- `battle_effect_spawn_emitter_particles` has an `s32` result it never sets
  and no caller reads.
- `battle_thread_resolve_id`, `battle_thread_resolve_id_after_current`,
  `world_thread_resolve_id` and `world_thread_resolve_id_after_current` return
  the leftover `$v0` of the void `thread_exit_current` (BATTLE's or
  WORLD's copy) when no thread slot is free (the WORLD source
  falls off its end). The caller has exited by then, but the scheduler
  resumes slot 0 without testing it, so a main-loop caller gets that value
  on the next frame.
- `battle_ai_calculate_clockticks_until_death_counter_expires` is defined
  with three parameters and both callers pass one; the other two are scratch
  it assigns before reading.
- `battle_map_calculate_slope_height` masks its level to a byte; two callers
  pass the 16-bit `action_target_elevation` unmasked.
- WLDCORE passes its 8-byte records by value:
  `RECT` to the three render-record builders and
  `open_point32_t` to `wldcore_window_build_yes_no_panel`.
- `script_filter_unit_id_by_mode` and
  `world_script_filter_unit_id_by_mode` read `in_id` as a 16-bit value; most
  callers pass the address of an `int` loop counter (BATTLE through a
  `(u16*)` cast), so the id is that counter's low half.
- `battle_move_calculate_walkto_pathing` has no return statement;
  `battle_move_start_unit_walk_to` uses the path that `battle_move_calculate_pathing` leaves in
  `$v0`, which a native build must return.
- `battle_map_load_gns_and_move_find_items` has no return statement for a map
  whose GNS sector is 0 (map ids 120-124, 126 and 127 in the retail table);
  `battle_map_load_data` compares the leftover `$v0` of the BIOS `bzero` with
  `g_battle_map_gns_records`. A native build must return a value there: the
  BIOS `bzero` returns its destination, that table (psx-spx), so the test
  passes and the load goes on as if the read had started. When the result is
  not the table (also when the read could not start),
  `battle_map_load_data` returns 0 with its 0x20000-byte load buffer still
  allocated.
- `src/battle/battle_reaction_try_distribute.c` passes the unit twice to the one-parameter
  `battle_reaction_fails_brave_roll` through a function-pointer cast; the second argument is ignored.
- `src/battle/battle_formula_calculate_critical_hit.c` calls the argument-less
  `battle_formula_calculate_knockback` through a cast with XA's address in `$a0`; the callee ignores it.
- `src/battle/battle_formula_apply_ability_element.c` calls the argument-less
  `battle_formula_nullify_action` through a cast with `mount_info` in `$a0`; the callee ignores it.
- `g_battle_effect_secondary_handlers` entries 0x07, 0x0a, 0x0b, 0x0e and 0x10
  (`battle_effect_unused_07_secondary_handler` and the four after it) set no result, although `battle_effect_update_secondary_effects`
  ends an effect on a handler result of 0. No record of
  `g_battle_effect_secondary_animations` names them.
- `menu_ability_is_non_action_learned` passes one 32-bit pointer slot (0x801f0050, named
  both `g_menu_selected_ability_data` and
  `g_menu_selected_ability_secondary_data`) for both outputs of
  `main_ability_calculate_pointers_and_type`, so the ability data pointer is
  overwritten by the secondary data pointer. Nothing reads the lost pointer: MP and
  CT use the secondary one (`menu_ability_get_mp_cost_display_value`,
  `menu_ability_get_ct_display_value`) and JP reads `g_main_ability_data` directly.
- Calls that omit arguments the callee reads take them from whatever the
  registers hold; a native build must pass them explicitly:
  - the three `battle_camera_get_input_direction` callers pass only `mode`;
    `input` is the leftover `$a1` when `g_battle_cursor_input_mode` is not 0/1;
  - `battle_camera_handle_tilt_input` passes nothing to
    `battle_camera_toggle_tilt`, whose tilt-target-1 path plays its sound
    with the leftover `$a1`;
  - the five `battle_effect_update_file_load` callers pass nothing, so the states
    it does not handle return the leftover `$a0`;
  - `menu_build_item_icon_source` (one source in ATTACK, BUNIT, DEBUGCHR,
    EQUIP and REQUIRE) calls
    `battle_gfx_set_item_icon_sprite` without the item id (`$a1`);
  - `battle_status_resolve_unit_changes_in_preview` (referenced nowhere on the disc) calls
    `battle_status_resolve_unit_changes` without `unit_id`/`removal_only`;
  - `world_script_is_deployment_running` calls `world_script_run_event_frame`
    without `ot`/`buttons`.
- These calls rely on the caller's own incoming registers instead, which
  hold the right values: `equip_text_start_character_thread_if_idle` (thread id in `$a0`)
  for `thread_is_running`, `battle_unit_start_post_attack_animation_display`
  (both arguments) for `battle_unit_set_target_animation_from_attack_type`, and
  the EQUIP selection wrappers (`input_mask` in `$a2`) for
  `menu_update_wrapped_horizontal_selection` and its vertical twin.

- `battle_unit_animate_squeeze_out` and
  `battle_unit_animate_teleport_to_destination` fall through without an
  `s32` result; the animation dispatcher ignores their results.
- `card_input_update_event_state` passes six 32-bit values starting at `g_card_input_event`
  to the event input loader; CARD's controller pointer at +4 is copied as held
  buttons, and the remaining 32-bit input values are shifted by one field.
- `world_input_update_menu_state` and `world_script_run_event_frame` pass six 32-bit values
  starting at `g_script_event_input` to the event input loader. The
  next five 32-bit values include unrelated status-label data; the corresponding held,
  previous and repeat globals reside elsewhere.
- `battle_move_start_next_post_move_event` passes the mover's `map_x`, `map_z` (the
  layer) and `map_y` to `battle_ai_record_sprung_trap`, which stores x, y and level,
  so the AI's sprung-trap tile holds the layer as y and the row as level.
- `battle_menu_collect_monster_skill_abilities` returns 0 from its early exits but
  falls off its end after building a list, and writes no terminator after it; both
  ability-list builders (`battle_menu_build_ability_list` and its WORLD copy) ignore
  the result and have filled the id column with 0xffff beforehand.
- `battle_effect_get_world_coords_by_battle_id` (no caller) passes the unit's runtime
  id to `battle_effect_resolve_target_world_coords`, which takes an index into the
  effect's target list.
## Declaration leads

- The unit status record (`battle_menu_unit_status_record_t`, `include/fft/menu.h`)
  is laid out again by the heads of `equip_unit_data_t` and `bunit_unit_t`,
  which hold other fields at 0x1e and 0x20, and by HELPMENU's raw
  `g_helpmenu_active_banner`.
- `jobstts_gfx_context_t` (`include/fft/event_jobstts.h`, 244 bytes) follows the
  pool layout of `bunit_menu_gfx_context_t` (`include/fft/gfx.h`, 236 bytes, shared
  by BUNIT, EQUIP and WORLD) through `0x5c` under other pool names, but has no
  draw-mode pool pointer at `0x60`.
- WLDCORE's `g_wldcore_calendar_zodiac_start_dates` (`include/fft/wldcore.h`) holds `calendar_date_t` records; OPEN's `g_open_birthday_zodiac_start_dates` is the same `{month, day}` table as a flat `[24]` of bytes, because the `[12][2]` spelling changes its
  reader's code.
- The movement step byte is one bit-field record (`battle_move_step_t`) in
  three places: the path elements (`battle_unit_t.path`),
  `battle_unit_t.current_path_step` and the working global
  `g_battle_move_step_value`.
- `g_main_item_location_flags` (`0x80059414`): only the first 64 bytes (512
  Move-Find bits) are proven; the next 64-byte block is saved with it but has no
  known meaning.
- `gfx_init_image_loading` takes a `void*` primitive that is either a
  `POLY_FT4` or a `SPRT` (code `0x64`, whose `w`/`h` overlap `x1`/`y1`). A
  typed version needs a primitive union or separate SPRT/POLY_FT4 paths.
- `void*` parameters whose callers pass another view of the data:
  `battle_gfx_set_draw_mode_from_origin` (a window record for its `mode0`
  `DR_MODE`), `battle_map_store_selected_tile_coordinates` (an `s16[3]` as an
  `SVECTOR`; only three 16-bit values are written), `open_gfx_load_opntex_into_frame_buffer`
  (OPNTEX bytes read as words), `world_menu_build_sprite_page` and its BATTLE
  twin (a `RECT` as an image location), `world_menu_init_and_load_scrollable_list`
  (menu scripts as a list record) and
  `world_menu_run_script_with_palette_mode` (display scripts declared as
  `u8[]`, `s16[]` or `world_menu_window_command_t`).
- `g_wldcore_sound_quad_command_bytes` (`0x8009ec18`) starts one row before
  the sound rows, which begin at `0x8009ec1c` for sound id 1: its row 0 is the
  eighth 32-bit entry of `g_wldcore_tutorial_event_ids`, whose eight entries
  `wldcore_list_handle_battle_tutorials_input` indexes.
- `g_battle_effect_ending_secondary_function_id` (`0x801b8530`) is
  `g_battle_effect_secondary_animations[BATTLE_EFFECT_SECONDARY_ANIMATION_ENDING].function_id`
  under a second name; the table's own symbol covers the 21 records before
  it.
- `g_open_file_openbk_end_sector_offsets` (`0x80073f5c`) is
  `g_open_file_openbk_start_sector_offsets + 1`: eight sector offsets of
  OPNBK.BIN under two names, each image ending where the next starts.
- `g_open_prologue_crossfade_progress` (`0x800854b5`) is
  `g_open_prologue_crossfade_row_flags[1]`;
  `open_prologue_prepare_crossfade` clears it through the array.

## Duplicated code

Twins stay separate functions because each lives in its own module or
translation unit. Share their types and constants through headers.

- EFFECT: 32 groups of byte-identical routines across 110 native files. E005's
  first routine appears in 17 overlays, and E450/E480 are identical.
  E336/E461, E230/E481 and E015/E047 differ only in immediates.
- BATTLE/WORLD pairs: scenario conditionals
  (`src/shared/script/script_run_scenario_conditions.c`, one source for
  both), action-slot restrictions, the
  portrait-flip toggle (instruction `0x37`), staged status, selected-tile data,
  and the event interpreters.
- `src/wldcore/wldcore_menu_push_fact_detail_level.c` and
  `src/wldcore/wldcore_menu_push_story_event_text_level.c` are one routine at
  menu levels `0x2f` and `0x1e`; the second reserves 8 unused stack bytes.
- `battle_map_blend_ambient_light_color.c` and
  `battle_map_blend_darkness_color.c` are one 1,032-byte function over
  different records.
- `world_gfx_init_scaled_draw_area_packets{,_2,_3,_4}.c` differ only in the
  eight bytes they install as the second `RECT`.
- Routines compiled twice into one module, identical apart from the addresses
  they jump to, call and load: `script_run_next_event` and
  `script_run_next_event_unused` (and the WORLD pair);
  `battle_unit_add_event_offset_by_runtime_id_unused` and
  `battle_unit_add_event_offset_by_runtime_id`;
  `battle_move_set_float_step_delta_center_to_edge` and
  `battle_move_set_float_step_delta_edge_to_center`, which repeat the
  `battle_move_set_unit_step_delta_*` pair;
  `text_advance_cursor_with_backreferences` and
  `menu_text_advance_cursor_with_backreferences` (and the WORLD pair);
  `world_menu_set_cursor_position` and `world_menu_init_cursor_position`;
  WORLD's `world_option_gfx_*` and `world_screen_gfx_*` scaled-draw-area
  routines and its two thin-window-frame threads. The two copies named unused
  have no caller.
- Overlays share load addresses, so an address match is not an identity
  match. Compare bytes: REQUIRE and EQUIP both have code at `0x801c2dcc`, and
  the two routines differ.

- Overlays call both the resident `thread_is_running` and local
  `menu_thread_is_running` entries. Keep those callees distinct even though
  their running-flag reads are identical.
- `attack_editor_run_numeric_thread` and `require_editor_run_numeric_thread` point
  both slot branches at `g_attack_editor_numeric_state` and
  `g_require_editor_numeric_state`, where the BUNIT, DEBUGCHR, EQUIP and WORLD
  copies give the preview slot a second state; both overlays start the thread only
  in `THREAD_SCREEN_SLOT_NUMERIC_EDITOR`, so the shared second branch never runs.
## Other surprises

- `main_boot_run_startup` installs the empty `main_file_handle_cd_ready_callback` and
  `main_file_handle_cd_read_callback` as LIBCD callbacks, and `main_file_init_cd`'s `CdInit` replaces
  them with LIBCD's defaults right after, so they never run.
- `main_gfx_load_efc_fnt`: patches the low immediate byte of `FntLoad` at `0x80022f5c`, changing its font CLUT placement from `y + 128` to `y + 127`.
- `StCdInterrupt`: tracks frame numbers through the header's low `u16`, while DMA completion and OPEN retain the full `u32` frame number.
- `psyq_gte_apply_rotation_ir`: writes saturated IR1–IR3 32-bit output values and returns its unchanged third argument.
- `sprintf`: unsupported conversions terminate output, `%#p` prefixes `0p`, hexadecimal alternate form prefixes zero, and `%#.0o` with zero emits no digit.
- JOBSTTS.OUT offset `0x0` and BUNIT.OUT offset `0x200` hold the literal `"%d"`
  (`g_jobstts_text_decimal_format`, `g_bunit_text_decimal_format`) used by `src/event/jobstts_menu_script_draw_formatted_number.c` and
  `src/event/bunit_menu_script_draw_plain_number.c`.
- The 32-bit game-options value packs its last five fields out of array order
  (`include/fft/menu.h`).
- CallFunction (`script_execute_event.c`, `world_script_execute_event.c`)
  tests selectors in sequence against one operand that the arms modify. So
  `0x06` also runs the `0x0f` warp, and `0x0e` can fall into later arms.
- `src/battle/battle_reaction_check_when_targeted.c`: formula 7
  suppresses reactions. Counter Magic (`0x1b3`) is tested last, after byte
  `+0x8d`, so Counter (`0x1ba`) wins.
- `g_main_debug_display_enabled` is only ever cleared by retail code; the
  add-unit threads and unit-summary panels print debug output when it is set.
- `src/wldcore/wldcore_input_check_repeating_directional.c`: the arms test
  UP, DOWN, RIGHT, LEFT, R1, L1, but the counters sit in UP, DOWN, LEFT, RIGHT,
  R1, L1 address order. The early-out clears only the four direction counters.
- `src/wldcore/wldcore_proposition_select_report_speaker.c`: with more than one
  unit sent and a speaker mode other than 1-4, the returned `selected` is never set
  (every retail row uses mode 4, the lowest Faith).
- `src/wldcore/wldcore_map_roll_random_encounter.c`: encounter masks count from
  the high bit, so the chosen ENTD is `entd_ids[7 - bit]`.
- `wldcore_map_step_route_travel` stores the 0-based arrival location in the map
  cursor level's one-based `hovered_location` (`location_transition.location_id`,
  +0x0c); the cursor's next snap probe overwrites it before the label reads it. That
  level's `pending_mode` is only ever 0, so its unblock-and-leave branch never runs.
- `src/battle/battle_action_finalize_draw_out_katana_result.c`: when the
  katana breaks, the strike work's `can_earn_experience` is copied onto itself.
- `src/shared/menu/menu_run_icon_selection_loop.c` and its WORLD twin: the
  loop clears both records' `+0x78` words when they hold 0 and 2, but only the
  record builders write that word (always 0), so the reset never fires.
- `src/wldcore/wldcore_menu_step_treasure_detail_level.c` and its unexplored-land
  twin: phase 1 waits on the render record named by `+0x08` (`saved_record_index`),
  which neither push sets, so it tests whatever the previous level in that stack
  slot left there. Phase 3 waits on `render_index` instead.
- `src/lib/psyq/libc/memmove.c`: the overlap-safe copy returns the original destination on its backward path and the advanced destination on its forward path.
- `StartRCnt` and `StopRCnt` index the IRQ-mask table before validating a counter: selector 3 changes the VBlank mask even though `StartRCnt` returns zero; `StopRCnt` always returns one. Larger indices can read past the four-entry table.
- `SpuGetVoiceEnvelopeAttr`: the public `s32*` key-status output is written with a 16-bit store; its upper half remains unchanged.
- `InitCARD` and `StartCARD`: this linked version calls `ChangeClearPAD(0)`, while later SDK descriptions use one for startup; retain the retail value.
- `_patch_card2`: exchanges five resident/BIOS 32-bit instructions, so calling it again reverses the exchange rather than repeating an idempotent patch.
- `SetDrawLoad`: initializes the upload header and rectangle but leaves pixel payload storage to its caller.
- `suzuki_sound_get_largest_free_block`: reports the rounded gap including space needed for a new block header, rather than a directly usable payload size.
- `battle_gfx_load_spritesheet_into_vram_slot`: its fixed 0x30d4-byte copy from the variable compressed SPR tail at 0x9200 extends beyond the file-sized allocation in 153 of the 154 nonempty unit sprite-table entries and includes retained heap bytes.
- `battle_gfx_update_and_animate_unit_wep_eff`: when neither EVTCHR slot is free, one path tests `j` before setting it.
- `g_battle_effect_disc_entries` (`0x801b53e8`) stores whole-sector sizes, and entry 0 repeats entry 1: effect id 0 loads E001.BIN, so E000.BIN's code is never loaded through the table.
- `battle_effect_update_stage`: its RESTART_SCRIPT phase (6) restarts the effect script after `battle_effect_reset_frame_data_pointer` and `battle_effect_reset_animation_sequences_pointer`, but nothing sets `g_battle_effect_playback_phase` to 6, and the two start pointers those routines restore are never written (0 in BATTLE.BIN). Nothing stores phase 5 either; it has no case and no default, so it would return `result` unset.
- `src/world/world_gfx_load_wldface_to_frame_buffer.c`: the WLDFACE rectangles (`g_world_wldface_vram_rects`) use x = 1664–1856; the GPU masks VRAM transfer coordinates, so the pages land at x = 640–832.
- `suzuki_sound_init`: `suzuki_sound_init_sfx_music` reaches `suzuki_smd_insert_music`, which disables and re-enables `g_suzuki_sound_root_counter_2_event` before the initializer opens that event: handle 0 at boot, the handle `suzuki_sound_quit` closed on a soft reset.
- `src/event/equip_menu_load_images_and_reset_lists.c` and `bunit_gfx_init_vram_and_start_fade.c`: store 16-color CLUTs into the 4-entry `buf1`; they run on into `buf2`, which the frame places directly after it (the source reads as two arrays, the stack holds one).
- EFFECT scripts never run seven state handlers: E000's script calls only `effect_e000_noop` (opcode 07) and E314's names no file function, so native function ids 1 and 90 are bound by nothing; E134/E242 `update_ring_stack_mesh_state`, E454/E455 `map_set_3d_objects_to_state_2_state` and E456 `map_step_freeze_state` sit in callback slots (script opcode 06) that no keyframe selector fires. E457.BIN, which has no code, binds native function 88 (0x801c2578, E259's `effect_update_dome_mesh`; data in its own image) to slot 0, and none of its keyframes names a slot.
- `src/battle/battle_map_draw_mesh_and_weather.c`: when map command 0x96 (E458) ends, `battle_map_run_weather_command(0x8b)` restores a non-snow rain or storm mid-draw; the mode-0x55 block then re-links `ft3` drops the 0x96 block already linked at OT+2, an ordering-table cycle that holds `DrawSync` until its 240-VBlank timeout.
- `src/world/world_card_run_menu_screen.c`: its 0xCCD8-byte stack frame, containing packet pools and directory entries, reaches below 0x801ff000 into the top cells of MAIN's game heap (0x801df000-0x801ff000); it is safe only while those cells are free.
- `src/world/world_tutorial_command_end.c`: two paths fall off the end of the s32 handler and return a stale nonzero `$v0` (the fade's result, or the 1 just stored), so the formation loop keeps running.
- `src/world/world_menu_draw_thread_status_indicators.c` and its BUNIT twin (the ATTACK, DEBUGCHR, EQUIP and REQUIRE copies have no caller): while a unit-status banner thread (8 or 7) is being stopped its first parameter is 0, and the panel check reads that "display record" at RAM address 0.
- `src/open/open_script_draw_text_records.c`: passes OT entries by value (`otag[15]`, `otag[8]`) to `AddPrim`; after `ClearOTagR` each holds the physical address of the entry below it, so the credits fade, bands and glyphs link into layers 14 and 7 through the KUSEG mirror.
- `src/wldcore/wldcore_gfx_step_dissolve_image_upload.c`: the step-0 call (start the dissolve) has no return value of its own; `$v0` keeps its clearing cursor, the address one byte below the band-progress table. Its three callers ignore that result.
- `src/world/world_menu_build_icon_record.c`: for the Bar's send-unit list (header 0x10) the arrow x reads the "height" of the record at the panel's value pointer, `&g_wldcore_active_menu_value` + 6, which is the upper half of `g_wldcore_proposition_dispatch_days` (0).
- `src/battle/battle_unit_generate_treasure.c`: outside action execution (AI simulation, preview) it returns with no value, so `$v0` still holds `g_battle_action_state` and the crystal pickup result names item 1 or 2 as the treasure.
- `src/open/open_title_step_new_game_start.c`: no retail code sets step 5, which pops New Game and opens the Music Test (`open_menu_start_music_test_controller`, controller slots 7 and 8); only a poke of the 32-bit step value reaches it. Community sources say the Japanese release opened it for the name "BGM kikitai" (in kana), a check this release does not have.
- `main_overlay_call_battle_entrypoint` calls BATTLE.BIN's entry point at
  `main_overlay_battle_entry_stub` (0x80010ba8): MAIN's 4 KB of zero words
  between the C start-up and the game code, which run as nops up to the
  `jr ra` at 0x80011ba8. The call returns at once; `main_system_run_game_loop`
  enters BATTLE through `battle_state_run_game_loop` after the load.
- `main_overlay_exec_battle_bin` reads 752 sectors from BATTLE.BIN's, the
  whole overlay area up to `g_main_high_overlay_window`, 69 past the file:
  the EVENT directory record and 67 blank sectors land at
  0x801bc800-0x801df000, so BATTLE's effect state from `D_801BF000` on starts
  as the disc's zeros. Port debt: a loader that reads only the file leaves
  those bytes as they were, and a disc that moves BATTLE.BIN puts other
  sectors there.
- `main_overlay_exec_wldcore_and_world_bin` and `main_overlay_load_world_bin`
  read 480 sectors from WORLD.BIN's, its whole RAM window up to
  `WLDCORE_SCRATCH_BUFFER`, 4 past the file: the SOUND directory record lands
  at 0x801ce000-0x801d0000.
- WORLD's copy of the event interpreter never runs: `world_script_execute_event` is reached only through `world_script_start_current_event_thread`, whose callers `world_menu_start_entry_thread` (no caller) and `world_menu_dispatch_system_function` (reached only from `world_menu_init_system_function`, below) never run, and no table holds their addresses. It calls BATTLE routines (`battle_map_freeze`, `battle_gfx_load_unit_graphics_by_battle_id` and others), since WORLD links against BATTLE's addresses for the code it copies: `camera_wait_until_idle`, for one, calls `battle_camera_is_active` at 0x8008e17c, which with WORLD loaded is inside WLDCORE's `wldcore_map_find_best_route_path`, as do `world_menu_display_triangle_selection` and `world_menu_start_help_menu_thread`. The other copies bind the same way: `world_script_map_darkness_thread` calls `battle_map_start_ambient_light_blend` (0x800934d0, WLDCORE data), `world_unit_apply_staged_status_data` runs `battle_status_queue_runtime_graphics_flag_change` (0x80068e80, 0x10 bytes into `wldcore_map_update_scroll_axis_speed`), `world_unit_save_staged_status_data` runs `battle_status_apply_unit_action_removal` (0x8018ec44, WORLD text data), `world_unit_get_battle_id_by_entd_unit_id`, `world_unit_set_animation`, `world_script_blue_remove_unit`, `world_script_blue_remove_all_enemy_units` and `world_script_teleport_out_event_instruction` call `battle_unit_get_battle_id_by_runtime_id` (0x8008cdd0, 0x20 bytes before the end of WLDCORE's `wldcore_map_color_and_draw_dots`), and `world_menu_update_menus_and_event_speed` calls `battle_state_get_vsync_interval` (0x8008ded8, 0x2dc bytes into `wldcore_map_draw_path_animation`). Its camera threads (`world_camera_thread`, `world_camera_fusion_thread`) never run either.
- WORLD's copy of the battle menus is dead code: nothing in any module calls or names `world_menu_init_system_function` (0x800f5230) or `world_menu_start_mini_menu_display_thread` (0x800f0e48), so modes 1 (AT list) and 2 (crystal pickup panel) of `world_option_run_menu` never run; the world map's Options row runs mode 0.
- `src/shared/script/script_resolve_runtime_id.c` (BATTLE and WORLD): during
  Game Over event `0x194` every unit lookup resolves to Ramza, but no retail
  code, scenario chain, `BTLEVT.BIN` condition, event or world script starts
  `0x194`; the engine plays the byte-identical script at `0x190` without the
  redirect.
- Abilities no skillset lists: 0x16f (Attack), Reflect (0x1b8), Non-charge,
  Teleport 2, Move Underwater and Silent Walk. Four ids with no name text are
  played as Altima's teleport effects (0x28, 0xb8, 0xdb, 0xdc), and 0x200 is
  the follow-up strike that resolves a knockback.
- Skillset rows 0x86-0x8d, 0xa7 and 0xa8 have no name text but hold
  abilities; 0xa7 is the row the AI uses for a unit under Blood Suck, and
  0xa9, which it pairs with ability 0x16f, is empty.
- The ability names "Move in Water" (0x1f6) and "Walk on Water" (0x1f7) are
  the reverse of what the abilities do: 0x1f6 keeps the unit on the surface
  and 0x1f7 one level below it (`battle_unit_get_effective_height`,
  `battle_move_store_unit_movement_to_scratchpad`).
- Selling in the fur shop adds the sold copies to the poached stock
  (`world_shop_run_fur_sell_confirm_step`), so they can be bought back there.
- `world_shop_run_fur_buy_quantity_menu` divides the gil on hand by
  `price >> 1` with no lower bound, while the sell side
  (`world_item_get_half_price`) keeps half price at 1 or more.
- `battle_move_start_teleport_step` builds a secondary-effect record in a
  local and never uses it.
- A change to a map CLUT row reaches VRAM on the second call of
  `battle_map_upload_clut_rows`: one call loads the 256x14 image to y 0x1ee,
  the next moves it to y 0x1e0. Rows 8 and 9 have state and are uploaded, but
  no 16-bit CLUT ID in the code addresses them.
- Rolls one off their round number: `wldcore_map_roll_random_encounter` tests
  `chance >= pick` over 0..99, so the retail chance byte 30 gives 31 in 100;
  the preferred-job promotion in `wldcore_proposition_determine_success`
  passes on a roll below 0x3D, 61 in 100; `battle_formula_apply_status_to_action`
  divides the displayed accuracy of separate statuses by four and keeps each
  status on a roll below 24; `main_unit_generate_out_of_battle` and
  `main_unit_store_character_names` draw a generic name index with
  `rand() * 255 / 0x8000`, 0-254, so index 255 of each name page is never drawn.
- `battle_unit_apply_level_up_down_ability` stores the new level before
  calling `main_unit_apply_level_growth`, so an ability level-up divides each
  raw stat by growth + new level; an EXP level-up (`main_unit_check_level_up`)
  and the generated-unit replay in `main_unit_calculate_actual_stats` divide
  by growth + old level.
- `battle_unit_find_war_trophies_and_bonus_money` computes
  `level_bonus_money` ((highest enemy level - lowest party level) * 100, 0
  once a player unit has fallen) and nothing reads it; REQUIRE pays
  `level_sum_money` and `bonus_money`.
- `src/battle/battle_ai_load_ability_entry.c`: with Throw Item the AI gives an
  item a range of the unit's Move; `battle_target_set_panels_for_action` uses
  4, and gives a Mime the same.
- `battle_map_run_gns_command_record`: command rows 0x85, 0x86, 0x88, 0x89, 0x8a
  and 0x8b skip the map-state comparison and are applied in file order, so the
  last of a kind wins: MAP017.GNS has one set-outdoor row followed by six
  set-indoor rows and ends indoors. Row types 0x87 (49 rows in 14 maps) and
  0x8a (125 rows) have no case in `battle_map_dispatch_gns_resource`, and no
  retail GNS holds a 0x83, 0x84 or 0x8b row, the three that take arguments.
- Terrain data the code does not expect: five tiles hold a slope byte outside
  `map_slope_type_e` (0x10 in MAP017.10, MAP043.9 and MAP078.9, 0x46 in
  MAP036.46, 0x01 in MAP045.7), and the terrain chunks of 37 maps arrive with
  mark bits already set in tile byte 5, which
  `battle_map_copy_size_and_tile_data` copies as they are. Tile flag 0x80
  (`cannot_stop`), which `battle_move_set_reachable_tile_flags` tests, is set
  in no terrain chunk, and tile byte 1 is 0 in all of them.
- `g_battle_map_directory_path` is the string `C:\FFTIMG\MAP\GNS\`; every map
  file load passes it as the first argument, which
  `battle_map_start_file_load` ignores.
- The Brave Story "Fact" row never appears:
  `EVENT_SCRIPT_VAR_FACTS_ENABLED` (0x92) gates it
  (`wldcore_menu_build_brave_story_entries`), no code sets it, and the one
  event script that writes it, 0x075, stores 0 while setting facts
  0x1a4-0x1aa. The 24 fact names in WORLD.LZW are empty strings.
- `g_wldcore_treasure_sound_novel_ids` is 0 for every treasure but 31
  (Germonik Scriptures, novel 5), so `wldcore_menu_step_treasure_detail_level`
  opens no reader for the four book treasures 17-20.
- World-map script opcodes 0x10 and 0x11 pass on a date on or after and on or
  before their operand (`wldcore_script_check_date_on_or_after`,
  `_on_or_before`); scenario condition types 16 and 17 are the reverse
  (`EVENT_SCENARIO_CONDITION_DATE_ON_OR_BEFORE`, `_ON_OR_AFTER`), although the
  war-funds and casualty tests beside them share their numbers.
- The Music Test names and comments (OPEN.LZW text sections 24 and 25, text
  ids 0xC000 and 0xC800 + entry) are not translated: each string is the
  Japanese release's glyph codes around Latin fragments such as the composer
  tags (`open_menu_update_music_test` shows them).
- SCUS_942.21 0x80028f94-0x80028fe7, between `g_suzuki_smd_modulator_waveforms`
  and `g_suzuki_smd_key_octaves`, is referenced by no instruction or pointer in
  any executable: a five-byte SMD sequence (`B0 91 7F 99 90`), ten 32-bit values whose
  last nine are the LIBSPU reverb work-area sizes of modes 1-9, and a
  0x23-byte `feds` sound bank with id 0x7000.
- Ability data the code and the tables disagree on:
  `battle_target_mark_ability_area` passes range-record `flags_2`
  bit 0x20 (`ABILITY_SECONDARY_FLAG_2_TOP_DOWN`) to
  `battle_target_apply_vertical_tolerance` as `single_layer`, and none of the
  368 records sets it; the low nibble of
  `ability_data_t.type_flags` holds the ability type plus one (0 for ids 0,
  0x1fe and 0x1ff) and nothing reads it, because
  `main_ability_calculate_pointers_and_type` derives the type from the id.
- `main_status_init_check_data` builds eleven status check sets; sets 2 and 7
  (`MAIN_STATUS_CHECK_SET_UNREAD_2`, `_UNREAD_7`) are read by nothing.
- WORLD's copies of BATTLE's action-menu and tile-info routines
  (`world_menu_build_ability_list`, `world_menu_build_skillset_entries`,
  `world_menu_selected_tile_info_display_thread` and nine more) keep BATTLE's
  absolute data addresses 0x80173cbc, 0x8017403c, 0x80174048 and 0x8017404c.
  In WORLD's image those lie inside the text file embedded at 0x8016ee68, so
  the routines would read and write text bytes there.
- WORLD's copies of BATTLE and EVENT code bind 117 of those modules' names at
  their original addresses, and `make build` keeps them there: neither module
  is ever in RAM with WORLD. Of the 93 WORLD functions that name one, 70 are
  unreachable from code that runs; the other 23 are reached only through the
  menu tables WORLD copied from BATTLE or the text thread. With WORLD loaded
  each address lies inside WLDCORE code or WORLD data, never on the routine,
  so no working path depends on them.
- `helpmenu_menu_run_battle_help` and `helpmenu_menu_run_require_help` test
  `g_helpmenu_selected_unit_panel_mode`, a fixed halfword at 0x801ce8fe in the
  companion window: BUNIT's `g_bunit_panel_selected_billboard` + 2 while BUNIT
  is loaded, and whatever the overlay there holds otherwise (0xb6d9 in
  REQUIRE's file), which adds the navigation between help entries 2 and 7.
  `make build` keeps the address fixed.
- `g_main_terrain_geomancy_abilities` gives Gusty Wind for Sky and Kamaitachi
  for Darkness, Cross section and the unnamed surfaces 0x2f-0x3e; the terrain
  help text says "None" for Sky, Darkness and Cross section.
- `battle_camera_update_rotation` chains a held R1 through
  `battle_camera_set_left_turn_destination`, as it does L1; the two helpers differ only in
  which way they move a destination that is a multiple of 0x400.
- `main_unit_copy_job_data` adds the immortal and formation immunity sets
  only to a unit that is both immortal and flagged save- or load-formation;
  the per-flag tests inside the loop are then always true.
  In the retail ENTD files 46 units are both. The other 119 immortal units
  resist, beyond their job's immunities, only the formation set, which
  `battle_status_modify_inflictions` applies to every immortal target, so
  they can still be Confused, put to Sleep or given Don't Act.
- `battle_formula_apply_status_to_action`: the
  `BATTLE_ACTION_STATE_PREVIEW` test in the separate-status loop cannot pass;
  any state but EXECUTE was switched to all-or-nothing above it.
- `main_gfx_build_now_loading` loads the NOW LOADING CLUT to VRAM (0x70,
  0x1e2), 16-bit CLUT ID 0x7887; in battle that is
  `MAP_CLUT(MAP_CLUT_ROW_SPRITES, 7)`, the status bubble CLUT.
- The effect interpreter indexes fixed tables with wider fields and no check:
  the opcode is masked to nine bits over the 120 entries of
  `g_battle_effect_instructions` (0x2e-0x77 report an error without returning
  a result: `battle_effect_run_block` narrows the leftover `$v0` to `s16`
  and tests it while the script position stays unchanged);
  `g_battle_effect_particle_render_handlers` has four entries for
  `(kind_flags >> 12) & 0xf`, so `EFFECT_WORK_KIND_HEAP_BLOCK` would call
  through `g_battle_effect_camera_rotation_mode`; opcode 06 takes a six-bit
  callback slot over the five 32-bit slots of `g_battle_effect_callback_slots`, and a
  keyframe action names slot 1-7 over the record's four (5 writes
  `target_index`). Retail files use opcodes below 0x2e, kinds 0 and
  2 and slots 1-4.
- Special unit name 0x44, "Rofokare", is given to no ENTD unit; every other
  named entry of the 256 is.
- `battle_heap_alloc_block` has no out-of-memory exit: when no free block is large enough it
  walks the circular free list forever. `attack_deploy_select_unit_for_stats_display` likewise
  loops forever when no roster unit is deployable for the squad, and
  `battle_effect_alloc_work_record`, with its free chain empty, calls the empty
  `main_system_report_error_2` and then reads the next link through NULL.
- WORLD runs threads in slots 1-16: `thread_yield` wraps at 17, and `world_thread_resolve_id`
  and `world_thread_find_running_by_task` scan to 16 (BATTLE stops at 15). The array has room for
  the 17th slot (`g_gfx_tpage7_vram_allocation_grid` starts right after it), but
  `thread_reset_scheduler` clears only slots 0-15, so slot 16 keeps its state across a reset.
- `battle_state_handle_change_map_jumping_in_state` turns event results 9 and 0xA into game flow
  0 and 1, the reverse of `battle_state_handle_event_state`, and ignores 0xB and 0x13, so a
  scenario that finishes during an event map change's fade-in would take the other exit (or
  none).
- `battle_turn_take_next_event` handles turn event 0x400 (`BATTLE_TURN_EVENT_UNKNOWN_0400`), but
  `g_battle_turn_event` only takes `battle_turn_run_clock`'s results, which never include it, so
  that arm never runs.
- `battle_move_start_next_post_move_event` and `battle_state_handle_crystal_learn_state` handle
  post-move event `0x200` (`BATTLE_MOVE_POST_EVENT_SOURCE_DISPLAY`, a display refresh of the
  mover), but `g_battle_move_post_move_events` only takes `battle_move_get_post_move_events`'
  results, which never include it, so that arm never runs.
- `src/battle/battle_formula_apply_weather_effects_on_bows.c` reads the weather script variable
  instead of `battle_map_get_effective_weather`, so a snowstorm also cuts bow and crossbow hits and
  the map's ignore-weather flag is not checked; the element modifiers use the effective weather.
- `src/battle/battle_formula_damage_pa_times_wp_plus_y_status.c` (Holy Sword) and
  `battle_formula_break_equipped_damage_pa_times_wp.c` (Might Sword) apply the Strengthen and
  affinities of the weapon's element only; the element in the ability's own data (Holy for the
  Holy Sword abilities) is never read.
- `src/main/main_ability_store_skillset_abilities.c`: the support filter takes ids below `0x1e7`, so
  Move+1 (`0x1e6`) counts as a support ability and not a movement one. Every caller asks for action
  abilities or for all of them, so nothing shows it.
- `src/battle/battle_menu_display_projected_action_effect.c`: the 0x100 bit that
  `main_status_find_action_highest_order_effect` adds for a status flagged
  `STATUS_EFFECT_FLAG_2_SCRIPT_FILTER_EXCLUDED` changes nothing. The preview tests 0x80 (a removal) first, so
  its 0x180 test, which would pick marker image 0x1e, never passes.
- `src/battle/battle_unit_set_animation_based_on_status.c` picks animation 0x21 for bit 0x01 of the
  status mirror `status_flags_1_4`, but no status sets that bit
  (`g_battle_unit_status_mask_by_handler_index`), so the animation never plays.
- `src/battle/battle_ai_add_usable_skillset_abilities.c`: for a Jump skillset with no learned Level Jump,
  the bare Jump entry is written past the list's count, so the next entry (the next skillset's or Frog
  Attack) overwrites it and the unit has no Jump entry.
- `src/battle/battle_ai_build_unit_ability_list.c`: the weapon kinds of both hands are ORed and only one
  kind at a time is handled (none, damage, absorb, heal), so a unit holding two kinds (a Blood Sword with
  another weapon under Two Swords) gets no target-team flag on its Attack entry.
- Half of MP: `battle_menu_get_unit_skillset_ability_data` lists the half rounded up and the menu grays
  an ability by it, but `battle_action_check_and_consume_mp` and the AI use the half rounded down. Seven
  abilities have odd costs (Zodiac 99; Toad 2, Gravi 2, Flare 2, Blind 2, Confuse 2 and Sleep 2 at 35),
  so a unit with exactly the lower half sees them grayed.
- `src/battle/battle_ai_select_auto_battle_action.c`: Protect allies falls back to Run like a rabbit when
  `battle_ai_select_protect_allies_action` returns -2, which it never does.
- `src/battle/battle_ai_evaluate_origins.c`: an ability without
  `BATTLE_AI_ABILITY_EXHAUSTIVE_ORIGIN_SCAN` multiplies its origin and target tile counts and tests
  the byte product against 10, but both arms set `ranked_origin_search`, so every such ability
  takes its origins best first (`battle_ai_take_best_origin`).
- `src/battle/battle_ai_is_unit_untargetable.c`: a Transparent enemy is left out of the AI's
  targets, except a rider whose mount is Transparent too.
- `battle_menu_run_status_screen_thread` hands a unit with mount info (a
  rider or a mount) to DEBUGCHR.OUT's `debugchr_editor_run_unit_thread`
  (`battle_menu_open_rider_status_screen`); any other unit gets BATTLE's own
  panel.
- `src/event/debugchr_editor_run_session.c`: the debug editor moves and clears the job, equipment
  and ability fields like the others, but `debugchr_editor_apply_values_to_battle_unit` writes back
  only HP, MP, CT, level, experience, Brave, Faith, the zodiac sign, Move, Speed and Jump, and the
  reload after each step (`debugchr_editor_load_selected_unit`) restores the unit's job, equipment
  and abilities. Nothing opens the editor itself: no code or retail row action selects menu entry
  44 (`BATTLE_MENU_ID_DEBUGCHR`, `battle_menu_open_debugchr` and its WORLD copy), and
  `g_debugchr_editor_active` is set only by `debugchr_editor_run_session`.
- Functions that nothing references: no `jal` or `j` in any executable or overlay that can be resident with them targets them, and no word or `lui`/`addiu` pair there holds their address. Psy-Q library routines are left out, as are functions whose only callers are listed here.
  - BATTLE (118): `battle_action_activate_move_act`, `battle_action_load_last_used_ability`, `battle_action_store_8019387c_if_not_reacting`, `battle_camera_get_unit_sort_point_unbiased`, `battle_camera_save_offset_screen_coords`, `battle_camera_get_offset_screen_coords`, `battle_camera_start_tilt`, `battle_camera_restore_offset_screen_coords`, `battle_camera_start_screen_offset_move_uniform`, `battle_camera_start_screen_offset_move_relative`, `battle_camera_set_offset_screen_coords`, `battle_camera_set_real_coord_delta_from_destination`, `battle_camera_set_real_coord_delta_from_values`, `battle_camera_set_rotation_delta_from_value`, `battle_camera_set_rotation_delta_from_values`, `battle_camera_set_zoom_delta_from_value`, `battle_camera_set_zoom_delta_from_values`, `battle_camera_start_zoom`, `battle_unit_copy_world_coords_to_scratch`, `battle_effect_add_2_to_halfword`, `battle_effect_add_4_to_halfword`, `battle_effect_add_6_to_halfword`, `battle_effect_add_8_to_halfword`, `battle_effect_get_list_node`, `battle_effect_get_prim_buffer_offset`, `battle_effect_get_world_coords_by_battle_id`, `battle_effect_init_fall_dust_secondary`, `battle_effect_init_single_target_trajectory`, `battle_effect_start_file_load`, `battle_effect_report_stub`, `battle_effect_report_stub_return_zero`, `battle_effect_replace_flags_0006`, `battle_effect_select_larger_magnitude_s16`, `battle_effect_start_basic_ability_animation`, `battle_formula_49_unused_with_extra_steps`, `battle_gfx_clear_screen_color_modulation`, `battle_gfx_copy_screen_color_modulation`, `battle_gfx_get_vram_slot_summary`, `battle_gfx_restore_screen_color_modulation`, `battle_gfx_set_sprite_part_flip`, `battle_gfx_set_thrown_item_graphic_palette_by_battle_id`, `battle_heap_print_stats`, `battle_map_adjust_background_color`, `battle_map_copy_background_color`, `battle_map_get_background_color`, `battle_map_get_weather`, `battle_map_restore_background_color`, `battle_map_set_ambient_light_color`, `battle_map_set_background_color`, `battle_map_stop_palette_animation`, `battle_map_transition_and_store_ambient_light_color`, `battle_map_transition_and_store_darkness_color`, `battle_map_update_all_palettes_from_color_data`, `battle_map_update_and_store_palette_colors`, `battle_map_update_palette_colors_by_index`, `battle_menu_clear_window_buffer_pair_by_thread`, `menu_confirm_sound_delay_thread`, `battle_menu_display_text_entry_past_choice_marks`, `battle_menu_load_portrait_image`, `battle_menu_reset_unit_records_wrapper`, `battle_state_enter_ability_preview_help`, `battle_move_calculate_pathing_time_sliced`, `script_run_next_event_unused`, `battle_script_set_current_otag_entry_and_update_input`, `battle_state_enter_effect_playback`, `battle_state_get_frame_measurement`, `battle_state_enter_effect_state`, `battle_state_enter_map_jumping_in`, `battle_state_enter_map_jumping_out`, `battle_status_resolve_unit_changes_in_preview`, `text_measure`, `battle_text_set_substitution_values_7e_7f`, `battle_unit_add_event_offset_by_runtime_id_unused`, `battle_unit_resolve_projectile_hit_by_battle_ids`, `battle_unit_check_exists`, `battle_unit_get_action_command_by_battle_id`, `battle_unit_count`, `battle_unit_erase_pending_party_removal`, `battle_unit_get_camera_facing_quadrant_by_runtime_id`, `battle_unit_get_camera_facing_sixteenth_by_runtime_id`, `battle_unit_get_world_coords_by_battle_id`, `battle_unit_init_party_member_battle_data`, `battle_unit_mount_by_runtime_ids`, `battle_unit_restore_target_facings_after_attack`, `battle_unit_set_animation_from_facing_hint`, `battle_unit_set_movement_idle_animation_by_runtime_id`, `battle_unit_set_target_anim_by_battle_id`, `battle_unit_store_camera_facing_animation_for_all`, `battle_unit_update_display_by_battle_id`, `battle_noop_8008de88` (and the same at 80134430, 80134a6c, 80137b90, 8013bd64, 8013bdc4, 8013c740, 8013caf4, 8013d624, 8013d62c, 8013da60, 8013da68, 8013da78, 8013da80, 8013f4a0, 8013f7ac, 8014268c, 80147918, 8014a824, 801a1328, 801a6010, 801a6018, 801a866c, 801a8e74, 801aad30, 801b0ff4), `battle_return_argument_80133494`, `battle_return_zero_8008e6d0` (and the same at 8013d1f0). `battle_camera_start_zoom` and `battle_camera_start_tilt` are bypassed (L2 and R2 go through `battle_camera_toggle_zoom` and `battle_camera_toggle_tilt`); `battle_formula_49_unused_with_extra_steps` is a second entry to formula 0x49's handler that the formula table does not name. The eight `battle_camera_set_*_delta_*` setters are the only code that starts the countdowns of `battle_camera_update_real_coord_animation` and its offset, zoom and rotation siblings, which the action-execute, event and map-change states call every frame and which never move anything. `battle_state_enter_effect_playback` and `battle_state_enter_effect_state` are the only code that stores `BATTLE_GAME_STATE_EFFECT`, so `battle_state_handle_effect_state` never runs.
  - WORLD (87): `world_unit_copy_world_coords_to_scratch`, `world_text_upload_glyph_image`, `world_formation_show_unit_message`, `gfx_append_gpu_primitive_to_secondary_otag`, `world_gfx_append_poly_f4_to_otag`, `world_menu_enable_height_display`, `gfx_flip_poly_ft4_uv_if_negative`, `world_menu_suppress_height_display`, `job_calculate_and_store_unlocks`, `world_map_get_weather`, `world_menu_clear_window_buffer_pair_by_thread`, `menu_confirm_sound_delay_thread`, `world_menu_disable_alternate_main_palette`, `world_menu_enable_alternate_main_palette`, `world_menu_get_crystal_pickup_selection`, `world_menu_get_selected_command_address`, `world_menu_icon_clamped_list_thread`, `world_menu_icon_strip_thread`, `world_menu_init_and_load_scrollable_list_with_scroll`, `world_menu_init_attack_resources_and_threads`, `world_menu_init_system_function`, `world_menu_load_portrait_image`, `world_menu_poll_small_load`, `world_menu_preview_ability_turn_at_list`, `menu_request_open_companion_data_file`, `menu_request_open_small`, `world_menu_end_at_list_unit_view`, `world_menu_reset_unit_record`, `world_menu_reset_unit_records_wrapper`, `world_menu_resize_parent_entry_to_digits`, `world_menu_restore_selection_for_selected_unit`, `world_menu_run_require_help`, `world_menu_run_thread_with_selected_index`, `world_menu_set_default_entry_0_text_values`, `world_menu_set_default_entry_1_text_values`, `world_menu_set_main_scroll_parameters`, `world_menu_start_entry_chain_threads`, `world_menu_start_entry_thread`, `world_menu_start_mini_menu_display_thread`, `world_menu_store_auto_battle_main_target_id`, `world_menu_take_and_clear_thread_result`, `world_menu_text_entry_thread`, `world_option_menu_thin_window_frame_thread`, `world_menu_update_menus_and_event_speed`, `world_noop`, `world_option_gfx_build_scaled_draw_area_packets`, `world_option_gfx_init_scaled_draw_area_packets`, `world_return_argument_800e7b4c`, `world_return_zero`, `world_screen_gfx_build_scaled_draw_area_packets`, `world_screen_gfx_init_scaled_draw_area_packets`, `world_screen_menu_thin_window_frame_thread`, `script_filter_controller_input`, `world_script_get_event_finish_operation`, `world_script_is_deployment_running`, `world_script_reset_event_state`, `script_run_next_event_unused`, `script_scale_music_volume`, `world_script_set_current_otag_entry_and_update_input`, `world_text_count_characters`, `world_text_print_fnt_value_and_next`, `world_text_set_substitution_values_7e_7f`, `world_text_start_thread_if_idle`, `world_thread_start_idle_5_to_8`, `world_thread_stop_after_empty_slots_5_to_8`, `world_option_thread_stop_after_yield`, `world_unit_update_facing_and_pending_animations`, `world_noop_800e8aa8` (and the same at 800e90e4, 800eba28, 800ef97c, 800ef9cc, 800f01f0, 800f05a4, 800f0f54, 800f0f5c, 800f1378, 800f1380, 800f1390, 800f1398, 800f2d18, 800f3024, 800f5f04, 800fad84, 80106138), `world_return_argument_800e7b54`, `world_return_zero_800f0b5c`. Most are its copies of BATTLE and overlay code; the quirks recorded above for `world_menu_resize_parent_entry_to_digits` and `world_menu_run_require_help` therefore never execute in WORLD.
  - WLDCORE (3): `wldcore_sound_novel_find_section_instruction`, `wldcore_sound_novel_calculate_completion_percent`, `wldcore_text_get_padded_dimensions`.
  - OPEN (3): `open_card_wait_for_hardware_event`, `open_menu_init_render_record_fields`, `open_text_measure_menu_image`; the first is the only caller of `card_io_poll_hardware_event_result`.
  - MAIN (55): `main_sound_set_unread_value_800184e0`, `main_ability_get_aoe`, `main_ability_get_range`, `main_gfx_fade_in_orbonne_monastery`, `main_gfx_fade_out_orbonne_monastery`, `main_job_find_by_skillset`, `main_overlay_call_battle_return_zero`, `suzuki_smd_dispatch_snapshot`, `suzuki_smd_get_held_voices`, `suzuki_smd_get_instrument_attr`, `suzuki_smd_get_min_loop_count`, `suzuki_smd_get_music_filename`, `suzuki_smd_get_play_time`, `suzuki_smd_init_channel_streams`, `suzuki_smd_pause_music`, `suzuki_smd_resume_music`, `suzuki_smd_seed_random`, `suzuki_smd_set_channel_mute_mask`, `suzuki_smd_set_channel_select`, `suzuki_smd_set_flags_all_channels`, `suzuki_smd_set_stop_bar`, `suzuki_sound_find_waveset`, `suzuki_sound_flush_key_off`, `suzuki_sound_flush_key_on`, `suzuki_sound_free_wavesets`, `main_sound_get_current_scenario_music`, `suzuki_sound_get_largest_free_block`, `main_sound_get_music_handle`, `suzuki_sound_get_sfx_channels`, `main_sound_start_music_load_into_slot`, `suzuki_sound_play_sfx_in_channel`, `suzuki_sound_play_sfx_with_settings_in_channel`, `main_sound_set_current_music_balance_shift_timed`, `main_sound_set_current_music_pitch_shift_timed`, `main_sound_set_current_music_tempo_scale_timed`, `suzuki_sound_set_sfx_balance`, `suzuki_sound_stop_all`, `suzuki_sound_stop_sfx_channel_pair`, `suzuki_sound_sum_resource_words`, `main_sound_wait_for_music_load`, `suzuki_sound_disable_root_counter_2_event`, `suzuki_sound_enable_root_counter_2_event`, `main_unit_init_stats_and_statuses`, `suzuki_noop_800121cc` (and the same at 800121d4, 800124ac, 800124b4, 800124bc, 800124c4, 80013634, 80015864, 80018654, 800186bc, 80059ac0), `suzuki_return_zero_80014204`. `main_gfx_set_display_draw_new_game` is called only by `main_gfx_fade_in_orbonne_monastery`. `main_sound_set_unread_value_800184e0` stores a 16-bit value that `suzuki_sound_init` and `suzuki_sound_quit` clear and nothing reads.
  - ATTACK (17): `attack_deploy_get_valid_tile`, `attack_deploy_init_valid_tiles`, `attack_deploy_set_arrow_position_mode`, `attack_deploy_set_cursor_tile`, `attack_deploy_set_valid_tile`, `attack_gfx_build_gradient_grid_primitives`, `menu_build_item_icon_source`, `attack_gfx_build_zodiac_primitives`, `attack_input_update_event_state`, `menu_draw_thread_status_indicators`, `attack_panel_run_ability_list_thread`, `attack_panel_run_equipment_list_thread`, `attack_return_zero`, `menu_text_render_decimal_entry_list`, `attack_noop_801c087c` (and the same at 801c3070, 801c61dc).
  - BUNIT (13): `menu_build_item_icon_source`, `bunit_gfx_get_transition_frame`, `gfx_move_image_and_wait`, `job_calculate_and_store_unlocks`, `bunit_menu_clear_selection_values`, `menu_icon_strip_thread`, `bunit_menu_init_and_draw_scrollable_list`, `bunit_menu_init_and_draw_scrollable_list_simple`, `menu_text_entry_wait_thread`, `bunit_panel_run_ability_list_thread`, `bunit_panel_run_equipment_list_thread`, `menu_text_render_decimal_entry_list`, `bunit_noop_801c0adc`.
  - CARD (7): `card_gfx_enqueue_textured_quad_list`, `card_gfx_set_draw_color`, `menu_set_vsync_mode_and_event_speed`, `card_save_clear_selection_indices`, `card_text_count_encoded_glyphs`, `card_text_start_character_thread_if_idle`, `card_text_start_thread_if_idle`.
  - DEBUGCHR (12): `debugchr_gfx_build_gradient_grid_primitives`, `menu_build_item_icon_source`, `menu_build_zodiac_primitives`, `debugchr_input_update_event_state`, `menu_draw_thread_status_indicators`, `menu_icon_list_thread`, `debugchr_menu_icon_strip_thread`, `menu_text_entry_wait_thread`, `debugchr_panel_run_ability_list_thread`, `debugchr_panel_run_equipment_list_thread`, `menu_text_render_decimal_entry_list`, `debugchr_noop_801c090c`.
  - EQUIP (17): `equip_gfx_enqueue_textured_quad_corners`, `menu_gfx_enqueue_textured_quad_list`, `gfx_move_image_and_wait`, `equip_gfx_set_draw_color`, `equip_input_update_event_state`, `equip_menu_clear_selection_record`, `equip_menu_draw_thread_status_indicators`, `menu_icon_strip_thread`, `equip_menu_init_and_draw_scrollable_list`, `equip_menu_init_and_draw_scrollable_list_simple`, `equip_menu_stop_threads`, `equip_menu_text_entry_wait_thread`, `equip_panel_run_ability_list_thread`, `menu_run_status_panel_thread`, `equip_set_s16_801d86ac`, `equip_unit_has_two_hands`, `equip_noop_801c151c`.
  - ETC (1): `etc_noop_801bf3d8`.
  - JOBSTTS (8): `menu_bits_init_secondary_reader`, `menu_bits_read_secondary`, `jobstts_gfx_enqueue_oriented_textured_quad`, `jobstts_gfx_enqueue_textured_quad_corners`, `jobstts_gfx_enqueue_textured_quad_list`, `jobstts_gfx_enqueue_textured_quad_rect`, `jobstts_menu_init_and_draw_scrollable_list`, `jobstts_menu_init_and_draw_scrollable_list_simple`.
  - OPTION (4): `menu_build_scaled_draw_area_packets`, `menu_init_scaled_draw_area_packets`, `option_menu_thin_window_frame_thread`, `option_thread_stop_after_yield`.
  - REQUIRE (15): `require_file_load_sync`, `menu_build_item_icon_source`, `menu_build_zodiac_primitives`, `require_input_update_event_state`, `require_input_wait_skip`, `menu_draw_thread_status_indicators`, `menu_icon_list_thread`, `menu_text_entry_wait_thread`, `require_panel_run_ability_list_thread`, `require_panel_run_equipment_list_thread`, `sound_play_music_tracks`, `require_text_render_decimal_entry_list`, `require_thread_stop_after_16_frames`, `require_noop_801c07ac` (and the same at 801c9fa4).
  - SMALL (1): `small_return_zero`.

- `StartCARD` and `StopCARD` leave their declared signed 32-bit results unspecified.
- `effect_e000_noop`: the native callback returns without setting a result; opcode 07 ignores that result.
- E464's map-vertex particle callback accepts groups 1-12 but indexes a
  three-entry location table; all 16 shipped emitters use group 0, so the
  out-of-range path is inactive.
- `world_card_load_globals_from_save_image` copies the saved first item-order list
  (`order_0` of `g_main_item_type_order_tables`) only if none of its first 11 keys is
  11, and the second (`g_main_item_type_order_1`) only if none of its first seven is;
  otherwise it keeps the live list. It writes each list's -1 terminator either way and
  goes on loading the save.
- `battle_map_run_weather_command` picks rain or snow from the raw weather variable and
  the map's snow flag: the precipitation paths do not test
  `BATTLE_MAP_WEATHER_FLAG_IGNORE_WEATHER`, which takes the weather out of the damage
  and movement rules through `battle_map_get_effective_weather` but does not itself
  stop the particles.
- OPTION.OUT carries its own copy of `menu_init_scaled_draw_area_packets` (0x801c02d0,
  in the list above), but its list, options and window-frame threads call BATTLE's
  `battle_gfx_init_scaled_draw_area_packets` (0x80138adc).
- Ability 457, named "Equip Knife", sets the Equip Katana support bit
  (`BATTLE_SUPPORT_SET_1_EQUIP_KATANA`): the unit can equip katanas, not knives.
- `FlushCache` is called only by the BIOS patches (`_patch_card`, `_patch_card2`,
  `_patch_gte`, `psyq_card_restore_exception_prefix`); no overlay load flushes the
  instruction cache before the new code runs.
- `_spu_FsetDelayR` sets the SPU bus timing for DMA writes and `_spu_FsetDelayW` for
  reads, and `_spu_t` and `_spu_Fr_` call them that way: each suffix names the opposite
  direction.
- `SpuQuit` closes the SPU DMA event before disabling it, so `DisableEvent` gets a
  closed descriptor.
- World-map script opcodes 0x22-0x24 (`wldcore_script_request_draw_location`,
  `wldcore_script_request_erase_location`, `wldcore_script_focus_location`) read their
  operand without advancing `ip`; each ends the block, so no opcode is read from the
  operand.
- `battle_map_get_move_find_result` repeats the same `battle_unit_find_at_tile` call up
  to 21 times: its occupant loop never uses the counter.
- `battle_turn_advance` calls `battle_unit_store_entd_flags_without_control_flag` twice,
  around `battle_menu_reset_unit_record`, which leaves the team flags alone; the first
  call reads `unit->stats` before the null test that guards the second.
- `battle_effect_start_action_projectile_secondary` tests Throw Shuriken (382) on its
  own before the Item/Throw range 368-393, which already contains it.
- `battle_formula_steal_exp_hit_sp_x_percent` (formula 0x28) calls
  `battle_formula_set_exp_stolen` after a failed hit test whenever the action is not
  executing; outside execution only a 0% chance misses, so the preview shows 0% with an
  EXP amount. Formula 0x27 (steal gil) computes the gil only after its test.
- `battle_effect_spawn_emitter_particles`: an emitter with no velocity-spread curve uses
  half its start spread, but with a curve it uses the interpolated spread whole, so a
  curve at factor 0 gives twice the angle range of no curve. Every other spawn value
  uses the start value unchanged when no curve is bound.
- `battle_map_update_texture_animations`: the two loop modes (1 and 3) add 1 per frame
  and step when the timer reaches `duration`; the other modes add
  `g_main_system_vsync_interval` and step when it exceeds `duration`, so one duration
  means `duration` frames in a loop and over `duration` vertical blanks otherwise.
- Lookups whose result is discarded: `battle_target_show_cursor_target_panel`,
  `battle_state_handle_resume_attack_phase_state` and
  `battle_state_handle_change_turn_state` call `battle_unit_get_source`;
  `battle_state_handle_targeting_range_state` (twice),
  `battle_unit_get_tile_center_and_height_by_battle_id`,
  `battle_camera_calculate_cursor_tile_vector_normal` (the other level's tile),
  `battle_move_set_velocity_for_contiguous_clamped_steps`,
  `battle_move_set_velocity_for_contiguous_steps_with_final_tile_height` and
  `battle_move_apply_unit_step_velocity` (twice) call `battle_map_get_tile_data_pointer`;
  and `battle_gfx_draw_wait_direction_unit` runs a `RotTrans` of the unit's feet that
  the next `RotTrans` overwrites.
- `equip_menu_update_item_action_state` tests the full menu's `selected_index`
  (`g_equip_item_action_menu`) for the sort-order row (2) even while the single-item
  menu runs.
- `bunit_gfx_init_double_packet_buffers` and `world_gfx_init_double_packet_buffers` set
  their draw and display environments 255 pixels wide where MAIN and CARD use 256, so
  the default drawing area ends at x = 254; `PutDispEnv` still picks the 256-pixel
  mode.
- `world_text_measure_entry_pixel_width` adds the operand of 0xE7
  (`TEXT_SET_X_POSITION`) to the width as an advance, while the renderers
  (`menu_text_render_encoded_ids_to_image`, `world_text_render_encoded_ids_to_image`,
  `jobstts_text_render_encoded_ids_to_image`) move the pen to that x. Its only caller,
  `world_formation_build_job_wheel_job_name`, centers the job wheel's job name.
- `main_gfx_build_zodiac_sprite_polygon` and `attack_gfx_build_zodiac_sprite_polygon`
  (MAIN, ATTACK) and `gfx_set_scaled_poly_ft4_8bpp` (WORLD, CARD, DEBUGCHR, REQUIRE)
  round a scaled offset or size by testing the remainder of a truncating division
  against half a pixel, so a negative product rounds toward zero: -1.75 pixels gives -1
  where 1.75 gives 2.
- `main_file_poll_load` and `wldcore_file_poll_vram_image_stream` count failed seeks,
  refused reads and stalls in `error_count`, which nothing reads, so a disc error
  restarts the read indefinitely. `main_file_poll_load` also counts `wait_frames`,
  which nothing reads; its stall timeout uses `g_main_system_frame_timer`.
## Pointers held in 32-bit integers

The PS1's pointers and `s32` are both 32 bits wide, so some code passes or
computes addresses as integers. A 64-bit build must give these pointer types.

- The thread parameters (`thread_t.function_parameter_1`..`_4`, and the
  `*_thread_get_current_parameter_*` getters) are `s32` but often carry record
  pointers, e.g. into `battle_script_run_sprite_move` and
  `world_menu_confirm_action_silently`.
- Matching spellings compute addresses through `(u32)` casts, e.g.
  `script_get_variable_word_pointer_from_id` (offsets from
  `g_script_variables`) and `battle_ai_is_ability_known` (the
  learned-ability row).

## Hand-written assembly

The original programmers wrote these routines in assembly; each is a `.s`
file whose opening comment says why it is not C. Port debt: a native build
needs a portable C twin of each.

- `main_system_store_stack_pointer`: stores `$sp` in the word at its
  argument, which is how `main` saves the game loop's base stack in
  `g_main_system_game_loop_stack_pointer` before the loop starts; a native
  build saves that context another way (for example `setjmp`).
- `main_restore_game_loop_stack_pointer`: replaces `$sp` with the stack
  address `main` saved at boot and jumps to the top of
  `main_system_run_game_loop`, abandoning every frame above it on a soft
  reset; a native build returns there another way (for example `setjmp` and
  `longjmp`).
- `main_overlay_battle_entry_stub`: 4 KB of nops and a `jr ra`, which MAIN
  calls as BATTLE's entry point and which do nothing. A native build calls an
  empty function, and the 4 KB are MAIN's largest range of free RAM.
- `main_system_frame_hook`: an empty per-frame hook whose callers enter at a
  leading `nop`; a native build uses an empty function.
- `g_psyq_card_interrupt_jump_template`: LIBCARD's five words that load
  `psyq_card_interrupt_tail`'s address and jump there, which `_patch_card`
  copies into the BIOS exception handler. A native build has no BIOS handler
  to patch and calls the card interrupt tail itself.
- `g_psyq_card_delay_template`: LIBCARD's five words that call their own
  place, which `_patch_card2` swaps into the BIOS pad-clear routine; the five
  instructions they displace then run there and fall through into
  `psyq_card_short_delay`. A native build has no BIOS routine to patch.
- `wldcore_system_switch_to_stack`: stores `$sp` in the word at its argument
  (the scratchpad's last word) and runs on the stack below it, so the calls
  up to `wldcore_system_restore_previous_stack` keep their frames in the 1 KB
  scratchpad. Its callers touch no slot of their own frame in between. A
  native build has no scratchpad stack and keeps its own.
- `wldcore_system_restore_previous_stack`: restores the `$sp` that
  `wldcore_system_switch_to_stack` stored, from the word just above the
  current one.
- `battle_map_queue_textured_triangles`: transforms, culls, lights and links
  the map part's textured triangles through the GTE; a native build needs a C
  twin of the same arithmetic, the SXY packing under "Retail bugs" included.
- `battle_map_queue_textured_quads`: the same for the textured quads.
- `battle_map_queue_untextured_triangles`: the same for the untextured
  triangles, unlit, at the masked and offset depth slot.
- `battle_map_queue_untextured_quads`: the same for the untextured quads.
- `text_blit_glyph`: copies a glyph's 4bpp pixels with the callee-saved
  registers parked in the scratchpad; a native build needs a plain C loop.
- `world_text_blit_glyph`: WORLD's `text_blit_glyph`, without its x test.
- `battle_text_render_glyph_to_4bpp_image`: decodes a 2-bit glyph into a 4bpp
  image with its registers in the scratchpad; a native build needs a plain C
  loop.
- `world_text_blit_font_glyph_to_4bpp`: WORLD's twin of
  `battle_text_render_glyph_to_4bpp_image`.
- `battle_menu_clear_render_buffer`: a byte and word zero fill; a native build
  uses `memset`, which also handles the odd addresses this one cannot.
- `world_menu_clear_render_buffer`: WORLD's byte-identical twin.
- `thread_yield` (BATTLE and WORLD): switches threads by saving and restoring
  `$sp`, `k0`, `k1`, `$gp` and the callee-saved registers in `g_threads`; a
  native build needs another context switch (fibers, `ucontext` or host
  threads).
- `battle_util_mul_div_s64`: a 64-bit multiply and divide by shift and
  subtract with its registers in the scratchpad; a native build needs a C
  twin over 64-bit integers that keeps the retail negation results above.
- `world_util_mul_div_s64`: WORLD's twin of `battle_util_mul_div_s64`.
- `battle_util_cross_product_q12`: a 64-bit cross product with an overflow
  flag; a native build needs a C twin over 64-bit integers.
- `world_util_cross_product_q12`: WORLD's twin of
  `battle_util_cross_product_q12`.
- `thread_call_on_main_stack` (BATTLE and WORLD): calls
  `g_thread_call_target` on the main loop's stack by replacing `$sp` and
  keeps its return address in `g_thread_call_saved_ra`; a native build
  calls the target directly or on its own main-context stack.

