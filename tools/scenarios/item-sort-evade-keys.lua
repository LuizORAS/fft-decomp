-- Retail bugs of world_item_sort_id_list (QUIRKS.md), which EQUIP's
-- equip_item_sort_list_by_criteria shares the first of:
-- - the evade keys read g_main_item_shield_data for ids 0x7a..0x8f and
--   g_main_item_accessory_data for 0x90..0xef by the item's secondary data id,
--   so throwables, headgear and body armor read unrelated bytes. The evade
--   keys follow type, price and ranking in every key list, so they only break
--   ties: among headgear and armor only Cachusha and Barette tie;
-- - an unpaid fitting-room item carries WORLD_EQUIPMENT_SHOP_ITEM (0x8000) in a
--   unit's equipment, and counting the units' equipment by id then increments
--   a byte 32 KB past the 256-byte count array.
-- No retail screen sorts these lists while the scenario can watch them, so
-- calls make it at one stop of WORLD's thread_yield at the formation screen
-- (the world-map save): each slot list that holds such items (headgear, body
-- armor, throwables) sorted by the physical-evade key list (1) and the
-- magic-evade one (0), with a watch on the reads of the table the items do not
-- belong to; then the headgear list again with the first unit's first item
-- marked unpaid, with a watch on the byte its count reaches. The list and the
-- equipment are put back after the calls. On the original the headgear sorts
-- read the accessory table 10 times each and the unpaid item's count lands
-- past the array; on the Developer's Cut neither happens. Both discs give the
-- same order with this save's inventory.
local bit = require('bit')
local e = emulator
local BUTTON = e.BUTTON

-- WORLD's menu row Formation, the offset of thread_yield's second
-- instruction, an item record's bytes and its secondary data id, the first
-- equipment of a formation record, the unpaid flag, the item marked unpaid,
-- the list's bytes, the key lists and the item lists.
local FORMATION = { { menu = 4, row = 1, name = 'Formation' } }
local YIELD_SECOND_INSTRUCTION = 4
local ITEM_BYTES = 12
local SECONDARY_ID = 4
local EQUIPMENT = 0x54
local SHOP_ITEM = 0x8000
local LEATHER_HELMET = 0x90
local LIST_BYTES = 0x200
local MAGIC_EVADE_KEYS, PHYSICAL_EVADE_KEYS = 0, 1
local LISTS = {
    headgear = { first = 0x90, last = 0xaf, table = 'g_main_item_accessory_data' },
    armor = { first = 0xb0, last = 0xcf, table = 'g_main_item_accessory_data' },
    throwables = { first = 0x7a, last = 0x7f, table = 'g_main_item_shield_data' },
}
local HEADGEAR_ORDER = '9a 99 98 97 96 95 94 93 92 91 90 9c 9b a8 a7 a6 a5 a4 a3 a2 a1 a0 9f 9e 9d ab aa a9 af ae ad ac'

local anchor = e.symbol('world', 'thread_yield')
local sort = e.symbol('world', 'world_item_sort_id_list')
local memset = e.symbol('main', 'memset')

local values = {}
local saved = {}
local watched = {}
local watching, wrongReads, farWrites = false, 0, 0
local listAt, unit

local function secondaryId(item)
    return e.read8(e.address('main', 'g_main_item_primary_data') + ITEM_BYTES * item + SECONDARY_ID)
end

