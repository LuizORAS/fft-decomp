#include "fft/wldcore.h"
#include "psx/types.h"

/* Sound novel opcode 49: stop the selected music track, unload its slot and clear the selection. */
void wldcore_sound_novel_stop_music(void) {
    s16* music = &g_wldcore_sound_novel_record.music_selection;
    wldcore_audio_music_slots_t* state;
    s32* tracks;

    if (*music != 0) {
        state = &g_wldcore_audio_queue.music;
        state->flags &= ~WLDCORE_AUDIO_MUSIC_PLAYING;
        main_sound_stop_current_music();
        main_sound_unload_scenario_mus(*music >> 8);
        /* Indexed one 32-bit value above the table wldcore_sound_change_scenario_music fills. */
        tracks = state->tracks;
#ifdef FIX_BUGS
        /* The edition clears the slot's own track (QUIRKS.md). */
        tracks[(*music >> 8) - 1] = 0;
#else
        tracks[*music >> 8] = 0;
#endif
        g_wldcore_audio_queue.current_slot = 0;
        *music = 0;
    }
    g_wldcore_sound_novel_record.state_flags |= WLDCORE_SOUND_NOVEL_STATE_FETCH;
}
