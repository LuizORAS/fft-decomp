-- Retail bug of battle_move_start_next_post_move_event (QUIRKS.md): when a
-- unit springs a trap, it passes the mover's x, layer and row to
-- battle_ai_record_sprung_trap, which takes x, row and layer, so the AI keeps
-- the row as the layer and the layer as the row and keeps off the wrong tile
-- (from row 2 its mask write lands in another AI mask). Traps lie only in
-- four random-battle maps and the Deep Dungeon, so the trap is put in RAM on
-- Gariland's real route (the new-game save): from the AI stop of an
-- Auto-battle turn, the first unit that ends a move gets a Steel Needle under
-- it (Move-Find entry 0 becomes its tile, with no item, at
-- battle_move_get_post_move_events' entry), and the trap springs through the
-- real post-move flow. At battle_ai_record_sprung_trap the scenario reads the
-- arguments (x, row, layer as the function takes them) and, after it returns,
-- the AI's first sprung-trap tile; entry 0 is put back. On the original the
-- AI records row 0 and layer 9 for the tile at row 9 of layer 0; on the
-- Developer's Cut it records that tile.
local bit = require('bit')
local e = emulator
local BUTTON = e.BUTTON

-- The Auto-battle menu rows; a unit's x and position (row in the low byte,
-- the layer in bit 15); a Move-Find entry's bytes and its flags for no item
-- and a Steel Needle; the AI workspace's first sprung-trap tile (x, layer,
-- row).
local AUTO_BATTLE = { { row = 4, name = 'Auto-battle' }, { menu = 1, row = 3, name = 'Save fading life' } }
local STATS_X, STATS_POSITION = 0x047, 0x048
local ENTRY_BYTES, NO_ITEM_STEEL_NEEDLE = 4, 0x28
local SPRUNG = 0x181c

local post = e.symbol('battle', 'battle_move_get_post_move_events')
local record = e.symbol('battle', 'battle_ai_record_sprung_trap')

local values = {}

local step = { label = 'trap sprung', before = { BUTTON.CIRCLE }, budget = 12000, start = function(passed)
    local entry = e.address('battle', 'g_battle_current_map_move_find_item_data')
    local ai = e.address('battle', 'g_battle_ai_data_base')
    local saved = {}
    e.breakAt(post.address, 'trap under the mover', function()
        if not e.resident(post) then return true end
        local unit = tonumber(PCSX.getRegisters().GPR.n.a0)
        local x, position = e.read8(unit + STATS_X), e.read16(unit + STATS_POSITION)
        local y, layer = bit.band(position, 0xff), bit.rshift(position, 15)
        values.tile = string.format('x %d y %d layer %d', x, y, layer)
        for i = 0, ENTRY_BYTES - 1 do saved[i] = e.read8(entry + i) end
        e.write8(entry, bit.bor(bit.lshift(x, 4), y))
        e.write8(entry + 1, bit.bor(bit.lshift(layer, 7), NO_ITEM_STEEL_NEEDLE))
        e.write8(entry + 2, 0)
        e.write8(entry + 3, 0)
        return false
    end)
    e.breakAt(record.address, 'record', function()
        if not e.resident(record) then return true end
        local regs = PCSX.getRegisters()
        values.arguments = string.format('%d %d %d', tonumber(regs.GPR.n.a0), tonumber(regs.GPR.n.a1),
            tonumber(regs.GPR.n.a2))
        e.breakAt(tonumber(regs.GPR.n.ra), 'after record', function()
            local tile = ai + SPRUNG
            values.recorded = string.format('x %d y %d layer %d', e.read8(tile), e.read8(tile + 2), e.read8(tile + 1))
            for i = 0, ENTRY_BYTES - 1 do e.write8(entry + i, saved[i]) end
            passed()
            return false
        end)
        return false
    end)
end }

return {
    why = 'the AI records a sprung trap with its row and layer swapped (real route; the trap is put in RAM, as Gariland has none)',
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
        original = { tile = 'x 7 y 9 layer 0', arguments = '7 0 9', recorded = 'x 7 y 0 layer 9' },
        dc = { tile = 'x 7 y 9 layer 0', arguments = '7 9 0', recorded = 'x 7 y 9 layer 0' },
    },
}
