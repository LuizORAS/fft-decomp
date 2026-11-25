-- Retail bug of battle_ai_build_monster_skill_tile_mask (QUIRKS.md): the mask
-- of the tiles next to a Monster Skill holder, which limits where the AI plans
-- a monster's ability that needs one, compares whole heights within 2, where
-- the AI's final test (battle_ai_can_use_ability_at_coords) and the menu's check
-- (battle_menu_init_monster_skill_check) count slopes, within 5 half levels.
-- Calls make the three tests at the AI stop of a Gariland Auto-battle turn (the
-- next BATTLE thread_yield after the AI first considers an ability, the new-game
-- save), with the acting unit made the holder:
-- * real: at 8,6 and at 8,7 of Gariland's terrain, whose other one stands 6 half
--   levels (whole heights 2) apart; the terrain has no neighbors 5 half levels
--   and whole heights 3 apart, so for the other two sets heights are put in RAM;
-- * up, down: at 8,12, 22 half levels high, with its four neighbors 6 (whole
--   heights 2 apart), 5 (3 apart), 5 (2 apart) and 6 (3 apart) half levels above
--   it, then as many below.
-- The menu's check gets another unit, put on the holder's team and on each
-- neighbor, with every other unit off the field. The scenario reads, per neighbor
-- (+x, +y, -x, -y; +y then -y for real), whether each test accepts it (+) or not
-- (.); the workspace, the units and the tiles are put back after the last call.
-- On both discs the two other tests refuse both real neighbors and accept the
-- second and the third of the others. The original's mask accepts the first real
-- one and the first of the others (its -1 offsets lose each -x and -y neighbor);
-- the Developer's Cut's mask accepts the same as the other two tests.
local bit = require('bit')
local e = emulator
local BUTTON = e.BUTTON

-- The Auto-battle menu rows; the AI workspace's bytes and fields (the considered
-- ability's skillset and fourth flag byte, the mask, the scratch coordinates, the
-- acting unit, the map's width, the water penalty); the flag of an ability that
-- needs Monster Skill and a monster skillset; a unit record's bytes and fields;
-- a tile's bytes, its height and the byte of its half height and depth; the tiles;
-- the offset of thread_yield's second instruction.
local AUTO_BATTLE = { { row = 4, name = 'Auto-battle' }, { menu = 1, row = 3, name = 'Save fading life' } }
local WORKSPACE_BYTES, SKILLSET, FLAGS_4, TARGETABLE, COORDS = 0x19c8, 0x0000, 0x000f, 0x0b4c, 0x0c74
local ACTING_ID, MAP_MAX_X, WATER_PENALTY = 0x0e2e, 0x0e3a, 0x19b9
local REQUIRES_MONSTER_SKILL, MONSTER_SKILLSET = 0x08, 0xb0
local STATS_BYTES, UNITS, ENTD_SLOT, TEAM_FLAGS, X, Y, SUPPORT_4, MONSTER_SKILL = 448, 21, 0x001, 0x005, 0x047, 0x048, 0x092, 0x80
local TILE_BYTES, TILES, HEIGHT, HALF, ABSENT = 8, 512, 2, 3, 0xff
local YIELD_SECOND_INSTRUCTION = 4

