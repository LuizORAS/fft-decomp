---
type: guide
mechanics: [Roster and Formation]
---

# Changing the formation and roster

## Goal

Change what the Formation screen allows: who can be removed, who is locked
out of the menus, how the hands pair, what a job change keeps or re-equips,
and how the roster is ordered.

## Where it lives

All of it is WORLD code and a few WORLD and MAIN tables. The
[[Roster and Formation]] page describes the retail rules.

| What | Where |
|---|---|
| Who can be removed | [[world_formation_can_dismiss_unit]] (Ramza's sprite sets and the guest slots 16-19 cannot); a unit on a proposition is refused first in [[world_formation_run_dismiss_unit_menu]] |
| What a dismissal tells the story | [[g_world_formation_dismiss_entries]] (sprite set, script variable set to `0xC`) |
| A story character's farewell and refusal lines | text `0xe800 +` sprite set + `0x17` (farewell) or `0x61` (refusal) in the dismiss reply text; generic units draw one of eight lines by sex or monster |
| Who is locked out of Item, Ability and Change Job | [[world_formation_get_unit_lock]] (proposition, monster, egg, Mime) and the row tests in [[world_formation_run_item_menu]] and [[world_formation_run_ability_menu]] |
| Hand pairing | [[world_formation_can_equip_item_in_slot]]; the weapon flags it reads are item data (Two Swords compatible, two-handed only) |
| What a job change drops | [[world_formation_remove_invalid_unit_loadout]] (the innate supports Chemist/Throw Item, Monk/Martial Arts, Mediator/Monster Talk and Ninja/Two Swords are literals there) |
| When the best fit is re-equipped | [[world_formation_change_unit_job]] (job wheel), [[world_formation_set_unit_ability_slot]] with [[world_ability_is_equip_support_change]] (supports), both gated by the Max Equip at Job Change option |
| The roster order | [[g_world_formation_sort_field_indices]] (WORLD `0x8018d7cc`: the record 16-bit value each key compares), the key list in MAIN ([[g_main_item_type_order_1]], saved with the game), [[world_formation_sort_roster]] (Number ascending, the rest descending, eggs last) |
| The cards' readouts | [[world_formation_draw_unit_readout]] and the callbacks [[world_formation_init_selection_cursor]] installs |

## What changes with it

- **BATTLE's copies.** EQUIP ([[equip_unit_can_equip_item_id]]) applies the
  hand rules again in battle, and BUNIT has its own unit list. A rule changed
  in WORLD should change there too.
- **The shops.** The outfitter's fitting room, the fur shop and the soldier
  office build the same formation records and use the same equipment rules,
  so a change here reaches them.
- **The save.** The Order Unit keys are saved with the game; the load
  rejects a saved list that holds key 11 and keeps the current one.
- **Text.** The menu rows and messages are WORLD's resident formation text
  (`0xd800 +` n and `0xc000 +` n); a new row or message needs its text there.
- **Roster size.** 20 records, the grid's 4 x 5 layout, the scroll positions
  and the record arrays (20 records plus the preview record) all assume it.

## What can be done today

Nothing that changes a byte of the original build: `make check` compares every
function with its original hash. To try an idea, change the game's memory in an
emulator: `make run` launches PCSX-Redux, and `make map` writes the symbol maps
that name these tables in its debugger. For example, the Order Unit keys are
the `-1`-terminated bytes at [[g_main_item_type_order_1]], and the field each
key compares is the byte at `0x8018d7cc + key` while WORLD is loaded. In the
Developer's Cut, a change goes behind an option: code under `#ifdef` and a
table's bytes in place through `target/edits.yaml` (`AGENTS.md`, "Changes under
an option").
