#include "fft/battle_ai.h"
#include "fft/battle_map.h"
#include "fft/battle_unit.h"

/* Mark in targetable_tiles the tiles next to each same-team Monster Skill holder that can react,
 * within 2 levels of its tile's height (slope and depth do not count), for abilities that need Monster
 * Skill. Only the lower level is marked, and the -x and -y neighbors are lost: the target loads their
 * 0xff offsets as 255 (QUIRKS.md). */
void battle_ai_build_monster_skill_tile_mask(void) {
    battle_ai_data_t* ai = &g_battle_ai_data_base;
    battle_stats_t* unit;
    s32 i;
    s32 direction;
    s32 x;
    s32 y;
    s32 level;
    s32 height;
    u16* row;
    u32 level_base;

    for (level = 0; level < BATTLE_AI_GRID_ROW_COUNT; level++) {
        ai->targetable_tiles[0][level] = 0;
        ai->targetable_tiles[1][level] = 0;
    }
    for (i = 0; i < BATTLE_UNIT_SLOT_COUNT; i++) {
        unit = &g_battle_unit_stats[i];
        if (unit->entd_slot == BATTLE_ENTD_SLOT_NONE)
            continue;
        if (!(unit->ability_flags.support[3] & BATTLE_SUPPORT_SET_4_MONSTER_SKILL))
            continue;
        if ((unit->initial_team_flags & BATTLE_TEAM_MASK) != ai->acting_unit_team)
            continue;
        if (main_unit_has_status_in_set(unit, MAIN_STATUS_CHECK_SET_PREVENT_REACTION))
            continue;
        height = g_battle_map_tile_data[unit->position.higher_elevation * MAP_TILES_PER_LAYER
            + unit->position.y * ai->map_max_x + unit->x]
                     .height;
#ifdef FIX_BUGS
        /* The edition reads the -1 offsets signed, marks both levels and counts slopes (QUIRKS.md). */
        for (direction = 0; direction < BATTLE_AI_CARDINAL_DIRECTION_COUNT; direction++) {
            x = unit->x + (s8)g_battle_ai_facing_tile_offsets.by_direction[direction][0];
            if (x < 0 || x >= ai->map_max_x)
                continue;
            y = unit->position.y + (s8)g_battle_ai_facing_tile_offsets.by_direction[direction][1];
            if (y < 0 || y >= ai->map_max_y)
                continue;
            ai->considered_unit_coords.fields.x = x;
            ai->considered_unit_coords.fields.y = y;
            for (level = MAP_TERRAIN_LAYER_COUNT - 1; level >= 0; level--) {
                ai->considered_unit_coords.fields.elevation = level;
                height = battle_ai_get_unit_height_above_coords(&ai->considered_unit_coords, unit);
                if (height >= -2 && height <= 2) {
                    /* The original's row write (below). */
                    level_base = level * sizeof(ai->targetable_tiles[0]) + (u32)ai;
                    row = (u16*)(level_base + ((u32)&ai->targetable_tiles - (u32)ai));
                    row[y] |= 0x8000 >> x;
                }
            }
        }
#else
        for (direction = 0; direction < BATTLE_AI_CARDINAL_DIRECTION_COUNT; direction++) {
            x = unit->x + g_battle_ai_facing_tile_offsets.by_direction[direction][0];
            if (x < 0 || x >= ai->map_max_x)
                continue;
            y = unit->position.y + g_battle_ai_facing_tile_offsets.by_direction[direction][1];
            if (y < 0 || y >= ai->map_max_y)
                continue;
            for (level = 0; level <= 0; level++) {
                if ((g_battle_map_tile_data[(level * MAP_TILES_PER_LAYER) + y * ai->map_max_x + x].height - height)
                        >= -2
                    && (g_battle_map_tile_data[(level * MAP_TILES_PER_LAYER) + y * ai->map_max_x + x].height - height)
                        <= 2) {
                    /* A typed level-row pointer folds the field offset into
                     * the induction value. Preserve the target's AI-base
                     * cursor with a member-derived displacement. */
                    level_base = level * sizeof(ai->targetable_tiles[0]) + (u32)ai;
                    row = (u16*)(level_base + ((u32)&ai->targetable_tiles - (u32)ai));
                    row[y] |= 0x8000 >> x;
                }
            }
        }
#endif
    }
}
