---
type: mechanic
tier: 1
scope:
  - battle_state
  - battle_turn
---

# Battle flow

## What it does in the game

How a battle runs from its setup to its close. Units take turns in CT order:
every clock tick adds a unit's Speed to its CT, and the first past 99 gets a
turn. On its turn a unit may move once and act once, in either order, then
chooses a facing; a unit the player does not control decides through the AI.
Abilities with a charge time resolve later, when their own counter runs out.
Reactions answer an action, and a Hamedo can cancel it. Between turns,
timed statuses run out and event scripts can play. An event script also ends
the battle: on to the world map, the next battle, a reset or the ending.

## Data and structures

- **The state machine.** [[g_battle_game_state]] holds a
  [[battle_game_state_e]] value, and [[battle_state_run_game_loop]] runs its
  handler once a frame (the table below). Effects and map changes save it in
  [[g_battle_previous_game_state]] and return to it.
- **Frame rate.** [[g_main_system_vsync_interval]] is 1 (60 fps) or 2
  (30 fps), and animation counters advance by it, so they keep real time at
  both rates. A slow effect frame starts a slowdown
  ([[g_main_system_slowdown_frames]]); effect scripts can ask for a minimum
  ([[g_main_system_min_vsync_interval]]).
- **Turn order.** Each unit's [[battle_stats_t]] holds `ct`, `has_turn`,
  `movement_taken`, `action_taken` and `charged_ability_ct` (0xff when nothing
  is charging). The clock's phase is [[g_battle_turn_clock_state]],
  [[g_battle_turn_event]] holds the type of the last event it gave
  ([[battle_turn_event_e]]) and [[g_battle_turn_unit_id]] the unit whose turn
  it is. [[battle_turn_get_status_flags]] sums up what a unit's statuses do to
  its turns ([[battle_turn_status_flags_e]]).
- **The AT list.** 40 [[battle_at_entry_t]] entries that
  [[battle_turn_build_at_list]] rebuilds whenever it is shown, by simulating
  the clock; nothing keeps the list between uses.
- **Window answers.** States that open a window read
  [[g_battle_menu_selected_command]]: 7 goes on, 8 or 0xff (cancel) goes
  back. [[g_battle_action_post_action]] marks that the window or the step has
  finished.
- **The action.** The acting unit's command sits in its
  [[battle_unit_t]] (`command_state`), and [[g_battle_action_phase]]
  steps through a Hamedo (0), the action itself (1) and the reactions
  (2).

## Main flow

1. **Setup.** [[battle_state_run_battle_setup]] loads the map, units, event,
   music and the deployment screen, the turn clock is reset
   (`battle_turn_run_clock(1)`), and [[battle_state_run_game_loop]] runs one
   state handler a frame.
2. **The turn clock.** [[battle_turn_advance]] takes the next event through
   [[battle_turn_take_next_event]], which runs [[battle_turn_run_clock]]
   until a phase yields one:

| Clock phase | What happens | Event |
|---|---|---|
| 0 | Every unit gains its Speed in CT: half under Slow, 1.5 times under Haste, at most 254 | |
| 1 | The unit with the highest CT above 99 gets its turn, after its status upkeep (Reraise, the death counter, Death Sentence, the end of Defend, low Brave); a jumping unit waits at CT 99 | `UNIT_READY`, or `ACTION_RESULT` |
| 2–3 | Charged abilities count down; one that reaches 0 resolves | `ABILITY_READY` |
| 9 | After it, a unit at CT 255 (Quick) gets its turn at once | `UNIT_READY` |
| 4–6 | Timed statuses run out, and each unit's result comes out | `ACTION_RESULT` |
| 10, 19, 15, 17 | Turns that are over end; the turn unit's poison and regen, terrain poison and the end of Transparent come out | `ACTION_RESULT` |
| 13–14 | After an ability, each Mime copies it | `ABILITY_READY`, or `MIME` |

3. **A turn event begins.** Unless an ending or scenario event starts first,
   [[battle_turn_advance]] enters CHANGE_TURN. A unit's turn opens its menus
   (OPEN_ACTION_MENUS, then IDLING_ACTION_MENUS), or AI_COMMAND for a unit
   the player does not control; a due ability goes to ACTION_CAST; a status
   result or a failed Mime copy shows its message in STATUS_EXECUTE.
