---
type: mechanic
tier: 2
scope:
  - main_item
  - world_item
  - world_shop
  - equip_item
  - equip_unit
  - world_item_count_equipped_by_party()
  - world_unit_can_equip_item_id()
  - world_unit_build_best_fit_equipment()
  - unit_is_two_handing_weapon()
  - battle_unit_find_war_trophies_and_bonus_money()
  - require_reward_collect_war_trophies_and_bonus_money()
  - require_party_store_equipment_or_sell_overflow()
  - require_party_sell_equipment_over_limit()
  - require_party_restore_item_counts()
  - world_menu_has_items()
  - menu_clear_stat_preview()
  - menu_subtract_scaled_stat_preview()
---

# Items and Shops

## What it does in the game

- **Items.** There are 253 items (ids 1 to 0xfd), grouped by id:

  | Ids | Kind |
  |---|---|
  | `0x01`-`0x79` | weapons |
  | `0x7a`-`0x7f` | throwing items (shuriken, balls) |
  | `0x80`-`0x8f` | shields |
  | `0x90`-`0xab` | headgear |
  | `0xac`-`0xcf` | body armor |
  | `0xd0`-`0xef` | accessories |
  | `0xf0`-`0xfd` | consumables (Potion to Phoenix Down) |

  Each has a record in [[g_main_item_primary_data]] (price, type, item level,
  shop level) and, by kind, a weapon, shield, armor, accessory or consumable
  record.
- **The inventory.** [[g_main_item_quantities]] holds a byte per item. The
  party owns at most 99 copies of an item, counting the ones its units have
  equipped ([[main_item_get_total_equipment_quantity]]). A new game starts
  with 5 Potions, 2 Hi-Potions, 2 Eye Drops, 2 Phoenix Downs and one each of
  ten other consumables ([[main_item_init_new_game_inventory]]).
- **Over 99.** A copy that would be the hundredth is turned into gil instead:
  - in battle (a steal, a catch, a treasure) and in the war trophies, for a
    quarter of its price;
  - after the battle, gear that departing units hand back, for a quarter of
    its price ([[require_party_store_equipment_or_sell_overflow]]);
  - on leaving the soldier office, whose recruits bring their own gear, for
    half its price ([[world_item_cash_out_excess_inventory]]).
- **Who can equip what.** A unit can equip an item when its job's
  equippable types include the item's type. The Equip support abilities add
  types (Equip Armor covers headgear and armor), and a female unit can also
  equip ribbons, bags and perfume ([[main_unit_set_equippable_items]],
  [[world_unit_can_equip_item_id]]).

  The hands pair as follows ([[world_formation_can_equip_item_in_slot]],
  [[equip_unit_can_equip_item_id]]):
  - two weapons need Two Swords and two Two Swords compatible weapons;
  - two shields never pair;
  - a shield never pairs with a two-handed-only weapon.

  Equipping an item that does not pair takes the other hand's item off.
- **Best fit.** Best fit picks the best-ranked item for each slot:
  - weapons by power, shields by physical evade, headgear and armor by HP
    bonus, accessories by item level;
  - with the same Two Hands, Two Swords and Martial Arts rules
    ([[world_unit_build_best_fit_equipment]] in the formation,
    [[world_shop_build_fitting_room_best_fit_equipment]] in a shop).
- **Outfitters.** Each of the fifteen towns has an outfitter (shop id = its
  location id). It sells an item when that town's bit is set in the item's
  [[g_world_shop_item_availability]] mask and the item's shop level is at
  most the story's shop progress, `EVENT_SCRIPT_VAR_SHOP_ITEM_AVAILABILITY`
  ([[world_shop_build_item_list]]). The menu is Buy / Sell / Fitting room /
  Leave store.

  | Town | Items stocked | Town | Items stocked |
  |---|---|---|---|
  | Lesalia | 107 | Dorter | 130 |
  | Riovanes | 116 | Zaland | 95 |
  | Igros | 116 | Goug | 49 |
  | Lionel | 116 | Warjilis | 130 |
  | Limberry | 116 | Bervenia | 95 |
  | Zeltennia | 116 | Zarghidas | 130 |
  | Gariland | 105 | | |
  | Yardow | 95 | | |
  | Goland | 95 | | |

  The shop levels run from 1 (Dagger, Broad Sword, Rod...) to 15 (Thief
  Hat). 55 items carry level 20 and no town bit (only bit `0x0001`), so no
  outfitter sells them.
