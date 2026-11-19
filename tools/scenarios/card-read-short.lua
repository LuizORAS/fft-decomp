-- Retail bug of world_card_read_file_chunked (QUIRKS.md): before each of its
-- 40 attempts it seeks by the bytes read so far from the current position
-- (lseek origin 1), not from the start, so after a short first read of n bytes
-- the rest of the image comes from offset 2n. A card fault that cuts a read
-- short is not something the emulator does, so the BIOS read() is asked for
-- SHORT bytes on the first attempt; the runs use OpenBIOS, and the console's
-- BIOS was not tested. From the world map (the world-map save) the route opens
-- Data, card slot 1 and Load, and loads File 01. On the original the second
-- seek lands at 2n and the load refuses the image it read; on the Developer's
-- Cut the second seek lands at n and the load takes it (and restarts the game
-- from it, as any load from the world map does).
local bit = require('bit')
local e = emulator
local BUTTON = e.BUTTON
local run = e.run

-- The first read's bytes, the save image's bytes and WORLD's menu row Data.
local SHORT = 0x100
local IMAGE = 0x1e00
local DATA = { { menu = 4, row = 4, name = 'Data' } }

local values = { reads = 0 }

local function signed(register) return tonumber(bit.tobit(tonumber(register))) end

-- The load's read: the first BIOS read() inside it asks for SHORT bytes; the
-- second seek's result is the offset the next read works at; then the load's
-- verdict on the image.
local function armRead()
    local fn = e.symbol('world', 'world_card_read_file_chunked')
    local read = e.symbol('main', 'read')
    local lseek = e.symbol('main', 'lseek')
    local globals = e.symbol('world', 'world_card_load_globals_from_save_image')
    local inside, seeks = false, 0
    e.breakAt(fn.address, 'load read', function()
        if not e.resident(fn) then return true end
        inside = true
        e.breakAt(tonumber(PCSX.getRegisters().GPR.n.ra), 'load read return', function()
            inside = false
            values.read_returned = signed(PCSX.getRegisters().GPR.n.v0)
            return false
        end)
        return false
    end)
    e.breakAt(lseek.address, 'seek', function()
        if not inside then return true end
        seeks = seeks + 1
        if seeks == 2 then
            e.breakAt(tonumber(PCSX.getRegisters().GPR.n.ra), 'seek return', function()
                values.second_seek_at = signed(PCSX.getRegisters().GPR.n.v0)
                return false
            end)
        end
        return true
    end)
    e.breakAt(read.address, 'read', function()
        if not inside then return true end
        values.reads = values.reads + 1
        if values.reads == 1 then e.setGPR(6, SHORT) end
        return true
    end)
    e.breakAt(globals.address, 'load check', function()
        if not e.resident(globals) then return true end
        e.breakAt(tonumber(PCSX.getRegisters().GPR.n.ra), 'load check return', function()
            values.load_accepted = tonumber(PCSX.getRegisters().GPR.n.v0) ~= 0 and 1 or 0
            return false
        end)
        return false
    end)
end

return {
    why = 'a save read after a short first read comes from the wrong offset and is refused (forced fault: no card fault in the emulator)',
    save = { file = 'world-map.mcd', sha256 = 'adee3f05a445925002945e98791b6e8a461e5b3bc46ff28842c879f44243b945' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'wldcore', name = 'wldcore_entrypoint', label = 'world map WLDCORE', before = { BUTTON.CIRCLE },
          budget = 6000, shot = 90 },
    }),
    trigger = {
        { module = 'world', name = 'world_card_run_menu_screen', label = 'Data card screen', before = {},
          budget = 6000, shot = 60, start = function() e.openWorldMenu(DATA) end },
        { module = 'world', name = 'world_card_run_slot_select_step', label = 'card slot 1', before = {},
          budget = 1200, shot = 30, after = { { nil, 30 }, { BUTTON.CIRCLE, 40 } } },
        -- Load (row 1), File 01 (the first occupied file), then Yes.
        { module = 'world', name = 'world_card_run_mode_select_step', label = 'load File 01', before = {},
          budget = 1200, shot = 30,
          after = { { nil, 60 }, { BUTTON.DOWN, 40 }, { nil, 20 }, { BUTTON.CIRCLE, 60 }, { nil, 90 }, { BUTTON.CIRCLE, 60 },
              { nil, 60 }, { BUTTON.CIRCLE, 60 } } },
        { module = 'world', name = 'world_card_run_load_step', label = 'load', before = {},
          budget = 3000, shot = 30, start = armRead },
        { label = 'settle', frames = 600 },
    },
    check = function() return values end,
    expect = {
        original = { reads = 2, second_seek_at = 2 * SHORT, read_returned = IMAGE, load_accepted = 0 },
        dc = { reads = 2, second_seek_at = SHORT, read_returned = IMAGE, load_accepted = 1 },
    },
}
