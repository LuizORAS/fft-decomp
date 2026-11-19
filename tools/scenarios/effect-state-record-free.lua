-- Retail bug of battle_effect_free_state_record (QUIRKS.md): after freeing a
-- work record it follows that record's next link, which the free has just
-- pointed at the free chain. No retail route is known to free a state record
-- that still holds two work records, so the calls make one. The state and work
-- records live in the effect window, which ATTACK.OUT holds at the deployment
-- screen, so the calls run while the first effect of the Gariland battle plays
-- (Auto-battle). BATTLE's thread_yield does not run during an effect, so the
-- anchor is battle_effect_update_stage. At one stop the calls start a state
-- record, attach two work records and free the state record.
-- battle_effect_alloc_work_record leaves the kind to its caller, so the
-- scenario writes the no-data kind, as the emitter writes its own. On the
-- original the walk then reaches the free chain: each free of a record that
-- is not one of the two returns at once, so nothing is freed twice and the
-- stale kinds of those records release nothing; the guard stays as a net. The
-- original frees the newer record, walks the free chain and leaves the older
-- one counted and out of the free chain; the Developer's Cut frees both.
local e = emulator
local BUTTON = e.BUTTON

local AUTO_BATTLE = { { row = 4, name = 'Auto-battle' }, { menu = 1, row = 3, name = 'Save fading life' } }

-- A work record's next link and kind, its no-data kind, and how many work
-- records there are.
local NEXT = 0x04
local KIND_FLAGS = 0x4c
local KIND_NO_DATA = 0x5000
local WORK_RECORDS = 256

local anchor = e.symbol('battle', 'battle_effect_update_stage')
local freeWork = e.symbol('battle', 'battle_effect_free_work_record')
local values = {}
local index, works, countBefore, headBefore = 0, {}, 0, 0
local inside, own, stray, hooked = false, 0, 0, false

local function yes(flag) return flag and 1 or 0 end
local function workCount() return e.read32(e.address('battle', 'g_battle_effect_work_record_count')) end
local function allocatedHead() return e.read16(e.address('battle', 'g_battle_effect_allocated_record_head')) end

-- Whether the free chain of work records holds work, walked from its head.
local function inFreeChain(work)
    local at = e.read32(e.address('battle', 'g_battle_effect_free_work_record_head'))
    for _ = 1, WORK_RECORDS do
        if at == 0 then return false end
        if at == work then return true end
        at = e.read32(at + NEXT)
    end
    return false
end

-- Counts each free of one of the two work records and returns at once from
-- a free of any other record while the state record is freed.
local function hookFrees()
    if hooked then return end
    hooked = true
    e.breakAt(freeWork.address, 'free work record', function()
        if not inside then return true end
        local regs = PCSX.getRegisters()
        local work = tonumber(regs.GPR.n.a1)
        if work == works[1] or work == works[2] then
            own = own + 1
            e.log('free work record 0x%08x (one of the two)', work)
        else
            stray = stray + 1
            if stray <= 4 then e.log('free work record 0x%08x (free chain, returned at once)', work) end
            regs.pc = regs.GPR.n.ra
        end
        return true
    end)
end

local function allocateWork(which)
    return { target = e.symbol('battle', 'battle_effect_alloc_work_record'),
        args = function() return { index, KIND_NO_DATA } end,
        done = function(result)
            works[which] = result.v0
            if result.v0 ~= 0 then e.write16(result.v0 + KIND_FLAGS, KIND_NO_DATA) end
        end }
end

return {
    why = 'battle_effect_free_state_record walks into the free chain (direct calls during an effect: no retail route is known)',
    save = { file = 'smoke-new-game.mcd', sha256 = '02203c586fc996bd3c3dfa1fb07bb6e0025cb590b69b59dae955fc3b098c47e3' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'event-attack', name = 'attack_deploy_run_screen', label = 'overlay menu ATTACK.OUT deployment',
          before = { BUTTON.CIRCLE }, budget = 9000, shot = 90 },
        { module = 'battle', name = 'battle_effect_run_block', label = 'an effect runs', before = { BUTTON.CIRCLE },
          budget = 40000, shot = 1, start = function()
              e.startDeployment()
              e.playTurns(AUTO_BATTLE)
          end },
    }),
    trigger = {
        e.call({ label = 'free state record', anchor = anchor,
            guards = { { address = freeWork.address, limit = 400, label = 'free walk' } },
            calls = {
                { target = e.symbol('battle', 'battle_effect_start_script_record'),
                  args = function()
                      values.phase = e.read32(e.address('battle', 'g_battle_effect_playback_phase'))
                      headBefore = allocatedHead()
                      countBefore = workCount()
                      return { e.read32(e.address('battle', 'g_battle_effect_script')), 0, 0 }
                  end,
                  done = function(result) index = result.v0 % 0x10000 end },
                allocateWork(1),
                allocateWork(2),
                { target = e.symbol('battle', 'battle_effect_free_state_record'),
                  args = function()
                      hookFrees()
                      inside = true
                      return { index }
                  end,
                  done = function(result)
                      inside = false
                      e.log('state record %d, work records 0x%08x 0x%08x', index, works[1] or 0, works[2] or 0)
                      values.record_started = yes(index ~= 0)
                      values.work_allocated = yes((works[1] or 0) ~= 0 and (works[2] or 0) ~= 0)
                      values.free_returned = yes(result.returned)
                      values.guard = result.guard or 'none'
                      values.own_frees = own
                      values.free_chain_frees = stray
                      values.older_in_free_chain = yes(inFreeChain(works[1]))
                      values.newer_in_free_chain = yes(inFreeChain(works[2]))
                      values.record_unlinked = yes(allocatedHead() == headBefore)
                      values.work_count_restored = yes(workCount() == countBefore)
                  end },
            } }),
    },
    check = function() return values end,
    expect = {
        original = { phase = 4, record_started = 1, work_allocated = 1, free_returned = 1, guard = 'none',
            own_frees = 1, free_chain_frees = 254, older_in_free_chain = 0, newer_in_free_chain = 1,
            record_unlinked = 1, work_count_restored = 0 },
        dc = { phase = 4, record_started = 1, work_allocated = 1, free_returned = 1, guard = 'none',
            own_frees = 2, free_chain_frees = 0, older_in_free_chain = 1, newer_in_free_chain = 1,
            record_unlinked = 1, work_count_restored = 1 },
    },
}
