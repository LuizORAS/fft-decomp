---
type: mechanic
tier: 1
scope:
  - battle_target
  - battle_effect_set_and_validate_arc_trajectory()
  - battle_effect_check_direct_trajectory_to_target()
  - battle_unit_find_selectable_at_tile()
---

# Targeting

## What it does in the game

Before an action runs, the game shows where it can be aimed (its range), the
player picks a tile with the map cursor, the game shows what that choice would
hit (its area) and a preview, and the player confirms.

- **The range** depends on the command. An ability reaches its `range` in
  steps on both map layers; some keep only the unit's row and column
  (vertical fixed), only tiles within `vertical` levels of the unit's height
  (vertical tolerance), or leave out the unit's own tile. A weapon reaches by
  its kind: a striking weapon the four adjacent tiles within a height band, a
  lunging weapon tiles 1 and 2 steps away in a line, a direct weapon its range
  beyond 2 steps, and an arc weapon tiles at least 3 steps away whose distance,
  less a quarter of the unit's height advantage, is within range. Jump reaches
  the best learned Jump; Item reaches 1 tile (4 with Throw Item or for a Mime),
  and Throw reaches Move tiles.
- **The area** spreads `aoe` steps from the chosen tile within the vertical
  window (the whole map for 0xff). Linear and three-direction abilities hit
  lines from the unit instead; ally-only and enemy-only abilities leave the
  other team's units out; some abilities spare the caster; Random Fire hits
  one random tile of the area, and Moldball Virus does not reach water.
  Arithmeticks hits the units whose CT, level, EXP or height is prime or a
  multiple of 5, 4 or 3.
- **Weapons check their path** when they strike: an arc or a straight shot
  can be stopped by the terrain or by a unit on the way (of two possible arcs,
  the one that meets an enemy of the shooter wins), a lunging weapon by the
  tile or unit between, and a striking weapon needs the two units' heights to
  overlap.
- **An action aims at a tile or at a unit.** An ability that follows its
  target hits wherever the unit has moved to when it resolves; one flagged
  "cannot follow target" keeps the tile.
- **Reflect** sends a spell on from the reflecting unit to another tile.

## Data and structures

- **Panels.** [[g_battle_target_panels]] holds one [[battle_target_panel_t]]
  per map tile (two layers of 256), plus 16 the pathfinding uses: the range
  still left and a mark (a spread frontier, or a tile left to choose).
- **Tile marks.** Three bits of each [[map_tile_t]]'s
  `ceiling_depth_and_marks`: `MAP_TILE_FLAG_MOVE_DESTINATION`,
  `MAP_TILE_FLAG_ABILITY_RANGE` and `MAP_TILE_FLAG_TARGETED`. The map shows
  them as tinted polygons ([[battle_target_tint_marked_tiles]],
  [[battle_tile_mark_mode_e]], [[g_battle_target_tile_mark_modes]]).
- **Ability and weapon data** (MAIN): [[ability_secondary_data_t]] in
  [[g_main_ability_range_data]] (range, aoe, vertical and the
  [[ability_secondary_flags_1_e]], [[ability_secondary_flags_2_e]] and
  [[ability_secondary_flags_4_e]] bits), [[weapon_data_t]] (range and
  `WEAPON_FLAG_*`) and the Jump records [[g_main_jump_ability_data]].
- **The action.** A [[action_command_t]]: unit, skillset, ability,
  targeting type (5 a tile, 6 a unit), target tile or unit; the skillset's
  menu type ([[g_main_action_menu_types_by_skillset]]) picks the targeting
  routine. The acting unit's `ability_preview_phase` keeps the range result.
- **The cursor.** [[g_battle_cursor_x]], [[g_battle_cursor_y]],
  [[g_battle_cursor_z]] (the layer) and [[g_battle_target_cursor_visible]].
- **The targets.** [[g_battle_target_ability_targets_list]] holds the units
  an action hits, nearest first for lines
  ([[g_battle_sort_targets_nearest_first]]).

## Main flow

1. **The range.** IDLING_ACTION_MENUS (and AI_COMMAND) call
   [[battle_target_set_panels_for_action]], which picks the routine by menu
   type:

