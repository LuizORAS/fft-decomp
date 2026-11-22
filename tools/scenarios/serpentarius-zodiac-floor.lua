-- Retail bug of battle_formula_apply_zodiac_compatibility (QUIRKS.md): the
-- compatibility leaves XA at least 1 for every sign pair, neutral ones too,
-- but returns for Serpentarius (Elidibus's sign, from job 151) before that
-- floor, so an XA already cut to 0 (Protect and Defense Up on an XA of 1 or
-- 2) stays 0 and the hit deals 0 damage. Calls make it at one stop of
-- BATTLE's thread_yield, from the AI stop of an Auto-battle turn on
-- Gariland's real route (the new-game save): unit slot 0 (sign 0) acts on
-- unit slot 1 with XA 0 against Serpentarius (sign 12), XA 0 against sign 1
-- (a neutral pair) and XA 10 against Serpentarius. The scenario reads XA after
-- each call; both units' zodiac bytes, XA and the two action pointers are put
-- back after the last call. On the original XA is 0, 1 and 10; on the
-- Developer's Cut 1, 1 and 10: Serpentarius reaches the floor and stays
-- neutral.
local bit = require('bit')
local e = emulator
local BUTTON = e.BUTTON

-- The Auto-battle menu rows; the two units, a unit record's bytes and its
-- zodiac byte (the sign in the high nibble); the current ability's XA;
-- Serpentarius; the offset of thread_yield's second instruction.
local AUTO_BATTLE = { { row = 4, name = 'Auto-battle' }, { menu = 1, row = 3, name = 'Save fading life' } }
local ACTOR, TARGET, STATS_BYTES = 0, 1, 448
local ZODIAC_BYTE = 0x009
local XA = 0x0e
local SERPENTARIUS = 12
local YIELD_SECOND_INSTRUCTION = 4

local anchor = e.symbol('battle', 'thread_yield')
local zodiac = e.symbol('battle', 'battle_formula_apply_zodiac_compatibility')
local report = e.symbol('main', 'main_system_handle_error')

local values = { calls_returned = 0 }
local saved
local units, ability, pointers

local cases = {
    { name = 'serpentarius_xa_0', sign = SERPENTARIUS, xa = 0 },
    { name = 'neutral_xa_0', sign = 1, xa = 0 },
    { name = 'serpentarius_xa_10', sign = SERPENTARIUS, xa = 10 },
}

local function unit(slot) return units + STATS_BYTES * slot end

local function setSign(at, sign)
    e.write8(at + ZODIAC_BYTE, bit.bor(bit.band(e.read8(at + ZODIAC_BYTE), 0x0f), bit.lshift(sign, 4)))
end

local calls = {}
for index, case in ipairs(cases) do
    calls[#calls + 1] = { target = zodiac, label = case.name,
        args = function()
            if not saved then
                units = e.address('battle', 'g_battle_unit_stats')
                ability = e.address('battle', 'g_battle_action_current_ability')
                pointers = { e.address('battle', 'g_battle_action_attacker'), e.address('battle', 'g_battle_action_target') }
                saved = { actor = e.read8(unit(ACTOR) + ZODIAC_BYTE), target = e.read8(unit(TARGET) + ZODIAC_BYTE),
                          xa = e.read16(ability + XA), pointers = { e.read32(pointers[1]), e.read32(pointers[2]) } }
            end
            setSign(unit(ACTOR), 0)
            setSign(unit(TARGET), case.sign)
            e.write16(ability + XA, case.xa)
            e.write32(pointers[1], unit(ACTOR))
            e.write32(pointers[2], unit(TARGET))
            return {}
        end,
        done = function(result)
            if result.returned then values.calls_returned = values.calls_returned + 1 end
            values[case.name] = e.signed16(e.read16(ability + XA))
            if index == #cases then
                e.write8(unit(ACTOR) + ZODIAC_BYTE, saved.actor)
                e.write8(unit(TARGET) + ZODIAC_BYTE, saved.target)
                e.write16(ability + XA, saved.xa)
                for i, at in ipairs(pointers) do e.write32(at, saved.pointers[i]) end
            end
        end }
end

return {
    why = 'Serpentarius skips the zodiac compatibility\'s floor of 1 (direct calls: Elidibus alone has the sign)',
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
        e.call({ label = 'zodiac floor', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' },
                       { address = report.address, limit = 0, label = 'error report' } } }),
    },
    check = function() return values end,
    expect = {
        original = { serpentarius_xa_0 = 0, neutral_xa_0 = 1, serpentarius_xa_10 = 10, calls_returned = 3 },
        dc = { serpentarius_xa_0 = 1, neutral_xa_0 = 1, serpentarius_xa_10 = 10, calls_returned = 3 },
    },
}
