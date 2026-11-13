-- Retail bug of battle_gfx_append_unit_graphics_load_descriptor (QUIRKS.md):
-- it writes its record before it tests the count, which stops at 16, so a
-- seventeenth append writes the record one past the 16-record queue, on the
-- head of g_main_zodiac_primitives. Every load pass empties the queue first
-- and a battle holds at most 16 units, so no retail sequence does it; the
-- calls make it at one stop of BATTLE's thread_yield at Gariland's deployment
-- screen (the new-game save), where the queue is empty: 17 appends, with a
-- watch on the writes of the record's bytes past the queue. The next load
-- pass empties the queue again. On the original the seventeenth append
-- writes past the queue and changes those bytes; on the Developer's Cut it
-- writes nothing. Both keep 16 records, the last from the sixteenth append.
local e = emulator
local BUTTON = e.BUTTON

-- The queue's records, one record's bytes, the appends, the offset of the
-- records in the queue and the offset of thread_yield's second instruction.
local RECORDS = 16
local RECORD_BYTES = 20
local APPENDS = 17
local FIRST_RECORD = 4
local YIELD_SECOND_INSTRUCTION = 4

local anchor = e.symbol('battle', 'thread_yield')
local append = e.symbol('battle', 'battle_gfx_append_unit_graphics_load_descriptor')

local values = {}
local writes, watching, before = 0, false, nil

local function queue() return e.address('main', 'g_main_gfx_unit_graphics_load_queue') end
local function past() return e.address('main', 'g_main_zodiac_primitives') end
local function count() return e.read32(queue()) end
local function pastBytes()
    local bytes = {}
    for i = 0, RECORD_BYTES - 1 do bytes[#bytes + 1] = string.format('%02x', e.read8(past() + i)) end
    return table.concat(bytes)
end

local calls = {}
for i = 1, APPENDS do
    calls[#calls + 1] = { target = append, label = 'append ' .. i,
        args = function()
            if i == 1 then
                values.count_at_start = count()
                before = pastBytes()
                watching = true
                e.watch(past(), 'Write', RECORD_BYTES, 'record past the queue', function(where, pc)
                    if not watching then return false end
                    writes = writes + 1
                    return true
                end)
            end
            -- map x (the append's number), map y, level, facing, sheet, palette,
            -- runtime id, no unit, undrawn.
            return { i, 2, 0, 0, 1, 0, i, 0, 1 }
        end,
        done = function()
            if i == RECORDS then values.count_after_16 = count() end
            if i == APPENDS then
                watching = false
                values.count_after_17 = count()
                values.last_record_x = e.read8(queue() + FIRST_RECORD + RECORD_BYTES * (RECORDS - 1))
                values.writes_past_queue = writes
                values.past_bytes_changed = pastBytes() ~= before and 1 or 0
            end
        end }
end

return {
    why = 'a seventeenth append writes past the unit graphics load queue (direct calls: no retail route is known)',
    save = { file = 'smoke-new-game.mcd', sha256 = '02203c586fc996bd3c3dfa1fb07bb6e0025cb590b69b59dae955fc3b098c47e3' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'event-attack', name = 'attack_deploy_run_screen', label = 'overlay menu ATTACK.OUT deployment',
          before = { BUTTON.CIRCLE }, budget = 9000, shot = 90 },
    }),
    trigger = {
        e.call({ label = 'load queue', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' } } }),
    },
    check = function() return values end,
    expect = {
        original = { count_at_start = 0, count_after_16 = 16, count_after_17 = 16, last_record_x = 16,
            writes_past_queue = 9, past_bytes_changed = 1 },
        dc = { count_at_start = 0, count_after_16 = 16, count_after_17 = 16, last_record_x = 16,
            writes_past_queue = 0, past_bytes_changed = 0 },
    },
}
