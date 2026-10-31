#ifndef MAIN_H
#define MAIN_H

/* SCUS_942.21: resident game services and data tables. */

#include "fft/data.h"
#include "fft/disc.h"
#include "fft/gfx.h"
#include "fft/map.h"
#include "fft/menu.h"
#include "fft/menu_script.h"
#include "fft/ram.h"
#include "fft/script.h"
#include "fft/thread.h"
#include "fft/unit.h"
#include "psx/crt.h"
#include "psx/libapi.h"
#include "psx/libcd.h"
#include "psx/libspu.h"
#include "psx/pad.h"
#include "psx/suzuki.h"

/* card */
/* One save-file icon of the CARD screens. */

/* One selectable save-file icon: 16-color CLUT plus 16x16 4bpp image. */
typedef struct card_save_icon {
    palette_t clut; /* 0x00 */
    u8 image[0x80]; /* 0x20 */
} card_save_icon_t;

/* system */
/* The top-level game loop and its soft reset (main_system_store_stack_pointer saves the loop's stack at boot and
 * main_restore_game_loop_stack_pointer puts it back, both hand-written assembly), the game flow states and error
 * codes, the VSync play-time counter and hsync counts, the empty error hooks and the debug output switches. */

/* g_main_system_game_flow_state: what follows when the BATTLE.BIN loop
 * closes a battle or event. */
typedef enum main_game_flow_state {
    MAIN_GAME_FLOW_BATTLE = 0, /* stay in battle_state_run_game_loop for the next event */
    MAIN_GAME_FLOW_WORLD = 1,  /* back to the world map */
    MAIN_GAME_FLOW_RESET = 2,  /* stop display and audio and restart from the opening */
    MAIN_GAME_FLOW_ENDING = 3, /* run the ending, then restart */
} main_game_flow_state_e;

/* Error codes passed to main_system_handle_error, whose report
 * hook is empty in retail. */
typedef enum main_system_error {
    MAIN_SYSTEM_ERROR_HEAP_ALLOC = 1,            /* main_file_alloc_and_load got no block */
    MAIN_SYSTEM_ERROR_FILE_LOAD = 2,             /* a checked file load returned 0 */
    MAIN_SYSTEM_ERROR_NO_UNIT_RUNTIME_SLOT = 6,  /* battle_unit_claim_slot found none free */
    MAIN_SYSTEM_ERROR_SHP_FRAME = 8,             /* frame index above 0x200 or frame pointer -1 */
    MAIN_SYSTEM_ERROR_SEQ_ANIMATION = 9,         /* animation index above 0x100 or sequence pointer -1 */
    MAIN_SYSTEM_ERROR_FRAME_PART_COUNT = 10,     /* more than eight parts in a unit frame */
    MAIN_SYSTEM_ERROR_NO_SPRITESHEET_SLOT = 11,  /* no free or reusable spritesheet slot */
    MAIN_SYSTEM_ERROR_SEQ_OPCODE = 13,           /* unknown sequence opcode, or a queued animation for slot 0 */
    MAIN_SYSTEM_ERROR_SMD_LOAD = 17,             /* no SMD buffer or the SMD load could not start */
    MAIN_SYSTEM_ERROR_DISTORTION_ANIMATION = 23, /* distortion animation id without a handler */
} main_system_error_e;

extern s32 g_main_system_frontend_world_result;
extern u32 g_main_system_game_flow_state; /* main_game_flow_state_e */
extern u32 g_main_system_game_loop_stack_pointer;
extern u8 g_main_system_go_straight_to_battle;
extern int g_main_system_graphics_error;
extern s32 g_main_system_play_time_frames;
extern s32 g_main_system_play_time_hours;
extern s32 g_main_system_play_time_minutes;
extern s32 g_main_system_play_time_seconds;

/* main_system_flags_e bits; cleared by main_save_init_data_tables. */
extern u32 g_main_system_flags;

extern s32 g_main_system_frame_timer;
extern s32 g_main_system_battle_frame_hsync_count;
extern s32 g_main_system_battle_frame_hsync_count_max;
extern s32 g_main_font_print_enabled;
extern s32 g_main_debug_display_enabled;

/* 0x800449ec: empty (nop, jr ra). Every main loop and every blocking wait calls it once per frame. */
void main_system_frame_hook(void);

/* 0x800449f8: a return-only hook. Main and overlay callers supply two 32-bit arguments,
 * but neither their original meaning nor the hook's original name is proven. */
void main_system_report_error_2(s32 category, s32 code);

void main_restore_game_loop_stack_pointer(u32* source);
void main_system_handle_error(int error_code);
void main_system_handle_battle_load_exception(s32 value);
void main_system_report_error(s32 allocator_id, s32 error_code);
void main_system_handle_pointer_exception(int error_code);
void main_system_reset_game(void);
void main_system_run_game_loop(void);
void main_system_stop_display_and_audio_for_game_reset(void);
void main_system_store_stack_pointer(u32* destination);
void main_system_handle_draw_sync_callback(void);
void main_system_handle_vsync_callback(void);

/* boot */
/* Cold boot, the per-session game state reset and the startup logos. */

extern s32 g_main_boot_game_state_initialized;
extern u32 g_main_boot_squaresoft_logo_image_words[];
void main_boot_run_startup(void);
void main_boot_reset_game_state(void);
void main_boot_show_sceap_logo(void);
void main_boot_fade_in_squaresoft_logo(void);
int main_boot_draw_sceap_logo_frame(u32* otag, u32* image);
int main_boot_draw_squaresoft_logo_frame(u32* otag, u32* image);
void main_boot_fade_out_squaresoft_logo(void);

/* overlay */
/* Load OPEN, WLDCORE and WORLD, and BATTLE from fixed disc sectors and run them. */

extern s32 g_main_battle_bin_sector;
void main_overlay_battle_entry_stub(void);
void main_overlay_call_battle_entrypoint(void);
void main_overlay_exec_battle_bin(void);
int main_overlay_exec_open_bin_main_loop(int mode);
void main_overlay_exec_open_bin_ending(void);
int main_overlay_exec_wldcore_and_world_bin(int load_world);
void main_overlay_load_world_bin(s32 mode);

/* Game and music heaps allocate whole pages, tagged by these byte tables. */
enum {
    MAIN_HEAP_PAGE_SHIFT = 11,
    MAIN_HEAP_PAGE_SIZE = 1 << MAIN_HEAP_PAGE_SHIFT,
    MAIN_HEAP_GAME_PAGE_COUNT = 64,
    MAIN_HEAP_SMD_PAGE_COUNT = 16,
};

/* heap */
/* The game heap (64 cells of 2 KB at 0x801df000) and the SMD heap for sound files, as tables of cell tags. */

extern u8* g_main_heap_low_overlay_load_address;
extern u8* g_main_heap_world_overlay_load_address;
extern u8* g_main_heap_high_overlay_load_address;
extern u8 g_main_heap_game_allocator_table[MAIN_HEAP_GAME_PAGE_COUNT];
extern u8 g_main_heap_smd_allocator_table[MAIN_HEAP_SMD_PAGE_COUNT];
extern u8 g_main_heap_smd_base[];
extern u8* g_main_heap_companion_overlay_load_address;
void* main_heap_alloc(u32 size);
u8* main_heap_reserve_at(u8* allocation, u32 size);
s32 main_heap_free(void* allocation);
void main_heap_call_free(void* allocation);
void main_heap_clear_game_allocator_table(void);
void* main_heap_alloc_smd(u32 size);
s32 main_heap_free_smd(void* allocation);
void main_heap_clear_smd_allocator_table(void);

/* util */
/* MAIN's small helpers: the pass/fail roll, the vector setters, and byte and action-record copies and
 * clears. */

s32 main_util_roll_pass_fail(s32 maximum, s32 threshold);
void main_util_set_svector(SVECTOR* vector, s32 x, s32 y, s32 z);
void main_util_set_vector(VECTOR* vector, s32 x, s32 y, s32 z);
void main_util_copy_action_data(const action_command_t* source, action_command_t* destination);

/* file */
/* Disc reads: a descriptor that main_file_poll_load advances once per frame, and the loaders that wait on it. */

/* Loading-display mode passed to the shared file-header builder. */
typedef enum main_file_loading_display_mode {
    MAIN_FILE_LOADING_DISPLAY_OFF = -1,
    MAIN_FILE_LOADING_DISPLAY_INIT = 0,
    MAIN_FILE_LOADING_DISPLAY_PRESERVE = 1,
} main_file_loading_display_mode_e;

/* One poll advances at most one phase; recoverable CD errors restart setup. */
typedef enum main_file_load_state {
    MAIN_FILE_LOAD_STATE_IDLE = 0,
    MAIN_FILE_LOAD_STATE_SET_DOUBLE_SPEED = 1,
    MAIN_FILE_LOAD_STATE_WAIT_AFTER_SET_MODE = 2,
    MAIN_FILE_LOAD_STATE_START_SEEK = 3,
    MAIN_FILE_LOAD_STATE_POLL_SEEK = 4,
    MAIN_FILE_LOAD_STATE_START_READ = 5,
    MAIN_FILE_LOAD_STATE_POLL_READ = 6,
    MAIN_FILE_LOAD_STATE_UPLOAD_SECTOR = 7, /* WLDCORE image stream only */
} main_file_load_state_e;

/* Asynchronous CD read state advanced by main_file_poll_load. */
typedef struct main_file_load_descriptor {
    s32 _unknown_00;
    s32 state; /* MAIN_FILE_LOAD_STATE_* */
    s32 error_count;
    s32 wait_frames;
    s32 sector_count;
    s32 lba;
    s32 sector_index; /* 0x18; sector being read by wldcore_file_poll_vram_image_stream */
    CdlLOC position;
    u8* destination;
} main_file_load_descriptor_t;
typedef char main_file_load_descriptor_size_must_be_36[sizeof(main_file_load_descriptor_t) == 36 ? 1 : -1];

