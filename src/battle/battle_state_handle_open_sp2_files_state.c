#include "fft/battle.h"

/* OPEN_SP2_FILES: read the casting unit's SP2 file (attack frames beyond its SEQ) into a
 * 0x8000-byte heap buffer, then upload it (64x256) to the first free one of the two SP2 VRAM
 * slots (owner unit id | 0x40) and prepare the strike (battle_action_start_strike). With no SP2
 * to load the strike starts at once; while neither slot is free the state waits. The buffer is
 * freed when the read starts and kept when the loader was busy, so the file lands in the kept
 * block only because the next allocation gets the same pages back (QUIRKS.md). */
void battle_state_handle_open_sp2_files_state(void) {
    RECT image_rect;
    battle_unit_t* unit;
    s32 open_result;
    s32 counter;
    s32 empty_slot;
    s32 offset;
    RECT* image_rect_address;
    u32* sp2_data;

    unit = battle_unit_get_casting();
    if (g_battle_gfx_sp2_data == 0) {
        g_battle_gfx_sp2_data = main_heap_alloc(0x8000);
        open_result = battle_gfx_open_sp2_file(unit, g_battle_gfx_sp2_data);
#ifdef FIX_BUGS
        /* The edition keeps the buffer while the read runs and frees it on a refusal (QUIRKS.md). */
        if (open_result == -1) {
            main_heap_free(g_battle_gfx_sp2_data);
            g_battle_gfx_sp2_data = 0;
            battle_action_start_strike();
        } else if (open_result != 0) {
            main_heap_free(g_battle_gfx_sp2_data);
            g_battle_gfx_sp2_data = 0;
        }
#else
        if (open_result == -1) {
            main_heap_free(g_battle_gfx_sp2_data);
            g_battle_gfx_sp2_data = 0;
            battle_action_start_strike();
        } else if (open_result == 0) {
            main_heap_free(g_battle_gfx_sp2_data);
            g_battle_gfx_sp2_data = 0;
        }
#endif
    } else if (main_file_is_still_loading() == 0) {
        counter = 0;
        empty_slot = BATTLE_GFX_VRAM_SLOT_OWNER_NONE;
        /* Keep the target's byte-offset induction; advancing a slot pointer changes the exact code. */
        offset = 0;
        do {
            if (((battle_gfx_vram_slot_t*)((u8*)g_battle_gfx_vram_slots + offset))->owner == empty_slot) {
                image_rect_address = &image_rect;
                sp2_data = g_battle_gfx_sp2_data;
                ((battle_gfx_vram_slot_t*)((u8*)g_battle_gfx_vram_slots + offset))->owner
                    = unit->runtime_id | BATTLE_GFX_VRAM_SLOT_OWNER_SP2;
                image_rect_address->x
                    = ((battle_gfx_vram_slot_t*)((u8*)g_battle_gfx_vram_slots + offset))->image_rect.x;
                image_rect_address->y
                    = ((battle_gfx_vram_slot_t*)((u8*)g_battle_gfx_vram_slots + offset))->image_rect.y;
                image_rect_address->w = 0x40;
                image_rect_address->h = 0x100;
                LoadImage(image_rect_address, sp2_data);
                battle_action_start_strike();
                break;
            }
            counter++;
            offset += 0x7564;
        } while (counter < 2);
    }
    battle_state_update_units();
    battle_menu_update_menus_and_event_speed(main_gfx_get_otag(), g_main_input_current_buttons);
}