-- The holder's height and half height in the sets with heights in RAM; each
-- set's name, its holder's tile (the unit's own without one) and each neighbor's
-- dx and dy, with its height and half height.
local HOLDER_HEIGHT, HOLDER_HALF = 10, 2
local SETS = {
    { name = 'real', holder = { 8, 6 }, { 0, 1 } },
    { name = 'real', holder = { 8, 7 }, { 0, -1 } },
    { name = 'up', { 1, 0, 12, 4 }, { 0, 1, 13, 1 }, { -1, 0, 12, 3 }, { 0, -1, 13, 2 } },
    { name = 'down', { 1, 0, 8, 0 }, { 0, 1, 7, 3 }, { -1, 0, 8, 1 }, { 0, -1, 7, 2 } },
}

local anchor = e.symbol('battle', 'thread_yield')
local build = e.symbol('battle', 'battle_ai_build_monster_skill_tile_mask')
local test = e.symbol('battle', 'battle_ai_can_use_ability_at_coords')
local menu = e.symbol('battle', 'battle_menu_init_monster_skill_check')
local report = e.symbol('main', 'main_system_handle_error')
local values = { returned = 0 }
local ai, stats, tiles, holder, mover, hx, hy, width
local saved

local function keep(at, count)
    local bytes = {}
    for i = 0, count - 1 do bytes[i] = e.read8(at + i) end
    return bytes
end

local function put(at, bytes, count)
    for i = 0, count - 1 do e.write8(at + i, bytes[i]) end
end

local function setTile(x, y, height, half)
    local at = tiles + TILE_BYTES * (y * width + x)
    e.write8(at + HEIGHT, height)
    e.write8(at + HALF, half)
end

-- Once, before the first call: what the calls change is kept, the holder gets
-- its support and the considered ability needs it.
local function prepare()
    if saved then return end
    ai = e.address('battle', 'g_battle_ai_data_base')
    stats = e.address('battle', 'g_battle_unit_stats')
    tiles = e.address('battle', 'g_battle_map_tile_data')
    saved = { ai = keep(ai, WORKSPACE_BYTES), stats = keep(stats, STATS_BYTES * UNITS), tiles = keep(tiles, TILE_BYTES * TILES) }
    local acting = e.read8(ai + ACTING_ID)
    holder = stats + STATS_BYTES * acting
    width = e.read8(ai + MAP_MAX_X)
    hx, hy = e.read8(holder + X), e.read8(holder + Y)
    e.write8(holder + SUPPORT_4, bit.bor(e.read8(holder + SUPPORT_4), MONSTER_SKILL))
    for i = 0, UNITS - 1 do
        local unit = stats + STATS_BYTES * i
        if i ~= acting and e.read8(unit + ENTD_SLOT) ~= ABSENT then
            if not mover then mover = unit else e.write8(unit + ENTD_SLOT, ABSENT) end
        end
    end
    e.write8(mover + TEAM_FLAGS, e.read8(holder + TEAM_FLAGS))
    e.write8(ai + SKILLSET, MONSTER_SKILLSET)
    e.write8(ai + FLAGS_4, bit.bor(e.read8(ai + FLAGS_4), REQUIRES_MONSTER_SKILL))
    e.write8(ai + WATER_PENALTY, 0)
    values.holder = string.format('%d,%d', hx, hy)
end

local calls = {}
for _, set in ipairs(SETS) do
    local x, y
    local function add(target, kind, args, accepts)
        calls[#calls + 1] = { target = target, label = set.name .. ' ' .. kind, args = args, done = function(result)
            if result.returned then values.returned = values.returned + 1 end
            local name = set.name .. '_' .. kind
            values[name] = (values[name] or '') .. accepts(result)
        end }
    end
    add(build, 'mask', function()
        prepare()
        x, y = unpack(set.holder or { hx, hy })
        e.write8(holder + X, x)
        e.write16(holder + Y, y)
        if not set.holder then
            setTile(x, y, HOLDER_HEIGHT, HOLDER_HALF)
            for _, n in ipairs(set) do setTile(x + n[1], y + n[2], n[3], n[4]) end
        end
        return {}
    end, function()
        local marks = ''
        for _, n in ipairs(set) do
            local row = e.read16(ai + TARGETABLE + 2 * (y + n[2]))
            marks = marks .. (bit.band(row, bit.rshift(0x8000, x + n[1])) ~= 0 and '+' or '.')
        end
        return marks
    end)
    for _, n in ipairs(set) do
        add(test, 'test', function()
            e.write8(ai + COORDS, x + n[1])
            e.write8(ai + COORDS + 1, 0)
            e.write8(ai + COORDS + 2, y + n[2])
            e.write8(ai + COORDS + 3, 0)
            return { ai + COORDS }
        end, function(result) return result.v0 == 1 and '+' or '.' end)
        add(menu, 'menu', function()
            e.write8(mover + X, x + n[1])
            e.write16(mover + Y, y + n[2])
            return { mover }
        end, function(result) return result.v0 ~= 0 and '+' or '.' end)
    end
end

local last = calls[#calls]
local lastDone = last.done
last.done = function(result)
    lastDone(result)
    put(ai, saved.ai, WORKSPACE_BYTES)
    put(stats, saved.stats, STATS_BYTES * UNITS)
    put(tiles, saved.tiles, TILE_BYTES * TILES)
end

return {
    why = 'the AI Monster Skill mask compares whole heights where its final test and the menu count slopes (direct calls)',
    save = { file = 'smoke-new-game.mcd', sha256 = '02203c586fc996bd3c3dfa1fb07bb6e0025cb590b69b59dae955fc3b098c47e3' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'event-attack', name = 'attack_deploy_run_screen', label = 'overlay menu ATTACK.OUT deployment',
          before = { BUTTON.CIRCLE }, budget = 9000, shot = 90 },
        { module = 'battle', name = 'battle_ai_load_ability_entry', label = 'the AI considers an ability',
          before = { BUTTON.CIRCLE }, budget = 40000, shot = 1, start = function()
              e.startDeployment()
              e.playTurns(AUTO_BATTLE)
          end },
    }),
    trigger = { e.call({ label = 'heights', anchor = anchor, calls = calls,
        guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' },
                   { address = report.address, limit = 0, label = 'error report' } } }) },
    check = function() return values end,
    expect = {
        original = { holder = '8,12', returned = 24, real_mask = '+.', real_test = '..', real_menu = '..',
                     up_mask = '+...', up_test = '.++.', up_menu = '.++.',
                     down_mask = '+...', down_test = '.++.', down_menu = '.++.' },
        dc = { holder = '8,12', returned = 24, real_mask = '..', real_test = '..', real_menu = '..',
               up_mask = '.++.', up_test = '.++.', up_menu = '.++.',
               down_mask = '.++.', down_test = '.++.', down_menu = '.++.' },
    },
}
