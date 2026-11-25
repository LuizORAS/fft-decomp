-- Retail bug of battle_formula_raise_level_by_one_add_status_on_caster
-- (QUIRKS.md): Please Eat stores a full HP and MP heal and then overwrites the
-- result type with the level change's alone, so the post-action display shows
-- Level Up and no HP or MP number. Gariland has no Please Eat, so calls make
-- it at one stop of BATTLE's thread_yield, from the AI stop of an Auto-battle
-- turn on Gariland's real route (the new-game save): unit slot 0 casts on
-- unit slot 1, once living and once undead, and after each cast
-- battle_gfx_prepare_post_action_display builds the target's display flags
-- (with its numeric display marked active, so no display starts). The
-- scenario reads the target's result type and display flags; both units, the
-- current ability, the four action pointers, the canceled statuses and the
-- target's runtime unit with its three display tpages are put back after the
-- last call. On the original both casts end as pseudo-status (0x01) with
-- Level Up alone (0x00020000); on the Developer's Cut the living target keeps
-- HP and MP healing (0x51) and shows both numbers with Level Up (0x0002000a),
-- and the undead target stays as retail.
local bit = require('bit')
local e = emulator
local BUTTON = e.BUTTON

-- The Auto-battle menu rows; the two units, a unit record's bytes, its first
-- current status byte and Undead's bit there, and its result record (bytes,
-- hit, result type); the current ability's bytes and its status infliction
-- type (0: the caster's status step adds nothing and needs no roll); the
-- canceled statuses' bytes; the runtime units, their count and bytes, the
-- stats link, the display flags, the numeric display's active byte and its
-- three display pointers, and a display's tpage; the offset of thread_yield's
-- second instruction.
local AUTO_BATTLE = { { row = 4, name = 'Auto-battle' }, { menu = 1, row = 3, name = 'Save fading life' } }
local ACTOR, TARGET, STATS_BYTES = 0, 1, 448
local CURRENT_UNDEAD, UNDEAD_BIT = 0x058, 0x10
local ACTION, ACTION_BYTES, HIT, ATTACK_TYPE = 0x18c, 0x2c, 0x00, 0x25
local ABILITY_BYTES, INFLICTION_TYPE = 80, 0x46
local CANCELED_BYTES = 5
local RUNTIME_COUNT, RUNTIME_BYTES, STATS_LINK, DISPLAY_FLAGS, DISPLAY_ACTIVE, DISPLAYS = 16, 1088, 0x134, 0x1b8, 0x2bc, 0x2c4
local DISPLAY_TPAGE = 0x04
local YIELD_SECOND_INSTRUCTION = 4

local anchor = e.symbol('battle', 'thread_yield')
local formula = e.symbol('battle', 'battle_formula_raise_level_by_one_add_status_on_caster')
local display = e.symbol('battle', 'battle_gfx_prepare_post_action_display')
local report = e.symbol('main', 'main_system_handle_error')

local values = { calls_returned = 0 }
local saved
local units, ability, canceled, pointers, runtime, displays

local cases = {
    { name = 'living', undead = false },
    { name = 'undead', undead = true },
}

local function unit(slot) return units + STATS_BYTES * slot end

local function readBytes(at, count)
    local bytes = {}
    for i = 0, count - 1 do bytes[i] = e.read8(at + i) end
    return bytes
end

local function writeBytes(at, bytes, count)
    for i = 0, count - 1 do e.write8(at + i, bytes[i]) end
end

-- The runtime unit linked to the target, and its three display records.
local function findRuntime()
    local base = e.address('battle', 'g_battle_units')
    for slot = 0, RUNTIME_COUNT - 1 do
        local at = base + RUNTIME_BYTES * slot
        if e.read32(at + STATS_LINK) == unit(TARGET) then
            local records = {}
            for i = 0, 2 do
                records[i + 1] = e.read32(at + DISPLAYS + 4 * i)
                if records[i + 1] < 0x80000000 then error('the target\'s display record ' .. i .. ' is unset') end
            end
            return at, records
        end
    end
    error('no runtime unit links the target')
end

local function save()
    units = e.address('battle', 'g_battle_unit_stats')
    ability = e.address('battle', 'g_battle_action_current_ability')
    canceled = e.address('battle', 'g_battle_action_canceled_statuses')
    pointers = { attacker = e.address('battle', 'g_battle_action_attacker'),
                 attacker_data = e.address('battle', 'g_battle_action_attacker_data'),
                 target = e.address('battle', 'g_battle_action_target'),
                 target_data = e.address('battle', 'g_battle_action_target_data') }
    runtime, displays = findRuntime()
    saved = { actor = readBytes(unit(ACTOR), STATS_BYTES), target = readBytes(unit(TARGET), STATS_BYTES),
              ability = readBytes(ability, ABILITY_BYTES), canceled = readBytes(canceled, CANCELED_BYTES),
              runtime = readBytes(runtime, RUNTIME_BYTES), pointers = {}, tpages = {} }
    for name, at in pairs(pointers) do saved.pointers[name] = e.read32(at) end
    for i, at in ipairs(displays) do saved.tpages[i] = e.read16(at + DISPLAY_TPAGE) end
end

local function restore()
    writeBytes(unit(ACTOR), saved.actor, STATS_BYTES)
    writeBytes(unit(TARGET), saved.target, STATS_BYTES)
    writeBytes(ability, saved.ability, ABILITY_BYTES)
    writeBytes(canceled, saved.canceled, CANCELED_BYTES)
    writeBytes(runtime, saved.runtime, RUNTIME_BYTES)
    for name, at in pairs(pointers) do e.write32(at, saved.pointers[name]) end
    for i, at in ipairs(displays) do e.write16(at + DISPLAY_TPAGE, saved.tpages[i]) end
end

local calls = {}
for index, case in ipairs(cases) do
    calls[#calls + 1] = { target = formula, label = case.name .. ' cast',
        args = function()
            if not saved then save() end
            writeBytes(unit(ACTOR), saved.actor, STATS_BYTES)
            writeBytes(unit(TARGET), saved.target, STATS_BYTES)
            local status = e.read8(unit(TARGET) + CURRENT_UNDEAD)
            e.write8(unit(TARGET) + CURRENT_UNDEAD,
                case.undead and bit.bor(status, UNDEAD_BIT) or bit.band(status, bit.bnot(UNDEAD_BIT)))
            for _, slot in ipairs({ ACTOR, TARGET }) do
                for i = 0, ACTION_BYTES - 1 do e.write8(unit(slot) + ACTION + i, 0) end
            end
            e.write8(unit(TARGET) + ACTION + HIT, 1)
            e.write8(ability + INFLICTION_TYPE, 0)
            e.write32(pointers.attacker, unit(ACTOR))
            e.write32(pointers.attacker_data, unit(ACTOR) + ACTION)
            e.write32(pointers.target, unit(TARGET))
            e.write32(pointers.target_data, unit(TARGET) + ACTION)
            return {}
        end,
        done = function(result)
            if result.returned then values.calls_returned = values.calls_returned + 1 end
            values[case.name .. '_type'] = string.format('0x%02x', e.read8(unit(TARGET) + ACTION + ATTACK_TYPE))
        end }
    calls[#calls + 1] = { target = display, label = case.name .. ' display',
        args = function()
            writeBytes(runtime, saved.runtime, RUNTIME_BYTES)
            e.write32(runtime + DISPLAY_FLAGS, 0)
            e.write8(runtime + DISPLAY_ACTIVE, 1)
            return { runtime }
        end,
        done = function(result)
            if result.returned then values.calls_returned = values.calls_returned + 1 end
            values[case.name .. '_display'] = string.format('0x%08x', e.read32(runtime + DISPLAY_FLAGS))
            if index == #cases then restore() end
        end }
end

return {
    why = 'Please Eat heals HP and MP but shows Level Up alone (direct calls: Gariland has no Please Eat)',
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
    trigger = {
        e.call({ label = 'Please Eat', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' },
                       { address = report.address, limit = 0, label = 'error report' } } }),
    },
    check = function() return values end,
    expect = {
        original = { living_type = '0x01', living_display = '0x00020000', undead_type = '0x01',
                     undead_display = '0x00020000', calls_returned = 4 },
        dc = { living_type = '0x51', living_display = '0x0002000a', undead_type = '0x01',
               undead_display = '0x00020000', calls_returned = 4 },
    },
}
