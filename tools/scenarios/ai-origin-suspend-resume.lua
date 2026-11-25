-- Retail bugs of battle_ai_evaluate_self_target_origins and
-- battle_ai_evaluate_reflected_target_origins (QUIRKS.md): both test the
-- acting unit's own tile with battle_ai_can_use_ability_at_coords in the
-- move-then-act pass too; the search around the unit itself, suspended in its
-- first pass, skips the moved pass on resume; and the search through a
-- Reflect unit resumes without reloading the reflector, so it mirrors the
-- next targets through the caller's $s2, the AI workspace, whose bytes 0x47
-- and 0x48 are the best action's target id and target x. A suspension comes
-- from the AI's frame budget and a Reflect unit from late battles, so calls
-- make both at one stop of BATTLE's thread_yield, from the AI stop of an
-- Auto-battle turn on Gariland's real route (the new-game save). While they
-- run, battle_ai_evaluate_target_tiles returns at once (-1 for the first
-- search of a suspended sequence, else 0) and records the pass and, for the
-- reflected search, the origin; battle_ai_can_use_ability_at_coords returns 1
-- and records the tile it gets; Reflect is active on one unit only, every
-- target is useful and no unit moves before the charge lands; the temporary
-- unit's move and the ranking return at once. The acting unit stands at 1,1.
-- Around the unit: the destination is 5,5; a call suspends, a call resumes
-- (resume_pending set, as the caller does), then a call runs whole. Through a
-- Reflect unit: the reflector stands at 3,3 and two targets at 2,3 and 3,2,
-- whose mirrored origins 4,3 and 3,4 are the only reachable tiles; the best
-- action names unit 2 and x 2; a call suspends, then a call resumes with the
-- caller's $s2. The AI workspace, the stage and the three units' tiles are
-- put back after the last call. On the original the resumed search around the
-- unit runs no moved pass, both searches test 1,1 in the moved pass, and the
-- resumed reflected search mirrors the second target to 1,2, unreachable; on
-- the Developer's Cut the resumed search runs its moved pass, each moved
-- pass tests its destination, and the second target's origin is 3,4.
local bit = require('bit')
local e = emulator
local BUTTON = e.BUTTON

-- The Auto-battle menu rows; a unit record's bytes and slots, its x and its
-- y byte; the AI workspace's bytes, the considered ability's flag byte with
-- follow-target (0x1000 of its flags word), the current action's pass and
-- tile, the best action's target id and target x, the reachable tiles (by
-- scenario and level, a row per y with bit 15 - x), the targetable units, the
-- considered tile, the scenario tiles, the scenario, the acting unit and its
-- tile, the decision records and their flags byte 2, and the resume flag;
-- the useful effect; the registers v0 and s2; the offset of thread_yield's
-- second instruction and of the reflected search's second instruction, before
-- it saves s2.
local AUTO_BATTLE = { { row = 4, name = 'Auto-battle' }, { menu = 1, row = 3, name = 'Save fading life' } }
local STATS_BYTES, UNIT_SLOTS = 448, 21
local STATS_X, STATS_Y = 0x047, 0x048
local WORKSPACE_BYTES = 0x19c8
local FLAGS_BYTE_1, FOLLOW_TARGET_BIT = 0x0d, 0x10
local ACT_BEFORE_MOVE, COORDS = 0x14, 0x18
local BEST_TARGET_ID, BEST_TARGET_X = 0x47, 0x48
local REACHABLE, SCENARIO_BYTES, LEVEL_BYTES, ROW_BYTES = 0x0a74, 72, 36, 2
local UNIT_TARGETABLE = 0x0c8d
local CONSIDERED_COORDS, CANDIDATE_COORDS, COORDS_BYTES = 0x0c74, 0x0cc4, 4
local MOVEMENT_SCENARIO, ACTING_UNIT_ID, ACTING_COORDS = 0x0e2d, 0x0e2e, 0x0e30
local DECISIONS, DECISION_BYTES, TARGETING_FLAGS_2 = 0x182c, 16, 0x07
local RESUME_PENDING = 0x19b7
local USEFUL = 1
local V0, S2 = 2, 18
local YIELD_SECOND_INSTRUCTION, SAVES_S2_AFTER = 4, 4

