-- Retail bug of battle_map_stop_clut_cycle (QUIRKS.md):
-- battle_map_start_clut_cycle returns the slot index + 1 as the handle, and
-- the stop clears the active byte of slot `handle`, not `handle - 1`, so it
-- stops the slot after the handle's own and handle 8 writes the byte after
-- the eight slots. No retail sequence shows it (the map's rotations stop at
-- its exit, and the next map clears every slot), so the calls make it at one
-- stop of BATTLE's thread_yield at Gariland's deployment screen (the
-- new-game save), where every slot is free: eight rotations of one color
-- (the first color is the last, so nothing rotates) take handles 1 to 8, then
-- each handle is stopped, with a watch on the writes of the bytes after the
-- slots. battle_map_reset_clut_cycles then frees the eight slots, which were
-- all free at the start. On the original slot 1 stays active and the stop of
-- handle 8 writes after the slots; on the Developer's Cut every slot stops
-- and nothing is written there.
local e = emulator
local BUTTON = e.BUTTON

-- The slots, their bytes, the rotation's frames, CLUT row and color, the
-- bytes after the slots that the watch covers, and the offset of
-- thread_yield's second instruction.
local SLOTS = 8
local SLOT_BYTES = 10
local FRAMES = 255
local ROW_MAP = 0
local COLOR = 15
local AFTER_BYTES = 2
local YIELD_SECOND_INSTRUCTION = 4

local anchor = e.symbol('battle', 'thread_yield')
local start = e.symbol('battle', 'battle_map_start_clut_cycle')
local stop = e.symbol('battle', 'battle_map_stop_clut_cycle')
local reset = e.symbol('battle', 'battle_map_reset_clut_cycles')

local values = {}
local handles = {}
local writes, watching = 0, false

local function slots() return e.address('battle', 'g_battle_map_clut_cycle_states') end
local function after() return e.address('battle', 'g_battle_gfx_cycled_deployed_palettes') end
-- One digit per slot: its active byte.
local function actives()
    local digits = {}
    for i = 0, SLOTS - 1 do digits[#digits + 1] = tostring(e.read8(slots() + SLOT_BYTES * i)) end
    return table.concat(digits)
end

local calls = {}
for i = 1, SLOTS do
    calls[#calls + 1] = { target = start, label = 'rotation ' .. i,
        args = function()
            if i == 1 then values.actives_at_start = actives() end
            return { FRAMES, ROW_MAP, COLOR, COLOR }
        end,
        done = function(result) handles[i] = result.returned and result.v0 % 0x100000000 or 0 end }
end
for i = 1, SLOTS do
    calls[#calls + 1] = { target = stop, label = 'stop handle ' .. i,
        args = function()
            if i == 1 then
                values.handles = table.concat(handles, ' ')
                values.actives_full = actives()
                watching = true
                e.watch(after(), 'Write', AFTER_BYTES, 'bytes after the slots', function(where, pc)
                    if not watching then return false end
                    writes = writes + 1
                    e.log('write 0x%08x after the slots at pc 0x%08x', where, pc)
                    return true
                end)
            end
            return { handles[i] }
        end,
        done = function()
            if i == SLOTS then
                watching = false
                values.writes_after_slots = writes
                values.actives_after_stops = actives()
            end
        end }
end
calls[#calls + 1] = { target = reset, label = 'free the slots',
    done = function() values.actives_at_end = actives() end }

return {
    why = 'battle_map_stop_clut_cycle stops the slot after the handle (direct calls: no retail route is known)',
    save = { file = 'smoke-new-game.mcd', sha256 = '02203c586fc996bd3c3dfa1fb07bb6e0025cb590b69b59dae955fc3b098c47e3' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'event-attack', name = 'attack_deploy_run_screen', label = 'overlay menu ATTACK.OUT deployment',
          before = { BUTTON.CIRCLE }, budget = 9000, shot = 90 },
    }),
    trigger = {
        e.call({ label = 'clut cycles', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' } } }),
    },
    check = function() return values end,
    expect = {
        original = { actives_at_start = '00000000', handles = '1 2 3 4 5 6 7 8', actives_full = '11111111',
            actives_after_stops = '10000000', writes_after_slots = 1, actives_at_end = '00000000' },
        dc = { actives_at_start = '00000000', handles = '1 2 3 4 5 6 7 8', actives_full = '11111111',
            actives_after_stops = '00000000', writes_after_slots = 0, actives_at_end = '00000000' },
    },
}
