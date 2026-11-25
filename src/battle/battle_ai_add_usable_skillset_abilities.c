#include "fft/battle_ai.h"
#include "fft/battle_unit.h"

/* Append a skillset's AI-usable abilities to the unit's AI list and return the new count. Throw adds the
 * three shuriken and the three balls when their abilities are learned, and one generic entry for the first
 * other learned Throw ability (battle_ai_set_ability_considerations picks its weapon); Math Skill adds one
 * entry; Charge adds nothing for a unit holding a gun. Other skillsets add each learned action ability
 * flagged usable by the AI; a default ability also needs its MP cost (Half of MP halves it, rounding
 * down) within the unit's max MP and the sword or Materia Blade it requires, and an innately Innocent
 * unit drops it when the ability considered last, not this one, depends on Faith (QUIRKS.md). Jump keeps
 * only the highest learned Level Jump; with none its fallback entry is lost (QUIRKS.md). In a monster
 * skillset the fourth ability is marked as needing a Monster Skill supporter.
 *
 * The known-throw tail sits before the main body because the target emits it there and GCC 2.6.3 keeps
 * source block order. The per-loop temporaries, ai as a local and the int max_mp reproduce the target's
 * allocation, loop set-up order and signed comparison. */
s32 battle_ai_add_usable_skillset_abilities(s32 unit_id, s32 skillset, s32 count) {
    s32 jump_found;
    battle_ai_data_t* ai;
    u16* abilities;
    s32 menu;
    s32 list_id;
    s32 i;
    battle_ai_ability_entry_t* entry;
    battle_ai_ability_entry_t* slot;
    battle_ai_ability_entry_t* row;
    battle_stats_t* unit;
    s32 ability_id;
    s32 unit_bits;
    u8 mp_cost;
    s32 max_mp;
    u8 flags;

    ai = &g_battle_ai_data_base;
    if (skillset == 0) {
        return count;
    }
    jump_found = 0;
    abilities = main_ability_store_skillset_abilities(skillset, SKILLSET_ABILITY_FILTER_ACTION);
    menu = g_main_action_menu_types_by_skillset[skillset];
    list_id = ai->unit_compact_indices[unit_id];
    if (menu == ACTION_MENU_TYPE_WEAPON_INVENTORY) {
        if (battle_ai_is_ability_known(unit_id, skillset, 0)) {
            unit_bits = unit_id << BATTLE_AI_ABILITY_ENTRY_UNIT_SHIFT;
            for (i = 0, slot = ai->ability_lists[list_id]; i < 3; i++) {
                slot[count].skillset = skillset;
                slot[count].packed_id = unit_bits + i + ITEM_ID_THROWABLE_FIRST;
                count++;
            }
        }
        if (battle_ai_is_ability_known(unit_id, skillset, 1)) {
            i = 0;
            unit_bits = unit_id << BATTLE_AI_ABILITY_ENTRY_UNIT_SHIFT;
            for (row = ai->ability_lists[list_id]; i < 3; i++) {
                row[count].skillset = skillset;
                row[count].packed_id = unit_bits + i + ITEM_ID_FIRE_BALL;
                count++;
            }
        }
        for (i = 2; i < 12; i++) {
            if (battle_ai_is_ability_known(unit_id, skillset, i)) {
                entry = &ai->ability_lists[list_id][count];
                count++;
                entry->skillset = skillset;
                entry->packed_id = unit_id << BATTLE_AI_ABILITY_ENTRY_UNIT_SHIFT;
                break;
            }
        }
        return count;
    }
    if (menu == ACTION_MENU_TYPE_ARITHMETICKS) {
        entry = &ai->ability_lists[list_id][count];
        entry->skillset = skillset;
        entry->packed_id = unit_id << BATTLE_AI_ABILITY_ENTRY_UNIT_SHIFT;
        return count + 1;
    }
    if (menu == ACTION_MENU_TYPE_CHARGE) {
        i = ai->unit_weapon_data[list_id].fields.weapon_id;
        if (i >= ITEM_ID_ROMANDA_GUN && i <= ITEM_ID_BLAST_GUN) {
            return count;
        }
    }
    unit = &g_battle_unit_stats[unit_id];
    for (i = 0; i < RSM_SKILLSET_SLOT_FIRST; i++) {
        ability_id = abilities[i];
        if (ability_id == 0) {
            continue;
        }
        entry = &ai->ability_lists[list_id][count];
        if (!(g_main_ability_data[ability_id].ai_flags.flags_4 & BATTLE_AI_ABILITY_FLAG_4_USABLE_BY_AI)) {
            continue;
        }
        if (ability_id < ABILITY_ID_ITEM_FIRST) {
            mp_cost = g_main_ability_range_data[ability_id].mp_cost;
            if (unit->ability_flags.support[1] & BATTLE_SUPPORT_SET_2_HALF_MP) {
                mp_cost >>= 1;
            }
            max_mp = unit->max_mp;
            if (max_mp < mp_cost) {
                continue;
            }
#ifdef FIX_BUGS
            /* The edition tests the Faith flag of the ability it adds (QUIRKS.md). */
            if ((g_main_ability_data[ability_id].ai_flags.flags_2 & BATTLE_AI_ABILITY_FLAG_2_AFFECTED_BY_FAITH)
                && (unit->status_sets.innate[BATTLE_STATUS_BYTE_INDEX(BATTLE_STATUS_ID_INNOCENT)]
                    & BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_INNOCENT))) {
                continue;
            }
#else
            /* Retail bug (QUIRKS.md): the flags are the ability considered last, not ability_id's. */
            if ((ai->considered_ability.ai_flags.word & BATTLE_AI_ABILITY_AFFECTED_BY_FAITH)
                && (unit->status_sets.innate[BATTLE_STATUS_BYTE_INDEX(BATTLE_STATUS_ID_INNOCENT)]
                    & BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_INNOCENT))) {
                continue;
            }
#endif
            flags = g_main_ability_range_data[ability_id].flags_4;
            if (flags & ABILITY_SECONDARY_FLAG_4_REQUIRES_SWORD) {
                if (!(unit->equipped_flags & BATTLE_UNIT_EQUIPPED_FLAG_SWORD)) {
                    continue;
                }
            } else if (flags & ABILITY_SECONDARY_FLAG_4_REQUIRES_MATERIA_BLADE) {
                if (!(unit->equipped_flags & BATTLE_UNIT_EQUIPPED_FLAG_MATERIA_BLADE)) {
                    continue;
                }
            }
        }
        if (!battle_ai_is_ability_known(unit_id, skillset, i)) {
            continue;
        }
        if (skillset >= SKILLSET_ID_MONSTER_FIRST) {
            if (i == 3) {
                entry->usage.policy.requires_monster_skill = 1;
            } else {
                entry->usage.policy.requires_monster_skill = 0;
            }
        }
        entry->skillset = skillset;
        entry->packed_id = ability_id;
        if (menu == ACTION_MENU_TYPE_JUMP) {
            if (ability_id >= ABILITY_ID_JUMP_VERTICAL_2) {
                continue;
            }
            if (jump_found == 1) {
                count--;
                entry = &ai->ability_lists[list_id][count];
                entry->packed_id = ability_id;
            }
            jump_found = 1;
        }
        count++;
        entry->packed_id += unit_id << BATTLE_AI_ABILITY_ENTRY_UNIT_SHIFT;
    }
    /* Written past count, so the next entry overwrites it (QUIRKS.md). */
    if (menu == ACTION_MENU_TYPE_JUMP && jump_found == 0) {
        entry->skillset = SKILLSET_ID_JUMP;
        entry->packed_id = unit_id << BATTLE_AI_ABILITY_ENTRY_UNIT_SHIFT;
    }
    return count;
}
