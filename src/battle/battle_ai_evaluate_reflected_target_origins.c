#include "fft/battle_ai.h"
#include "fft/battle_unit.h"
#include "fft/memory.h"

/* Evaluate bouncing the considered ability off a Reflect unit onto another unit. A reflector is
 * targetable, not the acting unit, keeps Reflect through the charge, cannot move before it lands, and
 * if dead does not crystallize first; its area gives the target tiles. For each other unit the
 * ability is useful on that cannot move first, the origin is the tile mirrored through the reflector
 * (2 x reflector - target); where that tile is reachable, the use is evaluated acting before and
 * after moving (battle_ai_evaluate_target_tiles; battle_ai_can_use_ability_at_coords tests the unit's
 * own tile both times). Ends by ranking the best use (battle_ai_insert_ranked_action); returns -1 to
 * suspend, else 0.
 *
 * Stored counters survive suspension, but the resume entry does not reload the reflector pointer;
 * later iterations use the caller's $s2, the AI workspace (QUIRKS.md). */
s32 battle_ai_evaluate_reflected_target_origins(void) {
    battle_ai_data_t* ai = &g_battle_ai_data_base;
    battle_stats_t* reflector;
    battle_ai_unit_decision_t* decision;
    battle_ai_tile_mask_mode_e mode;
    s32 ct;
#ifdef FIX_BUGS
    /* The edition's tile the current pass acts from (QUIRKS.md). */
    battle_ai_coords_t* origin;
#endif

    if (g_battle_ai_data_base.resume_pending) {
        /* Resume: the target jumps into the middle of the origin loop. */
#ifdef FIX_BUGS
        /* The edition reloads the reflector the loop was on (QUIRKS.md). */
        reflector = &g_battle_unit_stats[ai->reflector_unit_index];
#endif
        goto evaluate_targets;
    }
    g_battle_ai_data_base.current_action.reflected_action = 1;
    g_battle_ai_data_base.targetability.live.target_setting_flags &= ~BATTLE_AI_TARGET_SETTING_CONSUME_TARGET_TILES;
    for (ai->reflector_unit_index = 0; ai->reflector_unit_index < BATTLE_UNIT_SLOT_COUNT; ai->reflector_unit_index++) {
        reflector = &g_battle_unit_stats[ai->reflector_unit_index];
        decision = &ai->unit_decisions[ai->reflector_unit_index];
        if (decision->targeting_flags_2 & BATTLE_AI_TARGET_UNTARGETABLE)
            continue;
        if (ai->reflector_unit_index == ai->acting_unit_id)
            continue;
        if (!battle_ai_is_status_active_through_delay(ai->considered_ability.ct, reflector, BATTLE_STATUS_ID_REFLECT))
            continue;
        if (battle_ai_moves_before_charge_lands(ai->reflector_unit_index))
            continue;
        if (decision->targeting_flags_2 & BATTLE_AI_TARGET_DEAD_WITHOUT_RERAISE) {
            ct = ai->considered_ability.ct;
            if (battle_ai_calculate_clockticks_until_death_counter_expires(reflector) < ct)
                continue;
        }
        ai->current_action.command.targeting_type = BATTLE_TARGETING_TYPE_TILE;
        battle_ai_transfer_unit_coordinates(ai->reflector_unit_index, &ai->considered_unit_coords);
        mode = BATTLE_AI_TILE_MASK_AOE;
        if (ai->current_action.command.targeting_type == BATTLE_TARGETING_TYPE_UNIT)
            mode = BATTLE_AI_TILE_MASK_EXPLICIT;
        battle_ai_build_targetable_tile_mask(mode, 0);
        battle_mem_copy_u16_values(ai->target_tiles, ai->targetable_tiles, sizeof(ai->target_tiles));
        for (ai->reflected_target_unit_index = 0; ai->reflected_target_unit_index < BATTLE_UNIT_SLOT_COUNT;
            ai->reflected_target_unit_index++) {
            if (!ai->targetability.live.unit_targetable[ai->reflected_target_unit_index])
                continue;
            if (ai->reflected_target_unit_index == ai->reflector_unit_index)
                continue;
            if (ai->reflected_target_unit_index == ai->acting_unit_id)
                continue;
            if (battle_ai_moves_before_charge_lands(ai->reflected_target_unit_index))
                continue;
            if (battle_ai_classify_ability_effect(ai->reflected_target_unit_index, 0)
                != BATTLE_AI_ABILITY_EFFECT_USEFUL)
                continue;
            ai->current_action.coords.fields.x
                = reflector->x * 2 - g_battle_unit_stats[ai->reflected_target_unit_index].x;
            ai->current_action.coords.fields.y
                = reflector->position.y * 2 - g_battle_unit_stats[ai->reflected_target_unit_index].position.y;
            if (ai->current_action.coords.fields.x >= ai->map_max_x)
                continue;
            if (ai->current_action.coords.fields.y >= ai->map_max_y)
                continue;
            for (ai->reflected_candidate_level = 0; ai->reflected_candidate_level < MAP_TERRAIN_LAYER_COUNT;
                ai->reflected_candidate_level++) {
                ai->current_action.coords.fields.elevation = ai->reflected_candidate_level;
                if ((ai->reachable_tiles[ai->movement_scenario][ai->reflected_candidate_level]
                                        [ai->current_action.coords.fields.y]
                        << ai->current_action.coords.fields.x)
                    & 0x8000) {
                    battle_ai_update_tile_pickup();
                    ai->current_action.act_before_move = 1;
                    ai->reflected_origin_phase = 0;
#ifdef FIX_BUGS
                    /* The edition tests the tile each pass acts from: the unit's, then the destination (QUIRKS.md). */
                    origin = &ai->acting_unit_coords;
                    for (;;) {
                        if (battle_ai_can_use_ability_at_coords(origin)) {
                        evaluate_targets:
                            if (battle_ai_evaluate_target_tiles() == -1)
                                return -1;
                        }
                        if (ai->reflected_origin_phase != 0)
                            break;
                        ai->reflected_origin_phase = 1;
                        ai->current_action.act_before_move = 0;
                        origin = &ai->current_action.coords;
                    }
#else
                    for (;;) {
                        if (battle_ai_can_use_ability_at_coords(&ai->acting_unit_coords)) {
                        evaluate_targets:
                            if (battle_ai_evaluate_target_tiles() == -1)
                                return -1;
                        }
                        if (ai->reflected_origin_phase != 0)
                            break;
                        ai->reflected_origin_phase = 1;
                        ai->current_action.act_before_move = 0;
                    }
#endif
                }
            }
        }
    }
    battle_ai_move_temp_unit_to_coords(&ai->acting_unit_coords);
    ai->current_action.reflected_action = 0;
    battle_ai_insert_ranked_action();
    return 0;
}
