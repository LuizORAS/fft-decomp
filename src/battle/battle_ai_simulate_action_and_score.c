#include "fft/battle_action.h"
#include "fft/battle_ai.h"
#include "fft/battle_status.h"
#include "fft/battle_unit.h"
#include "psx/libetc.h"

/* Unit-state utility at full HP/MP, unchanged equipment, or one average HP of Golem. */
enum {
    AI_HP_SCORE_SCALE = 128,
    AI_MP_SCORE_SCALE = 64,
    AI_EQUIPMENT_SCORE = 51,
    AI_GOLEM_SCORE_SCALE = 128,
};

/* Simulate the current action and score the state it leaves (battle_ai_score_current_action keeps
 * the best). The unit moves to the action's destination (battle_ai_simulate_movement_and_pickup)
 * before or after using the action (battle_action_commit_command, battle_ai_simulate_ability_use);
 * Defend makes it Defending, and Defend or moving only count as a sure hit. Its turn then ends and the
 * clock runs (battle_turn_run_clock, at most 256 events) while any unit is charging or performing:
 * charged actions land, statuses tick, and a unit that gets a turn and can move is marked as acting
 * before this one. The score sums over every unit in the plan, negated for enemies: HP * 128 / max HP;
 * each status's weight (g_battle_ai_status_priority_weights), Darkness and Silence scaled by how much
 * the unit relies on what they block, helpful and disabling statuses ignored on a Confused, Blood
 * Sucked or Charmed unit; 51 per equipment slot unchanged; MP * 64 / max MP, halved below its
 * cheapest cost, times its share of MP abilities; and its team's Golem. A thrown item other than a
 * shuriken or ball halves the hit chance. Every unit is restored afterwards. With
 * REFRESH_UNIT_STATUS_FLAGS it only refreshes the decision flags. Returns 1 after scoring, 0 when the
 * action fails or for the refresh pass, -1 to suspend (also past 441 horizontal blanks, or 313 while
 * the clock runs).
 *
 * Save/restore helpers preserve selected unit data, not the whole AI workspace; resumed phases skip
 * fresh action preparation. */
