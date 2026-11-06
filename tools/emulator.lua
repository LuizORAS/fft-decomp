-- What the PCSX-Redux runs of `make smoke` and `make scenario` share: files
-- and the log, RAM and registers, the names of the disc's link, the pad,
-- breakpoints and checkpoints, the steps a run walks, the game screens they
-- drive (the title, Continue, menus, a battle's turns and deployment, the
-- world map), the BIOS calls OpenBIOS lacks, savestates and the frame tick.
--
-- tools/emulator.sh loads this file with -dofile, then the run's script
-- (smoke.lua, scenario.lua), whose entry calls emulator.start(DIR, NAME,
-- BEGIN). DIR is the run's directory, with the disc's names.tsv one level
-- up. A run writes log.txt, checkpoints.txt, result.txt and raw screenshots
-- there, then quits: 0 passed, 1 failed, 2 the emulator paused, 3 a script
-- error.
--
-- `tools smoke prepare` and `tools scenario prepare` write each disc's names
-- (names.tsv: module, name, address and the first bytes of a function's
-- code) from that disc's own link, so one script runs on every disc. A
-- checkpoint is an Exec breakpoint at a function. It passes when the bytes in
-- RAM are that function's: overlays share addresses, and another module's
-- code at the same address runs through the breakpoint too. A screenshot
-- follows a few frames later. Each step taps the buttons that lead to the
-- next checkpoint, so the run waits on the game, never on a frame count. A
-- pause of the emulator (a CPU exception, a break of a shifted disc's filler,
-- the BIOS stopping) fails the run, and so does a step that runs past its
-- budget.
--
-- Every hook is a breakpoint, the frame tick included, and one fires only
-- while the script holds the object PCSX.addBreakpoint returns. A breakpoint
-- invoker is a LuaJIT FFI callback, which runs on the thread of the last FFI
-- call; an event listener that calls FFI functions leaves the invokers on
-- the coroutine every event shares, whose stack then fills until Lua stops.
-- So a run is a list of steps the frame tick walks, never a coroutine, and
-- the listeners only set flags.

local bit = require('bit')

local BUTTON = PCSX.CONSTS.PAD.BUTTON
local pad = PCSX.SIO0.slots[1].pads[1]

-- Frames a tap holds its button, and the frames from one tap to the next.
local TAP_HOLD = 4
local TAP_PERIOD = 24
-- Frames before a menu takes input, and between the presses that walk its
-- cursor.
local MENU_SETTLE = 40
local MENU_STEP = 16
-- A battle menu's entry in the current menu table (world_menu_entry_t) and
-- its cursor row.
local MENU_ENTRY_BYTES = 60
local MENU_SELECTED_INDEX = 0x38
-- The run's whole budget, and the frames between the log's heartbeat lines
-- (the runner closes an emulator whose log stops).
local RUN_BUDGET = 110000
local HEARTBEAT = 1800
-- Frames a savestate may take to load.
local LOAD_BUDGET = 5

local run = {
    frame = 0,
    names = {},
    effects = {},
    breakpoints = {},
    checkpoints = 0,
    pending = {},
    expected = {},
}

local function writer(path)
    if io and io.open then
        local file = io.open(path, 'wb')
        if file then
            return {
                write = function(text) file:write(text); file:flush() end,
                close = function() file:close() end,
            }
        end
    end
    local file = Support.File.open(path, 'TRUNCATE')
    return { write = function(text) file:write(text) end, close = function() file:close() end }
end

local function readText(path)
    if io and io.open then
        local file = io.open(path, 'rb')
        if file then
            local text = file:read('*a')
            file:close()
            return text
        end
    end
    local file = Support.File.open(path, 'READ')
    if file:failed() then return nil end
    local text = tostring(file:read(file:size()))
    file:close()
    return text
end

local function exists(path)
    local file = Support.File.open(path, 'READ')
    local found = not file:failed()
    file:close()
    return found
end

local function log(format, ...)
    local line = string.format('%6d ' .. format, run.frame, ...)
    print(run.name .. ': ' .. line)
    if run.log then run.log.write(line .. '\n') end
end

-- RAM and registers

local memory = PCSX.getMemPtr()

