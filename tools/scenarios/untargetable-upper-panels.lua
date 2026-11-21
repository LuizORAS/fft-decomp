-- Retail bug of battle_target_clear_panels_on_untargetable_tiles (QUIRKS.md):
-- the range of a single-tile or direct-targeting ability drops each tile the
-- map marks untargetable, but the loop stops after the lower layer's 256
-- panels, so an untargetable upper-layer tile stays in range (15 maps have
-- such tiles, and no unit can stop on one). Gariland has none, so at one stop
-- of BATTLE's thread_yield on its deployment screen (the new-game save) three
-- panels get range 3 and mark 1: lower tile 5 marked untargetable (the
-- control, cleared on both discs), upper tile 256 + 5 marked untargetable and
-- upper tile 256 + 6 unmarked (kept on both discs); then the function runs.
-- Tile flags and panels are put back after the call. On the original the
-- untargetable upper panel keeps its range and mark; on the Developer's Cut
-- it is cleared like the lower one.
local bit = require('bit')
local e = emulator
local BUTTON = e.BUTTON

-- A map tile's bytes, its flags byte and untargetable bit; a target panel's
-- bytes, range and mark; the offset of thread_yield's second instruction.
local TILE_BYTES, FLAGS, UNTARGETABLE = 8, 6, 0x02
local PANEL_BYTES, RANGE, MARK = 5, 0, 1
local CASES = { { name = 'lower_untargetable', tile = 5, flag = true },
                { name = 'upper_untargetable', tile = 256 + 5, flag = true },
                { name = 'upper_targetable', tile = 256 + 6, flag = false } }
local YIELD_SECOND_INSTRUCTION = 4

local anchor = e.symbol('battle', 'thread_yield')
local clear = e.symbol('battle', 'battle_target_clear_panels_on_untargetable_tiles')
local report = e.symbol('main', 'main_system_handle_error')

local values = {}
local saved = {}
local tiles, panels

local calls = {
    { target = clear, label = 'clear untargetable panels',
      args = function()
          tiles = e.address('battle', 'g_battle_map_tile_data')
          panels = e.address('battle', 'g_battle_target_panels')
          for _, c in ipairs(CASES) do
              local t, p = tiles + TILE_BYTES * c.tile + FLAGS, panels + PANEL_BYTES * c.tile
              saved[c.name] = { flags = e.read8(t), range = e.read8(p + RANGE), mark = e.read8(p + MARK) }
              local flags = bit.band(saved[c.name].flags, bit.bnot(UNTARGETABLE))
              if c.flag then flags = bit.bor(flags, UNTARGETABLE) end
              e.write8(t, flags)
              e.write8(p + RANGE, 3)
              e.write8(p + MARK, 1)
          end
          return {}
      end,
      done = function(result)
          values.returned = result.returned and 1 or 0
          for _, c in ipairs(CASES) do
              local t, p = tiles + TILE_BYTES * c.tile + FLAGS, panels + PANEL_BYTES * c.tile
              values[c.name] = string.format('range %d mark %d', e.read8(p + RANGE), e.read8(p + MARK))
              e.write8(t, saved[c.name].flags)
              e.write8(p + RANGE, saved[c.name].range)
              e.write8(p + MARK, saved[c.name].mark)
          end
      end },
}

return {
    why = 'an untargetable upper-layer tile stays in an ability\'s range (direct call: Gariland has no such tile)',
    save = { file = 'smoke-new-game.mcd', sha256 = '02203c586fc996bd3c3dfa1fb07bb6e0025cb590b69b59dae955fc3b098c47e3' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'event-attack', name = 'attack_deploy_run_screen', label = 'overlay menu ATTACK.OUT deployment',
          before = { BUTTON.CIRCLE }, budget = 9000, shot = 90 },
    }),
    trigger = {
        e.call({ label = 'clear panels', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' },
                       { address = report.address, limit = 0, label = 'error report' } } }),
    },
    check = function() return values end,
    expect = {
        original = {
            returned = 1, lower_untargetable = 'range 0 mark 0', upper_untargetable = 'range 3 mark 1',
            upper_targetable = 'range 3 mark 1',
        },
        dc = {
            returned = 1, lower_untargetable = 'range 0 mark 0', upper_untargetable = 'range 0 mark 0',
            upper_targetable = 'range 3 mark 1',
        },
    },
}
