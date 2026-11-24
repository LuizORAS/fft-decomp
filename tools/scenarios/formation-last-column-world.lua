-- Retail bug of world_gfx_init_double_packet_buffers (QUIRKS.md), which
-- BUNIT's unit list shares: the draw and display environments are 255 pixels
-- wide where MAIN's and CARD's are 256, so the default drawing area ends at
-- x = 254 and the screen's last column stays black. The formation screen (the
-- world-map save) draws its floor across all 256 columns. After 30 of its
-- frames the scenario reads the width of both packet buffers' environments
-- and the screenshot's last two columns: whether each has a pixel that is not
-- black. On the original the widths are 255 and the last column is black; on
-- the Developer's Cut they are 256 and it shows the floor.
local e = emulator
local BUTTON = e.BUTTON

-- WORLD's menu row Formation; the frames to wait; a packet buffer's bytes,
-- its draw environment's clip and its display environment's rectangle, with a
-- rectangle's width.
local FORMATION = { { menu = 4, row = 1, name = 'Formation' } }
local FRAMES = 30
local BUFFER_BYTES, DRAW_CLIP, DISP, WIDTH = 0xec, 0x64, 0xc0, 0x04

local present = e.symbol('world', 'world_gfx_present_frame_and_swap_packet_buffer')

local values = {}

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
    local buffers = e.read32(e.address('world', 'g_world_gfx_packet_buffers'))
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
    why = 'the formation screen\'s drawing area stops one column short of the screen (real route)',
    save = { file = 'world-map.mcd', sha256 = 'adee3f05a445925002945e98791b6e8a461e5b3bc46ff28842c879f44243b945' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'wldcore', name = 'wldcore_entrypoint', label = 'world map WLDCORE', before = { BUTTON.CIRCLE },
          budget = 6000, shot = 90 },
        { module = 'world', name = 'world_formation_run_screen', label = 'WORLD menu formation screen', before = {},
          budget = 6000, shot = 90, start = function() e.openWorldMenu(FORMATION) end },
    }),
    trigger = { { label = 'formation frames', budget = 900, start = waitFrames } },
    check = function() return values end,
    expect = { original = expected(255, 0), dc = expected(256, 1) },
}
