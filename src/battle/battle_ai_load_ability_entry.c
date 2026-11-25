#include "fft/battle_ai.h"
#include "fft/battle_unit.h"
#include "fft/memory.h"

/* Load an AI ability-list entry into the considered ability (g_battle_ai_data_base.considered_ability):
 * skillset, id and owner (entry bits 10-15), CT, range, area, element, MP cost, statuses and AI flags.
 * Attack and Charge take the owner's weapon summary (Charge adds its CT); Jump does too, with its range,
 * enemies as targets and a CT estimate from the unit's CT and Speed; an item reaches 1 tile, or the
 * unit's Move with Throw Item; Throw reaches the unit's Move with the chosen shuriken, ball or weapon;
 * Draw Out names its katana; Math Skill gets every effect flag and map-tile targeting. A default ability
 * takes its 14-byte data, a weapon-range ability the weapon's range, and Half of MP halves its MP cost
 * (rounding down). Unless a default ability perseveres, the CT is halved with the Monster Skill bit and
 * zeroed with the Defend bit, the neighbors of Short Charge and Non-Charge (QUIRKS.md). */
void battle_ai_load_ability_entry(const battle_ai_ability_entry_t* entry) {
    battle_ai_data_t* ai = &g_battle_ai_data_base;
    s32 ability_id, unit_id, menu, status_id;
    battle_stats_t* unit;
    battle_ai_weapon_data_t* weapon;
    ability_secondary_data_t* secondary;
    s32 value;

    battle_mem_zero_32bit_values(&ai->considered_ability, sizeof(battle_ai_considered_ability_t));
    battle_mem_zero_32bit_values(&ai->current_action.command.calculator_type, 4);
    ai->considered_ability.skillset = entry->skillset;
    ability_id = g_battle_ai_data_base.considered_ability.ability_id
        = entry->packed_id & BATTLE_AI_ABILITY_ENTRY_ABILITY_MASK;
    unit_id = entry->packed_id >> BATTLE_AI_ABILITY_ENTRY_UNIT_SHIFT;
    unit = &g_battle_unit_stats[unit_id];
    menu = g_main_action_menu_types_by_skillset[entry->skillset];
    status_id = 0;
    switch (menu) {
    case ACTION_MENU_TYPE_ATTACK:
    attack:
        weapon = &ai->unit_weapon_data[ai->unit_compact_indices[unit_id]];
        ai->considered_ability.parameters.fields.range = weapon->fields.range;
        ai->considered_ability.element = weapon->fields.element;
    weapon_flags:
        ai->considered_ability.parameters.fields.item_id = weapon->fields.weapon_id;
        ai->considered_ability.ai_flags.fields.flags_1 = weapon->fields.flags_1;
        ai->considered_ability.ai_flags.fields.flags_2 = weapon->fields.flags_2;
        ai->considered_ability.ai_flags.fields.flags_3 = weapon->fields.flags_3;
        ai->considered_ability.ai_flags.fields.flags_4 = entry->usage.flags;
        if (menu == ACTION_MENU_TYPE_JUMP)
            ai->considered_ability.ai_flags.word
                = (ai->considered_ability.ai_flags.word | BATTLE_AI_ABILITY_TARGET_ENEMIES)
                & ~BATTLE_AI_ABILITY_TARGET_ALLIES;
        return;
    case ACTION_MENU_TYPE_ITEM_INVENTORY:
        status_id = g_main_item_secondary_data[ability_id - ABILITY_ID_ITEM_FIRST].status_infliction_id;
#ifdef FIX_BUGS
        /* The edition gives a thrown item the targeting's range of 4 (QUIRKS.md). */
        if (unit->ability_flags.support[2] & BATTLE_SUPPORT_SET_3_THROW_ITEM)
            ai->considered_ability.parameters.fields.range = 4;
        else
            ai->considered_ability.parameters.fields.range = 1;
#else
        if (unit->ability_flags.support[2] & BATTLE_SUPPORT_SET_3_THROW_ITEM)
            ai->considered_ability.parameters.fields.range = unit->move;
        else
            ai->considered_ability.parameters.fields.range = 1;
#endif
        ai->considered_ability.parameters.fields.aoe = 0;
        ai->considered_ability.parameters.fields.item_id
            = g_main_item_ability_item_ids[ability_id - ABILITY_ID_ITEM_FIRST];
        break;
    case ACTION_MENU_TYPE_KATANA_INVENTORY:
        ai->considered_ability.parameters.fields.item_id
            = ability_id - (ABILITY_ID_DRAW_OUT_ASURA - ITEM_ID_ASURA_KNIFE);
        break;
    case ACTION_MENU_TYPE_WEAPON_INVENTORY:
        if (ability_id >= ITEM_ID_SHURIKEN && ability_id <= ITEM_ID_YAGYU_DARKNESS) {
            ai->considered_ability.ability_id = ABILITY_ID_THROW_SHURIKEN;
            ai->considered_ability.parameters.fields.item_id = ability_id;
            ai->considered_ability.element = g_main_item_weapon_data[ability_id].element;
        } else if (ability_id >= ITEM_ID_FIRE_BALL && ability_id <= ITEM_ID_LIGHTNING_BALL) {
            ai->considered_ability.ability_id = ABILITY_ID_THROW_BALL;
            ai->considered_ability.parameters.fields.item_id = ability_id;
            ai->considered_ability.element = g_main_item_weapon_data[ability_id].element;
        } else {
            ai->considered_ability.ability_id = ai->throw_ability_id;
            ai->considered_ability.parameters.fields.item_id = ai->throw_weapon_id;
        }
        ai->considered_ability.parameters.fields.range = unit->move;
        ability_id = ai->considered_ability.ability_id;
        break;
    case ACTION_MENU_TYPE_JUMP:
        if (unit_id == ai->acting_unit_id)
            ai->considered_ability.ct
                = ((UNIT_TURN_CT_THRESHOLD - ai->acting_unit->ct) / ai->acting_unit->attributes[UNIT_ATTRIBUTE_SPEED])
                / 2;
        else {
            if (unit->ct > UNIT_TURN_CT_THRESHOLD) {
                ai->considered_ability.ct = 0;
                value = unit->ct - UNIT_TURN_CT_THRESHOLD;
            } else {
                ai->considered_ability.ct
                    = (UNIT_TURN_CT_THRESHOLD - unit->ct) / unit->attributes[UNIT_ATTRIBUTE_SPEED];
                value = unit->attributes[UNIT_ATTRIBUTE_SPEED]
                    - (UNIT_TURN_CT_THRESHOLD - unit->ct) % unit->attributes[UNIT_ATTRIBUTE_SPEED];
            }
            ai->considered_ability.ct
                += ((UNIT_TURN_CT_THRESHOLD - value) / unit->attributes[UNIT_ATTRIBUTE_SPEED]) / 2;
        }
        if (!ability_id)
            ai->considered_ability.parameters.fields.range = 1;
        else
            ai->considered_ability.parameters.fields.range
                = g_main_jump_ability_data[ability_id - ABILITY_ID_JUMP_FIRST].range;
        weapon = &ai->unit_weapon_data[ai->unit_compact_indices[unit_id]];
        goto weapon_flags;
    case ACTION_MENU_TYPE_CHARGE:
        ai->considered_ability.ct = g_main_charge_ability_data[ability_id - ABILITY_ID_CHARGE_FIRST].ct;
        goto attack;
    case ACTION_MENU_TYPE_ARITHMETICKS: {
        u32 status_cursor;
        s32 included = 0xff;
        value = 4;
        /* A direct status pointer folds the member offset into its base;
         * retain the field-derived base cursor for the exact five-byte fill. */
        status_cursor = (u32)ai + value;
        do {
            *(u8*)(status_cursor + ((u32)&ai->considered_ability.parameters.fields.status_infliction - (u32)ai))
                = included;
            value--;
            status_cursor--;
        } while (value >= 0);
        ai->considered_ability.ai_flags.fields.flags_1 = BATTLE_AI_ABILITY_AFFECTS_HP | BATTLE_AI_ABILITY_AFFECTS_MP
            | BATTLE_AI_ABILITY_CANCEL_STATUS | BATTLE_AI_ABILITY_ADD_STATUS | BATTLE_AI_ABILITY_AFFECTS_STATS
            | BATTLE_AI_ABILITY_UNEQUIP;
        ai->considered_ability.ai_flags.fields.flags_4 = entry->usage.flags;
        ai->considered_ability.ai_flags.word |= BATTLE_AI_ABILITY_TARGET_MAP_TILES;
        return;
    }
    }
    battle_mem_copy_u16_values(&ai->considered_ability.ai_flags, &g_main_ability_data[ability_id].ai_flags, 4);
    if (menu == ACTION_MENU_TYPE_ITEM_INVENTORY || menu == ACTION_MENU_TYPE_WEAPON_INVENTORY)
        ai->considered_ability.ai_flags.word |= BATTLE_AI_ABILITY_DIRECT_TRAJECTORY;
    ai->considered_ability.ai_flags.fields.flags_4 = entry->usage.flags;
    if (ability_id < ABILITY_ID_ITEM_FIRST) {
        s32 ct;
        secondary = &g_main_ability_range_data[ability_id];
        ct = secondary->ct;
        status_id = secondary->status_infliction_id;
        ai->considered_ability.ct = ct;
        ai->considered_ability.element = secondary->element;
        ai->considered_ability.mp_cost = secondary->mp_cost;
        if (unit->ability_flags.support[1] & BATTLE_SUPPORT_SET_2_HALF_MP)
            ai->considered_ability.mp_cost >>= 1;
        if (secondary->flags_1 & ABILITY_SECONDARY_FLAG_1_WEAPON_RANGE) {
            ai->considered_ability.parameters.fields.range
                = ai->unit_weapon_data[ai->unit_compact_indices[unit_id]].fields.range;
            ai->considered_ability.parameters.fields.aoe = 0;
        } else {
            ai->considered_ability.parameters.fields.range = secondary->range;
            ai->considered_ability.parameters.fields.aoe = secondary->aoe;
        }
        if (secondary->flags_3 & ABILITY_SECONDARY_FLAG_3_PERSEVERE)
            goto copy_status;
    }
#ifdef FIX_BUGS
    /* The edition estimates the CT the charge takes: Non-Charge first, Short Charge rounded up (QUIRKS.md). */
    if (unit->ability_flags.support[3] & BATTLE_SUPPORT_SET_4_NON_CHARGE)
        ai->considered_ability.ct = 0;
    else if (unit->ability_flags.support[3] & BATTLE_SUPPORT_SET_4_SHORT_CHARGE)
        ai->considered_ability.ct = (ai->considered_ability.ct + 1) >> 1;
#else
    /* Preserve the native support-byte masks, including their CT quirk. */
    if (unit->ability_flags.support[3] & BATTLE_SUPPORT_SET_4_MONSTER_SKILL)
        ai->considered_ability.ct >>= 1;
    else if (unit->ability_flags.support[3] & BATTLE_SUPPORT_SET_4_DEFEND)
        ai->considered_ability.ct = 0;
#endif
copy_status:
    battle_mem_copy_bytes(ai->considered_ability.parameters.fields.status_infliction,
        g_main_status_infliction_data[status_id].statuses,
        sizeof(ai->considered_ability.parameters.fields.status_infliction));
}
