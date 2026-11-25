-- Retail bug of bunit_gfx_init_double_packet_buffers (QUIRKS.md), which
-- WORLD's formation screen shares: the draw and display environments are 255
-- pixels wide where MAIN's and CARD's are 256, so the default drawing area
-- ends at x = 254 and the screen's last column stays black. The unit list
-- draws its floor across all 256 columns. The route is the Gariland battle
-- (the new-game save): on the first turn menu Cross frees the cursor, Triangle
-- opens the mini menu and its Unit List row opens BUNIT. After 30 of its frames
-- the scenario reads the width of both packet buffers' environments and the
-- screenshot's last two columns: whether each has a pixel that is not black.
-- On the original the widths are 255 and the last column is black; on the
-- Developer's Cut they are 256 and it shows the floor.
local e = emulator
local BUTTON = e.BUTTON
local run = e.run

-- The mini menu's Unit List row; the frames to wait; a packet buffer's bytes,
-- its draw environment's clip and its display environment's rectangle, with a
-- rectangle's width.
local UNIT_LIST = { { menu = 10, row = 1, name = 'Unit List' } }
local FRAMES = 30
local BUFFER_BYTES, DRAW_CLIP, DISP, WIDTH = 0xec, 0x64, 0xc0, 0x04

local present = e.symbol('event-bunit', 'bunit_gfx_submit_frame_and_swap_buffers')

local values = {}

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

-- Whether a column of a 16-bit or 24-bit screenshot has a pixel that is not black.
local function drawn(shot, column)
    local depth = tonumber(shot.bpp) == 0 and 2 or 3
    local width, height = tonumber(shot.width), tonumber(shot.height)
    local bytes = tostring(shot.data)
    for y = 0, height - 1 do
        local at = (y * width + width - column) * depth
        for i = 1, depth do
            if string.byte(bytes, at + i) ~= 0 then return 1 end
        end
    end
    return 0
end

local function readScreen()
    local buffers = e.read32(e.address('event-bunit', 'g_bunit_gfx_context_base'))
    for i = 0, 1 do
        local buffer = buffers + BUFFER_BYTES * i
        values['draw_width_' .. i] = e.read16(buffer + DRAW_CLIP + WIDTH)
        values['display_width_' .. i] = e.read16(buffer + DISP + WIDTH)
    end
    local shot = PCSX.GPU.takeScreenShot()
    values.column_254_drawn = drawn(shot, 2)
    values.column_255_drawn = drawn(shot, 1)
end

-- The screen after a number of its frames.
local function waitFrames(passed)
    local count = 0
    e.breakAt(present.address, 'frame', function()
        if not e.resident(present) then return true end
        count = count + 1
        if count < FRAMES then return true end
        readScreen()
        passed()
        return false
    end)
end

local function expected(width, last)
    return { draw_width_0 = width, draw_width_1 = width, display_width_0 = width, display_width_1 = width,
        column_254_drawn = 1, column_255_drawn = last }
end

return {
    why = 'the unit list\'s drawing area stops one column short of the screen (real route)',
    save = { file = 'smoke-new-game.mcd', sha256 = '02203c586fc996bd3c3dfa1fb07bb6e0025cb590b69b59dae955fc3b098c47e3' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'event-attack', name = 'attack_deploy_run_screen', label = 'overlay menu ATTACK.OUT deployment',
          before = { BUTTON.CIRCLE }, budget = 9000, shot = 90 },
        { module = 'event-bunit', name = 'bunit_menu_run_display_script', label = 'overlay menu BUNIT.OUT unit list',
          before = { BUTTON.CIRCLE }, budget = 40000, shot = 90, start = openUnitList },
    }),
    trigger = { { label = 'unit list frames', budget = 900, start = waitFrames } },
    check = function() return values end,
    expect = { original = expected(255, 0), dc = expected(256, 1) },
}
