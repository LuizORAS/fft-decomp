---
type: mechanic
tier: 2
scope:
  - main_unit
  - main_party
  - main_entd
  - main_job
  - world_job
  - bunit_job
  - jobstts_job
  - battle_unit_apply_action_rewards()
  - battle_unit_update_attacker_earned_experience()
  - battle_unit_store_jp_and_calculate_unlocked_jobs()
  - battle_unit_load_job_level()
  - battle_unit_build_gained_exp_jp_level_job_level()
  - battle_unit_calculate_move_exp_jp_up_to_earn()
  - battle_unit_apply_level_up_down_ability()
  - world_unit_update_monster_breeding()
  - world_unit_get_zodiac_for_date()
  - job_calculate_and_store_unlocks()
  - job_find_first_for_skillset()
  - job_get_generic_index()
  - job_get_skillset()
  - job_is_special_monster()
  - menu_job_calculate_next_level_jp_requirement()
  - menu_job_get_current_level()
  - menu_job_get_next_level_jp_requirement()
---

# Units and Jobs

## What it does in the game

A unit is a record of who it is (identity, job, sex, birthday and zodiac
sign), what it can do (job levels, JP, unlocked jobs, learned abilities,
skillsets, reaction, support and movement abilities) and how strong it is
(level, EXP, Brave, Faith and five raw stats).

- **The roster.** [[g_main_party_units]] holds 20 records: 16 ordinary slots
  and 4 guest slots (16-19). A battle unit is built from a roster record
  ([[main_unit_init_from_party_slot]]) or from an ENTD record
  ([[main_unit_init_from_entd]]), and the roster takes it back after the
  battle ([[main_party_save_unit]]).
- **Raw stats and growth.** HP, MP, Speed, PA and MA are each a 24-bit raw
  value. The stat a player sees is raw * job multiplier / 1638400, at least
  1; HP and MP stop at 999, Speed at 50, PA and MA at 99
  ([[main_unit_calculate_actual_stats]]). Each level adds
  raw / (job growth + level) to every raw stat; a lower growth number means
  faster growth ([[main_unit_apply_level_growth]]).
- **Levels.** A unit levels up when its EXP reaches 100: EXP goes back to 0,
  the excess is lost, and at level 99 EXP stays at 99
  ([[main_unit_check_level_up]]). An action levels a unit up once at most.
- **EXP.** The acting unit earns the highest award among its targets: 10
  for a hit that changed the target, 20 divided by the target's KO count for
  a KO (20, then 10, then 6), plus the target's level minus its own, at least
  1, doubled by Gained EXP Up, at most 99
  ([[battle_unit_update_attacker_earned_experience]]).
