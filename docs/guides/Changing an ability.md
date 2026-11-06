---
type: guide
mechanics: [Abilities]
---

# Changing an ability

## Goal

Change what an ability costs, how it is learned, what it does, which job
teaches it, how it looks; or change what a reaction, support or movement
ability does.

## Where it lives

An ability is mostly disc data, read by code that this repository holds. The
[[Abilities]] page explains the id classes and lists where each reaction,
support and movement ability is implemented.

| What | Where |
|---|---|
| JP cost, learn rate, Learn on Hit, the name window, "cannot learn with JP" | the ability's record ([[ability_data_t]], [[g_main_ability_data]], MAIN `0x8005ebf0`, 8 bytes per id, disc data) |
| Range, area, vertical reach, targeting flags, element, formula, X, Y, status set, CT, MP of an action ability | its 14-byte record ([[ability_secondary_data_t]], [[g_main_ability_range_data]], MAIN `0x8005fbf0`, disc data) |
| The item an Item ability uses, the item type a Throw ability throws, a Jump's reach, a Charge's CT and power, a Math Skill's flags | the class data at MAIN `0x80061010`, `0x80061020`, `0x8006102c`, `0x80061044` and `0x80061054` (disc data) |
| What the formula does with X, Y and the stats | the formula's handler ([[Formulas]], [[Changing a formula]]) |
| The statuses it adds or removes | the status set the record names ([[Changing a status]]) |
| Who it can target | the targeting flags and [[Targeting]] |
| Which job teaches it, and in which slot | the skillset's list ([[g_main_ability_human_skillset_abilities]], MAIN `0x80064a94`, 25 bytes per skillset; monsters at `0x80065bc4`, disc data) |
| A job's innate abilities | the job record ([[job_data_t]], `innate_abilities`, disc data) |
| How the skillset's menu is filled | [[g_main_action_menu_types_by_skillset]] (disc data) and the loader [[battle_menu_load_skillset]] picks |
| Learning on hit or from a crystal | [[battle_ability_find_learn_on_hit_unit]], [[battle_unit_generate_crystal_or_treasure]] |
| Learning with JP, and which reaction, support and movement abilities a unit may equip | [[world_ability_run_learn_list]], [[world_ability_find_unit_abilities]], [[world_ability_build_equippable_rsm_list]] |
| What a reaction, support or movement ability does | the functions in the [[Abilities]] page's "Where" column; for a reaction also its moment in the [[Action]] page |
| Its charge gesture and strike animation | [[g_battle_ability_animation_data]] and [[g_battle_ability_charge_animation_sets]] (BATTLE, disc data), [[battle_unit_select_attack_animation_for_ability]] |
| Its effect file | [[g_battle_effect_ability_files]] (BATTLE, disc data) and [[battle_effect_request_ability_effect]] |
| Whether the unit speaks a spell quote | flag `ABILITY_SECONDARY_FLAG_3_SPELL_QUOTE`, the exception list [[g_battle_spell_quote_exception_skillsets]] and [[battle_text_select_ability_announcement]] |

A reaction, support or movement ability's effect is code, not data: the
code tests its bit in the unit's active bitsets. Searching for its
`BATTLE_REACTION_SET_*`, `BATTLE_SUPPORT_SET_*` or `BATTLE_MOVEMENT_SET_*`
name, and for its `ABILITY_ID_*` name, finds every test.

## What changes with it

- **The AI.** The AI reads the same records and its own flags in each
  ability record (`ai_flags`); a new effect does not change how the AI
  values the ability.
- **Estimates.** The action preview and the AI's simulations run the same
  formulas, so a changed formula shows in both.
- **WORLD and the menus.** The WORLD job menus and the BUNIT and JOBSTTS
  screens list abilities with their own copies of the list builders and the
  JP, MP and CT readers.
- **Units from the scenario data.** Their abilities and random reaction,
  support and movement picks come from the same skillsets
  ([[main_unit_learn_job_abilities]], [[main_unit_calculate_rsm]]).
- **Names and help text** are game text, separate from the code. Two
  movement abilities have swapped names, and "Equip Knife" equips katanas
  (the [[Abilities]] page).
- **Quirks.** The [[Abilities]] page lists the `QUIRKS.md` entries that touch
  abilities.

## What can be done today

Nothing that changes a byte of the original build: `make check` compares every
function with its original hash, and the ability tables are disc data, which
the repository does not hold. To try an idea, change the game's memory in an
emulator: `make run` launches PCSX-Redux, and `make map` writes the symbol maps
that name these tables in its debugger (the ability records start at
`0x8005ebf0`, 8 bytes per id; the action abilities' data at `0x8005fbf0`, 14
bytes per id). In the Developer's Cut, a change goes behind an option: code
under `#ifdef` and a table's bytes in place through `target/edits.yaml`
(`AGENTS.md`, "Changes under an option").