s32 battle_ai_simulate_action_and_score(void) {
    battle_ai_data_t* ai = &g_battle_ai_data_base;
    battle_stats_t* unit;
    battle_ai_unit_decision_t* decision;
    action_command_t* action;
    battle_ai_data_t* clear_cursor;
    battle_ai_data_t* decision_base;
    s32 unit_index;
    s32 status;
    s32 equipment_slot;
    s32 weight_offset;
    s32 weight;
    s32 score;
    u32 below_cost;

    if (g_battle_ai_data_base.resume_pending && g_battle_ai_data_base.action_simulation_phase) {
        battle_ai_restore_unit_state(BATTLE_AI_SNAPSHOT_SIMULATED_STATUS);
        /* Resume: re-enter the suspended turn-advance or scoring phase. */
        if (g_battle_ai_data_base.action_simulation_phase == BATTLE_AI_SIMULATION_ADVANCE_TURNS)
            goto advance_turns;
        if (g_battle_ai_data_base.action_simulation_phase == BATTLE_AI_SIMULATION_SCORE_STATE)
            goto score_state;
    }
    if (VSync(1) >= BATTLE_AI_DECISION_SCANLINE_LIMIT) {
        ai->action_simulation_phase = BATTLE_AI_SIMULATION_PREPARE_ACTION;
        return -1;
    }
    battle_ai_save_unit_state(BATTLE_AI_SNAPSHOT_BASELINE);
    ai->ability_processing_done = 0;
    /* Direct array iteration changes the bytes; retain a workspace-relative cursor. */
    for (unit_index = BATTLE_UNIT_SLOT_COUNT - 1,
        clear_cursor = (battle_ai_data_t*)((u8*)ai + (BATTLE_UNIT_SLOT_COUNT - 1));
        unit_index >= 0; unit_index--, clear_cursor = (battle_ai_data_t*)((u8*)clear_cursor - 1)) {
        clear_cursor->unit_acts_before_me[0] = 0;
    }
    ai->current_action.rank.priority = 0;
    ai->current_action.rank.base_hit_percent = 0xff;
    if (ai->current_action.act_before_move) {
        if (ai->current_action.command.skillset_id != 0
            && ai->current_action.command.skillset_id != SKILLSET_ID_DEFEND) {
            action = &ai->current_action.command;
            if (battle_action_commit_command(action) == BATTLE_ATTACK_PREPARATION_IMMEDIATE
                && battle_ai_simulate_ability_use(action) == 0) {
                battle_ai_restore_unit_state(BATTLE_AI_SNAPSHOT_BASELINE);
                ai->resume_pending = 0;
                return 0;
            }
            ai->acting_unit->action_taken = 1;
            battle_ai_simulate_movement_and_pickup();
        }
    } else {
        battle_ai_simulate_movement_and_pickup();
        if (ai->current_action.command.skillset_id != 0
            && ai->current_action.command.skillset_id != SKILLSET_ID_DEFEND) {
            action = &ai->current_action.command;
            if (battle_action_commit_command(action) == BATTLE_ATTACK_PREPARATION_IMMEDIATE
                && battle_ai_simulate_ability_use(action) == 0) {
                battle_ai_restore_unit_state(BATTLE_AI_SNAPSHOT_BASELINE);
                ai->resume_pending = 0;
                return 0;
            }
            ai->acting_unit->action_taken = 1;
        }
    }
    if (ai->current_action.command.skillset_id == SKILLSET_ID_DEFEND) {
        battle_status_inflict_defending_to_battle_id(ai->acting_unit_id);
        ai->current_action.rank.base_hit_percent = 100;
        ai->acting_unit->action_taken = 1;
    }
    if (ai->current_action.command.skillset_id == 0)
        ai->current_action.rank.base_hit_percent = 100;
    battle_turn_end(ai->acting_unit_id);
    ai->saved_between_turn_state = g_battle_turn_clock_state;
    ai->saved_between_turn_resume_state = g_battle_turn_clock_resume_state;
    ai->saved_turn_unit_id = g_battle_turn_unit_id;
    ai->simulated_turn_events = 0;
    for (;;) {
    advance_turns:
        if (VSync(1) >= BATTLE_AI_TURN_SIMULATION_SCANLINE_LIMIT) {
            battle_ai_save_unit_state(BATTLE_AI_SNAPSHOT_SIMULATED_STATUS);
            ai->action_simulation_phase = BATTLE_AI_SIMULATION_ADVANCE_TURNS;
            return -1;
        }
        ai->simulated_turn_events++;
        if (ai->simulated_turn_events >= BATTLE_AI_SIMULATED_TURN_LIMIT)
            break;
        unit_index = battle_turn_run_clock(BATTLE_BETWEEN_TURN_MODE_RUN);
        status = unit_index & 0xff00;
        unit_index &= 0xff;
        unit = &g_battle_unit_stats[unit_index];
        if (status == BATTLE_TURN_EVENT_NONE)
            break;
        if (status == BATTLE_TURN_EVENT_ACTION_RESULT) {
            battle_action_apply_target_result(unit_index);
        } else if (status == BATTLE_TURN_EVENT_ABILITY_READY) {
            if (battle_ai_simulate_ability_use(&unit->action_command) == 0) {
                g_battle_turn_clock_state = ai->saved_between_turn_state;
                g_battle_turn_clock_resume_state = ai->saved_between_turn_resume_state;
                g_battle_turn_unit_id = ai->saved_turn_unit_id;
                /* An early return duplicates this shared cleanup tail. */
                goto finish_without_scoring;
            }
            unit->charged_ability_ct = 0xff;
            unit->status_sets.current[BATTLE_STATUS_BYTE_INDEX(BATTLE_STATUS_ID_CHARGING)]
                &= ~(BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_CHARGING)
                    | BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_PERFORMING));
            unit->inflicted_status[BATTLE_STATUS_BYTE_INDEX(BATTLE_STATUS_ID_CHARGING)]
                &= ~(BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_CHARGING)
                    | BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_PERFORMING));
        } else if (status == BATTLE_TURN_EVENT_UNIT_READY) {
            if (battle_ai_can_unit_move_within(0, unit_index))
                ai->unit_acts_before_me[unit_index] = 1;
            unit->has_turn = 0;
        }
        for (unit_index = 0; unit_index < BATTLE_UNIT_SLOT_COUNT; unit_index++) {
            unit = &g_battle_unit_stats[unit_index];
            if (unit->entd_slot != BATTLE_ENTD_SLOT_NONE
                && (unit->status_sets.current[BATTLE_STATUS_BYTE_INDEX(BATTLE_STATUS_ID_CHARGING)]
                    & (BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_CHARGING)
                        | BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_PERFORMING))))
                break;
        }
        if (unit_index == BATTLE_UNIT_ID_NONE)
            break;
    }
    g_battle_turn_clock_state = ai->saved_between_turn_state;
    g_battle_turn_clock_resume_state = ai->saved_between_turn_resume_state;
    g_battle_turn_unit_id = ai->saved_turn_unit_id;
