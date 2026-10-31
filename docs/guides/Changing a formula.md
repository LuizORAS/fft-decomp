---
type: guide
mechanics: [Formulas]
---

# Changing a formula

## Goal

Change how an action computes its result on a target: its damage or healing,
its hit chance, what evades it, the statuses and special effects it adds; or
give an ability, weapon or consumable another formula.

## Where it lives

The [[Formulas]] page lists each formula id with its handler and its users.
A handler is a short list of helper calls, so most changes either call other
helpers or change one helper. The helpers by step:

| Step | Helpers |
|---|---|
| Evasion | [[battle_formula_calculate_physical_evade]], [[battle_formula_calculate_magical_evade]], or none |
| XA and YA | the `battle_formula_store_*` helpers (PA, MA, Speed, X, Y, the weapon's power), [[battle_formula_calculate_base_xa]] (by weapon type), [[battle_formula_store_jump_xa_ya]] |
| Modifiers on XA | [[battle_formula_apply_physical_xa_modifiers]], [[battle_formula_apply_physical_status_xa_modifiers]], [[battle_formula_apply_attack_up_and_martial_arts]], [[battle_formula_apply_magical_xa_modifiers]], [[battle_formula_apply_ability_element_strengthen]], [[battle_formula_apply_weapon_element_strengthen]], [[battle_formula_apply_zodiac_compatibility]], [[battle_formula_apply_charge]], [[battle_formula_calculate_critical_hit]] |
| Hit chance | [[battle_formula_store_hit_chance]], [[battle_formula_calculate_faith]], [[battle_formula_roll_hit_chance]]; or the bundles [[battle_formula_calculate_physical_accuracy]], [[battle_formula_calculate_physical_status_accuracy]], [[battle_formula_calculate_magic_accuracy]] and its variants |
| Damage | [[battle_formula_store_xa_times_ya_damage]], [[battle_formula_calculate_physical_damage]], [[battle_formula_calculate_elemental_xa_times_ya]], [[battle_formula_calculate_hp_percent_damage]], [[battle_formula_calculate_mp_percent_damage]] |
| Element and Faith | [[battle_formula_apply_weather_elemental_effects]], [[battle_formula_apply_ability_element]], [[battle_formula_apply_weapon_element]], [[battle_formula_calculate_faith]], [[battle_formula_apply_elemental_absorption]] |
| Healing and drains | [[battle_formula_apply_undead_reversal]], [[battle_formula_apply_hp_absorption]], [[battle_formula_apply_mp_absorption]], [[battle_formula_convert_hp_damage_to_mp_recovery]] |
| Added effect and status | [[battle_formula_roll_conditional_status_proc]], [[battle_formula_apply_status]], [[battle_formula_apply_status_to_action]], [[battle_formula_apply_status_and_check_undead]] |
| Failing on purpose | [[battle_formula_force_attack_miss]], [[battle_formula_force_sleeping_target_miss]], [[battle_formula_check_dragon]], [[battle_formula_apply_maintenance]], [[battle_formula_select_target_equipment]] |

The inputs come from the game data in MAIN: an ability's formula id, X, Y,
element and status; a weapon's formula id, power, element and added effect;
a consumable's formula id, Z and status. The command can override the
ability's choice ([[battle_action_run_pre_formula_setup]]: Throw, Jump, and
the weapon's formula for Attack and Charge).

## What changes with it

- **Estimates.** The menus' preview and the AI's simulation run the same
  handler ([[g_battle_action_state]]): a change shows in the preview and
  steers the AI. Their random rolls take the middle value
  ([[battle_formula_get_random_0_7fff]]), and the 19% added effect counts in
  a preview but never for the AI.
- **Formula ids tested elsewhere** ([[ability_formula_e]]): formula 0x07
  provokes no reactions (the `battle_reaction_check_*` functions); formulas
  below 0x07 check Poach and Train; 0x1e, 0x1f and 0x5e set how many strikes
  an action makes ([[battle_action_init_current_ability_strike_data]]);
  0x03 loads no status; the AI sets its own weapon flag for formulas 0x06 and
  0x07 (`battle_ai_store_weapon_attack_data`).
- **Shared helpers.** A helper's page lists its callers; changing it changes
  every formula that calls it.
- **The result.** [[battle_action_apply_target_result]] applies the HP and
  MP amounts whatever the result type, while the result type and special
  effects choose the animation and the messages (the [[Action]] page).
- **What does not follow.** The ability's effect animation, its help text
  and the AI's ability scoring describe the original behavior.
- **Quirks.** The [[Formulas]] page lists the `QUIRKS.md` entries that live
  in the formulas (Oil, the bow weather check, the Holy Sword element).

## What can be done today

Nothing that changes a byte: `make check` compares every function with its
original hash, so a new step in a handler or a different constant fails until
the build accepts different code (build 3). Formula ids, X, Y and Z are disc
data, which the repository does not hold. To try an idea, change the game's
memory in an emulator: `make run` launches PCSX-Redux, and `make map` writes
the symbol maps that name these functions in its debugger.