local anchor = e.symbol('battle', 'thread_yield')
local selfSearch = e.symbol('battle', 'battle_ai_evaluate_self_target_origins')
local reflectedSearch = e.symbol('battle', 'battle_ai_evaluate_reflected_target_origins')
local targetSearch = e.symbol('battle', 'battle_ai_evaluate_target_tiles')
local canUse = e.symbol('battle', 'battle_ai_can_use_ability_at_coords')
local statusThroughDelay = e.symbol('battle', 'battle_ai_is_status_active_through_delay')
local movesFirst = e.symbol('battle', 'battle_ai_moves_before_charge_lands')
local classify = e.symbol('battle', 'battle_ai_classify_ability_effect')
local moveTemp = e.symbol('battle', 'battle_ai_move_temp_unit_to_coords')
local rank = e.symbol('battle', 'battle_ai_insert_ranked_action')
local report = e.symbol('main', 'main_system_handle_error')

local values = { calls_returned = 0 }
local records = { self_resumed = { searches = {}, tiles = {} }, self_whole = { searches = {}, tiles = {} },
                  reflected = { searches = {}, tiles = {} } }
local returns = {}
local saved = {}
local ai, units, stage, reflector, first, second
local sequence, active, suspend, hooked = nil, false, false, false

local function unit(slot) return units + STATS_BYTES * slot end

local function setCoords(at, x, y)
    e.write8(at, x)
    e.write8(at + 1, 0)
    e.write8(at + 2, y)
    e.write8(at + 3, 0)
end

local function tile(at) return string.format('%d,%d', e.read8(at), e.read8(at + 2)) end

-- Return from the function just entered with value in v0.
local function returnWith(value)
    e.setGPR(V0, value % 0x100000000)
    local regs = PCSX.getRegisters()
    regs.pc = regs.GPR.n.ra
end

local function hookCallees()
    if hooked then return end
    hooked = true
    e.breakAt(targetSearch.address, 'target search', function()
        if not active then return true end
        e.write8(ai + RESUME_PENDING, 0)
        local pass = e.read8(ai + ACT_BEFORE_MOVE) ~= 0 and 'b' or 'a'
        if sequence == 'reflected' then pass = tile(ai + COORDS) .. pass end
        table.insert(records[sequence].searches, pass)
        returnWith(suspend and -1 or 0)
        suspend = false
        return true
    end)
    e.breakAt(canUse.address, 'can use', function()
        if not active then return true end
        table.insert(records[sequence].tiles, tile(tonumber(PCSX.getRegisters().GPR.n.a0)))
        returnWith(1)
        return true
    end)
    e.breakAt(statusThroughDelay.address, 'Reflect', function()
        if not active then return true end
        returnWith(tonumber(PCSX.getRegisters().GPR.n.a1) == unit(reflector) and 1 or 0)
        return true
    end)
    for _, callee in ipairs({ { movesFirst, 0 }, { classify, USEFUL }, { moveTemp, 0 }, { rank, 0 } }) do
        e.breakAt(callee[1].address, callee[1].name or 'callee', function()
            if not active then return true end
            returnWith(callee[2])
            return true
        end)
    end
    -- The caller (battle_ai_evaluate_ability_outcome) holds the AI workspace in s2.
    e.breakAt(reflectedSearch.address + SAVES_S2_AFTER, 'caller s2', function()
        if active then e.setGPR(S2, ai) end
        return true
    end)
end

local function save()
    ai = e.address('battle', 'g_battle_ai_data_base')
    units = e.address('battle', 'g_battle_unit_stats')
    stage = e.address('battle', 'g_battle_ai_self_target_origin_stage')
    saved.workspace = {}
    for i = 0, WORKSPACE_BYTES - 1 do saved.workspace[i] = e.read8(ai + i) end
    saved.stage = e.read32(stage)
    local acting = e.read8(ai + ACTING_UNIT_ID)
    local slots = {}
    for slot = 0, UNIT_SLOTS - 1 do
        if slot ~= acting and #slots < 3 then slots[#slots + 1] = slot end
    end
    reflector, first, second = slots[1], slots[2], slots[3]
    saved.units = {}
    for _, slot in ipairs(slots) do
        saved.units[slot] = { e.read8(unit(slot) + STATS_X), e.read8(unit(slot) + STATS_Y) }
    end
end

local function restore()
    for i = 0, WORKSPACE_BYTES - 1 do e.write8(ai + i, saved.workspace[i]) end
    e.write32(stage, saved.stage)
    for slot, place in pairs(saved.units) do
        e.write8(unit(slot) + STATS_X, place[1])
        e.write8(unit(slot) + STATS_Y, place[2])
    end
end

local function prepareSelf()
    e.write8(ai + FLAGS_BYTE_1, bit.band(e.read8(ai + FLAGS_BYTE_1), bit.bnot(FOLLOW_TARGET_BIT)))
    setCoords(ai + ACTING_COORDS, 1, 1)
    setCoords(ai + CANDIDATE_COORDS + COORDS_BYTES * e.read8(ai + MOVEMENT_SCENARIO), 5, 5)
end

