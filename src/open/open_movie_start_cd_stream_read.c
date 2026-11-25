#include "fft/open.h"
#include "psx/types.h"

/* Set the read location and start the streaming read (CdRead2 mode 0x1a0),
 * repeating both until the read is accepted. */
void open_movie_start_cd_stream_read(const CdlLOC* location) {
    for (;;) {
        while (CdControl(CdlSetloc, (u8*)location, 0) == 0) { }
#ifdef FIX_BUGS
        /* The edition restarts with CdlModeRT, as the first read starts (QUIRKS.md). */
        if (CdRead2(0x1e0) != 0) {
            break;
        }
#else
        if (CdRead2(0x1a0) != 0) {
            break;
        }
#endif
    }
}
