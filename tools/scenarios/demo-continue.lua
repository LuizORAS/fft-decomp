-- The demonstration scenario: no fix, so both discs give the same values.
-- Continue from the save the smoke's new-game session writes on the original
-- disc to the Gariland deployment screen; setup then writes the war funds,
-- which only the state carries to the check. The squad limit and the units
-- that can join come from the save and the battle's data.
local e = emulator
local BUTTON = e.BUTTON

-- EVENT_SCRIPT_VAR_WAR_FUNDS's word in g_main_script_variables.
local WAR_FUNDS = 4 * 0x2c
local FUNDS = 123456

local function warFunds() return e.address('main', 'g_main_script_variables') + WAR_FUNDS end

return {
    why = 'Demonstration: no fix; both discs give the same values',
    save = { file = 'smoke-new-game.mcd', sha256 = '02203c586fc996bd3c3dfa1fb07bb6e0025cb590b69b59dae955fc3b098c47e3' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'event-attack', name = 'attack_deploy_run_screen', label = 'overlay menu ATTACK.OUT deployment',
          before = { BUTTON.CIRCLE }, budget = 9000, shot = 90 },
        { start = function(passed)
              e.write32(warFunds(), FUNDS)
              passed()
          end },
    }),
    trigger = { { frames = 60 } },
    check = function()
        local squad = e.read32(e.address('event-attack', 'g_attack_deploy_current_squad_data'))
        return {
            war_funds = e.read32(warFunds()),
            squad_limit = e.read8(squad + 8), -- attack_deployment_squad_data_t.unit_limit
            fieldable = e.read32(e.address('event-attack', 'g_attack_deploy_fieldable_unit_count')),
        }
    end,
    expect = {
        original = { war_funds = FUNDS, squad_limit = 5, fieldable = 7 },
        dc = { war_funds = FUNDS, squad_limit = 5, fieldable = 7 },
    },
}
