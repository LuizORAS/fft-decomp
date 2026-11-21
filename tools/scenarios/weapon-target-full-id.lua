-- Retail bug of battle_target_validate_weapon_target (QUIRKS.md): it reads
-- an ability's WEAPON_RANGE flag from g_main_ability_range_data[(u8)id], so an
-- ability of id 0x100 and up reads the record of its low byte. Retail never
-- reaches it: both callers read the full id's flags first, and Shock (0x100,
-- Magic Sword), the one ability whose result would differ, has neither
-- WEAPON_RANGE nor DIRECT_TARGETING. A call forces Shock in at one stop of
-- BATTLE's thread_yield on Gariland's deployment screen (the new-game save):
-- unit 0's action becomes Shock aimed at the empty tile two to its east (or
-- west), and guards count which trajectory test the function takes. The
-- action is put back after. On the original Shock reads ability 0's record,
-- which has WEAPON_RANGE, and takes the bare-handed weapon's height-overlap
-- test; on the Developer's Cut it takes the direct test of other skillsets.
local e = emulator
local BUTTON = e.BUTTON

-- A unit's stats bytes, its x, y and right-hand weapon (equipment slot 3),
-- its action command and the command's bytes, actor, skillset, ability,
-- targeting type and target position (x, elevation, y); Magic Sword, Shock and
-- tile targeting; the offset of thread_yield's second instruction.
local STATS_X, STATS_Y, RIGHT_HAND, ACTION, ACTION_BYTES = 0x047, 0x048, 0x01a + 3, 0x16e, 20
local ACTOR, SKILLSET, ABILITY, TARGETING, TARGET_X, TARGET_ELEVATION, TARGET_Y = 0, 1, 2, 0x0a, 0x0c, 0x0e, 0x10
local MAGIC_SWORD, SHOCK, TILE = 69, 0x100, 5
local YIELD_SECOND_INSTRUCTION = 4

local anchor = e.symbol('battle', 'thread_yield')
local validate = e.symbol('battle', 'battle_target_validate_weapon_target')
local overlap = e.symbol('battle', 'battle_target_validate_height_overlap')
local direct = e.symbol('battle', 'battle_effect_check_direct_trajectory_to_target')
local arc = e.symbol('battle', 'battle_effect_set_and_validate_arc_trajectory')
local lunging = e.symbol('battle', 'battle_target_validate_lunging_target')
local report = e.symbol('main', 'main_system_handle_error')

local values = {}
local saved = {}
local unit

local calls = {
    { target = validate, label = 'validate Shock',
      args = function()
          unit = e.address('battle', 'g_battle_unit_stats')
          local x, y = e.read8(unit + STATS_X), e.read8(unit + STATS_Y)
          local target_x = x >= 2 and x - 2 or x + 2
          values.unit = string.format('%d,%d', x, y)
          values.right_hand = e.read8(unit + RIGHT_HAND)
          for i = 0, ACTION_BYTES - 1 do saved[i] = e.read8(unit + ACTION + i) end
          e.write8(unit + ACTION + ACTOR, 0)
          e.write8(unit + ACTION + SKILLSET, MAGIC_SWORD)
          e.write16(unit + ACTION + ABILITY, SHOCK)
          e.write8(unit + ACTION + TARGETING, TILE)
          e.write16(unit + ACTION + TARGET_X, target_x)
          e.write16(unit + ACTION + TARGET_ELEVATION, 0)
          e.write16(unit + ACTION + TARGET_Y, y)
          return { unit + ACTION }
      end,
      done = function(result)
          values.returned = result.returned and 1 or 0
          values.height_overlap = result.counts['height overlap']
          values.direct = result.counts['direct']
          values.arc = result.counts['arc']
          values.lunging = result.counts['lunging']
          for i = 0, ACTION_BYTES - 1 do e.write8(unit + ACTION + i, saved[i]) end
      end },
}

return {
    why = 'validate_weapon_target reads the weapon-range flag with the id\'s low byte (direct call: retail never sends Shock there)',
    save = { file = 'smoke-new-game.mcd', sha256 = '02203c586fc996bd3c3dfa1fb07bb6e0025cb590b69b59dae955fc3b098c47e3' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'event-attack', name = 'attack_deploy_run_screen', label = 'overlay menu ATTACK.OUT deployment',
          before = { BUTTON.CIRCLE }, budget = 9000, shot = 90 },
    }),
    trigger = {
        e.call({ label = 'validate', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' },
                       { address = report.address, limit = 0, label = 'error report' },
                       { address = overlap.address, limit = 100, label = 'height overlap' },
                       { address = direct.address, limit = 100, label = 'direct' },
                       { address = arc.address, limit = 100, label = 'arc' },
                       { address = lunging.address, limit = 100, label = 'lunging' } } }),
    },
    check = function() return values end,
    expect = {
        original = { unit = '0,0', right_hand = 0, returned = 1, height_overlap = 1, direct = 0, arc = 0, lunging = 0 },
        dc = { unit = '0,0', right_hand = 0, returned = 1, height_overlap = 0, direct = 1, arc = 0, lunging = 0 },
    },
}
