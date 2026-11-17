#include "fft/battle_action.h"
#include "fft/battle_menu.h"
#include "fft/battle_unit.h"
#include "fft/main.h"
#include "fft/menu.h"
#include "fft/thread.h"

/*
 * Attack-preview caster panel: stamp the selected unit's battle id into its pending action, run the
 * preview calculation on the main stack unless the ability's secondary flag 0x04 is set or the
 * skillset changes equipment, and publish the target's action results through
 * g_battle_menu_preview_target_action (0 when skipped). WORLD compiles this text too
 * (world_menu_preview_attack_caster_stats, publishing through g_world_menu_preview_target_action).
 *
 * For an item id the ability pointers are never set, yet secondary_data[5]
 * is still read before the id is tested.
 */
void menu_preview_attack_caster_stats(void) {
    battle_stats_t* attacker;
    battle_stats_t* target;
    /* Signed view of action_command_t for the ability id load. */
    action_command_t* action;
    ability_data_t* ability_data;
    main_ability_secondary_pointer_t secondary;
    s16* selected;

    attacker = battle_unit_get_active_turn_unit_stats();
    target = battle_unit_get_stats_from_battle_id(g_preview_target_unit_id);
    /* One address register serves both reads, as in the target. */
    selected = &g_menu_selected_unit.battle_id;
    action = battle_unit_get_runtime_action_command_by_battle_id(*selected);
    action->actor_id = *selected;
    if (action->ability_id < ABILITY_ID_ITEM_FIRST) {
        main_ability_calculate_pointers_and_type(action->ability_id & ABILITY_ID_MASK, &ability_data, &secondary);
    }
#ifdef FIX_BUGS
    /* The edition tests the item id before it reads the range flags (QUIRKS.md). */
    if ((action->ability_id < ABILITY_ID_ITEM_FIRST && (secondary.range->flags_3 & ABILITY_SECONDARY_FLAG_3_PERSEVERE))
        || action->skillset_id == SKILLSET_ID_EQUIP_CHANGE) {
        g_battle_menu_preview_target_action = 0;
    } else {
        g_thread_call_target = (void (*)(void))battle_action_calculate_projected_effect;
        thread_call_on_main_stack(attacker, target, action);
        g_battle_menu_preview_target_action = &target->action;
    }
#else
    /* Item IDs leave secondary unset before this range-flags read. */
    if (((secondary.range->flags_3 & ABILITY_SECONDARY_FLAG_3_PERSEVERE) && action->ability_id < ABILITY_ID_ITEM_FIRST)
        || action->skillset_id == SKILLSET_ID_EQUIP_CHANGE) {
        g_battle_menu_preview_target_action = 0;
    } else {
        g_thread_call_target = (void (*)(void))battle_action_calculate_projected_effect;
        thread_call_on_main_stack(attacker, target, action);
        g_battle_menu_preview_target_action = &target->action;
    }
#endif
    thread_yield();
    thread_yield();
    battle_menu_load_selected_unit();
    battle_menu_display_hovered_unit_stats(&g_menu_attack_caster_stats_rect, UNIT_STATS_PANEL_ATTACK_CASTER,
        g_battle_menu_attack_caster_stats_setup, &g_menu_selected_unit_status);
}
