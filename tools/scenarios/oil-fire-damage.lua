-- Retail bug of battle_formula_apply_ability_element (QUIRKS.md): Oil against
-- a fire ability doubles XA, but every caller has already stored XA * YA as
-- the HP damage, so Oil never raises the damage; it is still marked for
-- removal. Gariland has no Oil, so calls make it at one stop of BATTLE's
-- thread_yield, from the AI stop of an Auto-battle turn on Gariland's real
-- route (the new-game save): battle_formula_calculate_elemental_xa_times_ya,
-- the step of the magic formulas that stores XA * YA and applies the weather
-- and the element, runs with XA 10, YA 5 and a fire element on unit slot 0
-- (the action's target and its result record), without Oil, with Oil, and
-- with Oil and a fire weakness. The weather reads clear while the calls run.
-- The scenario reads the stored HP damage and whether Oil is marked for
-- removal; the unit, its result record, the current ability and the two
-- action pointers are put back after the last call. On the original the
-- damage is 50, 50 and 100; on the Developer's Cut 50, 100 and 200, with Oil
-- marked for removal in both cases that have it.
local bit = require('bit')
local e = emulator
local BUTTON = e.BUTTON

-- The Auto-battle menu rows; the unit, a unit record's bytes, its current
-- Oil byte and bit and the Oil byte of the statuses inflicted on it (an
-- inflicted status sets both; a removal keeps only these), its elemental
-- affinities (absorb, null, half, weak,
-- strengthen) and its result record (bytes, hit, HP damage, Oil's removal
-- byte); the current ability's bytes, XA, YA and element; fire; the register
-- v0; the offset of thread_yield's second instruction.
local AUTO_BATTLE = { { row = 4, name = 'Auto-battle' }, { menu = 1, row = 3, name = 'Save fading life' } }
local UNIT, STATS_BYTES = 0, 448
local CURRENT_OIL, OIL_BIT, INFLICTED_OIL = 0x05a, 0x80, 0x1bd
local AFFINITIES, AFFINITY_COUNT, WEAK = 0x06d, 5, 3
local ACTION, ACTION_BYTES, HIT, HP_DAMAGE, REMOVAL_OIL = 0x18c, 0x2c, 0x00, 0x04, 0x22
local ABILITY_BYTES, XA, YA, ELEMENT = 80, 0x0e, 0x10, 0x37
local FIRE = 0x80
local V0 = 2
local YIELD_SECOND_INSTRUCTION = 4

local anchor = e.symbol('battle', 'thread_yield')
local formula = e.symbol('battle', 'battle_formula_calculate_elemental_xa_times_ya')
local weather = e.symbol('battle', 'battle_map_get_effective_weather')
local report = e.symbol('main', 'main_system_handle_error')

local values = { calls_returned = 0 }
local saved
local stats, ability, targetPointer, dataPointer
local active, hooked = false, false

local cases = {
    { name = 'no_oil', oil = false, weak = false },
    { name = 'oil', oil = true, weak = false },
    { name = 'oil_weak', oil = true, weak = true },
}

local function readBytes(at, count)
    local bytes = {}
    for i = 0, count - 1 do bytes[i] = e.read8(at + i) end
    return bytes
end

local function writeBytes(at, bytes, count)
    for i = 0, count - 1 do e.write8(at + i, bytes[i]) end
end

local function save()
    stats = e.address('battle', 'g_battle_unit_stats') + STATS_BYTES * UNIT
    ability = e.address('battle', 'g_battle_action_current_ability')
    targetPointer = e.address('battle', 'g_battle_action_target')
    dataPointer = e.address('battle', 'g_battle_action_target_data')
    saved = { stats = readBytes(stats, STATS_BYTES), ability = readBytes(ability, ABILITY_BYTES),
              target = e.read32(targetPointer), data = e.read32(dataPointer) }
    -- The weather reads clear while a call runs.
    e.breakAt(weather.address, 'weather', function()
        if not active then return true end
        e.setGPR(V0, 0)
        local regs = PCSX.getRegisters()
        regs.pc = regs.GPR.n.ra
        return true
    end)
    hooked = true
end

local calls = {}
for index, case in ipairs(cases) do
    calls[#calls + 1] = { target = formula, label = case.name,
        args = function()
            if not hooked then save() end
            for _, at in ipairs({ CURRENT_OIL, INFLICTED_OIL }) do
                local oil = e.read8(stats + at)
                e.write8(stats + at, case.oil and bit.bor(oil, OIL_BIT) or bit.band(oil, bit.bnot(OIL_BIT)))
            end
            for i = 0, AFFINITY_COUNT - 1 do e.write8(stats + AFFINITIES + i, 0) end
            if case.weak then e.write8(stats + AFFINITIES + WEAK, FIRE) end
            for i = 0, ACTION_BYTES - 1 do e.write8(stats + ACTION + i, 0) end
            e.write8(stats + ACTION + HIT, 1)
            e.write16(ability + XA, 10)
            e.write16(ability + YA, 5)
            e.write8(ability + ELEMENT, FIRE)
            e.write32(targetPointer, stats)
            e.write32(dataPointer, stats + ACTION)
            active = true
            return {}
        end,
        done = function(result)
            active = false
            if result.returned then values.calls_returned = values.calls_returned + 1 end
            local removed = bit.band(e.read8(stats + ACTION + REMOVAL_OIL), OIL_BIT) ~= 0 and 1 or 0
            values[case.name] = string.format('%d %d', e.signed16(e.read16(stats + ACTION + HP_DAMAGE)), removed)
            if index == #cases then
                writeBytes(stats, saved.stats, STATS_BYTES)
                writeBytes(ability, saved.ability, ABILITY_BYTES)
                e.write32(targetPointer, saved.target)
                e.write32(dataPointer, saved.data)
            end
        end }
end

return {
    why = 'Oil never doubles a fire ability\'s damage (direct calls: Gariland has no Oil)',
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
        e.call({ label = 'fire on Oil', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' },
                       { address = report.address, limit = 0, label = 'error report' } } }),
    },
    check = function() return values end,
    expect = {
        original = { no_oil = '50 0', oil = '50 1', oil_weak = '100 1', calls_returned = 3 },
        dc = { no_oil = '50 0', oil = '100 1', oil_weak = '200 1', calls_returned = 3 },
    },
}
