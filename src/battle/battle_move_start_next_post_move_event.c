#include "fft/battle.h"
#include "fft/battle_map.h"
#include "fft/battle_menu.h"
#include "fft/battle_move.h"

/* Start the next pending post-move event (g_battle_move_post_move_events) and enter
 * BATTLE_GAME_STATE_CRYSTAL_LEARN, whose handler clears the event's bit once it has played and calls
 * this again. Order: a crystal or chest pickup, the movement-ability rewards and level-up reports
 * (two display phases), a found item, a trap, a canceled charge, then a display refresh of the
 * mover (SOURCE_DISPLAY, which nothing sets) or of its mount (MOUNT_STATUS_CHANGE). Returns 1 when
 * an event started; with none left it refreshes both units, enters the after-command state unless
 * the Deep Dungeon refresh takes over, and returns 0. */
s32 battle_move_start_next_post_move_event(void) {
    battle_unit_t* source;
    battle_unit_t* casting;
    battle_unit_t* target;
    battle_stats_t* stats;
    s32 item;
    s32 animation;
    s32 trap;

    source = battle_unit_get_source();
    casting = battle_unit_get_casting();
    battle_menu_init_system_function(BATTLE_MENU_SYSTEM_COMMAND_CRYSTAL_OR_TREASURE, 0, source->stats->battle_id, 0, 0);
    g_battle_game_state = BATTLE_GAME_STATE_CRYSTAL_LEARN;
    g_battle_action_post_action = 0;
    casting->state_frame_counter = 0;
    if (g_battle_move_post_move_events & BATTLE_MOVE_POST_EVENT_CRYSTAL_OR_TREASURE) {
        target = battle_unit_get_crystal_or_treasure_at_map_coords(source->map_x, source->map_y, source->map_z);
        item = battle_menu_get_crystal_pickup_selection(source->stats->battle_id);
        if (item == -1) {
            animation = BATTLE_UNIT_ANIMATION_RAISE_HANDS;
        } else {
            battle_gfx_store_item_display_data(source, item);
            animation = BATTLE_UNIT_ANIMATION_HOLD_ITEM;
        }
        battle_unit_store_animation_facing_movement_data(animation, source->facing, source);
        if (item == -1) {
            stats = source->stats;
            if (!(stats->initial_team_flags & BATTLE_TEAM_MASK)
                && (stats->initial_team_flags & BATTLE_TEAM_FLAG_PLAYER_CONTROLLED)) {
                battle_menu_init_system_function(
                    BATTLE_MENU_SYSTEM_COMMAND_CRYSTAL_OR_TREASURE, stats->battle_id, stats->battle_id, 0, 1);
            } else {
                battle_menu_init_system_function(
                    BATTLE_MENU_SYSTEM_COMMAND_CRYSTAL_OR_TREASURE, stats->battle_id, stats->battle_id, 0, 0);
                battle_unit_learn_from_crystal(source->stats, BATTLE_CRYSTAL_RESTORE_HP_MP);
                battle_action_apply_target_result(source->stats->battle_id);
                battle_gfx_prepare_post_action_display(source);
                battle_unit_update_display_by_runtime_id(source->runtime_id);
            }
        } else {
            if (!(source->stats->initial_team_flags & BATTLE_TEAM_MASK)) {
                battle_menu_init_system_function(BATTLE_MENU_SYSTEM_COMMAND_CRYSTAL_OR_TREASURE,
                    source->stats->battle_id, source->stats->battle_id, 0, 1);
            } else {
                battle_menu_init_system_function(BATTLE_MENU_SYSTEM_COMMAND_CRYSTAL_OR_TREASURE,
                    source->stats->battle_id, source->stats->battle_id, 0, 0);
            }
            if ((source->stats->initial_team_flags & BATTLE_TEAM_MASK)
                || !(source->stats->initial_team_flags & BATTLE_TEAM_FLAG_PLAYER_CONTROLLED)) {
                battle_unit_learn_from_crystal(source->stats, BATTLE_CRYSTAL_TAKE_TREASURE);
            }
        }
        battle_gfx_start_unit_red_fade_out(target->runtime_id);
        target->depth_height_offset
            = 0x24 - (battle_gfx_calculate_world_height_from_world_coords(source) - target->world_coords.vy);
        main_sound_play_sfx_find_channel(MAIN_SFX_COMPLETE);
        return 1;
    }
    if (g_battle_move_post_move_events & BATTLE_MOVE_POST_EVENT_MOVEMENT_BENEFIT) {
        switch (g_battle_action_post_action_display_phase) {
        case BATTLE_POST_ACTION_DISPLAY_LEVEL_UP:
            battle_action_init_movement_ability_benefit(casting->stats);
            battle_unit_build_gained_exp_jp_level_job_level(casting->stats, &casting->strike.action_rewards);
            battle_gfx_prepare_post_action_display(casting);
            battle_gfx_init_earned_exp_jp_display(casting);
            battle_action_report_level_up(casting);
            return 1;
        case BATTLE_POST_ACTION_DISPLAY_JOB_LEVEL_UP:
            battle_action_report_job_level_up(casting);
            return 1;
        }
        return 1;
    }
    if (g_battle_move_post_move_events & BATTLE_MOVE_POST_EVENT_ITEM_FOUND) {
        g_battle_move_find_display_item = *battle_map_determine_rare_common_item(casting->stats);
        battle_gfx_store_item_display_data(source, g_battle_move_find_display_item.item_id);
        battle_unit_store_animation_facing_movement_data(BATTLE_UNIT_ANIMATION_FOUND_ITEM, source->facing, source);
        if (!(source->team_flags & BATTLE_TEAM_MASK)) {
            battle_menu_init_system_function(BATTLE_MENU_SYSTEM_COMMAND_MOVE_FIND_ITEM,
                g_battle_move_find_display_item.item_id, source->stats->battle_id, 0, 1);
            main_sound_play_tune(MAIN_SOUND_TUNE_ITEM_FOUND);
            return 1;
        }
        battle_menu_init_system_function(BATTLE_MENU_SYSTEM_COMMAND_MOVE_FIND_ITEM,
            g_battle_move_find_display_item.item_id, source->stats->battle_id, 0, 0);
        return 1;
    }
    if (g_battle_move_post_move_events & BATTLE_MOVE_POST_EVENT_TRAP_TRIGGERED) {
        trap = battle_action_apply_tile_trap(casting->stats);
        if ((source->pending_attack_result = battle_action_apply_target_result(source->stats->battle_id)) == -1) {
            battle_unit_find_relocation_tile(source->stats->battle_id, &source->dismount);
            battle_unit_set_map_coords_after_death_dismount(source);
        }
        battle_gfx_prepare_post_action_display(source);
        battle_unit_set_target_animation_from_attack_type(0, source);
        battle_effect_set_secondary_trap(source);
        battle_menu_init_system_function(BATTLE_MENU_SYSTEM_COMMAND_TRAP_RESULT, trap, source->stats->battle_id, 0, 1);
#ifdef FIX_BUGS
        /* The edition passes the tile as the AI records it: x, row, layer (QUIRKS.md). */
        battle_ai_record_sprung_trap(casting->map_x, casting->map_y, casting->map_z);
#else
        battle_ai_record_sprung_trap(casting->map_x, casting->map_z, casting->map_y);
#endif
        return 1;
    }
    if (g_battle_move_post_move_events & BATTLE_MOVE_POST_EVENT_CHARGING_CANCEL) {
        battle_status_check_charging_charge(casting->stats, 1);
        battle_menu_init_system_function(
            BATTLE_MENU_SYSTEM_COMMAND_CRYSTAL_OR_TREASURE, 0, source->stats->battle_id, 0, 0);
        return 1;
    }
    if (g_battle_move_post_move_events & BATTLE_MOVE_POST_EVENT_SOURCE_DISPLAY) {
        battle_menu_init_system_function(
            BATTLE_MENU_SYSTEM_COMMAND_CRYSTAL_OR_TREASURE, 0, source->stats->battle_id, 0, 0);
        battle_gfx_prepare_post_action_display_by_runtime_id(source->runtime_id);
        battle_unit_update_display_by_runtime_id(source->runtime_id);
        return 1;
    }
    if (g_battle_move_post_move_events & BATTLE_MOVE_POST_EVENT_MOUNT_STATUS_CHANGE) {
        battle_menu_init_system_function(
            BATTLE_MENU_SYSTEM_COMMAND_CRYSTAL_OR_TREASURE, 0, source->stats->battle_id, 0, 0);
        if (source->mount_state != BATTLE_UNIT_MOUNT_STATE_NONE) {
            target = battle_unit_get_by_runtime_id(source->mount_partner_runtime_id);
            if (target != 0) {
                battle_gfx_prepare_post_action_display_by_runtime_id(target->runtime_id);
                battle_unit_update_display_by_runtime_id(target->runtime_id);
            }
        }
        return 1;
    }
    battle_unit_update_display_by_runtime_id(source->runtime_id);
    battle_unit_update_display_by_runtime_id(casting->runtime_id);
    if (battle_map_refresh_deep_dungeon() == 0) {
        battle_state_enter_after_command();
    }
    return 0;
}