/* L1 + R1 + Select + Start restarts the game. */
enum {
    MAIN_INPUT_RESET_BUTTONS = PSX_PAD_L1 | PSX_PAD_R1 | PSX_PAD_SELECT | PSX_PAD_START,
};

extern main_file_load_descriptor_t g_main_file_cd_state;
extern u32 g_main_file_still_loading;

int main_file_request_read(
    main_file_load_descriptor_t* state, int lba, int sector_count, void* destination, int loading_display_mode);

/* File-header wrapper at 0x80011bd0 forwards main_file_request_read's result. */
s32 main_file_request_read_quiet(main_file_load_descriptor_t* header, s32 sector, s32 sectors, void* destination);

s32 main_file_request_read_bytes(s32 sector, u32 size, void* destination);
void main_file_poll_load(main_file_load_descriptor_t* state);
int main_file_is_still_loading(void);
void main_file_load_data_from_disc(int sector, int sectors, void* destination, int loading_display_mode);
void* main_file_load_to_address(int sector, u32 size, void* destination);
void* main_file_load_to_address_checked(int sector, int size, void* destination);
void* main_file_alloc_and_load(s32 sector, u32 size);
void* main_file_alloc_and_load_checked(int sector, int size);
suzuki_smd_header_t* main_file_alloc_smd_and_load(int sector, u32 size);
void main_file_init_cd(void);
void main_file_cancel_and_pause_cd(main_file_load_descriptor_t* state);
void main_file_reset_cd_subsystems(void);
void main_file_handle_cd_read_callback(u8 event, u8* result);
void main_file_handle_cd_ready_callback(u8 event, u8* result);

/* input */
/* The pad as MAIN reads it each frame: the held, previous, newly pressed and released buttons, the
 * controller copies and stable-frame count, and the auto-repeat timing (the initial delay and two repeat
 * periods). */

extern u8 g_main_input_repeat_initial_delay;
extern u8 g_main_input_repeat_period;
extern u8 g_main_input_secondary_repeat_period;
extern s32 g_main_input_pressed_buttons;
extern s32 g_main_input_previous_buttons;
extern u32 g_main_input_current_buttons;
extern u32 g_main_input_released_buttons;
extern u32 g_main_input_pressed_copy_0;
extern u32 g_main_input_pressed_copy_1;
extern u32 g_main_input_pressed_copy_2;
extern u32 g_main_input_pressed_copy_3;
extern u32 g_main_input_previous_copy_0;
extern u32 g_main_input_previous_copy_1;
extern u32 g_main_input_previous_copy_2;
extern u32 g_main_input_previous_copy_3;
extern u32 g_main_input_stable_frames;
extern u8 g_main_menu_scroll_accel_delay;
extern u8 g_main_menu_scroll_slow_step;
extern u8 g_main_menu_scroll_fast_step;

/* card */
/* The memory card: the BIOS card event handles (main_card_init_events), the slot states and I/O results
 * that the CARD, OPEN and WORLD routines share, and the save image (card_save_buffer_t) with its card
 * header and summary. */

/* Memory-card BIOS event handles opened by main_card_init_events; the CARD,
 * OPEN and WORLD pollers test them by these indices. */
enum {
    MAIN_CARD_EVENT_BIOS_DONE,
    MAIN_CARD_EVENT_BIOS_ERROR,
    MAIN_CARD_EVENT_BIOS_TIMEOUT,
    MAIN_CARD_EVENT_BIOS_NEW_CARD,
    MAIN_CARD_EVENT_HARDWARE_DONE,
    MAIN_CARD_EVENT_HARDWARE_ERROR,
    MAIN_CARD_EVENT_HARDWARE_TIMEOUT,
    MAIN_CARD_EVENT_HARDWARE_NEW_CARD,
    MAIN_CARD_EVENT_COUNT
};

/* The scanner initializes 15 file states, then allocates available card
 * blocks to usable new-save entries. State 0 means a readable save header. */
typedef enum card_save_slot_state {
    CARD_SAVE_SLOT_UNSCANNED = -2,
    CARD_SAVE_SLOT_UNAVAILABLE = -1,
    CARD_SAVE_SLOT_OCCUPIED = 0,
    CARD_SAVE_SLOT_AVAILABLE = 1
} card_save_slot_state_e;

/* The software- and hardware-card pollers return the first event observed in
 * the IOE, ERROR, TIMOUT, NEW order installed by main_card_init_events. */
typedef enum card_io_result {
    CARD_IO_RESULT_PENDING = -1,
    CARD_IO_RESULT_COMPLETE = 0,
    CARD_IO_RESULT_ERROR = 1,
    CARD_IO_RESULT_TIMEOUT = 2,
    CARD_IO_RESULT_NEW_CARD = 3,
} card_io_result_e;

enum { CARD_SAVE_SLOT_COUNT = 15 };

/* g_main_card_save_last_used_slot when no slot was loaded or written. */
enum {
    CARD_SAVE_LAST_USED_SLOT_NONE = 0xff,
};

/* PlayStation memory-card save-file header block (256 bytes), as built by card_save_build_file_header and its WORLD
 * twin world_card_build_save_file_header. */
typedef struct card_save_header {
    u8 magic0;           /* 0x00: 'S' */
    u8 magic1;           /* 0x01: 'C' */
    u8 icon_flags;       /* 0x02: 0x11, one icon frame */
    u8 block_count;      /* 0x03 */
    u8 title[0x40];      /* 0x04: Shift-JIS save-file title */
    u8 reserved[0x1C];   /* 0x44 */
    palette_t icon_clut; /* 0x60 */
    u8 icon_image[0x80]; /* 0x80 */
} card_save_header_t;

/* The fixed leading bytes of card_save_header_t. */
enum {
    CARD_SAVE_HEADER_MAGIC_S = 'S',
    CARD_SAVE_HEADER_MAGIC_C = 'C',
    CARD_SAVE_HEADER_ICON_ONE_FRAME = 0x11,
    CARD_SAVE_HEADER_BLOCK_COUNT_ONE = 1,
};

/* What a save slot's line on the save screen shows: the first 0x18 bytes of a
 * save's data block. g_main_card_save_slot_metadata keeps one per slot. */
typedef struct card_save_summary {
    u8 slot;              /* 0x00; save slot, 0xff while the buffer is incomplete */
    u8 name[0x10];        /* 0x01; save-description leader name, 0xfe-terminated */
    u8 name_terminator;   /* 0x11 */
    u8 job_id;            /* 0x12 */
    u8 level;             /* 0x13 */
    calendar_date_t date; /* 0x14; month/day from script variables 0x2e/0x2f */
    u8 location;          /* 0x16; script variable 0x31, location text index */
    u8 format_version;    /* 0x17; CARD writes 4 and WORLD load requires 4 */
} card_save_summary_t;
typedef char card_save_summary_size_must_be_24[(sizeof(card_save_summary_t) == 24) ? 1 : -1];

/* The 0x1e00-byte memory-card save image, parity-covered per 128-byte block. WORLD holds it in g_world_load_work_buffer
 * and its serializer and loader establish the fields below; CARD fills the same layout from the SCUS globals in
 * card_save_build_buffer through g_card_save_buffer_pointer. */
typedef struct card_save_buffer {
    card_save_header_t header;   /* 0x000 */
    card_save_summary_t summary; /* 0x100 */
    u8 parity_bits[8];           /* 0x118; one parity bit per 128-byte block */
    s32 elapsed_seconds;         /* 0x120 */
    /* 0x124-0x1dbe: filled by world_card_build_save_image and
     * card_save_build_buffer from the SCUS globals named in their bcopy calls. */
    u8 treasure_acquisition_date_bits[0x35];     /* 0x124; 47 packed 9-bit dates and 1 spare bit */
    u8 land_discovery_date_bits[0x12];           /* 0x159; packed 9-bit discovery dates */
    u8 proposition_last_attempt_date_bits[0x6c]; /* 0x16b; packed 9-bit dates */
    u8 proposition_states[0x60];                 /* 0x1d7; low 6 bits are a remaining-day counter */
    u8 _unused_0237[1];                          /* 0x237; serialized and parity-covered */
    u8 saved_records[5][0x38];           /* 0x238; wldcore_sound_novel_record_t: one sound-novel reading state each */
    u32 saved_data_bits[40];             /* 0x350 */
    u32 secondary_saved_data_bits[2];    /* 0x3f0 */
    u8 brave_story_character_ages[0x40]; /* 0x3f8 */
    u8 proposition_count;                /* 0x438 */
    u8 active_propositions[8][9];        /* 0x439 */
    u8 _unused_0481[3];                  /* 0x481; serialized and parity-covered */
    u8 party_records[PARTY_ROSTER_SLOT_COUNT][0xe0]; /* 0x484; party_unit_t records truncated to 0xe0 bytes */
    u8 item_quantities[ITEM_ID_COUNT];               /* 0x1604; g_main_item_quantities */
    u8 poached_item_quantities[ITEM_ID_COUNT];       /* 0x1704; g_main_item_poached_quantities */
    /* 0x1804; first 0x40 bytes are Move-Find-Item flags, second half unknown. */
    u8 item_location_flags[0x80];
    s32 script_variables[0x100]; /* 0x1884 */
    game_options_fields_t
        options;                /* 0x1c84; copied to g_main_game_options by world_card_load_globals_from_save_image */
    u8 _unknown_1c88;           /* 0x1c88; D_80057b1c */
    s8 item_type_order_0[0xc];  /* 0x1c89; signed, -1-terminated */
    u8 _unused_1c95[1];         /* 0x1c95; serialized and parity-covered */
    s8 item_type_order_1[8];    /* 0x1c96; signed, -1-terminated */
    u8 _unused_1c9e[1];         /* 0x1c9e; serialized and parity-covered */
    s8 item_type_order_2[7];    /* 0x1c9f; signed, -1-terminated */
    s8 item_type_order_3[5];    /* 0x1ca6; signed, -1-terminated */
    s8 item_type_order_4[5];    /* 0x1cab; signed, -1-terminated */
    s8 item_type_order_5[7];    /* 0x1cb0; signed, -1-terminated */
    s8 item_type_order_6[5];    /* 0x1cb7; signed, -1-terminated */
    u8 weapon_page_order[0x8a]; /* 0x1cbc */
    u8 helmet_page_order[0x1d]; /* 0x1d46 */
    u8 armor_page_order[0x25];  /* 0x1d63 */
    u8 accessory_page_order[0x21]; /* 0x1d88 */
    u8 item_page_order[0x15];      /* 0x1da9 */
    u8 _unused_1dbe[0x42];         /* 0x1dbe; parity-covered tail */
} card_save_buffer_t;
typedef char
    card_save_buffer_options_offset_must_be_0x1c84[((unsigned long)&((card_save_buffer_t*)0)->options == 0x1c84) ? 1
                                                                                                                 : -1];
