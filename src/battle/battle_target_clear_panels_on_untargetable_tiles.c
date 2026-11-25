#include "fft/battle_map.h"
#include "fft/battle_target.h"
#include "psx/types.h"

/* Clear the range and mark of each untargetable tile's panel, but only on the lower layer: the loop
 * stops after 256 panels (QUIRKS.md). */
void battle_target_clear_panels_on_untargetable_tiles(void) {
    s32 i;
    map_tile_t* src;
    battle_target_panel_t* dst;

    i = 0;
    dst = g_battle_target_panels;
    src = g_battle_map_tile_data;
#ifdef FIX_BUGS
    /* The edition visits both layers' panels, as the weapon range does (QUIRKS.md). */
    for (; i < MAP_TILE_COUNT; src++, i++, dst++) {
        if (src->flags.untargetable) {
            dst->remaining_range = 0;
            dst->mark = 0;
        }
    }
#else
    for (; i < 256; src++, i++, dst++) {
        if (src->flags.untargetable) {
            dst->remaining_range = 0;
            dst->mark = 0;
        }
    }
#endif
}