4. **Move.** UNIT_MOVE (the Move window), SELECT_MOVE_TILE, UNIT_MOVING_SETUP
   ("move here?"), UNIT_MOVING (the walk) and MOVE_CONFIRM_MENU, where "no"
   undoes the move unless it was a teleport. CRYSTAL_LEARN then handles what
   the move left pending, such as a crystal on the tile.
5. **Act.** TARGETING_MESSAGE, TARGETING_RANGE (a tile out of range goes to
   ILLEGAL_RANGE), ABILITY_PREVIEW_HANDLING, CONFIRM_ACTION and
   PRE_ATTACK_ANIMATION, where [[battle_action_execute_ability]] starts
   charging an ability that has a charge time, acts at once, or jumps.
6. **The action.** COMMENCE_ATTACK_PHASE runs the phases through
   [[battle_state_announce_next_ability]]. Each part is announced
   (ANNOUNCE_ABILITY), its extra attack frames and effect file load
   (OPEN_SP2_FILES, START_EFFECT_FILE_OPEN), the effect plays
   (START_ACTION_EXECUTE, ACTION_EXECUTE) and its message shows
   (BATTLE_MESSAGE_DISPLAY). RESUME_ATTACK_PHASE shows the effect messages,
   strikes again for a continued attack or goes on to the next phase. Then
   the action's EXP and JP are granted, and LEARN_ABILITY_ON_HIT reports level
   ups and abilities learned on hit.
7. **After the command.** AFTER_COMMAND waits for the numbers to go;
   CONTINUE_TURN reopens the menus while the turn goes on, goes to Wait
   (WAIT_MENU, WAIT_DIRECTION) when it is over, or, after a due ability, takes
   the next turn event. When the turn ends, [[battle_turn_end]] sets the
   unit's CT: what was left, plus 20 if it did not move and 20 if it did not
   act, at most 60.
8. **Events and the end.** An event script runs in EVENT, and the
   MAP_JUMPING states change the map behind a fade. When a scenario finishes,
   CLOSE_BATTLE fades out and the game flow says what follows: the world map
   (event result 9), the next battle (0xA), a reset (0xB) or the ending
   (0x13).

### States

The iOS names are the ones the FFHacktics wiki records from the iOS release;
the other names come from what each state's code does.

