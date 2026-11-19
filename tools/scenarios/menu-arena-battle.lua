-- Retail bugs of battle_menu_alloc_memory (QUIRKS.md): it tests the end of
-- the menu arena before the size, so a request that needs the last page
-- waits forever, and it marks pages before claiming one of the 16 records,
-- so a request made while all 16 are taken leaves its pages marked with no
-- owner. No retail route is known to either, so the calls make them at one
-- stop of BATTLE's thread_yield at the Gariland deployment, where the arena
-- is empty and every record free: a request for the whole arena, then 16
-- one-page blocks and one more page. A request that waits is
-- abandoned there by the guard on thread_yield's second instruction, which
-- a call's own return to the anchor never reaches. Everything the calls took
-- goes back through battle_menu_free_memory. On the original the whole-arena
-- request waits and the last request leaves one page marked after every
-- block is freed; on the Developer's Cut the whole arena is returned and
-- nothing stays marked.
local e = emulator
local BUTTON = e.BUTTON

local M = { module = 'battle', alloc = 'battle_menu_alloc_memory', free = 'battle_menu_free_memory',
    map = 'g_battle_menu_buffer_allocation_map', arena = 'g_battle_menu_buffer_arena',
    records = 'g_battle_menu_allocator_slot_records', pages = 53 }

-- The allocation records, their size and free mark, the arena's page size and
-- the offset of thread_yield's second instruction.
local RECORDS = 16
local RECORD_BYTES = 8
local NO_BUFFER = 0xffffffff
local PAGE_BYTES = 256
local YIELD_SECOND_INSTRUCTION = 4

local anchor = e.symbol(M.module, 'thread_yield')
local alloc = e.symbol(M.module, M.alloc)
local release = e.symbol(M.module, M.free)

local values = {}
local whole, blocks = {}, {}

local function yes(flag) return flag and 1 or 0 end
local function arena() return e.address(M.module, M.arena) end
local function marked(p) return e.read8(e.address(M.module, M.map) + p) ~= 0 end
local function record(i)
    local at = e.address(M.module, M.records) + RECORD_BYTES * i
    return e.read32(at), e.read32(at + 4)
end
local function usedPages()
    local n = 0
    for p = 0, M.pages - 1 do if marked(p) then n = n + 1 end end
    return n
end
local function freeRecords()
    local n = 0
    for i = 0, RECORDS - 1 do if record(i) == NO_BUFFER then n = n + 1 end end
    return n
end
-- Pages the map marks that no record's run covers.
local function unowned()
    local covered = {}
    for i = 0, RECORDS - 1 do
        local buffer, count = record(i)
        if buffer ~= NO_BUFFER then
            local first = (buffer - arena()) / PAGE_BYTES
            for p = first, first + count - 1 do covered[p] = true end
        end
    end
    local n = 0
    for p = 0, M.pages - 1 do if marked(p) and not covered[p] then n = n + 1 end end
    return n
end
local function outcome(result)
    if result.returned then return 'returned' end
    return result.guard and 'waits' or 'timeout'
end

-- A request of pages pages; a returned block joins held.
local function allocate(pages, label, held, args, done)
    return { target = alloc, label = label,
        args = function()
            if args then args() end
            return { pages * PAGE_BYTES }
        end,
        done = function(result)
            if result.returned then held[#held + 1] = result.v0 % 0x100000000 end
            if done then done(result) end
        end }
end
-- Frees the kth block of held (-1, which the routine ignores, when there is none).
local function free(held, k, done)
    return { target = release, label = 'free',
        args = function() return { held[k] or NO_BUFFER } end,
        done = done }
end

local calls = {
    allocate(M.pages, 'whole arena request', whole, function()
        values.start_used_pages = usedPages()
        values.start_free_records = freeRecords()
    end, function(result)
        values.arena_request = outcome(result)
        values.arena_request_at_start = yes(result.returned and whole[1] == arena())
    end),
    free(whole, 1, function()
        values.arena_freed = yes(usedPages() == 0 and freeRecords() == RECORDS)
    end),
}
for i = 1, RECORDS do calls[#calls + 1] = allocate(1, 'one-page block ' .. i, blocks) end
calls[#calls + 1] = allocate(1, 'request with every record taken', blocks, nil, function(result)
    values.full_request = outcome(result)
    values.unowned_after_full_request = unowned()
end)
for k = 1, RECORDS + 1 do
    calls[#calls + 1] = free(blocks, k, k == RECORDS + 1 and function()
        values.unowned_after_release = unowned()
        values.map_restored = yes(usedPages() == 0)
        values.records_restored = yes(freeRecords() == RECORDS)
    end or nil)
end

return {
    why = 'battle_menu_alloc_memory never uses the last page and leaves pages unowned (direct calls: no retail route is known)',
    save = { file = 'smoke-new-game.mcd', sha256 = '02203c586fc996bd3c3dfa1fb07bb6e0025cb590b69b59dae955fc3b098c47e3' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'event-attack', name = 'attack_deploy_run_screen', label = 'overlay menu ATTACK.OUT deployment',
          before = { BUTTON.CIRCLE }, budget = 9000, shot = 90 },
    }),
    trigger = {
        e.call({ label = 'menu arena', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' } } }),
    },
    check = function() return values end,
    expect = {
        original = { start_used_pages = 0, start_free_records = 16, arena_request = 'waits', arena_request_at_start = 0,
            arena_freed = 1, full_request = 'waits', unowned_after_full_request = 1, unowned_after_release = 1,
            map_restored = 0, records_restored = 1 },
        dc = { start_used_pages = 0, start_free_records = 16, arena_request = 'returned', arena_request_at_start = 1,
            arena_freed = 1, full_request = 'waits', unowned_after_full_request = 0, unowned_after_release = 0,
            map_restored = 1, records_restored = 1 },
    },
}
