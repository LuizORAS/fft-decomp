#include "fft/battle_formula.h"

/* Formula 0x07, the Healing Staff: XA and YA by weapon type, the zodiac compatibility, Charge's
 * power and the physical XA modifiers, then XA * YA restored as HP, or taken as damage by an undead
 * target (battle_formula_apply_undead_reversal). It cannot miss and triggers no reactions. */
void battle_formula_weapon_heal(void) {
    battle_formula_calculate_base_xa();
#ifdef FIX_BUGS
    /* The edition leaves the compatibility to the physical XA modifiers (QUIRKS.md). */
#else
    battle_formula_apply_zodiac_compatibility();
#endif
    battle_formula_apply_charge();
    battle_formula_apply_physical_xa_modifiers();
    battle_formula_store_xa_times_ya_damage();
    battle_formula_apply_undead_reversal();
}
