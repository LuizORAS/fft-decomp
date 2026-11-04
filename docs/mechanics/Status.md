---
type: mechanic
tier: 1
scope:
  - battle_status
  - main_status
  - main_unit_has_status_in_set()
  - battle_gfx_update_float_height_and_status_bubble()
  - battle_gfx_init_status_bubble_sprite_display()
  - battle_gfx_update_status_bubble_display()
  - battle_gfx_refresh_status_bubble()
  - battle_gfx_determine_status_bubble_parameters()
  - battle_gfx_draw_status_bubble()
  - battle_gfx_update_status_bubbles_and_graphics()
  - battle_gfx_update_unit_status_palette()
  - battle_gfx_check_tile_status_palette_mod()
  - battle_gfx_apply_status_spritesheet_change()
  - battle_unit_set_animation_based_on_status()
  - battle_unit_call_set_animation_based_on_status()
  - battle_unit_set_status_animation_by_runtime_id()
  - battle_unit_update_animation_for_status_changes()
  - battle_unit_clear_status_staging_data()
  - battle_unit_apply_staged_status_data()
  - battle_unit_update_staged_status_data()
  - battle_unit_save_staged_status_data()
  - script_process_inflict_status_commands()
  - script_inflict_status_thread()
  - status_inflict_by_entd_unit_id()
---

# Status

## What it does in the game

A unit carries up to 40 statuses. Some come with its job and equipment and
never leave; the rest are inflicted by actions, by the turn clock and by
events, and leave when something removes them or their count runs out.

- **Three sets.** Each unit has an innate set (job and equipment), an
  immunity set and the inflicted statuses; its current set is innate |
  inflicted ([[main_status_store_current]]). A removal never touches an
  innate status.
