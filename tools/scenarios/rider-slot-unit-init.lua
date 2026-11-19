-- Retail bug of battle_unit_init (QUIRKS.md): it tints the new unit's
-- palette before it clears mount_state, and battle_unit_claim_slot clears
-- nothing, so a runtime slot last held by a rider looks up a mount that is
-- gone (battle_unit_get_mount answers NULL) and the tint's world height
-- reads the mount's world coordinates through NULL, from kernel RAM. No
-- retail sequence is known to reach it, so one call makes it at one stop of
-- BATTLE's thread_yield at Gariland's deployment screen (the new-game save),
-- where no runtime slot is in use and no spritesheet is loaded: the first free
-- runtime slot is cleared and left as a rider of itself (absent), and
-- battle_unit_init claims it for sheet 1 without file data, so nothing is
-- loaded into the sheet slot it claims. A guard at
-- battle_unit_set_move_and_world_coords abandons the call right after the
-- tint, before the unit joins the list; a watch counts the reads of the
-- fields through NULL. The runtime record, its in-use word and the sheet
-- slot's first two bytes are put back after the call. On the original the
-- tint makes 2 reads through NULL; on the Developer's Cut none.
local e = emulator
local BUTTON = e.BUTTON

-- Runtime slots and their bytes (the in-use word sits in the four bytes
-- before each record), the mount state and partner, a rider, the sheet
-- slots and their bytes, the sheet used, the fields read through NULL and
-- their bytes, and the offset of thread_yield's second instruction.
local UNITS = 16
local UNIT_BYTES = 1088
local IN_USE = -4
local MOUNT_STATE, MOUNT_PARTNER = 0x130, 0x131
local RIDER = 1
local SHEET_SLOTS = 9
local SHEET_SLOT_BYTES = 0x32d6
local SHEET = 1
local NULL_FIELDS = 0x40
local NULL_BYTES = 16
local YIELD_SECOND_INSTRUCTION = 4

local anchor = e.symbol('battle', 'thread_yield')
local init = e.symbol('battle', 'battle_unit_init')
local after = e.symbol('battle', 'battle_unit_set_move_and_world_coords')
local report = e.symbol('main', 'main_system_handle_error')

local values = {}
local saved = {}
local reads, watching = 0, false
local record, sheetSlot

local calls = {
    { target = init, label = 'unit init',
      args = function()
          local units = e.address('battle', 'g_battle_units')
          local sheets = e.address('battle', 'g_battle_gfx_spritesheet_slots')
          local runtime
          for i = 0, UNITS - 1 do
              if e.read32(units + UNIT_BYTES * i + IN_USE) == 0 then runtime = i; break end
          end
          for s = 0, SHEET_SLOTS - 1 do
              if e.read8(sheets + SHEET_SLOT_BYTES * s) == 0 then sheetSlot = sheets + SHEET_SLOT_BYTES * s; break end
          end
          values.runtime = runtime
          record = units + UNIT_BYTES * runtime
          saved.record = {}
          for i = IN_USE, UNIT_BYTES - 1 do saved.record[i] = e.read8(record + i) end
          saved.sheet = { e.read8(sheetSlot), e.read8(sheetSlot + 1) }
          for i = 0, UNIT_BYTES - 1 do e.write8(record + i, 0) end
          e.write8(record + MOUNT_STATE, RIDER)
          e.write8(record + MOUNT_PARTNER, runtime)
          watching = true
          e.watch(NULL_FIELDS, 'Read', NULL_BYTES, 'fields through NULL', function(where, pc)
              if not watching then return false end
              reads = reads + 1
              return true
          end)
          -- map x, map y, level, facing, sheet, palette, runtime id, stats,
          -- undrawn, no file data.
          return { 1, 1, 0, 0, SHEET, 0, runtime, e.address('battle', 'g_battle_unit_stats'), 1, 0 }
      end,
      done = function(result)
          watching = false
          values.stopped_after_tint = result.guard == 'after the tint' and 1 or 0
          values.null_reads = reads
          for i = IN_USE, UNIT_BYTES - 1 do e.write8(record + i, saved.record[i]) end
          e.write8(sheetSlot, saved.sheet[1])
          e.write8(sheetSlot + 1, saved.sheet[2])
      end },
}

return {
    why = 'battle_unit_init on a slot left by a rider reads through NULL (direct call: no retail route is known)',
    save = { file = 'smoke-new-game.mcd', sha256 = '02203c586fc996bd3c3dfa1fb07bb6e0025cb590b69b59dae955fc3b098c47e3' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'event-attack', name = 'attack_deploy_run_screen', label = 'overlay menu ATTACK.OUT deployment',
          before = { BUTTON.CIRCLE }, budget = 9000, shot = 90 },
    }),
    trigger = {
        e.call({ label = 'unit init', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' },
                       { address = after.address, limit = 0, label = 'after the tint' },
                       { address = report.address, limit = 0, label = 'error report' } } }),
    },
    check = function() return values end,
    expect = {
        original = { runtime = 0, stopped_after_tint = 1, null_reads = 2 },
        dc = { runtime = 0, stopped_after_tint = 1, null_reads = 0 },
    },
}