typedef char card_save_buffer_size_must_be_7680[(sizeof(card_save_buffer_t) == 7680) ? 1 : -1];

/* Hours, minutes and seconds shown for one memory-card save slot. */
typedef struct card_save_play_time {
    u8 hours;
    u8 minutes;
    u8 seconds;
} card_save_play_time_t;
typedef char card_save_play_time_size_must_be_3[(sizeof(card_save_play_time_t) == 3) ? 1 : -1];

extern s32 g_main_card_bios_events[MAIN_CARD_EVENT_COUNT];
extern u8 g_main_card_bios_events_opened;
extern u8 g_main_card_save_last_used_slot; /* the save slot last loaded or written */
extern u8 g_card_save_selected_slot;
extern s8 g_card_save_slot_file_states[];
extern card_save_summary_t g_main_card_save_slot_metadata[CARD_SAVE_SLOT_COUNT];
extern card_save_play_time_t g_main_card_save_slot_playtimes[CARD_SAVE_SLOT_COUNT];
void main_card_init_events(void);

/* options */
/* The player's Options menu settings, packed in one word (game_options_t in menu.h), and their defaults. */

extern game_options_t g_main_game_options;
extern game_options_t g_main_game_options_defaults;

/* save */
/* The world state a save stores besides the party and the items: proposition progress and dates, treasure
 * and land dates, the sound novels' reading states; and the new-game initializers. */

/* Byte view of the four-byte world-script instruction word at 0x800d4848:
 * opcode byte followed by up to three operand bytes. */
typedef struct wldcore_sound_novel_instruction_bytes {
    u8 opcode;
    u8 operand_0;
    u8 operand_1;
    u8 operand_2;
} wldcore_sound_novel_instruction_bytes_t;
typedef char
    wldcore_sound_novel_instruction_bytes_size_must_be_4[sizeof(wldcore_sound_novel_instruction_bytes_t) == 4 ? 1 : -1];

/* Nine-byte active-proposition records at 0x80057eec. Daily update 0x80074b30
 * advances elapsed_days and sets completion bit 0x04. The assignment at
 * 0x80074a34 stores the current world-map location at +0x04; 0x80074c40
 * filters completed records by that byte. */
typedef struct wldcore_proposition_progress {
    u8 flags;          /* 0x00; wldcore_proposition_progress_flag_e */
    u8 proposition_id; /* 0x01; text 0xd000 | id names it */
    u8 elapsed_days;
    u8 assigned_days;
    u8 location;
    u8 participant_count;      /* 0x05; speaker selection at 0x800795c4 trusts this count */
    u8 participant_indices[3]; /* 0x06; 0x80074a8c passes each index to party lookup before 0x80074ab4 stores it */
} wldcore_proposition_progress_t;
typedef char wldcore_proposition_progress_size_must_be_9[sizeof(wldcore_proposition_progress_t) == 9 ? 1 : -1];

/* Saved-data records at 0x80059594. The exact caller at 0x8008aba8 establishes
 * the 0x38 stride; 0x8008abe4 selectively resets these fields. Field meanings
 * beyond those stores remain provisional. */
struct wldcore_sound_novel_script_file;

/* Saved-data records at 0x80059594. The exact caller at 0x8008aba8 establishes the 0x38 stride; 0x8008abe4 selectively
 * resets these fields. Field meanings beyond those stores remain provisional. */
typedef struct wldcore_sound_novel_record {
    /* 0x00: the loaded sound-novel script, including its section-offset table; MAIN's initializer
     * (main_save_init_data_tables) points all five slots at g_main_save_slot_buffer (0x800d0bbc). */
    struct wldcore_sound_novel_script_file* buffer;
    u32* script_words; /* 0x04 */
    union {
        u32 word;
        wldcore_sound_novel_instruction_bytes_t bytes;
    } instruction; /* 0x08 */
    /* 0x0c-0x37 mirror the live sound-novel state at 0x800d484c. */
    u16 state_flags; /* 0x0c; wldcore_sound_novel_state_flag_e */
    s16 section;     /* 0x0e */
    s16 word_index;  /* 0x10; instruction word index in the section; cleared by wldcore_sound_novel_restore_saved_state
                      */
    s16 counter_delta;        /* 0x12 */
    s16 text_id;              /* 0x14 */
    s16 brightness;           /* 0x16 */
    s16 _unknown_18;          /* 0x18 */
    s16 background_set;       /* 0x1a */
    s16 picture_render_index; /* 0x1c */
    s16 picture_id;           /* 0x1e */
    s16 text_history_0;       /* 0x20; four-entry text history, newest last */
    s16 text_history_1;       /* 0x22 */
    s16 text_history_2;       /* 0x24 */
    s16 text_history_3;       /* 0x26 */
    s16 value_display_mode;   /* 0x28 */
    s16 music_selection;      /* 0x2a; track in the low byte, slot in the high byte */
    s16 sound_mode;           /* 0x2c */
    s16 picture_x;            /* 0x2e */
    s16 picture_y;            /* 0x30 */
    s16 sound_id;             /* 0x32 */
    s32 counter;              /* 0x34 */
} wldcore_sound_novel_record_t;
typedef char wldcore_sound_novel_record_size_must_be_56[sizeof(wldcore_sound_novel_record_t) == 56 ? 1 : -1];

extern u8 g_main_save_proposition_count;
extern s32 g_main_saved_vsync_interval;
extern s32 g_main_saved_weather_variable;

/* RAM bitmaps indexed by 32-bit values; the save serializer copies only their packed data
 * bytes and omits the padding at the end of each allocation. */
extern u32 g_main_proposition_last_attempt_date_bits[27];

extern u32 g_main_treasure_acquisition_date_bits[14];
extern u32 g_main_land_discovery_date_bits[5];
extern u8 g_main_proposition_states[96]; /* wldcore_proposition_state_e */
extern struct wldcore_sound_novel_script_file g_main_save_slot_buffer;
extern u32 g_main_sound_novel_section_bits[40];
extern wldcore_menu_cursor_state_t g_main_saved_list_cursor_state;
extern wldcore_proposition_progress_t g_main_active_propositions[8];
extern wldcore_sound_novel_record_t g_main_sound_novel_records[];
extern u8 D_80057b1c;
extern u8* g_main_brave_story_ages_source;
extern u8 D_800473A7;
extern u8 g_main_brave_story_character_ages[0x40];
void main_save_init_data_tables(void);
void main_save_init_state_and_options(void);

/* unit */
/* Battle unit records (battle_stats_t) built from an ENTD record or a party record: identity, jobs,
 * learned abilities, raw stats and their growth, equipment and its bonuses, R/S/M flags and names;
 * also a new recruit's party record and the casualty count. */

/* Selects rows in the base-data (0x8005e90c) and variance (0x8005e93c)
 * tables; these are not job IDs or gender bit flags. */
typedef enum main_unit_generation_type {
    MAIN_UNIT_TYPE_MALE = 0,
    MAIN_UNIT_TYPE_FEMALE = 1,
    MAIN_UNIT_TYPE_RAMZA = 2,
    MAIN_UNIT_TYPE_MONSTER = 3,
    MAIN_UNIT_TYPE_COUNT = 4
} main_unit_generation_type_e;

/* One row of the generation tables at 0x8005e90c and 0x8005e93c. The
 * records are selected by main_unit_generation_type_e, not by job id. */
typedef struct main_unit_generation_base_data {
    u8 raw_stat_seeds[UNIT_RAW_STAT_COUNT];
    u8 equipment[UNIT_EQUIPMENT_SLOT_COUNT];
} main_unit_generation_base_data_t;
typedef char main_unit_generation_base_data_size_must_be_12[(sizeof(main_unit_generation_base_data_t) == 12) ? 1 : -1];

/* Values accepted by main_status_set_action_state. */
typedef enum main_unit_action_state {
    MAIN_UNIT_ACTION_STATE_NONE = 0,
    MAIN_UNIT_ACTION_STATE_CHARGING = 5,
    MAIN_UNIT_ACTION_STATE_JUMPING = 6,
    MAIN_UNIT_ACTION_STATE_DEFENDING = 7,
    MAIN_UNIT_ACTION_STATE_PERFORMING = 8,
    MAIN_UNIT_ACTION_STATE_KEEP_PERFORMING = 255
} main_unit_action_state_e;

/* Packed highest-order action status: a one-based index, or zero for none. */
enum {
    MAIN_ACTION_STATUS_RESULT_INDEX_MASK = 0x7f,
    MAIN_ACTION_STATUS_RESULT_REMOVAL = 0x80,
    MAIN_ACTION_STATUS_RESULT_SCRIPT_FILTER_EXCLUDED = 0x100,
};

