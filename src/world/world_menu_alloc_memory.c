#include "fft/thread.h"
#include "fft/world.h"
#include "psx/libgpu.h"
#include "psx/types.h"

/* WORLD twin of battle_menu_alloc_memory. */
void* world_menu_alloc_memory(s32 size) {
    s32 page;
    s32 pages;
    s32 i;
    u8* buffer;

    size = (size & 0xFF) != 0 ? (size >> 8) + 1 : size >> 8;
    for (;;) {
        for (page = 0; page < WORLD_MENU_BUFFER_PAGE_COUNT; page++) {
            if (g_world_menu_buffer_allocation_map[page] == 0) {
#ifdef FIX_BUGS
                /* The edition tests the size first and marks pages only for a claimed record (QUIRKS.md). */
                for (pages = 0; pages < size; pages++) {
                    if (pages + page >= WORLD_MENU_BUFFER_PAGE_COUNT) {
                        /* Leaves the run and page loops for the retry wait. */
                        goto fail;
                    }
                    if (g_world_menu_buffer_allocation_map[pages + page] != 0) {
                        break;
                    }
                }
                if (pages >= size) {
                    for (i = 0; i < MENU_BUFFER_ALLOCATION_COUNT; i++) {
                        if (g_world_menu_allocator_slot_records[i].buffer == (u8*)-1) {
                            buffer = g_world_menu_buffer_arena[page];
                            g_world_menu_allocator_slot_records[i].buffer = buffer;
                            g_world_menu_allocator_slot_records[i].page_count = size;
                            for (pages = 0; pages < size; pages++) {
                                g_world_menu_buffer_allocation_map[page + pages] = 1;
                            }
                            return buffer;
                        }
                    }
                    break;
                }
#else
                for (pages = 0;; pages++) {
                    if (pages + page >= WORLD_MENU_BUFFER_PAGE_COUNT) {
                        /* Leaves the run and page loops for the retry wait. */
                        goto fail;
                    }
                    if (pages >= size || g_world_menu_buffer_allocation_map[pages + page] != 0) {
                        break;
                    }
                }
                if (pages >= size) {
                    for (pages = 0; pages < size; pages++) {
                        g_world_menu_buffer_allocation_map[page + pages] = 1;
                    }
                    for (i = 0; i < MENU_BUFFER_ALLOCATION_COUNT; i++) {
                        if (g_world_menu_allocator_slot_records[i].buffer == (u8*)-1) {
                            buffer = g_world_menu_buffer_arena[page];
                            g_world_menu_allocator_slot_records[i].buffer = buffer;
                            g_world_menu_allocator_slot_records[i].page_count = pages;
                            return buffer;
                        }
                    }
                    break;
                }
#endif
            }
        }
    fail:
        if (g_main_font_print_enabled != 0) {
            FntPrint(g_world_menu_wait_for_allocation_message, g_thread_current_id, size);
        }
        thread_yield();
    }
}
