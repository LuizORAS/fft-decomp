---
type: guide
mechanics: [Units and Jobs]
---

# Changing jobs and growth

## Goal

Change how units grow and progress: their stat growth and caps, the EXP and
JP they earn, the JP a job level needs, what unlocks a job, or what a new
recruit or a generated enemy starts with.

## Where it lives

Progression is part data, part code. The [[Units and Jobs]] page has the
retail values.

| What | Where |
|---|---|
| A job's growths and multipliers, Move, Jump, class evade, innate abilities, equippable types, status sets | the job record ([[job_data_t]], [[g_main_job_data]], MAIN `0x800610b8`, 48 bytes a job, disc data) |
| JP per job level | [[g_main_job_level_jp_requirements]] (MAIN `0x80066184`, eight 16-bit values, disc data) |
| Which job levels unlock a job | [[g_main_job_unlock_requirements]] (MAIN `0x800660c4`, ten packed bytes per job after Squire, disc data); the Bard, Dancer and Mime rule in [[main_job_calculate_unlocked]] |
| How a level grows the raw stats, and the stat caps | [[main_unit_apply_level_growth]], [[main_unit_calculate_actual_stats]], [[main_party_raise_unit_level]] |
| EXP to the next level | [[main_unit_check_level_up]] (100) |
| EXP per action | [[battle_unit_update_attacker_earned_experience]] |
| JP per action and the team's share | [[battle_unit_apply_action_rewards]], [[battle_unit_store_jp_and_calculate_unlocked_jobs]] |
| JP from propositions | [[main_job_add_proposition_jp]] |
| A new recruit | [[main_unit_generate_out_of_battle]]; raw-stat seeds, variances and equipment in [[g_main_unit_generation_base_data]] and [[g_main_unit_generation_raw_stat_variance]] (MAIN `0x8005e90c`, `0x8005e93c`, disc data) |
| A generated ENTD unit | [[main_unit_calculate_entd_data]] (level, Brave, Faith, EXP), [[main_unit_calculate_jobs_and_skillsets_from_entd]], [[main_unit_learn_job_abilities]], [[main_unit_calculate_random_equipment]], [[main_unit_calculate_rsm]] |

## What changes with it

- **The same rule in several places.** JP is added by
  [[battle_unit_store_jp_and_calculate_unlocked_jobs]] in battle and by
  [[main_job_add_proposition_jp]] out of it; a level's growth by
  [[main_unit_apply_level_growth]] in battle, by the replay in
  [[main_unit_calculate_actual_stats]] for a generated unit and by
  [[main_party_raise_unit_level]] for an egg. A rule changed in one place
  should change in the others.
- **The job menus.** WORLD, BUNIT and JOBSTTS each show job levels and the
  next level's JP from the same table (`world_job_*`, `bunit_job_*`,
  `jobstts_job_*`), and each lists jobs by the unlocked bitset.
- **The unlock bitset.** Squire is bit `0x800000` and each later job the next
  lower bit; the menus, the random secondary skillset
  ([[main_job_get_random_unlocked]]) and a generated unit's ability learning
  ([[main_unit_learn_job_abilities]]) read it.
- **The roster record.** [[party_unit_t]] keeps JP as byte pairs and saves
  0xe0 bytes per unit; a new field needs room there and in the save image.
- **Quirks.** The [[Units and Jobs]] page lists the `QUIRKS.md` entries here
  (the discarded `rand()`, the two level-up divisors, the double learned bit).

## What can be done today

Nothing that changes a byte: `make check` compares every function with its
original hash, and the job, unlock and generation tables are disc data,
which the repository does not hold. To try an idea, change the game's memory
in an emulator: `make run` launches PCSX-Redux, and `make map` writes the
symbol maps that name these functions and tables in its debugger (a job's
record is at `0x800610b8 + job id * 0x30`, its growths and multipliers at
`+0x0d`). Code changes wait for build 3.
