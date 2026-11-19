-- Retail bug of battle_state_handle_open_sp2_files_state (QUIRKS.md): it
-- frees its 0x8000-byte SP2 buffer when the read has started and keeps it
-- when the loader refuses the request, so the file lands in the kept block
-- only because the next allocation gets the same cells back. Only monster
-- sheets have SP2 files and the Gariland battle has no monster, so the route
-- is that battle (Auto-battle) up to the action after the first one, and each
-- battle_gfx_open_sp2_file call of that action makes the request that function
-- makes for the first sheet with a 0x8000-byte file (the Bomb's, BOM2.SP2):
-- forcing the sheet alone is not enough, since a human's SEQ has no SP2
-- sequences. The heap, the loader, the CD read, the upload to a free SP2 VRAM
-- slot (which the human does not draw), the strike and the release after the
-- action are the game's own. On the original the first request starts the
-- read and the handler frees the buffer; the next frame allocates the same
-- cells and its request is refused. On the Developer's Cut the buffer stays
-- allocated while the read runs. The read sets the strike frame either way.
local e = emulator
local BUTTON = e.BUTTON

local AUTO_BATTLE = { { row = 4, name = 'Auto-battle' }, { menu = 1, row = 3, name = 'Save fading life' } }

-- Argument registers, an SP2 file extent, the sheets, an SP2 buffer and the
-- game heap's 2 KB cells.
local A0, A1, A2 = 4, 5, 6
local EXTENT_BYTES = 8
local SHEETS = 0x100
local SP2_BYTES = 0x8000
local CELL_SHIFT = 11
local SP2_CELLS = SP2_BYTES / 2048

local handler = e.symbol('battle', 'battle_state_handle_open_sp2_files_state')
local open = e.symbol('battle', 'battle_gfx_open_sp2_file')
local updateUnits = e.symbol('battle', 'battle_state_update_units')
local strike = e.symbol('battle', 'battle_action_start_strike')
local request = e.symbol('main', 'main_file_request_read_bytes')
local alloc = e.symbol('main', 'main_heap_alloc')
local heapFree = e.symbol('main', 'main_heap_free')
local loadImage = e.symbol('main', 'LoadImage')

local values = {}
local active, inside, finished = false, false, false
local calls, firstFrame, struck = 0, 0, false
local buffer, uploaded = 0, 0
local sector, size = 0, 0
local results, states, allocations, frees = {}, {}, 0, 0
local resultHooked = false

local function yes(flag) return flag and 1 or 0 end
local function sp2Data() return e.read32(e.address('battle', 'g_battle_gfx_sp2_data')) end
local function loaderState() return e.read32(e.address('main', 'g_main_file_still_loading')) end

-- Whether every heap cell of the SP2 buffer at block is free.
local function cellsFree(block)
    local base = e.read32(e.address('main', 'g_main_heap_high_overlay_load_address'))
    local first = bit.rshift(block - base, CELL_SHIFT)
    local tags = e.address('main', 'g_main_heap_game_allocator_table')
    for i = 0, SP2_CELLS - 1 do
        if e.read8(tags + first + i) ~= 0 then return false end
    end
    return true
end

-- The first sheet with a 0x8000-byte SP2 file.
local function findMonsterFile()
    local files = e.address('battle', 'g_battle_gfx_sp2_files')
    for sheet = 0, SHEETS - 1 do
        local s, n = e.read32(files + EXTENT_BYTES * sheet), e.read32(files + EXTENT_BYTES * sheet + 4)
        if s ~= 0 and n == SP2_BYTES then
            e.log('sheet 0x%02x has an SP2 file: sector %d, %d bytes', sheet, s, n)
            return s, n, sheet
        end
    end
    error('no sheet has a 0x8000-byte SP2 file')
end

local function install(passed)
    sector, size, values.sheet = findMonsterFile()
    -- From the first handler call with no buffer (the next action), each
    -- call up to battle_state_update_units is inside the handler.
    e.breakAt(handler.address, 'handler', function()
        if finished then return true end
        if not active then
            if sp2Data() ~= 0 then return true end
            active, firstFrame = true, e.run.frame
            e.log('an action enters the handler, casting runtime id %d',
                e.read8(e.address('battle', 'g_battle_casting_runtime_id')))
        end
        calls = calls + 1
        if calls == 2 then
            values.loading_at_second_call = yes(loaderState() ~= 0)
            values.buffer_kept_at_second_call = yes(sp2Data() ~= 0 and sp2Data() == buffer)
            values.cells_free_at_second_call = yes(cellsFree(buffer))
            e.log('second call: buffer 0x%08x, loader %d, cells of 0x%08x free %d', sp2Data(), loaderState(),
                buffer, values.cells_free_at_second_call)
        end
        inside = true
        return true
    end)
    e.breakAt(updateUnits.address, 'update units', function()
        inside = false
        return true
    end)
    -- The open call becomes the read request it makes for the monster's
    -- sheet, returning to the handler with the request's result.
    e.breakAt(open.address, 'open sp2', function()
        if not inside or finished then return true end
        local regs = PCSX.getRegisters()
        local target = tonumber(regs.GPR.n.a1)
        if buffer == 0 then buffer = target end
        states[#states + 1] = loaderState()
        if not resultHooked then
            resultHooked = true
            e.breakAt(tonumber(regs.GPR.n.ra), 'open result', function()
                if inside and not finished then
                    results[#results + 1] = e.signed16(tonumber(PCSX.getRegisters().GPR.n.v0) % 0x10000)
                end
                return true
            end)
        end
        e.log('read sector %d into 0x%08x (loader %d)', sector, target, loaderState())
        e.setGPR(A2, target)
        e.setGPR(A0, sector)
        e.setGPR(A1, size)
        regs.pc = request.address
        return true
    end)
    e.breakAt(alloc.address, 'alloc', function()
        if inside and not finished then allocations = allocations + 1 end
        return true
    end)
    e.breakAt(heapFree.address, 'free', function()
        if not active or finished then return true end
        local block = tonumber(PCSX.getRegisters().GPR.n.a0)
        if inside then
            frees = frees + 1
            e.log('the handler frees 0x%08x', block)
        elseif struck and block == uploaded then
            values.buffer_freed_after_action = 1
            e.log('the buffer 0x%08x is freed after the action', block)
            finished = true
            e.run.timer = { at = e.run.frame + 1, call = passed }
        end
        return true
    end)
    e.breakAt(loadImage.address, 'upload', function()
        if inside and not finished then
            uploaded = tonumber(PCSX.getRegisters().GPR.n.a1)
            values.upload_from_buffer = yes(uploaded == buffer)
        end
        return true
    end)
    e.breakAt(strike.address, 'strike', function()
        if inside and not finished and not struck then
            struck = true
            values.frames_to_strike = e.run.frame - firstFrame
            e.log('strike after %d frames', values.frames_to_strike)
        end
        return true
    end)
end

return {
    why = 'battle_state_handle_open_sp2_files_state frees the SP2 buffer during its read (Gariland route, a Bomb SP2 read)',
    save = { file = 'smoke-new-game.mcd', sha256 = '02203c586fc996bd3c3dfa1fb07bb6e0025cb590b69b59dae955fc3b098c47e3' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'event-attack', name = 'attack_deploy_run_screen', label = 'overlay menu ATTACK.OUT deployment',
          before = { BUTTON.CIRCLE }, budget = 9000, shot = 90 },
        { module = 'battle', name = 'battle_state_enter_open_sp2_files', label = 'an action enters OPEN_SP2_FILES',
          before = { BUTTON.CIRCLE }, budget = 40000, shot = 1, start = function()
              e.startDeployment()
              e.playTurns(AUTO_BATTLE)
          end },
    }),
    trigger = {
        { label = 'a monster SP2 read in the next action', before = { BUTTON.CIRCLE }, budget = 9000,
          start = function(passed)
              e.playTurns(AUTO_BATTLE)
              install(passed)
          end },
    },
    check = function()
        values.open_results = table.concat(results, ' ')
        values.loader_state_at_open = table.concat(states, ' ')
        values.allocations = allocations
        values.handler_frees = frees
        return values
    end,
    expect = {
        original = { sheet = 136, open_results = '0 1', loader_state_at_open = '0 2', allocations = 2,
            handler_frees = 1, loading_at_second_call = 1, buffer_kept_at_second_call = 0,
            cells_free_at_second_call = 1, upload_from_buffer = 1, frames_to_strike = 39,
            buffer_freed_after_action = 1 },
        dc = { sheet = 136, open_results = '0', loader_state_at_open = '0', allocations = 1,
            handler_frees = 0, loading_at_second_call = 1, buffer_kept_at_second_call = 1,
            cells_free_at_second_call = 0, upload_from_buffer = 1, frames_to_strike = 38,
            buffer_freed_after_action = 1 },
    },
}
