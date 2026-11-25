-- Retail bug of battle_unit_apply_level_up_down_ability (QUIRKS.md): a
-- level-down on a level-1 unit clamps the level back to 1 and still takes
-- one level of growth off every raw stat. Level Blast misses below level 2,
-- so only the Degenerator trap gets there. Traps lie only in four
-- random-battle maps and the Deep Dungeon, so the trap is put in RAM on
-- Gariland's real route (the new-game save): from the AI stop of an
-- Auto-battle turn, the first unit that ends a move is set to level 1 and the
-- next other unit to level 5, and each gets a Degenerator under it (Move-Find
-- entry 0 becomes its tile, with no item, at battle_move_get_post_move_events'
-- entry); the trap springs through the real post-move flow. The scenario
-- reads whether the trap's result is a hit, and, at the next unit's move, the
-- unit's level and whether its raw stats stayed the same or fell; entry 0 is
-- put back. On the original the level-1 unit stays at level 1 and its raw
-- stats fall; on the Developer's Cut the trap misses it, as Level Blast does,
-- and nothing changes. The level-5 unit goes to level 4 and its raw stats
-- fall on both.
local bit = require('bit')
local e = emulator
local BUTTON = e.BUTTON

-- The Auto-battle menu rows; a unit's level, x, position (row in the low
-- byte, the layer in bit 15), raw stats and result record's hit byte; a
-- Move-Find entry's bytes and its flags for no item and a Degenerator.
local AUTO_BATTLE = { { row = 4, name = 'Auto-battle' }, { menu = 1, row = 3, name = 'Save fading life' } }
local STATS_LEVEL, STATS_X, STATS_POSITION = 0x022, 0x047, 0x048
local RAW_STATS, RAW_STAT_BYTES = 0x072, 15
local ACTION_HIT = 0x18c
local ENTRY_BYTES, NO_ITEM_DEGENERATOR = 4, 0x21

local post = e.symbol('battle', 'battle_move_get_post_move_events')
local trap = e.symbol('battle', 'battle_action_apply_tile_trap')

local values = {}
local cases = { { name = 'level_1', level = 1 }, { name = 'level_5', level = 5 } }

local function raw(unit)
    local bytes = {}
    for i = 0, RAW_STAT_BYTES - 1 do bytes[#bytes + 1] = string.format('%02x', e.read8(unit + RAW_STATS + i)) end
    return table.concat(bytes)
end

local step = { label = 'Degenerator sprung', before = { BUTTON.CIRCLE }, budget = 30000, start = function(passed)
    local entry = e.address('battle', 'g_battle_current_map_move_find_item_data')
    local saved = {}
    for i = 0, ENTRY_BYTES - 1 do saved[i] = e.read8(entry + i) end
    local index, current = 0, nil
    e.playTurns(AUTO_BATTLE)
    e.breakAt(post.address, 'Degenerator under the mover', function()
        if not e.resident(post) then return true end
        local unit = tonumber(PCSX.getRegisters().GPR.n.a0)
        if current then
            if unit == current.unit then return true end
            local before = current.raw
            local after = raw(current.unit)
            values[current.case.name .. '_level'] = string.format('%d -> %d', current.case.level,
                e.read8(current.unit + STATS_LEVEL))
            values[current.case.name .. '_raw'] = after == before and 'same' or 'changed'
            current = nil
        end
        index = index + 1
        local case = cases[index]
        if not case then
            for i = 0, ENTRY_BYTES - 1 do e.write8(entry + i, saved[i]) end
            passed()
            return false
        end
        e.write8(unit + STATS_LEVEL, case.level)
        current = { unit = unit, case = case, raw = raw(unit) }
        local x, position = e.read8(unit + STATS_X), e.read16(unit + STATS_POSITION)
        e.write8(entry, bit.bor(bit.lshift(x, 4), bit.band(position, 0xff)))
        e.write8(entry + 1, bit.bor(bit.lshift(bit.rshift(position, 15), 7), NO_ITEM_DEGENERATOR))
        e.write8(entry + 2, 0)
        e.write8(entry + 3, 0)
        return true
    end)
    e.breakAt(trap.address, 'trap', function()
        if not e.resident(trap) or not current then return true end
        local unit = current.unit
        local name = current.case.name
        e.breakAt(tonumber(PCSX.getRegisters().GPR.n.ra), 'after trap', function()
            values[name .. '_hit'] = e.read8(unit + ACTION_HIT)
            return false
        end)
        return true
    end)
end }

return {
    why = 'a level-1 unit on a Degenerator keeps level 1 and loses a level of growth (real route; the trap is put in '
        .. 'RAM, as Gariland has none)',
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
    trigger = { step },
    check = function() return values end,
    expect = {
        original = { level_1_hit = 1, level_1_level = '1 -> 1', level_1_raw = 'changed',
                     level_5_hit = 1, level_5_level = '5 -> 4', level_5_raw = 'changed' },
        dc = { level_1_hit = 0, level_1_level = '1 -> 1', level_1_raw = 'same',
               level_5_hit = 1, level_5_level = '5 -> 4', level_5_raw = 'changed' },
    },
}
