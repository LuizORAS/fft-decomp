---
type: mechanic
tier: 4
scope:
  - effect
---

# Effect Files

## What it does in the game

The disc holds 512 effect files, `EFFECT/E000.BIN` to `E511.BIN`; each
ability with a visual effect names one ([[Ability Effects]] explains how the
file is picked, loaded and played). Most are data only: a script,
timelines, emitters, sprite frames, a palette and sounds that BATTLE's
effect engine plays by itself. **110 of them also carry native code**, and
those are this repository's `effect-*` modules (`target/effect.yaml`). Their
code draws what particles cannot: spinning rings and domes, scrolling
textured grids, ribbons and tentacles, or it changes the battle map
(freezes it, switches its 3D objects, breaks it apart).

The files with code belong to the bigger abilities:
- the high white and time magic (Cure 4, Raise, Raise 2, Reraise, Holy,
  Haste 2, Slow 2, Reflect, Meteor);
- every summon from Shiva to Zodiac;
- the seven dances, two Draw Out swords, Wish and the sword skills Holy
  Explosion, Galaxy Stop and Shock;
- Cloud's eight Limits;
- the Ja spells and the dimension magic;
- Ultima, All-Ultima and many monster and boss skills (Morbol's Tentacle,
  Behemoth, Tiamat, Grand Cross, Midgar Swarm, Snake Carrier...);
- 31 of the effect ids past the abilities (0x201 to 0x225), which no ability
  names: event scripts request them.

Each file's routines say which ability or effect id uses it.

## Data and structures

| Record | What it holds |
|---|---|
| `g_battle_effect_native_callbacks` | The address of each native callback id. In the original, each file's code sits so that its callbacks land on the addresses their ids name; the build writes each entry from its routine (a relocation), so it follows the code. Ids 1 and 90 name no routine |
| `battle_effect_record_t.callback_phase` ([[effect_callback_phase_e]]) | The phase of a bound callback: INIT allocates its work block, UPDATE runs once a frame, DESTROY frees the block two frames before its keyframe ends |
| [[effect_geometry_entry_t]] and its views | The emitter record a callback reads through its own layout: [[effect_radial_mesh_geometry_t]] (ring, tube and summon-dome meshes), [[effect_polar_mesh_geometry_view_t]], [[effect_textured_quad_geometry_view_t]], [[effect_particle_ribbon_geometry_view_t]], [[effect_tentacle_mesh_geometry_view_t]] |
| The work blocks of `fft/effect.h` | Each renderer's double-buffered `POLY_GT4` quads and its spin, scroll and wave state, allocated from the effect heap |
| The scratchpad (`0x1f800000`) | The lattice points a mesh renderer builds before it projects them |

## Main flow

1. The effect file loads like any other ([[battle_effect_update_file_load]]).
2. Script opcode 06 puts a native callback in a callback slot
   ([[battle_effect_code_script_06_register_native_callback]]); a particle
   keyframe whose action names the slot binds it to the record. Opcode 07
   calls one directly.
3. While that keyframe runs, the particle channel calls the callback once a
   frame instead of the emitter, with the record, the slot, the emitter
   record and the frame. The callback reads its curves (`READ_CURVE`),
   rebuilds its mesh in one buffer, links it into the ordering table and
   flips buffers.
4. Two frames before the keyframe ends the channel sets DESTROY, and the
   callback frees its work block.

