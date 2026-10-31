---
type: guide
mechanics: [Event Scripts]
---

# Changing an event

## Goal

Change a story scene (what the units do, what they say, the camera, the
music), which event follows which, or what a world map location triggers.

## Where it lives

The scenes are data on the disc; the [[Event Scripts]] page describes how the
interpreters run them.

| What | Where |
|---|---|
| A scene's instructions and its text | Its 8 KB block of `EVENT/TEST.EVT`: event id × 4 sectors from `DISC_SECTOR_EVENT_TEST_EVT` ([[script_load_event]]) |
| What each instruction does and its operands | [[event_opcode_e]] in `include/fft/script.h`; the handlers are the arms of [[script_execute_event]] and WORLD's copy |
| Which event a battle plays next | The scenario's condition chains (loaded by ATTACK) and each event's finish operation in `g_main_scenario_event_finish_operations` |
| Story flags | The script variables ([[event_script_variable_id_e]]) |
| What a world map location starts | Its location scripts ([[wldcore_script_find_location_request]]) |

## What changes with it

- **Text.** A scene's text sits at the end of its own block; the block's
  first word points to it.
- **Variables.** Other events, the location scripts, the Brave Story and the
  shops read the same variables; a flag an event sets may gate something far
  away.
- **Units.** The units an event moves come from the battle's ENTD and the
  party; an event that names a unit the battle lacks finds no sprite, and
  most instructions then skip it.
- **The chain.** The event that follows is chosen by the scenario's
  conditions and the finish operation, not by the scene itself.

## What can be done today

Nothing that changes a byte: `make check` compares every function with its
original hash, and `TEST.EVT` is disc data. To try an idea, change the
game's memory in an emulator: `make run` launches PCSX-Redux, and `make map`
writes the symbol maps that name `g_main_script_event_block` and the script
variables in its debugger. While an event runs, its block is loaded there.
Code changes wait for build 3.
