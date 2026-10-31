---
type: mechanic
tier: 1
scope:
  - battle_move
  - battle_unit_dispatch_distortion_animation()
  - battle_unit_animate_no_distortion()
  - battle_unit_animate_squeeze_out()
  - battle_move_animate_descent_to_ground()
  - battle_unit_animate_teleport_to_destination()
  - battle_unit_animate_slide_back_from_target()
  - battle_unit_move_toward_action_target()
  - battle_unit_clear_distortion_animation_id_10()
  - battle_unit_clear_distortion_animation_id_11()
  - battle_map_calculate_location()
  - battle_map_copy_size_and_tile_data()
  - battle_map_get_tile_data_pointer()
  - battle_map_get_tile_data_ptr_from_battle_id()
  - battle_map_get_unit_tile_by_runtime_id()
  - battle_map_get_unit_tile_from_world_coords()
  - battle_map_get_tile_data_value()
  - battle_map_set_tile_data_value()
  - battle_map_load_move_find_item_data()
  - battle_map_get_move_find_result()
  - battle_map_determine_rare_common_item()
  - battle_map_calculate_move_find_item_flag()
  - battle_map_get_weather()
  - battle_map_get_effective_weather()
  - battle_move_get_weather_terrain_cost()
---

# Movement

## What it does in the game

A unit may move once a turn. Its range spreads out from its tile, and each
tile entered costs its surface type's cost until Move runs out: 1 on most
surfaces, 2 on a few, the weather severity on some (1, or 2 or 3 in a storm
or snowstorm), and others cannot be entered. Between two tiles the edges may rise or drop by
at most Jump levels, a tile whose ceiling leaves less room than the unit's
height is closed, and a gap of up to half of Jump (at most 3) can be jumped
across. Enemies block their tiles; a unit passes through allies, who step
aside, and can ride onto a mountable chocobo. Movement abilities and a few
statuses change these rules (the [[Movement rules]] guide lists them): Fly
and Ignore Height lift the height limit, Teleport reaches any tile but may
fail beyond Move, Float, Walk on Water, Move in Water, Move Underwater and Move
on Lava open water and lava, and Frog and Chicken take away Fly and Teleport.

At the end of a move the unit may pick up a crystal or a treasure chest, find
an item with Move-Find Item (the rare one with a (100 - Brave)% chance, once
per tile), set off a trap, earn the Move-HP Up, Move-MP Up, Move-Get-Exp and
Move-Get-JP rewards, cancel the ability it was charging, and free its mount of
some statuses. Until it is confirmed, a move can be undone, except a teleport.

Two more kinds of movement use the same code: a knockback pushes a target one
tile, and a ground unit that falls more levels than its Jump takes
(levels - Jump) * Max HP / 10 damage, at most 999; and event scenes walk units
along scripted paths that ignore the other units.

## Data and structures

- **The unit.** [[battle_stats_t]] holds `move`, `jump`, the three
  `movement_abilities` bytes ([[battle_unit_movement_set_1_e]],
  [[battle_unit_movement_set_2_e]], [[battle_unit_movement_set_3_e]]) and the
  map position. [[battle_unit_t]] holds the moving unit: map square
  and screen position, `step_phase` (below), the path (`movement_path_count`,
  `movement_path_offset`, `movement_path`), the step in progress
  (`movement_value`) and its `movement` destination, `walk_speed` and
  `step_speed`, the `previous_*` fields an undo restores, the mount pairing,
  and the SEQ-queued motion (`distortion_animation_id`, `_phase`, `_timer`).
- **The map.** [[g_battle_map_tile_data]] holds two layers of 256
  [[map_tile_t]] (surface type, height, water depth and slope half height,
  slope type, ceiling depth, flags), indexed by
  [[battle_map_calculate_location]]: layer * 256 + y * width + x, with the
  size in [[g_battle_map_max_x]] and [[g_battle_map_max_y]].
- **The pathfinding scratch pad** (0x1f800000).
  [[battle_move_store_unit_movement_to_scratchpad]] fills a
  [[battle_move_pathfind_scratch_t]] with the unit's Move (at most 124), Jump,
  movement class ([[battle_move_class_e]]), height and water rules, one
  [[battle_move_record_t]] for each other unit, and the cost of each of the 64
  surface types, taken from the class's row of
  [[g_main_terrain_movement_cost_tables]] (0 normal, 1 Fly or Teleport, 2
  Float, 3 Walk on Water or Move in Water, 4 Move Underwater, 5 Move on Lava).
- **The spread.** The 528 [[g_battle_target_panels]] (512 tiles and the 16
  unit records) hold each tile's remaining range and trace marks, and each
  tile has frontier flags (`FRONTIER_FLAG_*`).
- **The path.** [[g_battle_move_path]]: a step count, then one
  [[battle_move_step_bits_e]] byte per step (distance - 1, climb at the start
  or the end, onto a unit, upper layer, direction). A teleport stores 0xfe, x,
  y and layer instead, and a failed one 0xff.