- **JP.** An action that earns EXP earns 8 + job level * 2 + level / 4 JP
  in the actor's current job; Gained JP Up raises it by half. Every other
  unit whose starting team is the actor's current team gets a quarter of the
  plain amount in the same job (Charm sends it to the charmer's side).
  Current and total JP stop at 9999
  ([[battle_unit_apply_action_rewards]],
  [[battle_unit_store_jp_and_calculate_unlocked_jobs]]). Propositions pay JP
  the same way out of battle ([[main_job_add_proposition_jp]]).
- **Job levels.** A job's level is how many of these total-JP thresholds it
  has reached ([[main_job_calculate_level]]):

  | Level | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 |
  |---|---|---|---|---|---|---|---|---|
  | Total JP | 100 | 200 | 350 | 550 | 800 | 1150 | 1550 | 2100 |

- **Unlocking jobs.** Squire is always unlocked; every other generic job
  unlocks when each job level meets its row of
  [[g_main_job_unlock_requirements]] ([[main_job_calculate_unlocked]]). A
  female cannot take Bard and a male cannot take Dancer, and the one they
  cannot take never blocks Mime. Monsters unlock nothing.

  | Job | Needs |
  |---|---|
  | Chemist | nothing |
  | Knight, Archer | Squire 2 |
  | Monk | Squire 2, Knight 2 |
  | Priest, Wizard | Chemist 2 |
  | Time Mage | Chemist 2, Wizard 2 |
  | Summoner | Chemist 2, Wizard 2, Time Mage 2 |
  | Thief | Squire 2, Archer 2 |
  | Oracle | Chemist 2, Priest 2 |
  | Mediator | Chemist 2, Priest 2, Oracle 2 |
  | Geomancer | Squire 2, Knight 2, Monk 3 |
  | Lancer | Squire 2, Archer 2, Thief 3 |
  | Samurai | Squire 2, Knight 3, Archer 2, Monk 4, Thief 3, Lancer 2 |
  | Ninja | Squire 2, Knight 2, Archer 3, Monk 3, Thief 4, Geomancer 2 |
  | Calculator | Chemist 2, Priest 4, Wizard 4, Time Mage 3, Oracle 3 |
  | Bard | Chemist 2, Priest 2, Wizard 2, Time Mage 2, Summoner 4, Mediator 4, Oracle 2 |
  | Dancer | Squire 2, Knight 2, Archer 2, Monk 3, Thief 3, Geomancer 4, Lancer 4 |
  | Mime | Squire 8, Chemist 8, Knight 2, Archer 2, Monk 3, Priest 2, Wizard 2, Time Mage 2, Summoner 4, Thief 3, Mediator 4, Oracle 2, Geomancer 4, Lancer 4 |

- **New recruits.** A recruit starts at level 1 with 0 EXP, Brave and Faith
  rolled in 40..70 (Ramza 70 and 70), a random birthday, 100..199 JP and
  level 1 in every job but the one its sex cannot take, a Leather Hat,
  Clothes and a Broad Sword (male) or Dagger (female), and a name no roster
  member of its kind has ([[main_unit_generate_out_of_battle]]). Its raw
  stats are (seed + random(0..variance)) << 14:

  | Type | HP | MP | Speed | PA | MA |
  |---|---|---|---|---|---|
  | Male | 30-32 | 14-15 | 6 | 5 | 4 |
  | Female | 28-30 | 15-16 | 6 | 4 | 5 |
  | Ramza | 30-32 | 15-16 | 6 | 5 | 5 |
  | Monster | 35-38 | 8-9 | 5 | 5-6 | 5-6 |

- **ENTD units.** Story and random-battle units come from the ENTD files
  ([[main_entd_open_file]]). A record's level 0 or 0xfe is random in
  [L - L / 8, L] for the roster's highest level L, and 100 + n means L + n;
  Brave and Faith above 100 roll 45..74; equipment marked random takes the
  best items the unit's level allows ([[main_unit_calculate_random_equipment]]);
  reaction, support and movement marked random come from what the unit has
  learned ([[main_unit_calculate_rsm]]); a generated unit's raw stats are
  rolled like a recruit's and grown to its level.
- **Eggs and breeding.** Each day, with a monster in the roster, there is a
  32 in 256 chance of an egg of that monster's family (6 rare, 10 uncommon,
  16 common); it hatches in a few days at the level of a random roster member
  ([[world_unit_update_monster_breeding]], [[main_party_create_monster_egg]]).

## Data and structures

