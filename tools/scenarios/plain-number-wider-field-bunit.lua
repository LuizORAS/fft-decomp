-- Retail bug of bunit_menu_script_draw_plain_number (QUIRKS.md), which
-- WORLD's and EQUIP's copies share: it counts its blank cells down to -1 in an
-- s16 with no lower bound, so a value with more digits than its field starts
-- below -1 and queues a blank quad about 65,535 times, past the quad pool. No
-- retail field is known to be too narrow, and the unit list draws its numbers
-- with the formatted-number record on every tab, so calls make it at one stop
-- of BATTLE's thread_yield on BUNIT.OUT's unit list. The route is the
-- Gariland battle (the new-game save): on the first turn menu Cross frees the
-- cursor, Triangle opens the mini menu and its Unit List row opens BUNIT. A
-- plain-number command is written over the formatted-number record the list
-- has just drawn (put back after the calls); its getter is the row callback
-- the list installs for Brave, and its value the first listed unit, Ramza,
-- with Brave 70. In a field of 5 cells it takes 3 blank cells on both discs;
-- in a field of 1 cell it runs away on the original until a guard of 16 blank
-- quads abandons the call, and on the Developer's Cut it takes no blank cell
-- and prints its 2 digits.
local e = emulator
local BUTTON = e.BUTTON
local run = e.run

-- The mini menu's Unit List row, the offset of thread_yield's second
-- instruction, the plain-number opcode, a command's bytes, the row callback
-- slots, the first listed unit, the blank quads a call may queue and the
-- digit quads it may queue.
local UNIT_LIST = { { menu = 10, row = 1, name = 'Unit List' } }
local YIELD_SECOND_INSTRUCTION = 4
local PLAIN_NUMBER = 0x17
local COMMAND_BYTES = 10
local CALLBACKS = 16
local FIRST_UNIT = 0
local BLANK_LIMIT = 16
local DIGIT_LIMIT = 64

local anchor = e.symbol('battle', 'thread_yield')
local draw = e.symbol('event-bunit', 'bunit_menu_script_draw_plain_number')
local formatted = e.symbol('event-bunit', 'bunit_menu_script_draw_formatted_number')
local blank = e.symbol('event-bunit', 'menu_gfx_enqueue_oriented_textured_quad')
local digit = e.symbol('event-bunit', 'menu_gfx_enqueue_textured_quad')

local values = {}
local saved = {}
local command, getter

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

-- The next formatted-number record the list draws.
local function findRecord(passed)
    e.breakAt(formatted.address, 'formatted number', function()
        if not e.resident(formatted) then return true end
        command = tonumber(PCSX.getRegisters().GPR.n.a0)
        passed()
        return false
    end)
end

-- menu_draw_number_command_t: opcode, length, style, getter, width, spacing,
-- alternate palette, x, y, value.
local function drawNumber(label, width, done)
    return { target = draw, label = label,
        args = function()
            if not getter then
                for i = 0, COMMAND_BYTES - 1 do saved[i] = e.read8(command + i) end
                local callbacks = e.address('event-bunit', 'g_menu_row_callbacks')
                local brave = e.address('event-bunit', 'bunit_unit_get_brave')
                for slot = 0, CALLBACKS - 1 do
                    if e.read32(callbacks + 4 * slot) == brave then getter = slot; break end
                end
                if not getter then error('no Brave row callback') end
            end
            local bytes = { PLAIN_NUMBER, COMMAND_BYTES, 0, getter, width, 0, 0, 16, 16, FIRST_UNIT }
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
    drawNumber('fits', 5),
    drawNumber('wider', 1, function()
        for i = 0, COMMAND_BYTES - 1 do e.write8(command + i, saved[i]) end
    end),
}

return {
    why = 'a plain number wider than its field queues blank quads without end (direct calls: no retail field is known to be too narrow)',
    save = { file = 'smoke-new-game.mcd', sha256 = '02203c586fc996bd3c3dfa1fb07bb6e0025cb590b69b59dae955fc3b098c47e3' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'event-attack', name = 'attack_deploy_run_screen', label = 'overlay menu ATTACK.OUT deployment',
          before = { BUTTON.CIRCLE }, budget = 9000, shot = 90 },
        { module = 'event-bunit', name = 'bunit_menu_run_display_script', label = 'overlay menu BUNIT.OUT unit list',
          before = { BUTTON.CIRCLE }, budget = 40000, shot = 90, start = openUnitList },
    }),
    trigger = {
        { label = 'formatted number', budget = 300, start = findRecord },
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
            wider_returned = 1, wider_blank_quads = 0, wider_digit_quads = 2 },
    },
}
