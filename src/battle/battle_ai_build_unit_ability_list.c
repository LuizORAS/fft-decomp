#include "fft/battle_ai.h"
#include "fft/battle_unit.h"

/* Byte-oriented view of the same four-byte ability entry. */
typedef struct battle_ai_ability_entry_byte_view {
    u8 ability_id_low;
    u8 unit_and_id_high;
    u8 skillset;
    u8 usage_flags;
} battle_ai_ability_entry_byte_view_t;
typedef char battle_ai_ability_entry_byte_view_must_be_4[sizeof(battle_ai_ability_entry_byte_view_t) == 4 ? 1 : -1];

/* Build a unit's AI ability list. A human gets an Attack entry first, with the weapon summary of both
 * hands (battle_ai_store_weapon_attack_data): range (1 bare-handed), element, an arc or direct trajectory,
 * and a target team from the weapon kind: enemies, enemies with undead reversal for an absorbing weapon,
 * allies with undead reversal for a healing one (two different kinds give no team, QUIRKS.md). Then come
 * the usable abilities of its primary and secondary skillsets (battle_ai_add_usable_skillset_abilities);
 * a monster gets its monster skillset only. Frog Attack is appended for every unit, and the 0xff
 * terminator only while room is left (34 entries). */
void battle_ai_build_unit_ability_list(s32 unit_id) {
    battle_stats_t* unit = &g_battle_unit_stats[unit_id];
    battle_ai_data_t* ai = &g_battle_ai_data_base;
    battle_ai_weapon_data_t* weapon;
    s32 list_id = ai->unit_compact_indices[unit_id];
    s32 count = 0;
    s32 weapon_kinds;

    if (unit->primary_skillset < SKILLSET_ID_MONSTER_FIRST) {
        ai->ability_lists[list_id][0].skillset = SKILLSET_ID_ATTACK;
        ai->ability_lists[list_id][0].packed_id = unit_id << BATTLE_AI_ABILITY_ENTRY_UNIT_SHIFT;
        weapon = &ai->unit_weapon_data[list_id];
        weapon->words[0] = 0;
        weapon->words[1] = 0;
        weapon->fields.flags_1 |= BATTLE_AI_ABILITY_FLAG_1_AFFECTS_HP;
        weapon->fields.flags_2 |= BATTLE_AI_ABILITY_FLAG_2_EVADEABLE;
        weapon->fields.flags_3 |= BATTLE_AI_ABILITY_FLAG_3_PHYSICAL_ATTACK;
        weapon_kinds = BATTLE_AI_WEAPON_FORMULA_NONE;
        if (!(unit->unit_flags & UNIT_FLAG_MONSTER)) {
            battle_ai_store_weapon_attack_data(
                weapon, unit->equipment[UNIT_EQUIPMENT_SLOT_RIGHT_HAND_WEAPON], &weapon_kinds);
            battle_ai_store_weapon_attack_data(
                weapon, unit->equipment[UNIT_EQUIPMENT_SLOT_LEFT_HAND_WEAPON], &weapon_kinds);
        }
        /* Two different kinds (QUIRKS.md) match no case and leave both team flags clear. */
        switch (weapon_kinds) {
        case BATTLE_AI_WEAPON_FORMULA_NONE:
            weapon->fields.range = 1;
            /* Fall through to normal enemy targeting. */
        case BATTLE_AI_WEAPON_FORMULA_DAMAGE:
#ifdef FIX_BUGS
        /* The edition aims two weapon kinds at enemies, as a damage weapon (QUIRKS.md). */
        default:
#endif
            weapon->fields.flags_1 |= BATTLE_AI_ABILITY_FLAG_1_TARGET_ENEMIES;
            break;
        case BATTLE_AI_WEAPON_FORMULA_ABSORB_HP:
            weapon->fields.flags_1 |= BATTLE_AI_ABILITY_FLAG_1_TARGET_ENEMIES;
            weapon->fields.flags_2 |= BATTLE_AI_ABILITY_FLAG_2_UNDEAD_REVERSE;
            break;
        case BATTLE_AI_WEAPON_FORMULA_HEAL:
            weapon->fields.flags_1 |= BATTLE_AI_ABILITY_FLAG_1_TARGET_ALLIES;
            weapon->fields.flags_2 |= BATTLE_AI_ABILITY_FLAG_2_UNDEAD_REVERSE;
            break;
        }
        count++;
    }
    count = battle_ai_add_usable_skillset_abilities(unit_id, unit->primary_skillset, count);
    if (unit->primary_skillset < SKILLSET_ID_MONSTER_FIRST) {
        count = battle_ai_add_usable_skillset_abilities(unit_id, unit->secondary_skillset, count);
    }
    ai->ability_lists[list_id][count].skillset = SKILLSET_ID_FROG_ATTACK;
    ai->ability_lists[list_id][count].packed_id
        = (unit_id << BATTLE_AI_ABILITY_ENTRY_UNIT_SHIFT) + ABILITY_ID_FROG_ATTACK;
    /* The byte view preserves the target's separate final-store address. */
    ((battle_ai_ability_entry_byte_view_t*)&ai->ability_lists[list_id][count])->usage_flags
        = BATTLE_AI_ABILITY_ENTRY_USABLE | BATTLE_AI_ABILITY_ENTRY_EXHAUSTIVE_ORIGIN_SCAN;
    count++;
    if (count < BATTLE_AI_ABILITY_LIST_LENGTH) {
        ai->ability_lists[list_id][count].skillset = SKILLSET_ID_NONE;
    }
}
