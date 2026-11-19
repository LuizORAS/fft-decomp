#include "fft/battle_action.h"
#include "psx/types.h"

/* Queue the post-action effect messages of one unit that was hit, one 8-byte record (message code,
 * unit, value) per change in g_battle_action_post_effect_msgs, in this order: Speed 0x28, CT 0x27,
 * Brave 0x29, Faith 0x2a, PA 0x2b, MA 0x2c, gil gained 0x2e, EXP lost 0x2f, a level up or down 0x2d,
 * a broken item 0x1d (value bit 0x8000 for a broken katana) and a stolen item 0x1c. A unit is skipped
 * once 16 messages wait. battle_action_show_next_effect_message takes them from the end, so the last
 * one queued shows first.
 *
 * The 11-way dispatch really is a jump table in the target (at 0x80067548),
 * which is why the .ld places a .rodata output section there. */
void battle_action_queue_unit_effect_messages(battle_unit_t* runtime) {
    battle_action_data_t* action;
    s32 i;
    s32 queued;
    s32 off;

    if (runtime == 0) {
        return;
    }
    if (runtime->stats == 0) {
        return;
    }
    if (g_battle_action_post_effect_msg_counter >= 0x10) {
        return;
    }
    action = &runtime->stats->action;
    if (action->hit == 0) {
        return;
    }

    for (i = 0; i < 11; i++) {
        queued = 0;
        switch (i) {
        case 0:
            if (action->sp_change != 0) {
                g_battle_action_post_effect_msgs[g_battle_action_post_effect_msg_counter].code = 0x28;
                g_battle_action_post_effect_msgs[g_battle_action_post_effect_msg_counter].value = action->sp_change;
                queued = 1;
            }
            break;
        case 1:
            if (action->ct_change != 0) {
                g_battle_action_post_effect_msgs[g_battle_action_post_effect_msg_counter].code = 0x27;
                g_battle_action_post_effect_msgs[g_battle_action_post_effect_msg_counter].value = action->ct_change;
                queued = 1;
            }
            break;
        case 2:
            if (action->brave_change != 0) {
                g_battle_action_post_effect_msgs[g_battle_action_post_effect_msg_counter].code = 0x29;
                g_battle_action_post_effect_msgs[g_battle_action_post_effect_msg_counter].value = action->brave_change;
                queued = 1;
            }
            break;
        case 3:
            if (action->faith_change != 0) {
                g_battle_action_post_effect_msgs[g_battle_action_post_effect_msg_counter].code = 0x2a;
                g_battle_action_post_effect_msgs[g_battle_action_post_effect_msg_counter].value = action->faith_change;
                queued = 1;
            }
            break;
        case 4:
            if (action->pa_change != 0) {
                g_battle_action_post_effect_msgs[g_battle_action_post_effect_msg_counter].code = 0x2b;
                g_battle_action_post_effect_msgs[g_battle_action_post_effect_msg_counter].value = action->pa_change;
                queued = 1;
            }
            break;
        case 5:
            if (action->ma_change != 0) {
                g_battle_action_post_effect_msgs[g_battle_action_post_effect_msg_counter].code = 0x2c;
                g_battle_action_post_effect_msgs[g_battle_action_post_effect_msg_counter].value = action->ma_change;
                queued = 1;
            }
            break;
        case 6:
            if (action->gil_change > 0) {
                g_battle_action_post_effect_msgs[g_battle_action_post_effect_msg_counter].code = 0x2e;
                g_battle_action_post_effect_msgs[g_battle_action_post_effect_msg_counter].value = action->gil_change;
                queued = 1;
            }
            break;
        case 7:
            if (action->exp_change & 0x80) {
                g_battle_action_post_effect_msgs[g_battle_action_post_effect_msg_counter].code = 0x2f;
                g_battle_action_post_effect_msgs[g_battle_action_post_effect_msg_counter].value = action->exp_change;
                queued = 1;
            }
            break;
        case 8:
            if (action->special_effect
                & (BATTLE_ACTION_SPECIAL_EFFECT_LEVEL_UP | BATTLE_ACTION_SPECIAL_EFFECT_LEVEL_DOWN)) {
                g_battle_action_post_effect_msgs[g_battle_action_post_effect_msg_counter].code = 0x2d;
                g_battle_action_post_effect_msgs[g_battle_action_post_effect_msg_counter].value
                    = action->special_effect;
                queued = 1;
            }
            break;
        case 9:
            if (action->special_effect
                & (BATTLE_ACTION_SPECIAL_EFFECT_BREAK_EQUIPMENT
                    | BATTLE_ACTION_SPECIAL_EFFECT_DRAW_OUT_KATANA_BROKEN)) {
                g_battle_action_post_effect_msgs[g_battle_action_post_effect_msg_counter].code = 0x1d;
                off = g_battle_action_post_effect_msg_counter;
                g_battle_action_post_effect_msgs[off].value = action->item_lost;
                if (action->special_effect & BATTLE_ACTION_SPECIAL_EFFECT_DRAW_OUT_KATANA_BROKEN) {
                    g_battle_action_post_effect_msgs[off].value = action->item_lost | 0x8000;
                }
                queued = 1;
            }
            break;
        case 10:
            if (action->special_effect & BATTLE_ACTION_SPECIAL_EFFECT_STEAL_ITEM) {
                queued = 1;
                g_battle_action_post_effect_msgs[g_battle_action_post_effect_msg_counter].code = 0x1c;
                g_battle_action_post_effect_msgs[g_battle_action_post_effect_msg_counter].value = action->item_lost;
            }
            break;
        }
        if (queued != 0) {
            g_battle_action_post_effect_msgs[g_battle_action_post_effect_msg_counter].unit = runtime->stats->battle_id;
            g_battle_action_post_effect_msg_counter += 1;
#ifdef FIX_BUGS
            /* The edition stops once the queue is full (QUIRKS.md). */
            if (g_battle_action_post_effect_msg_counter >= 0x10) {
                break;
            }
#endif
        }
    }
}
