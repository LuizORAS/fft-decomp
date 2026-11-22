-- Retail bug of battle_formula_weapon_heal (QUIRKS.md): the Healing Staff's
-- formula applies the zodiac compatibility itself and again inside
-- battle_formula_apply_physical_xa_modifiers, so a compatible pair heals
-- more and an incompatible one less than any other weapon strike would give.
-- Calls make it at one stop of BATTLE's thread_yield, from the AI stop of an
-- Auto-battle turn on Gariland's real route (the new-game save): unit slot 0
-- (male, sign 0, MA 10) heals unit slot 1 with the Healing Staff (power 4),
-- with no supports, statuses or charge on either unit, five times, the target's
-- sign and sex giving each compatibility: sign 0 neutral, sign 4 good, sign 6
-- and female best, sign 3 bad, sign 6 and male worst. The scenario reads the
-- HP healing; both units, the current ability and the three action pointers
-- are put back after the last call. On the original the healing is 40, 60,
-- 88, 24 and 12; on the Developer's Cut, which applies the compatibility once,
-- 40, 48, 60, 32 and 20.
local bit = require('bit')
local e = emulator
local BUTTON = e.BUTTON

-- The Auto-battle menu rows; the two units, a unit record's bytes, its sex
-- and monster flags, its zodiac byte (the sign in the high nibble), its MA,
-- its current statuses, its reaction, support and movement flags, and its
-- result record (HP healing); the current ability's bytes, weapon id, two
-- hands, charge power, weapon flags and power; the Healing Staff; the
-- offset of thread_yield's second instruction.
local AUTO_BATTLE = { { row = 4, name = 'Auto-battle' }, { menu = 1, row = 3, name = 'Save fading life' } }
local HEALER, TARGET, STATS_BYTES = 0, 1, 448
local UNIT_FLAGS, MALE, FEMALE, MONSTER = 0x006, 0x80, 0x40, 0x20
local ZODIAC_BYTE, MA = 0x009, 0x037
local CURRENT, STATUS_BYTES = 0x058, 5
local ABILITY_FLAGS, ABILITY_FLAG_BYTES = 0x08b, 11
local ACTION, HP_HEALING = 0x18c, 0x06
local ABILITY_BYTES, WEAPON_ID, TWO_HANDS, CHARGE, WEAPON_FLAGS, WEAPON_POWER = 80, 0x18, 0x19, 0x24, 0x3f, 0x42
local HEALING_STAFF = 0x3d
local YIELD_SECOND_INSTRUCTION = 4

local anchor = e.symbol('battle', 'thread_yield')
local heal = e.symbol('battle', 'battle_formula_weapon_heal')
local report = e.symbol('main', 'main_system_handle_error')

local values = { calls_returned = 0 }
local saved
local units, ability, pointers

local cases = {
    { name = 'neutral', sign = 0, sex = MALE },
    { name = 'good', sign = 4, sex = MALE },
    { name = 'best', sign = 6, sex = FEMALE },
    { name = 'bad', sign = 3, sex = MALE },
    { name = 'worst', sign = 6, sex = MALE },
}

local function unit(slot) return units + STATS_BYTES * slot end

local function readBytes(at, count)
    local bytes = {}
    for i = 0, count - 1 do bytes[i] = e.read8(at + i) end
    return bytes
end

local function writeBytes(at, bytes, count)
    for i = 0, count - 1 do e.write8(at + i, bytes[i]) end
end

local function setUnit(at, sign, sex)
    e.write8(at + UNIT_FLAGS, bit.bor(bit.band(e.read8(at + UNIT_FLAGS), bit.bnot(MALE + FEMALE + MONSTER)), sex))
    e.write8(at + ZODIAC_BYTE, bit.bor(bit.band(e.read8(at + ZODIAC_BYTE), 0x0f), bit.lshift(sign, 4)))
    for i = 0, STATUS_BYTES - 1 do e.write8(at + CURRENT + i, 0) end
    for i = 0, ABILITY_FLAG_BYTES - 1 do e.write8(at + ABILITY_FLAGS + i, 0) end
end

local function save()
    units = e.address('battle', 'g_battle_unit_stats')
    ability = e.address('battle', 'g_battle_action_current_ability')
    pointers = { e.address('battle', 'g_battle_action_attacker'), e.address('battle', 'g_battle_action_target'),
                 e.address('battle', 'g_battle_action_target_data') }
    saved = { healer = readBytes(unit(HEALER), STATS_BYTES), target = readBytes(unit(TARGET), STATS_BYTES),
              ability = readBytes(ability, ABILITY_BYTES), pointers = {} }
    for i, at in ipairs(pointers) do saved.pointers[i] = e.read32(at) end
end

local calls = {}
for index, case in ipairs(cases) do
    calls[#calls + 1] = { target = heal, label = case.name,
        args = function()
            if not saved then save() end
            setUnit(unit(HEALER), 0, MALE)
            e.write8(unit(HEALER) + MA, 10)
            setUnit(unit(TARGET), case.sign, case.sex)
            for i = 0, 0x2b do e.write8(unit(TARGET) + ACTION + i, 0) end
            e.write8(unit(TARGET) + ACTION, 1)
            e.write8(ability + WEAPON_ID, HEALING_STAFF)
            e.write8(ability + TWO_HANDS, 0)
            e.write8(ability + CHARGE, 0)
            e.write8(ability + WEAPON_FLAGS, 0)
            e.write8(ability + WEAPON_POWER, 4)
            e.write32(pointers[1], unit(HEALER))
            e.write32(pointers[2], unit(TARGET))
            e.write32(pointers[3], unit(TARGET) + ACTION)
            return {}
        end,
        done = function(result)
            if result.returned then values.calls_returned = values.calls_returned + 1 end
            values[case.name] = e.signed16(e.read16(unit(TARGET) + ACTION + HP_HEALING))
            if index == #cases then
                writeBytes(unit(HEALER), saved.healer, STATS_BYTES)
                writeBytes(unit(TARGET), saved.target, STATS_BYTES)
                writeBytes(ability, saved.ability, ABILITY_BYTES)
                for i, at in ipairs(pointers) do e.write32(at, saved.pointers[i]) end
            end
        end }
end

return {
    why = 'the Healing Staff applies the zodiac compatibility twice (direct calls of the formula)',
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
        e.call({ label = 'Healing Staff', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' },
                       { address = report.address, limit = 0, label = 'error report' } } }),
    },
    check = function() return values end,
    expect = {
        original = { neutral = 40, good = 60, best = 88, bad = 24, worst = 12, calls_returned = 5 },
        dc = { neutral = 40, good = 48, best = 60, bad = 32, worst = 20, calls_returned = 5 },
    },
}