| Record | What it holds |
|---|---|
| [[battle_stats_t]] | A unit in battle (and in WORLD's formation): identity, job, statuses, stats, equipment, abilities, AI data |
| [[party_unit_t]] | A roster record (0x100 bytes, 0xe0 saved): what survives between battles |
| [[entd_unit_t]] | An ENTD record (0x28 bytes): a story or random-battle unit, with random markers |
| [[job_data_t]] | A job (48 bytes, [[g_main_job_data]] at MAIN `0x800610b8`, read through [[g_main_job_data_pointer]]): skillset, innate abilities, equippable types, growths and multipliers, Move, Jump, class evade, status sets, elemental affinities, graphics |
| [[g_main_job_level_jp_requirements]] | The eight job-level thresholds (MAIN `0x80066184`) |
| [[g_main_job_unlock_requirements]] | The unlock table (MAIN `0x800660c4`, 19 rows of 10 packed bytes) |
| [[g_main_unit_generation_base_data]], [[g_main_unit_generation_raw_stat_variance]] | A recruit's raw-stat seeds, variances and equipment by type (MAIN `0x8005e90c`, `0x8005e93c`) |

Job levels are packed two jobs a byte (the first in the high nibble), the
unlocked jobs are a 24-bit bitset with Squire at `0x800000`, and the learned
abilities are three bytes per job, read from the top bit down. The roster
keeps each job's JP as a byte pair.

## Main flow

1. **Battle start.** [[battle_unit_load_entd_units]] builds each ENTD unit
   ([[main_unit_init_from_entd]]): identity ([[main_unit_calculate_entd_data]];
   a party member is built from the roster instead), jobs and skillsets
   ([[main_unit_calculate_jobs_and_skillsets_from_entd]]), abilities
   ([[main_unit_calculate_abilities]]), raw stats, equipment
   ([[main_unit_calculate_entd_equipment]]), the equipment's bonuses, Move,
   Jump and names. [[battle_unit_init_deployed_units_data]] builds the units
   the player deploys from the roster.
2. **During battle.** Each action's rewards go through
   [[battle_unit_apply_action_rewards]]; equipment changes and level changes
   re-derive the unit ([[main_unit_refresh_stats_and_statuses]]).
3. **After battle.** The units are saved back into the roster
   ([[main_party_save_unit]]).
4. **On the world map.** The formation builds its records from the roster,
   recalculates them on job or equipment changes
   ([[main_unit_update_formation_stats]]) and saves them back; the calendar
   breeds monsters; propositions pay JP.

## Where to change

- **Stat growth or caps:** [[main_unit_apply_level_growth]] and
  [[main_unit_calculate_actual_stats]] (battle), [[main_party_raise_unit_level]]
  (an egg's levels); the growths and multipliers are job data.
- **EXP needed per level:** [[main_unit_check_level_up]].
- **EXP or JP awards:** [[battle_unit_update_attacker_earned_experience]],
  [[battle_unit_apply_action_rewards]] (the formula and the quarter shared with the team).
- **Job-level thresholds or unlock requirements:** the two tables above (disc
  data); [[main_job_calculate_unlocked]] for the Bard/Dancer/Mime rule.
- **New recruits:** [[main_unit_generate_out_of_battle]] and the generation
  tables.
- **Random ENTD units:** [[main_unit_calculate_entd_data]] (level, Brave,
  Faith), [[main_unit_calculate_random_equipment]],
  [[main_unit_calculate_rsm]], [[main_unit_learn_job_abilities]].

[[Changing jobs and growth]] walks through these changes.

## Quirks and debts

- [[main_unit_apply_level_growth]] calls `rand()` and discards the result.
- An ability level-up divides each raw stat by growth + the new level; an EXP
  level-up and a generated unit's replay divide by growth + the old level
  ([[battle_unit_apply_level_up_down_ability]]).
- A level-1 target of a level-down is clamped back to level 1 and still loses
  its growth. Only the Degenerator trap reaches it, as Level Blast misses
  there; in the Developer's Cut the trap misses too
  ([[battle_action_apply_tile_trap]]).
- [[main_unit_learn_job_abilities]] sets a bought ability's bit twice.
- [[main_unit_calculate_random_equipment]]: throwing and bomb item types index
  past the equipment-category bytes.
- [[main_unit_calculate_entd_equipment]] tests Equip Shield where Equip Armor
  was meant, for the random body armor. The Developer's Cut tests Equip Armor.
- [[main_unit_copy_job_data]] gives the immortal and formation immunity sets
  only to units that are both.
- The ENTD 7-byte AI copy reads three bytes past `ai_target_xy`.

## Functions in scope

![[Units and Jobs scope]]
