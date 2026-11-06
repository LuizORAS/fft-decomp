---
type: mechanic
tier: 1
scope:
  - battle_formula
---

# Formulas

## What it does in the game

A formula turns one action on one target into a result: whether it hits, how
much HP or MP it takes or gives, and which statuses and special effects it
adds. Every ability, weapon and consumable names a formula id, and the engine
runs that id's handler once per target and strike.

- **XA and YA.** Most formulas build two numbers. XA is the attacker's power
  (PA, MA, Speed, or a mix by weapon type) and YA the multiplier (the weapon's
  power, the ability's Y, or another stat). Damage and healing are XA * YA;
  a hit chance is XA + YA, as a percentage. X and Y come from the ability's
  data, Z from a consumable's.
- **Modifiers on XA.** Physical: Two Hands, Attack Up and Martial Arts; the
  attacker's Berserk and Frog; the target's Defense Up, Protect, Sleep,
  Charging, Chicken and Frog. Magical: Magic Attack Up, Magic Defense Up, the
  target's Shell, Frog and Chicken. Both: Strengthen for the element (XA *
  5 / 4), the zodiac compatibility (best +50%, good +25%, bad -25%, worst
  -50%) and Charge's power. A critical hit (4 in 100 while executing) grows
  XA by up to XA - 1 and may knock the target back.
- **Elements and weather.** After XA * YA, a storm cuts fire to 3/4 and
  raises lightning to 5/4, a snowstorm raises ice to 5/4; then the target's
  affinities: Absorb turns the damage into healing, Null nullifies the hit,
  Half halves and Weak doubles it. Oil should double fire damage but never
  does (`QUIRKS.md`).
- **Faith.** Magic scales by target Faith * attacker Faith / 10000; the
  Faith status counts as 100 and Innocent as 0. Some formulas skip Faith,
  and the Un-Truth formula scales by (100 - Faith) instead.
- **Evasion.** Physical attacks roll the target's accessory, shield and class
  evades in turn, each as evade / base hit (100). From the side the class
  evade is lost, from behind the shields too. Concentrate and Transparent
  remove the evades; the attacker's Darkness or Confusion, the target's
  Abandon and Defending double them; a sleeping, stopped, confused, charging,
  performing or Don't Act target loses them. Bows and crossbows lose a quarter of the
  base hit at night and another quarter in a storm. Magic rolls only the
  accessory and shield magic evades, whatever the direction. Only the basic
  attack and abilities flagged evadeable are evaded.
- **Hit chance.** A formula with a hit chance rolls it once: 100 or more
  always hits, 0 always misses.
- **Added effects.** A weapon's added status, a damaging spell's status and
  a spell-casting weapon's spell trigger on a 0-99 roll below 19 while
  executing; a preview always shows them, the AI never counts them. A weapon
  spell follows the hit, and is dropped when the hit kills.
- **Statuses.** A status set is added All or Nothing, as one Random status,
  each status Separately (24 in 100 each, and a quarter of the shown
  accuracy), or as removals (Cancel). Outside execution Random and Separate
  show the whole set. A set that changes nothing (immunity, a status already
  present) makes the action fail.
- **Estimates.** The menus' preview and the AI run the same handlers. Their
  random rolls take the middle value, so an estimate shows average damage and
  never a critical hit.

## Data and structures

- **The handler table.** [[g_battle_formula_handlers]] (BATTLE `0x8018f610`)
  holds the handlers for ids 0x01-0x64. [[ability_formula_e]] names the ids
  the engine tests itself.
- **The formula's inputs.** [[g_battle_action_current_ability]]
  ([[battle_current_ability_t]]): `xa` and `ya`, the base hit and the four
  evades, both Faiths, copies of the ability's range data (X, Y, element,
  status id), of the weapon's data (power, element, formula, added status or
  spell in `proc_id`) and of the status set, Charge's power and the formula
  id. [[battle_action_run_pre_formula_setup]] fills it for each target.