- **Buying and selling.**
  - **Buying:** the quantity stops at what the gil pays for and at 99 owned
    ([[world_shop_run_buy_quantity_menu]]).
  - **Selling:** half the price, at least 1 ([[world_item_get_half_price]]),
    and only copies in the inventory ("They're all equipped. You still gotta
    remove 'em").
  - **Who can equip it:** Triangle in the buy list shows the units that can
    equip the item, with a stat preview.
- **The fitting room.** Units try on shop items before paying:
  - **Try it on:** equips a shop item and adds its price to the fitting
    room's cost. The unit's own item is kept aside
    ([[g_world_shop_fitting_room_items]]).
  - **Best fit:** puts on the best the shop and the unit offer.
  - **Quit fitting:** takes a tried-on item off and puts the unit's own item
    back.
  - **Sell:** sells equipped items (Sell equip) or the stock.
  - **Fitting over:** asks for the total. Paying makes the tried-on items the
    units' own and puts the kept items in the inventory; leaving without
    buying puts everything back ([[world_shop_run_fitting_room_checkout_prompt]]).
- **The fur shop.**
  - **Entry:** it opens only when a roster unit has Secret Hunt.
  - **Its stock** ([[g_main_item_poached_quantities]]) is what monsters
    killed with Secret Hunt leave: the rare item with a chance of 31 in 256,
    else the common one, by monster job
    ([[battle_action_add_poached_item_to_fur_shop_inventory]]).
  - **Buying and selling** are both at half price, and sold copies join the
    stock.
- **The soldier office.** The menu is Male fighter / Female fighter / Change
  name / Leave store.
  - **A fighter:**
    - is generated into the roster with its starting gear;
    - costs 1000 gil plus that gear's prices (1500 and 1400 with the retail
      prices, the amounts the menu shows);
    - is confirmed or dismissed after a preview.
  - **Change name:** renames a monster for free; Reis in dragon form refuses.
- **War trophies.** After a battle each unit on the field gives its war
  trophy item and its bonus money × 100, and the war funds gain the enemies'
  level sum × 100 ([[battle_unit_find_war_trophies_and_bonus_money]],
  [[require_reward_collect_war_trophies_and_bonus_money]]). The war funds
  stop at 99,999,999 everywhere.

## Data and structures

| Record | What it holds |
|---|---|
| [[item_data_t]] | An item (12 bytes, [[g_main_item_primary_data]] at MAIN `0x80062eb8`): palette, sprite, item level, type flags, secondary record, type, attributes, price, shop level |
| [[weapon_data_t]], [[item_evade_t]] (shields and accessories), [[armor_data_t]], [[item_secondary_data_t]] | The kind records ([[g_main_item_weapon_data]] `0x80063ab8`, [[g_main_item_shield_data]] `0x80063eb8`, [[g_main_item_helm_armor_secondary_data]] `0x80063ed8`, [[g_main_item_accessory_data]] `0x80063f58`, [[g_main_item_secondary_data]] `0x80063f98`) |
| [[g_main_item_quantities]] | The inventory, a byte per item (MAIN `0x800596e0`) |
| [[g_main_item_poached_quantities]] | The fur shop's stock, a byte per item (MAIN `0x80059494`) |
| [[g_main_item_poach_table]] | Common and rare item per monster job (MAIN `0x80066064`, 48 rows) |
| [[g_world_shop_item_availability]] | Each item's town mask, most significant byte first: bit `0x8000 >> town` (WORLD `0x8018d844`) |
| [[g_world_shop_menu_step_handlers]] | The shop screen's 28 steps ([[world_shop_step_e]], WORLD `0x8019523c`) |
| [[g_world_shop_fitting_room_items]] | Per unit and slot, the own item kept aside while it tries on a shop item |
| [[battle_war_result_t]] | A battle's war trophies and money |