extern main_unit_generation_base_data_t g_main_unit_generation_base_data[MAIN_UNIT_TYPE_COUNT];
extern u8 g_main_unit_generation_raw_stat_variance[MAIN_UNIT_TYPE_COUNT][UNIT_RAW_STAT_COUNT];
extern u32 g_main_unit_name_uses_world_text;
extern unit_deployment_coordinates_t* g_main_unit_deployed_coords;
extern u8 g_main_crystal_learnable_abilities[];
extern unit_deployment_coordinates_t g_main_deployed_unit_map_coordinates[6];
void main_unit_apply_equipment_move_jump_and_name(battle_stats_t* unit);
void main_unit_apply_level_growth(battle_stats_t* unit, s32 level_down);
void main_unit_calculate_abilities(battle_stats_t* unit, entd_unit_t* entd);
void main_unit_calculate_actual_stats(battle_stats_t* unit, s32 mode);
s32 main_unit_calculate_entd_data(battle_stats_t* unit, entd_unit_t* entd);
void main_unit_calculate_entd_equipment(battle_stats_t* unit, entd_unit_t* entd);
void main_unit_calculate_jobs_and_skillsets_from_entd(battle_stats_t* unit, const entd_unit_t* entd);
void main_unit_calculate_move_jump(battle_stats_t* unit, s32 keep_position);
u8 main_unit_calculate_palette_spritesheet(battle_stats_t* unit, u8* palette);

s32 main_unit_calculate_random_equipment(
    battle_stats_t* unit, u8 item_type_flag, u8 weapon_flag_mask, u8 required_item_type);

u16 main_unit_calculate_rsm(battle_stats_t* unit, u16 ability_id, s32 flags, entd_unit_t* entd);
s32 main_unit_calculate_zodiac_symbol(u16 birthday);
s32 main_unit_check_level_up(battle_stats_t* unit);
void main_unit_copy_job_data(battle_stats_t* unit);
void main_unit_copy_job_growths_and_multipliers(battle_stats_t* unit);
void main_unit_copy_last_ability_ct(battle_stats_t* unit);
void main_unit_copy_party_record(battle_stats_t* unit, const party_unit_t* party_unit);
void main_unit_enable_rsm_flags(battle_stats_t* unit);

void main_unit_generate_base_raw_stats(
    unit_raw_stat_bytes_t raw_stats[UNIT_RAW_STAT_COUNT], main_unit_generation_type_e unit_type);

void main_unit_generate_out_of_battle(party_unit_t* party, main_unit_generation_type_e unit_type);
void main_unit_generate_party_base_raw_stats(party_unit_t* party, main_unit_generation_type_e unit_type);
void main_unit_generate_raw_stats(battle_stats_t* unit);
u8 main_unit_get_spritesheet_palette(const battle_stats_t* unit, u8* palette);
s32 main_unit_has_status_in_set(const battle_stats_t* unit, main_status_check_set_e status_set);
void main_unit_increase_casualty_counters(battle_stats_t* unit);
s32 main_unit_init_from_entd(battle_stats_t* unit, entd_unit_t* entd, u32 use_world_text, s32 kind);
s32 main_unit_init_from_entd_party_member(battle_stats_t* unit, entd_unit_t* entd);
s32 main_unit_init_from_party_slot(battle_stats_t* unit, s32 slot, s32 use_world_text);
void main_unit_init_job_levels(const u16* unit_job_jp, u8* unit_job_levels);
void main_unit_init_position_and_rewards(battle_stats_t* unit, entd_unit_t* entd);
void main_unit_init_status_and_rewards(battle_stats_t* unit, s32 clear_rewards);
void main_unit_learn_job_abilities(battle_stats_t* unit, s32 job_id, const entd_unit_t* entd);
void main_unit_refresh_stats_and_statuses(battle_stats_t* unit);
void main_unit_reset_battle_state(battle_stats_t* unit);
s32 main_unit_save_entd_unit_to_party(entd_encounter_t* entd, s32 unit_id, s32 guest_id, s32 initialize_for_battle);
void main_unit_set_equipment_attributes(battle_stats_t* unit, s32 level_up_check);

/* SCUS_942.21 unit equipment/name pipeline, 0x8005ab00 and
 * 0x8005c398..0x8005cbd0. Signatures are shared with the matching definitions. */
void main_unit_set_equipment_stats(battle_stats_t* unit);

void main_unit_set_equippable_items(battle_stats_t* unit);
void main_unit_set_rsm_flag(battle_stats_t* unit, u32 ability_id);
void main_unit_store_character_names(battle_stats_t* unit);
void main_unit_store_monster_equipment(battle_stats_t* unit, const entd_unit_t* entd);
void main_unit_store_ramza_name_birthday_zodiac(battle_stats_t* unit);
void main_unit_update_formation_stats(battle_stats_t* unit);
void main_unit_update_stats_statuses_and_equipment(battle_stats_t* unit, s32 skip_status_check, s32 initializing);

/* party */
/* The persistent roster (g_main_party_units: 16 ordinary slots, then 4 guest slots): new recruits
 * and monster eggs, saving a battle unit into it, the lookups and the highest level. */

extern u8 g_main_party_highest_level;
extern party_unit_t g_main_party_units[PARTY_ROSTER_SLOT_COUNT];
u8 main_party_calculate_highest_level(void);
void main_party_clear_all(void);
s32 main_party_create_monster_egg(s32 monster_job, s32 egg_modifier, s32 egg_color);
s32 main_party_find_free_slot(s32 save_formation, u8* palette);
s32 main_party_find_slot_by_sprite_set(s32 sprite_set);

/* Out-of-battle generation normalizes every value except male, female and
 * monster to Ramza. The raw-stat generator indexes the tables directly. */
s32 main_party_generate_unit(main_unit_generation_type_e unit_type);

party_unit_t* main_party_get_unit(s32 party_index);
void main_party_init_new_game(s32 mode);
u32 main_party_raise_unit_level(party_unit_t* party_unit, s32 level_delta);
void main_party_remove_unit(u32 index);
s32 main_party_save_unit(battle_stats_t* unit, s32 allow_guest);

/* entd */
/* The ENTD files (BATTLE/ENTD1-4.ENT, 128 encounters of 16 unit records each): loading the one that
 * holds an event's encounter, and saving an event's units as the roster. */

extern s32 g_main_entd_current_event_id;
extern entd_file_t* g_main_entd_set;

/* Current encounter, NULL while loading, or (entd_encounter_t *)-1 when
 * no encounter is selected. */
entd_encounter_t* main_entd_get_encounter(void);

void main_entd_init_event_unit_data(int event_id);
int main_entd_open_file(void);

/* job */
/* The job table and the job progression every module shares: job levels from JP, the unlock
 * requirements, the unlocked-job bitset and proposition JP. */

extern job_data_t g_main_job_data[];
extern job_data_t* g_main_job_data_pointer;
extern const u16 g_main_job_level_jp_requirements[UNIT_JOB_LEVEL_MAX];
extern u16 g_main_job_jp_requirements_by_level[];
extern const u8 g_main_job_unlock_requirements[UNIT_CAREER_JOB_COUNT - 1][UNIT_JOB_LEVEL_BYTE_COUNT];
void job_calculate_and_store_unlocks(party_unit_t* party, u8 unlocked_jobs[UNIT_UNLOCKED_JOB_BYTE_COUNT]);
s32 main_job_add_proposition_jp(u32 party_index, s32 jp);
u32 main_job_calculate_level(u16 job_jp);
u32 main_job_calculate_unlocked(const u8* packed_job_levels, u32 gender_flags);
s32 main_job_find_by_skillset(u8 skillset_id);
job_data_t* main_job_get_data_pointer(s32 job_id);
s32 main_job_get_random_unlocked(const battle_stats_t* unit);
void main_job_store_unlock_bitset(u8 destination[UNIT_UNLOCKED_JOB_BYTE_COUNT], u32 unlocked_jobs);

/* ability */
/* The class data of the item, Throw, Jump, Charge, Math Skill and R/S/M abilities, the skillset
 * lists, the crystal pickup record and the lookups every module uses (docs/mechanics/Abilities.md). */

typedef struct main_ability_human_skillset {
    u8 high_id_bits[3];
    u8 ability_ids[HUMAN_SKILLSET_ABILITY_COUNT];
} main_ability_human_skillset_t;

typedef struct main_ability_monster_skillset {
    u8 high_id_bits;
    u8 ability_ids[MONSTER_SKILLSET_ABILITY_COUNT];
} main_ability_monster_skillset_t;

extern ability_secondary_data_t g_main_ability_range_data[];

/* The item_type_e each Throw ability throws, indexed by ability id - ABILITY_ID_THROW_FIRST. */
extern u8 g_main_throw_ability_item_types[];

/* One secondary-data byte per ability from ABILITY_ID_MATH_FIRST on (math,
 * reaction, support, movement). */
extern u8 g_main_math_rsm_ability_data[ABILITY_ID_COUNT - ABILITY_ID_MATH_FIRST];

extern battle_ability_jump_record_t g_main_jump_ability_data[12];
extern main_ability_charge_record_t g_main_charge_ability_data[8];
extern ability_data_t g_main_ability_data[];

/* Indexed by reaction ability id - ABILITY_ID_REACTION_FIRST. */
extern const u8 g_main_reaction_behavior_flags[ABILITY_ID_SUPPORT_FIRST - ABILITY_ID_REACTION_FIRST];

extern main_ability_human_skillset_t g_main_ability_human_skillset_abilities[];
extern main_ability_monster_skillset_t g_main_ability_monster_skillset_abilities[];
extern u16 g_main_ability_temp_list[SKILLSET_ABILITY_LIST_COUNT];
extern main_crystal_pickup_result_t g_main_crystal_pickup_result;
extern u8 g_main_crystal_treasure_item_id;
extern u32 g_main_sound_novel_saved_flags[2];
extern u8 g_main_action_menu_types_by_skillset[0xe0];
extern u8 g_main_calculator_primes[CALCULATOR_PRIME_COUNT];

s32 main_ability_calculate_pointers_and_type(
    s32 id, ability_data_t** out_ability_data, main_ability_secondary_pointer_t* out_secondary_data);

s32 main_ability_get_id_from_skillset(s32 skillset_id, s32 ability_index);
u32 main_ability_get_known(const battle_stats_t* unit, u32 skillset_id);
u16* main_ability_store_skillset_abilities(s32 skillset, s32 flags);

/* item */
/* The inventory (quantities by item id, the fur shop's poached stock, the list orders) and the item
 * tables and lookups every module uses. */

