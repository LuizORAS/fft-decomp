-- Retail bug of battle_unit_place_in_new_location (QUIRKS.md): WarpUnit and
-- the paired warp hand battle_unit_set_tile_position the sprite's facing (the
-- quadrant shifted left 10) where it takes quarter turns, so the warped unit's
-- battle stats face south (0) while its sprite faces as asked, until the
-- event ends. The function runs only inside events, so a call makes it at one
-- stop of BATTLE's thread_yield, from the AI stop of an Auto-battle turn on
-- Gariland's real route (the new-game save): the runtime unit of unit slot 0
-- is warped onto its own tile facing 3, with the tile in the paired warp's
-- coordinate record. The scenario reads the facing of the unit's battle stats
-- and of its sprite; the unit, its runtime unit and the coordinate record are
-- put back after the call. On the original the stats face 0 and the sprite 3;
-- on the Developer's Cut both face 3.
local e = emulator
local BUTTON = e.BUTTON

-- The Auto-battle menu rows; the unit, a unit record's bytes and the byte
-- whose low nibble is its facing; the runtime units, their count and bytes,
-- the runtime id, the map tile, the sprite facing (0x400 per quarter turn)
-- and the stats link; the coordinate record's bytes and its x, layer and y;
-- the facing asked; the offset of thread_yield's second instruction.
local AUTO_BATTLE = { { row = 4, name = 'Auto-battle' }, { menu = 1, row = 3, name = 'Save fading life' } }
local UNIT, STATS_BYTES, STATS_FACING = 0, 448, 0x049
local RUNTIME_COUNT, RUNTIME_BYTES, RUNTIME_ID, MAP_X, MAP_Y, MAP_Z, FACING, STATS_LINK =
    16, 1088, 0x004, 0x07c, 0x07d, 0x07e, 0x070, 0x134
local COORDS_BYTES, COORD_X, COORD_LAYER, COORD_Y = 6, 0x00, 0x02, 0x04
local ASKED = 3
local YIELD_SECOND_INSTRUCTION = 4

local anchor = e.symbol('battle', 'thread_yield')
local place = e.symbol('battle', 'battle_unit_place_in_new_location')
local report = e.symbol('main', 'main_system_handle_error')

local values = { calls_returned = 0 }
local saved
local stats, runtime, coords

local function readBytes(at, count)
    local bytes = {}
    for i = 0, count - 1 do bytes[i] = e.read8(at + i) end
    return bytes
end

local function writeBytes(at, bytes, count)
    for i = 0, count - 1 do e.write8(at + i, bytes[i]) end
end

-- The runtime unit linked to the unit.
local function findRuntime()
    local base = e.address('battle', 'g_battle_units')
    for slot = 0, RUNTIME_COUNT - 1 do
        local at = base + RUNTIME_BYTES * slot
        if e.read32(at + STATS_LINK) == stats then return at end
    end
    error('no runtime unit links the unit')
end

local calls = {
    { target = place, label = 'warp facing 3',
      args = function()
          stats = e.address('battle', 'g_battle_unit_stats') + STATS_BYTES * UNIT
          coords = e.address('battle', 'g_battle_script_warp_unit_coords')
          runtime = findRuntime()
          saved = { stats = readBytes(stats, STATS_BYTES), runtime = readBytes(runtime, RUNTIME_BYTES),
                    coords = readBytes(coords, COORDS_BYTES) }
          e.write16(coords + COORD_X, e.read8(runtime + MAP_X))
          e.write16(coords + COORD_LAYER, e.read8(runtime + MAP_Z))
          e.write16(coords + COORD_Y, e.read8(runtime + MAP_Y))
          return { e.read8(runtime + RUNTIME_ID), coords, ASKED }
      end,
      done = function(result)
          if result.returned then values.calls_returned = values.calls_returned + 1 end
          values.stats_facing = e.read8(stats + STATS_FACING) % 16
          values.sprite_facing = math.floor(e.read16(runtime + FACING) / 1024) % 4
          writeBytes(stats, saved.stats, STATS_BYTES)
          writeBytes(runtime, saved.runtime, RUNTIME_BYTES)
          writeBytes(coords, saved.coords, COORDS_BYTES)
      end },
}

return {
    why = 'WarpUnit leaves the battle stats facing south (direct call: the function runs only inside events)',
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
        e.call({ label = 'warp', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' },
                       { address = report.address, limit = 0, label = 'error report' } } }),
    },
    check = function() return values end,
    expect = {
        original = { stats_facing = 0, sprite_facing = 3, calls_returned = 1 },
        dc = { stats_facing = 3, sprite_facing = 3, calls_returned = 1 },
    },
}
