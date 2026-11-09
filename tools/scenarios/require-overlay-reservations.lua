-- Retail bugs of require_overlay_open_jobstts and require_overlay_run_helpmenu
-- (QUIRKS.md): before REQUIRE loads JOBSTTS.OUT or HELPMENU.OUT at the game
-- heap's base, it reserves 0x1f000 bytes (62 cells) or 0x10000 bytes (32
-- cells), but the read fills 64 or 49 cells, so the cells past the
-- reservation are written without being claimed. REQUIRE opens both from
-- require_party_apply_post_battle_unit_changes, which first suspends the
-- event script's thread and saves its slot (JOBSTTS reads no input while
-- that thread runs, and the suspended slot is the first idle one): JOBSTTS
-- from the recruit prompt's third row, in thread 6, and the help with
-- Select, in the first idle thread. The Gariland battle recruits nobody, so
-- that function exits at once; the route is that battle (Auto-battle) up to
-- its war trophies, and at the function's entry the calls do what it does:
-- suspend the event thread and save its slot, run JOBSTTS and then the help
-- the way it starts each, with Cross closing each, then restore the slot
-- and resume the thread. The heap, the reads, the screens and the releases
-- are the game's own. On the original JOBSTTS leaves 2 of the cells it reads
-- unclaimed and the help 17; on the Developer's Cut each reserves the whole
-- high window and leaves none. Both release everything on either disc, and
-- the byte past the cell table, which a whole-window release reads, stays.
local e = emulator
local BUTTON = e.BUTTON
local run = e.run

local AUTO_BATTLE = { { row = 4, name = 'Auto-battle' }, { menu = 1, row = 3, name = 'Save fading life' } }

-- The game heap's cells, BATTLE's threads (the event script's, JOBSTTS's,
-- the first idle one), a thread slot and its running word.
local CELLS = 64
local CELL_BYTES = 0x800
local EVENT_THREAD = 1
local JOBSTTS_THREAD = 6
local FIRST_IDLE = 16
local SLOT_BYTES = 0x400
local RUNNING = 0x48

-- Cross presses after each screen's screenshot, until it closes.
local CLOSE = { { nil, 30 }, { BUTTON.CROSS, 60 }, { BUTTON.CROSS, 60 }, { BUTTON.CROSS, 60 },
    { BUTTON.CROSS, 60 }, { BUTTON.CROSS, 60 } }

local anchor = e.symbol('event-require', 'require_party_apply_post_battle_unit_changes')
local suspend = e.symbol('battle', 'thread_suspend')
local resume = e.symbol('battle', 'thread_resume')
local copy = e.symbol('battle', 'mem_copy_bytes_nonempty')
local resolve = e.symbol('battle', 'battle_thread_resolve_id')
local start = e.symbol('battle', 'thread_start')
local wait = e.symbol('battle', 'thread_wait_until_inactive')
local jobstts = e.symbol('event-require', 'require_overlay_open_jobstts')
local helpmenu = e.symbol('event-require', 'require_overlay_run_helpmenu')
local reserve = e.symbol('main', 'main_heap_reserve_at')
local request = e.symbol('main', 'main_file_request_read_bytes')
local heapFree = e.symbol('main', 'main_heap_free')

local values = {}
local screen = nil
local helpThread = 0
local savedSlot = {}
local hooked = {}

local function base() return e.read32(e.address('main', 'g_main_heap_high_overlay_load_address')) end
local function tags() return e.address('main', 'g_main_heap_game_allocator_table') end
local function used()
    local n = 0
    for i = 0, CELLS - 1 do if e.read8(tags() + i) ~= 0 then n = n + 1 end end
    return n
end
local function byteAfterTable() return e.read8(tags() + CELLS) end
local function slot(thread) return e.read32(e.address('battle', 'g_threads')) + SLOT_BYTES * thread end
local function running(thread) return e.read32(slot(thread) + RUNNING) end
local function slotWords(thread)
    local words = {}
    for i = 0, SLOT_BYTES / 4 - 1 do words[i] = e.read32(slot(thread) + 4 * i) end
    return words
end

-- Run body once at the next return to address.
local function onReturn(address, label, body)
    if not hooked[address] then
        e.breakAt(address, label, function()
            local next = hooked[address].body
            hooked[address].body = nil
            if next then next() end
            return true
        end)
    end
    hooked[address] = { body = body }
end

-- While a screen is open: its reservation at the heap's base, its read there
-- and the release of the base.
local function install()
    e.breakAt(reserve.address, 'reserve', function()
        local n = PCSX.getRegisters().GPR.n
        if not screen or tonumber(n.a0) ~= base() then return true end
        local what = screen
        values[what .. '_reserve_bytes'] = string.format('0x%x', tonumber(n.a1))
        onReturn(tonumber(n.ra), 'reserve return', function()
            values[what .. '_reserved_cells'] = used()
            e.log('%s reserves 0x%x bytes: %d cells', what, tonumber(n.a1), used())
        end)
        return true
    end)
    e.breakAt(request.address, 'request', function()
        local n = PCSX.getRegisters().GPR.n
        if not screen or tonumber(n.a2) ~= base() then return true end
        local cells = math.ceil(tonumber(n.a1) / CELL_BYTES)
        local free = 0
        for i = 0, cells - 1 do if e.read8(tags() + i) == 0 then free = free + 1 end end
        values[screen .. '_read_cells'] = cells
        values[screen .. '_unclaimed_cells'] = free
        e.log('%s reads 0x%x bytes from sector %d: %d cells, %d unclaimed', screen, tonumber(n.a1), tonumber(n.a0),
            cells, free)
        return true
    end)
    e.breakAt(heapFree.address, 'free', function()
        local n = PCSX.getRegisters().GPR.n
        if not screen or tonumber(n.a0) ~= base() then return true end
        local what = screen
        onReturn(tonumber(n.ra), 'free return', function()
            values[what .. '_cells_after_free'] = used()
            values[what .. '_byte_after_table'] = byteAfterTable()
            e.log('%s frees the base: %d cells in use, byte past the table %d', what, used(), byteAfterTable())
        end)
        return true
    end)