extern u8 g_main_item_quantities[ITEM_ID_COUNT];
extern item_attribute_t g_main_item_attributes[];
extern u8 g_main_item_page_order[0x15];

/* The first 0x40 bytes are Move-Find-Item flags; the second half is not yet
 * identified. Both halves are initialized and serialized as one block. */
extern u8 g_main_item_location_flags[0x80];

extern u8 g_main_item_poached_quantities[ITEM_ID_COUNT];
extern item_secondary_data_t g_main_item_secondary_data[];

/* &g_main_item_primary_data[0].secondary_data_id viewed as a stride-12 byte
 * array. main_unit_set_equipment_stats reads the secondary id through its own
 * %lo(g_main_item_primary_data+4) address while the type_flags test uses a
 * record pointer; reading both through item_data_t folds them onto one base
 * register and misses the byte-exact match. */
extern u8 g_main_item_secondary_data_ids[];

extern item_type_order_tables_t g_main_item_type_order_tables;

/* Views of g_main_item_type_order_tables.order_1..6 for the WORLD save-image
 * copies (world_card_build_save_image, world_card_load_globals_from_save_image),
 * whose code the member spelling changes. */
extern s8 g_main_item_type_order_1[8];

extern u8 g_main_item_type_order_2[7];
extern u8 g_main_item_type_order_3[5];
extern u8 g_main_item_type_order_4[5];
extern u8 g_main_item_type_order_5[7];
extern u8 g_main_item_type_order_6[5];
extern weapon_data_t g_main_item_weapon_data[]; /* 0x80063ab8 */
extern u8 g_main_weapon_page_order[0x8a];
extern u8 g_main_item_ability_item_ids[14];
extern item_poach_t g_main_item_poach_table[JOB_ID_MONSTER_LAST - JOB_ID_MONSTER_FIRST + 1];
extern item_evade_t g_main_item_accessory_data[]; /* 0x80063f58; unit accessory evade slots */
extern armor_data_t g_main_item_helm_armor_secondary_data[];
extern item_data_t g_main_item_primary_data[];
extern item_evade_t g_main_item_shield_data[]; /* 0x80063eb8; unit right/left shield evade slots */
extern u8 g_main_accessory_page_order[0x21];
extern u8 g_main_armor_page_order[0x25];
extern u8 g_main_helmet_page_order[0x1d];
void main_item_init_new_game_inventory(void);
void main_item_init_order_tables(void);
item_data_t* main_item_get_data_pointer(s32 item_id);
s32 main_item_get_total_equipment_quantity(u8 item_id, s32 include_battle);

/* status */
/* Status changes that BATTLE and WORLD share: a unit's current set, the timed status counts and
 * the death counter, the action state, the status check sets and the preview's status pick. */

/* main_status_change_unit, 0x8005e6cc: OR, AND-not, or replacement of
 * inflicted_status, followed by merging status_sets.innate into status_sets.current. */
typedef enum main_status_change {
    MAIN_STATUS_ADD = 0,
    MAIN_STATUS_REMOVE = 1,
    MAIN_STATUS_REPLACE = 2
} main_status_change_e;

/* Bits of g_main_system_flags, the 32-bit control flags of the WLDCORE world-map
 * loop: frame state, what the frame leaves out, requests left by WORLD and
 * OPENING, and the reason the loop ends. */
typedef enum main_system_flags {
    /* wldcore_entrypoint runs frames while set; wldcore_system_init_world_map_state sets it, a level clears it to
       leave. */
    MAIN_SYSTEM_FLAG_RUNNING = 0x00000001,
    /* Draw this frame with the 3D map; wldcore_gfx_draw_world_frame_with_map clears it after drawing. */
    MAIN_SYSTEM_FLAG_DRAW_MAP = 0x00000002,
    /* A level waits for a thread or a fade: wldcore_input_read_pad publishes no pad input. */
    MAIN_SYSTEM_FLAG_INPUT_BLOCKED = 0x00000004,
    /* Set by wldcore_gfx_start_screen_fade, cleared by wldcore_gfx_step_screen_fade one step before the fade ends. */
    MAIN_SYSTEM_FLAG_FADING = 0x00000008,
    /* wldcore_map_project_and_cull_dots hides every map dot. */
    MAIN_SYSTEM_FLAG_HIDE_MAP_DOTS = 0x00000010,
    /* wldcore_map_draw_visible_routes draws no route. */
    MAIN_SYSTEM_FLAG_HIDE_MAP_ROUTES = 0x00000020,
    /* wldcore_entrypoint masks the display for the frame. */
    MAIN_SYSTEM_FLAG_DISPLAY_OFF = 0x00000040,
    /* wldcore_menu_run_world_frame returns without running the menu frame; no code sets it. */
    MAIN_SYSTEM_FLAG_SKIP_MENU_FRAME = 0x00000080,
    /* Set by world_menu_update_brave_story_request: the return level opens the Brave Story screen. */
    MAIN_SYSTEM_FLAG_REQUEST_BRAVE_STORY = 0x00000100,
    /* wldcore_gfx_draw_world_frame does not load the scratch image into the display buffer. */
    MAIN_SYSTEM_FLAG_NO_BACKGROUND_IMAGE = 0x00000200,
    /* wldcore_gfx_dispatch_display_object_list draws nothing and wldcore_map_pulse_dot_colors returns. */
    MAIN_SYSTEM_FLAG_HIDE_DISPLAY_OBJECTS = 0x00000400,
    /* wldcore_gfx_draw_context_value_display draws nothing. */
    MAIN_SYSTEM_FLAG_HIDE_CONTEXT_VALUE = 0x00000800,
    /* wldcore_gfx_draw_calendar_date draws nothing. */
    MAIN_SYSTEM_FLAG_HIDE_CALENDAR_DATE = 0x00001000,
    /* wldcore_map_draw_location_label draws no marker; set while a level moves or covers the map view. */
    MAIN_SYSTEM_FLAG_HIDE_LOCATION_LABEL = 0x00002000,
    /* WORLD's menu chose a map (script variable CURRENT_MAP): the return level restarts the core and travels there. */
    MAIN_SYSTEM_FLAG_REQUEST_DESTINATION = 0x00004000,
    /* wldcore_map_step_route_travel rolled an encounter: wldcore_entrypoint returns 1. */
    MAIN_SYSTEM_FLAG_RANDOM_BATTLE = 0x00008000,
    /* Set after the card menu returns 1: wldcore_entrypoint restarts its loop instead of returning. */
    MAIN_SYSTEM_FLAG_RESTART = 0x00010000,
    /* The unit-panel threads (8, 9 and 12) run: the menu frame draws their windows, not the formation menu. */
    MAIN_SYSTEM_FLAG_UNIT_PANEL_THREADS = 0x00020000,
    /* A story event replays: wldcore_entrypoint returns 2 and the next entry reopens the story-event list. */
    MAIN_SYSTEM_FLAG_STORY_EVENT_REPLAY = 0x00040000,
    /* Set by open_title_update_menu: the world entry opens the tutorial menu, which leaves through RETURN_TO_TITLE. */
    MAIN_SYSTEM_FLAG_TITLE_TUTORIAL = 0x00080000,
    /* Set by world_menu_update_tutorial_categories_request: the return level opens the tutorial menu. */
    MAIN_SYSTEM_FLAG_REQUEST_TUTORIAL = 0x00100000,
    /* A tutorial event plays: wldcore_entrypoint returns 2 with the music kept; the next entry reopens its list. */
    MAIN_SYSTEM_FLAG_TUTORIAL_EVENT = 0x00200000,
    /* wldcore_system_init_world_map_state sets the rumor, path, location and story-event variables; no code sets it. */
    MAIN_SYSTEM_FLAG_UNLOCK_ALL = 0x00400000,
    /* Set by world_menu_update_debug_variable_list_request: the return level opens the variable list. */
    MAIN_SYSTEM_FLAG_REQUEST_VARIABLE_LIST = 0x00800000,
    /* Set by world_menu_update_party_number_panel_request: the return level opens the fixed number panel. */
    MAIN_SYSTEM_FLAG_REQUEST_PARTY_NUMBER_PANEL = 0x01000000,
    /* The location menu chose a deep-dungeon battle: wldcore_entrypoint returns 4. */
    MAIN_SYSTEM_FLAG_DEEP_DUNGEON_BATTLE = 0x02000000,
    /* Set by wldcore_gfx_clear_screen_and_set_map_clip_rect; the return level then recaptures the world frame. */
    MAIN_SYSTEM_FLAG_SCREEN_CLEARED = 0x04000000,
    /* Map dots stop pulsing and wldcore_gfx_draw_animated_display_object stops advancing frames. */
    MAIN_SYSTEM_FLAG_FREEZE_ANIMATION = 0x08000000,
    /* wldcore_entrypoint returns 5: main_system_run_game_loop goes back to OPENING. */
    MAIN_SYSTEM_FLAG_RETURN_TO_TITLE = 0x10000000,
    /* The world entry opens the Brave Story screen, which leaves through RETURN_TO_TITLE; no code sets it. */
    MAIN_SYSTEM_FLAG_TITLE_BRAVE_STORY = 0x20000000,
} main_system_flags_e;

extern status_infliction_data_t g_main_status_infliction_data[];
extern const status_effect_data_t g_main_status_effect_data[BATTLE_STATUS_COUNT];

/* Built status masks, indexed by main_status_check_set_e. */
extern u8 g_main_status_check_sets[MAIN_STATUS_CHECK_SET_COUNT][BATTLE_STATUS_BYTE_COUNT];

void main_status_init_check_data(void);
void main_status_change_unit(battle_stats_t* unit, s32 status_byte_index, u8 status_flag, s32 change_type);
void main_status_init_ct(battle_stats_t* unit);
void main_status_init_unit(battle_stats_t* unit);
void main_status_set_action_state(battle_stats_t* unit, u8 action_state);
s32 main_status_set_ct(battle_stats_t* unit, s32 status_id, s32 removing);
void main_status_store_current(battle_stats_t* unit);
void main_status_update_unit_flags_and_ct(battle_stats_t* unit);
s32 main_status_find_action_highest_order_effect(const battle_action_data_t* action);