| Command | Range | Routine |
|---|---|---|
| Ability (Elemental: the surface's ability; Draw Out: the katana's) | `range` steps, then the filters | [[battle_target_set_ability_panels]] |
| Attack, Charge | The weapon's shape | [[battle_target_set_weapon_attack_panels]], [[battle_target_calculate_weapon_range]] |
| Jump | The best learned Jump | [[battle_target_set_jump_ability_panels]] |
| Item | 1, or 4 with Throw Item or for a Mime | [[battle_target_set_item_range_panels]] |
| Throw | Move, without the unit's own tile | the same |

   It returns 0 or 1 when a tile is to be picked (1 when the action keeps the
   tile), 2 when there is nothing to pick, 3 when nothing is in range.
2. **The choice.** TARGETING_MESSAGE and the states that return to the choice
   call [[battle_target_begin_tile_selection]]; in TARGETING_RANGE the cursor
   moves with [[battle_target_move_cursor_by_input]], and the status panels
   follow it.
3. **The area and the preview.** [[battle_target_select_tile]] takes the tile
   and marks what it hits ([[battle_target_mark_action_area]],
   [[battle_target_mark_ability_area]]), then ABILITY_PREVIEW_HANDLING and
   CONFIRM_ACTION follow.
4. **The strike.** When the action resolves,
   [[battle_target_mark_hit_tiles]] marks what it really hits: a weapon-style
   action the one unit its weapon reaches
   ([[battle_target_validate_weapon_target]] and the trajectory tests), the
   others their area again, around the target unit's square if it moved;
   [[battle_target_list_units_on_panels]] lists the units, and
   [[battle_target_store_strike_destination]] keeps where the strike lands.
5. **Reflect.** [[battle_target_apply_reflect]] aims a reflected spell anew.
6. **Each frame**, [[battle_target_update_cursor]] projects the cursor tile
   for the camera and draws the cursor while it is shown.

## Where to change

- **An ability's range, area or vertical window:** the ability's
  [[ability_secondary_data_t]] (data in MAIN); the rules that read them,
  [[battle_target_set_ability_panels]] and
  [[battle_target_mark_ability_area]].
- **Weapon reach:** [[battle_target_calculate_weapon_range]] and its shapes
  ([[battle_target_calculate_strike_lunge_range]],
  [[battle_target_calculate_arc_range]],
  [[battle_target_remove_close_range]]).
- **Whether a weapon hits through units:**
  [[battle_target_validate_weapon_target]],
  [[battle_target_validate_lunging_target]],
  [[battle_effect_set_and_validate_arc_trajectory]] and
  [[battle_effect_check_direct_trajectory_to_target]].
- **Item and Throw range:** [[battle_target_set_panels_for_action]].
- **Jump range:** [[battle_target_set_jump_ability_panels]].
- **Ally-only and enemy-only abilities:**
  [[battle_target_apply_unit_team_eligibility]].
- **Lines:** [[battle_target_build_directional_attack_panels]].
- **Arithmeticks:** [[battle_target_run_calculator]].
- **Random Fire:** [[battle_target_select_random_tile_for_random_fire_abilities]].
- **The cursor and the tile tints:** [[battle_target_move_cursor_by_input]]
  and [[battle_target_tint_marked_tiles]].
- **Today's limits:** every change must keep the original bytes until build 3
  lets code grow or move; everything that names it already follows
  (`CODEBASE.md`, "What can change today").

## Quirks and debts

- [[battle_target_clear_panels_on_untargetable_tiles]] visits only the lower
  layer, so an untargetable upper-layer tile stays in range of a single-tile
  ability.
- [[battle_target_mark_unit_panels_by_team]] never advances its unit pointer;
  no retail ability sets the flags that reach it.
- The range data of items: `battle_action_init_current_ability_strike_data`
  and `battle_action_run_pre_formula_setup` read past the ability table for
  item ids (see `QUIRKS.md`).
- `battle_map_store_selected_tile_coordinates` and
  [[battle_target_build_directional_attack_panels]] keep type debts
  (`QUIRKS.md`, Declaration leads).

## Functions in scope

![[Targeting scope]]
