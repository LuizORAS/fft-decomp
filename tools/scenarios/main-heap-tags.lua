-- Retail bugs of main_heap_alloc, main_heap_alloc_smd and main_heap_reserve_at
-- (QUIRKS.md): the allocators tag a block one above the highest tag before
-- it, so a block can take the tag of the block right after it, and freeing it
-- frees that block too; main_heap_reserve_at checks only where a range
-- starts, so a range past the last cell tags the bytes after the cell table.
-- No retail sequence is known to do either, so the calls make them at one
-- stop of WORLD's thread_yield at the formation screen (the world-map save),
-- where the game heap is empty and the music holds the SMD heap's first 8
-- cells. On each heap blocks A and B fill the free cells, A is freed, C and D
-- fill A's cells (QUIRKS.md's 62, 2, free, 10, 52 on the game heap; 6, 2,
-- free, 1, 5 on the SMD heap), then D, B and C are freed. Then 4 cells are
-- reserved at the game heap's cell 62, with a watch on the reads of the bytes
-- after its table, and freed. A block not returned is never freed: its free
-- becomes a request no heap holds, which writes nothing. On the original D
-- takes B's tag, D's free clears B's cells too and B's own free returns 0,
-- and the reservation reads the 2 bytes after the table, tags them and
-- succeeds; on the Developer's Cut D takes the next tag, B stays until its own
-- free, and the reservation is refused with nothing read or written. Both
-- heaps end as they began on either disc.
local e = emulator
local BUTTON = e.BUTTON

local FORMATION = { { menu = 4, row = 1, name = 'Formation' } }

-- A heap cell's bytes, a request no heap holds, the bytes after the game
-- heap's table that the checks read, and the offset of thread_yield's second
-- instruction.
local CELL_BYTES = 0x800
local NO_ROOM = 0x10000000
local PAST_TABLE = 4
local YIELD_SECOND_INSTRUCTION = 4

local anchor = e.symbol('world', 'thread_yield')
local alloc = e.symbol('main', 'main_heap_alloc')
local reserve = e.symbol('main', 'main_heap_reserve_at')

local HEAPS = {
    game = { cells = 64, alloc = alloc, free = e.symbol('main', 'main_heap_free'),
        table = function() return e.address('main', 'g_main_heap_game_allocator_table') end,
        base = function() return e.read32(e.address('main', 'g_main_heap_high_overlay_load_address')) end,
        sizes = { a = 62, b = 2, c = 10, d = 52 } },
    smd = { cells = 16, alloc = e.symbol('main', 'main_heap_alloc_smd'), free = e.symbol('main', 'main_heap_free_smd'),
        table = function() return e.address('main', 'g_main_heap_smd_allocator_table') end,
        base = function() return e.address('main', 'g_main_heap_smd_base') end,
        sizes = { a = 6, b = 2, c = 1, d = 5 } },
}

local values = {}

