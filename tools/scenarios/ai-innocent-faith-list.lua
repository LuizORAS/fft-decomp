-- Retail bug of battle_ai_add_usable_skillset_abilities (QUIRKS.md): for an
-- innately Innocent unit the AI means to leave out the abilities that depend
-- on Faith, but it tests the Faith flag of the ability it considered last,
-- not of the one it adds, so it keeps or drops all of a skillset's abilities
-- at once. Only the Steel Giant job is innately Innocent and Gariland has
-- none, so calls make it at one stop of BATTLE's thread_yield, from the AI
-- stop of an Auto-battle turn on Gariland's real route (the new-game save):
-- unit slot 0 gets innate Innocent and enough MP, and the AI builds its list
-- for Celia's Use Hand skillset (Seal, Shadow Stitch, Stop Bracelet, Ultima,
-- Allure; only Ultima depends on Faith) after an ability that depends on
-- Faith and after one that does not, then once more without Innocent. The
-- unit's bytes and the AI workspace are put back after the last call. On the
-- original the Innocent unit gets none of the five after a Faith ability and
-- all five after the other; on the Developer's Cut it gets the four without
-- Ultima both times. Without Innocent it gets all five on both discs.
local bit = require('bit')
local e = emulator
local BUTTON = e.BUTTON

-- The Auto-battle menu rows; the unit, a unit record's bytes, its max MP and
-- innate status byte 4 with Innocent's bit; the AI workspace's bytes, the
-- considered ability's AI flag word and Faith's bit in it, the ability
-- lists, an entry's and a list's bytes, the compact unit indices and an
-- entry's ability id bits; Use Hand (Celia); the offset of thread_yield's
-- second instruction.
local AUTO_BATTLE = { { row = 4, name = 'Auto-battle' }, { menu = 1, row = 3, name = 'Save fading life' } }
local UNIT, STATS_BYTES = 0, 448
local MAX_MP, INNATE_4, INNOCENT = 0x02e, 0x052, 0x40
local WORKSPACE_BYTES = 0x19c8
local CONSIDERED_AI_FLAGS, AFFECTED_BY_FAITH = 0x0c, 0x400
local LISTS, ENTRY_BYTES, LIST_BYTES, COMPACT = 0x0ef0, 4, 0x88, 0x198c
local ABILITY_ID_MASK = 0x3ff
local USE_HAND_CELIA = 54
local YIELD_SECOND_INSTRUCTION = 4

local anchor = e.symbol('battle', 'thread_yield')
local add = e.symbol('battle', 'battle_ai_add_usable_skillset_abilities')
local report = e.symbol('main', 'main_system_handle_error')

local values = { calls_returned = 0 }
local saved = {}
local ai, stats

local cases = {
    { name = 'innocent_after_faith', innocent = true, flags = AFFECTED_BY_FAITH },
    { name = 'innocent_after_other', innocent = true, flags = 0 },
    { name = 'plain_after_faith', innocent = false, flags = AFFECTED_BY_FAITH },
}

local calls = {}
for index, case in ipairs(cases) do
    calls[#calls + 1] = { target = add, label = case.name,
        args = function()
            if index == 1 then
                ai = e.address('battle', 'g_battle_ai_data_base')
                stats = e.address('battle', 'g_battle_unit_stats') + STATS_BYTES * UNIT
                saved.workspace = {}
                for i = 0, WORKSPACE_BYTES - 1 do saved.workspace[i] = e.read8(ai + i) end
                saved.max_mp = e.read16(stats + MAX_MP)
                saved.innate = e.read8(stats + INNATE_4)
            end
            e.write16(stats + MAX_MP, 99)
            local innate = bit.band(saved.innate, bit.bnot(INNOCENT))
            e.write8(stats + INNATE_4, case.innocent and bit.bor(innate, INNOCENT) or innate)
            e.write32(ai + CONSIDERED_AI_FLAGS, case.flags)
            return { UNIT, USE_HAND_CELIA, 0 }
        end,
        done = function(result)
            local list = ai + LISTS + LIST_BYTES * e.read8(ai + COMPACT + UNIT)
            local ids = {}
            if result.returned then
                values.calls_returned = values.calls_returned + 1
                for i = 0, result.v0 - 1 do
                    ids[#ids + 1] = string.format('%03x', bit.band(e.read16(list + ENTRY_BYTES * i), ABILITY_ID_MASK))
                end
            end
            values[case.name] = #ids > 0 and table.concat(ids, ' ') or 'none'
            if index == #cases then
                for i = 0, WORKSPACE_BYTES - 1 do e.write8(ai + i, saved.workspace[i]) end
                e.write16(stats + MAX_MP, saved.max_mp)
                e.write8(stats + INNATE_4, saved.innate)
            end
        end }
end

return {
    why = 'the AI drops or keeps an Innocent unit\'s abilities by the Faith flag of the ability it considered last '
        .. '(direct calls: only the Steel Giant is innately Innocent, and Gariland has none)',
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
        e.call({ label = 'add Use Hand', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' },
                       { address = report.address, limit = 0, label = 'error report' } } }),
    },
    check = function() return values end,
    expect = {
        original = { innocent_after_faith = 'none', innocent_after_other = '0b5 0b6 0b7 09a 0c9',
                     plain_after_faith = '0b5 0b6 0b7 09a 0c9', calls_returned = 3 },
        dc = { innocent_after_faith = '0b5 0b6 0b7 0c9', innocent_after_other = '0b5 0b6 0b7 0c9',
               plain_after_faith = '0b5 0b6 0b7 09a 0c9', calls_returned = 3 },
    },
}
