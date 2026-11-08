-- Retail bug of battle_effect_remove_secondary (QUIRKS.md): removing a record
-- from the middle of the active chain leaves the record before it naming the
-- freed one as its next. No retail route is known to remove records in that
-- order, so the calls make it: at the Gariland deployment screen, where no
-- effect runs, one stop of BATTLE's thread_yield builds the chains afresh
-- (battle_effect_init_system), takes three records, removes the middle one
-- and then the oldest, reads both chains and builds them afresh again. The
-- calls run back to back because the game's own update removes every record
-- whose handler ends at the next frame, and these records have handler 0.
-- On the original the newest record still names the freed oldest one as the
-- record before it; the Developer's Cut relinks the chain at the first
-- removal.
local e = emulator
local BUTTON = e.BUTTON

-- A record of g_battle_effect_secondary_data, its previous_slot_id, and the
-- records the chains hold.
local RECORD_BYTES = 84
local PREVIOUS = 0x00
local RECORDS = 16

local anchor = e.symbol('battle', 'thread_yield')
local values = {}
local ids = {}
local returned = 0

local function yes(flag) return flag and 1 or 0 end

local function record(id) return e.address('battle', 'g_battle_effect_secondary_data') + RECORD_BYTES * id end
local function previous(id) return e.read8(record(id) + PREVIOUS) end

-- The ids a chain holds from its first record through previous_slot_id, at
-- most one per record so a loop ends.
local function chain(first)
    local found, id = {}, first
    while id ~= 0 and #found <= RECORDS do
        found[#found + 1] = id
        id = previous(id)
    end
    return found
end

local function activeChain() return chain(e.read16(e.address('battle', 'g_battle_effect_secondary_active_tail'))) end
local function freeChain() return chain(e.read16(e.address('battle', 'g_battle_effect_secondary_free_head'))) end

local function counted(result)
    if result.returned then returned = returned + 1 end
end

local function allocate()
    return { target = e.symbol('battle', 'battle_effect_allocate_secondary_slot'),
        done = function(result)
            counted(result)
            ids[#ids + 1] = result.v0 % 0x100
        end }
end

local function remove(which, done)
    return { target = e.symbol('battle', 'battle_effect_remove_secondary'),
        args = function() return { ids[which] } end, done = done }
end

-- What the two removals left: the active chain, the freed records it still
-- reaches, and the free chain.
local function readChains(result)
    counted(result)
    local active, free = activeChain(), freeChain()
    local freed = {}
    for _, id in ipairs(free) do freed[id] = true end
    local reached = 0
    for _, id in ipairs(active) do
        if freed[id] then reached = reached + 1 end
    end
    values.allocated = table.concat(ids, ' ')
    values.active_length = #active
    values.freed_in_active_chain = reached
    values.newest_previous = previous(ids[3])
    values.free_length = #free
    values.sentinel_previous = previous(0)
end

return {
    why = 'battle_effect_remove_secondary leaves a stale next link (direct calls: no retail route is known)',
    save = { file = 'smoke-new-game.mcd', sha256 = '02203c586fc996bd3c3dfa1fb07bb6e0025cb590b69b59dae955fc3b098c47e3' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'event-attack', name = 'attack_deploy_run_screen', label = 'overlay menu ATTACK.OUT deployment',
          before = { BUTTON.CIRCLE }, budget = 9000, shot = 90 },
    }),
    trigger = {
        e.call({ label = 'secondary effect chain', anchor = anchor, calls = {
            { target = e.symbol('battle', 'battle_effect_init_system'),
              args = function()
                  values.effects_idle_before = yes(#activeChain() == 0
                      and e.read8(e.address('battle', 'g_battle_effect_active_slot_head')) == 0)
                  return {}
              end,
              done = counted },
            allocate(), allocate(), allocate(),
            remove(2, counted),
            remove(1, readChains),
            { target = e.symbol('battle', 'battle_effect_init_system'),
              done = function(result)
                  counted(result)
                  values.clean_after = yes(#activeChain() == 0 and #freeChain() == RECORDS)
                  values.calls_returned = returned
              end },
        } }),
    },
    check = function() return values end,
    expect = {
        original = { effects_idle_before = 1, allocated = '1 2 3', active_length = 2, freed_in_active_chain = 1,
            newest_previous = 1, free_length = 15, sentinel_previous = 0, clean_after = 1, calls_returned = 7 },
        dc = { effects_idle_before = 1, allocated = '1 2 3', active_length = 1, freed_in_active_chain = 0,
            newest_previous = 0, free_length = 15, sentinel_previous = 0, clean_after = 1, calls_returned = 7 },
    },
}
