-- Retail bug of script_block_start_thread and world_script_block_start_thread
-- (QUIRKS.md): a block's WalkToAnim clears g_unit_was_moving_latch[runtime_id]
-- outside its absent-unit test, so an absent unit (2000) zeroes the 16-bit
-- value 4000 bytes on, the `w` of g_menu_window_buffers[5].sprites[3]. No
-- retail block is known to name an absent unit, so one call makes it at one
-- stop of BATTLE's thread_yield at Gariland's deployment screen (the new-game
-- save); WORLD compiles the same text. The call runs BlockStart's thread body
-- on the anchor's thread with the thread's first parameter pointing to a
-- block written over the effect-message queue, empty there: a WalkToAnim for
-- event unit slot 0, which is empty, then BlockEnd. The body ends in
-- thread_exit_current, where a guard abandons the call. The value 4000 bytes
-- on is set to a mark first, with a watch on its writes; everything the call
-- changed is put back after it. On the original the WalkToAnim writes it to
-- 0; on the Developer's Cut it stays untouched.
local e = emulator
local BUTTON = e.BUTTON

-- WalkToAnim and BlockEnd, event unit slot 0, the absent unit, the block's
-- bytes, a thread's bytes and its task id, the mark and the offset of
-- thread_yield's second instruction.
local WALK_TO_ANIM, BLOCK_END = 0x79, 0x2b
local SLOT_FIRST = 0x64
local ABSENT = 2000
local BLOCK = { WALK_TO_ANIM, SLOT_FIRST, 0, 1, 0, BLOCK_END }
local THREAD_BYTES = 1024
local TASK = 0x4c
local MARK = 0x1234
local YIELD_SECOND_INSTRUCTION = 4

local anchor = e.symbol('battle', 'thread_yield')
local body = e.symbol('battle', 'script_block_start_thread')
local exit = e.symbol('battle', 'thread_exit_current')

local values = {}
local saved = {}
local writes, watching = 0, false
local block, context, latch

local calls = {
    { target = body, label = 'block',
      args = function()
          block = e.address('battle', 'g_battle_action_post_effect_msgs')
          context = e.address('battle', 'g_thread_contexts')
              + THREAD_BYTES * e.read32(e.address('battle', 'g_thread_current_id'))
          latch = e.address('battle', 'g_unit_was_moving_latch') + 2 * ABSENT
          values.slot_0 = e.read16(e.address('battle', 'g_script_unit_slots'))
          saved.block = {}
          for i = 1, #BLOCK do saved.block[i] = e.read8(block + i - 1) end
          saved.parameter, saved.task, saved.latch = e.read32(context), e.read32(context + TASK), e.read16(latch)
          for i, byte in ipairs(BLOCK) do e.write8(block + i - 1, byte) end
          e.write32(context, block)
          e.write16(latch, MARK)
          watching = true
          e.watch(latch, 'Write', 2, 'value 4000 bytes past the latches', function(where, pc)
              if not watching then return false end
              writes = writes + 1
              return true
          end)
          return {}
      end,
      done = function(result)
          watching = false
          values.ended_at_exit = result.guard == 'exit' and 1 or 0
          values.writes_past_latches = writes
          values.value_after = e.read16(latch)
          for i = 1, #BLOCK do e.write8(block + i - 1, saved.block[i]) end
          e.write32(context, saved.parameter)
          e.write32(context + TASK, saved.task)
          e.write16(latch, saved.latch)
      end },
}

return {
    why = 'a block WalkToAnim of an absent unit writes 4000 bytes past the latches (direct call: no retail route is known)',
    save = { file = 'smoke-new-game.mcd', sha256 = '02203c586fc996bd3c3dfa1fb07bb6e0025cb590b69b59dae955fc3b098c47e3' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'event-attack', name = 'attack_deploy_run_screen', label = 'overlay menu ATTACK.OUT deployment',
          before = { BUTTON.CIRCLE }, budget = 9000, shot = 90 },
    }),
    trigger = {
        e.call({ label = 'block start', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' },
                       { address = exit.address, limit = 0, label = 'exit' } } }),
    },
    check = function() return values end,
    expect = {
        original = { slot_0 = 1000, ended_at_exit = 1, writes_past_latches = 1, value_after = 0 },
        dc = { slot_0 = 1000, ended_at_exit = 1, writes_past_latches = 0, value_after = MARK },
    },
}
