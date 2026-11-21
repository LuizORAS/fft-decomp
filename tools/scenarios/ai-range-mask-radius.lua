-- Retail bug of battle_ai_build_targetable_tile_mask (QUIRKS.md): the AI's
-- range diamond, the tiles within a radius of a target from which the AI
-- plans to act, builds each row from eight templates of half-width 0 to 7, so
-- radii 8 to 15 read past them into g_battle_ai_status_priority_weights: a
-- half-width of 8 gives an empty row and 9 to 15 give stray bits. Guns,
-- Shock, Dispose, Earth Slash, Kikuichimoji and Level Jump 8 (radius 8), Holy
-- Explosion and All-Ultima aimed at tiles (range plus area 10 and 8) and a
-- Throw with Move 8 or more reach it. No early battle has such a unit, so
-- calls make the diamond at the AI stop of a Gariland Auto-battle turn (the
-- next BATTLE thread_yield after the AI first considers an ability, map 15
-- rows): radius 7 around 2,7 (the control, the same on both discs), radius 8
-- around 2,7, 9,7 and 2,0, radius 10 and 15 around 5,7. Each call reads the
-- map's rows of the lower level. The workspace is put back after the last
-- call. On the original the target's row at radius 8 is empty and radii 10
-- and 15 leave holes and stray bits; on the Developer's Cut every row spans
-- the half-width it should.
local e = emulator
local BUTTON = e.BUTTON

-- The Auto-battle menu rows, the AI workspace's bytes, its mask, considered
-- coordinates (x, elevation, y) and map height, the explicit-radius mode, and
-- the offset of thread_yield's second instruction.
local AUTO_BATTLE = { { row = 4, name = 'Auto-battle' }, { menu = 1, row = 3, name = 'Save fading life' } }
local WORKSPACE_BYTES = 0x19c8
local TARGETABLE, COORDS, MAX_Y = 0x0b4c, 0x0c74, 0x0e3b
local EXPLICIT = 3
local YIELD_SECOND_INSTRUCTION = 4

local anchor = e.symbol('battle', 'thread_yield')
local mask = e.symbol('battle', 'battle_ai_build_targetable_tile_mask')
local report = e.symbol('main', 'main_system_handle_error')

local cases = {
    { name = 'r7_x2_y7', x = 2, y = 7, radius = 7 },
    { name = 'r8_x2_y7', x = 2, y = 7, radius = 8 },
    { name = 'r8_x9_y7', x = 9, y = 7, radius = 8 },
    { name = 'r8_x2_y0', x = 2, y = 0, radius = 8 },
    { name = 'r10_x5_y7', x = 5, y = 7, radius = 10 },
    { name = 'r15_x5_y7', x = 5, y = 7, radius = 15 },
}

local values = { calls_returned = 0 }
local saved = {}
local ai

local calls = {}
for index, case in ipairs(cases) do
    calls[#calls + 1] = { target = mask, label = case.name,
        args = function()
            if index == 1 then
                ai = e.address('battle', 'g_battle_ai_data_base')
                values.map_rows = e.read8(ai + MAX_Y)
                saved.workspace = {}
                for i = 0, WORKSPACE_BYTES - 1 do saved.workspace[i] = e.read8(ai + i) end
            end
            e.write8(ai + COORDS, case.x)
            e.write8(ai + COORDS + 1, 0)
            e.write8(ai + COORDS + 2, case.y)
            return { EXPLICIT, case.radius }
        end,
        done = function(result)
            local rows = {}
            for r = 0, values.map_rows - 1 do rows[#rows + 1] = string.format('%04x', e.read16(ai + TARGETABLE + 2 * r)) end
            values[case.name] = table.concat(rows, ' ')
            if result.returned then values.calls_returned = values.calls_returned + 1 end
            if index == #cases then
                for i = 0, WORKSPACE_BYTES - 1 do e.write8(ai + i, saved.workspace[i]) end
            end
        end }
end

-- The rows both discs share.
local R7_X2_Y7 = '2000 7000 f800 fc00 fe00 ff00 ff80 ffc0 ff80 ff00 fe00 fc00 f800 7000 2000'

return {
    why = 'the AI range diamond reads past its templates at radii 8-15 (direct calls: no early battle has such a unit)',
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
        e.call({ label = 'AI stop', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' },
                       { address = report.address, limit = 0, label = 'error report' } } }),
    },
    check = function() return values end,
    expect = {
        original = {
            map_rows = 15, calls_returned = 6, r7_x2_y7 = R7_X2_Y7,
            r8_x2_y7 = '7000 f800 fc00 fe00 ff00 ff80 ffc0 0000 ffc0 ff80 ff00 fe00 fc00 f800 7000',
            r8_x9_y7 = '00e0 01f0 03f8 07fc 0ffe 1fff 3fff 0000 3fff 1fff 0ffe 07fc 03f8 01f0 00e0',
            r8_x2_y0 = '0000 ffc0 ff80 ff00 fe00 fc00 f800 7000 2000 0000 0000 0000 0000 0000 0000',
            r10_x5_y7 = '3f80 7fc0 ffe0 fff0 fff8 0000 fd00 fd00 fd00 0000 fff8 fff0 ffe0 7fc0 3f80',
            r15_x5_y7 = '0000 fd00 fd00 ff64 0000 0000 0000 0000 0000 0000 0000 ff64 fd00 fd00 0000',
        },
        dc = {
            map_rows = 15, calls_returned = 6, r7_x2_y7 = R7_X2_Y7,
            r8_x2_y7 = '7000 f800 fc00 fe00 ff00 ff80 ffc0 ffe0 ffc0 ff80 ff00 fe00 fc00 f800 7000',
            r8_x9_y7 = '00e0 01f0 03f8 07fc 0ffe 1fff 3fff 7fff 3fff 1fff 0ffe 07fc 03f8 01f0 00e0',
            r8_x2_y0 = 'ffe0 ffc0 ff80 ff00 fe00 fc00 f800 7000 2000 0000 0000 0000 0000 0000 0000',
            r10_x5_y7 = '3f80 7fc0 ffe0 fff0 fff8 fffc fffe ffff fffe fffc fff8 fff0 ffe0 7fc0 3f80',
            r15_x5_y7 = 'fffc fffe ffff ffff ffff ffff ffff ffff ffff ffff ffff ffff ffff fffe fffc',
        },
    },
}
