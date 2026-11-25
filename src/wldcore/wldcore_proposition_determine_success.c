#include "fft/wldcore.h"
#include "psx/types.h"

/* Decide how a returned proposition went. Called back: a bad failure with no reward, and `recalled` set
 * so the report is skipped. Otherwise each unit sent scores, from the proposition data: its job
 * category's score for the row's category (record 3) and for its job list (record 4), and its Brave,
 * Faith and level tiers ((value - 1) / 20 for the first two, / 10 for the level; records 5 and 6). A
 * total of 99 or more succeeds, 59 or more fails, and less fails badly. Either failure still succeeds
 * with a roll below 61 in 100 when the row names a preferred job category that the unit tested holds.
 *
 * The preferred-job scan reads the party record once, with the scoring loop's
 * leftover index, as the target does (see QUIRKS.md). The target builds the
 * row walk pointers from the proposition_index address, which needs the rows and that field to be
 * members of one object (g_wldcore_proposition_outcome). */
void wldcore_proposition_determine_success(void) {
    const u8* job_table_a;
    const u8* job_table_b;
    const u8* tier_table_a;
    const u8* tier_table_b;
#ifdef FIX_BUGS
    /* The edition's level scores (QUIRKS.md). */
    const u8* level_table;
#endif
    party_unit_t* unit;
    s32 participant_count;
    s32 total;
    s32 score;
    s32 tier;
    s32 value;
    s32 i;

    participant_count = g_main_active_propositions[g_wldcore_proposition_outcome.proposition_index].participant_count;
    job_table_a = wldcore_proposition_get_data_pointer(WLDCORE_PROPOSITION_DATA_CATEGORY_JOB_SCORES);
    job_table_b = wldcore_proposition_get_data_pointer(WLDCORE_PROPOSITION_DATA_JOB_LIST_JOB_SCORES);
    tier_table_a = wldcore_proposition_get_data_pointer(WLDCORE_PROPOSITION_DATA_BRAVERY_LEVEL_SCORES);
    total = 0;
    tier_table_b = wldcore_proposition_get_data_pointer(WLDCORE_PROPOSITION_DATA_FAITH_SCORES);

    if (g_main_active_propositions[g_wldcore_proposition_outcome.proposition_index].flags
        & WLDCORE_PROPOSITION_PROGRESS_RECALLED) {
        for (i = 0; i < participant_count; i++) {
            g_wldcore_proposition_outcome.participant_gil[i] = 0;
            g_wldcore_proposition_outcome.participant_jp[i] = 0;
        }
        g_wldcore_proposition_outcome.recalled = 1;
        g_wldcore_proposition_outcome.result = WLDCORE_PROPOSITION_RESULT_BAD_FAILURE;
        g_wldcore_proposition_outcome.reward_type = WLDCORE_PROPOSITION_REWARD_NONE;
        return;
    }

    g_wldcore_proposition_outcome.recalled = 0;
    for (i = 0; i < participant_count; i++) {
        unit = wldcore_unit_get_party_unit(
            g_main_active_propositions[g_wldcore_proposition_outcome.proposition_index].participant_indices[i]);

        g_wldcore_proposition_outcome.participant_scores[i]
            = job_table_a[(g_wldcore_selected_proposition_row[0].category_index - 1) * 22
                + wldcore_proposition_get_job_category(unit->job_id)];
        g_wldcore_proposition_outcome.participant_scores[i]
            += job_table_b[(g_wldcore_selected_proposition_row[0].job_list_index - 1) * 22
                + wldcore_proposition_get_job_category(unit->job_id)];

        value = unit->bravery;
        if (value != 0) {
            tier = (value - 1) / 20;
        } else {
            tier = 0;
        }
        g_wldcore_proposition_outcome.participant_scores[i]
            += tier_table_a[(g_wldcore_selected_proposition_row[0].job_list_index - 1) * 5 + tier];

        value = unit->faith;
        if (value != 0) {
            tier = (value - 1) / 20;
        } else {
            tier = 0;
        }
        g_wldcore_proposition_outcome.participant_scores[i]
            += tier_table_b[(g_wldcore_selected_proposition_row[0].job_list_index - 1) * 5 + tier];

#ifdef FIX_BUGS
        /* The edition fetches the level scores' own record (QUIRKS.md). */
        level_table = wldcore_proposition_get_data_pointer(WLDCORE_PROPOSITION_DATA_LEVEL_SCORES);
#endif
        value = unit->level;
        if (value != 0) {
            tier = (value - 1) / 10;
        } else {
            tier = 0;
        }
#ifdef FIX_BUGS
        /* The edition reads the level scores, not the bravery scores (QUIRKS.md). */
        score = g_wldcore_proposition_outcome.participant_scores[i]
            + level_table[(g_wldcore_selected_proposition_row[0].job_list_index - 1) * 10 + tier];
#else
        score = g_wldcore_proposition_outcome.participant_scores[i]
            + tier_table_a[(g_wldcore_selected_proposition_row[0].job_list_index - 1) * 10 + tier];
#endif
        total += score;
        g_wldcore_proposition_outcome.participant_scores[i] = score;
    }

    g_wldcore_proposition_outcome.result = WLDCORE_PROPOSITION_RESULT_SUCCESS;
    if (total < 0x63) {
        g_wldcore_proposition_outcome.result = WLDCORE_PROPOSITION_RESULT_FAILURE;
    }
    if (total < 0x3B) {
        g_wldcore_proposition_outcome.result = WLDCORE_PROPOSITION_RESULT_BAD_FAILURE;
    }
    if (g_wldcore_proposition_outcome.result == WLDCORE_PROPOSITION_RESULT_SUCCESS) {
        return;
    }
    if (g_wldcore_selected_proposition_row[0].preferred_job == 0) {
        return;
    }

#ifdef FIX_BUGS
    /* The edition tests each unit sent (QUIRKS.md). */
    for (i = 0; i < participant_count; i++) {
        unit = wldcore_unit_get_party_unit(
            g_main_active_propositions[g_wldcore_proposition_outcome.proposition_index].participant_indices[i]);
        if (g_wldcore_selected_proposition_row[0].preferred_job - 1
            == wldcore_proposition_get_job_category(unit->job_id)) {
            if (((rand() * 100) >> 15) < 0x3D) {
                g_wldcore_proposition_outcome.result = WLDCORE_PROPOSITION_RESULT_SUCCESS;
                return;
            }
        }
    }
#else
    unit = wldcore_unit_get_party_unit(
        g_main_active_propositions[g_wldcore_proposition_outcome.proposition_index].participant_indices[i]);
    for (i = 0; i < participant_count; i++) {
        if (g_wldcore_selected_proposition_row[0].preferred_job - 1
            == wldcore_proposition_get_job_category(unit->job_id)) {
            if (((rand() * 100) >> 15) < 0x3D) {
                g_wldcore_proposition_outcome.result = WLDCORE_PROPOSITION_RESULT_SUCCESS;
                return;
            }
        }
    }
#endif
}
