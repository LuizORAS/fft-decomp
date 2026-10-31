---
type: mechanic
tier: 4
scope:
  - battle_effect
---

# Ability Effects

## What it does in the game

Every ability, item or weapon hit that shows something on the battle map
goes through BATTLE's effect engine. It plays two kinds of effect:

- **Effect files.** Fire, Cure, a summon or a sword skill each have an
  `EFFECT/E*.BIN` file (110 of them, loaded one at a time at `0x801c2500`).
  A file is code and data: a script, timelines, particle emitters, sprite
  frames, a palette, its own sounds, and native callbacks (the file's code)
  that draw meshes and run special steps.
- **Secondary effects.** About twenty small effects live in BATTLE.BIN
  itself, so they need no load: weapon hits, arrows and thrown items, the
  charge of a spell or a summon, level up, the dust or splash of a landing,
  a sprung trap, the zodiac poof of a new Crystal or Treasure, the death
  smoke of a monster, Reflect's shield, teleport, equip break and the red
  orbs of Charge.

The effect also drives the camera while it plays (see [[Camera]]) and makes
the targets react (flinch, block, fall) on its keyframes.

## Data and structures

| Record | What it holds |
|---|---|
| `g_battle_effect_ability_files` | The effect file of each ability, with flags ([[battle_effect_is_item_ability]]); a negative entry means no file. Ids from 0x200 on continue the table after the action abilities |
| `g_battle_effect_disc_entries` ([[main_file_extent_t]]) | Sector and size of each effect file ([[battle_effect_get_file_extent]]) |
| [[effect_file_header_t]], `g_battle_effect_file_headers` | The section offsets of each file: frames, animation sequences, script, particle system, animation curves, timing curves, flags, timeline, sound, palette. The `battle_effect_set_*_pointer` routines bind them to globals when the effect starts |
| `g_battle_effect_native_callbacks` | 198 addresses in the effect overlay area that script opcodes 06 and 07 bind or call ([[battle_effect_get_native_callback]]); each is a relocation to an effect file's routine, except the two unused ids |
| [[battle_effect_record_t]], `g_battle_effect_state_records` | The script records: a script position, counters, child records, a parent and a child timeline, callback slots and the particles it owns |
| [[effect_timeline_t]] | The parent timeline: particle channels, color tracks (screen, caster, targets), sound tracks and camera keyframes for two phases; the child timeline runs per target |
| [[effect_geometry_table_t]], [[effect_geometry_entry_t]] | The particle-system section: gravity, inertia threshold and one record per emitter or mesh, which each renderer reads through its own view |
| [[effect_animation_curve_t]], `g_battle_effect_animation_curves` | Per-frame factors out of 256 that move an emitter's values from start to end, tint particles and set homing strength |
| [[effect_flags_section_t]] | The effect's flags (EFFECT_FLAG_QUIET_MUSIC lowers the music) and its sound channels ([[battle_effect_sound_mode_e]]) |
| [[battle_effect_secondary_init_t]] | The init record an action builds: the caster, up to the targets with their hit results, and the spectators |
| [[battle_effect_secondary_data_t]], `g_battle_effect_secondary_data` | The running secondary effects (16 records), each an animation id run by a handler through the phases of [[battle_effect_secondary_phase_e]] |
| `g_battle_effect_secondary_animations` ([[battle_effect_secondary_animation_t]]) | Animation id ([[battle_effect_secondary_animation_id_e]]) to handler and parameter |
| `g_battle_effect_particle_groups` ([[battle_effect_emitter_values_t]]) | The 17 particle groups of the secondary effects ([[battle_effect_particle_group_e]]): when a group starts its effect slots and the fixed emitter they come from. FFHacktics calls this data "TRAP vfx" |
| [[battle_effect_slot_t]], `g_battle_effect_slots` | The effect slots, the particles of the secondary effects |

## Main flow

1. **Request.** When a unit acts, [[battle_action_start_strike]] skips
   abilities with no effect ([[battle_effect_ability_has_no_effect]]) and
   otherwise asks for it ([[battle_effect_request_action_effect]]): the init
   record gets the caster, each target with its hit result and the
   spectators, and [[battle_effect_request_ability_effect]] picks the file,
   sets the phase to REQUESTED and starts the ability's charge secondary
   effect. An ability without a file starts only the equip break or red
   orbs effect. Event scripts request effects with
   [[battle_effect_start_new]].
2. **Load.** The START_EFFECT_FILE_OPEN state runs
   [[battle_effect_update_file_load]] once a frame: it reads the file,
   lets the charge play for 60 frames, ends it, lowers the music when the
   file asks, and waits for the charge to finish.
3. **Release.** [[battle_action_play_ability_effect]] releases the effect
   ([[battle_effect_play]]) as the action executes. An item ability's
   effect waits instead for its thrown item to land
   ([[battle_effect_linear_projectile_secondary_handler]] releases it),
   unless the unit holds the item up.
4. **Play.** Each frame [[battle_effect_update_cycle]] runs the file's
   effect, the secondary effects and the effect camera.
   [[battle_effect_update_stage]] sends the palette and texture to VRAM,
   makes the rest of the file the effect heap, binds the sections and starts
   the root record's script. Every frame each record runs its script until
   it yields ([[battle_effect_run_block]]): jumps and counters, child
   records, native callbacks, camera moves, particles and the two timelines
   (opcodes 28 and 29). Timeline keyframes recolour the screen and the
   units, play the file's sounds, move the camera and make the targets
   react ([[battle_effect_run_keyframe_actions]]). The effect ends when no
   record is left; the music comes back up and the map unfreezes.
5. **Particles.** Opcode 26 spawns an emitter's particles
   ([[battle_effect_spawn_emitter_particles]]), opcode 25 moves them with
   gravity, inertia and homing ([[battle_effect_integrate_particle_motion]])
   and draws their sprites ([[battle_effect_render_particle_sprite]]); the
   file's native callbacks draw its meshes.
6. **Secondary effects.** [[battle_effect_update_secondary_effects]] runs
   each active one's handler once a frame, newest first, and frees it when
   the handler returns 0. The handlers start effect slots from their
   particle group ([[battle_effect_start_group]]), step them with their own
   gravity and draw them as billboard sprites.
7. **Projectiles.** Arrows follow an arc and bolts, stones and thrown items
   a straight line, traced against the terrain and the units
   ([[battle_effect_set_and_validate_arc_trajectory]],
   [[battle_effect_check_direct_trajectory_to_target]]); targeting uses the
   same traces (see [[Targeting]]).

| Secondary effect | Started by | Handler |
|---|---|---|
| Weapon hit, Golem, equip break (BREAK results) | the unit's SEQ script or a projectile's hit ([[battle_effect_start_action_weapon_hit_secondary]]) | [[battle_effect_weapon_hit_secondary_handler]], [[battle_effect_golem_secondary_handler]], [[battle_effect_equip_break_secondary_handler]] |
| Arrow, crossbow bolt, thrown stone or item | [[battle_effect_start_action_projectile_secondary]] by weapon type or ability | [[battle_effect_arrow_arc_secondary_handler]], [[battle_effect_linear_projectile_secondary_handler]] |
| Spell and summon charge | the ability's charge animation set, at the request | [[battle_effect_spell_charge_secondary_handler]], [[battle_effect_summon_charge_secondary_handler]] |
| Red orbs, equip break | Charge, Accumulate and the Break abilities, which have no file | [[battle_effect_red_orbs_secondary_handler]], [[battle_effect_equip_break_secondary_handler]] |
| Reflect | the child timeline, when a target reflects the spell | [[battle_effect_reflect_secondary_handler]] |
| Level up, landing dust and splash, trap, zodiac poof, death smoke, teleport | the `battle_effect_set_secondary_*` starters | [[battle_effect_level_up_secondary_handler]], [[battle_effect_fall_dust_secondary_handler]], [[battle_effect_splash_secondary_handler]], [[battle_effect_trap_secondary_handler]], [[battle_effect_zodiac_poof_secondary_handler]], [[battle_effect_death_smoke_secondary_handler]], [[battle_effect_teleport_secondary_handler]] |

## Where to change

| To change | Edit |
|---|---|
| Which effect an ability plays | Its entry in `g_battle_effect_ability_files` (data); Throw Ball picks Fire, Ice or Bolt by element in [[battle_effect_request_ability_effect]] |
| Whether an ability plays one at all | [[battle_effect_ability_has_no_effect]] and its callers |
| The charge before a spell or summon | `g_battle_effect_secondary_by_charge_animation` (data), indexed by the ability's charge animation set |
| The projectile of a weapon type | `g_battle_effect_weapon_projectile_animations` (data) and [[battle_effect_start_action_projectile_secondary]] |
| A resident effect's look | Its particle group in `g_battle_effect_particle_groups` and its handler |
| The landing effects | [[battle_effect_set_landing_secondary_by_surface_type]] |
| The music during an effect | EFFECT_FLAG_QUIET_MUSIC in the file's flags section; the volumes in [[battle_effect_update_file_load]] and [[battle_effect_update_stage]] |
| What an effect file shows | Its script, timeline and emitters are data in the file; changing their size waits for build 3, since the build reads the file's data bytes from the disc |

## Quirks and debts

- The restart phase of [[battle_effect_update_stage]] is never entered, and
  the two start pointers it would restore are never written.
- Effect id 0 loads E001.BIN: entry 0 of `g_battle_effect_disc_entries`
  repeats entry 1.
- The interpreter indexes its opcode, particle renderer and callback slot
  tables with wider fields than they have, unchecked.
- [[battle_effect_step_emitter_timeline]] can enter a callback's destroy
  phase before its init ran, and the destroy path reads through a null
  work slot.
- [[battle_effect_remove_secondary]] leaves a stale link when a record in
  the middle of the active chain goes.
- [[battle_effect_set_secondary]] copies parts of its init record as the
  stack left them.
- Five handler slots of `g_battle_effect_secondary_handlers` are empty
  routines that no animation names.
- Several routines return a value no path sets for an input outside their
  range: [[battle_effect_update_file_load]],
  [[battle_effect_summon_charge_secondary_handler]],
  [[battle_effect_linear_projectile_secondary_handler]] and
  [[battle_effect_next_channel_sound]].
- [[battle_effect_draw_projectile_model]] (the Reflect shield) keeps only
  the last corner's depth and links every triangle at entry 0.
- Many divisions, here and in the effect overlays, rely on the R3000's
  result for a zero divisor; see `QUIRKS.md`.

## Functions in scope

![[Ability Effects scope]]