- **The result.** [[battle_action_data_t]] for the target
  ([[g_battle_action_target_data]]) and for the actor
  ([[g_battle_action_attacker_data]]): hit, miss type, HP and MP damage and
  healing, stat changes (`BATTLE_ACTION_STAT_CHANGE_*`, CT
  `BATTLE_ACTION_CT_CHANGE_*`), special effects
  ([[battle_action_special_effect_e]]), statuses to add and remove, and the
  result type ([[battle_action_type_flags_e]]). While the hit chance is built,
  `hp_damage` holds it.
- **When it runs.** [[g_battle_action_state]]: executing, AI simulation or
  preview.
- **Game data in MAIN.** The ability range data
  ([[g_main_ability_range_data]], 14 bytes per ability: range, area,
  vertical, four flag bytes, element, formula, X, Y, status, CT, MP); the
  weapon data ([[g_main_item_weapon_data]], 8 bytes: range, flags, formula,
  power, evade, element, added status or spell); the consumables
  ([[g_main_item_secondary_data]]: formula, Z, status). The disc holds these
  tables, not the repository.

## Main flow

1. **The formula id.** [[battle_action_run_pre_formula_setup]] picks it by
   command: the ability's, the consumable's, 0x63 for Throw, 0x64 for Jump, or
   the weapon's for Attack and Charge (Charge only adds its power). Formula 0
   or one above 0x64 becomes 0x01, and formula 0x03 loads no status. The
   reactions that come before the formula may make the strike miss first
   (Reflect, Blade Grasp, Arrow Guard).
2. **The handler.** It typically checks evasion
   ([[battle_formula_calculate_physical_evade]] or
   [[battle_formula_calculate_magical_evade]]), builds XA and YA (a
   `battle_formula_store_*` helper or [[battle_formula_calculate_base_xa]]),
   applies the modifiers, then either rolls a hit chance
   ([[battle_formula_store_hit_chance]], [[battle_formula_roll_hit_chance]])
   or stores the damage ([[battle_formula_calculate_physical_damage]],
   [[battle_formula_calculate_elemental_xa_times_ya]]), scales it by Faith
   ([[battle_formula_calculate_faith]]), applies the element
   ([[battle_formula_apply_element_affinities]],
   [[battle_formula_apply_elemental_absorption]]) and rolls the added effect
   ([[battle_formula_roll_conditional_status_proc]]) before the status
   ([[battle_formula_apply_status_to_action]]).
3. **After it.** [[battle_action_finalize_target_current_action]] checks the
   result (999 caps, invalid targets); formulas below 0x07 check Poach and
   Train ([[battle_formula_apply_poach_and_train]]), and Jump drops its
   weapon. The [[Action]] page applies the result.

### The formulas

"Hit" is the hit chance; "with Faith" scales it by both Faiths. Users are the
retail abilities, weapons, consumables or commands that select the formula.

