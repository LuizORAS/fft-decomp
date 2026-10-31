---
type: mechanic
tier: 1
scope:
  - main_ability
  - battle_ability
  - world_ability
  - main_unit_enable_rsm_flags()
  - main_unit_set_rsm_flag()
  - battle_menu_build_ability_list()
  - world_menu_build_ability_list()
  - battle_menu_load_skillset()
  - battle_menu_get_unit_skillset_ability_data()
  - battle_menu_call_load_charge_skillset()
  - battle_menu_load_charge_skillset()
  - battle_menu_load_item_abilities()
  - battle_menu_load_throw_abilities()
  - battle_menu_collect_throwable_items()
  - battle_menu_load_draw_out_abilities()
  - battle_menu_collect_draw_out_katanas()
  - battle_menu_load_elemental_abilities()
  - battle_menu_load_math_skill_attributes()
  - battle_menu_load_math_skill_multiples()
  - battle_menu_collect_math_skill_abilities_by_flags()
  - battle_menu_collect_calculator_abilities()
  - battle_menu_collect_monster_skill_abilities()
  - battle_menu_init_monster_skill_check()
  - battle_menu_has_monster_skill_supporter_at_tile()
  - battle_unit_learn_used_ability()
  - battle_unit_generate_crystal_or_treasure()
  - battle_unit_generate_treasure()
  - battle_unit_get_crystal_or_treasure_at_map_coords()
  - battle_unit_learn_from_crystal()
  - battle_unit_select_attack_animation_for_ability()
  - battle_unit_set_animation_for_item_abilities()
  - battle_unit_start_ability_pre_attack_animation()
  - battle_unit_start_ability_charging_animation()
  - battle_text_select_ability_announcement()
  - world_text_determine_spell_quote()
  - battle_text_is_skillset_in_spell_quote_exception_list()
---

# Abilities

## What it does in the game

Every command a unit can give, and every passive rule it carries, is an
ability. A unit uses action abilities from its skillsets' menus. It carries
one reaction, one support and one movement ability that it equips, plus the
innate ones of its job.

- **Ids and classes.** An ability id runs from 0x000 to 0x1ff. Its range
  fixes its class (`ability_type_e`) and the table that holds the class's
  data ([[main_ability_calculate_pointers_and_type]]):

| Ids | Class | Class data (MAIN) |
|---|---|---|
| 0x000-0x16f | Default: spells, skills, monster abilities | 14 bytes ([[ability_secondary_data_t]], `0x8005fbf0`): range, area, vertical, four flag bytes, element, formula, X, Y, status set, CT, MP |
| 0x170-0x17d | Item | the item it uses (`0x80061010`) |
| 0x17e-0x189 | Throw | the item type it throws (`0x80061020`) |
| 0x18a-0x195 | Jump | horizontal and vertical reach (`0x8006102c`) |
| 0x196-0x19d | Charge | CT and power, added to XA (`0x80061044`) |
| 0x19e-0x1a5 | Math Skill | its Calculator flags (`0x80061054`) |
| 0x1a6-0x1c5 | Reaction | one byte each from `0x8006105c`, and the reaction behavior flags (`0x8005ebd0`) |
| 0x1c6-0x1e5 | Support | one byte each |
| 0x1e6-0x1fd | Movement | one byte each |
| 0x1fe-0x1ff | | a random reaction, support or movement pick for a unit from the scenario data ([[main_unit_calculate_rsm]]) |

- **Skillsets.** A human skillset lists 22 abilities: 16 action abilities,
  then 6 reaction, support and movement ones that a unit learns in that job.
  A monster skillset lists 4. A unit's job gives its primary skillset; the
  player picks the secondary. Each skillset's action menu type
  ([[g_main_action_menu_types_by_skillset]]) decides what its menu lists:
  abilities, items (Item), throwable weapons (Throw), Math Skill's sets, the
  one Elemental ability of the unit's tile, katanas (Draw Out), Charge, Jump
  or a monster's abilities.