end

-- Open a screen's thread and wait for it, Cross closing the screen.
local function shown(what, thread)
    return { target = wait, label = 'wait for ' .. what, args = function()
        table.insert(run.pending, { label = 'require-' .. what, at = run.frame + 150 })
        run.queue = CLOSE
        return { thread() }
    end, done = function(r)
        values[what .. '_closed'] = r.returned and 1 or 0
        screen = nil
    end }
end

local step = e.call({ label = 'REQUIRE screens', anchor = anchor, frames = 6000, calls = {
    { target = suspend, label = 'suspend the event thread', args = function()
          run.pattern = nil
          install()
          return { EVENT_THREAD }
      end, done = function() values.event_thread_running_in_screens = running(EVENT_THREAD) end },
    { target = copy, label = 'save the event thread', args = function()
          savedSlot = slotWords(EVENT_THREAD)
          return { e.address('event-require', 'g_require_saved_thread_block'), slot(EVENT_THREAD), SLOT_BYTES }
      end },
    { target = start, label = 'start JOBSTTS', args = function()
          screen = 'jobstts'
          return { JOBSTTS_THREAD, jobstts.address }
      end },
    shown('jobstts', function() return JOBSTTS_THREAD end),
    { target = resolve, label = 'first idle thread', args = { FIRST_IDLE },
      done = function(r)
          helpThread = r.v0 or -1
          values.help_thread = helpThread
      end },
    { target = start, label = 'start the help', args = function()
          screen = 'helpmenu'
          return { helpThread, helpmenu.address }
      end },
    shown('helpmenu', function() return helpThread end),
    { target = copy, label = 'restore the event thread',
      args = function()
          return { slot(EVENT_THREAD), e.address('event-require', 'g_require_saved_thread_block'), SLOT_BYTES }
      end, done = function()
          local now, same = slotWords(EVENT_THREAD), 1
          for i = 0, SLOT_BYTES / 4 - 1 do if now[i] ~= savedSlot[i] then same = 0 end end
          values.event_thread_slot_restored = same
      end },
    { target = resume, label = 'resume the event thread', args = { EVENT_THREAD },
      done = function() values.event_thread_running_after = running(EVENT_THREAD) end },
} })
step.before = { BUTTON.CIRCLE }

local shared = { jobstts_read_cells = 64, jobstts_closed = 1, jobstts_cells_after_free = 0,
    jobstts_byte_after_table = 128, helpmenu_read_cells = 49, helpmenu_closed = 1, helpmenu_cells_after_free = 0,
    helpmenu_byte_after_table = 128, event_thread_running_in_screens = 0, help_thread = EVENT_THREAD,
    event_thread_slot_restored = 1, event_thread_running_after = 1, cells_at_check = 0 }

local function expect(own)
    for k, v in pairs(shared) do own[k] = v end
    return own
end

return {
    why = 'require_overlay_open_jobstts and require_overlay_run_helpmenu reserve less than they read (Gariland route, '
        .. 'calls at require_party_apply_post_battle_unit_changes)',
    save = { file = 'smoke-new-game.mcd', sha256 = '02203c586fc996bd3c3dfa1fb07bb6e0025cb590b69b59dae955fc3b098c47e3' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'event-attack', name = 'attack_deploy_run_screen', label = 'overlay menu ATTACK.OUT deployment',
          before = { BUTTON.CIRCLE }, budget = 9000, shot = 90 },
        { module = 'event-require', name = 'require_reward_render_war_trophies', label = 'REQUIRE war trophies',
          before = { BUTTON.CIRCLE }, budget = 60000, shot = 1, start = function()
              e.startDeployment()
              e.playTurns(AUTO_BATTLE)
          end },
    }),
    trigger = { step },
    check = function()
        values.cells_at_check = used()
        return values
    end,
    expect = {
        original = expect({ jobstts_reserve_bytes = '0x1f000', jobstts_reserved_cells = 62,
            jobstts_unclaimed_cells = 2, helpmenu_reserve_bytes = '0x10000', helpmenu_reserved_cells = 32,
            helpmenu_unclaimed_cells = 17 }),
        dc = expect({ jobstts_reserve_bytes = '0x20000', jobstts_reserved_cells = 64, jobstts_unclaimed_cells = 0,
            helpmenu_reserve_bytes = '0x20000', helpmenu_reserved_cells = 64, helpmenu_unclaimed_cells = 0 }),
    },
}