/* zodiac */
/* The resident menu art's records: ZODIAC.BIN's sprite frames, its draw context and primitives, the sign
 * day limits and month offsets; EVENT/FRAME.BIN and the four WLDFACE.BIN portrait sheets. */

/* Resident unit-graphics queue, filled and consumed by the BATTLE loader. */
typedef struct main_gfx_unit_graphics_load_descriptor {
    u8 map_x;
    u8 map_y;
    u8 map_level;
    u8 _padding_03; /* aligns facing */
    u16 facing;     /* a quarter turn per unit */
    s16 spritesheet_id;
    u16 palette_id;
    u16 runtime_id;
    battle_stats_t* battle_stats;
    u32 flags; /* nonzero: the unit starts undrawn */
} main_gfx_unit_graphics_load_descriptor_t;
typedef char main_gfx_unit_graphics_load_descriptor_size_must_be_20
    [(sizeof(main_gfx_unit_graphics_load_descriptor_t) == 20) ? 1 : -1];

/* The 0x144-byte queue occupies 0x80049c18..0x80049d5c. */
typedef struct main_gfx_unit_graphics_load_queue {
    s32 count;                                                                            /* 0x00 */
    main_gfx_unit_graphics_load_descriptor_t descriptors[BATTLE_UNIT_RUNTIME_SLOT_COUNT]; /* 0x04 */
} main_gfx_unit_graphics_load_queue_t;
typedef char
    main_gfx_unit_graphics_load_queue_size_must_be_324[(sizeof(main_gfx_unit_graphics_load_queue_t) == 324) ? 1 : -1];

/* EVENT/FRAME.BIN: 256x288 pixels at 4bpp and 22 palettes, sector rounded.
 * The loaders upload rows 32..287 and the two palette blocks. */
typedef struct main_gfx_frame_bin {
    u8 pixels[288][128];       /* 0x0000; two 4bpp pixels per byte */
    clut_t clut;               /* 0x9000; first 16 palettes */
    palette_t palette_tail[6]; /* 0x9200; remaining six palettes */
    u8 _padding_92c0[0x540];   /* 0x92c0; past the end of the file, to the sector boundary */
} main_gfx_frame_bin_t;
typedef char main_gfx_frame_bin_size_must_be_38912[(sizeof(main_gfx_frame_bin_t) == 38912) ? 1 : -1];

extern const u16 g_main_zodiac_day_limits[ZODIAC_SIGN_ORDINARY_COUNT];
extern POLY_FT4 g_main_zodiac_primitives[2][30];
extern u16 g_main_month_start_day_offsets[13];

/* script */
/* The scenario and script data MAIN keeps for every overlay: the script variables and the event block,
 * the scenario conditions and event finish operations, the tutorial progress masks and the replay index. */

extern u16 g_main_scenario_event_finish_operations[];

/* The script variables: 32-bit values first (event_script_variable_id_e), then the packed flags. */
extern s32 g_main_script_variables[0x100];

extern u16 g_main_scenario_condition_offsets[];
extern s16 g_main_scenario_condition_data[];
extern event_file_block_t g_main_script_event_block[];
extern u32 g_main_tutorial_progress_masks[];
extern s32 D_800459D8;
extern s32 g_main_replay_story_event_index;

/* gfx */
/* MAIN's display: the two display and draw environments and the screen polarity, the Now Loading
 * message, the FRAME.BIN image, the zodiac backdrop of the deployment screen, the debug font and BATTLE's
 * ordering tables. */

/* g_main_terrain_status_flags has one byte per map_surface_type_e; only
 * MAP_SURFACE_POISONED_MARSH has a bit. */
enum {
    MAIN_TERRAIN_STATUS_FLAG_POISON = 0x80, /* battle_status_prepare_terrain_poison poisons a unit standing there */
};

/* Rows of g_main_terrain_movement_cost_tables, chosen from the unit's movement
 * abilities by battle_move_store_unit_movement_to_scratchpad; each has one
 * cost per map_surface_type_e. */
typedef enum main_terrain_cost_table {
    MAIN_TERRAIN_COST_TABLE_DEFAULT = 0,
    MAIN_TERRAIN_COST_TABLE_FLY_TELEPORT = 1, /* every surface costs 1 */
    MAIN_TERRAIN_COST_TABLE_FLOAT = 2,        /* every surface costs 1 */
    MAIN_TERRAIN_COST_TABLE_WATER = 3,        /* Move in Water and Walk on Water */
    MAIN_TERRAIN_COST_TABLE_UNDERWATER = 4,
    MAIN_TERRAIN_COST_TABLE_LAVA = 5,
} main_terrain_cost_table_e;

/* Costs in those rows that are not a number of Move points. */
enum {
    MAIN_TERRAIN_COST_WEATHER = 0,    /* replaced by battle_move_get_weather_terrain_cost */
    MAIN_TERRAIN_COST_BLOCKED = 0xff, /* stays so under Any Ground */
};

extern RECT g_main_gfx_frame_image_rect;
extern RECT g_main_gfx_frame_palette_rect;
extern RECT g_main_gfx_frame_palette_tail_rect;
extern palette_t g_main_gfx_now_loading_clut;
extern u32 g_main_gfx_now_loading_frame_counter;
extern u32 g_main_gfx_now_loading_otags[2][2];

/* Seven textured quads for each display buffer. */
extern POLY_FT4 g_main_gfx_now_loading_primitives[2][7];

extern u32 g_main_gfx_now_loading_visible;
extern upoint16_t g_main_gfx_zodiac_screen_position;
extern u8 g_main_gfx_zodiac_sprite_count;
extern sprite_sheet_frame_t g_main_gfx_zodiac_sprite_frames[];
extern point16_t g_main_gfx_zodiac_sprite_scales[];
extern point16_t g_main_gfx_zodiac_texture_position;
extern DISPENV g_main_gfx_display_envs[2];
extern DRAWENV g_main_gfx_draw_envs[2];
extern u32 g_main_gfx_battle_otags[2][0x180];
extern s32 g_main_gfx_loading_display_frame_counter;
extern s32 g_main_gfx_screen_polarity;
extern u8 g_main_common_menu_graphics_loaded;
extern u8 g_main_special_portrait_unit_id;
extern u8 g_main_special_portrait_wldface_id;
void main_gfx_add_now_loading_to_otag(u32* otag);
void main_gfx_build_now_loading(u32 visible, s32 x, s32 y);
void main_gfx_draw_now_loading_message(void);
u32* main_gfx_get_otag(void);
void main_gfx_load_efc_fnt(void);
void main_gfx_load_frame_bin_into_vram(void);
void main_gfx_load_zodiac_frame(void);
void main_gfx_reset_display(int width, int height, int projection, u8 red, u8 green, u8 blue);
void main_gfx_swap_and_clear_otag(void);
int main_gfx_swap_display_area(u32* otag);
void main_gfx_set_display_draw(int width, int height, int projection, u8 red, u8 green, u8 blue);
void main_gfx_set_display_draw_new_game(int width, int height, int projection, u8 red, u8 green, u8 blue);
void main_gfx_build_now_loading_center(u32 visible);
void main_gfx_build_zodiac_primitives(zodiac_draw_context_t* context, POLY_FT4* poly);
void main_gfx_call_build_zodiac_bin(zodiac_draw_context_t* context, POLY_FT4* poly);

void main_gfx_build_zodiac_sprite_polygon(POLY_FT4* poly, point16_t* texture, upoint16_t* base,
    sprite_sheet_frame_t* frame, point16_t* scale, main_zodiac_draw_offset_t* offset);

/* terrain */
/* Data per map surface type (map_surface_type_e): the movement cost rows, the status flags, the tile-info
 * pairs and the Geomancy abilities. */

extern const u8 g_main_terrain_movement_cost_tables[][64];
extern const u8 g_main_terrain_status_flags[64];

/* Two bytes per map_surface_type_e; all 64 pairs are 0x19, 0x19. */
extern u8 g_main_terrain_tile_info_pairs[64][2];

extern const u8 g_main_terrain_geomancy_abilities[];

/* state */
/* The battle state machine: battle_state_run_game_loop runs the handler of g_battle_game_state each frame. */

extern s32 g_main_system_vsync_interval;

/* unit */
/* The battle units: the battle stats of the 21 slots (g_battle_unit_stats) and the display records of
 * the misc unit list (g_battle_units), and what reads and changes them by misc or battle id:
 * positions, facings and mounts, animation requests and reactions, the ENTD load and the deployment,
 * staged statuses for events, the status pictures of the unit panels and the unit view thread. */

extern main_gfx_unit_graphics_load_queue_t g_main_gfx_unit_graphics_load_queue;

/* effect */
/* The ability effect files (EFFECT/E*.BIN) and the engine that plays them: the request and the load
 * (battle_effect_request_ability_effect, battle_effect_update_file_load), the file's section header and the
 * setters that bind its sections, the script records and their interpreter (battle_effect_run_block,
 * g_battle_effect_instructions), the parent and child timelines with their color, sound and camera tracks,
 * and the native callbacks the scripts bind. The work record types are here because battle_effect_record_t embeds
 * them. */

/* Sector and byte length of a resource read through the resident file loader. */
typedef struct main_file_extent {
    s32 sector;
    s32 size;
} main_file_extent_t;
typedef char main_file_extent_size_must_be_8[(sizeof(main_file_extent_t) == 8) ? 1 : -1];

/* sound */
/* MAIN's sound control over the Suzuki driver: sound resources and sound ids, the music slots (two for
 * the scenario's music, five for the tunes) and their loads, the tune that interrupts the music, the
 * music and sound effect ids, the eight SFX channels and their priorities, the background sound, the
 * sound type, volumes and reverb, the wavesets in SPU RAM and the root-counter tick that drives the
 * driver. */

/* Scenario-music playback state at 0x8004d95c (SCUS_942.21). current_music
 * is the Suzuki music record of the playing scenario music:
 * main_sound_stop_current_music hands it straight to
 * suzuki_smd_stop_music, which takes a suzuki_music_t*. */