local function order(count)
    local ids = {}
    for i = 0, count - 1 do ids[#ids + 1] = string.format('%02x', bit.band(e.read16(listAt + 2 * i), 0x3ff)) end
    return table.concat(ids, ' ')
end

-- A watch on the record of the other table at each item's secondary data id.
local function watchWrongTable(name)
    if watched[name] then return end
    watched[name] = true
    local list = LISTS[name]
    local base = e.address('main', list.table)
    local seen = {}
    for item = list.first, list.last do
        local at = base + 2 * secondaryId(item)
        if not seen[at] then
            seen[at] = true
            e.watch(at, 'Read', 2, name .. ' in ' .. list.table, function(where, pc)
                if not watching then return false end
                wrongReads = wrongReads + 1
                return true
            end)
        end
    end
end

local function sortList(label, name, keys, prepare, done)
    return { target = sort, label = label,
        args = function()
            if not listAt then
                listAt = e.address('world', 'g_world_menu_entry_ids')
                for i = 0, LIST_BYTES - 1 do saved[i] = e.read8(listAt + i) end
            end
            if prepare then prepare() end
            local list = LISTS[name]
            local count = 0
            for item = list.first, list.last do
                e.write16(listAt + 2 * count, item)
                count = count + 1
            end
            e.write16(listAt + 2 * count, 0xffff)
            values[label .. '_count'] = count
            watchWrongTable(name)
            wrongReads = 0
            watching = true
            return { keys, listAt }
        end,
        done = function()
            watching = false
            values[label .. '_wrong_table_reads'] = wrongReads
            if name == 'headgear' then values[label .. '_order'] = order(values[label .. '_count']) end
            if done then done() end
        end }
end

local original
local calls = {
    sortList('headgear_physical', 'headgear', PHYSICAL_EVADE_KEYS),
    sortList('headgear_magic', 'headgear', MAGIC_EVADE_KEYS),
    sortList('armor_physical', 'armor', PHYSICAL_EVADE_KEYS),
    sortList('throwables_physical', 'throwables', PHYSICAL_EVADE_KEYS),
    sortList('unpaid', 'headgear', MAGIC_EVADE_KEYS, function()
        unit = e.read32(e.address('world', 'g_world_formation_unit_pointers'))
        original = e.read16(unit + EQUIPMENT)
        e.write16(unit + EQUIPMENT, SHOP_ITEM + LEATHER_HELMET)
        -- The count array is memset's destination; the unpaid item's count
        -- lands at its id with the flag.
        e.breakAt(memset.address, 'count array', function()
            local counts = tonumber(PCSX.getRegisters().GPR.n.a0)
            e.watch(counts + SHOP_ITEM + LEATHER_HELMET, 'Write', 1, 'count past the array', function(where, pc)
                farWrites = farWrites + 1
                return true
            end)
            return false
        end)
    end, function()
        values.writes_past_counts = farWrites
        e.write16(unit + EQUIPMENT, original)
        for i = 0, LIST_BYTES - 1 do e.write8(listAt + i, saved[i]) end
    end),
}

return {
    why = 'the item sorts read the wrong evade tables and count an unpaid item past their array (direct calls: no screen sorts them where the scenario can watch)',
    save = { file = 'world-map.mcd', sha256 = 'adee3f05a445925002945e98791b6e8a461e5b3bc46ff28842c879f44243b945' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'wldcore', name = 'wldcore_entrypoint', label = 'world map WLDCORE', before = { BUTTON.CIRCLE },
          budget = 6000, shot = 90 },
        { module = 'world', name = 'world_formation_run_screen', label = 'WORLD menu formation screen', before = {},
          budget = 6000, shot = 90, start = function() e.openWorldMenu(FORMATION) end },
    }),
    trigger = {
        e.call({ label = 'item sorts', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' } } }),
    },
    check = function() return values end,
    expect = {
        original = { headgear_physical_count = 32, headgear_physical_wrong_table_reads = 10, headgear_physical_order = HEADGEAR_ORDER,
            headgear_magic_count = 32, headgear_magic_wrong_table_reads = 10, headgear_magic_order = HEADGEAR_ORDER,
            armor_physical_count = 32, armor_physical_wrong_table_reads = 0,
            throwables_physical_count = 6, throwables_physical_wrong_table_reads = 0,
            unpaid_count = 32, unpaid_wrong_table_reads = 10, unpaid_order = HEADGEAR_ORDER, writes_past_counts = 1 },
        dc = { headgear_physical_count = 32, headgear_physical_wrong_table_reads = 0, headgear_physical_order = HEADGEAR_ORDER,
            headgear_magic_count = 32, headgear_magic_wrong_table_reads = 0, headgear_magic_order = HEADGEAR_ORDER,
            armor_physical_count = 32, armor_physical_wrong_table_reads = 0,
            throwables_physical_count = 6, throwables_physical_wrong_table_reads = 0,
            unpaid_count = 32, unpaid_wrong_table_reads = 0, unpaid_order = HEADGEAR_ORDER, writes_past_counts = 0 },
    },
}
