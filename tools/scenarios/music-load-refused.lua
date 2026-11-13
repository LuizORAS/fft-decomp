-- Retail bug of main_sound_start_music_load_into_free_slot and
-- main_sound_start_music_load_into_slot (QUIRKS.md): each allocates the SMD
-- block before it requests the read, and when the read is refused (a read
-- already in flight) it returns 0 without freeing the block, which stays
-- taken until a reset. No retail sequence is known to refuse it, so the calls
-- make one at one stop of WORLD's thread_yield at the formation screen (the
-- world-map save), where both scenario slots are full: slot 1 holds the world
-- map's music, slot 2 the formation's, which plays. Slot 1 is unloaded, a
-- seek with no destination keeps the read descriptor busy without writing
-- anything, and each loader asks for the Game Over music (one cell) and is
-- refused. A block a loader leaves is freed, never one that is not there:
-- that free becomes a request no heap holds, which writes nothing. Then
-- main_file_load_to_address with no destination waits for the seek, and
-- main_sound_wait_for_music_load reloads the world map's music into slot 1,
-- which ends with the same block, bytes and cell table as at the start. On
-- the original each refused load leaves one cell taken; on the Developer's
-- Cut none.
local e = emulator
local BUTTON = e.BUTTON

local FORMATION = { { menu = 4, row = 1, name = 'Formation' } }

-- Offsets in g_main_sound_music: state.current_slot, slots.music[0] and
-- slots.smd[0].
local CURRENT_SLOT = 0x00
local SLOT_MUSIC = 0x18
local SLOT_SMD = 0x34
-- The music ids of the Game Over screen and the world map, the slot the world
-- map's music holds, the SMD heap's cells and their bytes, the bytes of the
-- music compared after the reload, a request no heap holds and the offset of
-- thread_yield's second instruction.
local GAME_OVER = 50
local WORLD_MAP = 27
local SLOT = 1
local SMD_CELLS = 16
local CELL_BYTES = 0x800
local HEAD_BYTES = 256
local NO_ROOM = 0x10000000
local YIELD_SECOND_INSTRUCTION = 4

local anchor = e.symbol('world', 'thread_yield')
local alloc = e.symbol('main', 'main_heap_alloc')
local freeSmd = e.symbol('main', 'main_heap_free_smd')

local values = {}

local function music() return e.address('main', 'g_main_sound_music') end
local function smd(slot) return e.read32(music() + SLOT_SMD + 4 * (slot - 1)) % 0x100000000 end
local function cellTable() return e.address('main', 'g_main_heap_smd_allocator_table') end
local function busy() return e.read32(e.address('main', 'g_main_file_still_loading')) end
local function file(id)
    local at = e.address('main', 'g_main_sound_scenario_smd_files') + 8 * id
    return e.read32(at), e.read32(at + 4)