typedef struct main_sound_music_state {
    s32 current_slot;                 /* 0x00 0x8004d95c slot of the playing scenario music */
    suzuki_music_t* current_music;    /* 0x04 0x8004d960 */
    suzuki_smd_header_t* current_smd; /* 0x08 0x8004d964 SMD data of the playing music */
    s32 volume;                       /* 0x0c 0x8004d968 */
} main_sound_music_state_t;

/* Music slots 1..2 hold the scenario music opened by main_sound_load_music_into_free_slot; slots 3..7 hold the five
 * permanent tunes (MUSIC_45..49.SMD) that main_sound_open_generic_sfx loads. Both arrays store slot n at index n - 1.
 * Evidence: 0x80043de0 indexes the music records at 0x8004d978 + 4t and smd at 0x8004d994 + 4t; 0x80043a90 reads smd at
 * 0x8004d98c + 4*track and the music record 0x1c bytes below it. */
typedef struct main_sound_music_slots {
    s32 tune;                    /* 0x00: pending tune (id | 0x80) or playing tune id */
    s32 loading_slot;            /* 0x04: slot whose SMD load is in flight, or zero */
    suzuki_music_t* music[7];    /* 0x08: music records returned by suzuki_smd_create_music */
    suzuki_smd_header_t* smd[7]; /* 0x24: loaded SMD data */
} main_sound_music_slots_t;

/* The music state and slot table are one object: main_sound_open_generic_sfx
 * derives the slot pointers from the &state.current_smd base register, and the
 * tune/slot helpers reach across tune, handles and smd from single bases. */
typedef struct main_sound_music {
    main_sound_music_state_t state; /* 0x00 0x8004d95c */
    main_sound_music_slots_t slots; /* 0x10 0x8004d96c */
} main_sound_music_t;

/* Music ids: the index of a MUSIC_NN.SMD file in
 * g_main_sound_scenario_smd_files (MAIN_MUSIC_COUNT entries). Only the ids
 * the code passes by number are named, after the screen that plays them;
 * event scripts and scenario data choose the others. The music test offers
 * ids 1-96 (g_open_music_test_music_ids). */
typedef enum main_music {
    MAIN_MUSIC_NONE = 0,       /* the *_sound_play_music_tracks functions and the WLDCORE audio queue skip it */
    MAIN_MUSIC_ALTIMA = 19,    /* EVENT_CALL_FUNCTION_PLAY_ALTIMA_MUSIC */
    MAIN_MUSIC_MILESTONE = 20, /* wldcore_proposition_step_milestone_level, in the bar */
    MAIN_MUSIC_TUTORIAL = 26,  /* the world menu's tutorial screen */
    MAIN_MUSIC_WORLD_MAP = 27,
    MAIN_MUSIC_SHOP = 28,                         /* world_shop_run_screen: the item shop */
    MAIN_MUSIC_SOLDIER_OFFICE = 29,               /* world_shop_run_screen */
    MAIN_MUSIC_FUR_SHOP = 30,                     /* world_shop_run_screen */
    MAIN_MUSIC_FORMATION = 31,                    /* world_formation_run_screen */
    MAIN_MUSIC_BRAVE_STORY = 32,                  /* the Brave Story screens */
    MAIN_MUSIC_BAR = 33,                          /* the bar of a town */
    MAIN_MUSIC_NEW_GAME = 34,                     /* name and birthday entry of a new game */
    MAIN_MUSIC_PROPOSITION_REPORT = 35,           /* a finished proposition's report */
    MAIN_MUSIC_SAVE = 41,                         /* the memory card screens */
    MAIN_MUSIC_DEPLOYMENT = 42,                   /* the ATTACK unit placement screen */
    MAIN_MUSIC_CONGRATULATIONS = 43,              /* require_text_show_battle_congratulations */
    MAIN_MUSIC_CONGRATULATIONS_FINAL_BATTLE = 44, /* the same screen after event 0x145 */
    MAIN_MUSIC_GAME_OVER = 50,                    /* etc_graphic_show_game_over */
    MAIN_MUSIC_COUNT = 100,
} main_music_e;

/* Tunes of main_sound_play_tune: short pieces that fade the scenario music
 * out, play once and fade it back in (main_sound_update_tunes). Tune t is
 * MUSIC_(44 + t).SMD, kept loaded in music slot 2 + t. Tune 4 has no caller
 * in the code; EVENT_OPCODE_PLAY_TUNE passes its operand. */
typedef enum main_sound_tune {
    MAIN_SOUND_TUNE_NONE = 0,
    MAIN_SOUND_TUNE_LEVEL_UP = 1,
    MAIN_SOUND_TUNE_ITEM_FOUND = 2, /* an item found by moving, a poach, a trained monster */
    MAIN_SOUND_TUNE_JOB_LEVEL_UP = 3,
    MAIN_SOUND_TUNE_JOB_CHANGE = 5,
} main_sound_tune_e;

/* Packed tune-slot id and pending flag. */
enum {
    MAIN_SOUND_TUNE_ID_MASK = 0x3f,
    MAIN_SOUND_TUNE_PENDING = 0x80,
};

/* Sound resources. A sound id is MAIN_SUZUKI_SOUND_ID(resource, sound): the id of a
 * loaded suzuki_sound_resource_t in the high 16 bits and the index of a
 * sound in it in the low. Each battle effect file brings a resource of its
 * own (battle_effect_load_sound_section). */
typedef enum main_sound_resource_id {
    MAIN_SOUND_RESOURCE_SYSTEM = 0,        /* SOUND/SYSTEM.SED: main_sfx_e */
    MAIN_SOUND_RESOURCE_ENV = 1,           /* SOUND/ENV.SED: main_env_sfx_e */
    MAIN_SOUND_RESOURCE_SOUND_NOVEL = 225, /* g_wldcore_sound_novel_sound_resource, 81 sounds */
} main_sound_resource_id_e;

/* Sound-effect ids passed to main_sound_play_sfx and the module queues that
 * feed it (wldcore_sound_play_effect, world_sound_dispatch_effect and the
 * g_*_sound_*effect_id globals); every path reaches suzuki_sound_start_sfx with
 * MAIN_SOUND_RESOURCE_SYSTEM in the high 16 bits, so this is one id space.
 * SYSTEM.SED holds MAIN_SFX_COUNT sounds; the ones named here are named
 * from their users in the code, the others are played only by event and
 * animation script data. The event frames store -1 in the pending globals
 * for "no request"; the dispatchers turn a negative id into MAIN_SFX_NONE. */