- **Learning.** A unit learns abilities per job. Each job keeps three bytes
  of learned bits (one per skillset slot) and its own JP.
  - **With JP** in the WORLD job menus ([[world_ability_run_learn_list]],
    [[world_ability_learn]]): the ability's JP cost comes off the job's JP.
    Zodiac and Ultima carry `ABILITY_TYPE_FLAG_CANNOT_LEARN_WITH_JP`: the
    learn list leaves them out and the view list hides their costs.
  - **On hit:** 16 abilities carry `ABILITY_TYPE_FLAG_LEARN_ON_HIT` (Cure 4,
    Protect 2, Shell 2, Fire 4, Bolt 4, Ice 4, Haste 2, Slow 2, Bahamut,
    Odin, Leviathan, Salamander, Lich, Cyclops, Zodiac, Ultima). A human the
    action reached learns it on a roll of the ability's learn rate out of 100,
    when its current job's skillset holds the ability, it has not learned it
    yet and no status stops it from reacting
    ([[battle_ability_find_learn_on_hit_unit]], [[battle_unit_learn_used_ability]]).
  - **From a crystal:** a unit that steps on a crystal may take the
    abilities the dead unit had learned and it has not, each offered on a
    roll of its learn rate and only when neither unit is a monster
    ([[battle_unit_generate_crystal_or_treasure]]), or restore all its HP and
    MP instead ([[battle_unit_learn_from_crystal]]). A chest gives an item
    instead: one of the dead unit's equipment, or with none a random
    consumable from Potion to Phoenix Down except Elixir
    ([[battle_unit_generate_treasure]]).
  - **Units from the scenario data** learn theirs when they are built
    ([[main_unit_learn_job_abilities]]).
- **Equipping.** In the WORLD menus a unit equips one reaction, one support
  and one movement ability from those it learned in its base job and the
  generic jobs except Mime ([[world_ability_build_equippable_rsm_list]]). The job's four
  innate abilities and the three equipped ones make the unit's active bitsets
  ([[main_unit_enable_rsm_flags]]); the code tests those bits, not the
  equipped ids.
- **Monsters.** A monster uses the first three abilities of its skillset;
  the fourth needs a same-team unit with the Monster Skill support next to
  it that can react and stands within 5 height
  ([[battle_menu_init_monster_skill_check]]).
- **Announcing.** Before an action plays, a window shows its name, or the
  unit speaks a spell quote ([[battle_text_select_ability_announcement]]). The name
  shows for an ability with `ABILITY_TYPE_FLAG_SHOW_NAME` while the Ability
  Names option is on, and always for Item and Throw. Attack, Defend, Equip
  Change, Jump, Item and Throw never quote, and the first four show no name
  either ([[g_battle_spell_quote_exception_skillsets]]).

## Data and structures

