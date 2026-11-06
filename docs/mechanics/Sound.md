---
type: mechanic
tier: 3
scope:
  - main_sound
  - battle_sound
  - world_sound
  - wldcore_sound
  - sound_play_music_tracks()
  - sound_queue_sfx_cancel()
  - sound_queue_sfx_confirm()
  - sound_queue_sfx_confirm_unless_restoring()
  - sound_queue_sfx_cursor_move()
  - sound_queue_sfx_invalid()
  - sound_run_background_fade_thread()
  - sound_run_background_sfx_thread()
  - sound_wait_for_music_stub()
  - suzuki_sound_alloc_spu_ram()
  - suzuki_sound_calculate_noise_voices()
  - suzuki_sound_calculate_pitch_lfo_voices()
  - suzuki_sound_calculate_reverb_voices()
  - suzuki_sound_clear_memory()
  - suzuki_sound_commit_volume_change()
  - suzuki_sound_copy_memory()
  - suzuki_sound_find_sfx_channel()
  - suzuki_sound_find_waveset()
  - suzuki_sound_flush_key_off()
  - suzuki_sound_flush_key_on()
  - suzuki_sound_free_spu_ram()
  - suzuki_sound_free_waveset()
  - suzuki_sound_free_wavesets()
  - suzuki_sound_get_sfx_channels()
  - suzuki_sound_get_type()
  - suzuki_sound_init_sfx_music()
  - suzuki_sound_init_stub()
  - suzuki_sound_load_waveset()
  - suzuki_sound_play_2_sfx()
  - suzuki_sound_play_4_sfx()
  - suzuki_sound_play_sfx_in_channel()
  - suzuki_sound_play_sfx_with_settings_find_channel()
  - suzuki_sound_play_sfx_with_settings_in_channel()
  - suzuki_sound_put_type()
  - suzuki_sound_quit()
  - suzuki_sound_release_sfx_voices()
  - suzuki_sound_remove_resource()
  - suzuki_sound_root_counter_2_handler()
  - suzuki_sound_set_cd_reverb()
  - suzuki_sound_set_cd_volume()
  - suzuki_sound_set_master_volume()
  - suzuki_sound_set_reverb_mode()
  - suzuki_sound_set_sfx_balance()
  - suzuki_sound_set_sfx_enabled()
  - suzuki_sound_set_type()
  - suzuki_sound_set_vol_balance()
  - suzuki_sound_spu_event_handler()
  - suzuki_sound_start_sfx()
  - suzuki_sound_stop_all()
  - suzuki_sound_stop_resource_sfx()
  - suzuki_sound_stop_sfx_channel_pair()
  - suzuki_sound_stop_sfx_channels()
  - suzuki_sound_sum_resource_words()
  - suzuki_sound_transfer_spu_data()
  - suzuki_sound_update_voice_attrs()
  - suzuki_sound_wait_for_spu_transfer()
---

# Sound

## What it does in the game

MAIN controls every sound the game makes through the Suzuki sound driver
(`suzuki_smd_*`, `Suzuki*`; see [[Sound Driver]]), which plays sequences on the SPU's 24 voices from
an interrupt (root counter 2, [[suzuki_sound_root_counter_2_handler]]). Three
kinds of sound reach it:

| What the player hears | What does it | Code |
|---|---|---|
| The music of a battle, the world map or a screen | A music file (MUSIC_NN.SMD, 100 of them) loaded into one of two scenario music slots and started from its beginning with a fade | [[main_sound_load_music_into_slot]], [[main_sound_switch_music_track]] |
| A short jingle over the music: Level Up, an item found, Job Level Up, a job change | A tune: the music fades out, the tune plays once from its own slot, then the music fades back in | [[main_sound_play_tune]], [[main_sound_update_tunes]] |
| Menu clicks, swings and hits, steps and landings, rain and thunder, the sounds of an ability | A sound effect: a sound of a loaded sound resource played on a pair of the eight SFX channels | [[main_sound_play_sfx]], [[main_sound_play_sfx_find_channel]], [[suzuki_sound_start_sfx]] |
| An environment sound behind an event | A background sound of ENV.SED that an event starts, fades and stops | [[sound_run_background_sfx_thread]], [[sound_run_background_fade_thread]] |
| Mono, stereo or wide sound | The sound type of the Options screen and the title's Sound menu | [[suzuki_sound_set_type]] |

OPEN's movies and the ending's XA tracks play CD audio, which
[[open_sound_set_cd_mix_and_volume]] fades in and out (see
[[Opening and Title]]).

## Data and structures

