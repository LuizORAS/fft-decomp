-- The PCSX-Redux run behind `make scenario`: bring the game to one situation
-- from a memory card save, then read what a scenario checks, with what
-- tools/emulator.lua shares.
--
-- A scenario is tools/scenarios/NAME.lua, a chunk that returns a table:
--   why      what it shows (for a fix, the QUIRKS entry of its bug)
--   save     the memory card save in SAVES that slot 1 holds, on one line in
--            this form, which `tools scenario prepare` reads as text:
--            save = { file = 'NAME.mcd', sha256 = '<64 hex digits>' },
--   setup    the steps from the boot to the situation (emulator.steps)
--   trigger  optional steps from the situation on: where the bug happens
--   check    a function that returns the values it reads, by name
--   expect   the values each disc gives: { original = {...}, dc = {...} }
--
-- `tools scenario prepare` copies the scenario into each disc's run
-- directory (scenario.lua), the save as memcard1.mcd and the situation a
-- passing run on that disc saved before, when there is one (cached.state).
-- Without cached.state the run walks setup and, at the next frame tick,
-- saves the emulator's state to situation.state and loads it back; with it
-- the run loads it at the first frame tick. A state made in the frame tick
-- resumes at the next frame, with the game's RAM as the uninterrupted run
-- had it, so both ways trigger and check run from the same state. The run
-- writes values.txt (name, value, expected) and passes when every value is
-- the one the disc expects; `tools scenario report` keeps a passing run's
-- situation.state for the next run. PCSX-Redux 30177 (2026-11-02) loads
-- these states; the build of 2026-10-03 quits with an access violation
-- loading any state made once the game has left the title screen.
--
-- tools/emulator.sh loads tools/emulator.lua, then this file, and starts a
-- run with -exec "scenario_run([[DIR]], 'DISC')".

local e = emulator
local run = e.run

local function loadScenario(path)
    local text = e.readText(path)
    if not text then error('no scenario at ' .. path) end
    local chunk, err = loadstring(text, '=scenario.lua')
    if not chunk then error(err) end
    local scenario = chunk()
    if type(scenario) ~= 'table' then error('the scenario returns no table') end
    for field, kind in pairs({ why = 'string', save = 'table', setup = 'table', check = 'function', expect = 'table' }) do
        if type(scenario[field]) ~= kind then error(string.format('the scenario has no %s (%s)', field, kind)) end
    end
    if scenario.trigger ~= nil and type(scenario.trigger) ~= 'table' then error('the scenario\'s trigger is no list of steps') end
    return scenario
end

-- Compare check's values with the disc's expected ones, writing values.txt;
-- returns the differences.
local function compare(values, expected)
    local names, seen = {}, {}
    for _, list in ipairs({ expected, values }) do
        for name in pairs(list) do
            if not seen[name] then
                seen[name] = true
                names[#names + 1] = name
            end
        end
    end
    table.sort(names)
    local file = e.writer(run.dir .. '/values.txt')
    local wrong = {}
    for _, name in ipairs(names) do
        local got, want = values[name], expected[name]
        file.write(string.format('%s\t%s\t%s\n', name, tostring(got), tostring(want)))
        e.log('%s = %s (expected %s)', name, tostring(got), tostring(want))
        if got ~= want then wrong[#wrong + 1] = string.format('%s is %s, expected %s', name, tostring(got), tostring(want)) end
    end
    file.close()
    return wrong
end

function scenario_run(dir, disc)
    e.start(dir, 'scenario', function()
        local scenario = loadScenario(dir .. '/scenario.lua')
        local expected = scenario.expect[disc]
        if type(expected) ~= 'table' then error('the scenario expects nothing on the ' .. disc .. ' disc') end
        e.log('%s disc: %s', disc, scenario.why)
        local function situation()
            e.steps(scenario.trigger or {}, function()
                e.settle()
                e.screenshot('check')
                local wrong = compare(scenario.check(), expected)
                if #wrong > 0 then return e.fail(table.concat(wrong, '; ')) end
                e.finish(0, 'pass')
            end)()
        end
        local cached = dir .. '/cached.state'
        if e.exists(cached) then
            e.log('the situation comes from cached.state')
            run.timer = { at = run.frame + 1, call = function() e.loadState(cached, situation) end }
            return
        end
        e.steps(scenario.setup, function()
            e.settle()
            run.timer = { at = run.frame + 1, call = function()
                e.clearBreakpoints()
                local state = dir .. '/situation.state'
                e.saveState(state)
                e.loadState(state, situation)
            end }
        end)()
    end)
end
