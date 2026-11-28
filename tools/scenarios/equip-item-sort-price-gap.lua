-- Retail bug of equip_item_sort_list_by_criteria (QUIRKS.md): EQUIP's item sort
-- keeps the difference of two keys in 16 bits, so two items whose prices are
-- more than 32,767 apart compare by price in reverse. The order menu of Equip
-- Change's item list runs it, on the real route: the Gariland battle (the
-- new-game save), the first turn's Act and Equip Change, List, the headgear
-- tab, an item, Order, then the Type key and the Price key. Two things are put
-- in RAM at that turn: the Equip Change support of every unit, and seven
-- headgears with their place in the headgear page order, which the tab lists
-- (four hats, with Thief Hat's 35,000, and three hair ornaments, with Ribbon's
-- 60,000). The scenario reads the list each key leaves and the page order MAIN
-- keeps for the save. On the original both keys leave one order, with Thief Hat
-- and Ribbon last of their types; on the Developer's Cut Type puts the dearest
-- of each type first and Price the dearest of all.
local bit = require('bit')
local e = emulator
local BUTTON = e.BUTTON
local run = e.run

-- A unit record's bytes, its fourth support byte and Equip Change's bit there;
-- the units; Leather Hat, Triangle Hat, Flash Hat, Thief Hat, Cachusha,
-- Barette and Ribbon with the count each gets; the end of a page order.
local STATS_BYTES, SUPPORT_4, EQUIP_CHANGE, UNITS = 448, 0x092, 0x20, 21
local HEADGEAR, HELD = { 157, 161, 167, 168, 169, 170, 171 }, 2
local PAGE_ORDER_END = 0xff
-- The headgear slot, and an entry's item id (EQUIP_ITEM_ID_MASK).
local HEAD_SLOT, ITEM_ID_MASK = 1, 0x3ff

local values = { sorts = 0 }
local stored = {}

local function prepare()
    local stats = e.address('battle', 'g_battle_unit_stats')
    for unit = 0, UNITS - 1 do
        local at = stats + STATS_BYTES * unit + SUPPORT_4
        e.write8(at, bit.bor(e.read8(at), EQUIP_CHANGE))
    end
    local quantities = e.address('main', 'g_main_item_quantities')
    local order = e.address('main', 'g_main_helmet_page_order')
    for i, item in ipairs(HEADGEAR) do
        e.write8(quantities + item, HELD)
        e.write8(order + i - 1, item)
    end
    e.write8(order + #HEADGEAR, PAGE_ORDER_END)
end

-- Act, then Equip Change, the row Up wraps to.
local function openEquipChange()
    e.startDeployment()
    local turn = e.symbol('battle', 'battle_menu_build_idle_action_menu')
    local menus = e.symbol('battle', 'battle_menu_build_unit_action_menus')
    e.breakAt(menus.address, 'action menus', function()
        if e.resident(menus) then prepare() end
        return true
    end)
    e.breakAt(turn.address, 'turn menu', function()
        if not e.resident(turn) then return true end
        run.pattern = nil
        prepare()
        run.queue = { { nil, 60 }, { BUTTON.DOWN, 30 }, { BUTTON.CIRCLE, 60 }, { BUTTON.UP, 60 }, { BUTTON.CIRCLE, 60 } }
        return false
    end)
end

local function pageOrder()
    local at = e.address('main', 'g_main_helmet_page_order')
    local ids = {}
    for i = 0, 28 do
        local item = e.read8(at + i)
        if item == PAGE_ORDER_END then break end
        ids[#ids + 1] = item
    end
    return table.concat(ids, ' ')
end

local function watchSorts()
    local sort = e.symbol('event-equip', 'equip_item_sort_list_by_criteria')
    local store = e.symbol('event-equip', 'equip_item_store_category_list')
    e.breakAt(sort.address, 'sort', function()
        if e.resident(sort) then values.sorts = values.sorts + 1 end
        return true
    end)
    e.breakAt(store.address, 'store', function()
        if not e.resident(store) then return true end
        local regs = PCSX.getRegisters()
        local slot, list = tonumber(regs.GPR.n.a0), tonumber(regs.GPR.n.a1)
        local ids = {}
        for i = 0, 143 do
            local entry = e.signed16(e.read16(list + 2 * i))
            if entry == -1 then break end
            ids[#ids + 1] = bit.band(entry, ITEM_ID_MASK)
        end
        stored[#stored + 1] = slot == HEAD_SLOT and table.concat(ids, ' ') or 'slot ' .. slot
        e.log('stored slot %d: %s', slot, table.concat(ids, ' '))
        return true
    end)
end

-- List is the command menu's fourth row; the first Circle on an item starts
-- the rearrange mode and the second, on the same item, opens the item's menu,
-- whose third row is Order.
local STAGES = {
    { 'list', { { BUTTON.DOWN, 24 }, { BUTTON.DOWN, 24 }, { BUTTON.DOWN, 24 }, { BUTTON.CIRCLE, 90 } } },
    { 'headgear', { { BUTTON.RIGHT, 60 } } },
    { 'item-menu', { { BUTTON.CIRCLE, 50 }, { BUTTON.CIRCLE, 70 } } },
    { 'order-menu', { { BUTTON.DOWN, 30 }, { BUTTON.DOWN, 30 }, { BUTTON.CIRCLE, 70 } } },
    { 'type', { { BUTTON.CIRCLE, 90 } } },
    { 'price', { { BUTTON.DOWN, 30 }, { BUTTON.CIRCLE, 90 } } },
}

local function order(passed)
    watchSorts()
    local index = 0
    local function stage()
        index = index + 1
        local current = STAGES[index]
        if not current then return passed() end
        local frames = 40
        for _, press in ipairs(current[2]) do frames = frames + press[2] end
        run.queue = { unpack(current[2]) }
        run.timer = { at = run.frame + frames, call = function()
            e.screenshot(current[1])
            stage()
        end }
    end
    stage()
end

return {
    why = 'EQUIP\'s item sort compares prices more than 32,767 apart in reverse (the real route, with the Equip Change support and the items put in RAM)',
    save = { file = 'smoke-new-game.mcd', sha256 = '02203c586fc996bd3c3dfa1fb07bb6e0025cb590b69b59dae955fc3b098c47e3' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'event-attack', name = 'attack_deploy_run_screen', label = 'overlay menu ATTACK.OUT deployment',
          before = { BUTTON.CIRCLE }, budget = 9000, shot = 90 },
        { module = 'event-equip', name = 'equip_entrypoint', label = 'overlay menu EQUIP.OUT equip change',
          before = { BUTTON.CIRCLE }, budget = 40000, shot = 120, start = openEquipChange },
    }),
    trigger = { { start = order, label = 'order', budget = 4000 } },
    check = function()
        values.type_order, values.price_order = stored[1], stored[2]
        values.stores = #stored
        values.page_order = pageOrder()
        return values
    end,
    expect = {
        original = { sorts = 2, stores = 2, type_order = '167 161 157 168 169 170 171', price_order = '167 161 157 168 169 170 171',
            page_order = '167 161 157 168 169 170 171' },
        dc = { sorts = 2, stores = 2, type_order = '168 167 161 157 171 170 169', price_order = '171 168 170 169 167 161 157',
            page_order = '171 168 170 169 167 161 157' },
    },
}
