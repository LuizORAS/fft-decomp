---
type: guide
mechanics: [Status]
---

# Changing a status

## Goal

Change what a status does, how long it lasts, what it cancels or what blocks
it; or change which statuses an ability, weapon or item adds or removes.

## Where it lives

A status is part data, part code. The [[Status]] page lists the 40 statuses
with what the code does for each.

| What | Where |
|---|---|
| Count, cancels, "blocked by", preview order, check-set flags | the status's record in the status data ([[status_effect_data_t]], MAIN `0x80065de4`, disc data) |
| Which statuses an ability, weapon or item adds or removes, and how (All or Nothing, Random, Separate, Cancel) | the status infliction records ([[status_infliction_data_t]], MAIN `0x80063fc4`, disc data), the record id in the ability, weapon or item data, and [[battle_formula_apply_status_to_action]] |
| What the status does in a formula | the `battle_formula_*` helper the [[Status]] table names (for example [[battle_formula_apply_target_physical_status_xa_modifiers]] for Protect, Sleep, Chicken and Frog) |
| What it does to the turn | [[battle_turn_get_status_flags]] (CT frozen, Haste, Slow), [[battle_status_build_upkeep_action]], [[battle_status_update_expiring]], [[battle_status_apply_poison_and_regen]], [[battle_status_remove_transparent]] |
| Who controls the unit | [[battle_status_remove_control]] and [[battle_status_apply_pending_inflictions]] (Charm and Invite) |
| What lands and what is filtered | [[battle_status_modify_inflictions]] (immunities, the immortal and rider masks, the "blocked by" lists, Frog on a frog) |
| Statuses the engine sets itself | [[battle_action_apply_target_result]] (Dead, Critical, Chicken, the HP-damage removals) |
| How it looks | [[g_battle_unit_status_mask_by_handler_index]] and [[battle_status_queue_runtime_graphics_flag_change]] (the runtime mirror), [[g_battle_gfx_status_bubble_status_masks]] (the bubble), [[battle_gfx_update_unit_status_palette]], [[battle_gfx_apply_status_spritesheet_change]], [[battle_unit_set_animation_based_on_status]], [[g_battle_status_display_image_ids]] (the preview) |

A status's effect is rarely in one place: a test such as
`status_sets.current[BATTLE_STATUS_BYTE_INDEX(BATTLE_STATUS_ID_SLEEP)]`
appears wherever the rule applies. Searching for the status's
`BATTLE_STATUS_ID_*` name (and its `BATTLE_MISC_STATUS_*` mirror bit) finds
them; some older code tests the raw byte and mask instead.

## What changes with it

- **The check sets.** A status's flag bits put it in the sets of
  [[main_status_check_set_e]] (KO, lost, CT freeze, no reactions, the
  immunities, event-excluded, mount removal). Changing a flag changes every
  reader of that set, listed on the [[Status]] page.
- **The AI.** The AI scores statuses with its own tables and checks
  (`battle_ai_classify_ability_effect` and its neighbors), so a new effect
  does not change how the AI values the status.
- **Estimates.** Previews and AI simulations run the same status code;
  only execution queues the graphics
  ([[battle_status_queue_graphics_change_if_executing]]).
- **Events.** An event hides and restores statuses through
  [[g_battle_event_status_masks]]; a new status that should not show in a
  cutscene needs its bit there.
- **WORLD and the menus.** WORLD has its own copies of the event staging
  (`world_unit_apply_staged_status_data`, `world_script_inflict_status_thread`),
  and the unit status lists of BATTLE, WORLD and the menu overlays name the
  statuses by bit.
- **Help text and names** are game text, separate from the code.
- **Quirks.** The [[Status]] page lists the `QUIRKS.md` entries that touch
  statuses (Oil, the preview's unused 0x100 bit, animation 0x21).

## What can be done today

Nothing that changes a byte of the original build: `make check` compares every
function with its original hash, and the status data and infliction records are
disc data, which the repository does not hold. To try an idea, change the
game's memory in an emulator: `make run` launches PCSX-Redux, and `make map`
writes the symbol maps that name these functions and tables in its debugger
(the status data starts at `0x80065de4`, 16 bytes per status). In the
Developer's Cut, a change goes behind an option: code under `#ifdef` and a
table's bytes in place through `target/edits.yaml` (`AGENTS.md`, "Changes under
an option").