end
local function dump()
    local bytes = {}
    for i = 0, SMD_CELLS - 1 do bytes[#bytes + 1] = string.format('%02x', e.read8(cellTable() + i)) end
    return table.concat(bytes, ' ')
end
local function used()
    local n = 0
    for i = 0, SMD_CELLS - 1 do if e.read8(cellTable() + i) ~= 0 then n = n + 1 end end
    return n
end
local function head(address)
    local bytes = {}
    for i = 0, HEAD_BYTES - 1 do bytes[#bytes + 1] = string.format('%02x', e.read8(address + i)) end
    return table.concat(bytes)
end
local function yes(flag) return flag and 1 or 0 end
local function v0(result) return result.returned and result.v0 % 0x100000000 or 0 end

local start = {}
local before, leak = nil, 0

-- A loader asked for the Game Over music; the first cell it left taken is leak.
local function load(name, key, args, release)
    return { target = e.symbol('main', name), label = key .. ' load',
        args = function()
            before = dump()
            return args()
        end,
        done = function(result)
            values[key .. '_returned'] = v0(result)
            values[key .. '_cells_left'] = used() - values.used_after_unload
            leak = 0
            for i = SMD_CELLS - 1, 0, -1 do
                if e.read8(cellTable() + i) ~= 0 and tonumber(before:sub(3 * i + 1, 3 * i + 2), 16) == 0 then
                    leak = e.address('main', 'g_main_heap_smd_base') + i * CELL_BYTES
                end
            end
            e.log('%s: cells %s', key, dump())
            if leak == 0 then
                release.target, release.args, release.label = alloc, { NO_ROOM }, release.label .. ' (nothing to free)'
            end
        end }
end
local function release(key)
    return { target = freeSmd, label = 'free the ' .. key .. ' block', args = function() return { leak } end }
end

local freeRelease, slotRelease = release('free slot'), release('slot')
local calls = {
    { target = e.symbol('main', 'main_sound_unload_scenario_mus'), label = 'unload slot 1',
        args = function()
            start = { smd = smd(SLOT), head = head(smd(SLOT)), cells = dump() }
            values.current_slot = e.read32(music() + CURRENT_SLOT)
            return { SLOT }
        end,
        done = function(result)
            values.unloaded = v0(result)
            values.used_after_unload = used()
        end },
    { target = e.symbol('main', 'main_file_request_read_bytes'), label = 'seek with no destination',
        args = function()
            local sector = file(GAME_OVER)
            return { sector, CELL_BYTES, 0 }
        end,
        done = function(result)
            values.seek_returned = v0(result)
            values.busy_after_seek = busy()
        end },
    load('main_sound_start_music_load_into_free_slot', 'free_slot', function() return { GAME_OVER } end, freeRelease),
    freeRelease,
    load('main_sound_start_music_load_into_slot', 'into_slot', function() return { GAME_OVER, SLOT } end, slotRelease),
    slotRelease,
    { target = e.symbol('main', 'main_file_load_to_address'), label = 'wait for the seek',
        args = function()
            local sector = file(GAME_OVER)
            return { sector, 0, 0 }
        end,
        done = function() values.busy_at_end = busy() end },
    { target = e.symbol('main', 'main_sound_wait_for_music_load'), label = 'reload the world map music',
        args = { WORLD_MAP },
        done = function(result)
            values.reloaded_into = v0(result)
            values.reload_same_block = yes(smd(SLOT) == start.smd)
            values.reload_same_bytes = yes(head(smd(SLOT)) == start.head)
            values.reload_same_cells = yes(dump() == start.cells)
            e.log('after the reload: cells %s, slot 1 music 0x%08x', dump(),
                e.read32(music() + SLOT_MUSIC) % 0x100000000)
        end },
}

return {
    why = 'the music loaders leave the SMD block of a refused read (direct calls: no retail route is known)',
    save = { file = 'world-map.mcd', sha256 = 'adee3f05a445925002945e98791b6e8a461e5b3bc46ff28842c879f44243b945' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'wldcore', name = 'wldcore_entrypoint', label = 'world map WLDCORE', before = { BUTTON.CIRCLE },
          budget = 6000, shot = 120 },
        { module = 'world', name = 'world_formation_run_screen', label = 'WORLD menu formation screen', before = {},
          budget = 6000, shot = 90, start = function() e.openWorldMenu(FORMATION) end },
    }),
    trigger = {
        e.call({ label = 'music load', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' } } }),
    },
    check = function() return values end,
    expect = {
        original = { current_slot = 2, unloaded = 1, used_after_unload = 7, seek_returned = 0, busy_after_seek = 1,
            free_slot_returned = 0, free_slot_cells_left = 1, into_slot_returned = 0, into_slot_cells_left = 1,
            busy_at_end = 0, reloaded_into = 1, reload_same_block = 1, reload_same_bytes = 1, reload_same_cells = 1 },
        dc = { current_slot = 2, unloaded = 1, used_after_unload = 7, seek_returned = 0, busy_after_seek = 1,
            free_slot_returned = 0, free_slot_cells_left = 0, into_slot_returned = 0, into_slot_cells_left = 0,
            busy_at_end = 0, reloaded_into = 1, reload_same_block = 1, reload_same_bytes = 1, reload_same_cells = 1 },
    },
}
