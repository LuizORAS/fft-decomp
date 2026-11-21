-- Retail bug of battle_ai_build_unit_ability_list (QUIRKS.md): the kinds of
-- both hands' weapons are ORed and only one kind at a time gets a target
-- team, so a unit holding two kinds under Two Swords (a Blood Sword, which
-- absorbs, or a Healing Staff, which heals, with a weapon of another kind)
-- gets an Attack entry with no target team. The AI still attacks enemies
-- with it, but leaves it out of the unit's reach toward an enemy and of its
-- share of harmful abilities. No ENTD unit can hold two kinds, so calls make
-- it at one stop of BATTLE's thread_yield, from the AI stop of an Auto-battle
-- turn on Gariland's real route (the new-game save): unit slot 0 gets a Long
-- Sword, a Blood Sword, both, and a Healing Staff with a Long Sword, and the
-- AI builds its list each time; the scenario reads the Attack summary's
-- flag bytes 1 and 2 (target team 0x02 enemies, 0x01 allies; 0x80 affects
-- HP; 0x20 undead reversal, 0x02 evadable). The unit's hands and the AI
-- workspace are put back after the last call. On the original two kinds
-- give 80 (no team); on the Developer's Cut they give 82, as a damage weapon.
local e = emulator
local BUTTON = e.BUTTON

-- The Auto-battle menu rows; the unit, a unit record's bytes and its hands'
-- weapon slots; the AI workspace's bytes, the Attack summaries, a summary's
-- bytes and flag bytes 1 and 2, and the compact unit indices; the weapons;
-- the offset of thread_yield's second instruction.
local AUTO_BATTLE = { { row = 4, name = 'Auto-battle' }, { menu = 1, row = 3, name = 'Save fading life' } }
local UNIT, STATS_BYTES = 0, 448
local RIGHT_WEAPON, LEFT_WEAPON = 0x01d, 0x01f
local WORKSPACE_BYTES = 0x19c8
local WEAPON_DATA, WEAPON_BYTES, FLAGS_1, FLAGS_2, COMPACT = 0x1778, 8, 1, 2, 0x198c
local NONE, LONG_SWORD, BLOOD_SWORD, HEALING_STAFF = 0xff, 0x14, 0x17, 0x3d
local YIELD_SECOND_INSTRUCTION = 4

local anchor = e.symbol('battle', 'thread_yield')
local build = e.symbol('battle', 'battle_ai_build_unit_ability_list')
local report = e.symbol('main', 'main_system_handle_error')

local values = { calls_returned = 0 }
local saved = {}
local ai, stats

local cases = {
    { name = 'long_sword', right = LONG_SWORD, left = NONE },
    { name = 'blood_sword', right = BLOOD_SWORD, left = NONE },
    { name = 'blood_and_long_sword', right = BLOOD_SWORD, left = LONG_SWORD },
    { name = 'healing_staff_and_long_sword', right = HEALING_STAFF, left = LONG_SWORD },
}

local calls = {}
for index, case in ipairs(cases) do
    calls[#calls + 1] = { target = build, label = case.name,
        args = function()
            if index == 1 then
                ai = e.address('battle', 'g_battle_ai_data_base')
                stats = e.address('battle', 'g_battle_unit_stats') + STATS_BYTES * UNIT
                saved.workspace = {}
                for i = 0, WORKSPACE_BYTES - 1 do saved.workspace[i] = e.read8(ai + i) end
                saved.right = e.read8(stats + RIGHT_WEAPON)
                saved.left = e.read8(stats + LEFT_WEAPON)
            end
            e.write8(stats + RIGHT_WEAPON, case.right)
            e.write8(stats + LEFT_WEAPON, case.left)
            return { UNIT }
        end,
        done = function(result)
            local summary = ai + WEAPON_DATA + WEAPON_BYTES * e.read8(ai + COMPACT + UNIT)
            values[case.name] = string.format('%02x %02x', e.read8(summary + FLAGS_1), e.read8(summary + FLAGS_2))
            if result.returned then values.calls_returned = values.calls_returned + 1 end
            if index == #cases then
                for i = 0, WORKSPACE_BYTES - 1 do e.write8(ai + i, saved.workspace[i]) end
                e.write8(stats + RIGHT_WEAPON, saved.right)
                e.write8(stats + LEFT_WEAPON, saved.left)
            end
        end }
end

return {
    why = 'two weapon kinds leave the AI\'s Attack with no target team (direct calls: no ENTD unit holds two kinds)',
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
        e.call({ label = 'build the list', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' },
                       { address = report.address, limit = 0, label = 'error report' } } }),
    },
    check = function() return values end,
    expect = {
        original = { long_sword = '82 02', blood_sword = '82 22', blood_and_long_sword = '80 02',
                     healing_staff_and_long_sword = '80 02', calls_returned = 4 },
        dc = { long_sword = '82 02', blood_sword = '82 22', blood_and_long_sword = '82 02',
               healing_staff_and_long_sword = '82 02', calls_returned = 4 },
    },
}
