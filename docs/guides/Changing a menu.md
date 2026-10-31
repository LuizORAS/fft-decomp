---
type: guide
mechanics: [Menus]
---

# Changing a menu

## Goal

Change a battle menu (its rows, text, what a row opens or what a choice
does), a prompt or message the battle raises, or the look of a WORLD menu.

## Where it lives

The [[Menus]] page describes the engine.

| What | Where |
|---|---|
| A battle menu: rows, window, text id, thread | Its [[world_menu_entry_t]] in `g_battle_menu_idle_action_entries`, indexed by [[battle_menu_id_e]] |
| The row texts | The text entry the menu entry names (`text_id`) |
| What a row opens | The entry's row actions: a menu id, plus 0x1000 for no confirm sound, or a negative count of levels to close |
| What a choice does | `g_battle_menu_command_maps` turns it into a command; the battle state reads it |
| A prompt or message | Its [[battle_menu_system_function_t]] in `g_battle_system_function_table` |
| A WORLD menu's look | Its display script ([[menu_script_opcode_e]]) |

## What changes with it

- **The twin.** WORLD has a copy of each battle menu routine and table; a change
  that must hold in both modules is made twice.
- **The remembered rows.** The first twelve menu ids keep each unit's last row
  (`g_battle_menu_unit_selection_records`); a menu with more rows than before
  may restore a row that no longer exists.
- **Navigation messages.** A system function has an alternate record used when
  navigation messages are off.
- **The tutorial.** Tutorial events disable rows through
  `EVENT_SCRIPT_VAR_DISABLED_MENU_ACTIONS`, by row number.

## What can be done today

Nothing that changes a byte: the tables are data in BATTLE.BIN and WORLD.BIN,
and `make check` compares every function with its original hash. To try an
idea, edit the tables in an emulator's memory: `make run` launches PCSX-Redux,
and `make map` writes the symbol maps that name them. Code changes wait for
build 3.
