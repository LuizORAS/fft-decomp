---
type: mechanic
tier: 1
scope:
  - battle_ai
  - battle_mem_copy_bytes()
  - battle_mem_copy_u16_values()
  - battle_mem_zero_32bit_values()
---

# AI

## What it does in the game

The AI plays every unit the player does not control: enemies, guests, units
under Auto-Battle and units that a status takes over (Chicken, Blood Suck,
Confusion, Berserk). At the start of such a unit's turn it plans where to
move, what to do and where to face, and it plans again once the unit has
acted.

- **It simulates with the game's rules.** For each candidate it runs the real
  strike code (formulas, hit chances, reactions) and the turn clock until
  every charged action has landed, then scores the state that results.
- **It looks one turn ahead.** Other units' turns that come before a charge
  lands are noted (they may move away), but what those units would do is not
  simulated.
- **Each unit plans alone.** Nothing coordinates the plans of different
  units.
- **Its choice is partly random.** The eight best actions are tried in order,
  each with its hit chance; random-use abilities, the reach toward a far
  target (halved one time in seven outside the player team) and Praise,
  Accumulate and Yell (useful half the time) add more chance.
- **It works within the frame.** Planning stops when the frame's time is
  spent and resumes on the next frame from saved state, so a long plan takes
  several frames.

Who decides, in order ([[battle_ai_select_action]]):

| Case | What runs |
|---|---|
| Chicken, Blood Suck, Confusion or Berserk | [[battle_ai_select_status_action]]: run, bite, act at random or attack the nearest enemy |
| Frog or Critical with an ally able to help, or Don't Act with a focus target | [[battle_ai_select_retreat_action]]: head for a healer or a crystal |
| An Auto-Battle mode | [[battle_ai_select_auto_battle_action]]: Fight for life, Protect allies, Save fading life, Run like a rabbit |
| Anything else | [[battle_ai_select_general_action]]: the general search |

A player unit takes its Auto-Battle setting from the menu. Any other unit
takes a mode from its scenario (ENTD) AI flags
([[battle_ai_init_unit_abilities]]): the special-behavior bit with Conserve
CT runs like a rabbit, and with Focus and Aggressive it fights for life
against its focus unit, or protects it when it is on the same team. That mode
lasts while the unit is not Invited and its focus unit is in play
([[battle_ai_init_target_consideration]]). Without one, guests seek a healer
or a crystal below 45/128 HP, supporters of other teams switch to Save fading
life while their team can still help, and the rest target the nearest
eligible enemy.

## Data and structures

- **The workspace:** one shared [[battle_ai_data_t]], [[g_battle_ai_data_base]]
  at `0x8019f3c4` (`0x19c8` bytes). It holds the actions being compared, the
  tile grids and masks, the decision records, the ability lists and the
  resume state of every step that can pause.
- **Ability lists:** 34 entries per unit ([[battle_ai_ability_entry_t]],
  `BATTLE_AI_ABILITY_LIST_LENGTH`), built from its skillsets, weapons and
  items ([[battle_ai_build_unit_ability_list]]); units 0-15 get one. Two
  fixed entries serve Blood Suck and Frog Attack.
- **What an ability is for:** the 32 AI flags in each ability's record
  ([[ability_data_t]] `ai_flags`, disc data, [[battle_ai_ability_flags_t]]):
  whom it targets, whether it affects HP, MP, stats or statuses, what stops
  it (Silence, Faith, evasion), random use, follow target, Undead reversal,
  Reflect, trajectories, map-tile targeting, Monster Skill, team limits.
- **Decision records:** one [[battle_ai_unit_decision_t]] per unit (16
  bytes): the decision flags (dead, petrified, jumping, the scenario's
  special behavior, aggressive, stay near, focus target, worth targeting),
  the main target, targeting flags 1 (ranged, healer, supporter, reflected
  action) and 2 (HP and MP state, Run like a rabbit, untargetable, dead), the
  enemy flag, the MP costs, the shares of its abilities that cost MP, that
  Silence stops and that can be evaded, and its reach toward enemies and
  allies.
