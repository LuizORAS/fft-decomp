---
type: guide
mechanics: [Movement]
---

# Movement rules

## Goal

Change what a unit can reach in battle: how far it moves, which heights,
gaps, water and lava it can cross, and how movement abilities, statuses and
the weather change that.

## Where it lives

[[battle_move_store_unit_movement_to_scratchpad]] turns a unit into the rules
the range spread uses: it writes a [[battle_move_pathfind_scratch_t]] at
0x1f800000 and the terrain cost of each surface type. The spread then applies
them ([[battle_move_update_candidate]], [[battle_move_propagate_destination]],
[[battle_move_spread_horizontal_jump]], [[battle_move_check_horizontal_jump]]).
Each rule below names the code that applies it.

| Rule | What the code does | Where |
|---|---|---|
| Move | The range budget, at most 124; Move+1 to +3 are already in the unit's `move` | [[battle_move_store_unit_movement_to_scratchpad]], `main_unit_calculate_move_jump` |
| Jump | Between two tiles the edges may rise or drop by at most Jump levels; a gap of up to Jump / 2 tiles (Jump counted up to 7) can be jumped across; Jump+1 to +3 are already in `jump` | [[battle_move_update_candidate]], [[battle_move_spread_horizontal_jump]] |
| Height of the unit | 6 half levels (3 levels), 4 for Frog and Chicken, 2 more with the Float ability or a rider on top; a tile whose ceiling leaves less room cannot be entered | [[battle_move_store_unit_movement_to_scratchpad]], [[battle_move_update_candidate]] |
| Terrain | Each surface type costs the value in the movement class's row of [[g_main_terrain_movement_cost_tables]]: 0 means the weather severity, 0xff cannot be entered | [[battle_move_store_unit_movement_to_scratchpad]] |
| Weather | Severity 1 for clear, rain or snow, 2 for a storm or snowstorm, 3 for a strong one; maps that ignore the weather count as clear | [[battle_move_get_weather_terrain_cost]], [[battle_map_get_effective_weather]] |
| Ignore Height | No height limit (Jump becomes 0x1f) | [[battle_move_store_unit_movement_to_scratchpad]] |
| Fly | Class Fly, cost row 1 (1 on every surface but the last); no height limit | [[battle_move_store_unit_movement_to_scratchpad]] |
| Teleport, Teleport 2 | The whole map is in range; Teleport fails beyond Move on a roll under 10 per extra tile, Teleport 2 never fails | [[battle_move_set_reachable_tiles]], [[battle_move_roll_teleport_success]] |
| Float (ability) | Class Float, cost row 2; crosses lava; counts as Walk on Water against sinking and drowning | [[battle_move_store_unit_movement_to_scratchpad]] |
| Float (status) | Class Float, like the ability, but without the lava, the sinking rule or the taller body | [[battle_move_store_unit_movement_to_scratchpad]], [[battle_move_get_effective_flags]] |
| Walk on Water (the game's "Move in Water", 0x1f6) | Class water surface, cost row 3; does not sink or drown; stands on the water | [[battle_move_store_unit_movement_to_scratchpad]], [[battle_move_get_water_standing_offset]] |
| Move in Water (the game's "Walk on Water", 0x1f7) | Class water depth one, cost row 3; does not sink or drown; stands one level below the surface | the same |
| Move Underwater | Class underwater, cost row 4; does not drown | [[battle_move_store_unit_movement_to_scratchpad]] |
| Move on Lava | Class lava, cost row 5; crosses lava | [[battle_move_store_unit_movement_to_scratchpad]] |
| Cannot enter water (`BATTLE_MOVEMENT_SET_2_CANNOT_ENTER_WATER`) | Loses Move in Water and Move Underwater; cannot enter water without Walk on Water, Float or Fly, nor stay on it without Walk on Water or Float | [[battle_move_store_unit_movement_to_scratchpad]] |
| Any Weather | Weather-dependent surfaces cost 1 | [[battle_move_store_unit_movement_to_scratchpad]] |
| Any Ground | Every surface that can be entered costs 1 | [[battle_move_store_unit_movement_to_scratchpad]] |
| Frog, Chicken | Smaller body; no Fly, Teleport or Teleport 2 | [[battle_move_store_unit_movement_to_scratchpad]], [[battle_move_get_effective_flags]] |
| Other units | An enemy's tile cannot be entered; allies can be passed (they step aside); a mountable ally can be ridden onto | [[battle_move_set_tile_flags_for_pathfinding]], [[battle_move_displace_unit_at_destination_tile]] |

The class order decides when a unit has several: Fly, then Teleport, Float,
Move on Lava, Walk on Water, Move in Water, Move Underwater, and normal.

## What changes with it

- **The AI** builds its range from the same scratch pad and spread functions
  (`battle_ai_propagate_target_movement`), so it follows a rule change made
  there.
- **Scripted walks** do not: [[battle_move_calculate_pathing]] sets up its own
  generic unit (Move 124, the given Jump, normal costs) and ignores the other
  units.
- **The walk and the sprite** read the unit's effective flags
  ([[battle_move_get_effective_flags]]) and its water handling
  ([[battle_move_get_water_animation_mode]],
  [[battle_move_get_water_standing_offset]]): a new rule for water or flight
  must agree with them, or the unit walks or stands where the range did not
  expect it. Knockback ([[battle_move_init_knockback]],
  `battle_formula_calculate_knockback`) reads the same flags.
- **The terrain cost rows** are data in MAIN; scripted walks read row 0 too.
- **The [[Movement]] page** lists the rest of the movement code.

## What can be done today

Nothing that changes a byte of the original build: `make check` compares every
function with its original hash, so even a one-constant change (a new teleport
chance, another Move cap) fails it. The terrain cost table and the Move-Find
tiles are disc data, which the repository does not hold. In the Developer's
Cut, a change goes behind an option: code under `#ifdef` and a table's bytes in
place through `target/edits.yaml` (`AGENTS.md`, "Changes under an option").
