-- Retail bug of battle_unit_face_towards_action_target and
-- battle_unit_restore_facing_after_attack (QUIRKS.md): a rider that turns
-- toward an action turns its mount's sprite too, but the second
-- battle_unit_set_tile_position names the rider again, so the mount's battle
-- stats keep the old facing. Turning back evens them out, but a rider killed
-- in that action is not turned back, so its mount is left with a sprite and
-- battle stats, which the evasion reads, that face different ways. Gariland
-- has no chocobo, so calls make it at one stop of BATTLE's thread_yield, from
-- the AI stop of an Auto-battle turn on Gariland's real route (the new-game
-- save): the runtime units of unit slots 0 and 1 become a rider and its
-- mount, both facing 1, and the rider turns toward a third unit put east of
-- it. In one sequence the rider lives and both are turned back; in the other
-- it is marked dead, dismounts as a killed rider does, and both go through
-- the same turn back. The scenario reads the mount's sprite facing and stats
-- facing after each; the three runtime units and the two units are put back
-- after each sequence. On the original the mount's stats stay at 1 while its
-- sprite turns to 3 and stays there when the rider is killed; on the
-- Developer's Cut they turn together, to 3, and back to 1 when the rider
-- lives.
local e = emulator
local BUTTON = e.BUTTON

-- The Auto-battle menu rows; a unit record's bytes and the byte whose low
-- nibble is its facing; the runtime units, their count and bytes, the runtime
-- id, the world x, y and z, the sprite facing (0x400 per quarter turn) and the
-- one saved before an action (0xffff for none), the map tile, the mount state
-- (rider, mount) and partner id, the stats link, the status flags and the dead
-- flag, the pending result a killed rider has and the tile it dismounts to;
-- the facing both start with, a map tile's world units and the offset of
-- thread_yield's second instruction.
local AUTO_BATTLE = { { row = 4, name = 'Auto-battle' }, { menu = 1, row = 3, name = 'Save fading life' } }
local STATS_BYTES, STATS_FACING = 448, 0x049
local RUNTIME_COUNT, RUNTIME_BYTES, RUNTIME_ID = 16, 1088, 0x004
local WORLD_X, WORLD_Y, WORLD_Z = 0x040, 0x042, 0x044
local FACING, SAVED_FACING, NO_FACING = 0x070, 0x072, 0xffff
local MAP_X, MAP_Y, MAP_Z = 0x07c, 0x07d, 0x07e
local MOUNT_STATE, MOUNT_PARTNER, RIDER, MOUNT = 0x130, 0x131, 1, 2
local STATS_LINK, STATUS_FLAGS, DEAD = 0x134, 0x140, 0x04
local PENDING_RESULT, DISMOUNT_X, DISMOUNT_LAYER, DISMOUNT_Y = 0x180, 0x184, 0x186, 0x188
local START_FACING, TILE = 1, 28
local YIELD_SECOND_INSTRUCTION = 4

local anchor = e.symbol('battle', 'thread_yield')
local face = e.symbol('battle', 'battle_unit_face_towards_action_target')
local restore = e.symbol('battle', 'battle_unit_restore_facing_after_attack')
local dismount = e.symbol('battle', 'battle_unit_set_map_coords_after_death_dismount')
local report = e.symbol('main', 'main_system_handle_error')

local values = { calls_returned = 0 }
local saved
local riderStats, mountStats, rider, mount, target

local function readBytes(at, count)
    local bytes = {}
    for i = 0, count - 1 do bytes[i] = e.read8(at + i) end
    return bytes
end

local function writeBytes(at, bytes, count)
    for i = 0, count - 1 do e.write8(at + i, bytes[i]) end
end

-- The runtime unit linked to a unit, and another linked to neither.
local function findRuntime(stats)
    local base = e.address('battle', 'g_battle_units')
    for slot = 0, RUNTIME_COUNT - 1 do
        local at = base + RUNTIME_BYTES * slot
        if e.read32(at + STATS_LINK) == stats then return at end
    end
    error('no runtime unit links the unit')
end

local function findOther()
    local base = e.address('battle', 'g_battle_units')
    for slot = 0, RUNTIME_COUNT - 1 do
        local at = base + RUNTIME_BYTES * slot
        if at ~= rider and at ~= mount and e.read32(at + STATS_LINK) ~= 0 then return at end
    end
    error('no third runtime unit')
end

