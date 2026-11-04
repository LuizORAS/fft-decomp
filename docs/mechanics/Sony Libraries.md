---
type: mechanic
tier: 4
scope:
  - src/lib/psyq/
---

# Sony Libraries

## What it does in the game

The game is built on Sony's Psy-Q SDK: the libraries it linked do the
hardware work, and this repository rebuilds them as C under
[`src/lib/psyq/<library>/`](../../src/psyq/), with their SDK names. Game code
reads the disc, plays sound, draws, reads the pads and saves through them.
Their summaries come from the code; what the BIOS does with a call comes
from [psx-spx](https://psx-spx.consoledev.net/), and each such summary says
so.

| Library | Linked in | Routines | What the game uses it for |
|---|---|---|---|
| LIBCD | MAIN | 60 | Every disc read ([[CdRead]] through [[main_file_poll_load]]), the drive's commands and interrupts, CD audio volumes, and the streaming ring of OPEN's movies and WLDCORE's image stream |
| LIBSPU | MAIN | 57 | The voices, SPU RAM, transfers and reverb the [[Sound Driver]] drives |
| LIBGPU | MAIN | 112 | The GPU's operation queue, VRAM transfers, ordering tables, display and draw environments, primitive headers and the debug font |
| LIBGTE | MAIN | 64 | GTE set-up, matrices, transforms and projection, and the fixed-point sine, cosine, arctangent and square roots |
| LIBETC | MAIN | 31 | The interrupt dispatcher, VSync and the pads |
| LIBAPI | MAIN | 59 | Two-instruction veneers into the BIOS: events, critical sections, exception hooks, memory card files, string and memory routines and [[rand]]; plus the root counters |
| LIBCARD | MAIN | 10 | The memory card start-up and its two BIOS patches |
| LIBC | MAIN | 4 | [[sprintf]] (the menu scripts' numbers), [[strlen]], [[memchr]] and [[memmove]] |
| C start-up | MAIN | 3 | [[__SN_ENTRY_POINT]], which clears the BSS and calls [[main]] |
| LIBGS | WORLD | 24 | WLDCORE's drawing: double buffers, ordering tables and sorted sprites, polygons, lines and boxes |
| LIBPRESS | OPEN | 18 | The MDEC decoder behind OPEN's movies |

The sound driver's four CPU heap routines, [[suzuki_sound_init_heap]],
[[suzuki_sound_alloc]], [[suzuki_sound_free]] and
[[suzuki_sound_get_largest_free_block]], live in `src/lib/suzuki/` with the
rest of the driver; the [[Sound Driver]] page covers them. `target/main.yaml`,
`world.yaml` and `opening.yaml` list each module's library ranges under
`libraries:`.

## Data and structures

| What | Where |
|---|---|
| Declarations | One header per library in `include/psx/` (`libcd.h`, `libspu.h`, `libgpu.h`, `libgte.h`, `libetc.h`, `libapi.h`, `libcard.h`, `libc.h`, `libgs.h`, `libpress.h`), the BIOS table and service numbers in `bios.h`, and the hardware helpers in focused headers (`abi_inline.h`, `gte_inline.h`, `gte_regs.h`, `cpu_abi_inline.h` and others) |
| LIBETC's dispatch table | `g_psyq_etc_dispatch` (0x80031764): [[setIntr]], [[startIntr]], [[stopIntr]], [[restartIntr]], and the VBlank and DMA setters [[startIntr]] installs ([[setIntrVSync]], [[setIntrDMA]]); [[ResetCallback]], [[InterruptCallback]], [[DMACallback]] and the VSync callbacks go through it |
| LIBETC's interrupt state | `g_psyq_etc_interrupt_state`: the eleven interrupt callbacks, the setjmp state [[trapIntr]] runs from and its own 4 KB stack |
| LIBGPU's dispatch table | `g_psyq_gpu_dispatch_table` (0x8003284c): [[_addque]], [[_addque2]], [[_clr]], [[_ctl]], [[_cwb]], [[_cwc]], [[_drs]], [[_dws]], [[_exeque]], [[_getctl]], [[_otc]], [[_param]], [[_reset]], [[_status]] and [[_sync]]; every LIBGPU call that touches the GPU goes through it |
| LIBGPU's queue | 64 operations, run from the GPU DMA callback ([[_exeque]]) |
| LIBCD's state | `g_psyq_cd_irq_state` (the sync and ready states the waits poll) and the read in progress ([[CdRead]], [[cb_read]]) |
| LIBSPU's state | `_spu_RXX` (the SPU registers), the SPU RAM heap records ([[SpuInitMalloc]]) and the reverb settings |
| LIBGTE's matrix stack | 20 rotation and translation entries ([[PushMatrix]], [[PopMatrix]]) |
| The BIOS patches | Templates copied into the BIOS: [[_patch_card]] and [[_patch_card2]] (memory card), [[_patch_gte]] (exception handler) |

## Main flow

1. **Start-up.** [[__SN_ENTRY_POINT]] clears the BSS, sets the stack, heap
   and global pointer and calls [[main]].
2. **Boot.** [[main_boot_run_startup]] starts LIBETC's interrupts
   ([[ResetCallback]]), installs the VSync and DrawSync callbacks, resets the
   GPU ([[ResetGraph]]), the pads ([[PadInit]]) and the SPU ([[SpuInit]]),
   and starts LIBCD ([[main_file_init_cd]]: [[SetMem]], [[CdInit]]). The
   memory card follows ([[main_card_init_events]]: the card events,
   [[InitCARD]], [[StartCARD]], [[_bu_init]]) and the sound driver
   ([[suzuki_sound_init]]: SPU RAM heap, root counter 2 at about 240
   ticks a second).
3. **Each frame.** The game's loops draw their ordering tables
   ([[DrawOTag]]), wait for drawing and the VBlank ([[DrawSync]], [[VSync]])
   and read the pads ([[PadRead]]). [[main_system_handle_vsync_callback]]
   runs on each VBlank through LIBETC.
4. **Disc reads.** [[main_file_poll_load]] seeks and reads with [[CdControl]]
   and [[CdRead]], polled by [[CdReadSync]]; movies stream through
   [[CdRead2]] and the `St*` ring.
5. **Saves.** The card screens open, read and write `bu00:` and `bu10:`
   files through the BIOS veneers ([[open]], [[read]], [[write]]) and poll
   the card events ([[TestEvent]]).

## Where to change

| Change | Where |
|---|---|
| Disc read retries and time limits | [[CdReadSync]] and [[cd_read_retry]] (a read restarts after 60 quiet VBlanks and gives up after 1200), [[CD_sync]] (960 VBlanks) |
| The VBlank wait and its time limit | [[VSync]], [[v_wait]] |
| The sound driver's tick | [[suzuki_sound_init]]'s [[SetRCnt]] call |
| The debug text | [[FntOpen]], [[FntPrint]], [[FntFlush]]; [[SetDumpFnt]] routes LIBGPU's diagnostics there |
| PAL or NTSC display | The video mode stays 0 (NTSC): nothing calls [[SetVideoMode]] |
| A port | These libraries are the hardware boundary: a port replaces them, the BIOS veneers (`PSYQ_BIOS_TAIL_CALL` in `abi_inline.h`) and the GTE helpers (`gte_inline.h`) with its own equivalents |

Like the game, every routine here matches its original bytes in the original
build; a change to one goes behind an option, as in the game's code
(`AGENTS.md`, "Changes under an option").

## Quirks and debts

- Retail bugs kept: [[sprintf]] and [[FntPrint]] can overrun their 512-byte
  buffers; [[DrawOTagEnv]]'s diagnostic prints a literal `&08x`;
  [[StCdInterrupt]] copies four unset bytes without `CdlModeSize1`;
  [[DecDCTvlc]]'s pause path writes over the birthday table, though no frame
  pauses here.
- Surprises: [[StartRCnt]] and [[StopRCnt]] index their mask table before
  checking the counter; [[SpuGetVoiceEnvelopeAttr]] stores its key state as
  a 16-bit value; [[_patch_card2]] undoes itself on a second call; [[InitCARD]]
  and [[StartCARD]] pass 0 to ChangeClearPAD; [[SetDrawLoad]] leaves the
  pixels to its caller; [[memmove]] returns different pointers on its two
  paths; [[main_gfx_load_efc_fnt]] patches [[FntLoad]]'s CLUT row.
- The game relies on library return values in a few places, which a port
  must reproduce: the BIOS [[bzero]] returns its destination
  ([[battle_map_load_data]]).
- Linking takes whole library objects, so about 70 routines here are
  referenced by no code (the LIBGTE squares and outer products, the LIBGPU
  dumps and TMD readers, [[GsInitGraph]], [[DecDCTvlcSize]], [[InitHeap]]
  and others); their summaries say so.

## Functions in scope

![[Sony Libraries scope]]