| Value | State | iOS name | What it does |
|---|---|---|---|
| 0x00 | [[battle_state_handle_free_cursor_state\|FREE_CURSOR]] | | The free cursor; its buttons open help, the team highlight, the menus, a move range, the AT list (Start) and the mini menu. Unknown values run it too |
| 0x01 | [[battle_state_handle_free_cursor_help_state\|FREE_CURSOR_HELP]] | | The free cursor's help window |
| 0x02 | [[battle_state_handle_highlight_units_state\|HIGHLIGHT_UNITS]] | | Square held: units tinted by team |
| 0x03 | [[battle_state_handle_open_action_menus_state\|OPEN_ACTION_MENUS]] | | Waits for the camera, then opens the action menu |
| 0x04 | [[battle_state_handle_idling_action_menus_state\|IDLING_ACTION_MENUS]] | | Acts on the command chosen in the action menu |
| 0x05 | [[battle_state_handle_ai_command_state\|AI_COMMAND]] | | An AI unit builds and starts its command |
| 0x06 | [[battle_state_handle_action_help_menu_state\|ACTION_HELP_MENU]] | | The action menu's help window |
| 0x07 | [[battle_state_handle_display_move_area_state\|DISPLAY_MOVE_AREA]] | DisplayModeAreaMode | A unit's move range, from the free cursor |
| 0x08 | [[battle_state_handle_mini_menu_state\|MINI_MENU]] | | The free cursor's mini menu |
| 0x09 | [[battle_state_handle_mini_menu_help_state\|MINI_MENU_HELP]] | | The mini menu's help window |
| 0x0a | [[battle_state_handle_active_turn_state\|ACTIVE_TURN]] | ActiveTurnMode | An AT list entry chosen in the mini menu |
| 0x0b | [[battle_state_handle_status_execute_state\|STATUS_EXECUTE]] | StatusExecuteMode | The message of a status result or a failed Mime copy |
| 0x0c | [[battle_state_handle_unit_move_state\|UNIT_MOVE]] | | The Move command's window |
| 0x0d | [[battle_state_handle_select_move_tile_state\|SELECT_MOVE_TILE]] | | Choose where to move |
| 0x0e | [[battle_state_handle_move_range_exception_state\|MOVE_RANGE_EXCEPTION]] | MoveRangeExceptionMode | A message window before the action menu |
| 0x0f | [[battle_state_handle_illegal_move_menu_state\|ILLEGAL_MOVE_MENU]] | | The unreachable-tile message |
| 0x10 | [[battle_state_handle_unit_moving_setup_state\|UNIT_MOVING_SETUP]] | | "Move here?" |
| 0x11 | [[battle_state_handle_unit_moving_state\|UNIT_MOVING]] | | The walk along the path |
| 0x12 | [[battle_state_handle_move_confirm_menu_state\|MOVE_CONFIRM_MENU]] | | Keep or undo the move |
| 0x13 | [[battle_state_handle_wait_direction_state\|WAIT_DIRECTION]] | | Choose the facing and end the turn |
| 0x14 | [[battle_state_handle_wait_menu_state\|WAIT_MENU]] | | The Wait window |
| 0x15 | [[battle_state_handle_crystal_learn_state\|CRYSTAL_LEARN]] | | The crystal choice and the other results of a move |
| 0x16 | [[battle_state_handle_targeting_message_state\|TARGETING_MESSAGE]] | | "Specify a target", or the cannot-execute message |
| 0x17 | [[battle_state_handle_targeting_range_state\|TARGETING_RANGE]] | | Choose the target tile |
| 0x18 | [[battle_state_handle_illegal_range_state\|ILLEGAL_RANGE]] | | The out-of-range message |
| 0x19 | [[battle_state_handle_ability_preview_state\|ABILITY_PREVIEW_HANDLING]] | | The preview of the ability on its target |
| 0x1a | [[battle_state_handle_ability_preview_help_state\|ABILITY_PREVIEW_HELP]] | | The preview's help window |
| 0x1b | [[battle_state_handle_confirm_action_state\|CONFIRM_ACTION]] | | "Execute?", which also picks how the ability aims |
| 0x1c | [[battle_state_handle_pre_attack_animation_state\|PRE_ATTACK_ANIMATION]] | | The charge animation; then the ability charges, acts or jumps |
| 0x1d | [[battle_state_handle_action_cast_state\|ACTION_CAST]] | ActionCast2Mode | A charged ability comes due |
| 0x1e | [[battle_state_handle_commence_attack_phase_state\|COMMENCE_ATTACK_PHASE]] | | Runs the action's phases, or ends a Jump |
| 0x1f | [[battle_state_handle_target_select_start_state\|TARGET_SELECT_START]] | BattleTargetSelectStartMode | The Auto-Battle target prompt |
| 0x20 | [[battle_state_handle_target_select_state\|TARGET_SELECT]] | BattleTargetSelectMode | Choose the Auto-Battle target |
| 0x21 | [[battle_state_handle_target_select_denied_state\|TARGET_SELECT_DENIED]] | BattleTargetSelectDeneidMode | That unit cannot be the target |
| 0x22 | [[battle_state_handle_target_select_confirm_state\|TARGET_SELECT_CONFIRM]] | BattleTargetSelectConfirmMode | Confirm the target |
| 0x23 | [[battle_state_handle_target_display_start_state\|TARGET_DISPLAY_START]] | BattleTargetDisplayStartMode | The message before showing the target |
| 0x24 | [[battle_state_handle_target_display_state\|TARGET_DISPLAY]] | BattleTargetDisplayMode | The cursor on the Auto-Battle target |
| 0x25 | [[battle_state_handle_after_command_state\|AFTER_COMMAND]] | AfterCommandMode | Waits for the numbers to go |
| 0x26 | [[battle_state_handle_continue_turn_state\|CONTINUE_TURN]] | | Decides what follows the command |
| 0x27 | [[battle_state_handle_change_turn_state\|CHANGE_TURN]] | ChangeTurnMode | Starts the next turn event |
| 0x28 | [[battle_state_handle_learn_ability_on_hit_state\|LEARN_ABILITY_ON_HIT]] | | Level up, job level up and learn-on-hit reports |
| 0x29 | [[battle_state_handle_announce_ability_state\|ANNOUNCE_ABILITY]] | | The ability's name or spell quote |
| 0x2a | [[battle_state_handle_open_sp2_files_state\|OPEN_SP2_FILES]] | | Loads the unit's extra attack frames |
| 0x2b | [[battle_state_handle_start_effect_file_open_state\|START_EFFECT_FILE_OPEN]] | | Waits for the effect file |
| 0x2c | [[battle_state_handle_start_action_execute_state\|START_ACTION_EXECUTE]] | | One frame: plays the effect |
| 0x2d | [[battle_state_handle_action_execute_state\|ACTION_EXECUTE]] | ActionExecuteMode | The strike and its animations |
| 0x2e | [[battle_state_handle_battle_message_display_state\|BATTLE_MESSAGE_DISPLAY]] | | The ability's message |
| 0x2f | [[battle_state_handle_resume_attack_phase_state\|RESUME_ATTACK_PHASE]] | | Effect messages, continued strikes, the next phase, then EXP and JP |
| 0x30 | [[battle_state_handle_deep_dungeon_mesh_load_state\|DEEP_DUNGEON_MESH_LOAD]] | | Deep Dungeon map update, then the next turn |
| 0x31 | [[battle_state_handle_deep_dungeon_mesh_finish_state\|DEEP_DUNGEON_MESH_FINISH]] | | Deep Dungeon map update, then AFTER_COMMAND |
| 0x32 | | MaxStatus | No state; the free cursor's handler would run |
| 0x33 | [[battle_state_handle_effect_state\|EFFECT]] | EffectMode | An effect plays, then the saved state resumes |
| 0x34 | [[battle_state_handle_event_state\|EVENT]] | EventMode | An event script runs |
| 0x35 | [[battle_state_handle_map_jumping_out_state\|MAP_JUMPING_OUT]] | MapJumpingOut | Fade out to change the map |
| 0x36 | [[battle_state_handle_map_init_state\|MAP_INITIALIZE]] | MapInitialize | Load the new map behind a black screen |
| 0x37 | [[battle_state_handle_map_jumping_in_state\|MAP_JUMPING_IN]] | MapJumpingIn | Fade in |
| 0x38 | [[battle_state_handle_change_map_jumping_out_state\|MAP_JUMPING_OUT_2]] | MapJumpingOut2 | The same for an event, which fades back in itself |
| 0x39 | [[battle_state_handle_change_map_init_state\|MAP_INITIALIZE_2]] | MapInitialize2 | Load the new map; the screen stays black for the event |
| 0x3a | [[battle_state_handle_change_map_jumping_in_state\|MAP_JUMPING_IN_2]] | MapJumpingIn2 | The event's fade in, with its script running |
| 0x3b | [[battle_state_handle_close_battle_state\|CLOSE_BATTLE]] | | Fade out and end the battle |

