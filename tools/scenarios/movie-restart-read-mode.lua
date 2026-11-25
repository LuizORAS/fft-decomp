-- Retail bug of open_movie_start_cd_stream_read (QUIRKS.md): the opening
-- movie's first read asks the drive for mode 0x1e0, and the read that restarts
-- a stream that fell behind for 0x1a0, without CdlModeRT (0x40), so the XA
-- audio would stop. No restart was seen in 14000 frames of the movie, so a
-- call makes one at one stop of the title menu. CdControl and CdRead2 return
-- 1 at once while it runs, so the drive is not touched. The scenario reads the
-- mode the read asks for. On the original it is 0x1a0; on the Developer's Cut
-- 0x1e0.
local e = emulator

-- The register each stub answers in.
local V0 = 2

local anchor = e.symbol('opening', 'open_title_update_menu')
local restart = e.symbol('opening', 'open_movie_start_cd_stream_read')
local control = e.symbol('main', 'CdControl')
local read = e.symbol('main', 'CdRead2')

local values = { calls_returned = 0 }
local calling = false

-- A function that answers 1 at once while the call runs.
local function answer(symbol, label, seen)
    e.breakAt(symbol.address, label, function()
        if calling then
            local regs = PCSX.getRegisters()
            if seen then seen(tonumber(regs.GPR.n.a0)) end
            e.setGPR(V0, 1)
            regs.pc = regs.GPR.n.ra
        end
        return true
    end)
end

local calls = {
    { target = restart, label = 'restart the stream',
      args = function()
          calling = true
          answer(control, 'CdControl')
          answer(read, 'CdRead2', function(mode) values.read_mode = mode end)
          return { e.address('main', 'g_main_party_units') }
      end,
      done = function(result)
          calling = false
          if result.returned then values.calls_returned = values.calls_returned + 1 end
      end },
}

return {
    why = 'a restarted opening movie reads without CdlModeRT (direct call: no restart was seen)',
    save = { file = 'world-map.mcd', sha256 = 'adee3f05a445925002945e98791b6e8a461e5b3bc46ff28842c879f44243b945' },
    setup = { e.title() },
    trigger = {
        e.call({ label = 'movie restart', anchor = anchor, calls = calls,
            guards = { { address = read.address, limit = 8, label = 'read' } } }),
    },
    check = function() return values end,
    expect = {
        original = { read_mode = 0x1a0, calls_returned = 1 },
        dc = { read_mode = 0x1e0, calls_returned = 1 },
    },
}
