---
type: mechanic
tier: 3
scope:
  - battle_gfx
  - battle_unit
  - world_unit
  - world_gfx
  - main_gfx
  - battle_unit_copy_world_coords_to_scratch()
  - world_unit_copy_world_coords_to_scratch()
  - gfx_append_gpu_primitive_to_secondary_otag()
  - gfx_draw_or_append_gpu_primitive()
  - gfx_flip_poly_ft4_uv_if_negative()
  - gfx_init_image_loading()
  - gfx_set_primitive_brightness()
  - menu_copy_sprite_position_uv_and_size()
  - menu_init_default_poly_ft4()
  - script_prepare_unit_filter()
  - unit_copy_world_coords()
  - unit_copy_world_coords_to_vector()
  - unit_get_stats_by_unit_id()
  - unit_has_any_current_status()
  - unit_increase_blue_team_brave_by_10()
  - unit_is_moving()
  - unit_project_to_screen()
  - unit_reset_animation_states()
  - unit_set_facing()
---

# Sprites and Graphics

## What it does in the game

The units of a battle are 2D sprites standing in the 3D map. Each one has a
display record, the "runtime unit" record ([[battle_unit_t]], sixteen
of them in a list), which holds where the sprite stands, which way it faces,
the animation it plays and the pictures it shows. BATTLE draws them every
frame together with what surrounds them: shadows, status bubbles, the numbers
and messages after an action, the map cursor and the wait arrows.

| What the player sees | What does it | Code |
|---|---|---|
| A unit's picture | Its sheet (SPR: the pictures and 16 palettes) sits in one of nine spritesheet slots in VRAM; the sheet's SHP file cuts each frame into parts, and its SEQ file strings frames into animations | [[battle_gfx_load_spritesheet_into_vram_slot]], [[battle_gfx_run_unit_seq_script]] |
| The same unit from the front or from behind | The facing relative to the camera picks sequence 2n (front) or 2n + 1 (behind) of animation n and mirrors the sprite | [[battle_gfx_update_and_animate_unit_wep_eff]] |
| A swing, a cast, a weapon in the hand | Three sprite blocks beside the unit's own sprite play the weapon and effect sequences of WEP and EFF | [[battle_gfx_run_wep_eff_seq_script]], [[battle_gfx_draw_unit_sprite_layers]] |
| Attack poses and extra frames | A sheet's attack image is kept compressed and expanded into one of two EVTCHR slots when an attack sequence needs it; some sheets read the frames of their later attack animations from an SP2 file; events load EVTCHR.BIN blocks there | [[battle_gfx_load_unit_into_evtchr_slot]], [[battle_gfx_open_sp2_file]], [[battle_gfx_unpack_evtchr_file_to_vram_slot]] |
| A unit tinted red, green, blue or gray | Its CLUT in the units' row of map CLUTs is tinted for its first tinting status, for the weather, the time of day and the tile, or by team while a target is chosen | [[battle_gfx_update_unit_status_palette]], [[battle_gfx_tint_unit_palette_for_weather_and_tile]], [[battle_gfx_modify_palette_based_on_team]] |
| A unit flashing away | The fade-out removal: a color flash, additive blending and a palette fade, then the record is removed | [[battle_gfx_start_unit_fade_out]], [[battle_unit_update_fade_out_removal]] |
| Numbers and words over a unit | After an action: damage and healing, stat changes, Missed, Guarded, the statuses added or removed, Level UP, EXP and JP, and the failure messages of a charged action | [[battle_gfx_prepare_post_action_display]], [[battle_gfx_build_next_action_result_display]] |
| The status bubble | It shows the unit's statuses in turn, one every 16 ticks, once the camera has been still for 60 frames | [[battle_gfx_determine_status_bubble_parameters]], [[battle_gfx_update_status_bubbles_and_graphics]] |
| Shadows, the cursor and the wait arrows | A shadow quad laid on the tile, the map selection cursor with its tile glow, and four arrows around the acting unit | [[battle_gfx_draw_unit_shadow]], [[battle_gfx_draw_map_selection_cursor]], [[battle_gfx_draw_wait_direction_arrows]] |
| The screen fading or tinted | A full-screen tile that subtracts or adds a gray level, and a color modulation that events fade | [[battle_gfx_submit_screen_fade_overlay]], [[battle_gfx_start_screen_color_modulation_fade]] |

WORLD has its own drawing library (`world_gfx_*`) for its menus: two packet
buffers with pools of every primitive kind, quads and tiles, portraits, the
zodiac backdrop and the screen fades. Most of its `world_unit_*` routines are
copies of BATTLE's unit routines for its copy of the event interpreter, which
never runs.
MAIN keeps the display and draw environments, the Now Loading message, the
FRAME.BIN image, the zodiac backdrop of the deployment screen and the debug
font (`main_gfx_*`).

## Data and structures

