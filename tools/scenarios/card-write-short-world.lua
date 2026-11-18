-- Retail bug of world_card_write_buffer_to_file (QUIRKS.md): before each of
-- its 40 attempts it seeks by the bytes written so far from the current
-- position (lseek origin 1), not from the start, so after a short first write
-- of n bytes the rest of the save lands at offset 2n. A card fault that cuts
-- a write short is not something the emulator does, so the BIOS write() is
-- asked for SHORT bytes on the first attempt; the runs use OpenBIOS, and the
-- console's BIOS was not tested. From the world map (the world-map save) the
-- route opens Data, card slot 1 and Save, and saves to File 02, which is empty
-- and asks for no overwrite. Once the save step is done the card file is
-- compared with the image the game wrote; then Data's Load loads File 02 back.
-- On the original the second seek lands at 2n, the game reports the save
-- written, the file differs from the image and the load refuses it; on the
-- Developer's Cut the second seek lands at n, the file equals the image and the
-- load takes it. The run writes only the copy of the card in its own directory.
local bit = require('bit')
local e = emulator
local BUTTON = e.BUTTON
local run = e.run

-- The first write's bytes, the save image's bytes, WORLD's menu row Data, the
-- card's block and directory frame bytes, and the frames the card file needs
-- to reach memcard1.mcd after the write.
local SHORT = 0x100
local IMAGE = 0x1e00
local DATA = { { menu = 4, row = 4, name = 'Data' } }
local BLOCK = 0x2000
local FRAME = 0x80
local CARD_FLUSH = 600

local values = { writes = 0 }
local fileName, source

local function signed(register) return tonumber(bit.tobit(tonumber(register))) end
local function text(at)
    local chars = {}
    for i = 0, 20 do
        local c = e.read8(at + i)
        if c == 0 then break end
        chars[#chars + 1] = string.char(c)
    end
    return table.concat(chars)
end

-- The save's write: the first BIOS write() inside it asks for SHORT bytes;
-- the second seek's result is the offset the next write works at.
local function armWrite()
    local fn = e.symbol('world', 'world_card_write_buffer_to_file')
    local write = e.symbol('main', 'write')
    local lseek = e.symbol('main', 'lseek')
    local inside, seeks = false, 0
    e.breakAt(fn.address, 'save write', function()
        if not e.resident(fn) then return true end
        local n = PCSX.getRegisters().GPR.n
        inside = true
        fileName, source = text(tonumber(n.a0)), tonumber(n.a1)
        e.breakAt(tonumber(n.ra), 'save write return', function()
            inside = false
            values.write_returned = signed(PCSX.getRegisters().GPR.n.v0)
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
    e.breakAt(write.address, 'write', function()
        if not inside then return true end
        values.writes = values.writes + 1
        if values.writes == 1 then e.setGPR(6, SHORT) end
        return true
    end)
end

-- The card file's first IMAGE bytes against the image in RAM.
local function compare(passed)
    run.timer = { at = run.frame + CARD_FLUSH, call = function()
        local card = e.readText(run.dir .. '/memcard1.mcd') or ''
        local name = fileName:match('BASCUS%-%d+.*') or fileName
        local file
        for k = 1, 15 do
            if card:sub(FRAME * k + 11, FRAME * k + 10 + #name) == name then file = card:sub(BLOCK * k + 1, BLOCK * k + IMAGE) end
        end
        values.file_found = file and 1 or 0
        if file then
            local differing = 0
            for i = 0, IMAGE - 1 do
                if file:byte(i + 1) ~= e.read8(source + i) then differing = differing + 1 end
            end
            values.bytes_differing = differing
        end
        passed()
    end }
end

-- The load's verdict on the image it read back.
local function armLoad()
    local globals = e.symbol('world', 'world_card_load_globals_from_save_image')
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
    why = 'a save written after a short first write is misplaced on the card and refused on load (forced fault: no card fault in the emulator)',
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
        -- Save (row 0), then File 02.
        { module = 'world', name = 'world_card_run_mode_select_step', label = 'save to File 02', before = {},
          budget = 1200, shot = 30,
          after = { { nil, 60 }, { BUTTON.CIRCLE, 60 }, { nil, 90 }, { BUTTON.DOWN, 40 }, { nil, 30 }, { BUTTON.CIRCLE, 30 } } },
        { module = 'world', name = 'world_card_run_save_step', label = 'save', before = {},
          budget = 3000, shot = 30, start = armWrite },
        { label = 'card file', budget = CARD_FLUSH + 60, start = compare },
        { module = 'world', name = 'world_card_run_slot_select_step', label = 'card slot 1 again', before = {},
          budget = 1800, shot = 30, start = function() run.queue = { { BUTTON.CROSS, 60 } } end,
          after = { { nil, 30 }, { BUTTON.CIRCLE, 40 } } },
        -- Load (row 1), File 02 (the second occupied file), then Yes.
        { module = 'world', name = 'world_card_run_mode_select_step', label = 'load File 02', before = {},
          budget = 1800, shot = 30,
          after = { { nil, 60 }, { BUTTON.DOWN, 40 }, { nil, 20 }, { BUTTON.CIRCLE, 60 }, { nil, 90 }, { BUTTON.DOWN, 40 },
              { nil, 30 }, { BUTTON.CIRCLE, 60 }, { nil, 60 }, { BUTTON.CIRCLE, 60 } } },
        { module = 'world', name = 'world_card_run_load_step', label = 'load', before = {},
          budget = 3000, shot = 30, start = armLoad },
        { label = 'settle', frames = 600 },
    },
    check = function() return values end,
    expect = {
        original = { writes = 2, second_seek_at = 2 * SHORT, write_returned = IMAGE, file_found = 1, bytes_differing = 1878,
            load_accepted = 0 },
        dc = { writes = 2, second_seek_at = SHORT, write_returned = IMAGE, file_found = 1, bytes_differing = 0,
            load_accepted = 1 },
    },
}
