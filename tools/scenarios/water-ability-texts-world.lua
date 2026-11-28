-- Retail text of abilities 0x1f6 and 0x1f7 (QUIRKS.md): their names, "Move
-- in Water" and "Walk on Water", say the reverse of what each does (0x1f6
-- keeps the unit on the surface, 0x1f7 1h below it), and 0x1f7's help speaks
-- of the surface of the water. The Developer's Cut swaps the names and gives
-- 0x1f7 a help that says what it does, in every copy on the disc
-- (target/edits.yaml).
-- This scenario reads the world map's copies in RAM on the real route, the
-- formation screen of the world-map save: WORLD's text block, its ability-name
-- block, which the formation screen binds (and the names the bound section
-- gives), EVENT/WORLD.LZW as WLDCORE's entry loads it, and WORLD's copy of the
-- WLDHELP help (compressed). On the original each copy reads as retail; on the
-- Developer's Cut the names trade places, 0x1f7's help is the new one and
-- every other entry stays (build/scratch checks the rest).
local bit = require('bit')
local e = emulator
local BUTTON = e.BUTTON

-- A text block's offset table, the end of an entry and the two sections; the
-- two abilities; WORLD's menu row Formation.
local TABLE_BYTES, END = 0x80, 0xfe
local ABILITY_NAMES, ABILITY_HELP = 14, 15
local IN_WATER, ON_WATER = 0x1f6, 0x1f7
local FORMATION = { { menu = 4, row = 1, name = 'Formation' } }

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

local function check()
    local screen = e.symbol('world', 'world_formation_run_screen')
    values.formation_resident = e.resident(screen) and 1 or 0
    readPair('world_names', section(e.address('world', 'g_world_text_section_offsets'), ABILITY_NAMES))
    readPair('name_block', e.address('world', 'g_world_text_ability_names_data'))
    readPair('bound_names', e.read32(e.address('world', 'g_text_section_pointers') + 4 * ABILITY_NAMES))
    readPair('world_lzw_names', section(e.address('world', 'g_world_menu_text_file_buffer'), ABILITY_NAMES))
    readPair('wldhelp_help', section(e.address('world', 'g_world_text_wldhelp_section_offsets'), ABILITY_HELP))
    return values
end

local IN_NAME, ON_NAME = 'Move in Water', 'Walk on Water'
-- 0x1f6's help (its own on both discs), 0x1f7's retail help and its new one.
local ON_WATER_HELP = 'Move or stop on water{d9b6}'
local SURFACE_HELP, BELOW_SURFACE_HELP = 'Mover or stop over the surface of the water{d9b6}',
    'Move and stop 1h below the surface of water{d9b6}'

local function expected(first, second, secondHelp)
    return { formation_resident = 1,
        world_names_0x1f6 = first, world_names_0x1f7 = second,
        name_block_0x1f6 = first, name_block_0x1f7 = second,
        bound_names_0x1f6 = first, bound_names_0x1f7 = second,
        world_lzw_names_0x1f6 = first, world_lzw_names_0x1f7 = second,
        wldhelp_help_0x1f6 = ON_WATER_HELP, wldhelp_help_0x1f7 = secondHelp }
end

return {
    why = 'the names of abilities 0x1f6 and 0x1f7 say the reverse of what each does, and 0x1f7\'s help speaks of the surface (the world map\'s copies in RAM, real route)',
    save = { file = 'world-map.mcd', sha256 = 'adee3f05a445925002945e98791b6e8a461e5b3bc46ff28842c879f44243b945' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'wldcore', name = 'wldcore_entrypoint', label = 'world map WLDCORE', before = { BUTTON.CIRCLE },
          budget = 6000, shot = 90 },
        { module = 'world', name = 'world_formation_run_screen', label = 'WORLD menu formation screen', before = {},
          budget = 6000, shot = 90, start = function() e.openWorldMenu(FORMATION) end },
    }),
    trigger = { { label = 'formation frames', frames = 60 } },
    check = check,
    expect = {
        original = expected(IN_NAME, ON_NAME, SURFACE_HELP),
        dc = expected(ON_NAME, IN_NAME, BELOW_SURFACE_HELP),
    },
}
