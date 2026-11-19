#include "fft/main.h"
#include "psx/types.h"

/* Allocate size bytes from the SMD heap, 16 cells of 2 KB at g_main_heap_smd_base tagged in
 * g_main_heap_smd_allocator_table, the same way main_heap_alloc does; returns 0 when nothing
 * fits. Music and sound effect files live there. */
void* main_heap_alloc_smd(u32 size) {
    u32 run = 0;
    u32 needed = size >> MAIN_HEAP_PAGE_SHIFT;
    u32 tag = 1;
    u32 index;
    u32 start;

    if ((size & (MAIN_HEAP_PAGE_SIZE - 1)) != 0) {
        needed++;
    }
    for (index = 0; index < MAIN_HEAP_SMD_PAGE_COUNT; index++) {
        u32 cell;

        cell = g_main_heap_smd_allocator_table[index];
        if (cell == 0) {
            if (run == 0) {
                start = index;
            }
            run++;
            if (run >= needed) {
                break;
            }
        } else {
            if (cell >= tag) {
                tag = cell + 1;
            }
            run = 0;
        }
    }
#ifdef FIX_BUGS
    /* The edition gives the block the next tag when the cell after it carries its tag (QUIRKS.md). */
    if (run >= needed) {
        u8* cells = &g_main_heap_smd_allocator_table[start];

        if (start + run < MAIN_HEAP_SMD_PAGE_COUNT && cells[run] == tag) {
            tag++;
        }
        for (index = 0; index < run; index++) {
            cells[index] = tag;
        }
        return &g_main_heap_smd_base[start << MAIN_HEAP_PAGE_SHIFT];
    }
#else
    if (run >= needed) {
        for (index = 0; index < run; index++) {
            g_main_heap_smd_allocator_table[start + index] = tag;
        }
        return &g_main_heap_smd_base[start << MAIN_HEAP_PAGE_SHIFT];
    }
#endif
    return 0;
}
