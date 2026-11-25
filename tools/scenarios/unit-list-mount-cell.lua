-- Retail bug of bunit_unit_build_list_index_map (QUIRKS.md): the unit list's
-- grid leaves out the mounts, which stand in their riders' cells, but a
-- mount's entry of the map from a unit to its cell is its rider's place in the
-- list of units, so browsing to it with L1 or R1 puts the grid cursor on a
-- later unit's cell. Gariland has no chocobo, so a call makes the map on
-- BUNIT.OUT's unit list, at one stop of BATTLE's thread_yield. The route is
-- the Gariland battle (the new-game save): on the first turn menu Cross frees
-- the cursor, Triangle opens the mini menu and its Unit List row opens BUNIT.
-- For the call the unit at place 1 becomes a mount whose rider is the unit at
-- place 4, after it in the list. The scenario reads the map's cells of the
-- mount, its rider and the two units between; the two records and the map are
-- put back after the call. The rider's cell is 3, as place 1 takes none. On
-- the original the mount's cell is 4, the rider's place; on the Developer's
-- Cut it is 3 too.
local e = emulator
local BUTTON = e.BUTTON
local run = e.run

-- The mini menu's Unit List row; the offset of thread_yield's second
-- instruction; a record's battle id and mount byte, with the mount and rider
-- flags; the mount's and the rider's places and the map's bytes.
local UNIT_LIST = { { menu = 10, row = 1, name = 'Unit List' } }
local YIELD_SECOND_INSTRUCTION = 4
local BATTLE_ID, MOUNT_INFO, IS_MOUNT, IS_RIDER = 0x2c, 0x73, 0x40, 0x80
local MOUNT_PLACE, RIDER_PLACE, MAP_BYTES = 1, 4, 21

local anchor = e.symbol('battle', 'thread_yield')
local build = e.symbol('event-bunit', 'bunit_unit_build_list_index_map')
local report = e.symbol('main', 'main_system_handle_error')

local values = { calls_returned = 0 }
local saved
local mount, rider, map

-- The first turn menu: Cross, then Triangle, which opens the mini menu, whose
-- walk takes the Unit List row.
local function openUnitList()
    e.startDeployment()
    local turn = e.symbol('battle', 'battle_menu_build_idle_action_menu')
    local mini = e.symbol('battle', 'battle_menu_start_mini_menu_display_thread')
    e.breakAt(turn.address, 'turn menu', function()
        if not e.resident(turn) then return true end
        run.pattern = nil
        run.queue = { { nil, 60 }, { BUTTON.CROSS, 40 }, { nil, 40 }, { BUTTON.TRIANGLE, 40 } }
        return false
    end)
    e.breakAt(mini.address, 'mini menu', function()
        if not e.resident(mini) then return true end
        run.walk = { plan = UNIT_LIST, index = 1, tables = e.address('battle', 'g_menu_thread_menu_data') }
        run.queue = { { nil, 40 } }
        return false
    end)
end

local calls = {
    { target = build, label = 'build the map',
      args = function()
          local pointers = e.address('event-bunit', 'g_bunit_unit_pointers')
          mount, rider = e.read32(pointers + 4 * MOUNT_PLACE), e.read32(pointers + 4 * RIDER_PLACE)
          map = e.address('event-bunit', 'g_bunit_unit_list_index_map')
          saved = { mount = e.read8(mount + MOUNT_INFO), rider = e.read8(rider + MOUNT_INFO), map = {} }
          for i = 0, MAP_BYTES - 1 do saved.map[i] = e.read8(map + i) end
          e.write8(mount + MOUNT_INFO, IS_MOUNT + e.read16(rider + BATTLE_ID))
          e.write8(rider + MOUNT_INFO, IS_RIDER + e.read16(mount + BATTLE_ID))
          return {}
      end,
      done = function(result)
          if result.returned then values.calls_returned = values.calls_returned + 1 end
          values.mount_cell = e.read8(map + MOUNT_PLACE)
          values.rider_cell = e.read8(map + RIDER_PLACE)
          values.place_2_cell = e.read8(map + 2)
          values.place_3_cell = e.read8(map + 3)
          e.write8(mount + MOUNT_INFO, saved.mount)
          e.write8(rider + MOUNT_INFO, saved.rider)
          for i = 0, MAP_BYTES - 1 do e.write8(map + i, saved.map[i]) end
      end },
}

return {
    why = 'the unit list puts a mount on the cell of its rider\'s place in the list (direct call: Gariland has no chocobo)',
    save = { file = 'smoke-new-game.mcd', sha256 = '02203c586fc996bd3c3dfa1fb07bb6e0025cb590b69b59dae955fc3b098c47e3' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'event-attack', name = 'attack_deploy_run_screen', label = 'overlay menu ATTACK.OUT deployment',
          before = { BUTTON.CIRCLE }, budget = 9000, shot = 90 },
        { module = 'event-bunit', name = 'bunit_menu_run_display_script', label = 'overlay menu BUNIT.OUT unit list',
          before = { BUTTON.CIRCLE }, budget = 40000, shot = 90, start = openUnitList },
    }),
    trigger = {
        e.call({ label = 'unit list map', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' },
                       { address = report.address, limit = 0, label = 'error report' } } }),
    },
    check = function() return values end,
    expect = {
        original = { mount_cell = 4, rider_cell = 3, place_2_cell = 1, place_3_cell = 2, calls_returned = 1 },
        dc = { mount_cell = 3, rider_cell = 3, place_2_cell = 1, place_3_cell = 2, calls_returned = 1 },
    },
}