| Id | Handler | Users | What it does |
|---|---|---|---|
| 0x01 | [[battle_formula_weapon_damage]] | Attack with 112 weapons or bare hands; monster attacks such as Tackle and Bite | Physical evade; weapon damage; 19% added status |
| 0x02 | [[battle_formula_weapon_damage_with_proc]] | Ice Brand, Flame Rod, Ice Rod, Thunder Rod, Flame Whip, Lightning Bow, Holy Lance | As 0x01; on 19% the weapon's spell follows the hit |
| 0x03 | [[battle_formula_gun_damage]] | Romanda Gun, Mythril Gun, Stone Gun | WP * WP; no evade, no status |
| 0x04 | [[battle_formula_magic_gun]] | Blaze Gun, Glacier Gun, Blast Gun | A Fire, Ice or Bolt spell: level 1 (60%), 2 (30%) or 3 (10%), WP * Y, with Faith; no evade |
| 0x05 | [[battle_formula_weapon_damage_without_element]] | None | A weapon strike without the bow weather penalty or the element |
| 0x06 | [[battle_formula_weapon_absorb_hp]] | Blood Sword, Bloody Strings | Weapon damage drained as HP |
| 0x07 | [[battle_formula_weapon_heal]] | Healing Staff | XA * YA healing; cannot miss; no reactions |
| 0x08 | [[battle_formula_magical_damage]] | 41 attack spells and summons | Magic evade; MA * Y with Faith, weather and element; 19% status |
| 0x09 | [[battle_formula_magic_hp_percent_damage]] | Demi, Demi 2, Lich | Hit MA + X with Faith; Y% of max HP |
| 0x0a | [[battle_formula_hit_faith_ma_x_percent]] | 38 status spells | Magic evade; hit MA + X with Faith; status |
| 0x0b | [[battle_formula_friendly_hit_faith_ma_x_percent]] | Supporting spells (Protect, Shell, Haste, Regen, Reflect...) | Hit MA + X with Faith; status |
| 0x0c | [[battle_formula_magical_heal]] | Cure to Cure 4, Moogle, Fairy | MA * Y healing with Faith |
| 0x0d | [[battle_formula_heal_y_percent_faith]] | Raise, Raise 2 | Hit MA + X with Faith; status; Y% healing |
| 0x0e | [[battle_formula_damage_hp_percent_hit_faith_ma_x_percent]] | Death | Hit MA + X with Faith; status; Y% damage, healing for the undead |
| 0x0f | [[battle_formula_absorb_mp_y_percent]] | Spell Absorb, Aspel | Hit MA + X with Faith; Y% of max MP drained |
| 0x10 | [[battle_formula_absorb_hp_y_percent]] | Life Drain, Drain | Hit MA + X with Faith; Y% of max HP drained |
| 0x11 | [[battle_formula_11_unused]] | None | Nothing |
| 0x12 | [[battle_formula_set_quick]] | Quick | Hit MA + X with Faith; Quick |
| 0x13 | [[battle_formula_13_unused]] | None | Nothing |
| 0x14 | [[battle_formula_set_golem]] | Golem | Hit from the caster's MA + X and Faith; the Golem guard |
| 0x15 | [[battle_formula_set_ct_zero]] | Return 2 | Hit MA + X with Faith; CT 0 |
| 0x16 | [[battle_formula_damage_mp_target_current_mp]] | Mute | Hit MA + X with Faith; all MP lost |
| 0x17 | [[battle_formula_damage_target_hp_minus_one]] | Gravi 2 | Hit MA + X with Faith; damage of current HP - 1 |
| 0x18 | [[battle_formula_18_unused]] | None | Nothing |
| 0x19 | [[battle_formula_19_unused]] | None | Nothing |
| 0x1a | [[battle_formula_magical_stat_reduction]] | Speed Ruin, Power Ruin, Mind Ruin | Hit MA + Y with Faith; Speed, PA or MA - X |
| 0x1b | [[battle_formula_damage_mp_percent]] | Magic Ruin | Hit MA + X with Faith; Y% of max MP |
| 0x1c | [[battle_formula_song]] | The Bard's songs | X% hit; MA + Y effects |
| 0x1d | [[battle_formula_dance]] | The Dancer's dances | X% hit; effects from the dancer's weapon |
| 0x1e | [[battle_formula_multihit_truth_magic]] | Rafa's Truth abilities, Holy Bracelet | MA * (MA + Y) / 2 without Faith; 1 to X strikes |
| 0x1f | [[battle_formula_unfaith_magical_damage]] | Malak's Un-Truth abilities | MA * (MA + Y) / 2 scaled by 100 - Faith; 1 to X strikes |
| 0x20 | [[battle_formula_draw_out_damage]] | Draw Out: Asura, Koutetsu, Heaven's Cloud, Muramasa, Kikuichimoji, Chirijiraden | Katana break roll; MA * Y without Faith |
| 0x21 | [[battle_formula_draw_out_mp_damage]] | Bizen Boat | Katana break roll; MA * Y MP damage |
| 0x22 | [[battle_formula_draw_out_status]] | Kiyomori, Masamune | Katana break roll; status |
| 0x23 | [[battle_formula_draw_out_heal]] | Murasame | Katana break roll; MA * Y healing |
| 0x24 | [[battle_formula_ma_pa_half_damage]] | Elemental | Magic evade; MA * (PA + Y) / 2 without Faith |
| 0x25 | [[battle_formula_break_equipped_hit_pa_wp_y_percent]] | Head, Armor, Shield and Weapon Break | Hit PA + WP + Y; breaks the piece; nothing to break makes it an Attack |
| 0x26 | [[battle_formula_steal_equipment]] | Steal Helmet, Armor, Shield, Weapon, Accessory | Hit Speed + X; steals the piece |
| 0x27 | [[battle_formula_steal_gil]] | Gil Taking, Shine Lover | Hit Speed + X; Speed * Level gil |
| 0x28 | [[battle_formula_steal_exp_hit_sp_x_percent]] | Steal Exp | Hit Speed + X; Speed + Y EXP |
| 0x29 | [[battle_formula_opposite_sex_hit_ma_x_percent]] | Steal Heart, Allure, Nose Bracelet | Hit MA + X; fails on the same sex |
| 0x2a | [[battle_formula_talk_skill_hit_ma_x_percent]] | Talk Skill | Monster Talk; Finger Guard; hit MA + X; the talk's effect |
| 0x2b | [[battle_formula_physical_stat_reduction]] | Speed Break, Power Break, Mind Break | Hit PA + Y; Speed, PA or MA - X |
| 0x2c | [[battle_formula_physical_mp_percent_damage]] | Magic Break | Hit PA + Y; Y% of max MP |
| 0x2d | [[battle_formula_damage_pa_times_wp_plus_y_status]] | Holy Sword | PA * (WP + Y), the weapon's element; status on every hit |
| 0x2e | [[battle_formula_break_equipped_damage_pa_times_wp]] | Might Sword | PA * WP; breaks the piece; fails with nothing to break |
| 0x2f | [[battle_formula_absorb_mp_pa_times_wp]] | Dark Sword | PA * WP drained as MP |
| 0x30 | [[battle_formula_absorb_hp_pa_times_wp]] | Night Sword | PA * WP drained as HP |
| 0x31 | [[battle_formula_damage_pa_plus_y_half_times_pa]] | Spin Fist, Wave Fist, Earth Slash and six more | PA * (PA + Y) / 2; 19% status |
| 0x32 | [[battle_formula_damage_random_x_times_pa_plus_pa_plus_y_half]] | Repeating Fist | (PA + (PA + Y) / 2) times 1 to X; no hit roll |
| 0x33 | [[battle_formula_hit_pa_x_percent]] | Stigma Magic | Hit PA + X; status |
| 0x34 | [[battle_formula_heal_pa_times_y_and_mp]] | Chakra | PA * Y HP and half as MP, the undead too |
| 0x35 | [[battle_formula_heal_y_percent_hit_pa_x_percent]] | Revive, Oink | Hit PA + X; status; Y% healing |
| 0x36 | [[battle_formula_add_pa_y]] | Accumulate, Gather Power | PA + Y |
| 0x37 | [[battle_formula_damage_random_y_times_pa]] | Dash, Throw Stone, Cat Kick, Tail Swing | PA times 1 to Y; knockback |
| 0x38 | [[battle_formula_apply_status_to_action]] | Heal, Shadow Stitch, Stop Bracelet, Grand Cross and 18 more | Status; always hits |
| 0x39 | [[battle_formula_add_speed_y]] | Yell | Speed + Y |
| 0x3a | [[battle_formula_add_brave_y]] | Cheer Up | Brave + Y |
| 0x3b | [[battle_formula_add_brave_x_stats_y]] | Scream | Brave + X; PA, MA and Speed + Y |
| 0x3c | [[battle_formula_3c_damage_caster_max_hp_one_fifth_heal_target_two_fifths]] | Wish, Energy | The caster loses a fifth of its max HP; the target heals twice that |
| 0x3d | [[battle_formula_hit_ma_x_percent_magical_evade]] | Blaster, Mind Blast, Death Sentence (monster) | Magic evade; hit MA + X; status |
| 0x3e | [[battle_formula_damage_target_hp_minus_one_always_hit]] | None | Damage of current HP - 1, no checks |
| 0x3f | [[battle_formula_hit_sp_x_percent]] | Leg Aim, Arm Aim | Hit Speed + X; status |
| 0x40 | [[battle_formula_undead_hit_sp_x_percent]] | Seal Evil | As 0x3f, undead targets only |
| 0x41 | [[battle_formula_hit_ma_x_percent_miss_same_zodiac]] | Galaxy Stop | Hit MA + X; fails on the caster's zodiac sign |
| 0x42 | [[battle_formula_damage_pa_times_y_damage_caster_pa_times_y_over_x]] | Destroy, Compress, Dispose, Crush | PA * Y; the caster takes a 1/X share |
| 0x43 | [[battle_formula_43_damage_caster_missing_hp]] | Shock, Blade Beam, Ulmaguest, Lifebreak | The caster's max HP - HP |
| 0x44 | [[battle_formula_damage_hp_target_current_mp]] | Difference | The target's current MP as damage |
| 0x45 | [[battle_formula_damage_target_missing_hp]] | Climhazzard | The target's max HP - HP |
| 0x46 | [[battle_formula_46_unused]] | None | Nothing |
| 0x47 | [[battle_formula_absorb_hp_y_percent_status]] | Blood Suck | Y% of max HP drained; status |
| 0x48 | [[battle_formula_heal_z_times_ten]] | Potion, Hi-Potion, X-Potion | Z * 10 HP (30, 70, 150) |
| 0x49 | [[battle_formula_heal_mp_z_times_ten]] | Ether, Hi-Ether | Z * 10 MP (20, 50) |
| 0x4a | [[battle_formula_heal_full_hp_mp]] | Elixir | All HP and MP |
| 0x4b | [[battle_formula_heal_random_one_to_z_add_status]] | Phoenix Down | Status; 1 to 20 HP |
| 0x4c | [[battle_formula_heal_ma_times_y]] | Choco Cure, Spirit of Life | MA * Y healing |
| 0x4d | [[battle_formula_absorb_hp_y_percent_hit_ma_x_percent]] | Mutilate, Drain Touch | Hit MA + X; Y% of max HP drained |
| 0x4e | [[battle_formula_damage_ma_times_y]] | 25 abilities: Cloud's limits, the Bracelets, monster attacks | Magic evade; MA * Y without Faith |
| 0x4f | [[battle_formula_damage_caster_missing_hp_hit_ma_x_percent]] | Goblin Punch | Hit MA + X; the caster's max HP - HP |
| 0x50 | [[battle_formula_hit_ma_x_percent_physical_evade]] | Secret Fist and monster touches | Physical evade; hit MA + X; status |
| 0x51 | [[battle_formula_hit_ma_x_percent_no_evade]] | Choco Esuna, Protect Spirit, Clam Spirit | Hit MA + X; status |
| 0x52 | [[battle_formula_damage_caster_missing_hp_self_sacrifice]] | Self Destruct | The caster's max HP - HP and status; the caster takes its own HP |
| 0x53 | [[battle_formula_damage_y_percent_hit_ma_x_percent]] | Hurricane, Triple Bracelet | Hit MA + X; Y% of max HP |
| 0x54 | [[battle_formula_heal_mp_ma_times_y]] | Magic Spirit | MA * Y MP |
| 0x55 | [[battle_formula_lower_pa_y]] | Beaking | Hit MA + X; PA - Y |
| 0x56 | [[battle_formula_lower_ma_y]] | Circle | Hit MA + X; MA - Y |
| 0x57 | [[battle_formula_raise_level_by_one_add_status_on_caster]] | Please Eat | Full restore and a level; the caster takes the status |
| 0x58 | [[battle_formula_set_morbol]] | Moldball Virus | Hit MA + X; turns the target into a Morbol |
| 0x59 | [[battle_formula_lower_level_by_one]] | Level Blast | Hit MA + X; Level - 1 |
| 0x5a | [[battle_formula_dragon_hit_100]] | Dragon Tame | Dragons and hydras; status |
| 0x5b | [[battle_formula_dragon_wish_add_status]] | Dragon Care | Dragons and hydras; Wish's exchange; status |
| 0x5c | [[battle_formula_dragon_brave_x_stats_y]] | Dragon Power Up | Dragons and hydras; Brave + X, stats + Y |
| 0x5d | [[battle_formula_dragon_set_quick]] | Dragon Level Up | Dragons and hydras; Quick |
| 0x5e | [[battle_formula_5e_damage_ma_plus_y_half_times_ma]] | Triple Thunder, Triple Flame, Dark Whisper | MA * (MA + Y) / 2 without Faith; X + 1 strikes |
| 0x5f | [[battle_formula_5f_damage_ma_plus_y_half_times_ma]] | Nanoflare | MA * (MA + Y) / 2 without Faith |
| 0x60 | [[battle_formula_60_damage_ma_plus_y_half_times_ma]] | None | As 0x5f without the evade check |
| 0x61 | [[battle_formula_lower_brave_y]] | Foxbird, Chicken | Hit MA + X with Faith; Brave - Y |
| 0x62 | [[battle_formula_lower_brave_y_without_faith]] | Look of Fright | Hit MA + X; Brave - Y |
| 0x63 | [[battle_formula_throw_damage_sp_times_wp]] | Throw | Catch; Speed * the thrown item's power |
| 0x64 | [[battle_formula_jump_damage_pa_times_wp]] | Jump | PA (* 3 / 2 with a spear) * WP; never evaded |

