---
type: guide
mechanics: [World Map and Travel]
---

# Changing travel and random encounters

## Goal

Change how the party moves on the world map: how often random battles
happen and against whom, which path the marker takes, how fast it walks, how
many days a trip costs, and what a town offers.

## Where it lives

All of it is WLDCORE code and WLDCORE tables; the
[[World Map and Travel]] page describes the retail rules.

| What | Where |
|---|---|
| Random battle chance, tiers and enemies | The encounter table `g_wldcore_random_battle_data` (per field, three roads of 24 bytes: route, deployment squad, four tiers of threshold / chance / ENTD mask, eight ENTDs, battle map, progress variable), read by [[wldcore_map_roll_random_encounter]] |
| Which fields can have battles | [[wldcore_map_roll_random_encounter]] rolls only for locations 24 to 42 (`WORLD_LOCATION_FIELD_FIRST`) |
| Which location gets the roll | [[wldcore_map_step_route_travel]] rolls at every location the marker reaches, for the route it came by |
| Path choice | [[wldcore_map_find_best_route_path]] (fewest legs, then the lowest total of the route costs; at most 16 legs) and the cost byte of each route header ([[wldcore_route_record_t]]) |
| Walking speed | [[wldcore_map_begin_route_segment]]: a segment lasts twice its point's distance in frames |
| Days per trip | [[wldcore_map_step_route_travel]] calls [[wldcore_calendar_advance_day]] once per location reached; events add days through `EVENT_SCRIPT_VAR_DATE_ADVANCE` ([[wldcore_calendar_advance_days]]) |
| What a day does | [[wldcore_calendar_advance_day]]: propositions, birthdays, monster eggs |
| A town's services | [[wldcore_map_build_location_menu_entries]] (Bar, Shop, Soldier Office, the Fur Shop towns) and [[wldcore_menu_step_location_menu_level]] (what each entry opens) |
| The Deep Dungeon floors | `g_wldcore_deep_dungeon_battles` (map, squad, eight ENTDs per floor) and the location's `DEEP_DUNGEON_ENTRY` condition |
| Map controls | [[wldcore_menu_step_map_cursor_level]] |

## What changes with it

- **The scripts.** Where a story event waits, which locations and routes are
  visible and when the Fur Shop opens are script variables that the event and
  location scripts set. A route the scripts never make visible is never
  traveled.
- **Battle data.** A random battle names an ENTD, a map and a deployment
  squad (`0x200` + the road's squad id); the battle itself is BATTLE's. A new
  enemy needs its ENTD.
- **The encounter roll is off by one.** A chance byte `c` gives `c + 1` in
  100; a chance of 0 is the only one that never triggers.
- **Propositions.** A day counts toward the propositions out on a job only
  when it passes one at a time; an event's date advance does not (see
  `QUIRKS.md`).
- **Text.** A new town service needs an entry in the world menu text (the
  `TEXT_ID_WORLD_MENU_BASE` section that the Bar and Shop rows use), its help
  text, and a case in [[wldcore_menu_step_location_menu_level]].

## What can be done today

Nothing that changes a byte of the original build: `make check` compares every
function with its original hash. To try an idea, change the game's memory in an
emulator: `make run` launches PCSX-Redux, and `make map` writes the symbol maps
that name these tables in its debugger. For example, the encounter table is the
pointer at `g_wldcore_random_battle_data` while WLDCORE is loaded; the tier
chances are the bytes at +3, +6, +9 and +12 of each 24-byte road. In the
Developer's Cut, a change goes behind an option: code under `#ifdef` and a
table's bytes in place through `target/edits.yaml` (`AGENTS.md`, "Changes under
an option").
