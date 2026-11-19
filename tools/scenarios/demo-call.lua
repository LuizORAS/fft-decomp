-- The demonstration of direct calls (emulator.call) and watches
-- (emulator.watch). Both discs give the same values but one: the Developer's
-- Cut's heap tag fix (QUIRKS.md) reads the cell after the new block to pick
-- its tag, so the reserve reads the cell table 9 times there, 8 on the
-- original. At the Gariland deployment screen, where BATTLE runs, every call
-- starts from BATTLE's thread_yield. MAIN's SMD heap reserves a block and
-- frees it while two watches count the reads and writes of its cell table
-- (the game heap's 64 cells all hold BATTLE's overlays there); VSync(-1)
-- returns the VBlank count; VSync(300), a wait of 300 frames, is abandoned by
-- a guard on v_wait's loop, and the game runs on (the count moves); the same
-- wait then outlasts its call's 30 frames.
local e = emulator
local BUTTON = e.BUTTON

-- v_wait's loop head, one poll of the VBlank count (v_wait's code is the same
-- on both discs); the 16 cells of the SMD heap's table; the block, one cell.
-- The reserve ends where the free, the next function, starts on that disc
-- (the fix makes it longer).
local V_WAIT_LOOP = 0x28
local CELLS = 16
local BLOCK_BYTES = 0x800

local anchor = e.symbol('battle', 'thread_yield')
local reserver = e.symbol('main', 'main_heap_alloc_smd')
local freer = e.symbol('main', 'main_heap_free_smd')
local vsync = e.symbol('main', 'VSync')
local values = {}
local cells, block, vcount = nil, nil, nil
local counting, reads, writes = false, 0, 0

local function yes(flag) return flag and 1 or 0 end

local function cellTable()
    local base = e.address('main', 'g_main_heap_smd_allocator_table')
    local bytes = {}
    for i = 0, CELLS - 1 do bytes[#bytes + 1] = string.format('%02x', e.read8(base + i)) end
    return table.concat(bytes)
end

-- The reserve's arguments, as the call begins: the table before it, and the
-- watches that count its accesses until the call ends.
local function reserve()
    local base = e.address('main', 'g_main_heap_smd_allocator_table')
    cells, counting = cellTable(), true
    e.watch(base, 'Read', CELLS, 'cell reads', function()
        if not counting then return false end
        reads = reads + 1
        return true
    end)
    e.watch(base, 'Write', CELLS, 'cell writes', function(where, pc)
        if not counting then return false end
        writes = writes + 1
        if writes == 1 then
            e.log('the first write: 0x%08x at pc 0x%08x', where, pc)
            values.first_write_in_table = yes(where >= base and where < base + CELLS)
            values.first_write_by_reserve = yes(pc >= reserver.address and pc < freer.address)
        end
        return true
    end)
    return { BLOCK_BYTES }
end

return {
    why = 'Demonstration: direct calls and watches; the heap tag fix adds one cell read on the DC',
    save = { file = 'smoke-new-game.mcd', sha256 = '02203c586fc996bd3c3dfa1fb07bb6e0025cb590b69b59dae955fc3b098c47e3' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'event-attack', name = 'attack_deploy_run_screen', label = 'overlay menu ATTACK.OUT deployment',
          before = { BUTTON.CIRCLE }, budget = 9000, shot = 90 },
    }),
    trigger = {
        e.call({ label = 'heap reserve', anchor = anchor, target = reserver, args = reserve,
            done = function(result)
                counting, block = false, result.v0
                values.reserve_returned = yes(result.returned)
                values.block_reserved = yes(result.returned and block ~= 0)
                values.cell_table_changed = yes(cellTable() ~= cells)
                values.cell_reads, values.cell_writes = reads, writes
            end }),
        e.call({ label = 'heap free', anchor = anchor, target = freer,
            args = function() return { block } end,
            done = function(result)
                values.free_returned = yes(result.returned)
                values.cell_table_restored = yes(cellTable() == cells)
            end }),
        e.call({ label = 'VBlank count', anchor = anchor, target = vsync, args = { -1 },
            done = function(result)
                vcount = result.v0
                values.vcount_returned = yes(result.returned)
            end }),
        e.call({ label = 'guarded wait', anchor = anchor, target = vsync, args = { 300 },
            guards = { { address = e.address('main', 'v_wait') + V_WAIT_LOOP, limit = 1000, label = 'v_wait loop' } },
            done = function(result)
                values.guard = result.guard or 'none'
                values.guard_hits = result.counts['v_wait loop']
            end }),
        e.call({ label = 'VBlank count after the guard', anchor = anchor, target = vsync, args = { -1 },
            done = function(result) values.game_runs_on = yes(result.returned and result.v0 > vcount) end }),
        e.call({ label = 'wait past its time', anchor = anchor, target = vsync, args = { 300 }, frames = 30,
            done = function(result) values.wait_timed_out = yes(result.timeout == 30) end }),
    },
    check = function() return values end,
    expect = {
        original = { reserve_returned = 1, block_reserved = 1, cell_table_changed = 1, cell_reads = 8, cell_writes = 1,
            first_write_in_table = 1, first_write_by_reserve = 1, free_returned = 1, cell_table_restored = 1,
            vcount_returned = 1, guard = 'v_wait loop', guard_hits = 1001, game_runs_on = 1, wait_timed_out = 1 },
        dc = { reserve_returned = 1, block_reserved = 1, cell_table_changed = 1, cell_reads = 9, cell_writes = 1,
            first_write_in_table = 1, first_write_by_reserve = 1, free_returned = 1, cell_table_restored = 1,
            vcount_returned = 1, guard = 'v_wait loop', guard_hits = 1001, game_runs_on = 1, wait_timed_out = 1 },
    },
}