score_state:
    if (VSync(1) >= BATTLE_AI_DECISION_SCANLINE_LIMIT) {
        battle_ai_save_unit_state(BATTLE_AI_SNAPSHOT_SIMULATED_STATUS);
        ai->action_simulation_phase = BATTLE_AI_SIMULATION_SCORE_STATE;
        return -1;
    }
    if (ai->targetability.live.target_setting_flags & BATTLE_AI_TARGET_SETTING_REFRESH_UNIT_STATUS_FLAGS) {
        for (unit_index = 0; unit_index < BATTLE_UNIT_SLOT_COUNT; unit_index++) {
            decision = &ai->unit_decisions[unit_index];
            unit = &g_battle_unit_stats[unit_index];
            decision->flags &= ~(
                BATTLE_AI_DECISION_DEAD_WITHOUT_RERAISE | BATTLE_AI_DECISION_PETRIFIED | BATTLE_AI_DECISION_JUMPING);
            if ((unit->status_sets.current[BATTLE_STATUS_BYTE_INDEX(BATTLE_STATUS_ID_DEAD)]
                    & BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_DEAD))
                && !(unit->status_sets.current[BATTLE_STATUS_BYTE_INDEX(BATTLE_STATUS_ID_RERAISE)]
                    & BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_RERAISE)))
                decision->flags |= BATTLE_AI_DECISION_DEAD_WITHOUT_RERAISE;
            if (unit->status_sets.current[BATTLE_STATUS_BYTE_INDEX(BATTLE_STATUS_ID_PETRIFY)]
                & BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_PETRIFY))
                decision->flags |= BATTLE_AI_DECISION_PETRIFIED;
#ifdef FIX_BUGS
            /* The edition builds the bit from Blood Suck, the status Protect allies reads it for (QUIRKS.md). */
            if (unit->status_sets.current[BATTLE_STATUS_BYTE_INDEX(BATTLE_STATUS_ID_BLOOD_SUCK)]
                & BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_BLOOD_SUCK))
                decision->flags |= BATTLE_AI_DECISION_JUMPING;
#else
            if (unit->status_sets.current[BATTLE_STATUS_BYTE_INDEX(BATTLE_STATUS_ID_JUMP)]
                & BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_JUMP))
                decision->flags |= BATTLE_AI_DECISION_JUMPING;
