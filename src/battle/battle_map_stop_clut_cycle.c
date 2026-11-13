#include "fft/battle_map.h"
#include "psx/types.h"

/* Stop a color rotation by the handle battle_map_start_clut_cycle
 * returned (slot index + 1). Returns 1 for a handle 1-8, else 0.
 *
 * Retail clears the active byte at handle * 10, not (handle - 1) * 10: it stops
 * the slot after the handle's own, and handle 8 clears the byte that follows the
 * eight slots, the first of g_battle_gfx_cycled_deployed_palettes. */
s32 battle_map_stop_clut_cycle(s32 handle) {
    if (handle >= 1 && handle <= MAP_CLUT_CYCLE_SLOT_COUNT) {
#ifdef FIX_BUGS
        /* The edition stops the handle's own slot (QUIRKS.md). */
        g_battle_map_clut_cycle_states[handle - 1].active = 0;
#else
        g_battle_map_clut_cycle_states[handle].active = 0;
#endif
        return 1;
    }
    return 0;
}