typedef enum main_sfx {
    /* Nothing queued. SYSTEM.SED entry 0 has no streams, so playing it stops
     * the channel pair main_sound_play_sfx uses;
     * wldcore_menu_step_map_path_level plays it when the path animation ends. */
    MAIN_SFX_NONE = 0,
    MAIN_SFX_CONFIRM = 1,
    MAIN_SFX_CANCEL = 2,
    MAIN_SFX_CURSOR_MOVE = 3,
    MAIN_SFX_GIL_TICKER = 4, /* looping count-up of the REQUIRE gil reward */
    MAIN_SFX_INVALID = 5,
    MAIN_SFX_PAGE_SWITCH = 6, /* L1/R1 unit and page switches */
    MAIN_SFX_UNEQUIP = 7,     /* remove equipment/abilities */
    MAIN_SFX_RED_ORBS = 8,    /* battle_effect_red_orbs_secondary_handler starts */
    MAIN_SFX_DISMISS = 9,     /* dismiss unit */
    MAIN_SFX_EQUIP = 10,      /* equip, set ability, best fit */
    MAIN_SFX_CAMERA_ROTATE_LEFT = 11,
    MAIN_SFX_CAMERA_ROTATE_RIGHT = 12,
    MAIN_SFX_CAMERA_ROTATE_STOP = 13, /* the rotation reached its target */
    /* Paired zoom sounds: battle_camera_toggle_zoom and battle_camera_start_zoom
     * play 0x11 with zoom action 4 and 0xe with action 2; the in/out reading
     * follows the zoom-target comment in battle_camera_toggle_zoom. The
     * ATTACK deploy screen also plays 0x11 when a placed unit is moved. */
    MAIN_SFX_CAMERA_ZOOM_IN = 14,
    /* L1/R1 on the world map (wldcore_menu_step_map_cursor_level): 0x10 when
     * the toggle sets the zoomed-out bit of g_wldcore_map_zoom_motion, 0xf
     * when it clears it. */
    MAIN_SFX_WORLD_MAP_ZOOM_IN = 15,
    MAIN_SFX_WORLD_MAP_ZOOM_OUT = 16,
    MAIN_SFX_CAMERA_ZOOM_OUT = 17,
    MAIN_SFX_WINDOW_OPEN = 18, /* help, message or text window opens */
    /* The weapon sounds of battle_sound_play_weapon_sfx, by the weapon's
     * item_type_e: g_battle_sound_weapon_swing_sfx_ids (the swing, played
     * before the hit is known), g_battle_sound_weapon_hit_sfx_ids and
     * g_battle_sound_shield_deflect_sfx_ids. */
    MAIN_SFX_HIT_BLADE = 19,    /* knife, ninja blade, sword, knight sword, katana */
    MAIN_SFX_HIT_HEAVY = 21,    /* axe, flail; also the deflect of a fist, bag or cloth */
    MAIN_SFX_HIT_UNARMED = 24,  /* fist, bag, cloth */
    MAIN_SFX_SWING_LIGHT = 25,  /* fist, knife, ninja blade, instrument, book, bag, cloth */
    MAIN_SFX_SWING_MEDIUM = 26, /* sword, knight sword, katana, rod, staff */
    MAIN_SFX_SWING_HEAVY = 27,  /* axe, flail, polearm, pole */
    /* Started by the spell-charge and summon-charge secondary effects, which
     * stop them with suzuki_sound_stop_sfx_channels when they finish. */
    MAIN_SFX_SPELL_CHARGE = 31,
    MAIN_SFX_SUMMON_CHARGE = 33,
    MAIN_SFX_VENOM_TRAP = 34, /* battle_effect_trap_secondary_handler starts */
    /* battle_sound_play_landing_sfx_by_surface_type: water surfaces play
     * 0x23, grassland, thicket and rug 0x28, wooden floor, tree and box 0x3a,
     * every other surface 0x29. A step that ends on another unit also plays
     * 0x28, and mounting a unit named by battle id plays 0x29. */
    MAIN_SFX_LAND_WATER = 35,
    MAIN_SFX_JUMP = 39, /* a jump rise starts; the fall that ends a mount ride starts */
    MAIN_SFX_LAND_SOFT = 40,
    MAIN_SFX_LAND_DEFAULT = 41,
    MAIN_SFX_TEXT_PAGE = 45,            /* dialogue/scroll window advances a page */
    MAIN_SFX_SHIELD_DEFLECT_LIGHT = 46, /* blades, rod, staff, polearm, pole */
    MAIN_SFX_SHIELD_DEFLECT_HEAVY = 47, /* axe, flail */
    /* An error message opens: a memory-card load, save or scan failure, or a
     * refused equip, dismissal, job change or ability.
     * battle_sound_play_weapon_sfx plays it for an attack stopped by an
     * accessory or class evade, a forced failure, a blade grasp or a catch,
     * and as the shield deflect of an instrument or book. */
    MAIN_SFX_ERROR_MESSAGE = 48,
    MAIN_SFX_CAMERA_TILT = 49, /* battle_camera_start_tilt */
    MAIN_SFX_HIT_ROD = 56,     /* rod, staff */
    MAIN_SFX_LAND_WOOD = 58,
    /* Circle on a name-entry character key, or on an occupied tile of the
     * ATTACK deploy screen. */
    MAIN_SFX_SELECT = 60,
    MAIN_SFX_HIT_POLE = 62,         /* polearm, pole */
    MAIN_SFX_EVENT_REVIVE = 65,     /* the event script's revive status change (*_script_inflict_status_thread) */
    MAIN_SFX_WORLD_MAP_ARRIVE = 67, /* a route leg ends at a location with no event or encounter */
    /* Dead status added outside an event
     * (battle_unit_update_animation_for_status_changes), chosen by the unit's
     * male, female or monster flag; the male sound is 0x9a. */
    MAIN_SFX_DEATH_FEMALE = 69,
    MAIN_SFX_DEATH_MONSTER = 70,
    MAIN_SFX_SHIELD_DEFLECT_BULLET = 71, /* gun */
    MAIN_SFX_SHIELD_DEFLECT_ARROW = 73,  /* crossbow, bow */
    /* Proposition result (g_wldcore_proposition_outcome.result): 0 plays the
     * 0x4d/0x4e pair through suzuki_sound_play_2_sfx, 1 the 0x4f/0x50 pair,
     * 2 plays 0x51. */
    MAIN_SFX_PROPOSITION_SUCCESS_A = 77,
    MAIN_SFX_PROPOSITION_SUCCESS_B = 78,
    MAIN_SFX_PROPOSITION_PARTIAL_A = 79,
    MAIN_SFX_PROPOSITION_PARTIAL_B = 80,
    MAIN_SFX_PROPOSITION_FAILURE = 81,
    MAIN_SFX_SWING_BOW = 82,
    MAIN_SFX_SWING_CROSSBOW = 86,
    MAIN_SFX_SWING_GUN = 90,
    MAIN_SFX_HIT_GUN = 91,
    MAIN_SFX_TELEPORT_OUT = 106,
    MAIN_SFX_TELEPORT_IN = 107,
    /* Animation step 4 of the world event-transition indicator
     * (wldcore_menu_step_event_transition_level): 0x6c in mode 0, which a
     * random encounter pushes, 0x6d in the other spinning modes. */
    MAIN_SFX_EVENT_TRANSITION_MODE_0 = 108,
    MAIN_SFX_EVENT_TRANSITION_MODE_1 = 109,
    MAIN_SFX_STATUS_MENU_REQUEST
    = 113,                       /* Triangle queues the status menu on another unit's turn; again when it opens */
    MAIN_SFX_CRITICAL_HIT = 114, /* a weapon hit whose action is critical (battle_sound_play_weapon_sfx) */
    MAIN_SFX_TEXT_GLYPH = 115,   /* typewriter tick after a dialogue glyph */
    MAIN_SFX_HIT_ARROW = 116,    /* crossbow, bow */
    /* World map: 0x76 when a path animation starts, drawing or erasing; 0x77
     * when a hidden location is made visible. */
    MAIN_SFX_WORLD_MAP_PATH = 118,
    MAIN_SFX_WORLD_MAP_LOCATION_APPEAR = 119,
    /* A save or load finished, Start pressed in the opening's new-game
     * transition or to end deployment, a crystal or treasure picked up, or a
     * proposition report showing its reward picture. */
    MAIN_SFX_COMPLETE = 133,
    MAIN_SFX_SHOP_TRANSACTION = 151, /* shop buy/sell confirmed */
    MAIN_SFX_ABILITY_LEARNED = 152,  /* an ability bought in the learn list */
    MAIN_SFX_DEATH_MALE = 154,
    MAIN_SFX_HIT_BOOK = 156, /* instrument, book */
    MAIN_SFX_COUNT = 168,    /* the sound count in the SYSTEM.SED header */
} main_sfx_e;

/* Sounds of MAIN_SOUND_RESOURCE_ENV that the code plays by number. ENV.SED
 * holds 27 (0, 0x15, 0x16 and 0x17 are empty); event scripts choose the
 * others through EVENT_OPCODE_BG_SOUND. */
typedef enum main_env_sfx {
    MAIN_ENV_SFX_RAIN = 2,      /* battle_map_run_weather_command: rain without snow, and the thunder command */
    MAIN_ENV_SFX_THUNDER_A = 3, /* battle_map_draw_mesh_and_weather picks one of the two for a lightning flash */
    MAIN_ENV_SFX_THUNDER_B = 14,
} main_env_sfx_e;

extern u8 g_main_sound_env_sed_data[];
extern u8 g_main_sound_system_sed_data[];
extern main_sound_music_t g_main_sound_music;
extern main_file_extent_t g_main_sound_scenario_smd_files[];
extern main_file_extent_t g_main_sound_permanent_smd_files[5];
extern s32 g_main_sound_weather_sfx_id; /* the background sound main_sound_play_bg_sfx started, 0 for none */
void main_sound_set_sfx_echo(int sound_id, int volume);
void main_sound_stop_weather_sfx_music(void);
void main_sound_stop_current_weather_sfx(void);
s32 main_sound_get_current_scenario_music(void);
suzuki_music_t* main_sound_get_music_handle(s32 slot);

/* Verified SCUS scenario-music and tune wrappers. Include this header in
 * their definitions as well as callers so ABI declarations cannot drift. */
s32 main_sound_load_music_into_slot(s32 music_id, s32 slot);

void main_sound_open_generic_sfx(void);
s32 main_sound_load_music_into_free_slot(s32 music_id);
s32 main_sound_start_music_load_into_slot(s32 music_id, s32 slot);
s32 main_sound_start_music_load_into_free_slot(s32 music_id);
void main_sound_play_sfx(s32 sound_id);
void main_sound_play_bg_sfx(s32 sound_id);
void main_sound_play_sfx_find_channel(s32 sound_id);
s32 main_sound_play_tune(s32 tune);
s32 main_sound_poll_music_load(void);
s32 main_sound_set_current_music_balance_shift_timed(s32 value, s32 time);
s32 main_sound_set_current_music_pitch_shift_timed(s32 value, s32 time);
s32 main_sound_set_current_music_target(s32 volume, s32 time);
s32 main_sound_set_current_music_tempo_scale_timed(s32 value, s32 time);
void main_sound_set_current_music_track(s32 music_id);
s32 main_sound_set_current_music_volume(s32 volume, s32 time);
s32 main_sound_set_current_music_volume_timed(s32 time);
s32 main_sound_set_tune_volume(s32 time);
s32 main_sound_stop_current_music(void);

/* battle_map_step_init_sequence calls this without loading $a0, but the body reads sound_id. */
void main_sound_stop_sfx(int sound_id);

suzuki_music_t* main_sound_switch_music_track(s32 slot, s32 volume, s32 fade);
void main_sound_unload_current_scenario_music(void);
s32 main_sound_unload_scenario_mus(s32 slot);
void main_sound_unload_scenario_music_and_stop_tune(void);
s32 main_sound_update_tunes(void);
s32 main_sound_wait_for_music_load(s32 music_id);
void main_sound_replay_bg_sfx(void);
s32 main_sound_is_music_loading_stub(void);

/* action */
/* The action pipeline: the ability and charge time a command resolves to, the action record, each
 * strike's targets and formula setup, the result applied to each unit, the rewards and the effect
 * messages. */

extern entd_unit_t* g_main_entd_current_unit;

/* job */
/* The job list of BUNIT's unit screen: a unit's selectable jobs and the numbers its rows show (job
 * level, JP, total JP, JP to the next level, mastered), which the job list caches for the cursor job
 * and the ability list reads back. */

s32 job_find_first_for_skillset(s32 skillset);
s32 job_get_generic_index(s32 job_id);
s32 job_get_skillset(s32 job_id);
s32 job_is_special_monster(s32 job_id);

/* card */
/* The memory card: the selected slot, the BIOS and hardware card events, and _card_info, _card_load and
 * _card_clear with retries. */

void card_io_consume_all_events(void);
void card_io_consume_bios_events(void);
void card_io_consume_hardware_events(void);
s32 card_io_poll_bios_event_result(void);
s32 card_io_poll_hardware_event_result(void);

/* declarations only build-2 has (merge dry run) */
/* Second link name of main_item_get_data_pointer with no parameter: battle_gfx_set_thrown_item_graphic_palette
 * calls it with the item id still in $a0 (QUIRKS.md). */
u8* main_item_get_data_pointer_no_argument(void);

#endif