#endif
            if (!(decision->flags & BATTLE_AI_DECISION_WORTH_TARGETING)
                && !(unit->status_sets.current[BATTLE_STATUS_BYTE_INDEX(BATTLE_STATUS_ID_PETRIFY)]
                    & BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_PETRIFY))
                && (!(unit->status_sets.current[BATTLE_STATUS_BYTE_INDEX(BATTLE_STATUS_ID_DEAD)]
                        & BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_DEAD))
                    || (unit->status_sets.current[BATTLE_STATUS_BYTE_INDEX(BATTLE_STATUS_ID_RERAISE)]
                        & BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_RERAISE)))
                && !(unit->status_sets.current[BATTLE_STATUS_BYTE_INDEX(BATTLE_STATUS_ID_DEATH_SENTENCE)]
                    & BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_DEATH_SENTENCE)))
                decision->flags |= BATTLE_AI_DECISION_WORTH_TARGETING;
        }
    } else {
        battle_ai_clear_current_action_decided();
        for (unit_index = 0; unit_index < BATTLE_UNIT_SLOT_COUNT; unit_index++) {
            status = ai->unit_compact_indices[unit_index];
            if (status == 0xff)
                continue;
            unit = &g_battle_unit_stats[unit_index];
            score = unit->hp;
            score *= AI_HP_SCORE_SCALE;
            score /= unit->max_hp;
            status = 0;
            /* A pointer to unit_decisions adds an addiu and changes scheduling;
             * retain the workspace-relative stride. */
            decision_base = (battle_ai_data_t*)((u8*)ai + unit_index * sizeof(battle_ai_unit_decision_t));
            for (weight_offset = 0; status < BATTLE_STATUS_COUNT; status++, weight_offset += 2) {
                if (!((unit->status_sets.current[status / 8] << (status % 8)) & 0x80))
                    continue;
                /* Indexing the s16 table changes register allocation; retain the byte cursor. */
                weight = *(s16*)((u8*)g_battle_ai_status_priority_weights + weight_offset);
                if (((unit->status_sets.current[BATTLE_STATUS_BYTE_INDEX(BATTLE_STATUS_ID_CONFUSION)]
                         & (BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_CONFUSION)
                             | BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_BLOOD_SUCK)))
                        || (unit->status_sets.current[BATTLE_STATUS_BYTE_INDEX(BATTLE_STATUS_ID_CHARM)]
                            & BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_CHARM)))
                    && weight >= 0)
                    continue;
                switch (status) {
                case BATTLE_STATUS_ID_DARKNESS:
                    weight = weight * decision_base->unit_decisions[0].evade_mod / 4;
                    break;
                case BATTLE_STATUS_ID_SILENCE:
                    weight = weight * decision_base->unit_decisions[0].silence_mod / 4;
                    break;
                case BATTLE_STATUS_ID_CONFUSION:
                case BATTLE_STATUS_ID_BLOOD_SUCK:
                case BATTLE_STATUS_ID_CHARM:
                    if ((unit->status_sets.current[BATTLE_STATUS_BYTE_INDEX(BATTLE_STATUS_ID_SLOW)]
                            & (BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_SLOW)
                                | BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_STOP)))
                        || (unit->status_sets.current[BATTLE_STATUS_BYTE_INDEX(BATTLE_STATUS_ID_SLEEP)]
                            & (BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_SLEEP)
                                | BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_DONT_MOVE)
                                | BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_DONT_ACT))))
                        weight /= 2;
                    break;
                case BATTLE_STATUS_ID_SLOW:
                case BATTLE_STATUS_ID_STOP:
                case BATTLE_STATUS_ID_SLEEP:
                case BATTLE_STATUS_ID_DONT_MOVE:
                case BATTLE_STATUS_ID_DONT_ACT:
                    if ((unit->status_sets.current[BATTLE_STATUS_BYTE_INDEX(BATTLE_STATUS_ID_CONFUSION)]
                            & (BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_CONFUSION)
                                | BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_BLOOD_SUCK)))
                        || (unit->status_sets.current[BATTLE_STATUS_BYTE_INDEX(BATTLE_STATUS_ID_CHARM)]
                            & BATTLE_STATUS_BYTE_MASK(BATTLE_STATUS_ID_CHARM)))
                        continue;
                    break;
                }
                score += weight;
            }
            if (!(unit->unit_flags & UNIT_FLAG_MONSTER)) {
                for (equipment_slot = 0; equipment_slot < UNIT_EQUIPMENT_SLOT_COUNT; equipment_slot++) {
                    if (unit->equipment[equipment_slot]
                        == g_battle_ai_unit_snapshots[ai->unit_compact_indices[unit_index]].equipment[equipment_slot])
                        score += AI_EQUIPMENT_SCORE;
                }
            }
            if (unit->max_mp) {
                weight_offset = unit->mp;
                below_cost = weight_offset;
                weight_offset *= AI_MP_SCORE_SCALE;
                below_cost = below_cost < ai->unit_decisions[unit_index].lowest_mp_cost;
                weight_offset /= unit->max_mp;
                if (below_cost)
                    weight_offset = (u32)weight_offset >> 1;
                weight_offset = weight_offset * ai->unit_decisions[unit_index].mp_ability_mod / 4;
                score += weight_offset;
            }
            {
                u32 golem;

                weight_offset = g_battle_team_golem[(unit->team_flags >> BATTLE_TEAM_SHIFT) & BATTLE_TEAM_INDEX_MASK];
                golem = weight_offset;
                weight_offset *= AI_GOLEM_SCORE_SCALE;
                score += weight_offset / ai->present_unit_average_max_hp;
                if (golem < ai->team_golem[(unit->team_flags >> BATTLE_TEAM_SHIFT) & BATTLE_TEAM_INDEX_MASK])
                    score--;
            }
            if (ai->unit_decisions[unit_index].enemy_flag)
                score = -score;
            ai->current_action.rank.priority += score;
        }
        if (g_main_action_menu_types_by_skillset[ai->considered_ability.skillset] == ACTION_MENU_TYPE_WEAPON_INVENTORY
            && ai->considered_ability.ability_id != ABILITY_ID_THROW_SHURIKEN
            && ai->considered_ability.ability_id != ABILITY_ID_THROW_BALL)
            ai->current_action.rank.base_hit_percent >>= 1;
        battle_ai_restore_unit_state(BATTLE_AI_SNAPSHOT_BASELINE);
        ai->resume_pending = 0;
        return 1;
    }
finish_without_scoring:
    battle_ai_restore_unit_state(BATTLE_AI_SNAPSHOT_BASELINE);
    ai->resume_pending = 0;
    return 0;
}
