-- Retail bug of battle_ai_simulate_action_and_score and
-- battle_ai_evaluate_status_cancellation (QUIRKS.md): Protect allies tries to
-- remove an ally's Blood Suck only when the AI marks the ally with decision
-- bit 0x04, which the refresh builds from Jump, so the search never runs for
-- an ally that has Blood Suck and is not jumping. Gariland has no Blood Suck,
-- so the status is put in RAM on the real route (the new-game save): from the
-- AI stop of an Auto-battle turn, when the next decision refreshes its flags
-- (battle_ai_refresh_unit_decision_flags), the first present ally of the
-- acting unit gets Blood Suck, and after the refresh the scenario reads the
-- ally's bit and puts its status back. A direct call then asks
-- battle_ai_evaluate_status_cancellation to remove that ally's Blood Suck, as
-- Protect allies does, with the ranking empty as the decision leaves it; the
-- AI workspace is put back after it. On the original the bit is 0 and the
-- call returns 1 (go on) without looking; on the Developer's Cut the bit is 1
-- and the call looks for a cure, finds none in Gariland's party and returns 0
-- (Protect allies then turns to the ally most in peril).
local bit = require('bit')
local e = emulator
local BUTTON = e.BUTTON

-- The Auto-battle menu rows; a unit record's bytes, its ENTD slot (0xff when
-- absent), its Jump and Blood Suck status bytes and bits and its team; the
-- AI workspace's bytes, acting unit, decision records, first ranked action's
-- hit chance and resume flag; the decision bit; Blood Suck's status id; the
-- offset of thread_yield's second instruction.
local AUTO_BATTLE = { { row = 4, name = 'Auto-battle' }, { menu = 1, row = 3, name = 'Save fading life' } }
local STATS_BYTES, UNIT_SLOTS = 448, 21
local ENTD_SLOT, ABSENT = 0x001, 0xff
local CURRENT_JUMP, CURRENT_BLOOD_SUCK, STATUS_BIT = 0x058, 0x059, 0x04
local INITIAL_TEAM, TEAM_MASK = 0x1ba, 0x30
local WORKSPACE_BYTES = 0x19c8
local ACTING_UNIT_ID, DECISIONS, DECISION_BYTES, DECISION_FLAGS = 0x0e2e, 0x182c, 16, 0x04
local RANKED_HIT_PERCENT, RESUME_PENDING = 0x0054 + 0x1d, 0x19b7
local DECISION_BIT = 0x04
local BLOOD_SUCK = 13
local YIELD_SECOND_INSTRUCTION = 4

local anchor = e.symbol('battle', 'thread_yield')
local refresh = e.symbol('battle', 'battle_ai_refresh_unit_decision_flags')
local cancellation = e.symbol('battle', 'battle_ai_evaluate_status_cancellation')
local search = e.symbol('battle', 'battle_ai_select_active_units')
local report = e.symbol('main', 'main_system_handle_error')

local values = {}
local ally, ai, units

local function unit(slot) return units + STATS_BYTES * slot end

local step = { label = 'refresh with Blood Suck', before = { BUTTON.CIRCLE }, budget = 12000, start = function(passed)
    ai = e.address('battle', 'g_battle_ai_data_base')
    units = e.address('battle', 'g_battle_unit_stats')
    local saved
    e.breakAt(refresh.address, 'refresh', function()
        if not e.resident(refresh) then return true end
        local acting = e.read8(ai + ACTING_UNIT_ID)
        local team = bit.band(e.read8(unit(acting) + INITIAL_TEAM), TEAM_MASK)
        for slot = 0, UNIT_SLOTS - 1 do
            if slot ~= acting and e.read8(unit(slot) + ENTD_SLOT) ~= ABSENT
                and bit.band(e.read8(unit(slot) + INITIAL_TEAM), TEAM_MASK) == team then
                ally = slot
                break
            end
        end
        if not ally then return true end
        values.ally_jumping = bit.band(e.read8(unit(ally) + CURRENT_JUMP), STATUS_BIT)
        saved = e.read8(unit(ally) + CURRENT_BLOOD_SUCK)
        e.write8(unit(ally) + CURRENT_BLOOD_SUCK, bit.bor(saved, STATUS_BIT))
        e.breakAt(tonumber(PCSX.getRegisters().GPR.n.ra), 'after refresh', function()
            -- A suspended refresh (-1) returns here again when it resumes.
            if tonumber(PCSX.getRegisters().GPR.n.v0) ~= 0 then return true end
            local flags = e.read8(ai + DECISIONS + DECISION_BYTES * ally + DECISION_FLAGS)
            values.decision_bit = bit.band(flags, DECISION_BIT) ~= 0 and 1 or 0
            e.write8(unit(ally) + CURRENT_BLOOD_SUCK, saved)
            passed()
            return false
        end)
        return false
    end)
end }

local saved = {}
local calling = false
local call = { target = cancellation, label = 'cancel Blood Suck',
    args = function()
        for i = 0, WORKSPACE_BYTES - 1 do saved[i] = e.read8(ai + i) end
        e.write8(ai + RESUME_PENDING, 0)
        e.write8(ai + RANKED_HIT_PERCENT, 0)
        values.searched = 0
        calling = true
        e.breakAt(search.address, 'search', function()
            if calling then values.searched = values.searched + 1 end
            return calling
        end)
        return { ally, BLOOD_SUCK }
    end,
    done = function(result)
        calling = false
        values.result = result.returned and result.v0 or 'none'
        for i = 0, WORKSPACE_BYTES - 1 do e.write8(ai + i, saved[i]) end
    end }

return {
    why = 'Protect allies tests an ally\'s Blood Suck with the bit built from Jump (real route, Blood Suck put in RAM '
        .. 'as Gariland has none; then a direct call of the cancellation)',
    save = { file = 'smoke-new-game.mcd', sha256 = '02203c586fc996bd3c3dfa1fb07bb6e0025cb590b69b59dae955fc3b098c47e3' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'event-attack', name = 'attack_deploy_run_screen', label = 'overlay menu ATTACK.OUT deployment',
          before = { BUTTON.CIRCLE }, budget = 9000, shot = 90 },
        { module = 'battle', name = 'battle_ai_load_ability_entry', label = 'the AI considers an ability',
          before = { BUTTON.CIRCLE }, budget = 40000, shot = 1, start = function()
              e.startDeployment()
              e.playTurns(AUTO_BATTLE)
          end },
    }),
    trigger = {
        step,
        e.call({ label = 'status cancellation', anchor = anchor, calls = { call },
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' },
                       { address = report.address, limit = 0, label = 'error report' } } }),
    },
    check = function()
        values.ally = ally or 'none'
        return values
    end,
    expect = {
        original = { ally = 16, ally_jumping = 0, decision_bit = 0, searched = 0, result = 1 },
        dc = { ally = 16, ally_jumping = 0, decision_bit = 1, searched = 1, result = 0 },
    },
}
