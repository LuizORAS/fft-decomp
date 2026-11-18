-- Retail bug of world_menu_script_draw_plain_number (QUIRKS.md), which
-- BUNIT's and EQUIP's copies share: it counts its blank cells down to -1 in an
-- s16 with no lower bound, so a value with more digits than its field starts
-- below -1 and queues a blank quad about 65,535 times, past the quad pool. No
-- retail field is known to be too narrow, so calls make it at one stop of
-- WORLD's thread_yield at the formation screen (the world-map save): a
-- plain-number command written into g_world_menu_entry_ids (put back after
-- the calls), whose getter is a row callback slot left empty, so the value is
-- the command's own byte. 42 in a field of 5 cells takes 3 blank cells on
-- both discs; 200 in a field of 1 cell runs away on the original until a guard
-- of 16 blank quads abandons the call, and on the Developer's Cut it takes no
-- blank cell and prints its 3 digits.
local e = emulator
local BUTTON = e.BUTTON

-- WORLD's menu row Formation, the offset of thread_yield's second
-- instruction, the plain-number opcode, a command's bytes, the row callback
-- slots, the blank quads a call may queue and the digit quads it may queue.
local FORMATION = { { menu = 4, row = 1, name = 'Formation' } }
local YIELD_SECOND_INSTRUCTION = 4
local PLAIN_NUMBER = 0x17
local COMMAND_BYTES = 10
local CALLBACKS = 16
local BLANK_LIMIT = 16
local DIGIT_LIMIT = 64

local anchor = e.symbol('world', 'thread_yield')
local draw = e.symbol('world', 'world_menu_script_draw_plain_number')
local blank = e.symbol('world', 'world_gfx_enqueue_oriented_textured_quad')
local digit = e.symbol('world', 'world_gfx_enqueue_textured_quad')

local values = {}
local saved = {}
local command, getter

-- menu_draw_number_command_t: opcode, length, style, getter, width, spacing,
-- alternate palette, x, y, value.
local function drawNumber(label, width, value, done)
    return { target = draw, label = label,
        args = function()
            if not command then
                command = e.address('world', 'g_world_menu_entry_ids')
                for i = 0, COMMAND_BYTES - 1 do saved[i] = e.read8(command + i) end
                local callbacks = e.address('world', 'g_world_menu_script_callbacks')
                for slot = 0, CALLBACKS - 1 do
                    if e.read32(callbacks + 4 * slot) == 0 then getter = slot; break end
                end
                if not getter then error('no empty row callback slot') end
            end
            local bytes = { PLAIN_NUMBER, COMMAND_BYTES, 0, getter, width, 0, 0, 16, 16, value }
            for i, b in ipairs(bytes) do e.write8(command + i - 1, b) end
            return { command }
        end,
        done = function(result)
            values[label .. '_returned'] = result.returned and 1 or 0
            values[label .. '_blank_quads'] = result.counts['blank quad']
            values[label .. '_digit_quads'] = result.counts['digit quad']
            if done then done() end
        end }
end

local calls = {
    drawNumber('fits', 5, 42),
    drawNumber('wider', 1, 200, function()
        for i = 0, COMMAND_BYTES - 1 do e.write8(command + i, saved[i]) end
    end),
}

return {
    why = 'a plain number wider than its field queues blank quads without end (direct calls: no retail field is known to be too narrow)',
    save = { file = 'world-map.mcd', sha256 = 'adee3f05a445925002945e98791b6e8a461e5b3bc46ff28842c879f44243b945' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'wldcore', name = 'wldcore_entrypoint', label = 'world map WLDCORE', before = { BUTTON.CIRCLE },
          budget = 6000, shot = 90 },
        { module = 'world', name = 'world_formation_run_screen', label = 'WORLD menu formation screen', before = {},
          budget = 6000, shot = 90, start = function() e.openWorldMenu(FORMATION) end },
    }),
    trigger = {
        e.call({ label = 'plain numbers', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' },
                       { address = blank.address, limit = BLANK_LIMIT, label = 'blank quad' },
                       { address = digit.address, limit = DIGIT_LIMIT, label = 'digit quad' } } }),
    },
    check = function() return values end,
    expect = {
        original = { fits_returned = 1, fits_blank_quads = 3, fits_digit_quads = 2,
            wider_returned = 0, wider_blank_quads = BLANK_LIMIT + 1, wider_digit_quads = 0 },
        dc = { fits_returned = 1, fits_blank_quads = 3, fits_digit_quads = 2,
            wider_returned = 1, wider_blank_quads = 0, wider_digit_quads = 3 },
    },
}