| Family | Files | What it draws or does | Example |
|---|---|---|---|
| Ring mesh | 32 | A spinning, flaring ring of 2x32 textured quads | [[effect_update_ring_mesh]] |
| Ring stack mesh | 31 | A spinning stack of 8 rings of 8 quads | [[effect_update_ring_stack_mesh]] |
| Textured quad grid | 27 (4x4), 7 (8x8), 3 (2x2) | A grid whose texture scrolls along curves | [[effect_update_textured_quad_grid_4x4]] |
| Polar mesh | 23, plus 5 anchored to a target | A four-fold symmetric mesh of quads | [[effect_update_polar_mesh]], [[effect_update_anchored_polar_mesh]] |
| Summon mesh | 12 | A spinning dome of 8x16 quads | [[effect_update_dome_mesh]] |
| Particle ribbons | 8 short, 8 long | Homing particles that trail a ribbon | [[effect_update_particle_ribbon_short]] |
| Wave mesh | 8 | An 8x8 grid distorted by waves | [[effect_update_wave_mesh]] |
| Tentacle mesh | 6 | Up to eight textured strips that follow points | [[effect_update_tentacle_mesh]] |
| Tube mesh | 4 | A spinning closed tube of 9x8 quads | [[effect_update_tube_mesh]] |
| Particle trails | 4 | The tentacle renderer without its mesh | [[effect_update_particle_trails]] |
| Single ring mesh | 3 | One ring of 32 quads | [[effect_update_single_ring_mesh]] |
| Map steps | E450, E453-E456, E458, E480 | Freeze and unfreeze the map, switch its 3D objects, split its mesh, break it, time weather command 0x96 | [[effect_map_freeze]], [[effect_e456_map_update_specialized_destruction_state]] |
| Particle spawns | E454, E464 | Spawn particles at work vertices: Banish and Altima's transformation | [[effect_e454_particle_update_banish_state]], [[effect_e464_update_altima_transformation_map_vertex_particles]] |

## Copies

Each family is one source compiled into every file that uses it, at that
file's addresses. So the 210 routines of the 110 files are 115 distinct
codes: 32 groups are byte for byte identical across files, and a few
differ only in constants. [[effect_load_dome_mesh_translated_vertex]] is the
one routine that keeps a single name in all 12 files that carry it. A fix
to a family means a fix to every copy.

## Where to change

| To change | Edit |
|---|---|
| What a family draws | Its routine in each file that carries it (see the table) and its record in `fft/effect.h` |
| Which callback an effect uses, and when | The file's script (opcodes 06 and 07) and keyframes: data in the file |
| The curves a renderer follows | The emitter record and the animation curves of the file: data |
| Adding code to an effect file | A change under an option, in the Developer's Cut: the callback table and the file header offsets follow the code, so a file grows up to the effect window's end (`make build EDITION=dc` prints the room), which shrinks the battle heap that runs from its palette section to there |

## Quirks and debts

- E000's script calls only [[effect_e000_noop]], and E314's names no
  routine of its file, so callback ids 1 and 90 are bound by nothing. Some
  handlers sit in slots no keyframe fires, and E457.BIN, which has no code,
  binds a callback of E259 (see `QUIRKS.md`).
- The `effect_eNNN_update_*` renderers can enter DESTROY before INIT ran;
  the destroy path then reads through a null work slot.
- The wave mesh copies divide by an interpolated value that can be 0.

## Which ability each file plays

Each `EFFECT/E*.BIN` file and the ability (or effect id past the abilities) it is the effect of.