- **Behavior from the unit:** the Auto-Battle setting and target
  ([[battle_stats_t]] `auto_battle_setting`, `auto_battle_target`,
  [[battle_auto_battle_setting_e]], named after the game's menu), and the
  scenario's AI fields copied from [[entd_unit_t]]: the stay-near tile, the
  AI flags, the focus unit and the targeting flags (with Conserve CT).
- **Actions:** [[battle_ai_action_data_t]] (32 bytes): act before or after
  moving, the destination, the command (skillset, ability, Math Skill type
  and multiplier, item, tile or unit target), the score (`priority`), the hit
  chance and the facing for Wait. The workspace keeps the current one, the
  best of the current ability, eight ranked ones, the "only move" baseline
  and the selected one.
- **Movement:** three scenarios of reachable tiles (`reachable_tiles`, a
  bit per tile, 16 per row) with a destination each (`candidate_coords`),
  and per-tile grids: movement cost from the decision target
  (`tile_target_distance`), nearby enemies (`tile_foe_proximity`) and the
  negated score of standing there (`tile_priority`).
- **Ability search:** the origin tiles to act from (`origin_tiles`), the
  target tiles to aim at (`target_tiles`) and a scratch mask for ranges
  (`targetable_tiles`).
- **Tables (BATTLE data):** the status weights
  ([[g_battle_ai_status_priority_weights]], 40 values at `0x8019f308`), the
  range-row templates, the four neighbor offsets and the statuses Protect
  allies tries to remove ([[g_battle_ai_recovery_status_ids]]: Dead,
  Petrify, Blood Suck).
- **Snapshots:** every planned unit's state before a simulation
  ([[g_battle_ai_unit_snapshots]], [[battle_ai_extended_snapshot_t]]).
- **The result:** a [[battle_ai_command_t]] (MOVE, ACT or END_TURN), kept in
  [[g_battle_ai_command_history]].

## Main flow

1. **Battle setup:** [[battle_ai_init_workspace]] numbers the units and builds
   their ability lists.
2. **The unit's turn:** state `0x05` ([[battle_state_handle_ai_command_state]],
   the [[Battle flow]] page) calls [[battle_ai_build_command]] every frame
   until it returns a command. The first call plans:
   [[battle_ai_begin_simulation]], [[battle_ai_refresh_unit_decision_flags]],
   [[battle_ai_set_ability_considerations]] (usable abilities and the unit's
   hints), [[battle_ai_build_reachable_tiles]],
   [[battle_ai_update_team_targeting_flags]],
   [[battle_ai_init_target_consideration]] (mode and main target), then
   [[battle_ai_select_action]].
3. **Movement scenarios** ([[battle_ai_build_movement_scenarios]], or
   [[battle_ai_build_targeted_movement]] for Fight for life and Protect
   allies): the cost from the decision target to every tile
   ([[battle_ai_fill_target_distance_grid]]), the score of standing on each
   tile ([[battle_ai_score_reachable_tiles]]), the tiles to act from and a
   destination ([[battle_ai_select_destination]]).
4. **Baseline:** only moving to the destination
   ([[battle_ai_set_movement_baseline]]); a charged action is kept if that
   scores better ([[battle_ai_evaluate_charged_action]]).
5. **Every usable ability** ([[battle_ai_evaluate_ability_list]],
   [[battle_ai_evaluate_ability_outcome]]): the units it is useful on
   ([[battle_ai_keep_useful_targets]], [[battle_ai_classify_ability_effect]]),
   then bounces off a Reflect unit, areas around the unit itself, Math Skill
   combinations, and the origin search
   ([[battle_ai_build_origin_and_target_tiles]],
   [[battle_ai_evaluate_origins]], [[battle_ai_evaluate_target_tiles]]).
6. **Every candidate** is simulated and scored
   ([[battle_ai_score_current_action]],
   [[battle_ai_simulate_action_and_score]]); the best use of each ability
   joins the eight ranked actions ([[battle_ai_insert_ranked_action]]) if it
   beats only moving.
7. **The choice:** [[battle_ai_select_ranked_action]] takes a ranked action by
   its hit chance; with none, the unit only moves
   ([[battle_ai_select_move_only_action]]).
8. **The commands:** MOVE first unless the action is used from the unit's own
   tile, then ACT, then END_TURN with a facing
   ([[battle_ai_choose_wait_facing]]); [[battle_ai_end_simulation]] closes the
   plan.

