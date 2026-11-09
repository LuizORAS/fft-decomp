#include "fft/main.h"

/* Claim the game-heap cells that cover size bytes from allocation, if they are all free, with
 * the lowest tag not in use. Returns the first cell's address, or 0 when allocation is outside
 * the heap's 0x20000 bytes or a cell is taken. WORLD and the EVENT overlays retry it at the heap
 * base before loading a companion overlay or an image there. */
u8* main_heap_reserve_at(u8* allocation, u32 size) {
    u32 addr;
    u32 base;
    s32 start;
    u32 nblocks;
    s32 id;
    u32 i;
    u32 j;

    addr = (u32)allocation;
    base = (u32)g_main_heap_high_overlay_load_address;
    id = 1;
    start = (allocation - g_main_heap_high_overlay_load_address) / MAIN_HEAP_PAGE_SIZE;
    nblocks = size >> MAIN_HEAP_PAGE_SHIFT;
    if ((size & (MAIN_HEAP_PAGE_SIZE - 1)) != 0) {
        nblocks += 1;
    }
    if (addr < base) {
        return 0;
    }
#ifdef FIX_BUGS
    /* The edition refuses a range that runs past the heap's last cell (QUIRKS.md). */
    if (start + nblocks > MAIN_HEAP_GAME_PAGE_COUNT) {
        return 0;
    }
#endif
    if (addr < base + MAIN_HEAP_GAME_PAGE_COUNT * MAIN_HEAP_PAGE_SIZE) {
        do {
            for (i = 0; i < MAIN_HEAP_GAME_PAGE_COUNT; i++) {
                if (g_main_heap_game_allocator_table[i] == id) {
                    id += 1;
                    break;
                }
            }
        } while (i != MAIN_HEAP_GAME_PAGE_COUNT);

        for (i = start; i < start + nblocks; i++) {
            if (g_main_heap_game_allocator_table[i] != 0) {
                break;
            }
        }
        if (i == start + nblocks) {
            for (j = 0; j < nblocks; j++) {
                g_main_heap_game_allocator_table[start + j] = id;
            }
            return ((start << MAIN_HEAP_PAGE_SHIFT) + g_main_heap_high_overlay_load_address);
        }
    }
    return 0;
}