- **SEQ-queued motions.** A unit's SEQ animation queues a motion (opcode
  0xc1) and waits for it (0xc0); [[g_battle_unit_distortion_animation_handlers]]
  holds one handler per [[battle_distortion_animation_e]] id (table below).
- **After the move.** [[g_battle_move_post_move_events]] holds the pending
  [[battle_move_post_event_flags_e]] bits. Each map has four Move-Find tiles
  ([[g_battle_current_map_move_find_item_data]]), whose found items stay in
  [[g_main_item_location_flags]]; [[battle_map_get_move_find_result]] reports
  a tile as a [[battle_move_find_result_data_t]].
- **Knockback.** `g_battle_action_current_ability.knockback_flags` (0x80 pending; kind 1
  ground, 2 flier) and `knockback_fall_height` (in half levels).

## Main flow

1. **The range.** UNIT_MOVE opens the range through
   [[battle_move_set_reachable_tiles]]:
   [[battle_move_store_unit_movement_to_scratchpad]],
   [[battle_move_set_tile_flags_for_pathfinding]], then one spread pass per
   point of Move ([[battle_move_spread_to_adjacent_tiles]],
   [[battle_move_spread_horizontal_jump]], which test each step with
   [[battle_move_propagate_destination]] and
   [[battle_move_update_candidate]]), and
   [[battle_move_set_reachable_tile_flags]]. A teleporting unit gets the whole
   map. The states that cancel back to the range open it again, the free
   cursor's move range uses the same function, and the AI spreads with its own
   time-sliced copy, `battle_ai_propagate_target_movement`.
2. **The path.** SELECT_MOVE_TILE calls [[battle_move_build_path_to_tile]]:
   it traces the remaining ranges back from the chosen tile and
   [[battle_move_encode_path_steps]] writes the steps; a teleport rolls
   [[battle_move_roll_teleport_success]] instead.
3. **The walk.** UNIT_MOVING runs [[battle_move_update_path_step]] once a
   frame. It starts each step (a walk, a climb, a hop, a jump, a fly or a float
   step) and runs the handler of the step phase:

| Phases | Movement | Handlers |
|---|---|---|
| 0x01–0x08 | Walk: odd from the center to the edge, even from the edge to the center | [[battle_move_update_walking_step_at_tile_edge]], [[battle_move_update_unit_step_to_destination_tile_center]] |
| 0x09–0x10 | The same at climb speed | the same |
| 0x11–0x20 | Jumps, four per direction: crouch, rise, fall, landing | [[battle_move_update_path_step]], [[battle_move_update_airborne_ascent_phase]], [[battle_move_finish_unit_step_at_tile_edge]], [[battle_move_update_landing_phase]] |
| 0x22–0x29 | Fly: even to the exit edge, odd to the center | [[battle_move_update_fly_step_to_exit_edge]], [[battle_move_update_fly_step_to_center]] |
| 0x2b–0x39 | Float, four per direction: to the exit edge, to the center, climb jump | [[battle_move_update_float_step_to_current_tile_exit_edge]], [[battle_move_update_float_step_to_destination_tile_center]], [[battle_move_update_float_jump_to_entry_edge]] |
| 0x3b | Reset the coordinates | `battle_move_update_teleport_start` |
| 0x3c | End of the walk | [[battle_move_finalize_path_after_animation]] |

   At each tile edge [[battle_move_update_walking_step_at_tile_edge]] chooses a
   plain walk, a climb hop or a jump from the two edge heights. A unit standing
   on the path is pushed aside ([[battle_move_displace_unit_at_destination_tile]],
   [[battle_move_displace_overlapping_unit]]).
4. **Keep or undo.** MOVE_CONFIRM_MENU either undoes the move
   ([[battle_move_undo_unit_move]]) or keeps it, and
   `battle_unit_copy_rider_data_to_mount` then calls
   [[battle_move_start_post_move_events]].
5. **After the move.** [[battle_move_get_post_move_events]] collects the
   events, and the CRYSTAL_LEARN state plays them one at a time through
   [[battle_move_start_next_post_move_event]].
6. **Knockback.** `battle_formula_calculate_knockback` picks the tile
   ([[battle_move_check_knockback_destination]]);
   `battle_action_run_pre_formula_setup` turns it into the action result
   ([[battle_move_apply_knockback]]); the target's animation starts
   the push ([[battle_move_start_knockback]], [[battle_move_init_knockback]]),
   and ACTION_EXECUTE runs [[battle_move_update_knockback_step]] for each
   target.
7. **Event walks.** A script's walk calls [[battle_move_start_unit_walk_to]],
   whose path comes from [[battle_move_calculate_pathing]] for a generic unit,
   and the event states advance every walking unit with
   [[battle_move_update_all_walking_units]].
