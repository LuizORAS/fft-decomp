-- Retail bug of battle_menu_get_unit_skillset_ability_data (QUIRKS.md): with
-- Half of MP the ability menu lists the half rounded up, and
-- battle_menu_build_ability_list grays an ability when the unit's MP minus
-- that listed cost is below 0, but battle_action_check_and_consume_mp and the
-- AI charge the half rounded down: Zodiac lists 50 and costs 49, so a unit
-- with 49 MP sees it grayed. Gariland has no Summoner with Half of MP, so
-- calls make it at one stop of BATTLE's thread_yield, from the AI stop of an
-- Auto-battle turn on Gariland's real route (the new-game save): unit slot 0,
-- marked a monster for the call so every Summon Magic ability counts as
-- learned, lists Summon Magic into the menu's own buffers in the AI's work
-- block, once with Half of MP and once without. The scenario reads the
-- listed MP of Zodiac (99, the odd cost) and Moogle (8, an even one); the
-- unit and the list's part of the work block are put back after the last
-- call. On the original Half of MP lists 50 and 4; on the Developer's Cut 49
-- and 4. Without it both list 99.
local bit = require('bit')
local e = emulator
local BUTTON = e.BUTTON

-- The Auto-battle menu rows; the unit, a unit record's bytes, its unit flags
-- and the monster flag, its second support byte and Half of MP's bit there;
-- Summon Magic, Zodiac and Moogle; the menu list in the AI's work block (its
-- bytes, and the ids, MP, CT, flag and turn columns), its row count and the
-- end of the ids; the offset of thread_yield's second instruction.
local AUTO_BATTLE = { { row = 4, name = 'Auto-battle' }, { menu = 1, row = 3, name = 'Save fading life' } }
local UNIT, STATS_BYTES = 0, 448
local UNIT_FLAGS, MONSTER = 0x006, 0x20
local SUPPORT_2, HALF_MP = 0x090, 0x80
local SUMMON_MAGIC, ZODIAC, MOOGLE = 13, 75, 60
local LIST_BYTES, IDS, MP, CT, FLAGS, TURNS = 0x386, 0x000, 0x1ec, 0x23e, 0x290, 0x2e2
local ROWS, END = 82, 0xffff
local YIELD_SECOND_INSTRUCTION = 4

local anchor = e.symbol('battle', 'thread_yield')
local list = e.symbol('battle', 'battle_menu_get_unit_skillset_ability_data')
local report = e.symbol('main', 'main_system_handle_error')

local values = { calls_returned = 0 }
local saved
local stats, work

local cases = {
    { name = 'half', half = true },
    { name = 'full', half = false },
}

local function readBytes(at, count)
    local bytes = {}
    for i = 0, count - 1 do bytes[i] = e.read8(at + i) end
    return bytes
end

local function writeBytes(at, bytes, count)
    for i = 0, count - 1 do e.write8(at + i, bytes[i]) end
end

-- The MP the list gives an ability, or -1 when it is not listed.
local function listedMp(ability)
    for row = 0, ROWS - 1 do
        local id = e.read16(work + IDS + 2 * row)
        if id == END then return -1 end
        if id == ability then return e.read8(work + MP + row) end
    end
    return -1
end

local calls = {}
for index, case in ipairs(cases) do
    calls[#calls + 1] = { target = list, label = case.name,
        args = function()
            if not saved then
                stats = e.address('battle', 'g_battle_unit_stats') + STATS_BYTES * UNIT
                work = e.read32(e.address('battle', 'g_ai_workspace_ptr'))
                saved = { stats = readBytes(stats, STATS_BYTES), work = readBytes(work, LIST_BYTES) }
            end
            writeBytes(stats, saved.stats, STATS_BYTES)
            e.write8(stats + UNIT_FLAGS, bit.bor(e.read8(stats + UNIT_FLAGS), MONSTER))
            local support = e.read8(stats + SUPPORT_2)
            e.write8(stats + SUPPORT_2, case.half and bit.bor(support, HALF_MP) or bit.band(support, bit.bnot(HALF_MP)))
            for i = 0, ROWS - 1 do e.write16(work + IDS + 2 * i, END) end
            return { UNIT, SUMMON_MAGIC, work + IDS, work + MP, work + CT, 0, work + FLAGS, work + TURNS }
        end,
        done = function(result)
            if result.returned then values.calls_returned = values.calls_returned + 1 end
            values[case.name .. '_zodiac'] = listedMp(ZODIAC)
            if case.half then values.half_moogle = listedMp(MOOGLE) end
            if index == #cases then
                writeBytes(stats, saved.stats, STATS_BYTES)
                writeBytes(work, saved.work, LIST_BYTES)
            end
        end }
end

return {
    why = 'Half of MP lists an odd cost rounded up but charges it rounded down (direct calls: Gariland has no Summoner with Half of MP)',
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
        e.call({ label = 'Summon Magic list', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' },
                       { address = report.address, limit = 0, label = 'error report' } } }),
    },
    check = function() return values end,
    expect = {
        original = { half_zodiac = 50, half_moogle = 4, full_zodiac = 99, calls_returned = 2 },
        dc = { half_zodiac = 49, half_moogle = 4, full_zodiac = 99, calls_returned = 2 },
    },
}