| Record | What it holds |
|---|---|
| [[battle_unit_t]] (`g_battle_units`) | A unit's display record: position, facing, mount link, animation state, its sprite display and three sprite blocks, the numeric displays and the status bubble |
| [[battle_gfx_spritesheet_slot_t]] (`g_battle_gfx_spritesheet_slots`) | The nine sheets in VRAM: the sheet id, its compressed attack image and its 16 CLUTs |
| [[battle_gfx_vram_slot_t]] (`g_battle_gfx_vram_slots`) | The two EVTCHR slots: who owns the 256x200 page (an attack image, an SP2 file or an EVTCHR block) and the buffer it is built in |
| [[battle_gfx_spritesheet_data_t]] (`g_battle_gfx_spritesheet_data`) | Per sheet id ([[battle_spritesheet_id_e]]): its SHP and SEQ types, the flying flag and the sprite height |
| [[battle_gfx_unit_shp_frame_tables_t]], [[battle_gfx_unit_seq_t]] | A sheet's frame tables and animation table. Resident: the SHP types TYPE1, TYPE2, CYOKO, MON and OTHER and the SEQ types TYPE1, TYPE3, CYOKO, MON and OTHER; a battle with a boss loads ARUTE or KANZEN (SHP and SEQ) or RUKA (SEQ) |
| [[battle_gfx_sprite_display_data_t]] | A sprite's parts: their places, texture rectangles and flags |
| `g_battle_render_buffers` ([[battle_render_buffer_t]]) | BATTLE's two render buffers: the map polygon pools, the background gradient quad, the weather sprites and the ordering table |
| `g_battle_gfx_sprite_prim_pool` | The 256 POLY_FT4s, used as a ring, that sprites, shadows and markers are drawn with |
| [[battle_numeric_display_selector_e]] | The numbers and words a unit can show after an action |

## Main flow

1. **Loading.** The map load sequences queue each unit's graphics
   ([[battle_gfx_append_unit_graphics_load_descriptor]]) and step the loader
   ([[battle_gfx_step_queued_unit_graphics_load]]): per unit its SHP, SEQ and
   SPR files, skipping what is already resident. The shared files (sprite,
   weapon and effect SHP and SEQ, WEP.SPR, OTHER.SPR) load once
   ([[battle_gfx_step_load_unit_shp_seq_data]]). A unit's display record is
   claimed and filled by [[battle_unit_init]].
2. **Each frame** [[battle_unit_update_and_animate_units]] advances every
   unit's animation, status bubble and fade-out; requests made since the last
   frame ([[battle_unit_set_animation]], [[battle_unit_store_animation_facing]])
   start there. Then [[battle_gfx_update_status_bubbles_and_graphics]] draws
   each unit's layers: its own sprite and its sprite blocks in the order the
   animation asks, a rider inside its mount's layers, then the shadow and the
   afterimage copies; each part becomes one POLY_FT4
   ([[battle_gfx_construct_polygon_data_for_units]]).
3. **After an action** [[battle_gfx_prepare_post_action_display]] queues the
   unit's displays and they play one after the other.

## Where to change

| To change | Edit |
|---|---|
| The status tints and their order | [[battle_gfx_update_unit_status_palette]] |
| The weather, night and tile tints | [[battle_gfx_tint_unit_palette_for_weather_and_tile]] |
| The team colors of the target selection | [[battle_gfx_modify_palette_based_on_team]] |
| How a unit vanishes | [[battle_gfx_start_unit_fade_out]], [[battle_unit_update_fade_out_removal]] |
| The damage numbers and messages | [[battle_gfx_build_next_action_result_display]], [[battle_gfx_animate_post_action_text]] |
| The status bubble's timing | [[battle_gfx_determine_status_bubble_parameters]], [[battle_gfx_update_status_bubbles_and_graphics]] |
| Shadows | [[battle_gfx_draw_unit_shadow]], [[battle_gfx_calculate_sprite_shadow_from_tile_slope]] |
| Which sheet a unit uses | [[battle_spritesheet_id_e]] and `g_battle_gfx_spritesheet_data` (data) |

## Quirks and debts

- A tenth sheet has no slot, but the claim's failure is stored in a byte and
  tested as 0xffff, so it loads into slot 255 ([[battle_unit_init]]).
- [[battle_unit_init]] tints a new unit's palette before it clears
  the mount state, so a slot last used by a rider reads through NULL.
- [[battle_gfx_append_unit_graphics_load_descriptor]] writes a seventeenth
  descriptor past the 16-entry queue.
- [[battle_gfx_load_unit_attack_image_into_evtchr_slot]] fills one slot's image and
  marks the caller's slot; [[battle_gfx_release_reserved_vram_slot]] also
  releases a saved EVTCHR slot.
- [[battle_gfx_draw_unit_shadow]] recalculates the shadow on every call.
- The CT-zero display reads an unset row offset
  ([[battle_gfx_build_next_action_result_display]]), so CT 0 stands lower
  than Quick and Missed. The Developer's Cut shows it at their height.
- A unit off the map reads the null tile for its shadow and palette.

## Functions in scope

![[Sprites and Graphics scope]]
