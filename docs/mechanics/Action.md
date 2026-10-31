---
type: mechanic
tier: 1
scope:
  - battle_action
  - battle_reaction
---

# Action

## What it does in the game

An action is what a unit does with its Act (an ability, an attack, an item, a
throw or a jump), from the command to its result on each unit, the reactions
it provokes and the rewards.

- **Charge time.** Each command resolves to the ability that runs and its
  charge time (CT). An ability takes the CT in its data, halved (rounding up)
  by Short Charge and canceled by Non-Charge, unless it is a performed
  (Persevere) ability; Charge takes its own table, which neither support
  changes; Jump takes 50 / Speed (at least 1). Item, Throw, Math Skill and
  Attack act at once. A command with CT 0 acts now; any other charges, and the
  turn clock resolves it later.
- **Refusals.** When the action starts it can be refused: the unit cannot
  act, Silence stops a silence-affected ability, the MP is short (Half of MP
  halves the cost), or a Frog uses anything but Attack, Frog Attack and the
  Frog spell. A Hamedo from the target cancels it.
- **Strikes.** An action strikes once; twice with Two Swords (Attack, Charge
  and abilities that use the weapon's range, with weapons in both hands or in
  neither); 1 to X times at random (formulas 0x1e and 0x1f) or X + 1 times
  (formula 0x5e). A weapon's spell, which follows its hit, and a knockback
  each take a strike of their own.
- **Each target** gets the command's formula: the ability's, the item's,
  0x63 for Throw, 0x64 for Jump, or the weapon's for Attack and Charge (with
  Charge's power, and Two Hands when the weapon allows it). The result is then
  checked (a dead, petrified or walled target makes it a miss; damage and
  healing stop at 999) and applied: HP and MP within 0 and their maximum;
  Speed 1-50, CT 0-255, PA and MA 1-99, Brave and Faith 0-100 (a rider keeps
  Brave 10 or more); broken or stolen equipment, gil, EXP, JP, Level Up/Down
  and Poach. HP damage removes Sleep, Charm, Transparent and Confusion; HP at
  or below a fifth of the maximum means Critical, Brave below 10 Chicken.
- **Reactions.** A target may react, with a chance equal to its Brave, at
  four moments (see the table below). A unit under a status that prevents
  reactions, or one that is already reacting, does not; formula 7 provokes
  none.
- **Rewards.** The actor's EXP and JP come at the end, with the level up, job
  level up and learn-on-hit reports.
- **Mime.** After an action, each Mime of the actor's team copies it, turned
  to its own facing and with the actor's weapons lent, unless the ability
  cannot be mimicked.

## Data and structures

- **The command.** A [[action_command_t]] at `battle_stats_t`
  `+0x16e` (`action_actor_id`): unit, skillset, ability, item, target kind and
  target. [[g_battle_action_saved_command]] keeps a copy while a reaction or a
  preview borrows the record.
- **The result record.** A [[battle_action_data_t]] at `battle_stats_t`
  `+0x18c`: hit and miss type, HP and MP damage and healing, gil, the reaction
  id, the special effect ([[battle_action_special_effect_e]]), stat changes
  (bit `0x80` up, the low bits the amount), status inflictions and removals,
  the attack type ([[battle_action_type_flags_e]]), EXP and JP changes and the
  accuracy shown. [[g_battle_action_target_data]] and
  [[g_battle_action_attacker_data]] point at the target's and at the actor's
  own ([[g_battle_action_attacker_action_data]]).
