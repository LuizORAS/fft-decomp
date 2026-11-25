-- Retail bug of wldcore_sound_novel_stop_music (QUIRKS.md): the sound novels
-- keep slot n's track in tracks[n - 1], but stopping the music clears
-- tracks[n], which for slot 2 is the current slot, so tracks[1] keeps the id
-- of the unloaded track. The one novel a treasure opens never stops its music,
-- so a call makes it at one stop of WLDCORE's world map cursor (the world-map
-- save): track 0x11 is in slot 1 and track 0x22 in slot 2, the novel's. The
-- driver's stop and unload return at once, so the map's own music stays. The
-- scenario reads both slots' tracks and the current slot; the queue's music
-- part and the novel's record are put back after the call. On the original
-- slot 2 keeps 0x22; on the Developer's Cut it is cleared. Slot 1 keeps its
-- track and the current slot is cleared on both.
local e = emulator
local BUTTON = e.BUTTON

-- The audio queue's music flags, two tracks and current slot (its bytes kept);
-- the novel's state flags and music selection (slot in the high byte, track in
-- the low one); the tracks.
local MUSIC, MUSIC_BYTES, TRACK_1, TRACK_2, CURRENT_SLOT = 0x8c, 0x10, 0x90, 0x94, 0x98
local RECORD_BYTES, SELECTION = 0x38, 0x2a
local FIRST_TRACK, NOVEL_TRACK, NOVEL_SLOT = 0x11, 0x22, 2

local anchor = e.symbol('wldcore', 'wldcore_menu_step_map_cursor_level')
local stop = e.symbol('wldcore', 'wldcore_sound_novel_stop_music')
local driverStop = e.symbol('main', 'main_sound_stop_current_music')
local driverUnload = e.symbol('main', 'main_sound_unload_scenario_mus')
local report = e.symbol('main', 'main_system_handle_error')

local values = { calls_returned = 0 }
local saved
local queue, record
local calling = false

local function readBytes(at, count)
    local bytes = {}
    for i = 0, count - 1 do bytes[i] = e.read8(at + i) end
    return bytes
end

local function writeBytes(at, bytes, count)
    for i = 0, count - 1 do e.write8(at + i, bytes[i]) end
end

-- The driver's functions return at once while the call runs.
local function skip(symbol, label)
    e.breakAt(symbol.address, label, function()
        if calling then
            local regs = PCSX.getRegisters()
            regs.pc = regs.GPR.n.ra
        end
        return true
    end)
end

local calls = {
    { target = stop, label = 'stop the novel\'s music',
      args = function()
          queue = e.address('wldcore', 'g_wldcore_audio_queue')
          record = e.address('wldcore', 'g_wldcore_sound_novel_record')
          saved = { music = readBytes(queue + MUSIC, MUSIC_BYTES), record = readBytes(record, RECORD_BYTES) }
          e.write32(queue + TRACK_1, FIRST_TRACK)
          e.write32(queue + TRACK_2, NOVEL_TRACK)
          e.write32(queue + CURRENT_SLOT, NOVEL_SLOT)
          e.write16(record + SELECTION, NOVEL_SLOT * 256 + NOVEL_TRACK)
          calling = true
          skip(driverStop, 'driver stop')
          skip(driverUnload, 'driver unload')
          return {}
      end,
      done = function(result)
          calling = false
          if result.returned then values.calls_returned = values.calls_returned + 1 end
          values.slot_1_track = e.read32(queue + TRACK_1)
          values.slot_2_track = e.read32(queue + TRACK_2)
          values.current_slot = e.read32(queue + CURRENT_SLOT)
          writeBytes(queue + MUSIC, saved.music, MUSIC_BYTES)
          writeBytes(record, saved.record, RECORD_BYTES)
      end },
}

return {
    why = 'a sound novel that stops its music keeps the unloaded track in its slot (direct call: no novel a treasure opens does)',
    save = { file = 'world-map.mcd', sha256 = 'adee3f05a445925002945e98791b6e8a461e5b3bc46ff28842c879f44243b945' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'wldcore', name = 'wldcore_entrypoint', label = 'world map WLDCORE', before = { BUTTON.CIRCLE },
          budget = 6000, shot = 90 },
    }),
    trigger = {
        e.call({ label = 'novel music', anchor = anchor, calls = calls,
            guards = { { address = report.address, limit = 0, label = 'error report' } } }),
    },
    check = function() return values end,
    expect = {
        original = { slot_1_track = FIRST_TRACK, slot_2_track = NOVEL_TRACK, current_slot = 0, calls_returned = 1 },
        dc = { slot_1_track = FIRST_TRACK, slot_2_track = 0, current_slot = 0, calls_returned = 1 },
    },
}