## Where to change

- **A formula's steps:** its handler in the table above. Formulas share the
  helpers, so a change in a `battle_formula_apply_*` or
  `battle_formula_calculate_*` helper reaches every formula that calls it;
  the [[Changing a formula]] guide walks through it.
- **Which formula an ability, weapon or item uses, and its X, Y and Z:** the
  game data in MAIN (disc data).
- **Which formula a command forces (Throw, Jump, Charge's weapon):**
  [[battle_action_run_pre_formula_setup]].
- **Strikes per action (formulas 0x1e, 0x1f, 0x5e):**
  [[battle_action_init_current_ability_strike_data]].
- **Evasion:** [[battle_formula_calculate_physical_evade]],
  [[battle_formula_calculate_magical_evade]] and their steps; direction,
  [[battle_formula_calculate_facing_evade]].
- **Critical hits:** [[battle_formula_calculate_critical_hit]].
- **Supports and statuses on XA:**
  [[battle_formula_apply_physical_attack_supports]],
  [[battle_formula_apply_physical_status_xa_modifiers]],
  [[battle_formula_apply_magical_xa_modifiers]].
- **Elements, weather, Faith, zodiac:**
  [[battle_formula_apply_element_affinities]],
  [[battle_formula_apply_weather_elemental_effects]],
  [[battle_formula_calculate_faith]],
  [[battle_formula_apply_zodiac_compatibility]].
- **The 19% added effect:**
  [[battle_formula_roll_conditional_status_proc_inner]].
- **How a status set lands:** [[battle_formula_apply_status_to_action]].
- **Steal, Break and Might Sword targets:**
  [[battle_formula_select_target_equipment]].
- **Today's limits:** the original build keeps every byte; a change goes behind
  an option into the Developer's Cut, where code can grow and move and
  everything that names it follows (`CODEBASE.md`, "What can change today").

## Quirks and debts

- Oil never raises fire damage: [[battle_formula_apply_ability_element]]
  doubles XA after the damage is stored (`QUIRKS.md`).
- The bow weather penalty reads the raw weather variable, so a snowstorm
  counts and the map's ignore-weather flag does not
  ([[battle_formula_apply_weather_effects_on_bows]], `QUIRKS.md`).
- Holy Sword and Might Sword apply the weapon's element only; the element in
  the ability's data is never read (`QUIRKS.md`).
- [[battle_formula_calculate_critical_hit]] and
  [[battle_formula_apply_ability_element]] call argument-less helpers through
  casts that pass an ignored argument (`QUIRKS.md`).
- Formulas 0x05, 0x11, 0x13, 0x18, 0x19, 0x3e, 0x46 and 0x60 have no retail
  user, and [[battle_formula_49_unused_with_extra_steps]] is referenced
  nowhere on the disc.

## Functions in scope

![[Formulas scope]]
