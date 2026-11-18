#include "fft/event_bunit.h"
#include "fft/menu.h"
#include "fft/menu_script.h"
#include "psx/types.h"

/* Draw record: print a callback-supplied number right-aligned in a field of
 * cmd->number.width eight-pixel cells, then advance past the record. */
menu_instruction_t* bunit_menu_script_draw_plain_number(menu_instruction_t* cmd) {
    s8 digits[16];
    RECT glyph_rect;
    s32 count;
    s16 blanks;
    s16 i;
    s32 j;
    s32 x;
    s32 y;

    count = g_menu_row_callbacks[cmd->number.getter].value(cmd->number.value);
    count = sprintf(digits, g_bunit_text_decimal_format, count);
    x = cmd->number.x;
    y = cmd->number.y;
    blanks = cmd->number.width - count;
    count = cmd->number.width - blanks;
    g_bunit_gfx_blank_cell_rect.position.y = y + 4;
#ifdef FIX_BUGS
    /* The edition draws no blank cell for a number wider than its field (QUIRKS.md). */
    for (i = blanks - 1; i >= 0; i--) {
        g_bunit_gfx_blank_cell_rect.position.x = x;
        menu_gfx_enqueue_oriented_textured_quad(
            &g_bunit_gfx_blank_cell_rect, 0, TEXTURE_UV_FLIP_NONE, 0, g_menu_gfx_otag_index);
        x += 8;
    }
#else
    for (i = blanks - 1; i != -1; i--) {
        g_bunit_gfx_blank_cell_rect.position.x = x;
        menu_gfx_enqueue_oriented_textured_quad(
            &g_bunit_gfx_blank_cell_rect, 0, TEXTURE_UV_FLIP_NONE, 0, g_menu_gfx_otag_index);
        x += 8;
    }
#endif
    glyph_rect.x = x;
    glyph_rect.y = y;
    glyph_rect.w = 8;
    glyph_rect.h = 0x10;
    for (j = 0; j < count; j++) {
        menu_gfx_enqueue_textured_quad(&glyph_rect, (digits[j] - '0') * 8 + 0xA8, 0x30, 0, 0,
            g_bunit_menu_icon_texture_page, 0x3ED3, g_menu_gfx_otag_index);
        glyph_rect.x += 8;
    }
    return menu_next_instruction(cmd);
}
