-- Retail bugs of battle_ai_load_ability_entry (QUIRKS.md):
-- - the AI's charge-time estimate halves the CT under Monster Skill and zeroes
--   it under Defend, the support bits beside Short Charge and Non-Charge, so a
--   unit with Short Charge or Non-Charge is planned at the full CT;
-- - with Throw Item an item's range is the unit's Move, where the real
--   targeting gives 4.
-- The estimate decides which units move before a charge lands and which
-- statuses last until it does; the range decides where the AI plans to throw
-- from. Calls make both at one stop of BATTLE's thread_yield at Gariland's
-- deployment screen (the new-game save), for ENTD slot 0: Fire 2 (CT 5) with
-- no CT support, Short Charge, Non-Charge, both, Monster Skill and Defend in
-- support byte 4; then a Potion with no Throw Item, and with Throw Item at Move
-- 3 and Move 6. Each call reads the considered ability's CT or range. The AI
-- workspace and the unit's support bytes and Move are put back after the last
-- call. On the original the CT is 5, 5, 5, 5, 2, 0 and the range follows Move;
-- on the Developer's Cut it is what the game charges (5, 3, 0, 0, 5, 5: Short
-- Charge rounds up, Non-Charge wins) and the range is 4 with Throw Item.
local e = emulator
local BUTTON = e.BUTTON

-- The unit, a unit record's bytes and its Move and support bytes 3 and 4, the
-- AI workspace's bytes, the considered ability's CT and range, a free entry of
-- the last ability list, the skillsets and abilities, the support bits, and
-- the offset of thread_yield's second instruction.
local UNIT = 0
local STATS_BYTES = 448
local MOVE, SUPPORT_3, SUPPORT_4 = 0x03a, 0x091, 0x092
local WORKSPACE_BYTES = 0x19c8
local CONSIDERED_CT, CONSIDERED_RANGE = 0x01, 0x04
local SCRATCH_ENTRY = 0x0ef0 + 15 * 0x88 + 33 * 4
local SKILLSET_BLACK_MAGIC, SKILLSET_ITEM = 11, 6
local FIRE_2, POTION = 0x11, 0x170
local THROW_ITEM = 0x08
local NON_CHARGE, SHORT_CHARGE, DEFEND, MONSTER_SKILL = 0x04, 0x08, 0x40, 0x80
local YIELD_SECOND_INSTRUCTION = 4

local anchor = e.symbol('battle', 'thread_yield')
local load = e.symbol('battle', 'battle_ai_load_ability_entry')
local report = e.symbol('main', 'main_system_handle_error')

local values = { calls_returned = 0 }
local saved = {}
local ai, stats

-- An ability list entry for the unit: its packed id, skillset and usage.
local function entry(skillset, ability)
    e.write16(ai + SCRATCH_ENTRY, UNIT * 1024 + ability)
    e.write8(ai + SCRATCH_ENTRY + 2, skillset)
    e.write8(ai + SCRATCH_ENTRY + 3, 0)
    return { ai + SCRATCH_ENTRY }
end

local cases = {
    { name = 'ct_none', support4 = 0 },
    { name = 'ct_short_charge', support4 = SHORT_CHARGE },
    { name = 'ct_non_charge', support4 = NON_CHARGE },
    { name = 'ct_short_and_non_charge', support4 = SHORT_CHARGE + NON_CHARGE },
    { name = 'ct_monster_skill', support4 = MONSTER_SKILL },
    { name = 'ct_defend', support4 = DEFEND },
    { name = 'range_potion', item = true, support3 = 0, move = 3 },
    { name = 'range_throw_item_move_3', item = true, support3 = THROW_ITEM, move = 3 },
    { name = 'range_throw_item_move_6', item = true, support3 = THROW_ITEM, move = 6 },
}

local calls = {}
for index, case in ipairs(cases) do
    calls[#calls + 1] = { target = load, label = case.name,
        args = function()
            if index == 1 then
                ai = e.address('battle', 'g_battle_ai_data_base')
                stats = e.address('battle', 'g_battle_unit_stats') + STATS_BYTES * UNIT
                saved.workspace = {}
                for i = 0, WORKSPACE_BYTES - 1 do saved.workspace[i] = e.read8(ai + i) end
                saved.move = e.read8(stats + MOVE)
                saved.support3 = e.read8(stats + SUPPORT_3)
                saved.support4 = e.read8(stats + SUPPORT_4)
            end
            e.write8(stats + SUPPORT_3, case.support3 or 0)
            e.write8(stats + SUPPORT_4, case.support4 or 0)
            e.write8(stats + MOVE, case.move or saved.move)
            if case.item then return entry(SKILLSET_ITEM, POTION) end
            return entry(SKILLSET_BLACK_MAGIC, FIRE_2)
        end,
        done = function(result)
            values[case.name] = e.read8(ai + (case.item and CONSIDERED_RANGE or CONSIDERED_CT))
            if result.returned then values.calls_returned = values.calls_returned + 1 end
            if index == #cases then
                for i = 0, WORKSPACE_BYTES - 1 do e.write8(ai + i, saved.workspace[i]) end
                e.write8(stats + MOVE, saved.move)
                e.write8(stats + SUPPORT_3, saved.support3)
                e.write8(stats + SUPPORT_4, saved.support4)
            end
        end }
end

return {
    why = 'the AI plans charges with the wrong support bits and a thrown item at Move range (direct calls: '
        .. 'Gariland has no unit with these supports)',
    save = { file = 'smoke-new-game.mcd', sha256 = '02203c586fc996bd3c3dfa1fb07bb6e0025cb590b69b59dae955fc3b098c47e3' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'event-attack', name = 'attack_deploy_run_screen', label = 'overlay menu ATTACK.OUT deployment',
          before = { BUTTON.CIRCLE }, budget = 9000, shot = 90 },
    }),
    trigger = {
        e.call({ label = 'load ability entry', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' },
                       { address = report.address, limit = 0, label = 'error report' } } }),
    },
    check = function() return values end,
    expect = {
        original = { ct_none = 5, ct_short_charge = 5, ct_non_charge = 5, ct_short_and_non_charge = 5,
                     ct_monster_skill = 2, ct_defend = 0, range_potion = 1, range_throw_item_move_3 = 3,
                     range_throw_item_move_6 = 6, calls_returned = 9 },
        dc = { ct_none = 5, ct_short_charge = 3, ct_non_charge = 0, ct_short_and_non_charge = 0,
               ct_monster_skill = 5, ct_defend = 5, range_potion = 1, range_throw_item_move_3 = 4,
               range_throw_item_move_6 = 4, calls_returned = 9 },
    },
}