- **The ability record** ([[g_main_ability_data]], MAIN `0x8005ebf0`, 8
  bytes, [[ability_data_t]]): JP cost, learn rate, type flags
  ([[ability_type_flag_e]]; the low four bits repeat the class in
  FFTPatcher's numbering and nothing reads them) and four bytes of flags
  that the AI and the action results read.
- **The class data**, as in the table above: [[g_main_ability_range_data]]
  with [[ability_secondary_flags_1_e]] to [[ability_secondary_flags_4_e]];
  [[g_main_item_ability_item_ids]], [[g_main_throw_ability_item_types]],
  [[g_main_jump_ability_data]], [[g_main_charge_ability_data]],
  [[g_main_math_rsm_ability_data]] and
  [[g_main_reaction_behavior_flags]]. Each table is indexed
  from the first id of its class.
- **Skillsets** ([[skillset_id_e]]): human skillsets 0x00-0xaf
  ([[g_main_ability_human_skillset_abilities]], `0x80064a94`, 25 bytes: three
  bytes of ninth id bits, then 22 ids) and monster skillsets 0xb0-0xdf
  ([[g_main_ability_monster_skillset_abilities]], `0x80065bc4`, 5 bytes);
  [[main_ability_get_id_from_skillset]] reads one slot and
  [[main_ability_store_skillset_abilities]] a filtered list.
- **On the unit** ([[battle_stats_t]]): `primary_skillset`,
  `secondary_skillset`, `innate_abilities`, the equipped
  `reaction_ability`, `support_ability` and `movement_ability`, the active
  bitsets `reaction_abilities`, `support_abilities` and `movement_abilities`
  ([[battle_unit_reaction_set_1_e]] and the other ten enums in `unit.h`),
  `learned_abilities` (three bytes per job row, [[main_ability_get_known]])
  and `job_points`.
- **In BATTLE:** [[g_battle_ability_animation_data]] (for ids below 0x1c6:
  the charge animation set, the attack animation and a third byte that no
  code reads), [[g_battle_ability_charge_animation_sets]] (pre-attack and
  charging variants), [[g_battle_effect_ability_files]] (the effect file, or
  none), [[g_battle_spell_quote_exception_skillsets]] and the crystal result
  [[g_main_crystal_pickup_result]] ([[main_crystal_pickup_result_t]]).

## Main flow

1. **A unit is set up.** The job's skillset, innate abilities and the
   equipped reaction, support and movement abilities fill the active
   bitsets ([[main_unit_enable_rsm_flags]], [[main_unit_set_rsm_flag]]).
2. **The menus.** The action menu lists the unit's skillsets
   ([[battle_menu_build_unit_action_menus]]); opening one loads its usable
   entries through the menu type's loader ([[battle_menu_load_skillset]]):
   [[battle_menu_get_unit_skillset_ability_data]] lists the learned action
   abilities with their MP (Half of MP halves it, rounding up), CT (Short
   Charge halves it, rounding up; Non-Charge makes it 0; a performed ability
   keeps it) and how many turns come first; a Frog lists only the Frog spell,
   and an ability that needs a sword or Materia Blade drops without one.
   [[battle_menu_build_ability_list]] then grays the abilities the unit lacks
   the MP for or cannot cast while Silenced.
3. **The command.** The chosen ability goes through the [[Action]] pipeline,
   [[Targeting]] and its formula ([[Formulas]]). Once it is confirmed, the
   unit plays the pre-attack gesture
   ([[battle_unit_start_ability_pre_attack_animation]]); a charged ability
   then waits its CT ([[battle_action_prepare_attack]]) in the charging pose
   ([[battle_unit_start_ability_charging_animation]]). When the action runs,
   the announcement plays ([[battle_state_announce_next_ability]]), the
   effect file plays ([[battle_effect_request_ability_effect]]) and the unit
   strikes ([[battle_unit_select_attack_animation_for_ability]]: the weapon's
   swing, the item's use or the ability's own animation; the Lucavi and
   Altima always 0x2c).
4. **After the action.** A unit that can learn the ability on hit does
   ([[battle_action_init_learn_ability_on_hit]], the LEARN_ABILITY_ON_HIT
   state of [[Battle flow]]).
5. **A crystal or chest.** A unit that ends its move on one gets the
   CRYSTAL_LEARN post-move event; the choice is applied by
   [[battle_unit_learn_from_crystal]].
6. **Between battles.** The WORLD job menus learn abilities with JP and equip
   the reaction, support and movement abilities.

### Where each reaction, support and movement ability is implemented

"Where" names the functions that test the ability's bit or id. Reactions
need a Brave roll first; the [[Action]] page says when each one is checked.

| Id | Reaction | What the code does | Where |
|---|---|---|---|
| 0x1a6-0x1a8 | PA Save, MA Save, Speed Save | +1 PA, MA or Speed after HP damage | [[battle_reaction_check_after_result]] |
| 0x1a9 | Sunken State | Transparent when targeted; a random pick never chooses it | [[battle_reaction_check_when_targeted]], [[main_unit_calculate_rsm]] |
| 0x1aa | Caution | Defending when targeted | [[battle_reaction_check_when_targeted]] |
| 0x1ab | Dragon Spirit | Reraise when targeted | the same |
| 0x1ac | Regenerator | Regen after HP damage | [[battle_reaction_check_after_result]] |
| 0x1ad, 0x1ae | Brave Up, Faith Up | +3 Brave or Faith when targeted (Faith Up by an ability that costs MP) | [[battle_reaction_check_when_targeted]] |
| 0x1af-0x1b2 | HP Restore, MP Restore, Critical Quick, Meatbone Slash | While Critical: full HP, full MP, a full CT, an Attack for the unit's max HP | [[battle_reaction_check_after_result]], [[battle_reaction_prepare_next]] |
| 0x1b3 | Counter Magic | Casts back the spell it was hit by | [[battle_reaction_check_when_targeted]] |
| 0x1b4 | Counter Tackle | Answers with Dash | the same |
| 0x1b5 | Counter Flood | Answers with the Elemental ability of its own tile | the same |
| 0x1b6 | Absorb Used MP | Gains the MP the action cost | the same |
| 0x1b7 | Gilgame Heart | Gains the damage as gil | [[battle_reaction_check_after_result]] |
| 0x1b8 | Reflect | The id the Reflect status reacts with | [[battle_reaction_mark_reflected]], [[battle_target_apply_reflect]] |
| 0x1b9 | Auto Potion | Drinks the weakest potion the party has | [[battle_reaction_select_auto_potion_item]] |
| 0x1ba | Counter | Answers with Attack | [[battle_reaction_check_when_targeted]] |
| 0x1bb | (blank) | Nothing | |
| 0x1bc | Distribute | Shares excess healing with injured allies | [[battle_reaction_try_distribute]] |
| 0x1bd | MP Switch | HP damage taken from MP | [[battle_reaction_try_mp_switch]] |
| 0x1be | Damage Split | Half the damage back to the actor | [[battle_reaction_try_damage_split]] |
| 0x1bf | Weapon Guard | The unit's weapon evade counts | [[battle_formula_store_physical_evade_values]] |
| 0x1c0 | Finger Guard | Cancels a Talk Skill action | [[battle_formula_apply_finger_guard]] |
| 0x1c1 | Abandon | Halves the attacker's base hit | [[battle_formula_calculate_abandon]] |
| 0x1c2 | Catch | Catches a thrown weapon | [[battle_formula_apply_catch]], [[battle_action_apply_target_result]] |
| 0x1c3, 0x1c4 | Blade Grasp, Arrow Guard | The strike misses | [[battle_reaction_try_blade_grasp]], [[battle_reaction_try_arrow_guard]] |
| 0x1c5 | Hamedo | Strikes first and cancels the action | [[battle_reaction_prepare_hamedo_for_pending_action]] |

| Id | Support | What the code does | Where |
|---|---|---|---|
| 0x1c6-0x1cd | Equip Armor, Shield, Sword, Knife (katanas), Crossbow, Spear, Axe, Gun | Adds the equipment categories | [[main_unit_set_equippable_items]], [[main_unit_calculate_entd_equipment]] |
| 0x1ce | Half of MP | MP cost halved: spent rounded down, listed and grayed rounded up (`QUIRKS.md`) | [[battle_action_check_and_consume_mp]], [[battle_menu_get_unit_skillset_ability_data]], the AI |
| 0x1cf | Gained JP Up | Action JP * 3 / 2; Move-Get JP * 2 | [[battle_unit_apply_action_rewards]], [[battle_action_init_movement_ability_benefit]] |
| 0x1d0 | Gained Exp Up | EXP * 2 (an award stays at most 99); Move-Get EXP * 2 | [[battle_unit_update_attacker_earned_experience]], the same |
| 0x1d1 | Attack Up | Physical XA * 4 / 3 | [[battle_formula_apply_physical_attack_supports]], [[battle_formula_apply_attack_up_and_martial_arts]] |
| 0x1d2 | Defense Up | Physical XA * 2 / 3 against it | [[battle_formula_apply_defense_up]] |
| 0x1d3, 0x1d4 | Magic Attack Up, Magic Defend Up | Magical XA * 4 / 3, or * 2 / 3 against it | [[battle_formula_apply_magic_attack_up]], [[battle_formula_apply_magic_defense_up]] |
| 0x1d5 | Concentrate | The target's accessory, shield and class evades become 0 | [[battle_formula_calculate_concentrate]] |
| 0x1d6, 0x1d7 | Train, Secret Hunt | A monster hit down to max HP / 5 or less is invited (Train); one brought to 0 HP or killed is poached instead (Secret Hunt), which also opens the fur shop | [[battle_formula_apply_poach_and_train]], [[world_ability_has_secret_hunt]] |
| 0x1d8 | Martial Arts | Bare-handed XA * 3 / 2; the WORLD menus keep the hands empty | [[battle_formula_apply_physical_attack_supports]], [[world_unit_build_best_fit_equipment]] |
| 0x1d9 | Monster Talk | Talk Skill works on monsters | [[battle_formula_talk_skill_hit_ma_x_percent]] |
| 0x1da | Throw Item | Item range 4 instead of 1 | [[battle_target_set_panels_for_action]] |
| 0x1db | Maintenance | Break, Steal and Might Sword fail on it | [[battle_formula_apply_maintenance]] |
| 0x1dc | Two Hands | Doubles the XA of a weapon held in both hands | [[battle_action_run_pre_formula_setup]], [[battle_formula_apply_physical_attack_supports]], [[world_ability_has_two_hands]] |
| 0x1dd | Two Swords | Two strikes, one per weapon | [[battle_action_init_current_ability_strike_data]], [[world_ability_has_two_swords]] |
| 0x1de | Monster Skill | An adjacent monster of its team gets its fourth ability | [[battle_menu_has_monster_skill_supporter_at_tile]], the AI |
| 0x1df | Defend | The Defend command | [[battle_menu_build_unit_action_menus]], the AI |
| 0x1e0 | Equip Change | The Equip Change command | [[battle_menu_build_unit_action_menus]] |
| 0x1e1 | (blank) | Nothing | |
| 0x1e2, 0x1e3 | Short Charge, Non-Charge | CT halved, rounding up, or 0 | [[battle_action_prepare_attack]], [[battle_menu_get_unit_skillset_ability_data]] |
| 0x1e4, 0x1e5 | (blank) | Nothing | |

| Id | Movement | What the code does | Where |
|---|---|---|---|
| 0x1e6-0x1eb | Move +1 to +3, Jump +1 to +3 | Adds to Move or Jump | [[main_unit_calculate_move_jump]] |
| 0x1ec | Ignore Height | No height limit (Jump becomes 31) | [[battle_move_store_unit_movement_to_scratchpad]] |
| 0x1ed, 0x1ee | Move-HP Up, Move-MP Up | Max HP or MP / 10 (rounding up) after a move | [[battle_move_get_movement_ability_events]], [[battle_action_init_movement_ability_benefit]] |
| 0x1ef, 0x1f0 | Move-Get EXP, Move-Get JP | EXP or JP after a move | the same |
| 0x1f1 | (Cannot enter water) | Innate in monsters and several bosses: no Move in Water or Move Underwater, and no water without a way across it | [[battle_move_store_unit_movement_to_scratchpad]], [[battle_unit_validate_placement]] |
| 0x1f2, 0x1f3 | Teleport, Teleport 2 | Moves without a path; Teleport can fail beyond Move | [[battle_move_get_effective_flags]], [[battle_move_roll_teleport_success]] |
| 0x1f4, 0x1f5 | Any Weather, Any Ground | Weather-dependent surfaces cost 1, or every surface that can be entered | [[battle_move_store_unit_movement_to_scratchpad]] |
| 0x1f6 | "Move in Water" | Walks on the water's surface (the game's names of 0x1f6 and 0x1f7 are swapped) | the same, [[battle_unit_get_effective_height]] |
| 0x1f7 | "Walk on Water" | Moves in water one level deep | the same |
| 0x1f8, 0x1f9 | Move on Lava, Move Underwater | Their own terrain classes | [[battle_move_store_unit_movement_to_scratchpad]] |
| 0x1fa | Float | Moves as Float and gives the Float status | the same, [[main_status_update_unit_flags_and_ct]] |
| 0x1fb | Fly | Moves as Fly | [[battle_move_get_effective_flags]] |
| 0x1fc | Silent Walk | Sets an effective-movement bit that nothing reads; no job learns it or has it | [[battle_move_get_effective_flags]] |
| 0x1fd | Move-Find Item | Finds the tile's hidden item after a move | [[battle_move_get_post_move_events]] |

