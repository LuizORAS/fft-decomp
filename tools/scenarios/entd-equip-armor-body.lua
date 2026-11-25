-- Retail bug of main_unit_calculate_entd_equipment (QUIRKS.md): an ENTD unit's
-- random helmet prefers the helmet type under Equip Armor, but its random body
-- prefers the armor type under Equip Shield. No ENTD record fixes either
-- support and Gariland's units roll their own, so calls make it at one stop of
-- BATTLE's thread_yield at Gariland's deployment screen (the new-game save):
-- unit slot 0 becomes a level 20 Knight with no support, with Equip Armor and
-- with Equip Shield, and takes the equipment of an ENTD record that rolls only
-- its body. The scenario reads the item type the body roll asks for first (the
-- armor type or any) and, when it asks for armor, the item worn, which at
-- level 20 can only be the Gold Armor. The unit and the slot that holds the
-- record are put back after the last call. On the original Equip Shield asks
-- for armor; on the Developer's Cut Equip Armor does.
local bit = require('bit')
local e = emulator
local BUTTON = e.BUTTON

-- The unit, the slot whose bytes hold the ENTD record, a unit record's bytes,
-- its unit flags (male), level, body, equippable types and support byte 1 (with
-- byte 3, emptied); a job record's bytes, its equippable types and the Knight;
-- an ENTD record's bytes, its body and the random value; the support bits, the
-- types Equip Armor adds; the body item class, the armor type, any type and
-- the Gold Armor; the offset of thread_yield's second instruction.
local UNIT, RECORD_UNIT, STATS_BYTES = 0, 1, 448
local UNIT_FLAGS, MALE, LEVEL, BODY, CATEGORIES, SUPPORT_1, SUPPORT_3 = 0x006, 0x80, 0x022, 0x01b, 0x04a, 0x08f, 0x091
local JOB_BYTES, JOB_CATEGORIES, KNIGHT = 48, 9, 76
local ENTD_BYTES, ENTD_BODY, RANDOM = 40, 0x13, 0xfe
local EQUIP_ARMOR, EQUIP_SHIELD, ARMOR_TYPES = 0x80, 0x40, 0x09
local BODY_CLASS, TYPE_ARMOR, TYPE_ANY, GOLD_ARMOR = 0x10, 23, 255, 178
local YIELD_SECOND_INSTRUCTION = 4

local anchor = e.symbol('battle', 'thread_yield')
local equip = e.symbol('main', 'main_unit_calculate_entd_equipment')
local pick = e.symbol('main', 'main_unit_calculate_random_equipment')
local report = e.symbol('main', 'main_system_handle_error')

local values = { calls_returned = 0 }
local saved
local stats, record, asked

local function readBytes(at, count)
    local bytes = {}
    for i = 0, count - 1 do bytes[i] = e.read8(at + i) end
    return bytes
end

local function writeBytes(at, bytes, count)
    for i = 0, count - 1 do e.write8(at + i, bytes[i]) end
end

-- The type of the first body roll of the call.
local function watchPick()
    e.breakAt(pick.address, 'random equipment', function()
        local regs = PCSX.getRegisters()
        if not asked and tonumber(regs.GPR.n.a1) % 256 == BODY_CLASS then asked = tonumber(regs.GPR.n.a3) % 256 end
        return true
    end)
end

local cases = {
    { name = 'none', support = 0, types = 0 },
    { name = 'equip_armor', support = EQUIP_ARMOR, types = ARMOR_TYPES },
    { name = 'equip_shield', support = EQUIP_SHIELD, types = 0 },
}

local calls = {}
for index, case in ipairs(cases) do
    calls[#calls + 1] = { target = equip, label = case.name,
        args = function()
            if index == 1 then
                stats = e.address('battle', 'g_battle_unit_stats') + STATS_BYTES * UNIT
                record = e.address('battle', 'g_battle_unit_stats') + STATS_BYTES * RECORD_UNIT
                saved = { stats = readBytes(stats, STATS_BYTES), record = readBytes(record, ENTD_BYTES) }
                watchPick()
            end
            asked = nil
            local job = e.address('main', 'g_main_job_data') + JOB_BYTES * KNIGHT + JOB_CATEGORIES
            for i = 0, 3 do e.write8(stats + CATEGORIES + i, e.read8(job + i)) end
            e.write8(stats + CATEGORIES + 2, bit.bor(e.read8(stats + CATEGORIES + 2), case.types))
            e.write8(stats + UNIT_FLAGS, MALE)
            e.write8(stats + LEVEL, 20)
            e.write8(stats + SUPPORT_1, case.support)
            e.write8(stats + SUPPORT_3, 0)
            for i = 0, ENTD_BYTES - 1 do e.write8(record + i, 0) end
            e.write8(record + ENTD_BODY, RANDOM)
            return { stats, record }
        end,
        done = function(result)
            if result.returned then values.calls_returned = values.calls_returned + 1 end
            values[case.name .. '_asks_armor'] = asked == TYPE_ARMOR and 1 or asked == TYPE_ANY and 0 or -1
            if asked == TYPE_ARMOR then
                values[case.name .. '_wears_gold_armor'] = e.read8(stats + BODY) == GOLD_ARMOR and 1 or 0
            end
            if index == #cases then
                writeBytes(stats, saved.stats, STATS_BYTES)
                writeBytes(record, saved.record, ENTD_BYTES)
            end
        end }
end

return {
    why = 'an ENTD unit\'s random body prefers armor under Equip Shield, not Equip Armor (direct calls: Gariland rolls its own supports)',
    save = { file = 'smoke-new-game.mcd', sha256 = '02203c586fc996bd3c3dfa1fb07bb6e0025cb590b69b59dae955fc3b098c47e3' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'event-attack', name = 'attack_deploy_run_screen', label = 'overlay menu ATTACK.OUT deployment',
          before = { BUTTON.CIRCLE }, budget = 9000, shot = 90 },
    }),
    trigger = {
        e.call({ label = 'ENTD equipment', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' },
                       { address = report.address, limit = 0, label = 'error report' } } }),
    },
    check = function() return values end,
    expect = {
        original = { none_asks_armor = 0, equip_armor_asks_armor = 0, equip_shield_asks_armor = 1,
            equip_shield_wears_gold_armor = 1, calls_returned = 3 },
        dc = { none_asks_armor = 0, equip_armor_asks_armor = 1, equip_armor_wears_gold_armor = 1,
            equip_shield_asks_armor = 0, calls_returned = 3 },
    },
}
