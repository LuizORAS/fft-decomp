-- The PCSX-Redux run behind `make smoke`: boot the disc the emulator was
-- given, press the pad from this script and pass each checkpoint of a
-- session, with what tools/emulator.lua shares.
--
-- Two sessions. new-game: the title, the name entry, the first battle
-- (ATTACK.OUT at its start, an effect file's code, Wait on Ramza's turns),
-- then the save screen (CARD.OUT), which writes memcard1.mcd. world-map:
-- Continue from that save into the Gariland battle (ATTACK.OUT's deployment,
-- Auto-battle for every player unit), then the world map and the formation
-- screen.
--
-- tools/emulator.sh loads tools/emulator.lua, then this file, and starts a
-- session with -exec "smoke_run([[DIR]], 'SESSION')".

local bit = require('bit')

local e = emulator
local BUTTON = e.BUTTON
local run = e.run

-- No effect this early in the game has code: 80 of the 454 abilities and 34
-- event effects play a file with native callbacks, none before Chapter 1's
-- later events. The first effect request of the first battle plays Cure 4
-- (EFFECT/E004.BIN) instead, so the game reads that file at the sector its
-- table gives and runs its native callbacks at the addresses
-- g_battle_effect_native_callbacks holds. The action keeps its own result.
local INJECTED_ABILITY = 0x004

-- The turn menu rows each session's battle chooses: Wait (row 2) in Orbonne,
-- where the guests win; Auto-battle (row 4), then Save fading life (row 3 of
-- the Auto-battle menu, id 1) in Gariland, where the AI then plays each unit.
-- Save fading life takes no target (Fight for life wants an enemy under the
-- map cursor) and runs the general search unless an ally is in peril.
local WAIT = { { row = 2, name = 'Wait' } }
local AUTO_BATTLE = { { row = 4, name = 'Auto-battle' }, { menu = 1, row = 3, name = 'Save fading life' } }
-- WORLD's menu row the world map session opens: the formation screen.
local FORMATION = { { menu = 4, row = 1, name = 'Formation' } }

-- The first effect request plays INJECTED_ABILITY's file; the checkpoint is
-- the first native callback its script asks for, in that file's code.
local function injectEffect(passed)
    local request = e.symbol('battle', 'battle_effect_request_ability_effect')
    e.breakAt(request.address, 'effect request', function()
        if not e.resident(request) then return true end
        e.log('the effect request of ability 0x%03x plays ability 0x%03x instead (Cure 4, E004.BIN)',
            tonumber(PCSX.getRegisters().GPR.n.a1), INJECTED_ABILITY)
        e.setGPR(5, INJECTED_ABILITY)
        return false
    end)
    local getter = e.symbol('battle', 'battle_effect_get_native_callback')
    local callbacks = e.symbol('battle', 'g_battle_effect_native_callbacks').address
    local armed, passedOnce = {}, false
    e.breakAt(getter.address, 'effect callback', function()
        if passedOnce then return false end
        if not e.resident(getter) then return true end
        local id = e.signed16(bit.band(tonumber(PCSX.getRegisters().GPR.n.a0), 0xffff))
        local target = e.read32(callbacks + 4 * id)
        if armed[target] then return true end
        armed[target] = true
        e.log('the effect script asks for native callback %d at 0x%08x', id, target)
        e.breakAt(target, 'effect code', function()
            if passedOnce then return false end
            for _, found in ipairs(run.effects[target] or {}) do
                if e.resident(found) then
                    passedOnce = true
                    e.checkpoint('effect code', string.format('callback %d runs %s.%s at 0x%08x, code checked', id,
                        found.module, found.name, target), 8, passed)
                    return false
                end
            end
            e.log('native callback %d at 0x%08x: no effect function starts there in RAM; waiting', id, target)
            return true
        end)
        return true
    end)
end

-- The save is on the card once memcard1.mcd holds the game's file
-- (BASCUS-94221...), some frames after the write.
local function checkSave(passed)
    run.timer = { at = run.frame + 600, call = function()
        local card = e.readText(run.dir .. '/memcard1.mcd')
        if not (card and card:find('BASCUS-94221', 1, true)) then return e.fail('memcard1.mcd holds no save of the game') end
        e.checkpoint('save written', 'memcard1.mcd holds the game\'s save file BASCUS-94221', 30, passed)
    end }
end

-- Sessions

local sessions = {}

local function done()
    local missing = e.missingExpected()
    if #missing > 0 then return e.fail('no checkpoint ' .. table.concat(missing, ', ')) end
    e.finish(0, 'pass')
end

sessions['new-game'] = function()
    e.steps({
        e.title(),
        { module = 'world', name = 'world_name_run_entry_screen', label = 'new game WORLD name entry run by OPEN',
          before = { BUTTON.START }, budget = 9000, shot = 60 },
        { module = 'world', name = 'world_name_run_confirmation_menu', before = { BUTTON.START }, budget = 1800 },
        { module = 'battle', name = 'battle_state_run_game_loop', label = 'first battle BATTLE main loop',
          before = { BUTTON.CIRCLE, BUTTON.START }, budget = 9000, shot = 240, start = function()
              e.expect('event-attack', 'attack_scenario_load_conditionals', 'overlay ATTACK.OUT battle start')
          end },
        { label = 'effect code', before = { BUTTON.CIRCLE }, budget = 20000, start = function(passed)
              e.playTurns(WAIT)
              injectEffect(passed)
          end },
        { module = 'event-card', name = 'card_save_run_menu', label = 'overlay menu CARD.OUT save screen',
          before = { BUTTON.CIRCLE }, budget = 50000, shot = 60 },
        { module = 'event-card', name = 'card_file_write_buffer', before = { BUTTON.CIRCLE }, budget = 3000 },
        { label = 'save written', budget = 1200, start = checkSave },
    }, done)()
end

sessions['world-map'] = function()
    e.steps(e.join(e.continueFromTitle(), {
        { module = 'event-attack', name = 'attack_deploy_run_screen', label = 'overlay menu ATTACK.OUT deployment',
          before = { BUTTON.CIRCLE }, budget = 9000, shot = 90 },
        { module = 'wldcore', name = 'wldcore_entrypoint', label = 'world map WLDCORE', before = { BUTTON.CIRCLE },
          budget = 60000, shot = 300, start = function()
              e.startDeployment()
              e.playTurns(AUTO_BATTLE)
          end },
        { module = 'world', name = 'world_formation_run_screen', label = 'WORLD menu formation screen', before = {},
          budget = 6000, shot = 90, start = function() e.openWorldMenu(FORMATION) end },
    }), done)()
end

function smoke_run(dir, session)
    e.start(dir, 'smoke', function()
        e.log('session %s', session)
        if not sessions[session] then error('no session ' .. session) end
        sessions[session]()
    end)
end
