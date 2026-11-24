-- Retail bug of battle_gfx_build_next_action_result_display (QUIRKS.md): the
-- CT 0 display (Persuade, Return 2, Last Dance) reads y_shift without setting
-- it. The value lives in s8, which neither caller sets, so CT 0 stands at the
-- height the caller's s8 gives (0x801ffff8, so -8, except at a weapon strike,
-- which never queues CT 0) instead of the -32 of Quick and Missed. Gariland
-- has no CT 0 ability, so a call makes it at one stop of BATTLE's
-- thread_yield, from the AI stop of an Auto-battle turn on Gariland's real
-- route (the new-game save): the runtime unit of unit slot 0 builds its next
-- result with only the CT 0 flag queued. The scenario reads the height of the
-- two images (CT and 0) and compares it with the low byte of the s8 the anchor
-- holds; the runtime unit and its three display records are put back after the
-- call. On the original both images take the caller's s8; on the Developer's
-- Cut both stand at -32.
local e = emulator
local BUTTON = e.BUTTON

-- The Auto-battle menu rows; the unit and a unit record's bytes; the runtime
-- units, their count and bytes, the display flags, the three display
-- pointers and the stats link; the bytes of a display record kept and the
-- height of its first part (a signed byte); the CT 0 flag; s8; the offset of
-- thread_yield's second instruction.
local AUTO_BATTLE = { { row = 4, name = 'Auto-battle' }, { menu = 1, row = 3, name = 'Save fading life' } }
local UNIT, STATS_BYTES = 0, 448
local RUNTIME_COUNT, RUNTIME_BYTES, DISPLAY_FLAGS, DISPLAYS, STATS_LINK = 16, 1088, 0x1b8, 0x2c4, 0x134
local DISPLAY_BYTES, PART_0_HEIGHT = 0x20, 0x0f
local CT_ZERO = 0x00200000
local S8 = 30
local YIELD_SECOND_INSTRUCTION = 4

local anchor = e.symbol('battle', 'thread_yield')
local build = e.symbol('battle', 'battle_gfx_build_next_action_result_display')
local report = e.symbol('main', 'main_system_handle_error')

local values = { calls_returned = 0 }
local saved
local runtime, displays, s8

local function readBytes(at, count)
    local bytes = {}
    for i = 0, count - 1 do bytes[i] = e.read8(at + i) end
    return bytes
end

local function writeBytes(at, bytes, count)
    for i = 0, count - 1 do e.write8(at + i, bytes[i]) end
end

local function signed8(value) return value >= 0x80 and value - 0x100 or value end

-- The runtime unit linked to the unit, and its three display records.
local function findRuntime()
    local stats = e.address('battle', 'g_battle_unit_stats') + STATS_BYTES * UNIT
    local base = e.address('battle', 'g_battle_units')
    for slot = 0, RUNTIME_COUNT - 1 do
        local at = base + RUNTIME_BYTES * slot
        if e.read32(at + STATS_LINK) == stats then
            local records = {}
            for i = 0, 2 do
                records[i + 1] = e.read32(at + DISPLAYS + 4 * i)
                if records[i + 1] < 0x80000000 then error('the unit\'s display record ' .. i .. ' is unset') end
            end
            return at, records
        end
    end
    error('no runtime unit links the unit')
end

local calls = {
    { target = build, label = 'CT 0',
      args = function()
          s8 = tonumber(PCSX.getRegisters().GPR.r[S8])
          runtime, displays = findRuntime()
          saved = { runtime = readBytes(runtime, RUNTIME_BYTES), displays = {} }
          for i, at in ipairs(displays) do saved.displays[i] = readBytes(at, DISPLAY_BYTES) end
          e.write32(runtime + DISPLAY_FLAGS, CT_ZERO)
          return { runtime }
      end,
      done = function(result)
          if result.returned then values.calls_returned = values.calls_returned + 1 end
          local ct = signed8(e.read8(displays[2] + PART_0_HEIGHT))
          local zero = signed8(e.read8(displays[3] + PART_0_HEIGHT))
          local leftover = signed8(s8 % 0x100)
          e.log('CT 0: s8 0x%08x, heights %d and %d', s8, ct, zero)
          if ct == leftover and zero == leftover then
              values.ct_zero_height = 'caller s8'
          else
              values.ct_zero_height = string.format('%d %d', ct, zero)
          end
          writeBytes(runtime, saved.runtime, RUNTIME_BYTES)
          for i, at in ipairs(displays) do writeBytes(at, saved.displays[i], DISPLAY_BYTES) end
      end },
}

return {
    why = 'CT 0 stands at the height a caller leaves in s8 (direct call: Gariland has no CT 0 ability)',
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
        e.call({ label = 'CT 0 display', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' },
                       { address = report.address, limit = 0, label = 'error report' } } }),
    },
    check = function() return values end,
    expect = {
        original = { ct_zero_height = 'caller s8', calls_returned = 1 },
        dc = { ct_zero_height = '-32 -32', calls_returned = 1 },
    },
}
