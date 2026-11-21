-- Retail bug of battle_ai_build_monster_skill_tile_mask (QUIRKS.md): the
-- mask of the tiles next to a Monster Skill holder, which limits where the AI
-- plans a monster's ability that needs one, loads the -1 neighbor offsets
-- unsigned (+255), so only the +x and +y neighbors survive, and its level
-- loop runs for the lower level only. The real check
-- (battle_menu_init_monster_skill_check) and the AI's own final test take all
-- four neighbors. No early battle has a Monster Skill holder, so a call makes
-- the mask at the AI stop of a Gariland Auto-battle turn (the next BATTLE
-- thread_yield after the AI first considers an ability): the acting unit
-- (slot 0, at 8,12 on the lower level, height 2, its four neighbors at height
-- 2) gets Monster Skill in support byte 4, and the scenario reads the mask
-- rows on both levels. Gariland has no upper level; its tiles read height 0,
-- within 2 of the holder, so the Developer's Cut marks them too, which the
-- AI's reachable tiles (none on the upper level, also read here) leave out.
-- The workspace and the support byte are put back after the call. On the
-- original only 9,12 and 8,13 are marked; on the Developer's Cut all four
-- neighbors are, on both levels.
local bit = require('bit')
local e = emulator
local BUTTON = e.BUTTON

-- The Auto-battle menu rows, the AI workspace's bytes, its mask, reachable
-- tiles and fields, a unit record's bytes and fields, Monster Skill, and the
-- offset of thread_yield's second instruction.
local AUTO_BATTLE = { { row = 4, name = 'Auto-battle' }, { menu = 1, row = 3, name = 'Save fading life' } }
local WORKSPACE_BYTES = 0x19c8
local TARGETABLE, REACHABLE, ROW_COUNT, SCENARIO_COUNT, LEVEL_COUNT = 0x0b4c, 0x0a74, 18, 3, 2
local ACTING_ID = 0x0e2e
local STATS_BYTES, X, Y, SUPPORT_4 = 448, 0x047, 0x048, 0x092
local MONSTER_SKILL = 0x80
local YIELD_SECOND_INSTRUCTION = 4

local anchor = e.symbol('battle', 'thread_yield')
local build = e.symbol('battle', 'battle_ai_build_monster_skill_tile_mask')
local report = e.symbol('main', 'main_system_handle_error')

local values = {}
local saved = {}
local ai, stats

-- The nonzero rows of a level of a row array, as y=mask, or 'none'.
local function rows(base, level)
    local out = {}
    for r = 0, ROW_COUNT - 1 do
        local v = e.read16(base + 2 * (level * ROW_COUNT + r))
        if v ~= 0 then out[#out + 1] = string.format('y%d=%04x', r, v) end
    end
    return #out > 0 and table.concat(out, ' ') or 'none'
end

local calls = {
    { target = build, label = 'monster skill mask',
      args = function()
          ai = e.address('battle', 'g_battle_ai_data_base')
          stats = e.address('battle', 'g_battle_unit_stats') + STATS_BYTES * e.read8(ai + ACTING_ID)
          values.holder = string.format('%d,%d', e.read8(stats + X), e.read8(stats + Y))
          local upper = 0
          for scenario = 0, SCENARIO_COUNT - 1 do
              if rows(ai + REACHABLE + 2 * ROW_COUNT * LEVEL_COUNT * scenario, 1) ~= 'none' then upper = upper + 1 end
          end
          values.reachable_upper_scenarios = upper
          saved.workspace = {}
          for i = 0, WORKSPACE_BYTES - 1 do saved.workspace[i] = e.read8(ai + i) end
          saved.support4 = e.read8(stats + SUPPORT_4)
          e.write8(stats + SUPPORT_4, bit.bor(saved.support4, MONSTER_SKILL))
          return {}
      end,
      done = function(result)
          values.returned = result.returned and 1 or 0
          values.lower_level = rows(ai + TARGETABLE, 0)
          values.upper_level = rows(ai + TARGETABLE, 1)
          for i = 0, WORKSPACE_BYTES - 1 do e.write8(ai + i, saved.workspace[i]) end
          e.write8(stats + SUPPORT_4, saved.support4)
      end },
}

return {
    why = 'the AI Monster Skill mask keeps two neighbors on one level (direct call: no early battle has a holder)',
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
        e.call({ label = 'AI stop', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' },
                       { address = report.address, limit = 0, label = 'error report' } } }),
    },
    check = function() return values end,
    expect = {
        original = { holder = '8,12', reachable_upper_scenarios = 0, returned = 1,
                     lower_level = 'y12=0040 y13=0080', upper_level = 'none' },
        dc = { holder = '8,12', reachable_upper_scenarios = 0, returned = 1,
               lower_level = 'y11=0080 y12=0140 y13=0080', upper_level = 'y11=0080 y12=0140 y13=0080' },
    },
}
