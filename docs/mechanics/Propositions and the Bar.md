---
type: mechanic
tier: 3
scope:
  - wldcore_proposition
  - wldcore_bar
---

# Propositions and the Bar

## What it does in the game

Every town has a Bar ([[wldcore_bar_push_menu_level]]). Its menu
([[wldcore_bar_build_menu_rows]]) has these rows:

| Row | When | What it does |
|---|---|---|
| Rumors | always | The town's rumors |
| Propositions | `EVENT_SCRIPT_VAR_PROPOSITIONS_ENABLED` | The jobs on offer |
| Job reports | a job has come back to this town | Hear how it went and collect |
| Jobs in progress | a job is still out | Call its units back |
| Leave | always | Back to the town's menu |

- **Rumors.** 64 rumors. A rumor shows in a town once its variable is set and
  its location mask includes that town ([[wldcore_bar_open_rumors]]).
- **Jobs on offer.** 96 propositions. One is offered at a Bar when all of
  these hold ([[wldcore_proposition_is_offered_at]]):
  - it is neither in progress nor completed;
  - it is not waiting out the days after a failure: 32 days after a failure,
    16 after a bad failure or a call-back;
  - its town matches, or it has none;
  - its conditions hold. A row can require:

    | Condition | Retail rows |
    |---|---|
    | A shop level | 75 |
    | Another proposition completed | 17 |
    | Today's zodiac sign | 4 |

  When the player opens a job:
  - the barkeeper sometimes gives a hint about it: 10 in 100, 20 after a
    failure, 40 after a bad failure ([[wldcore_proposition_push_detail_level]]);
  - at most 8 jobs can be out at once.
- **Taking a job.** The steps, in order:
  1. Pay the fee: 0 to 10,000 gil, the sum of the row's two fee amounts.
  2. Choose one to three units to send
     ([[wldcore_proposition_step_send_unit_level]]). Only generic humans of
     the first 16 roster slots who are not already away can go.
  3. Choose the number of days, between the row's minimum and maximum.
  4. Confirm the conditions.

  The units are then away (`proposition_status`). The job comes back on
  its last day, or the day after a call-back
  ([[wldcore_proposition_advance_daily_counters]]). Each location the party
  reaches on the map is a day.
- **The report.** At the Bar of the town where the job was taken:
  1. One unit reports in its own voice
     ([[wldcore_proposition_select_report_speaker]]). Every retail row picks
     the unit with the lowest Faith, and the line varies with its Faith and
     Brave and whether it is male.
  2. Others may speak, and the report may single out a unit for its job
     ([[wldcore_proposition_find_preferred_job_unit]]).
  3. The result panel pays the units and the war funds
     ([[wldcore_proposition_step_result_level]]).
- **Success** ([[wldcore_proposition_determine_success]]). Each unit sent
  scores from the job's category and job list:
  - its job category: two tables of 22 categories, by the row's category and
    by its job list;
  - its Brave tier and its Faith tier, in bands of 20;
  - its level tier, in bands of 10 (see the quirks).

  The three job lists weigh Brave and Faith differently:

  | Job list | Brave tiers (1-20 … 81-100) | Faith tiers |
  |---|---|---|
  | 1 | 5, 8, 10, 15, 20 | 20, 15, 10, 8, 5 |
  | 2 | 20, 15, 10, 8, 5 | 5, 8, 10, 15, 20 |
  | 3 | 10 each | 10 each |

  The sum of the units' scores gives the result:

  | Total | Result |
  |---|---|
  | 99 or more | success |
  | 59 to 98 | failure |
  | below 59 | bad failure |

  A unit of the row's preferred job category turns a failure into a success
  61 times in 100 (but see the quirks). A call-back is always a bad failure
  with nothing earned.
- **Rewards.** All of these depend on the result:
  - **JP and gil.** Each unit earns the row's JP (10 to 120) and gil (150 to
    5,500), plus its score ([[wldcore_proposition_calculate_base_jp_and_gil]]).
    A failure pays a tenth of the JP and a fifth of the gil, and a bad
    failure a twentieth of both.
  - **Extra days.** Days beyond the minimum scale the JP up to 120% and the
    gil up to 150% ([[wldcore_proposition_apply_extra_days_multiplier]]).
  - **Discoveries.** A successful row that finds something
    ([[wldcore_proposition_roll_bonus_reward]]) gives either a treasure (31
    rows, one of the first 31 treasures not yet found) or an unexplored land
    (16 rows), with extra JP and gil.
  - **Random bonus.** Other successful rows with the random bonus flag draw
    one of three bonuses: 60, 30 and 10 in 100.
  - **Milestones.** Reaching a count of treasures (1, 4, 8 … 31) or lands
    (1, 3, 6 … 16) plays a milestone announcement and jingle
    ([[wldcore_proposition_get_milestone_rank]]).

