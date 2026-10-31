---
type: guide
mechanics: [AI]
---

# Changing the AI's decisions

## Goal

Change how computer-controlled units judge abilities, value outcomes, pick
their action, choose where to stand and which mode they play; or change how
Auto-Battle and the statuses that take control behave.

## Where it lives

The [[AI]] page explains the whole decision. Each lever sits in one place:

| What | Where |
|---|---|
| What the AI believes an ability does: whom it targets, whether it touches HP, MP, stats or statuses, what stops it, random use, follow target, Undead reversal, Reflect, trajectories, team limits | the ability's `ai_flags` ([[ability_data_t]] bytes 4-7, [[g_main_ability_data]], MAIN `0x8005ebf0`, 8 bytes per id, disc data) |
| How much each status is worth to the side that has it | [[g_battle_ai_status_priority_weights]] (BATTLE `0x8019f308`, 40 signed 16-bit values by status id, disc data) |
| How HP, MP, equipment and the Golem count in the score | [[battle_ai_simulate_action_and_score]] |
| When an ability is useful, adverse or pointless on a unit: the HP and MP thresholds, the statuses worth adding or removing | [[battle_ai_classify_ability_effect]] |
| Which list entries are usable, and when Protect, Shell, Reflect and Float are dropped; the healer and supporter hints | [[battle_ai_set_ability_considerations]] |
| Which abilities a unit has to consider | [[battle_ai_build_unit_ability_list]], [[battle_ai_add_usable_skillset_abilities]] |
| How the turn's action is drawn from the eight best | [[battle_ai_select_ranked_action]] |
| Where a unit moves: destinations, distance limits, the 3-tile floor for ranged units, the 1-in-7 halving | [[battle_ai_build_movement_scenarios]], [[battle_ai_build_targeted_movement]], [[battle_ai_select_destination]] and its two wrappers |
| How standing on a tile is judged, and how nearby enemies weigh | [[battle_ai_score_reachable_tiles]], [[battle_ai_fill_foe_proximity_grid]] |
| Who counts as a threat, and who can be targeted | [[battle_ai_should_exclude_from_imminent_enemies]], [[battle_ai_is_unit_untargetable]], [[battle_ai_is_unit_out_of_play]] |
| A scenario unit's mode, focus unit and stay-near tile | its ENTD record ([[entd_unit_t]] `ai_flags`, `ai_target_unit_id`, `ai_target_xy`, `ai_flags_2`, disc data), read by [[battle_ai_init_unit_abilities]] |
| The default target, the 45/128 HP retreat and the switch to Save fading life | [[battle_ai_init_target_consideration]] |
| When a Frog, Critical or Don't Act unit runs | [[battle_ai_select_action]], [[battle_ai_update_team_targeting_flags]] |
| What each Auto-Battle mode does | [[battle_ai_select_auto_battle_action]], [[battle_ai_select_fight_for_life_action]], [[battle_ai_select_protect_allies_action]], [[battle_ai_select_save_fading_life_action]], [[battle_ai_select_retreat_action]] |
| Chicken, Blood Suck, Confusion and Berserk | [[battle_ai_select_status_action]], [[battle_ai_select_random_action]] |
| The facing at Wait | [[battle_ai_choose_wait_facing]] |
| How much planning fits in a frame | the `VSync(1)` limits: 440 and 441 horizontal blanks in most steps, 313 while the simulation runs the clock, 325 in the movement propagation ([[battle_ai_is_propagation_budget_spent]]) |

## What changes with it

- **Predictions follow the game.** The AI simulates with the real strike,
  formula, reaction and clock code, so a changed formula or status rule
  changes what the AI expects without touching the AI.
- **Player units too.** Auto-Battle and the statuses that take control run the
  same code for the player's units.
- **Time.** More work per candidate makes enemy turns take more frames; the
  `VSync(1)` limits keep the screen running while the plan waits.
- **The workspace.** [[battle_ai_data_t]] has a fixed size and address
  (`0x8019f3c4`, `0x19c8` bytes); a new field waits for build 3, since the
  build reads the data bytes from the disc at their original size.
- **Mode names.** The Auto-Battle menu shows the game's text for each mode
  ([[battle_auto_battle_setting_e]]); the text is separate from the code.
- **Quirks.** The [[AI]] page lists the `QUIRKS.md` entries that touch the
  AI.

## What can be done today

Nothing that changes a byte: `make check` compares every function with its
original hash, and the weight and ability tables are disc data, which the
repository does not hold. To try an idea, change the game's memory in an
emulator during a battle: `make run` launches PCSX-Redux, and `make map`
writes the symbol maps that name these tables in its debugger (the status
weights at `0x8019f308`, 2 bytes per status id; an ability's AI flags at
`0x8005ebf0` + 8 × id + 4). Code changes wait for build 3.
