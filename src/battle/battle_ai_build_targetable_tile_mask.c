#include "fft/battle_ai.h"
#include "fft/memory.h"

/* Build targetable_tiles: the tiles within a Manhattan radius of considered_unit_coords, the same on
 * both levels (height is not counted). The radius is the considered ability's range, area or both, or
 * the given one for BATTLE_AI_TILE_MASK_EXPLICIT; a linear ability keeps only the center's row and
 * column, and a radius of 16 or more marks every tile without that filter.
 *
 * Radii 8-15 read past the eight row templates, into g_battle_ai_status_priority_weights, which
 * follows them (QUIRKS.md). */
void battle_ai_build_targetable_tile_mask(battle_ai_tile_mask_mode_e mode, s32 radius) {
    battle_ai_data_t* ai = &g_battle_ai_data_base;
    s32 i;
    s32 x;
    s32 y;

    /* Preserve the target's signed mode dispatch. */
    switch ((s32)mode) {
    case BATTLE_AI_TILE_MASK_RANGE:
        radius = g_battle_ai_data_base.considered_ability.parameters.fields.range;
        break;
    case BATTLE_AI_TILE_MASK_AOE:
        radius = g_battle_ai_data_base.considered_ability.parameters.fields.aoe;
        break;
    case BATTLE_AI_TILE_MASK_RANGE_PLUS_AOE:
        radius = g_battle_ai_data_base.considered_ability.parameters.fields.range
            + g_battle_ai_data_base.considered_ability.parameters.fields.aoe;
        break;
    }
    if (radius >= 16) {
        for (i = 0; i < BATTLE_AI_GRID_ROW_COUNT; i++) {
            ai->targetable_tiles[0][i] = 0xffff;
            ai->targetable_tiles[1][i] = 0xffff;
        }
        return;
    }
    for (i = BATTLE_AI_GRID_ROW_COUNT - 1; i >= 0; i--)
        ai->targetable_tiles[0][i] = 0;
    y = ai->considered_unit_coords.fields.y;
    x = ai->considered_unit_coords.fields.x;
#ifdef FIX_BUGS
    /* The edition computes each row from its half-width k, so radii 8-15 read no table (QUIRKS.md). */
    /* Columns x - k .. x + k (column c is bit 15 - c): 2k + 1 bits from bit 16 - k, shifted down by x + 1. */
    for (i = 0; i <= radius; i++) {
        if (y - i < 0)
            break;
        ai->targetable_tiles[0][y - i] = (((2U << ((radius - i) * 2)) - 1) << (16 - (radius - i))) >> (x + 1);
    }
    for (i = 1; i <= radius; i++) {
        if (y + i >= ai->map_max_y)
            break;
        ai->targetable_tiles[0][y + i] = (((2U << ((radius - i) * 2)) - 1) << (16 - (radius - i))) >> (x + 1);
    }
#else
    for (i = 0; i <= radius; i++) {
        if (y - i < 0)
            break;
        if (x < 8)
            ai->targetable_tiles[0][y - i] = g_battle_ai_range_row_masks[radius - i] << (7 - x);
        else
            ai->targetable_tiles[0][y - i] = g_battle_ai_range_row_masks[radius - i] >> (x - 7);
    }
    for (i = 1; i <= radius; i++) {
        if (y + i >= ai->map_max_y)
            break;
        if (x < 8)
            ai->targetable_tiles[0][y + i] = g_battle_ai_range_row_masks[radius - i] << (7 - x);
        else
            ai->targetable_tiles[0][y + i] = g_battle_ai_range_row_masks[radius - i] >> (x - 7);
    }
#endif
    if (ai->considered_ability.ai_flags.word & BATTLE_AI_ABILITY_LINEAR_TRAJECTORY) {
        for (y = 0; y < ai->map_max_y; y++) {
            i = ai->targetable_tiles[0][y];
            if (i != 0) {
                for (x = 0; x < ai->map_max_x; x++) {
                    if ((i << x) & 0x8000) {
                        if (x != ai->considered_unit_coords.fields.x && y != ai->considered_unit_coords.fields.y)
                            ai->targetable_tiles[0][y] ^= 0x8000 >> x;
                    }
                }
            }
        }
    }
    battle_mem_copy_u16_values(ai->targetable_tiles[1], ai->targetable_tiles[0], sizeof(ai->targetable_tiles[0]));
}