## Where to change

- **CT gain, Haste and Slow:** [[battle_turn_run_clock]] (phase 0). The AT
  list simulates the same rules in [[battle_turn_build_at_list]], and the AI
  runs the clock itself, so a change belongs in both functions.
- **CT after a turn (the 20/20/60 rule):** [[battle_turn_end]].
- **What ends a turn:** [[battle_turn_should_end]] and [[battle_turn_is_over]];
  which statuses stop turns or CT: [[battle_turn_get_status_flags]].
- **Undoing a move:** [[battle_state_handle_move_confirm_menu_state]].
- **The order of Hamedo, the action and reactions:**
  [[battle_state_announce_next_ability]] and
  [[battle_state_handle_resume_attack_phase_state]].
- **How long AI units pause (31 frames):** the AI branches of
  [[battle_state_handle_ai_command_state]],
  [[battle_state_handle_select_move_tile_state]],
  [[battle_state_handle_targeting_range_state]],
  [[battle_state_handle_ability_preview_state]] and
  [[battle_state_handle_wait_direction_state]].
- **Frame rate and slowdown:** [[battle_state_set_vsync_interval]] and
  [[battle_state_sync_frame]].
- **What follows a battle:** the event results in
  [[battle_state_handle_event_state]].
- **Today's limits:** the original build keeps every byte; a change goes behind
  an option into the Developer's Cut, where code can grow and move and
  everything that names it follows (`CODEBASE.md`, "What can change today").

## Quirks and debts

- [[battle_state_handle_change_map_jumping_in_state]] maps event results 9
  and 0xA to the opposite game flows from [[battle_state_handle_event_state]]
  and ignores 0xB and 0x13.
- [[battle_turn_take_next_event]] handles turn event 0x400, which the clock
  never produces.
- [[battle_state_run_game_loop]] leaves its frame loop in the frame that
  enters CLOSE_BATTLE, so [[battle_state_handle_close_battle_state]] never
  runs there.
- [[battle_turn_run_clock]]'s mode 2 has no caller.

## Functions in scope

![[Battle flow scope]]