### How a state is scored

[[battle_ai_simulate_action_and_score]] sums, over every unit in the plan,
with enemies counted negatively:

- **HP:** HP × 128 / max HP.
- **Statuses:** the weight of each status the unit has. Darkness and Silence
  are scaled by how much of the unit's kit they block; a Confused, Blood
  Sucked or Charmed unit counts no helpful or disabling status.
- **Equipment:** 51 for each slot still holding what it held before.
- **MP:** MP × 64 / max MP, halved below its cheapest ability, times its
  share of MP abilities.
- **Golem:** its team's Golem HP over the average max HP.

The action itself adds a little: +2 for picking up a crystal or treasure,
and for a stat ability +3 per hit on the intended side, −4 per hit on the
other (−1 for other abilities). Throwing anything but a shuriken or a ball
halves the hit chance the choice rolls.

| Status weights | |
|---|---|
| Invite −231 | Crystal, Dead, Treasure −192 |
| Petrify, Blood Suck −116 | Death Sentence −103 |
| Silence, Stop −90 | Darkness, Confusion, Charm, Don't Act −64 |
| Frog −52 | Undead, Berserk, Slow, Sleep, Don't Move −39 |
| Critical −32 | Chicken, Poison −26 |
| Oil, Innocent −7 | Faith +6 |
| Float +12 | Haste +19 |
| Regen, Protect, Shell, Reflect +25 | Transparent +38 |
| Reraise +51 | Wall +64 |

The other statuses weigh 0.

## Where to change

- **What the AI thinks an ability does:** the ability's `ai_flags` (disc
  data); the [[Abilities]] page explains the records.
- **How much a status matters:** [[g_battle_ai_status_priority_weights]]
  (BATTLE data).
- **Whether an ability helps or hurts a given unit:**
  [[battle_ai_classify_ability_effect]].
- **How a state is valued:** [[battle_ai_simulate_action_and_score]].
- **How the turn's action is picked among the best:**
  [[battle_ai_select_ranked_action]].
- **Where units move and stand:** [[battle_ai_build_movement_scenarios]],
  [[battle_ai_select_destination]], [[battle_ai_fill_foe_proximity_grid]].
- **Modes and targets:** [[battle_ai_init_target_consideration]],
  [[battle_ai_select_auto_battle_action]] and the `battle_ai_select_*_action`
  functions; a scenario unit's behavior bits are in its ENTD record (disc
  data).
- **Which abilities a unit considers:** [[battle_ai_build_unit_ability_list]]
  and [[battle_ai_set_ability_considerations]].
- The [[Changing the AI's decisions]] guide lists every lever.
- **Today's limits:** every change must keep the original bytes until build 3
  lets code grow or move; everything that names it already follows
  (`CODEBASE.md`, "What can change today").

## Quirks and debts

- **Wrong bits:** the CT estimate tests Monster Skill and Defend where Short
  Charge and Non-Charge were presumably meant
  ([[battle_ai_load_ability_entry]]), and Protect allies tests Blood Suck
  with the Jump bit ([[battle_ai_evaluate_status_cancellation]]).
- **Stale or lost data:** the Faith test reads the ability considered last
  ([[battle_ai_add_usable_skillset_abilities]]), a Jump skillset with no
  Level Jump loses its Jump entry, and a unit holding two weapon kinds gets
  no target team for Attack.
- **Masks:** the Monster Skill mask loses two neighbors and the upper level,
  and range radii 8-15 read past their templates.
- **Unreachable or ineffective code:** Protect allies' fallback to Run like a
  rabbit never runs, and the 10-combination test of the origin search has no
  effect.
- **Targeting oddities:** a Transparent enemy rider is targeted only when its
  mount is Transparent too, and a Wait facing toward an enemy on the unit's
  own tile reads past the four directions.
- **Ports:** the reflected search's resume relies on the caller's `$s2`, the
  Jump CT estimate at battle setup reads through a NULL acting unit, and the
  Half of MP rounding differs between the menu and the AI.

`QUIRKS.md` has each entry with its file.

## Functions in scope

![[AI scope]]