local function physical(address) return bit.band(address, 0x1fffff) end
local function read8(address) return memory[physical(address)] end
local function read16(address)
    local offset = physical(address)
    return memory[offset] + memory[offset + 1] * 0x100
end
local function read32(address)
    local offset = physical(address)
    return memory[offset] + memory[offset + 1] * 0x100 + memory[offset + 2] * 0x10000 + memory[offset + 3] * 0x1000000
end
local function write8(address, value) memory[physical(address)] = bit.band(value, 0xff) end
local function write16(address, value)
    local offset = physical(address)
    memory[offset] = bit.band(value, 0xff)
    memory[offset + 1] = bit.band(bit.rshift(value, 8), 0xff)
end
local function write32(address, value)
    local offset = physical(address)
    for i = 0, 3 do memory[offset + i] = bit.band(bit.rshift(value, 8 * i), 0xff) end
end
local function signed16(value) if value >= 0x8000 then return value - 0x10000 end return value end
local function codeAt(address, count)
    local bytes = {}
    for i = 0, count - 1 do bytes[#bytes + 1] = string.format('%02x', read8(address + i)) end
    return table.concat(bytes)
end

-- PCSX.setGPR and a pc store through the cpu are newer than some PCSX-Redux
-- builds, where the registers are plain fields.
local function setGPR(index, value)
    if PCSX.setGPR then PCSX.setGPR(index, value) else PCSX.getRegisters().GPR.r[index] = value end
end

-- Names

local function loadNames(path)
    local text = readText(path)
    if not text then error('no names at ' .. path) end
    local count = 0
    for module, name, address, code in text:gmatch('([^\t\n]+)\t([^\t\n]+)\t0x(%x+)\t([^\t\n]+)\n') do
        local symbol = { module = module, name = name, address = tonumber(address, 16) }
        if code ~= '-' then symbol.code = code end
        run.names[module] = run.names[module] or {}
        run.names[module][name] = symbol
        if symbol.code and module:sub(1, 7) == 'effect-' then
            run.effects[symbol.address] = run.effects[symbol.address] or {}
            table.insert(run.effects[symbol.address], symbol)
        end
        count = count + 1
    end
    return count
end

local function symbol(module, name)
    local found = run.names[module] and run.names[module][name]
    if not found then error('no ' .. module .. ' name ' .. name .. ' in names.tsv') end
    return found
end

local function address(module, name) return symbol(module, name).address end

-- The symbol's own code is in RAM at its address.
local function resident(found) return found.code ~= nil and codeAt(found.address, #found.code / 2) == found.code end

-- Pad

local held = nil
local function hold(button)
    if held == button then return end
    if held ~= nil then pad.clearOverride(held) end
    if button ~= nil then pad.setOverride(button) end
    held = button
end

-- A pattern taps its buttons in turn, one every TAP_PERIOD frames.
local function patternButton(pattern, tick)
    if not pattern or #pattern == 0 then return nil end
    if tick % TAP_PERIOD >= TAP_HOLD then return nil end
    return pattern[math.floor(tick / TAP_PERIOD) % #pattern + 1]
end

-- Ending

local function finish(code, result)
    if run.finished then return end
    run.finished = true
    hold(nil)
    log('result: %s', result)
    local file = writer(run.dir .. '/result.txt')
    file.write(result .. '\n')
    file.close()
    run.checkpointLog.close()
    if run.log then run.log.close() end
    PCSX.quit(code)
end

local function saveShot(stem)
    local shot = PCSX.GPU.takeScreenShot()
    local depth = tonumber(shot.bpp) == 0 and 16 or 24
    local name = string.format('%s/%s.%dx%dx%d.raw', run.dir, stem, shot.width, shot.height, depth)
    local file = Support.File.open(name, 'TRUNCATE')
    file:writeMoveSlice(shot.data)
    file:close()
end

local function screenshot(label) saveShot(string.format('%02d-%s', run.checkpoints, label)) end

-- A failure keeps the screen it ended on.
local function fail(reason)
    if run.finished then return end
    pcall(saveShot, 'zz-fail')
    finish(1, 'fail: ' .. reason)
end

-- Breakpoints

local function guarded(what, body)
    return function(...)
        local ok, result = pcall(body, ...)
        if not ok then
            finish(3, 'script error in ' .. what .. ': ' .. tostring(result))
            return false
        end
        return result
    end
end

-- Break at an address; onHit returns false to remove the breakpoint. The
-- hook holds the breakpoint and knows when the emulator removed it.
local function hook(address, label, onHit)
    local hit = guarded(label, function()
        if run.finished then return false end
        return onHit()
    end)
    local entry = {}
    entry.breakpoint = PCSX.addBreakpoint(address, 'Exec', 4, run.name .. ': ' .. label, function(...)
        local keep = hit(...)
        if not keep then entry.removed = true end
        return keep
    end)
    return entry
end

-- A breakpoint of the steps, which clearBreakpoints removes.
local function breakAt(address, label, onHit)
    local entry = hook(address, label, onHit)
    table.insert(run.breakpoints, entry)
    return entry
end

-- Remove the steps' breakpoints. It runs in the frame tick: a breakpoint
-- removed while its own invoker runs would be removed twice.
local function clearBreakpoints()
    for _, entry in ipairs(run.breakpoints) do
        if not entry.removed then
            entry.removed = true
            entry.breakpoint:remove()
        end
    end
    run.breakpoints = {}
end

-- A checkpoint passes when the function's own code runs; the screenshot waits
-- `shot` frames, with the pad released.
local function checkpoint(label, detail, shot, onPassed)
    run.checkpoints = run.checkpoints + 1
    local line = string.format('%02d %s: %s', run.checkpoints, label, detail)
    log('checkpoint %s', line)
    run.checkpointLog.write(line .. '\n')
    table.insert(run.pending, { label = label:gsub('[^%w]+', '-'), at = run.frame + (shot or 30), after = onPassed })
end

local function breakAtCheckpoint(module, name, label, shot, onPassed)
    local found = symbol(module, name)
    log('armed %s.%s at 0x%08x', module, name, found.address)
    return breakAt(found.address, name, function()
        if not resident(found) then return true end
        checkpoint(label, string.format('%s.%s at 0x%08x, code checked', module, name, found.address), shot, onPassed)
        return false
    end)
end

-- A checkpoint outside the steps' order, which the run's end requires.
local function expect(module, name, label)
    run.expected[label] = false
    breakAtCheckpoint(module, name, label, 30, function() run.expected[label] = true end)
end

-- The checkpoints expect named that never passed.
local function missingExpected()
    local missing = {}
    for label, passed in pairs(run.expected) do
        if not passed then missing[#missing + 1] = label end
    end
    return missing
end

-- OpenBIOS has no B(54h) _get_errno and no B(55h) _get_error, which the
-- game's memory card code calls after each file operation; it halts there.
-- The Sony BIOS returns the error code: here always 0, no error.
local function answerMissingBiosCalls()
    run.bios = hook(0xb0, 'B0 vector', function()
        local regs = PCSX.getRegisters()
        local call = tonumber(regs.GPR.n.t1)
        if call == 0x54 or call == 0x55 then
            local ra = tonumber(regs.GPR.n.ra)
            log('BIOS B0:%02x from 0x%08x answered 0', call, ra)
            setGPR(2, 0)
            regs.pc = ra
        end
        return true
    end)
end

-- Battles

-- On each turn of a player unit the turn menu opens
-- (battle_menu_build_idle_action_menu, its menu id in a0); the plan walks it
-- and the menus after it.
local function playTurns(plan)
    local tables = symbol('battle', 'g_menu_thread_menu_data').address
    local menu = symbol('battle', 'battle_menu_build_idle_action_menu')
    breakAt(menu.address, 'turn menu', function()
        if not resident(menu) then return true end
        local turnMenu = tonumber(PCSX.getRegisters().GPR.n.a0)
        local walk = {}
        for i, target in ipairs(plan) do walk[i] = { menu = target.menu or turnMenu, row = target.row, name = target.name } end
        run.walk = { plan = walk, index = 1, entries = read32(tables) }
        run.queue = { { nil, MENU_SETTLE } }
        return true
    end)
end

-- One press of a menu walk: Down or Up until the menu's cursor is on the row,
-- then Circle, which opens the next menu of the plan. A walk that starts with
-- its menu still opening reads the current menu table when it first steps.
local function walkStep()
    local walk = run.walk
    local target = walk.plan[walk.index]
    if not target then
        run.walk = nil
        return
    end
    walk.entries = walk.entries or read32(walk.tables)
    local row = signed16(read16(walk.entries + target.menu * MENU_ENTRY_BYTES + MENU_SELECTED_INDEX))
    if row < target.row then
        run.queue = { { BUTTON.DOWN, MENU_STEP } }
    elseif row > target.row then
        run.queue = { { BUTTON.UP, MENU_STEP } }
    else
        log('menu %d on row %d, %s', target.menu, target.row, target.name)
        run.queue = { { BUTTON.CIRCLE, MENU_SETTLE } }
        walk.index = walk.index + 1
    end
end

-- The deployment screen (ATTACK.OUT) starts with Ramza alone on the 5x5 grid,
-- and Ramza and the guest lose Gariland alone. Each press fills the squad:
-- R1 selects the next roster unit while the selected one is on the grid (or
-- could not join), the d-pad walks the cursor to an empty valid tile (Left and
-- Up raise the column and the row), Circle places the unit; once the squad is
-- full, Start and Circle begin the battle.
local function startDeployment()
    local function address(name) return symbol('event-attack', name).address end
    run.deploy = {
        valid = address('g_attack_deploy_valid_tiles'),
        tiles = address('g_attack_deploy_roster_id_by_tile'),
        column = address('g_attack_deploy_cursor_column'),
        row = address('g_attack_deploy_cursor_row'),
        selected = address('g_attack_deploy_selected_roster_index'),
        count = address('g_attack_deploy_deployed_unit_count'),
        fieldable = address('g_attack_deploy_fieldable_unit_count'),
        squad = address('g_attack_deploy_current_squad_data'),
        presses = 0,
    }
end

local function deployStep()
    local deploy = run.deploy
    local count = read32(deploy.count)
    local limit = read8(read32(deploy.squad) + 8) -- attack_deployment_squad_data_t.unit_limit
    local fieldable = read32(deploy.fieldable)
    local selected = read32(deploy.selected)
    deploy.presses = deploy.presses + 1
    if count >= limit or count >= fieldable or deploy.presses > 80 then
        log('deployment: %d units on the grid (squad limit %d, %d fieldable)', count, limit, fieldable)
        run.deploy = nil
        run.queue = { { BUTTON.START, MENU_SETTLE }, { BUTTON.CIRCLE, MENU_SETTLE } }
        return
    end
    local placed = false
    local target
    for tile = 0, 24 do
        local unit = read8(deploy.tiles + tile)
        if unit == selected then placed = true end
        if not target and unit == 0xff and read8(deploy.valid + tile) ~= 0 then target = tile end
    end
    -- The last Circle left this unit off the grid: it cannot join.
    local refused = deploy.circled == selected and deploy.circledCount == count
    deploy.circled = nil
    if placed or refused or not target then
        run.queue = { { BUTTON.R1, MENU_SETTLE } }
        return
    end
    local column, row = read32(deploy.column), read32(deploy.row)
    local targetRow, targetColumn = math.floor(target / 5), target % 5
    if column < targetColumn then
        run.queue = { { BUTTON.LEFT, MENU_STEP } }
    elseif column > targetColumn then
        run.queue = { { BUTTON.RIGHT, MENU_STEP } }
    elseif row < targetRow then
        run.queue = { { BUTTON.UP, MENU_STEP } }
    elseif row > targetRow then
        run.queue = { { BUTTON.DOWN, MENU_STEP } }
    else
        log('deployment: roster unit %d on tile %d', selected, target)
        deploy.circled, deploy.circledCount = selected, count
        run.queue = { { BUTTON.CIRCLE, MENU_SETTLE } }
    end
end

-- The world map

-- The first world map shows its tutorial messages, which Circle closes, then
-- frees the map cursor (WLDCORE runs wldcore_menu_step_map_cursor_level each
-- frame). Once the cursor has been free for MAP_SETTLE frames, Triangle opens
-- WORLD's menu (id 4: Move, Formation, Brave Story, Tutorial, Data, Option)
-- and the walk takes the plan's rows.
local MAP_SETTLE = 60

local function openWorldMenu(plan)
    local level = symbol('wldcore', 'wldcore_menu_step_map_cursor_level')
    breakAt(level.address, 'map cursor', function()
        if not resident(level) then return true end
        if run.map then run.map.seen = run.frame end
        return true
    end)
    run.map = { free = 0, plan = plan }
end

local function mapStep()
    local map = run.map
    if map.seen and run.frame - map.seen <= 1 then
        map.free = map.free + 1
        hold(nil)
        if map.free >= MAP_SETTLE then
            log('the map cursor is free: Triangle opens the menu')
            run.map = nil
            run.queue = { { BUTTON.TRIANGLE, MENU_SETTLE } }
            run.walk = { plan = map.plan, index = 1, tables = symbol('world', 'g_menu_thread_menu_data').address }
        end
        return
    end
    map.free = 0
    hold(patternButton({ BUTTON.CIRCLE }, run.frame - run.stepStart))
end

-- Steps

-- A step waits for its function while tapping `before`, then queues `after`,
-- a list of {button, frames} (button nil waits). A step without a function
-- runs start(passed), or waits `frames` frames. The list ends with `done`.
local function steps(list, done)
    local index = 0
    local function nextStep()
        index = index + 1
        local current = list[index]
        if not current then return done() end
        run.pattern = current.before
        run.stepStart = run.frame
        run.budget = current.budget
        run.stepLabel = current.label or current.name or string.format('%d', index)
        local function passed()
            run.queue = current.after and { unpack(current.after) } or {}
            nextStep()
        end
        if current.label and current.name then
            breakAtCheckpoint(current.module, current.name, current.label, current.shot, passed)
        elseif current.name then
            local found = symbol(current.module, current.name)
            breakAt(found.address, current.name, function()
                if not resident(found) then return true end
                log('step %s.%s', current.module, current.name)
                passed()
                return false
            end)
        elseif current.frames then
            run.timer = { at = run.frame + current.frames, call = passed }
        end
        if current.start then current.start(passed) end
    end
    return nextStep
end

-- The lists of steps in one list.
local function join(...)
    local joined = {}
    for _, list in ipairs({ ... }) do
        for _, step in ipairs(list) do joined[#joined + 1] = step end
    end
    return joined
end

-- The title screen: Start taps until its menu runs, then `after`.
local function title(after)
    return { module = 'opening', name = 'open_title_update_menu', label = 'title', before = { BUTTON.START }, budget = 9000,
        shot = 90, after = after }
end

-- Continue from the title to the card screen, which loads the save in slot 1.
local function continueFromTitle()
    return {
        title({ { BUTTON.DOWN, 20 }, { BUTTON.START, 20 } }),
        { module = 'world', name = 'world_card_run_menu_screen', label = 'continue WORLD card screen run by OPEN',
          before = {}, budget = 1800, shot = 60 },
    }
end

-- Nothing of the steps left to do: no button, queue, driver or timer.
local function settle()
    hold(nil)
    run.pattern, run.budget, run.queue, run.timer = nil, nil, nil, nil
    run.walk, run.deploy, run.map = nil, nil, nil
end

-- Savestates

-- The emulator's whole state, written raw to path.
local function saveState(path)
    local state = PCSX.createSaveState()
    local file = Support.File.open(path, 'TRUNCATE')
    file:writeMoveSlice(state)
    file:close()
    log('savestate written to %s', path)
end

-- Load path's state; the emulator loads it once the breakpoint returns, and
-- after() runs at the first frame tick from it.
local function loadState(path, after)
    local file = Support.File.open(path, 'READ')
    if file:failed() then return fail('no savestate at ' .. path) end
    log('loading the savestate %s (%d bytes)', path, tonumber(file:size()))
    run.loading = { path = path, at = run.frame, after = after }
    run.loaded = false
    PCSX.loadSaveState(file)
    file:close()
end

-- Frames

local function frame()
    run.frame = run.frame + 1
    if run.finished then return end
    if run.frame % HEARTBEAT == 0 then log('alive') end
    if run.frame > RUN_BUDGET then return fail('the run passed its budget of ' .. RUN_BUDGET .. ' frames') end
    if run.loading then
        local loading = run.loading
        if not run.loaded then
            if run.frame - loading.at > LOAD_BUDGET then return fail('the savestate ' .. loading.path .. ' did not load') end
            return
        end
        run.loading = nil
        log('savestate loaded')
        return loading.after()
    end
    if run.budget and run.frame - run.stepStart > run.budget then
        return fail(string.format('step %s passed its budget of %d frames', run.stepLabel, run.budget))
    end
    if run.timer and run.frame >= run.timer.at then
        local timer = run.timer
        run.timer = nil
        timer.call()
        if run.finished or run.loading then return end
    end
    local pending = run.pending[1]
    if pending then
        hold(nil)
        if run.frame >= pending.at then
            screenshot(pending.label)
            table.remove(run.pending, 1)
            if pending.after then pending.after() end
        end
        return
    end
    if run.queue and #run.queue > 0 then
        local action = run.queue[1]
        action.start = action.start or run.frame
        local tick = run.frame - action.start
        hold(tick < TAP_HOLD and action[1] or nil)
        if tick >= action[2] then table.remove(run.queue, 1) end
        return
    end
    if run.deploy then return deployStep() end
    if run.map then return mapStep() end
    if run.walk then return walkStep() end
    hold(run.pattern and patternButton(run.pattern, run.frame - run.stepStart) or nil)
end

-- The pause listener runs on the coroutine every event shares; it calls FFI
-- functions only to end the run.
local function paused(event)
    if run.finished then return end
    local regs = PCSX.getRegisters()
    local cause = tonumber(regs.CP0.r[13])
    local what = event.exception and string.format('CPU exception %d', bit.band(bit.rshift(cause, 2), 0x1f))
        or 'the emulator paused'
    finish(2, string.format('%s at pc 0x%08x (epc 0x%08x, cause 0x%08x); emulator.log shows a stop of the BIOS',
        what, tonumber(regs.pc), tonumber(regs.CP0.r[14]), cause))
end

-- Start a run in dir: its log, the disc's names, the frame tick, the pause
-- and savestate listeners and the BIOS answers, then begin().
local function start(dir, name, begin)
    run.dir, run.name = dir, name
    run.log = writer(dir .. '/log.txt')
    run.checkpointLog = writer(dir .. '/checkpoints.txt')
    local ok, err = pcall(function()
        log('%d names', loadNames(dir .. '/../names.tsv'))
        -- The frame tick: libetc's vertical blank handler runs once a frame.
        run.tick = hook(symbol('main', 'trapIntrVSync').address, 'frame tick', function()
            frame()
            return true
        end)
        run.pause = PCSX.Events.createEventListener('ExecutionFlow::Pause', guarded('pause', paused))
        run.loadedListener = PCSX.Events.createEventListener('ExecutionFlow::SaveStateLoaded', function()
            run.loaded = true
        end)
        answerMissingBiosCalls()
        begin()
    end)
    if not ok then finish(3, 'script error: ' .. tostring(err)) end
end

emulator = {
    BUTTON = BUTTON,
    MENU_SETTLE = MENU_SETTLE,
    run = run,
    readText = readText,
    exists = exists,
    writer = writer,
    log = log,
    read8 = read8,
    read16 = read16,
    read32 = read32,
    write8 = write8,
    write16 = write16,
    write32 = write32,
    signed16 = signed16,
    setGPR = setGPR,
    symbol = symbol,
    address = address,
    resident = resident,
    hold = hold,
    finish = finish,
    fail = fail,
    screenshot = screenshot,
    breakAt = breakAt,
    clearBreakpoints = clearBreakpoints,
    checkpoint = checkpoint,
    expect = expect,
    missingExpected = missingExpected,
    playTurns = playTurns,
    startDeployment = startDeployment,
    openWorldMenu = openWorldMenu,
    steps = steps,
    join = join,
    title = title,
    continueFromTitle = continueFromTitle,
    settle = settle,
    saveState = saveState,
    loadState = loadState,
    start = start,
}
