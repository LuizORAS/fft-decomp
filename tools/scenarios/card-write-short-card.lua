-- Retail bug of card_file_write_buffer (QUIRKS.md): before each of its 40
-- attempts it seeks by the bytes written so far from the current position
-- (lseek origin 1), not from the start, so after a short first write of n
-- bytes the rest of the save lands at offset 2n. A card fault that cuts a
-- write short is not something the emulator does, so the BIOS write() is asked
-- for SHORT bytes on the first attempt; the runs use OpenBIOS, and the
-- console's BIOS was not tested. CARD.OUT's save screen comes after the first
-- battle of a new game (the route of make smoke's new-game session, Wait on
-- Ramza's turns; about 42000 frames), on a card that holds the world-map save
-- in File 01; the save goes to File 02, which is empty and asks for no
-- overwrite. The card file is then compared with the image the game wrote.
-- On the original the second seek lands at 2n, the game reports the save
-- written and the file differs from the image; on the Developer's Cut the
-- second seek lands at n and the file equals the image. The run writes only the
-- copy of the card in its own directory.
local bit = require('bit')
local e = emulator
local BUTTON = e.BUTTON
local run = e.run

-- The first write's bytes, the save image's bytes, the turn menu row Orbonne's
-- turns choose, the card's block and directory frame bytes, and the frames the
-- card file needs to reach memcard1.mcd after the write.
local SHORT = 0x100
local IMAGE = 0x1e00
local WAIT = { { row = 2, name = 'Wait' } }
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
    local fn = e.symbol('event-card', 'card_file_write_buffer')
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

return {
    why = 'a save written after a short first write is misplaced on the card (forced fault: no card fault in the emulator)',
    save = { file = 'world-map.mcd', sha256 = 'adee3f05a445925002945e98791b6e8a461e5b3bc46ff28842c879f44243b945' },
    setup = {
        e.title(),
        { module = 'world', name = 'world_name_run_entry_screen', label = 'new game WORLD name entry run by OPEN',
          before = { BUTTON.START }, budget = 9000, shot = 60 },
        { module = 'world', name = 'world_name_run_confirmation_menu', before = { BUTTON.START }, budget = 1800 },
        { module = 'battle', name = 'battle_state_run_game_loop', label = 'first battle BATTLE main loop',
          before = { BUTTON.CIRCLE, BUTTON.START }, budget = 9000, shot = 240 },
        { label = 'turns', start = function(passed)
              e.playTurns(WAIT)
              passed()
          end },
        { module = 'event-card', name = 'card_save_run_menu', label = 'overlay menu CARD.OUT save screen',
          before = { BUTTON.CIRCLE }, budget = 50000, shot = 60 },
    },
    trigger = {
        -- Circle through the prompt and card slot 1 to the file list, then File 02.
        { module = 'event-card', name = 'card_save_update_and_draw_slot_list', label = 'save to File 02',
          before = { BUTTON.CIRCLE }, budget = 3000, shot = 30,
          after = { { nil, 60 }, { BUTTON.DOWN, 40 }, { nil, 40 }, { BUTTON.CIRCLE, 40 } } },
        { module = 'event-card', name = 'card_save_update_slot_write', label = 'save', before = {},
          budget = 3000, shot = 30, start = armWrite },
        { label = 'card file', budget = CARD_FLUSH + 60, start = compare },
    },
    check = function() return values end,
    expect = {
        original = { writes = 2, second_seek_at = 2 * SHORT, write_returned = IMAGE, file_found = 1, bytes_differing = 1822 },
        dc = { writes = 2, second_seek_at = SHORT, write_returned = IMAGE, file_found = 1, bytes_differing = 0 },
    },
}