The [[Movement rules]] guide details the movement classes.

## Where to change

- **An ability's cost, learn rate and flags:** its record in
  [[g_main_ability_data]] (disc data).
- **Range, area, element, formula, X, Y, statuses, CT and MP of an action
  ability:** its class data (disc data); the [[Formulas]] page says what the
  formula does with X and Y.
- **What a job learns:** its skillset's 22 slots (disc data,
  [[main_ability_get_id_from_skillset]] reads them); its innate abilities sit
  in the job record.
- **How a skillset's menu is filled:** [[g_main_action_menu_types_by_skillset]]
  and the loaders under [[battle_menu_load_skillset]].
- **Learning:** JP in [[world_ability_run_learn_list]]; on hit in
  [[battle_ability_find_learn_on_hit_unit]]; crystals in
  [[battle_unit_generate_crystal_or_treasure]].
- **What a reaction, support or movement ability does:** the "Where" column
  above.
- **Its animation and effect:** [[g_battle_ability_animation_data]] and
  [[g_battle_effect_ability_files]] (disc data).
- The [[Changing an ability]] guide walks through a change.
- **Today's limits:** every change must keep the original bytes until build 3
  lets code grow or move; everything that names it already follows
  (`CODEBASE.md`, "What can change today").

## Quirks and debts

- The game's names are swapped for two movement abilities ("Move in Water"
  walks on the surface, "Walk on Water" moves in the water), and "Equip
  Knife" equips katanas. The code names the bits by what they do.
- Silent Walk sets an effective-movement bit that nothing reads, and no job
  learns it or has it innate. Reaction 0x1bb and supports 0x1e1, 0x1e4 and
  0x1e5 have blank names and no code.
- The skillset filter counts Move+1 (0x1e6) as a support ability; no caller
  asks for supports alone (`QUIRKS.md`).
- The AI's CT estimate tests the Monster Skill and Defend bits where Short
  Charge and Non-Charge were presumably meant (`QUIRKS.md`).
- Outside execution, [[battle_unit_generate_treasure]] returns no value
  (`QUIRKS.md`).
- WORLD's copy of the announcement, [[world_text_determine_spell_quote]],
  is never called (`QUIRKS.md`).

## Functions in scope

![[Abilities scope]]