local function yes(flag) return flag and 1 or 0 end
local function v0(result) return result.returned and result.v0 % 0x100000000 or 0 end
-- The heap's cell tags and the extra bytes after them.
local function dump(h, extra)
    local bytes = {}
    for i = 0, h.cells - 1 + (extra or 0) do bytes[#bytes + 1] = string.format('%02x', e.read8(h.table() + i)) end
    return table.concat(bytes, ' ')
end
local function used(h)
    local n = 0
    for i = 0, h.cells - 1 do if e.read8(h.table() + i) ~= 0 then n = n + 1 end end
    return n
end
local function cellOf(h, address) return math.floor((address - h.base()) / CELL_BYTES) end
-- A free whose block was not returned asks for no room instead.
local function skip(op) op.target, op.args, op.label = alloc, { NO_ROOM }, op.label .. ' (nothing to free)' end

-- The heap's sequence: blocks A and B, free A, blocks C and D, free D, B and C.
local function sequence(name)
    local h = HEAPS[name]
    local held, releases, cells, tags, frees, start = {}, {}, {}, {}, {}, nil
    local function block(key)
        return { target = h.alloc, label = name .. ' block ' .. key,
            args = function()
                if key == 'a' then
                    start = dump(h)
                    values[name .. '_used_at_start'] = used(h)
                end
                return { h.sizes[key] * CELL_BYTES }
            end,
            done = function(result)
                held[key] = v0(result)
                if held[key] == 0 then
                    skip(releases[key])
                    cells[#cells + 1], tags[#tags + 1] = key .. '-', key .. '-'
                else
                    local cell = cellOf(h, held[key])
                    cells[#cells + 1] = key .. cell
                    tags[#tags + 1] = key .. e.read8(h.table() + cell)
                end
            end }
    end
    local function release(key, done)
        local op
        op = { target = h.free, label = name .. ' free ' .. key,
            args = function() return { held[key] } end,
            done = function(result)
                frees[#frees + 1] = key .. (op.target == h.free and v0(result) or '-')
                if done then done() end
            end }
        releases[key] = op
        return op
    end
    return {
        block('a'), block('b'), release('a'), block('c'), block('d'),
        release('d', function()
            values[name .. '_used_after_d_free'] = used(h)
            e.log('%s heap after D is freed: %s', name, dump(h))
        end),
        release('b'),
        release('c', function()
            values[name .. '_cells'] = table.concat(cells, ' ')
            values[name .. '_tags'] = table.concat(tags, ' ')
            values[name .. '_frees'] = table.concat(frees, ' ')
            values[name .. '_restored'] = yes(dump(h) == start)
        end),
    }
end

-- 4 cells reserved at the game heap's cell 62, then freed.
local function reservation()
    local h = HEAPS.game
    local reads, watching, before, past, got = 0, false, nil, nil, 0
    local function after() return e.read32(h.table() + h.cells) end
    local free = { target = h.free, label = 'free the reservation',
        args = function() return { got } end,
        done = function() values.reserve_restored = yes(dump(h, PAST_TABLE) == before) end }
    return {
        { target = reserve, label = 'reserve 4 cells at cell 62',
            args = function()
                before, past = dump(h, PAST_TABLE), after()
                watching = true
                e.watch(h.table() + h.cells, 'Read', PAST_TABLE, 'bytes after the game heap table', function(where, pc)
                    if not watching then return false end
                    reads = reads + 1
                    e.log('read 0x%08x after the table at pc 0x%08x', where, pc)
                    return true
                end)
                return { h.base() + (h.cells - 2) * CELL_BYTES, 4 * CELL_BYTES }
            end,
            done = function(result)
                watching, got = false, v0(result)
                values.reserve_returned = yes(got ~= 0)
                values.reserve_reads_past_table = reads
                values.reserve_writes_past_table = yes(after() ~= past)
                e.log('game heap after the reservation: %s', dump(h, PAST_TABLE))
                if got == 0 then skip(free) end
            end },
        free,
    }
end

local calls = e.join(sequence('game'), sequence('smd'), reservation())

return {
    why = 'main_heap_alloc gives a block the next block\'s tag and main_heap_reserve_at reserves past the table (direct calls: no retail route is known)',
    save = { file = 'world-map.mcd', sha256 = 'adee3f05a445925002945e98791b6e8a461e5b3bc46ff28842c879f44243b945' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'wldcore', name = 'wldcore_entrypoint', label = 'world map WLDCORE', before = { BUTTON.CIRCLE },
          budget = 6000, shot = 120 },
        { module = 'world', name = 'world_formation_run_screen', label = 'WORLD menu formation screen', before = {},
          budget = 6000, shot = 90, start = function() e.openWorldMenu(FORMATION) end },
    }),
    trigger = {
        e.call({ label = 'heap tags', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' } } }),
    },
    check = function() return values end,
    expect = {
        original = { game_used_at_start = 0, game_cells = 'a0 b62 c0 d10', game_tags = 'a1 b2 c1 d2',
            game_used_after_d_free = 10, game_frees = 'a1 d1 b0 c1', game_restored = 1,
            smd_used_at_start = 8, smd_cells = 'a8 b14 c8 d9', smd_tags = 'a8 b9 c8 d9',
            smd_used_after_d_free = 9, smd_frees = 'a1 d1 b0 c1', smd_restored = 1,
            reserve_returned = 1, reserve_reads_past_table = 2, reserve_writes_past_table = 1, reserve_restored = 1 },
        dc = { game_used_at_start = 0, game_cells = 'a0 b62 c0 d10', game_tags = 'a1 b2 c1 d3',
            game_used_after_d_free = 12, game_frees = 'a1 d1 b1 c1', game_restored = 1,
            smd_used_at_start = 8, smd_cells = 'a8 b14 c8 d9', smd_tags = 'a8 b9 c8 d10',
            smd_used_after_d_free = 11, smd_frees = 'a1 d1 b1 c1', smd_restored = 1,
            reserve_returned = 0, reserve_reads_past_table = 0, reserve_writes_past_table = 0, reserve_restored = 1 },
    },
}
