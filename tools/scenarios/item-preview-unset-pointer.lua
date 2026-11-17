-- Retail bug of battle_menu_preview_attack_caster_stats and
-- world_menu_preview_attack_caster_stats (QUIRKS.md): for an item the
-- ability's secondary-data pointer is never set, yet its flags_3 is read
-- through it before the item test. The read never changes the result, so no
-- retail screen shows it; one stop of BATTLE's thread_yield at Gariland's
-- deployment screen (the new-game save) makes it with two calls (WORLD
-- compiles the same text). No unit is in the runtime list there, so runtime
-- record 0 is linked as its only unit, with stats record 0, and selected:
-- battle_unit_get_runtime_action_command_by_battle_id gives its action, which
-- becomes the first item with the equipment-change skillset (so the preview
-- never runs the projection). The stack below the second call is filled with
-- the address of the effect-message queue, empty there, with a watch on its
-- reads; the call is abandoned at its first yield, after the test. Everything
-- the calls changed is put back after them. On the original the test reads
-- flags_3 through the unset pointer once; on the Developer's Cut never. Both
-- publish no preview.
local e = emulator
local BUTTON = e.BUTTON

-- A runtime record's bytes (the in-use word sits in the four bytes before
-- it), its stats link, the stats record's battle id, the selected unit's
-- battle id, the first item and the equipment-change skillset with their
-- offsets in the action, the stack the calls begin with below the anchor's,
-- the bytes filled below it and watched at the mark, and the offset of
-- thread_yield's second instruction.
local UNIT_BYTES = 1088
local IN_USE = -4
local STATS = 0x134
local STATS_BATTLE_ID = 0x18a
local SELECTED_BATTLE_ID = 0x0a
local SKILLSET, ABILITY = 1, 2
local EQUIP_CHANGE = 3
local ITEM_FIRST = 368
local CALL_FRAME = 32
local FILL_BYTES = 1024
local MARK_BYTES = 8
local YIELD_SECOND_INSTRUCTION = 4

local anchor = e.symbol('battle', 'thread_yield')
local lookup = e.symbol('battle', 'battle_unit_get_runtime_action_command_by_battle_id')
local preview = e.symbol('battle', 'battle_menu_preview_attack_caster_stats')
local report = e.symbol('main', 'main_system_handle_error')

local values = {}
local saved = {}
local reads, watching = 0, false
local units, head, selected, published, mark, stack, action

local calls = {
    { target = lookup, label = 'action lookup',
      args = function()
          units = e.address('battle', 'g_battle_units')
          head = e.address('battle', 'g_battle_unit_list_head')
          selected = e.address('battle', 'g_menu_selected_unit') + SELECTED_BATTLE_ID
          published = e.address('battle', 'g_battle_menu_preview_target_action')
          mark = e.address('battle', 'g_battle_action_post_effect_msgs')
          local stats = e.address('battle', 'g_battle_unit_stats')
          stack = tonumber(PCSX.getRegisters().GPR.n.sp) - CALL_FRAME
          stack = stack - stack % 8
          values.list_head_at_start = e.read32(head)
          saved.head, saved.selected, saved.published = e.read32(head), e.read16(selected), e.read32(published)
          saved.record = {}
          for i = IN_USE, UNIT_BYTES - 1 do saved.record[i] = e.read8(units + i) end
          local battleId = e.read8(stats + STATS_BATTLE_ID)
          e.write32(units, 0)
          e.write32(units + STATS, stats)
          e.write32(head, units)
          e.write16(selected, battleId)
          return { battleId }
      end,
      done = function(result)
          action = result.returned and result.v0 % 0x100000000 or 0
          values.action_found = action ~= 0 and 1 or 0
      end },
    { target = preview, label = 'item preview',
      args = function()
          saved.action = { e.read8(action + SKILLSET), e.read16(action + ABILITY) }
          e.write8(action + SKILLSET, EQUIP_CHANGE)
          e.write16(action + ABILITY, ITEM_FIRST)
          saved.fill = {}
          for at = stack - FILL_BYTES, stack - 4, 4 do
              saved.fill[at] = e.read32(at)
              e.write32(at, mark)
          end
          watching = true
          e.watch(mark, 'Read', MARK_BYTES, 'fields through the unset pointer', function(where, pc)
              if not watching then return false end
              reads = reads + 1
              return true
          end)
          return {}
      end,
      done = function(result)
          watching = false
          values.stopped_at_yield = result.guard == 'wait' and 1 or 0
          values.unset_pointer_reads = reads
          values.published = e.read32(published)
          for at, word in pairs(saved.fill) do e.write32(at, word) end
          e.write8(action + SKILLSET, saved.action[1])
          e.write16(action + ABILITY, saved.action[2])
          for i = IN_USE, UNIT_BYTES - 1 do e.write8(units + i, saved.record[i]) end
          e.write32(head, saved.head)
          e.write16(selected, saved.selected)
          e.write32(published, saved.published)
      end },
}

return {
    why = 'the attack preview of an item reads through an unset pointer (direct calls: no retail screen shows it)',
    save = { file = 'smoke-new-game.mcd', sha256 = '02203c586fc996bd3c3dfa1fb07bb6e0025cb590b69b59dae955fc3b098c47e3' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'event-attack', name = 'attack_deploy_run_screen', label = 'overlay menu ATTACK.OUT deployment',
          before = { BUTTON.CIRCLE }, budget = 9000, shot = 90 },
    }),
    trigger = {
        e.call({ label = 'item preview', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' },
                       { address = report.address, limit = 0, label = 'error report' } } }),
    },
    check = function() return values end,
    expect = {
        original = { list_head_at_start = 0, action_found = 1, stopped_at_yield = 1, unset_pointer_reads = 1, published = 0 },
        dc = { list_head_at_start = 0, action_found = 1, stopped_at_yield = 1, unset_pointer_reads = 0, published = 0 },
    },
}
