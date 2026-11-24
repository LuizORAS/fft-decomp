-- Retail bugs of wldcore_proposition_determine_success (QUIRKS.md):
-- - an errand's level score is read from the bravery scores (record 5) with a
--   ten-entry stride, where the level scores (record 7) are read by nothing;
-- - a failed errand that prefers a job rolls a success for each unit sent that
--   holds it, but every pass tests the one unit that the index past the team
--   names.
-- Calls make both at one stop of WLDCORE's world map cursor (the world-map
-- save): party units 1 to 3 become Mediators with Brave 10 and Faith 90, and
-- active errand 0 sends them on a row of category 1 and job list 1. One unit
-- at level 5 and at level 99 scores; then the three, at level 5, fail badly on
-- a row that prefers the Mediator, with the byte past the team naming party
-- unit 0, Ramza. The scenario reads the two scores and whether the success was
-- rolled. The outcome, the row, the errand records and the party units are put
-- back after the last call. On the original the scores are 15 and 15 and
-- nothing is rolled; on the Developer's Cut they are 11 and 60 and the team
-- rolls.
local e = emulator
local BUTTON = e.BUTTON

-- The outcome's bytes, its errand index and first score; the row's bytes, its
-- category, job list and preferred job; an errand record's bytes, its flags
-- (sent), unit count and units; a party unit's bytes, its job, level, Brave
-- and Faith; the Mediator's job and category number.
local OUTCOME_BYTES, OUTCOME_INDEX, OUTCOME_SCORES = 0x80, 0x0c, 0x40
local ROW_BYTES, ROW_CATEGORY, ROW_JOB_LIST, ROW_PREFERRED = 0x40, 0x00, 0x02, 0x28
local RECORD_BYTES, RECORD_FLAGS, SENT, RECORD_COUNT, RECORD_UNITS = 9, 0, 1, 5, 6
local PARTY_BYTES, JOB, LEVEL, BRAVE, FAITH = 0x100, 0x02, 0x16, 0x17, 0x18
local MEDIATOR, MEDIATOR_CATEGORY = 84, 11

local anchor = e.symbol('wldcore', 'wldcore_menu_step_map_cursor_level')
local determine = e.symbol('wldcore', 'wldcore_proposition_determine_success')
local roll = e.symbol('main', 'rand')
local report = e.symbol('main', 'main_system_handle_error')

local values = { calls_returned = 0 }
local saved
local outcome, row, records, party

local function readBytes(at, count)
    local bytes = {}
    for i = 0, count - 1 do bytes[i] = e.read8(at + i) end
    return bytes
end

local function writeBytes(at, bytes, count)
    for i = 0, count - 1 do e.write8(at + i, bytes[i]) end
end

-- The errand: its row and the units it sends, all at one level.
local function send(count, level, preferred)
    e.write16(row + ROW_CATEGORY, 1)
    e.write16(row + ROW_JOB_LIST, 1)
    e.write16(row + ROW_PREFERRED, preferred)
    for i = 0, 2 * RECORD_BYTES - 1 do e.write8(records + i, 0) end
    e.write8(records + RECORD_FLAGS, SENT)
    e.write8(records + RECORD_COUNT, count)
    for i = 1, 3 do
        e.write8(records + RECORD_UNITS + i - 1, i)
        local unit = party + PARTY_BYTES * i
        e.write8(unit + JOB, MEDIATOR)
        e.write8(unit + LEVEL, level)
        e.write8(unit + BRAVE, 10)
        e.write8(unit + FAITH, 90)
    end
    e.write32(outcome + OUTCOME_INDEX, 0)
end

local function returned(result)
    if result.returned then values.calls_returned = values.calls_returned + 1 end
end

local calls = {
    { target = determine, label = 'level 5',
      args = function()
          outcome = e.address('wldcore', 'g_wldcore_proposition_outcome')
          row = e.address('wldcore', 'g_wldcore_selected_proposition_row')
          records = e.address('main', 'g_main_active_propositions')
          party = e.address('main', 'g_main_party_units')
          saved = { outcome = readBytes(outcome, OUTCOME_BYTES), row = readBytes(row, ROW_BYTES),
                    records = readBytes(records, 2 * RECORD_BYTES), party = readBytes(party, 4 * PARTY_BYTES) }
          send(1, 5, 0)
          return {}
      end,
      done = function(result)
          returned(result)
          values.score_level_5 = e.read32(outcome + OUTCOME_SCORES)
      end },
    { target = determine, label = 'level 99',
      args = function() send(1, 99, 0) return {} end,
      done = function(result)
          returned(result)
          values.score_level_99 = e.read32(outcome + OUTCOME_SCORES)
      end },
    { target = determine, label = 'preferred job',
      args = function() send(3, 5, MEDIATOR_CATEGORY) return {} end,
      done = function(result)
          returned(result)
          values.success_rolled = result.counts['roll'] > 0 and 1 or 0
          writeBytes(outcome, saved.outcome, OUTCOME_BYTES)
          writeBytes(row, saved.row, ROW_BYTES)
          writeBytes(records, saved.records, 2 * RECORD_BYTES)
          writeBytes(party, saved.party, 4 * PARTY_BYTES)
      end },
}

return {
    why = 'an errand scores its units\' level from the Brave scores and tests one wrong unit for the preferred job (direct calls: a real errand takes days)',
    save = { file = 'world-map.mcd', sha256 = 'adee3f05a445925002945e98791b6e8a461e5b3bc46ff28842c879f44243b945' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'wldcore', name = 'wldcore_entrypoint', label = 'world map WLDCORE', before = { BUTTON.CIRCLE },
          budget = 6000, shot = 90 },
    }),
    trigger = {
        e.call({ label = 'errand', anchor = anchor, calls = calls,
            guards = { { address = roll.address, limit = 16, label = 'roll' },
                       { address = report.address, limit = 0, label = 'error report' } } }),
    },
    check = function() return values end,
    expect = {
        original = { score_level_5 = 15, score_level_99 = 15, success_rolled = 0, calls_returned = 3 },
        dc = { score_level_5 = 11, score_level_99 = 60, success_rolled = 1, calls_returned = 3 },
    },
}
