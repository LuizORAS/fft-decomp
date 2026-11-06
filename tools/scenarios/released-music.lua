-- Retail bug of main_sound_unload_scenario_mus (QUIRKS.md): Continue's card screen
-- plays its own music and unloads it on the way out, and the unload leaves that
-- released record in the current-music state, so the next
-- main_sound_stop_current_music stops it. The Developer's Cut clears the state
-- at the unload.
local e = emulator
local BUTTON = e.BUTTON

-- Offsets in g_main_sound_music: state.current_music and slots.music[0].
local CURRENT_MUSIC = 0x04
local SLOT_MUSIC = 0x18

local unloaded, leftInState, stopped = 0, 0, 0
local released = nil

local function watchMusic()
    local music = e.address('main', 'g_main_sound_music')
    local unload = e.symbol('main', 'main_sound_unload_scenario_mus')
    local stop = e.symbol('main', 'main_sound_stop_current_music')
    e.breakAt(unload.address, 'unload', function()
        local n = PCSX.getRegisters().GPR.n
        local record = e.read32(music + SLOT_MUSIC + 4 * (tonumber(n.a0) - 1))
        if record ~= 0 and record == e.read32(music + CURRENT_MUSIC) then
            unloaded = unloaded + 1
            released = record
            e.breakAt(tonumber(n.ra), 'unload return', function()
                if e.read32(music + CURRENT_MUSIC) == record then leftInState = leftInState + 1 end
                return false
            end)
        end
        return true
    end)
    e.breakAt(stop.address, 'stop', function()
        if released and e.read32(music + CURRENT_MUSIC) == released then stopped = stopped + 1 end
        released = nil
        return true
    end)
end

return {
    why = 'main_sound_unload_scenario_mus leaves the released music in the current-music state',
    save = { file = 'smoke-new-game.mcd', sha256 = '02203c586fc996bd3c3dfa1fb07bb6e0025cb590b69b59dae955fc3b098c47e3' },
    setup = e.continueFromTitle(),
    trigger = {
        { module = 'event-attack', name = 'attack_deploy_run_screen', label = 'overlay menu ATTACK.OUT deployment',
          before = { BUTTON.CIRCLE }, budget = 9000, start = watchMusic },
    },
    check = function()
        return { unloaded_playing_music = unloaded, left_in_state = leftInState, released_record_stopped = stopped }
    end,
    expect = {
        original = { unloaded_playing_music = 1, left_in_state = 1, released_record_stopped = 1 },
        dc = { unloaded_playing_music = 1, left_in_state = 0, released_record_stopped = 0 },
    },
}
