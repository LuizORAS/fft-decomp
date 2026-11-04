---
type: guide
mechanics: [Items and Shops]
---

# Changing items and shops

## Goal

Change an item (its stats, price or kind), what a town's outfitter sells and
from when, the sale and overflow rates, who can equip what, the fur shop or
the soldier office's recruits.

## Where it lives

Most of it is data. The [[Items and Shops]] page has the retail values.

| What | Where |
|---|---|
| An item's price, type, item level and shop level | its [[item_data_t]] record ([[g_main_item_primary_data]], MAIN `0x80062eb8`, 12 bytes an item, disc data; price at `+8`, shop level at `+10`) |
| A weapon's range, formula, power, evade, element, added status or spell and flags; a shield's or accessory's evades; headgear's and armor's HP and MP | the kind records ([[g_main_item_weapon_data]], [[g_main_item_shield_data]], [[g_main_item_helm_armor_secondary_data]], [[g_main_item_accessory_data]], disc data), chosen by the item's `secondary_data_id` |
| A consumable's effect | [[g_main_item_secondary_data]] (formula, Z, status), read by the item formulas |
| Which towns sell an item | [[g_world_shop_item_availability]] (WORLD `0x8018d844`, two bytes an item, most significant first: bit `0x8000 >> town`) |
| From when | the item's shop level against `EVENT_SCRIPT_VAR_SHOP_ITEM_AVAILABILITY`, which event scripts raise ([[world_shop_build_item_list]]) |
| Sale price | [[world_item_get_half_price]] (half, at least 1) |
| What a hundredth copy gives | [[battle_action_add_war_funds]] (a quarter, in battle), [[require_reward_collect_war_trophies_and_bonus_money]] and [[require_party_store_equipment_or_sell_overflow]] (a quarter, after a battle), [[world_item_cash_out_excess_inventory]] (half, leaving the soldier office) |
| Who can equip what | the job's equippable types (job data); [[main_unit_set_equippable_items]] for the Equip supports and the female-only types; the hand rules in [[world_formation_can_equip_item_in_slot]] (WORLD) and [[equip_unit_can_equip_item_id]] (BATTLE's EQUIP) |
| Best fit | [[world_item_get_ranking_value]] and [[equip_item_get_ranking_value]] (what an item ranks by), sort mode 7 of [[world_item_sort_id_list]] |
| The fur shop | its Secret Hunt gate in [[world_shop_run_fur_menu_step]]; its stock from [[battle_action_add_poached_item_to_fur_shop_inventory]] and [[g_main_item_poach_table]] (MAIN `0x80066064`, common and rare item per monster job) |
| Recruits | starting gear in [[g_main_unit_generation_base_data]]; the fee (1000 + the gear's prices) in [[world_shop_run_soldier_office_menu_step]]; the amounts the menu shows in [[world_shop_get_soldier_office_entry_fee]] |
| The new game's inventory | [[main_item_init_new_game_inventory]] |

## What changes with it

- **Two copies of the item rules.** WORLD's formation and shops
  (`world_item_*`, `world_unit_can_equip_item_id`) and BATTLE's EQUIP screen
  (`equip_item_*`, `equip_unit_*`) carry the same rules twice. A rule changed
  in one should change in the other.
- **The fee the menu shows.** [[world_shop_get_soldier_office_entry_fee]]
  shows fixed amounts (1500 and 1400). A new price for the starting gear
  changes the fee charged but not the one shown.
- **The 99 limit.** Purchases, battle gains, war trophies and the soldier
  office all test 99 owned, counting equipped copies. The inventory is a byte
  per item.
- **Item lists and sorts.** The shop lists, the formation's item screen and
  EQUIP sort with the same key lists. The evade keys misread some kinds
  (`QUIRKS.md`).
- **Text.** Item names and descriptions are in the text files (item names
  are text `0x3800 +` id, descriptions `0x6800 +` id). The shopkeepers' lines
  are WORLD's resident shop text.

## What can be done today

Nothing that changes a byte of the original build: `make check` compares every
function with its original hash, and the item, shop and poach tables are disc
data, which the repository does not hold. To try an idea, change the game's
memory in an emulator: `make run` launches PCSX-Redux, and `make map` writes
the symbol maps that name these tables in its debugger. For example, an item's
price is the 16-bit value at `0x80062eb8 + id * 12 + 8`, and its town mask the
two bytes at `0x8018d844 + id * 2` while WORLD is loaded. In the Developer's
Cut, a change goes behind an option: code under `#ifdef` and a table's bytes in
place through `target/edits.yaml` (`AGENTS.md`, "Changes under an option").
