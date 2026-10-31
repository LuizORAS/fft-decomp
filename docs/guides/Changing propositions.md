---
type: guide
mechanics: [Propositions and the Bar]
---

# Changing propositions

## Goal

Change the Bar's jobs: which are offered where, what they cost and pay, how
their success is judged, and how long a failure keeps one away.

## Where it lives

Everything is in WLDCORE: the code, and the proposition data, static at
`0x8009d344` (`g_wldcore_proposition_data_base`). The data is a table of 15
offsets, then the records ([[wldcore_proposition_data_record_e]]). The
[[Propositions and the Bar]] page describes the retail rules.

| What | Where |
|---|---|
| A job's town, conditions, days, fee, rewards, category, job list, speaker, preferred job | Its 23-byte row in record 0; [[wldcore_proposition_fields_t]] names the fields as [[wldcore_proposition_unpack_row]] expands them |
| Fee amounts | Record 1, indexed by the row's two fee bytes |
| Success scores | Records 3 (job category by row category), 4 (job category by job list), 5 (Brave, and the level by mistake), 6 (Faith) |
| Success thresholds, preferred-job roll | [[wldcore_proposition_determine_success]] (99, 59, 61 in 100) |
| JP and gil | Records 8 and 9 (by the row's reward bytes), the failure divisors in [[wldcore_proposition_calculate_base_jp_and_gil]], the extra-day percentages in records 12 and 13 |
| Discovery and random rewards | Records 10 and 11, [[wldcore_proposition_roll_bonus_reward]] |
| Wait after a failure | 32 and 16 days, [[wldcore_proposition_step_result_level]] |
| Jobs out at once | 8, [[wldcore_proposition_handle_available_list_input]] |
| Who can be sent | [[wldcore_proposition_load_send_unit_candidates]] and the same filter in [[wldcore_proposition_handle_accept_confirm_input]] |

## What changes with it

- **Text.** Each job's name, description, hint and report lines are text tied
  to its id and category; a new job needs them.
- **The save.** The jobs out, the per-job states and the dates are saved
  with the game; the 8-job limit is the size of their save record.
- **Scripts.** The story enables the Bar's propositions and sets the shop
  level the rows test.
- **Brave Story.** Its proposition, treasure and unexplored-land lists read
  the same variables and dates.

## What can be done today

Nothing that changes a byte: `make check` compares every function with its
original hash. To try an idea, change the game's memory in an emulator:
`make run` launches PCSX-Redux, and `make map` writes the symbol maps that
name these tables in its debugger. While WLDCORE is loaded, the proposition
rows start at `0x8009d344` + the first offset (row `n` at + `n` * 23), and
the extra-day percentages are record 12 and 13's ten 16-bit values. Code
changes wait for build 3.