local function prepareReflected()
    setCoords(ai + ACTING_COORDS, 1, 1)
    for slot, place in pairs({ [reflector] = { 3, 3 }, [first] = { 2, 3 }, [second] = { 3, 2 } }) do
        e.write8(unit(slot) + STATS_X, place[1])
        e.write8(unit(slot) + STATS_Y, place[2])
    end
    for slot = 0, UNIT_SLOTS - 1 do
        e.write8(ai + UNIT_TARGETABLE + slot, (slot == first or slot == second) and 1 or 0)
    end
    e.write8(ai + DECISIONS + DECISION_BYTES * reflector + TARGETING_FLAGS_2, 0)
    local rows = ai + REACHABLE + SCENARIO_BYTES * e.read8(ai + MOVEMENT_SCENARIO)
    for i = 0, SCENARIO_BYTES - 1 do e.write8(rows + i, 0) end
    e.write16(rows + ROW_BYTES * 3, bit.rshift(0x8000, 4))
    e.write16(rows + ROW_BYTES * 4, bit.rshift(0x8000, 3))
    e.write8(ai + BEST_TARGET_ID, 2)
    e.write8(ai + BEST_TARGET_X, 2)
end

local function step(label, target, name, resume, suspended, prepare)
    return { target = target, label = label,
        args = function()
            if not saved.workspace then
                save()
                hookCallees()
            end
            if prepare then prepare() end
            e.write8(ai + RESUME_PENDING, resume and 1 or 0)
            sequence, suspend, active = name, suspended, true
            return {}
        end,
        done = function(result)
            active = false
            if result.returned then
                values.calls_returned = values.calls_returned + 1
                local v0 = result.v0
                if v0 >= 0x80000000 then v0 = v0 - 0x100000000 end
                table.insert(returns, tostring(v0))
            else
                table.insert(returns, 'none')
            end
            if name == 'reflected' and resume then
                values.reflected_last_origin = tile(ai + COORDS)
                restore()
            end
        end }
end

local calls = {
    step('around the unit, suspended', selfSearch, 'self_resumed', false, true, prepareSelf),
    step('around the unit, resumed', selfSearch, 'self_resumed', true, false),
    step('around the unit, whole', selfSearch, 'self_whole', false, false, prepareSelf),
    step('through Reflect, suspended', reflectedSearch, 'reflected', false, true, prepareReflected),
    step('through Reflect, resumed', reflectedSearch, 'reflected', true, false),
}

return {
    why = 'the AI\'s searches around the unit and through Reflect test the unit\'s tile when moving first, lose the '
        .. 'moved pass and the reflector on resume (direct calls with a forced suspension: Gariland has no Reflect '
        .. 'and suspensions follow the frame budget)',
    save = { file = 'smoke-new-game.mcd', sha256 = '02203c586fc996bd3c3dfa1fb07bb6e0025cb590b69b59dae955fc3b098c47e3' },
    setup = e.join(e.continueFromTitle(), {
        { module = 'event-attack', name = 'attack_deploy_run_screen', label = 'overlay menu ATTACK.OUT deployment',
          before = { BUTTON.CIRCLE }, budget = 9000, shot = 90 },
        { module = 'battle', name = 'battle_ai_load_ability_entry', label = 'the AI considers an ability',
          before = { BUTTON.CIRCLE }, budget = 40000, shot = 1, start = function()
              e.startDeployment()
              e.playTurns(AUTO_BATTLE)
          end },
    }),
    trigger = {
        e.call({ label = 'origin searches', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' },
                       { address = report.address, limit = 0, label = 'error report' } } }),
    },
    check = function()
        for name, record in pairs(records) do
            values[name .. '_searches'] = table.concat(record.searches, ' ')
            values[name .. '_tiles'] = table.concat(record.tiles, ' ')
        end
        values.returns = table.concat(returns, ' ')
        return values
    end,
    expect = {
        original = { self_resumed_searches = 'b b', self_resumed_tiles = '1,1', self_whole_searches = 'b a',
                     self_whole_tiles = '1,1 1,1', reflected_searches = '4,3b 4,3b 4,3a', reflected_tiles = '1,1 1,1',
                     reflected_last_origin = '1,2', returns = '-1 0 0 -1 0', calls_returned = 5 },
        dc = { self_resumed_searches = 'b b a', self_resumed_tiles = '1,1 5,5', self_whole_searches = 'b a',
               self_whole_tiles = '1,1 5,5', reflected_searches = '4,3b 4,3b 4,3a 3,4b 3,4a',
               reflected_tiles = '1,1 4,3 1,1 3,4', reflected_last_origin = '3,4', returns = '-1 0 0 -1 0',
               calls_returned = 5 },
    },
}