## Data and structures

| Record | What it holds |
|---|---|
| Proposition data (`g_wldcore_proposition_data_base`, static in WLDCORE at `0x8009d344`) | Fifteen records ([[wldcore_proposition_data_record_e]]): the 96 23-byte rows, fees, voice adjustments, job-category scores, job-list scores, Brave/level and Faith scores, the unused level scores, JP and gil amounts, discovery and random reward pairs, the extra-day percentages, the job lists |
| [[wldcore_proposition_fields_t]] | A row unpacked ([[wldcore_proposition_unpack_row]]): category, job list, fee, days, name, rewards, town, conditions, speaker mode, preferred job |
| [[g_main_active_propositions]] ([[wldcore_proposition_progress_t]]), `g_main_save_proposition_count` | The jobs out, at most 8: flags (dispatched, called back, returned), days elapsed and assigned, the town, the units sent |
| [[g_main_proposition_states]] | Per proposition, the days before it is offered again and which failure it was |
| `EVENT_SCRIPT_VAR_PROPOSITION_FIRST` + id | Per proposition: in progress, completed, attempted ([[wldcore_proposition_variable_flag_e]]) |
| [[g_wldcore_proposition_outcome]] ([[wldcore_proposition_report_t]]) | The outcome being reported: who reports, the result, the reward, and each unit's score, JP and gil |
| `party_unit_t.proposition_status` | A unit away on a job |
| Dates (`g_main_proposition_last_attempt_date_bits`, the treasure and land date bits) | When each proposition last succeeded, and when each treasure or land was found ([[wldcore_calendar_write_date]]) |

## Main flow

1. **Bar.** [[wldcore_bar_push_menu_level]] opens the Bar, then
   [[wldcore_bar_handle_menu_input]] takes the rows.
2. **Taking a job.** The levels follow in order:
   - [[wldcore_proposition_open_available_list]]: the jobs on offer;
   - [[wldcore_proposition_push_detail_level]]: one job's detail;
   - [[wldcore_proposition_handle_accept_input]]: the fee;
   - [[wldcore_proposition_handle_accept_confirm_input]]: a free unit and
     the gil;
   - [[wldcore_proposition_push_send_unit_level]]: the units sent;
   - [[wldcore_proposition_push_days_level]]: the days;
   - [[wldcore_proposition_handle_dispatch_confirm_input]]: payment and the
     record.
3. **Days pass.** Each day [[wldcore_calendar_advance_day]] runs
   [[wldcore_proposition_advance_daily_counters]].
4. **The report.** It runs in this order:
   - [[wldcore_proposition_handle_report_list_input]]:
     [[wldcore_proposition_prepare_report]] works out the speaker, the
     success, the JP and gil, the bonus and the extra days;
   - [[wldcore_proposition_push_report_level]] and
     [[wldcore_proposition_step_report_level]]: the messages;
   - [[wldcore_proposition_push_result_level]] and
     [[wldcore_proposition_step_result_level]]: the payment and the
     bookkeeping;
   - [[wldcore_proposition_push_milestone_level]]: the milestone, if any.
5. **Calling back.** [[wldcore_proposition_open_active_list]] lists the jobs
   out, then [[wldcore_proposition_handle_recall_input]] calls the chosen
   one back.

## Where to change

| To change | Edit |
|---|---|
| Which jobs are offered where and when | The rows' town and condition fields, and [[wldcore_proposition_is_offered_at]] |
| How success is judged | The score records 3 to 6, the thresholds 99 and 59, and the preferred-job roll in [[wldcore_proposition_determine_success]] |
| What a job pays | The JP and gil amounts, the failure divisors in [[wldcore_proposition_calculate_base_jp_and_gil]], and the extra-day percentages |
| The wait after a failure | The 32 and 16 days set in [[wldcore_proposition_step_result_level]] |
| How many jobs can be out | The 8 in [[wldcore_proposition_handle_available_list_input]], and the size of `g_main_active_propositions` in the save |

## Quirks and debts

- [[wldcore_proposition_determine_success]]:
  - the preferred-job promotion tests one unit (the scoring loop's leftover
    index) every pass; the Developer's Cut tests each unit sent;
  - the level score is read from the Brave scores and runs into the Faith
    scores, while record 7, the real level scores, is never read; the
    Developer's Cut reads it.
- [[wldcore_proposition_select_report_speaker]] leaves its result unset for
  a speaker mode outside 1-4 (no retail row has one).
- An event's date advance does not count toward the jobs out
  ([[wldcore_calendar_advance_days]]).
- [[wldcore_proposition_step_result_level]] clears the in-progress bit with a
  mask that also drops the variable's high half.

## Functions in scope

![[Propositions and the Bar scope]]