| File | Effect of |
|---|---|
| `E000.BIN` | ability 0 and of effect id 0x225, one of the ids past the abilities |
| `E004.BIN` | `ABILITY_ID_WHITE_MAGIC_CURE_4` |
| `E005.BIN` | `ABILITY_ID_WHITE_MAGIC_RAISE` |
| `E006.BIN` | `ABILITY_ID_WHITE_MAGIC_RAISE_2` |
| `E007.BIN` | `ABILITY_ID_WHITE_MAGIC_RERAISE` |
| `E015.BIN` | `ABILITY_ID_WHITE_MAGIC_HOLY` |
| `E033.BIN` | `ABILITY_ID_TIME_MAGIC_HASTE_2` |
| `E035.BIN` | `ABILITY_ID_TIME_MAGIC_SLOW_2` |
| `E041.BIN` | `ABILITY_ID_TIME_MAGIC_REFLECT` |
| `E047.BIN` | `ABILITY_ID_TIME_MAGIC_METEOR` |
| `E065.BIN` | `ABILITY_ID_SUMMON_MAGIC_SHIVA` |
| `E067.BIN` | `ABILITY_ID_SUMMON_MAGIC_IFRIT` |
| `E068.BIN` | `ABILITY_ID_SUMMON_MAGIC_TITAN` |
| `E069.BIN` | `ABILITY_ID_SUMMON_MAGIC_GOLEM` |
| `E070.BIN` | `ABILITY_ID_SUMMON_MAGIC_CARBUNKLE` |
| `E071.BIN` | `ABILITY_ID_SUMMON_MAGIC_BAHAMUT` |
| `E072.BIN` | `ABILITY_ID_SUMMON_MAGIC_ODIN` |
| `E073.BIN` | `ABILITY_ID_SUMMON_MAGIC_LEVIATHAN` |
| `E074.BIN` | `ABILITY_ID_SUMMON_MAGIC_SALAMANDER` |
| `E076.BIN` | `ABILITY_ID_SUMMON_MAGIC_SILF` |
| `E077.BIN` | `ABILITY_ID_SUMMON_MAGIC_FAIRY` |
| `E079.BIN` | `ABILITY_ID_SUMMON_MAGIC_CYCLOPS` |
| `E080.BIN` | `ABILITY_ID_SUMMON_MAGIC_ZODIAC` |
| `E088.BIN` | `ABILITY_ID_DANCE_WITCH_HUNT` |
| `E089.BIN` | `ABILITY_ID_DANCE_WIZNAIBUS` |
| `E090.BIN` | `ABILITY_ID_DANCE_SLOW_DANCE` |
| `E091.BIN` | `ABILITY_ID_DANCE_POLKA_POLKA` |
| `E092.BIN` | `ABILITY_ID_DANCE_DISILLUSION` |
| `E093.BIN` | `ABILITY_ID_DANCE_NAMELESS_DANCE` |
| `E094.BIN` | `ABILITY_ID_DANCE_LAST_DANCE` |
| `E134.BIN` | `ABILITY_ID_DRAW_OUT_KOUTETSU` |
| `E138.BIN` | `ABILITY_ID_DRAW_OUT_KIYOMORI` |
| `E161.BIN` | `ABILITY_ID_GUTS_WISH` |
| `E167.BIN` | `ABILITY_ID_HOLY_SWORD_HOLY_EXPLOSION` |
| `E174.BIN` | `ABILITY_ID_ALL_MAGIC_DARK_HOLY` |
| `E176.BIN` | `ABILITY_ID_STARRY_HEAVEN_GALAXY_STOP` |
| `E193.BIN` | `ABILITY_ID_BYBLOS_SHOCK` |
| `E228.BIN` | `ABILITY_ID_DIMENSION_MAGIC_MELT` |
| `E229.BIN` | `ABILITY_ID_DIMENSION_MAGIC_TORNADO` |
| `E230.BIN` | `ABILITY_ID_ALL_MAGIC_QUAKE` |
| `E233.BIN` | `ABILITY_ID_JA_MAGIC_TOAD_2` |
| `E235.BIN` | `ABILITY_ID_JA_MAGIC_FLARE_2` |
| `E236.BIN` | `ABILITY_ID_JA_MAGIC_BLIND_2` |
| `E240.BIN` | `ABILITY_ID_JA_MAGIC_SLEEP_2` |
| `E241.BIN` | `ABILITY_ID_ULTIMATE_MAGIC_ULTIMA` |
| `E242.BIN` | `ABILITY_ID_COMPLETE_MAGIC_ALL_ULTIMA` |
| `E244.BIN` | `ABILITY_ID_SATURATION_DESPAIR_2` |
| `E245.BIN` | `ABILITY_ID_SATURATION_RETURN_2` |
| `E246.BIN` | `ABILITY_ID_MAGIC_SWORD_SHOCK` |
| `E247.BIN` | `ABILITY_ID_LIMIT_BRAVER` |
| `E248.BIN` | `ABILITY_ID_LIMIT_BLADE_BEAM` |
| `E249.BIN` | `ABILITY_ID_LIMIT_CROSS_SLASH` |
| `E255.BIN` | `ABILITY_ID_LIMIT_CLIMHAZZARD` |
| `E256.BIN` | `ABILITY_ID_LIMIT_METEORAIN` |
| `E257.BIN` | `ABILITY_ID_LIMIT_FINISH_TOUCH` |
| `E258.BIN` | `ABILITY_ID_LIMIT_CHERRY_BLOSSOM` |
| `E259.BIN` | `ABILITY_ID_LIMIT_OMNISLASH` |
| `E314.BIN` | `ABILITY_ID_DRAGON_HOLY_BRACELET` |
| `E317.BIN` | `ABILITY_ID_MONSTER_BLACK_CHOCOBO_CHOCO_BALL` |
| `E336.BIN` | `ABILITY_ID_MONSTER_PISCO_DEMON_TENTACLE` |
| `E338.BIN` | `ABILITY_ID_MONSTER_SQUIDLARKIN_ODD_SOUNDWAVE` |
| `E341.BIN` | ABILITY_ID_MONSTER_SKELETON_KNIFE_HAND and ABILITY_ID_REACTION_DAMAGE_SPLIT |
| `E355.BIN` | `ABILITY_ID_MONSTER_AHRIMAN_DEATH_SENTENCE` |
| `E371.BIN` | `ABILITY_ID_MONSTER_BULL_DEMON_SHAKE_OFF` |
| `E373.BIN` | `ABILITY_ID_MONSTER_SACRED_MIMIC_TITAN` |
| `E376.BIN` | `ABILITY_ID_MONSTER_MORBOL_TENTACLE` |
| `E381.BIN` | `ABILITY_ID_MONSTER_BEHEMOTH_STAB_UP` |
| `E382.BIN` | `ABILITY_ID_MONSTER_BEHEMOTH_SUDDEN_CRY` |
| `E383.BIN` | `ABILITY_ID_NIGHT_MAGIC_HURRICANE` |
| `E384.BIN` | `ABILITY_ID_NIGHT_MAGIC_ULMAGUEST` |
| `E385.BIN` | `ABILITY_ID_DARK_MAGIC_GIGA_FLARE` |
| `E395.BIN` | `ABILITY_ID_MONSTER_TIAMAT_DARK_WHISPER` |
| `E401.BIN` | `ABILITY_ID_DARK_CLOUD_SNAKE_CARRIER` |
| `E402.BIN` | `ABILITY_ID_DARK_CLOUD_POISON_FROG` |
| `E403.BIN` | `ABILITY_ID_PHANTOM_MIDGAR_SWARM` |
| `E408.BIN` | `ABILITY_ID_NIGHT_MAGIC_NANOFLARE` |
| `E409.BIN` | `ABILITY_ID_CHAOS_GRAND_CROSS` |
| `E411.BIN` | `ABILITY_ID_WORK_COMPRESS` |
| `E412.BIN` | `ABILITY_ID_WORK_DISPOSE` |
| `E450.BIN` | effect id 0x218, one of the ids past the abilities |
| `E451.BIN` | effect id 0x21d, one of the ids past the abilities |
| `E452.BIN` | effect id 0x209, one of the ids past the abilities |
| `E453.BIN` | effect id 0x221, one of the ids past the abilities |
| `E454.BIN` | effect id 0x219, one of the ids past the abilities |
| `E455.BIN` | effect id 0x20f, one of the ids past the abilities |
| `E456.BIN` | effect id 0x203, one of the ids past the abilities |
| `E458.BIN` | effect id 0x201, one of the ids past the abilities |
| `E460.BIN` | effect id 0x216, one of the ids past the abilities |
| `E461.BIN` | effect id 0x204, one of the ids past the abilities |
| `E462.BIN` | effect id 0x205, one of the ids past the abilities |
| `E463.BIN` | effect id 0x206, one of the ids past the abilities |
| `E464.BIN` | effect id 0x21a, one of the ids past the abilities |
| `E465.BIN` | effect id 0x207, one of the ids past the abilities |
| `E466.BIN` | effect id 0x208, one of the ids past the abilities |
| `E467.BIN` | effect id 0x210, one of the ids past the abilities |
| `E468.BIN` | effect id 0x211, one of the ids past the abilities |
| `E469.BIN` | effect id 0x212, one of the ids past the abilities |
| `E471.BIN` | effect id 0x214, one of the ids past the abilities |
| `E472.BIN` | effect id 0x215, one of the ids past the abilities |
| `E473.BIN` | effect id 0x21b, one of the ids past the abilities |
| `E474.BIN` | effect id 0x20a, one of the ids past the abilities |
| `E475.BIN` | effect id 0x20b, one of the ids past the abilities |
| `E476.BIN` | effect id 0x20c, one of the ids past the abilities |
| `E477.BIN` | effect id 0x20d, one of the ids past the abilities |
| `E478.BIN` | effect id 0x20e, one of the ids past the abilities |
| `E480.BIN` | effect id 0x220, one of the ids past the abilities |
| `E481.BIN` | effect id 0x21e, one of the ids past the abilities |
| `E482.BIN` | effect id 0x21f, one of the ids past the abilities |
| `E483.BIN` | effect id 0x21c, one of the ids past the abilities |
| `E485.BIN` | effect id 0x224, one of the ids past the abilities |

## Functions in scope

![[Effect Files scope]]