- **The strike.** [[g_battle_action_current_ability]] ([[battle_current_ability_t]]): the
  attacker and target, the strike count and counter, the two weapons, a
  pending weapon spell (`weapon_spell_pending`) or knockback
  ([[battle_knockback_kind_e]]), the formula's inputs and copies of the
  ability, weapon and status-infliction data. The strike work
  ([[battle_strike_work_t]], in the casting unit's runtime record at `+0x18c`)
  carries the targets and `continue_attack` to the display.
- **Context.** [[g_battle_action_state]] (executing, AI simulation or
  preview), [[g_battle_action_context]] (the primary action, or a reaction or
  simulation) and [[g_battle_action_phase]] (0 Hamedo, 1 the action,
  2 reactions).
- **Reactions.** The four `reaction_abilities` bytes
  ([[battle_unit_reaction_set_1_e]] to [[battle_unit_reaction_set_4_e]]), the
  behavior flags ([[g_main_reaction_behavior_flags]],
  [[battle_reaction_behavior_flag_e]]) and the reaction being resolved
  ([[g_battle_reaction_ability_id]]).
- **Effect messages.** [[g_battle_action_post_effect_msgs]], up to 16
  records of message code, unit and value.
- **Mime.** [[g_battle_action_current_attacker]]
  ([[battle_current_attacker_data_t]]): the last action, its offset to the
  target, the actor's facing, team and weapons, and `mimic_pending`.

## Main flow

1. **The command.** The action menus and AI_COMMAND find the ability a
   command uses ([[battle_action_resolve_command_ability]]), the
   [[Targeting]] page picks its target, and in PRE_ATTACK_ANIMATION
   [[battle_action_execute_ability]] marks the unit as having acted and
   commits the command ([[battle_action_commit_command]],
   [[battle_action_prepare_attack]]): 0 charges (AFTER_COMMAND), 1 acts now
   and 3 jumps (COMMENCE_ATTACK_PHASE). The Move, Wait and Execute? windows
   are `battle_state_enter_*` states ([[Battle flow]]).
2. **The phases.** [[battle_state_announce_next_ability]] runs phase 0, a
   Hamedo ([[battle_reaction_prepare_hamedo_for_pending_action]]);
   phase 1, the action ([[battle_action_begin]]: the refusals, the strike
   data); and phase 2, the reactions ([[battle_reaction_prepare_next]], one
   reacting unit at a time).
3. **Each strike.** OPEN_SP2_FILES or RESUME_ATTACK_PHASE start it
   ([[battle_action_start_strike]]). [[battle_action_resolve_ability_strike]]
   lists the targets and runs [[battle_action_run_pre_formula_setup]] for each:
   the reactions before the formula, the formula
   ([[g_battle_formula_handlers]]) and the result's check
   ([[battle_action_finalize_target_current_action]]). The effect file loads
   (START_EFFECT_FILE_OPEN) unless there is none;
   [[battle_action_apply_strike_results]] applies each target's result
   ([[battle_action_apply_target_result]]) and picks the actor's animation, and
   [[battle_action_play_ability_effect]] plays the effect (ACTION_EXECUTE).
4. **Messages.** BATTLE_MESSAGE_DISPLAY queues the targets' and the actor's
   effect messages ([[battle_action_queue_target_effect_messages]],
   [[battle_action_start_effect_messages]]); RESUME_ATTACK_PHASE shows them,
   the last queued first ([[battle_action_show_next_effect_message]]), strikes
   again while `continue_attack` is set, or goes on to the next phase.
5. **Rewards.** [[battle_action_grant_rewards]] grants the EXP and JP, and
   [[battle_action_init_learn_ability_on_hit]] shows the reports
   (LEARN_ABILITY_ON_HIT).
6. **Mimes.** The turn clock (phases 13 and 14) runs
   [[battle_action_prepare_mimic]] for each Mime.

### Reactions

Every reaction needs a successful Brave roll
([[battle_reaction_fails_brave_roll]]: a roll of 0-99 below the unit's
Brave).

| When | Checked by | Reactions | What happens |
|---|---|---|---|
| Before the formula | [[battle_reaction_check_before_formula]] | Reflect (the status), Blade Grasp, Arrow Guard | The strike misses |
| When targeted, even by a strike that misses | [[battle_reaction_check_when_targeted]] | Sunken State, Caution, Dragon Spirit, Brave Up, Faith Up (abilities that cost MP), Counter Tackle, Counter Flood, Absorb Used MP, Counter, Counter Magic (tested last) | Queued for phase 2 |
| Before HP and MP move | [[battle_reaction_check_before_hp_change]] | MP Switch, Distribute, Damage Split | MP Switch moves the whole HP damage to MP at once, wasting what the MP cannot take; the others are queued |
| After the result | [[battle_reaction_check_after_result]] | PA, MA and Speed Save, Regenerator, Gilgame Heart and Auto Potion (after HP damage); HP Restore, MP Restore, Critical Quick and Meatbone Slash (while Critical) | Queued for phase 2 |

In phase 2 a queued reaction becomes an action of its own: Counter and
Meatbone Slash answer with Attack and Counter Tackle with Dash, each only
within range; Counter Magic casts the spell it was hit by and Counter Flood the
Elemental ability of its own tile; Auto Potion drinks the first potion the party has
(Potion, then Hi-Potion, then X-Potion); Reflect sends the spell on
([[battle_target_apply_reflect]]). The others apply their own effect
([[battle_reaction_apply_effect]]): the saves +1, Brave Up and Faith Up +3,
the statuses of Sunken State (Transparent), Caution (Defending), Dragon Spirit
(Reraise) and Regenerator (Regen), full HP or MP, a full CT, damage equal to
the reacting unit's max HP (Meatbone Slash), the MP cost back, the damage as
gil, a share of the excess healing for each injured ally (Distribute) and half
the damage back to the actor (Damage Split). In an AI simulation only Reflect
reacts.

## Where to change

- **Charge times, Short Charge, Non-Charge and Jump's CT:**
  [[battle_action_prepare_attack]].
- **What refuses an action (Silence, MP, Frog):**
  [[battle_action_check_and_consume_mp]].
- **How many strikes an action makes:**
  [[battle_action_init_current_ability_strike_data]].
- **Which formula a command uses, Two Hands and Charge's power:**
  [[battle_action_run_pre_formula_setup]]. The formulas themselves are on
  the [[Formulas]] page.
- **Which units a strike hits:** [[battle_action_resolve_ability_strike]] and
  the [[Targeting]] page.
- **Stat limits, KO, Critical, Chicken, the statuses a hit removes, Golem:**
  [[battle_action_apply_target_result]].
- **Damage caps and invalid targets:**
  [[battle_action_finalize_target_current_action]].
- **Items for the party, war funds and Poach:**
  [[battle_action_add_party_item]], [[battle_action_add_war_funds]] and
  [[battle_action_add_poached_item_to_fur_shop_inventory]].
- **A reaction's trigger:** its `battle_reaction_try_*` function and the
  `battle_reaction_check_*` function of its moment; its effect,
  [[battle_reaction_apply_effect]]; the counters,
  [[battle_reaction_prepare_next]]; Hamedo,
  [[battle_reaction_prepare_hamedo_for_pending_action]].
- **Mime:** [[battle_action_prepare_mimic]].
- **Effect messages:** [[battle_action_queue_unit_effect_messages]].
- **Today's limits:** every change must keep the original bytes until build 3
  lets code grow or move; everything that names it already follows
  (`CODEBASE.md`, "What can change today").

## Quirks and debts

- [[battle_action_init_current_ability_strike_data]] and
  [[battle_action_run_pre_formula_setup]] read range data past the ability
  table for item ids (`QUIRKS.md`).
- `battle_unit_t` repeats [[battle_strike_work_t]] field by field
  (`QUIRKS.md`, Declaration leads).
- [[battle_reaction_try_distribute]] passes an ignored second argument
  (`QUIRKS.md`).
- When the katana breaks, [[battle_action_finalize_draw_out_katana_result]]
  copies a byte onto itself (`QUIRKS.md`).
- [[battle_action_activate_move_act]],
  [[battle_action_load_last_used_ability]] and
  [[battle_action_store_8019387c_if_not_reacting]] are never called or
  referenced on the disc.
- Reaction bit `0x04` of the third byte (id 0x1bb) has a blank name and no
  code tests it ([[Abilities]]).

## Functions in scope

![[Action scope]]