| Record | What it holds |
|---|---|
| `g_main_sound_music` ([[main_sound_music_t]]) | The playing scenario music (its slot, record, SMD data and volume), the tune, and the music slot table: slots 1 and 2 for the scenario's music, slots 3 to 7 for the five tunes |
| `g_main_sound_scenario_smd_files`, `g_main_sound_permanent_smd_files` ([[main_file_extent_t]]) | The sector and size of each music file ([[main_music_e]]) and of the five tunes, MUSIC_45 to MUSIC_49 ([[main_sound_tune_e]]) |
| [[suzuki_sound_resource_t]] | A loaded sound resource ("feds" block): SOUND/SYSTEM.SED ([[main_sfx_e]], 168 sounds), SOUND/ENV.SED ([[main_env_sfx_e]], 27), the block each ability effect file brings and the sound novels' bank. A sound id is the resource id in the high 16 bits and the sound in the low one ([[main_sound_resource_id_e]]) |
| `g_suzuki_sound_sfx_music` | The SFX record: eight channels on SPU voices 16 to 23. Each sound has two streams, so it plays on a channel pair |
| [[suzuki_sound_sfx_priority_e]] | Who owns a channel pair: [[main_sound_play_sfx]] takes channels 6-7 and replaces what plays there, the background sound takes 4-5, and the other sounds take a free pair or steal the oldest stealable one ([[suzuki_sound_find_sfx_channel]]) |
| `g_sound_pending_sfx_id` (BATTLE and WORLD) | The menu sound queued during a frame and played once the frame's threads have run |
| `g_wldcore_audio_queue` ([[wldcore_audio_queue_t]]) | The world map's audio queue: music loads, fades and unloads ([[wldcore_audio_command_e]]) run one after the other while the frames go on |
| `g_battle_sound_weapon_swing_sfx_ids` and its two siblings | The swing, hit and shield sounds of each weapon type |

## Main flow

1. **At boot** [[main_sound_open_generic_sfx]] starts the driver, uploads
   SOUND/WAVESET.WD (the instruments) to SPU RAM, appends SYSTEM.SED and
   ENV.SED to the sound resources and loads the five tunes, which stay
   loaded.
2. **Music.** A scenario record names two tracks
   (`g_battle_scenario_music_1_id`, `_2_id`). When its battle is set up,
   ATTACK loads both and plays the first, or keeps the track already playing
   when the new pair holds it
   ([[attack_sound_resync_scenario_music_and_apply_map_darkness]]); the
   deployment screen plays its own (`MAIN_MUSIC_DEPLOYMENT`, through
   [[sound_play_music_tracks]]). Event instructions switch
   between the two tracks, fade them, end a track or play a tune; BATTLE
   carries out the music requests at the end of the frame
   ([[battle_script_process_pending_requests]]). The world map runs its
   music through the audio queue ([[wldcore_sound_enqueue_music_start]],
   [[wldcore_sound_process_audio_queue]]); the shop, the formation screen and
   the save screens load their own track into a free slot
   ([[main_sound_load_music_into_free_slot]]) and unload it on the way out.
3. **Sound effects.** Menus queue theirs for the frame
   ([[sound_queue_sfx_confirm]] and its siblings,
   [[world_sound_dispatch_effect]]); WLDCORE plays them at once
   ([[wldcore_sound_play_effect]]). In battle the animation scripts play
   sounds (SEQ opcode `0xf6`), a weapon swing and its hit come from the
   weapon type ([[battle_sound_play_weapon_sfx]]), a landing from the tile's
   surface ([[battle_sound_play_landing_sfx_by_surface_type]]), and the
   weather from ENV.SED (rain, two thunders: [[battle_sound_start_weather_sfx]]).
   An ability effect file brings its own sound resource.
4. **Every tick** of root counter 2 the driver commits the voice attributes,
   key-ons and key-offs, steps the volume ramps and advances each playing
   sequence by its tempo; [[main_sound_update_tunes]] runs the tune once a
   frame from BATTLE's loop and the formation screen.

## Where to change

| To change | Edit |
|---|---|
| The music of a screen | The [[main_music_e]] id its code passes: `world_shop_run_screen`, `world_formation_run_screen`, the save screens, `etc_graphic_show_game_over`, `require_text_show_battle_congratulations` |
| A battle's music | The two music ids of its scenario record (data, read by `attack_scenario_load_conditionals`) |
| When a tune plays | The [[main_sound_play_tune]] callers: level up, job level up, items found, the job change |
| A menu or weapon sound | The [[main_sfx_e]] id at its caller, or the weapon tables `g_battle_sound_weapon_swing_sfx_ids` and its siblings (data) |
| The landing sounds | [[battle_sound_play_landing_sfx_by_surface_type]] |
| How sound effects share channels | [[suzuki_sound_find_sfx_channel]] and the channel ids of [[main_sound_play_sfx]] and [[main_sound_play_bg_sfx]] |
| The sound types | [[suzuki_sound_set_type]], [[suzuki_sound_put_type]] |

## Quirks and debts

- [[main_sound_unload_scenario_mus]] releases the playing slot's record
  without clearing the playing music, so the next stop hands the released
  record to the driver; the shop, formation and save screens unload their
  music this way. The Developer's Cut clears it at the unload.
- [[main_sound_update_tunes]] asks whether the scenario music plays even
  when none is loaded, and reads the null record.
- The battle map's load steps ([[battle_map_step_setup_sequence]],
  [[battle_map_step_init_sequence]]) call [[main_sound_stop_sfx]] without
  its sound id.
- SYSTEM.SED's sound 0 has no streams, so playing it stops channels 6-7;
  the world map does that when a path animation ends.
- [[battle_sound_stop_weather_sfx]] returns a result it never sets.
- WORLD's music requests and background sound threads belong to its copy of
  the event interpreter, which never runs.
- The driver's own quirks (unused opcodes, the waveset read past its copy)
  are on the [[Sound Driver]] page; the reverb mode's is in `QUIRKS.md`
  under `suzuki_sound_set_reverb_mode`.

## Functions in scope

![[Sound scope]]
