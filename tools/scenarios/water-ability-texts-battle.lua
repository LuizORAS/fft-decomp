-- Retail text of abilities 0x1f6 and 0x1f7 (QUIRKS.md): their names, "Move
-- in Water" and "Walk on Water", say the reverse of what each does (0x1f6
-- keeps the unit on the surface, 0x1f7 1h below it), and 0x1f7's help speaks
-- of the surface of the water. The Developer's Cut swaps the names and gives
-- 0x1f7 a help that says what it does, in every copy on the disc
-- (target/edits.yaml).
-- This scenario reads the battle's copies in RAM on the real route, the
-- Gariland battle (the new-game save): EVENT/ATCHELP.LZW as the deployment
-- screen loads it (its help, plain text), BATTLE's names, and BUNIT.OUT's
-- help table (compressed) once the first turn's mini menu opens the unit
-- list, with the names the bound ability-name section gives there. On the
-- original each copy reads as retail; on the Developer's Cut the names trade
-- places, 0x1f7's help is the new one and every other entry stays
-- (build/scratch checks the rest).
local bit = require('bit')
local e = emulator
local BUTTON = e.BUTTON
local run = e.run

-- A text block's offset table, the end of an entry and the two sections; the
-- two abilities; the mini menu's Unit List row.
local TABLE_BYTES, END = 0x80, 0xfe
local ABILITY_NAMES, ABILITY_HELP = 14, 15
local IN_WATER, ON_WATER = 0x1f6, 0x1f7
local UNIT_LIST = { { menu = 10, row = 1, name = 'Unit List' } }

local values = {}

-- Game text: a block opens with 32 section offsets counted from the end of
-- its 0x80-byte table, and each entry ends with 0xfe. A byte 0xd0-0xdf starts
-- a 2-byte glyph; 0xf0-0xf3 starts a 3-byte back-reference that replays
-- (b0 & 3) * 8 + (b1 >> 5) + 4 bytes from (b1 & 0xf) * 254 + b2 bytes before
-- it (text_advance_cursor_with_backreferences).
local function section(block, index) return block + TABLE_BYTES + e.read32(block + 4 * index) end

local function entry(start, index)
    for _ = 1, index do
        while e.read8(start) ~= END do start = start + 1 end
        start = start + 1
    end
    return start
end

local function decode(at)
    local bytes = {}
    while e.read8(at) ~= END do
        local b = e.read8(at)
        if b >= 0xf0 and b <= 0xf3 then
            local b1, b2 = e.read8(at + 1), e.read8(at + 2)
            local from = at - (bit.band(b1, 0xf) * 254 + b2)
            for k = 0, bit.band(b, 3) * 8 + bit.rshift(b1, 5) + 3 do bytes[#bytes + 1] = e.read8(from + k) end
            at = at + 3
        elseif b >= 0xd0 and b <= 0xdf then
            bytes[#bytes + 1], bytes[#bytes + 2] = b, e.read8(at + 1)
            at = at + 2
        else
            bytes[#bytes + 1] = b
            at = at + 1
        end
    end
    local out, i = {}, 1
    while i <= #bytes do
        local b = bytes[i]
        if b < 10 then out[#out + 1] = string.char(48 + b)
        elseif b < 36 then out[#out + 1] = string.char(55 + b)
        elseif b < 62 then out[#out + 1] = string.char(61 + b)
        elseif b == 0xfa then out[#out + 1] = ' '
        elseif b >= 0xd0 and b <= 0xdf then
            out[#out + 1] = string.format('{%02x%02x}', b, bytes[i + 1])
            i = i + 1
        else out[#out + 1] = string.format('{%02x}', b) end
        i = i + 1
    end
    return table.concat(out)
end

-- Both abilities' entries of a section, as name_0x1f6 and name_0x1f7.
local function readPair(name, start)
    values[name .. '_0x1f6'] = decode(entry(start, IN_WATER))
    values[name .. '_0x1f7'] = decode(entry(start, ON_WATER))
end

-- The deployment screen's text file, as attack_text_build_deployment_strings
-- binds it.
local function readAtchelp(passed)
    local bind = e.symbol('event-attack', 'attack_text_init_battle_pointers')
    e.breakAt(bind.address, 'ATCHELP bound', function()
        if not e.resident(bind) then return true end
        readPair('atchelp_help', section(tonumber(PCSX.getRegisters().GPR.n.a0), ABILITY_HELP))
        passed()
        return false
    end)
end

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

local function check()
    local list = e.symbol('event-bunit', 'bunit_menu_run_display_script')
    values.bunit_resident = e.resident(list) and 1 or 0
    readPair('battle_names', section(e.address('battle', 'g_battle_menu_text_section_offsets'), ABILITY_NAMES))
    readPair('bound_names', e.read32(e.address('battle', 'g_text_section_pointers') + 4 * ABILITY_NAMES))
    readPair('bunit_help', section(e.address('event-bunit', 'g_bunit_help_text_table'), ABILITY_HELP))
    return values
end

local IN_NAME, ON_NAME = 'Move in Water', 'Walk on Water'
-- 0x1f6's help (its own on both discs), 0x1f7's retail help and its new one.
local ON_WATER_HELP = 'Move or stop on water{d9b6}'
local SURFACE_HELP, BELOW_SURFACE_HELP = 'Mover or stop over the surface of the water{d9b6}',
    'Move and stop 1h below the surface of water{d9b6}'

local function expected(first, second, secondHelp)
    return { bunit_resident = 1,
        atchelp_help_0x1f6 = ON_WATER_HELP, atchelp_help_0x1f7 = secondHelp,
        battle_names_0x1f6 = first, battle_names_0x1f7 = second,
        bound_names_0x1f6 = first, bound_names_0x1f7 = second,
        bunit_help_0x1f6 = ON_WATER_HELP, bunit_help_0x1f7 = secondHelp }
end

return {
    why = 'the names of abilities 0x1f6 and 0x1f7 say the reverse of what each does, and 0x1f7\'s help speaks of the surface (the battle\'s copies in RAM, real route)',
    save = { file = 'smoke-new-game.mcd', sha256 = '02203c586fc996bd3c3dfa1fb07bb6e0025cb590b69b59dae955fc3b098c47e3' },
    setup = e.continueFromTitle(),
    trigger = {
        { label = 'ATCHELP bound', before = { BUTTON.CIRCLE }, budget = 9000, start = readAtchelp },
        { module = 'event-attack', name = 'attack_deploy_run_screen', label = 'overlay menu ATTACK.OUT deployment',
          before = { BUTTON.CIRCLE }, budget = 9000, shot = 90 },
        { module = 'event-bunit', name = 'bunit_menu_run_display_script', label = 'overlay menu BUNIT.OUT unit list',
          before = { BUTTON.CIRCLE }, budget = 40000, shot = 90, start = openUnitList },
    },
    check = check,
    expect = {
        original = expected(IN_NAME, ON_NAME, SURFACE_HELP),
        dc = expected(ON_NAME, IN_NAME, BELOW_SURFACE_HELP),
    },
}
