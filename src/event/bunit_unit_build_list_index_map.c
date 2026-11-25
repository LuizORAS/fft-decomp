#include "fft/event_bunit.h"

/* Build the unit-list selection-to-record index map.
 *
 * Linked entries encode a roster ID in their formation-order key; ordinary
 * entries retain their compact order among directly reorderable units.
 */
void bunit_unit_build_list_index_map(void) {
    s32 reorderable_index = 0;
    s32 unit_index;
#ifdef FIX_BUGS
    /* The edition's count of the cells before a mount's rider (QUIRKS.md). */
    s32 rider_index;
    s32 cell;
    s32 i;
#endif

    for (unit_index = 0; unit_index < g_bunit_unit_list_count; unit_index++) {
        bunit_unit_t* unit = g_bunit_unit_pointers[unit_index];

        if (!(unit->mount_info & BATTLE_MOUNT_INFO_FLAG_MOUNT)) {
            g_bunit_unit_list_index_map[unit_index] = reorderable_index;
            reorderable_index++;
        } else {
#ifdef FIX_BUGS
            /* The edition gives a mount its rider's cell; mounts take none (QUIRKS.md). */
            rider_index = bunit_unit_find_index_by_battle_id((unit->mount_info - BATTLE_MOUNT_INFO_FLAG_MOUNT));
            cell = 0;
            for (i = 0; i < rider_index; i++) {
                if (!(g_bunit_unit_pointers[i]->mount_info & BATTLE_MOUNT_INFO_FLAG_MOUNT)) {
                    cell++;
                }
            }
            g_bunit_unit_list_index_map[unit_index] = cell;
#else
            g_bunit_unit_list_index_map[unit_index]
                = bunit_unit_find_index_by_battle_id((unit->mount_info - BATTLE_MOUNT_INFO_FLAG_MOUNT));
#endif
        }
    }
}