-- Units 0 and 1 become a rider and its mount, facing the same way, with the
-- third unit one tile east of them.
local function pair()
    riderStats = e.address('battle', 'g_battle_unit_stats')
    mountStats = riderStats + STATS_BYTES
    rider, mount = findRuntime(riderStats), findRuntime(mountStats)
    target = findOther()
    saved = { riderStats = readBytes(riderStats, STATS_BYTES), mountStats = readBytes(mountStats, STATS_BYTES),
              rider = readBytes(rider, RUNTIME_BYTES), mount = readBytes(mount, RUNTIME_BYTES),
              target = readBytes(target, RUNTIME_BYTES) }
    for _, unit in ipairs({ rider, mount }) do
        e.write16(unit + FACING, START_FACING * 0x400)
        e.write16(unit + SAVED_FACING, NO_FACING)
        e.write32(unit + STATUS_FLAGS, 0)
    end
    for _, stats in ipairs({ riderStats, mountStats }) do
        local byte = e.read8(stats + STATS_FACING)
        e.write8(stats + STATS_FACING, byte - byte % 16 + START_FACING)
    end
    e.write8(rider + MOUNT_STATE, RIDER)
    e.write8(rider + MOUNT_PARTNER, e.read8(mount + RUNTIME_ID))
    e.write8(mount + MOUNT_STATE, MOUNT)
    e.write8(mount + MOUNT_PARTNER, e.read8(rider + RUNTIME_ID))
    e.write16(target + WORLD_X, e.read16(rider + WORLD_X) + TILE)
    e.write16(target + WORLD_Y, e.read16(rider + WORLD_Y))
    e.write16(target + WORLD_Z, e.read16(rider + WORLD_Z))
end

local function putBack()
    writeBytes(riderStats, saved.riderStats, STATS_BYTES)
    writeBytes(mountStats, saved.mountStats, STATS_BYTES)
    writeBytes(rider, saved.rider, RUNTIME_BYTES)
    writeBytes(mount, saved.mount, RUNTIME_BYTES)
    writeBytes(target, saved.target, RUNTIME_BYTES)
end

local function returned(result)
    if result.returned then values.calls_returned = values.calls_returned + 1 end
end

local function readMount(label)
    values[label .. '_mount_sprite'] = math.floor(e.read16(mount + FACING) / 0x400) % 4
    values[label .. '_mount_stats'] = e.read8(mountStats + STATS_FACING) % 16
end

-- One step of a sequence: the call, then what to read after it.
local function step(label, target, args, after)
    return { target = target, label = label, args = args,
             done = function(result)
                 returned(result)
                 if after then after() end
             end }
end

local calls = {
    step('the rider turns', face, function() pair() return { rider, target } end, function() readMount('turned') end),
    step('the rider turns back', restore, function() return { rider } end),
    step('the mount turns back', restore, function() return { mount } end, function()
        readMount('alive')
        putBack()
    end),
    step('the rider turns, then dies', face, function() pair() return { rider, target } end),
    step('the killed rider dismounts', dismount, function()
        e.write32(rider + STATUS_FLAGS, DEAD)
        e.write32(rider + PENDING_RESULT, 0xffffffff)
        e.write16(rider + DISMOUNT_X, e.read8(rider + MAP_X))
        e.write16(rider + DISMOUNT_LAYER, e.read8(rider + MAP_Z))
        e.write16(rider + DISMOUNT_Y, e.read8(rider + MAP_Y))
        return { rider }
    end, function() values.killed_mount_state = e.read8(mount + MOUNT_STATE) end),
    step('the killed rider is not turned back', restore, function()
        e.write32(rider + STATUS_FLAGS, DEAD)
        return { rider }
    end),
    step('its mount has nothing to turn back', restore, function() return { mount } end, function()
        readMount('killed')
        putBack()
    end),
}

return {
    why = 'a mount whose rider dies in the action it turned in keeps its old stats facing (direct calls: Gariland has no chocobo)',
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
        e.call({ label = 'mounted pair', anchor = anchor, calls = calls,
            guards = { { address = anchor.address + YIELD_SECOND_INSTRUCTION, limit = 0, label = 'wait' },
                       { address = report.address, limit = 0, label = 'error report' } } }),
    },
    check = function() return values end,
    expect = {
        original = { turned_mount_sprite = 3, turned_mount_stats = 1, alive_mount_sprite = 1, alive_mount_stats = 1,
            killed_mount_state = 0, killed_mount_sprite = 3, killed_mount_stats = 1, calls_returned = 7 },
        dc = { turned_mount_sprite = 3, turned_mount_stats = 3, alive_mount_sprite = 1, alive_mount_stats = 1,
            killed_mount_state = 0, killed_mount_sprite = 3, killed_mount_stats = 3, calls_returned = 7 },
    },
}
