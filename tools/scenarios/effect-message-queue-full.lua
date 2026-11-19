-- Retail bug of battle_action_queue_unit_effect_messages (QUIRKS.md):
-- it tests the 16-record capacity of g_battle_action_post_effect_msgs once,
-- before it walks the unit's outcomes, so with 15 records queued a unit with
-- two changes queues its second message past the array, over
-- g_battle_gfx_compressed_nibble_phase. No retail route to it is known, so
-- one call makes it at one stop of BATTLE's thread_yield at Gariland's
-- deployment screen (the new-game save). The deployment screen links no
-- runtime record to its stats yet, so the call links runtime record 0 to
-- stats record 0, gives that action a hit with a Speed and a CT change and
-- sets the queue to 15 records, with a watch on the writes of the record past
-- the array; everything it changed is put back after the call. On the
-- original the CT message lands past the array and the count reaches 17; on
-- the Developer's Cut the queue stops at 16 and nothing is written past it.
-- Both take the Speed message as the sixteenth record.
local e = emulator
local BUTTON = e.BUTTON

-- The queue's records and one record's bytes, the queue's count before the
-- call, the runtime record's stats link, the action in the stats record and
-- its bytes, the action's hit, Speed and CT bytes, the Speed message code and
-- the offset of thread_yield's second instruction.
local RECORDS = 16
local RECORD_BYTES = 8
local QUEUED = 15
local STATS = 0x134
local ACTION = 0x18c
local ACTION_BYTES = 44
local HIT, SP_CHANGE, CT_CHANGE = 0x00, 0x12, 0x13
local YIELD_SECOND_INSTRUCTION = 4

local anchor = e.symbol('battle', 'thread_yield')
local queue = e.symbol('battle', 'battle_action_queue_unit_effect_messages')

local values = {}
local saved = {}
local writes, watching = 0, false
local runtime, action

local function counter() return e.address('battle', 'g_battle_action_post_effect_msg_counter') end
local function msgs() return e.address('battle', 'g_battle_action_post_effect_msgs') end
local function past() return msgs() + RECORD_BYTES * RECORDS end
local function pastBytes()
    local bytes = {}
    for i = 0, RECORD_BYTES - 1 do bytes[#bytes + 1] = string.format('%02x', e.read8(past() + i)) end
    return table.concat(bytes)
end

local calls = {
    { target = queue, label = 'queue one unit',
      args = function()
          runtime = e.address('battle', 'g_battle_units')
          local stats = e.address('battle', 'g_battle_unit_stats')
          action = stats + ACTION
          saved.stats = e.read32(runtime + STATS)
          saved.counter = e.read32(counter())
          saved.action, saved.past = {}, {}
          for i = 0, ACTION_BYTES - 1 do saved.action[i] = e.read8(action + i) end
          for i = 0, RECORD_BYTES - 1 do saved.past[i] = e.read8(past() + i) end
          saved.pastBytes = pastBytes()
          e.write32(runtime + STATS, stats)
          for i = 0, ACTION_BYTES - 1 do e.write8(action + i, 0) end
          e.write8(action + HIT, 1)
          e.write8(action + SP_CHANGE, 1)
          e.write8(action + CT_CHANGE, 2)
          e.write32(counter(), QUEUED)
          watching = true
          e.watch(past(), 'Write', RECORD_BYTES, 'record past the queue', function(where, pc)
              if not watching then return false end
              writes = writes + 1
              return true
          end)
          return { runtime }
      end,
      done = function()
          watching = false
          values.count_after = e.read32(counter())
          values.record_16_code = e.read8(msgs() + RECORD_BYTES * QUEUED)
          values.writes_past_queue = writes
          values.past_bytes_changed = pastBytes() ~= saved.pastBytes and 1 or 0
          for i = 0, ACTION_BYTES - 1 do e.write8(action + i, saved.action[i]) end
          for i = 0, RECORD_BYTES - 1 do e.write8(past() + i, saved.past[i]) end
          e.write32(counter(), saved.counter)
          e.write32(runtime + STATS, saved.stats)
      end },
}

return {
    why = 'a unit queues an effect message past the full queue (direct call: no retail route is known)',
    save = { file = 'smoke-new-game.mcd', sha256 = '02203c586fc996bd3c3dfa1fb07bb6e0025cb590b69b59dae955fc3b098c47e3' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'event-attack', name = 'attack_deploy_run_screen', label = 'overlay menu ATTACK.OUT deployment',
          before = { BUTTON.CIRCLE }, budget = 9000, shot = 90 },
    }),
    trigger = {
        e.call({ label = 'effect messages', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' } } }),
    },
    check = function() return values end,
    expect = {
        original = { count_after = 17, record_16_code = 0x28, writes_past_queue = 3, past_bytes_changed = 1 },
        dc = { count_after = 16, record_16_code = 0x28, writes_past_queue = 0, past_bytes_changed = 0 },
    },
}