8. **SEQ-queued motions.** Each frame
   [[battle_unit_dispatch_distortion_animation]] runs the unit's queued motion:

| Id | Motion | Handler |
|---|---|---|
| 0 | None (skipped) | [[battle_unit_animate_no_distortion]] |
| 1 | Squeeze out (no SEQ uses it) | [[battle_unit_animate_squeeze_out]] |
| 2 | Rise three levels | [[battle_move_animate_fixed_rise]] |
| 3 | Descend to the ground | [[battle_move_animate_descent_to_ground]] |
| 4 | Glide to the action target | [[battle_move_glide_to_action_target_with_height_change]] |
| 5 | Return to the unit's tile | [[battle_move_step_unit_to_map_tile_center_with_height_change]] |
| 6 | Jump toward the target | [[battle_move_animate_jump_arc_to_target]] |
| 7 | Jump back to the unit's tile | [[battle_move_animate_jump_arc_to_own_tile]] |
| 8 | Glide to the action target, level | [[battle_move_glide_to_action_target_no_height_change]] |
| 9 | Return to the tile, level (no SEQ uses it) | [[battle_move_step_unit_to_map_tile_center_no_height_change]] |
| 0xa, 0xb | Stop (no SEQ uses them) | [[battle_unit_clear_distortion_animation_id_10]], [[battle_unit_clear_distortion_animation_id_11]] |
| 0xc | Jump up | [[battle_move_animate_jump_start]] |
| 0xd | Fall onto the target tile | [[battle_move_animate_fall_to_target_tile]] |
| 0xe | Teleport | [[battle_unit_animate_teleport_to_destination]] |
| 0xf | Slide back from the target | [[battle_unit_animate_slide_back_from_target]] |
| 0x10 | Jump back to the tile, with a fade | [[battle_move_animate_jump_arc_to_own_tile_with_fade]] |
| 0x11 | Jump up, with sound | [[battle_move_animate_jump_rise_with_sfx]] |

## Where to change

- **What a movement ability or status allows (Move, Jump, classes, water,
  Frog and Chicken):** [[battle_move_store_unit_movement_to_scratchpad]]; the
  player's range and the AI both read it. The walk itself reads
  [[battle_move_get_effective_flags]]. The [[Movement rules]] guide lists
  each rule.
- **Terrain costs:** the rows of [[g_main_terrain_movement_cost_tables]] (data
  in MAIN); the weather's share, [[battle_move_get_weather_terrain_cost]].
- **Height, ceilings and gaps:** [[battle_move_update_candidate]] and
  [[battle_move_propagate_destination]] (Jump against the edge heights, the
  unit's height against the ceilings), [[battle_move_check_horizontal_jump]]
  and [[battle_move_spread_horizontal_jump]] (gaps).
- **Which tiles block, and units on them:**
  [[battle_move_set_tile_flags_for_pathfinding]].
- **The teleport chance:** [[battle_move_roll_teleport_success]].
- **Scripted walks:** [[battle_move_calculate_pathing]] (its generic unit's
  Move, Jump and costs).
- **Knockback damage:** [[battle_move_apply_knockback]]; where the
  target lands: `battle_formula_calculate_knockback` and
  [[battle_move_check_knockback_destination]].
- **Move-Find Item:** the odds in [[battle_map_determine_rare_common_item]];
  the items and traps of each map are data
  ([[g_battle_map_move_find_item_data]]).
- **The rewards after a move:** [[battle_move_get_post_move_events]] and
  `battle_action_init_movement_ability_benefit`.
- **Undoing a move:** `battle_state_handle_move_confirm_menu_state` and
  [[battle_move_undo_unit_move]].
- **Today's limits:** every change must keep the original bytes until build 3
  lets code grow or move; everything that names it already follows
  (`CODEBASE.md`, "What can change today").

## Quirks and debts

- [[battle_move_update_walking_step_at_tile_edge]] adds the destination's
  water standing offset to the current edge height, so a step up onto deep
  water can take a plain walk where a hop or a jump is due.
- [[battle_move_animate_fall_to_target_tile]] writes map Y into `real_z` on its
  event path, and passes an extra coordinate buffer to a one-argument callee.
- [[battle_move_calculate_walkto_pathing]] has no return statement; its caller
  uses the path left in `$v0`.
- [[battle_map_load_gns_and_move_find_items]] has no return value for a map
  with no GNS sector.
- [[battle_map_get_tile_data_ptr_from_battle_id]] and
  [[battle_map_get_unit_tile_by_runtime_id]] do not check the unit lookup.
- Post-move event 0x200 (`SOURCE_DISPLAY`) is handled but never set.
- `battle_unit_t.movement_value` is a plain `u8` that holds a packed
  [[battle_move_step_bits_e]] step; its readers shift and mask it by hand.
- Only the first 64 bytes of [[g_main_item_location_flags]] (the Move-Find
  found flags) have a known meaning.

## Functions in scope

![[Movement scope]]