## Main flow

1. **Entering a shop.** WLDCORE's town menu calls [[world_shop_run_screen]]
   with the outfitter, the fur shop or the soldier office. It runs one step
   handler a frame until the step is `WORLD_SHOP_STEP_EXIT`, then saves the
   formation records to the roster.
2. **Outfitter.** [[world_shop_run_main_menu_step]] leads to:
   - the buy list ([[world_shop_run_buy_list_step]],
     [[world_shop_run_buy_confirm_step]]);
   - the sell list ([[world_shop_run_sell_list_step]],
     [[world_shop_run_sell_confirm_step]]);
   - the fitting room ([[world_shop_run_fitting_room_step]] and its steps,
     ending in [[world_shop_run_fitting_room_checkout_step]]).
3. **Fur shop and soldier office.** They start at [[world_shop_run_fur_menu_step]]
   and [[world_shop_run_soldier_office_menu_step]].
4. **Elsewhere.**
   - the formation's item screen ([[world_item_run_category_menu]]) and
     equipment menus;
   - BATTLE's EQUIP screen (`equip_*`, the same rules as WORLD's
     `world_item_*`);
   - item gains in battle (Action page);
   - the war trophies and departing units' gear after a battle (REQUIRE).

## Where to change

- **An item's stats, price or kind:** the item records (disc data).
- **What a town sells and when:** [[g_world_shop_item_availability]] and the
  item's shop level; the progress variable is set by event scripts.
- **Sale and overflow rates:**
  - half price: [[world_item_get_half_price]];
  - a quarter in battle and after it: [[battle_action_add_war_funds]],
    [[require_party_store_equipment_or_sell_overflow]],
    [[require_reward_collect_war_trophies_and_bonus_money]].
- **Who can equip what:** the job's equippable types (job data),
  [[main_unit_set_equippable_items]] for the Equip supports, and the hand
  rules in [[world_formation_can_equip_item_in_slot]] and
  [[equip_unit_can_equip_item_id]].
- **The fur shop:** its gate in [[world_shop_run_fur_menu_step]], its stock
  in [[battle_action_add_poached_item_to_fur_shop_inventory]] and the poach
  table.
- **Recruits:** the starting gear in [[g_main_unit_generation_base_data]];
  the fee in [[world_shop_run_soldier_office_menu_step]] and the amounts the
  menu shows in [[world_shop_get_soldier_office_entry_fee]].

[[Changing items and shops]] walks through these changes.

## Quirks and debts

- The evade sorts of [[world_item_sort_id_list]] and
  [[equip_item_sort_list_by_criteria]] index the shield and accessory tables
  with throwing items and armor. The evade keys only break ties of type, price
  and ranking, and in the item data only Cachusha and Barette tie, so retail
  puts Barette first. The Developer's Cut gives headgear and armor no evade and
  splits weapons from shields at the first shield (`QUIRKS.md`).
- [[world_item_sort_id_list]] counts each unit's equipment by id, and an
  unpaid fitting-room item's flag sends its count 32 KB past the array; the
  Developer's Cut leaves it out of the count (`QUIRKS.md`).
- [[world_item_cash_out_excess_inventory]] never reaches Phoenix Down.
- Fur shop quirks:
  - its "sell" list is the ordinary inventory at half price;
  - selling adds the copies to the poached stock;
  - buying divides the gil by half the price with no lower bound
    ([[world_shop_run_fur_buy_quantity_menu]]).
- [[world_item_build_category_list]] and [[world_shop_build_item_list]] treat
  the two hand filters the same.
- WLDCORE opens every outfitter with shop id 0; [[world_shop_run_screen]]
  takes the town from the location variable.
- [[battle_unit_find_war_trophies_and_bonus_money]] computes a level bonus
  that nothing pays.
- EQUIP never fills the accessory's physical evade slot of its unit
  preview.

## Functions in scope

![[Items and Shops scope]]