- **Landing.** An action's statuses arrive as a set with a type: All or
  Nothing, one Random status, each Separately, or a Cancel set of removals
  (the [[Formulas]] page). Before they land, the inflictions lose whatever
  the target is immune to, whatever it already has in the first three bytes
  (a timed status lands again and restarts its count), whatever one of its
  statuses blocks (each status's "blocked by" list), and for a rider
  anything outside the rider list; an immortal unit also drops the
  formation set (see the table of check sets). Each status that lands
  removes the ones it cancels.
  Inflicting Frog on a frog removes it instead. A set that changes nothing
  makes the action fail.
- **After every action result.** HP 0 means Dead; HP at or below a fifth of
  the maximum means Critical, above it the end of Critical; Brave below 10
  means Chicken, 10 or more the end of Chicken. HP damage removes Sleep,
  Charm, Transparent and Confusion, and a knockback cancels a Charge command
  ([[battle_action_apply_target_result]]).
- **Counts.** The timed statuses (Poison to Reflect) lose one count every
  clock tick, even while the unit's CT is frozen, but not while it is in
  the air from a Jump; at zero they end. Death Sentence and the death
  counter count the unit's turns instead (3 each).
- **At the start of a unit's turn** ([[battle_status_build_upkeep_action]]):
  a dead unit with Reraise revives with max HP / 10 (not an undead one); a
  dead unit's counter drops, and at zero it becomes a Crystal on a roll of
  1/2 (3/4 for a unit that started player-controlled) or else a Treasure,
  while an undead unit first gets a 50% chance to revive with 1 to max HP;
  immortal and formation units never run out. Death Sentence drops and at
  zero kills (an undead unit just loses it). Defending ends, and a Chicken
  gains 1 Brave.
- **After a unit's turn:** Poison takes max HP / 8, or without Poison Regen
  restores max HP / 8; a Poison Marsh tile poisons a grounded unit; a unit
  that acted loses Transparent.
- **Control.** Confusion, Blood Suck, Berserk, Chicken and Charm take a
  player's unit out of the player's hands; Charm and Invite give it the
  attacker's team (Invite for good). Ending them gives control and the team
  back ([[battle_status_remove_control]]).
- **Events.** An event hides statuses while it plays and gives them back
  at its end; its Inflict Status command can revive a unit, crystallize
  it or poison it for good.
- **What the player sees.** A status bubble over the unit cycles through
  its statuses, the death counter and Death Sentence's count; some statuses
  tint the palette, swap the spritesheet (Crystal, Treasure, Chicken, Frog)
  or change the standing animation; the action preview names the status
  change of highest order.

## Data and structures

- **On the unit** ([[battle_stats_t]]): `status_sets` ([[battle_status_sets_t]]:
  innate, immunity, current), `inflicted_status`, `status_ct` (16 counts, one
  per timed status, `BATTLE_STATUS_CT_INDEX`), `death_counter` and the
  action record's `status_infliction` and `status_removal`
  ([[battle_action_data_t]]).
- **Ids and masks.** [[battle_status_id_e]] numbers the statuses 0x00-0x27;
  a status lives in byte `id >> 3` with mask `0x80 >> (id & 7)`
  (`BATTLE_STATUS_BYTE_INDEX`, `BATTLE_STATUS_BYTE_MASK`). The event
  command numbers the bits from the low end instead
  (`BATTLE_STATUS_LSB_INDEX`).
- **The status data** ([[g_main_status_effect_data]], MAIN `0x80065de4`, 16
  bytes per status, [[status_effect_data_t]]): the preview order, the
  starting count, two flag bytes ([[status_effect_flags_1_e]],
  [[status_effect_flags_2_e]]), the statuses it cancels and the statuses that
  block it. The disc holds these tables, not the repository.
- **The status sets of an ability, weapon or item**
  ([[g_main_status_infliction_data]], 128 records of a type byte and five
  status bytes, [[status_infliction_data_t]]).
- **The check sets** ([[g_main_status_check_sets]],
  [[main_status_check_set_e]]), built once from the data flags by
  [[main_status_init_check_data]] and tested by
  [[main_unit_has_status_in_set]]:

| Set | Retail statuses | Read by |
|---|---|---|
| `KO` | Crystal, Dead, Petrify, Invite, Blood Suck, Treasure | the outcome check: a side with no unit outside it has lost ([[battle_action_check_battle_outcome]]) |
| `LOST` | Crystal, Invite, Treasure | the outcome check: a unit with team flags 0x03 (Ramza) under one ends the battle |
| `UNUSED_2` | Crystal, Treasure | nothing |
| `FREEZE_CT` | Crystal, Petrify, Treasure, Stop, Sleep | the turn clock: no CT gain ([[battle_turn_get_status_flags]]) |
| `PREVENT_REACTION` | Crystal, Dead, Jump, Petrify, Confusion, Blood Suck, Treasure, Berserk, Chicken, Frog, Stop, Sleep, Don't Act | reactions, Move abilities, learn-on-hit and the AI |
| `IMMORTAL_IMMUNITY` | Crystal, Dead, Undead, Petrify, Invite, Confusion, Blood Suck, Treasure, Chicken, Frog, Charm, Sleep, Don't Act, Death Sentence | job immunities of a unit both immortal and formation ([[main_unit_copy_job_data]]) |
| `FORMATION_IMMUNITY` | Crystal, Dead, Undead, Petrify, Invite, Blood Suck, Treasure, Chicken, Frog, Charm, Death Sentence | the same, and every immortal unit's inflictions ([[battle_status_modify_inflictions]]) |
| `UNUSED_7` | Darkness, Confusion, Silence, Transparent, Berserk, Chicken, Frog, Stop, Charm, Sleep, Don't Move, Don't Act | nothing |
| `EVENT_EXCLUDED` | Crystal, Dead, Undead, Jump, Petrify, Invite, Treasure, Critical | event commands aimed at a whole team skip these units ([[script_filter_unit_id_by_mode]]) |
| `MOUNT_REMOVAL` | Confusion, Berserk, Poison, Stop, Sleep, Don't Move, Don't Act, Death Sentence | the post-move events ([[battle_move_get_post_move_events]]) |
| `UNMOUNTABLE` | Crystal, Dead, Petrify, Blood Suck, Treasure, Berserk, Chicken, Frog, Charm (fixed in code) | riding a chocobo ([[battle_unit_get_chocobo_mount_state]]) |

- **The rider list** ([[g_battle_rider_status_infliction_mask]], the
  FFHacktics wiki's "Rider Allowed Status Inflictions"): a unit riding a
  mount takes only Crystal, Dead, the four action states, Reraise,
  Transparent, Critical, Regen, Protect, Shell, Haste, Wall, Faith,
  Innocent, Charm and Reflect.
- **The graphics.** Each runtime record mirrors the statuses in its own bit
  order ([[battle_unit_status_flags_1_4_e]], [[battle_unit_status_flags_5_6_e]],
  translated by [[g_battle_unit_status_mask_by_handler_index]]); changes wait
  in `statuses_to_add_*` and `statuses_to_remove_*` until the frame applies
  them. [[g_battle_gfx_status_bubble_status_masks]] gives each bubble its
  status, and [[g_battle_status_display_image_ids]] each status its preview
  image.
- **Events.** [[g_battle_unit_status_staging_data]] saves each staged unit;
  [[g_battle_event_status_masks]] lists what an event hides.

## Main flow

1. **A unit enters the battle.** [[main_status_init_unit]] splits its current
   statuses into innate and inflicted and clears the counts
   ([[main_status_init_ct]]); [[main_status_update_unit_flags_and_ct]] makes
   the Float movement ability an innate Float and starts the count of each
   timed status.
2. **An action inflicts or removes.** The formula fills the target's
   `status_infliction` and `status_removal`
   ([[battle_formula_apply_status_to_action]]; a Cancel set is first copied
   by [[battle_status_store_ability_cancellations]]).
   [[battle_action_apply_target_result]] adds Critical, Chicken and the
   HP-damage removals, then [[battle_status_resolve_unit_changes]] runs:
   [[battle_status_modify_inflictions]] filters the sets,
   [[battle_status_apply_pending_removals]] and
   [[battle_status_apply_pending_inflictions]] change the unit (counts through
   [[main_status_set_ct]], graphics through
   [[battle_status_queue_graphics_change_if_executing]]), Dead sets HP to 0,
   and [[battle_status_remove_control]] settles control. Previews and AI
   simulations run the same code; only execution queues the graphics.
3. **The clock.** [[battle_turn_run_clock]] calls
   [[battle_status_build_upkeep_action]] when a unit's turn comes,
   [[battle_status_update_expiring]] every clock tick,
   [[battle_status_apply_poison_and_regen]], [[battle_status_prepare_terrain_poison]]
   and [[battle_status_remove_transparent]] after the turn. Each change goes
   through the action record like an action's, so it shows as an action
   result.
4. **An event.** At its start the interpreter clears the staging record
   ([[battle_unit_clear_status_staging_data]]). With script variable 0x1fd
   set it stages every unit at once
   ([[battle_unit_update_staged_status_data]]); otherwise
   [[script_process_inflict_status_commands]] first runs the Inflict Status
   commands of floating or jumping units, and units are staged one by one,
   by an Inflict Status command or when the event adds them. Staging
   ([[battle_unit_save_staged_status_data]]) saves the unit and
   hides statuses: with the variable set, all but Crystal, Dead, Invite,
   Cursed, Treasure and Critical (monsters, Altima and the undead jobs keep
   Undead and Float); with it clear, Undead, Jump, Petrify, Darkness,
   Confusion, Transparent, Chicken, Frog, Haste, Slow, Charm, Sleep and Death
   Sentence. The command itself
   ([[script_inflict_status_thread]], through
   [[status_inflict_by_entd_unit_id]]) revives a dead unit at 1 HP
   with Critical, crystallizes it or poisons it, and leaves an exit mode. At
   the event's end [[battle_unit_apply_staged_status_data]] restores each
   staged unit and applies its exit mode, so that change stays.
5. **The graphics.** The queued bits reach the runtime record's mirror each
   frame ([[battle_unit_update_display]]), which plays
   [[battle_unit_update_animation_for_status_changes]], swaps the
   spritesheet ([[battle_gfx_apply_status_spritesheet_change]]), tints the
   palette ([[battle_gfx_update_unit_status_palette]]) and picks the
   standing animation ([[battle_unit_set_animation_based_on_status]]).
   [[battle_gfx_refresh_status_bubble]] switches the bubble on,
   [[battle_gfx_determine_status_bubble_parameters]] moves it to the next of
   22 bubbles every 16 ticks, and
   [[battle_gfx_update_status_bubbles_and_graphics]] draws it while the
   camera is still. The preview shows the change that
   [[main_status_find_action_highest_order_effect]] picks.

### The statuses

"Byte, mask" place the status in the five status bytes. "Count" is the
starting count in the status data: clock ticks, or turns where noted. "Added
by" counts the retail abilities, weapons and items whose status set adds it
([[g_main_status_infliction_data]]); the code adds others itself.

| Id | Status | Byte, mask | Count | Added by | What the code does |
|---|---|---|---|---|---|
| 0x00 | (no name) | 0, 0x80 | | nothing | No flags, no graphics, no rule |
| 0x01 | Crystal | 0, 0x40 | | Please Eat (on its user); a dead unit's counter; event mode 1 | Cancels every other status; counts as KO and lost; CT frozen; a crystal that a unit can step on ([[battle_unit_generate_crystal_or_treasure]]) |
| 0x02 | Dead | 0, 0x20 | 3 turns (death counter) | 10 abilities | HP 0; the counter's end above; Reraise; an action on the unit misses unless it removes Dead from a unit that is not undead ([[battle_action_finalize_target_current_action]]) |
| 0x03 | Undead | 0, 0x10 | | 4 abilities | Healing hurts it and drains heal it ([[battle_formula_apply_undead_reversal]], [[battle_formula_apply_hp_absorption]]); Death heals it; Phoenix Down takes its HP; blocks Reraise; may revive when its counter ends |
| 0x04 | Charging | 0, 0x08 | | the action state | Set while an ability charges ([[main_status_set_action_state]]); no evades; physical XA * 3 / 2 against it; a knockback cancels a Charge command |
| 0x05 | Jump | 0, 0x04 | | the action state | In the air: CT held at 99, no upkeep, counts paused; blocks every new status but Cursed |
| 0x06 | Defending | 0, 0x02 | until its turn | the action state | Doubles its evades (the base hit is halved); ends at the unit's next turn |
| 0x07 | Performing | 0, 0x01 | | the action state | A song or dance goes on; no evades; the turn counts as acted |
| 0x08 | Petrify | 1, 0x80 | | 15 abilities, Chaos Blade | Counts as KO; CT frozen; an action on it misses unless it removes Petrify |
| 0x09 | Invite | 1, 0x40 | | Invitation, Dragon Tame | Joins the attacker's team for good (initial team, no Auto-Battle); counts as KO and lost |
| 0x0a | Darkness | 1, 0x20 | | 15 abilities, 2 weapons | Its attacks halve the base hit, which doubles the target's evades ([[battle_formula_calculate_dark_confuse]]) |
| 0x0b | Confusion | 1, 0x10 | | 15 abilities, Ramia Harp | As Darkness for its attacks; no evades; out of the player's control; HP damage ends it |
| 0x0c | Silence | 1, 0x08 | | 14 abilities, Mage Masher | Abilities flagged as silenced cannot be used ([[battle_action_check_and_consume_mp]]) and are marked in the menu |
| 0x0d | Blood Suck | 1, 0x04 | | 2 Blood Suck abilities | Counts as KO; out of the player's control |
| 0x0e | Cursed | 1, 0x02 | | nothing | FFTPatcher's "Dark/Evil Looking": a darker palette and stance 2 (as Stop and Petrify), nothing else |
| 0x0f | Treasure | 1, 0x01 | | a dead unit's counter | As Crystal: cancels everything, KO and lost, CT frozen; a chest that a unit can step on |
| 0x10 | Oil | 2, 0x80 | | 5 abilities | Meant to double fire damage, but the doubling comes too late (`QUIRKS.md`); a darker palette |
| 0x11 | Float | 2, 0x40 | | Float; the Float movement ability (innate) | Earth-element actions are nullified ([[battle_formula_apply_ability_element]]); moves as Float ([[Movement rules]]) |
| 0x12 | Reraise | 2, 0x20 | | 3 abilities | A dead unit revives with max HP / 10 at its turn; never with Undead |
| 0x13 | Transparent | 2, 0x10 | | the Sunken State reaction | Its attacks ignore the target's evades ([[battle_formula_calculate_transparent]]); ends with HP damage, after it acts or after a Jump |
| 0x14 | Berserk | 2, 0x08 | | 5 abilities | XA * 3 / 2 on its attacks ([[battle_formula_apply_attacker_berserk_frog]]); out of the player's control |
| 0x15 | Chicken | 2, 0x04 | | Brave below 10 | Gains 1 Brave each turn; out of the player's control; XA * 3 / 2 against it; moves without Teleport or Fly |
| 0x16 | Frog | 2, 0x02 | | 11 abilities, Nagrarock | XA 1 on its attacks; XA * 3 / 2 against it; only Attack, Frog Attack and the Frog spell; moves without Teleport or Fly; re-inflicting it removes it |
| 0x17 | Critical | 2, 0x01 | | HP at most max HP / 5 | Triggers HP Restore, MP Restore, Critical Quick and Meatbone Slash ([[battle_reaction_try_while_critical]]); stance 0x24 (as Sleep) |
| 0x18 | Poison | 3, 0x80 | 36 | 10 abilities, 2 weapons; Poison Marsh | Max HP / 8 lost after each turn; cancels Regen |
| 0x19 | Regen | 3, 0x40 | 36 | 4 abilities; the Regenerator reaction | Max HP / 8 restored after each turn, unless poisoned; cancels Poison |
| 0x1a | Protect | 3, 0x20 | 32 | 8 abilities | Physical XA * 2 / 3 against it |
| 0x1b | Shell | 3, 0x10 | 32 | 13 abilities | Magical XA * 2 / 3 against it |
| 0x1c | Haste | 3, 0x08 | 32 | 5 abilities | Speed * 3 / 2 on the clock; cancels Slow |
| 0x1d | Slow | 3, 0x04 | 24 | 10 abilities, Slasher | Speed / 2 on the clock (before Haste); cancels Haste |
| 0x1e | Stop | 3, 0x02 | 20 | 9 abilities | CT frozen; no evades |
| 0x1f | Wall | 3, 0x01 | 24 | nothing (the Wall spell adds Protect and Shell) | An action on it misses unless it removes Wall; no Poach or Train |
| 0x20 | Faith | 4, 0x80 | 32 | 2 abilities, Faith Rod | Its Faith counts as 100 ([[battle_formula_calculate_faith]]); cancels Innocent |
| 0x21 | Innocent | 4, 0x40 | 32 | 2 abilities, Gokuu Rod | Its Faith counts as 0; cancels Faith |
| 0x22 | Charm | 4, 0x20 | 32 | 3 abilities, Fairy Harp | Fights for the attacker's team until it ends; HP damage ends it |
| 0x23 | Sleep | 4, 0x10 | 60 | 15 abilities, 2 weapons | CT frozen; no evades; physical XA * 3 / 2 against it; songs, dances and Talk Skill miss it ([[battle_formula_force_sleeping_target_miss]]); HP damage ends it |
| 0x24 | Don't Move | 4, 0x08 | 24 | 7 abilities, Ancient Sword | No Move |
| 0x25 | Don't Act | 4, 0x04 | 24 | 7 abilities, Spell Edge | No Act ([[battle_unit_get_action_block]]); no evades |
| 0x26 | Reflect | 4, 0x02 | 32 | 3 abilities | A strike that carries no reaction is marked reflected ([[battle_reaction_check_before_formula]], the [[Action]] page) |
| 0x27 | Death Sentence | 4, 0x01 | 3 turns | 7 abilities, Assassin Dagger | Kills at the end of its count (an undead unit just loses it); a second one does not restart it |

## Where to change

- **How long a status lasts, what it cancels, what blocks it, its check
  sets and its preview order:** its record in the status data (disc data,
  [[status_effect_data_t]]).
- **Which statuses an ability, weapon or item adds or removes, and how:** the
  status infliction records (disc data) and
  [[battle_formula_apply_status_to_action]].
- **What a status does:** the functions in the table above; most act inside
  the formulas, the turn clock and the action's checks, not here.
- **What lands and what is blocked:** [[battle_status_modify_inflictions]]
  (immunities, the immortal and rider masks, Frog on a frog) and
  [[battle_status_apply_pending_inflictions]] (counts, Charm and Invite).
- **When counts run and what happens at a turn:**
  [[battle_status_update_expiring]], [[battle_status_build_upkeep_action]],
  [[battle_status_apply_poison_and_regen]],
  [[battle_status_remove_transparent]], called by [[battle_turn_run_clock]].
- **Critical and Chicken thresholds, the HP-damage removals:**
  [[battle_action_apply_target_result]].
- **Who controls an affected unit:** [[battle_status_remove_control]].
- **What events hide:** [[g_battle_event_status_masks]] and
  [[battle_unit_save_staged_status_data]].
- **How a status looks:** [[g_battle_unit_status_mask_by_handler_index]] and
  [[battle_status_queue_runtime_graphics_flag_change]], the bubbles
  ([[g_battle_gfx_status_bubble_status_masks]]), the palette
  ([[battle_gfx_update_unit_status_palette]]), the spritesheet and the
  animation functions in the flow above; the preview image
  ([[g_battle_status_display_image_ids]]).
- The [[Changing a status]] guide walks through a change.
- **Today's limits:** the original build keeps every byte; a change goes behind
  an option into the Developer's Cut, where code can grow and move and
  everything that names it follows (`CODEBASE.md`, "What can change today").

## Quirks and debts

- Oil never raises fire damage (`QUIRKS.md`).
- The preview's 0x100 bit for an `EVENT_EXCLUDED` status changes nothing,
  and animation 0x21 belongs to a mirror bit that no status sets
  (`QUIRKS.md`).
- [[battle_status_resolve_unit_changes_in_preview]] passes no arguments to
  [[battle_status_resolve_unit_changes]], and nothing on the disc references
  it (`QUIRKS.md`).
- Outside execution, [[battle_unit_generate_treasure]] returns no value, so a
  crystal pickup names item 1 or 2 as its treasure (`QUIRKS.md`).
- Check sets `UNUSED_2` and `UNUSED_7`, the status data's flag bits 0x08,
  0x10, 0x20 and 0x40 of the first byte and 0x20 ("Ignore Attacks") of the
  second are built or stored but read by nothing.
- An immortal unit's inflictions are filtered by the formation set, while
  [[main_unit_copy_job_data]] adds both immunity sets only to a unit that is
  immortal and a formation unit at once.

## Functions in scope

![[Status scope]]
