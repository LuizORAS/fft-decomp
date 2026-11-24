#include "fft/event_bunit.h"
#include "fft/menu.h"
#include "psx/libgpu.h"
#include "psx/types.h"

/* Plain reads of these two capacities change the load scheduling. */
extern volatile u16 g_bunit_gfx_line_g2_capacity;
extern volatile u16 g_bunit_gfx_line_g3_capacity;

/* Bind both frame buffers' primitive pools, set up their draw/display
 * environments and present twice to establish the active pair.
 *
 * BUNIT twin of world_gfx_init_double_packet_buffers: one pool argument per
 * frame-buffer pointer, each advanced by its capacity for the second buffer. */
void bunit_gfx_init_double_packet_buffers(bunit_menu_gfx_context_t* buffers, u32* otags, POLY_F3* flat_triangles,
    POLY_FT3* textured_triangles, POLY_F4* flat_quads, POLY_FT4* textured_quads, POLY_G3* gouraud_triangles,
    POLY_GT3* gouraud_textured_triangles, POLY_G4* gouraud_quads, POLY_GT4* gouraud_textured_quads, LINE_F2* flat_lines,
    LINE_F3* flat_lines_3, LINE_F4* flat_lines_4, LINE_G2* gouraud_lines, LINE_G3* gouraud_lines_3,
    LINE_G4* gouraud_lines_4, TILE* tiles, TILE_1* tiles_1, TILE_8* tiles_8, TILE_16* tiles_16, SPRT* sprites,
    SPRT_8* sprites_8, SPRT_16* sprites_16, DR_MOVE* draw_moves, DR_AREA* draw_areas, DR_MODE* draw_modes) {
    s32 i;

    g_bunit_gfx_context_base = buffers;
    g_menu_gfx_context = buffers;
    for (i = 0; i < 2; i++) {
        g_bunit_gfx_context_base[i].otag = otags + g_bunit_gfx_otag_count * i;
        g_bunit_gfx_context_base[i].poly_f3 = flat_triangles + g_bunit_gfx_poly_f3_capacity * i;
        g_bunit_gfx_context_base[i].poly_ft3 = textured_triangles + g_bunit_gfx_poly_ft3_capacity * i;
        g_bunit_gfx_context_base[i].poly_f4 = flat_quads + g_bunit_gfx_poly_f4_capacity * i;
        g_bunit_gfx_context_base[i].textured_quads = textured_quads + g_bunit_gfx_poly_ft4_capacity * i;
        g_bunit_gfx_context_base[i].poly_g3 = gouraud_triangles + g_bunit_gfx_poly_g3_capacity * i;
        g_bunit_gfx_context_base[i].poly_gt3 = gouraud_textured_triangles + g_bunit_gfx_poly_gt3_capacity * i;
        g_bunit_gfx_context_base[i].poly_g4 = gouraud_quads + g_bunit_gfx_poly_g4_capacity * i;
        g_bunit_gfx_context_base[i].poly_gt4 = gouraud_textured_quads + g_bunit_gfx_poly_gt4_capacity * i;
        g_bunit_gfx_context_base[i].lines = flat_lines + g_bunit_gfx_line_f2_capacity * i;
        g_bunit_gfx_context_base[i].line_f3 = flat_lines_3 + g_bunit_gfx_line_f3_capacity * i;
        g_bunit_gfx_context_base[i].line_f4 = flat_lines_4 + g_bunit_gfx_line_f4_capacity * i;
        g_bunit_gfx_context_base[i].line_g2 = gouraud_lines + g_bunit_gfx_line_g2_capacity * i;
        g_bunit_gfx_context_base[i].line_g3 = gouraud_lines_3 + g_bunit_gfx_line_g3_capacity * i;
        g_bunit_gfx_context_base[i].line_g4 = gouraud_lines_4 + g_bunit_gfx_line_g4_capacity * i;
        g_bunit_gfx_context_base[i].tiles = tiles + g_bunit_gfx_tile_capacity * i;
        g_bunit_gfx_context_base[i].tiles_1 = tiles_1 + g_bunit_gfx_tile_1_capacity * i;
        g_bunit_gfx_context_base[i].tiles_8 = tiles_8 + g_bunit_gfx_tile_8_capacity * i;
        g_bunit_gfx_context_base[i].tiles_16 = tiles_16 + g_bunit_gfx_tile_16_capacity * i;
        g_bunit_gfx_context_base[i].sprites = sprites + g_bunit_gfx_sprite_capacity * i;
        g_bunit_gfx_context_base[i].sprites_8 = sprites_8 + g_bunit_gfx_sprite_8_capacity * i;
        g_bunit_gfx_context_base[i].sprites_16 = sprites_16 + g_bunit_gfx_sprite_16_capacity * i;
        g_bunit_gfx_context_base[i].moves = draw_moves + g_bunit_gfx_draw_move_capacity * i;
        g_bunit_gfx_context_base[i].draw_areas = draw_areas + g_bunit_gfx_draw_area_capacity * i;
        g_bunit_gfx_context_base[i].draw_modes = draw_modes + g_bunit_gfx_draw_mode_capacity * i;
        bunit_gfx_init_packet_pools(&g_bunit_gfx_context_base[i]);
#ifdef FIX_BUGS
        /* The edition draws all 256 columns of the screen (QUIRKS.md). */
        SetDefDrawEnv(&g_bunit_gfx_context_base[i].draw_env, 0, i * 0xf0, 0x100, 0xf0);
        SetDefDispEnv(&g_bunit_gfx_context_base[i].disp_env, 0, i == 0 ? 0xf0 : 0, 0x100, 0xf0);
#else
        SetDefDrawEnv(&g_bunit_gfx_context_base[i].draw_env, 0, i * 0xf0, 0xff, 0xf0);
        SetDefDispEnv(&g_bunit_gfx_context_base[i].disp_env, 0, i == 0 ? 0xf0 : 0, 0xff, 0xf0);
#endif
        g_bunit_gfx_context_base[i].draw_env.isbg = 0;
        setRGB0(&g_bunit_gfx_context_base[i].draw_env, 0, 0, 0);
        g_bunit_gfx_context_base[i].draw_env.ofs[0] = -0x80;
    }
    bunit_gfx_submit_frame_and_swap_buffers(0, -1);
    bunit_gfx_submit_frame_and_swap_buffers(0, -1);
}
