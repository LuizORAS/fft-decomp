#ifndef BATTLE_AI_H
#define BATTLE_AI_H

/* Battle AI workspace, ability evaluation and action selection. */

#include "fft/main.h"

/* ai */
/* The AI that plays computer, guest and Auto-Battle units: its workspace, the per-unit ability lists
 * and decision records, the movement scenarios and tile grids, the ability search, and the
 * simulation, scoring and ranking that choose the turn's command (docs/mechanics/AI.md). */

typedef enum battle_ai_command_kind {
    BATTLE_AI_COMMAND_MOVE = 0,
    BATTLE_AI_COMMAND_ACT = 1,
    BATTLE_AI_COMMAND_END_TURN = 2,
    /* The rest reach the command from battle_menu_resolve_selection, for a player's menu choice.
     * The auto-battle choices are 12 plus twice the option, plus one once a unit has been
     * picked for the two that name one (SELECT picks it, DISPLAY shows it); SELECT and the two
     * others equal their battle_auto_battle_setting_e values. */
    BATTLE_AI_COMMAND_NO_MENU_RECORD = 7, /* the current menu id has no record: back to the turn events */
    BATTLE_AI_COMMAND_AUTOBATTLE_TARGETED_SELECT = 12,
    BATTLE_AI_COMMAND_AUTOBATTLE_TARGETED_DISPLAY = 13,
    BATTLE_AI_COMMAND_AUTOBATTLE_RECOVERY_SELECT = 14,
    BATTLE_AI_COMMAND_AUTOBATTLE_RECOVERY_DISPLAY = 15,
    BATTLE_AI_COMMAND_AUTOBATTLE_FADING_LIFE = 16,
    BATTLE_AI_COMMAND_AUTOBATTLE_RETREAT = 17,
    BATTLE_AI_COMMAND_REFRESH_CONTROL = 18, /* copy the control flag back from the stats */
    BATTLE_AI_COMMAND_MOVE_SELF = 25,       /* MOVE that never redirects to the unit's mount */
    BATTLE_AI_COMMAND_UNRESOLVED_FE = 0xfe,
    BATTLE_AI_COMMAND_UNRESOLVED_FF = 0xff,
} battle_ai_command_kind_e;

/* Baseline records include action/equipment data; simulated status uses the compact records. */
typedef enum battle_ai_snapshot_mode {
    BATTLE_AI_SNAPSHOT_BASELINE = 0,
    BATTLE_AI_SNAPSHOT_SIMULATED_STATUS = 1,
} battle_ai_snapshot_mode_e;

typedef enum battle_ai_simulation_phase {
    BATTLE_AI_SIMULATION_PREPARE_ACTION = 0,
    BATTLE_AI_SIMULATION_ADVANCE_TURNS = 1,
    BATTLE_AI_SIMULATION_SCORE_STATE = 2,
} battle_ai_simulation_phase_e;

/* The four AI flag bytes of an ability, one enum per byte. */
enum {
    BATTLE_AI_ABILITY_FLAG_1_TARGET_ALLIES = 0x01,
    BATTLE_AI_ABILITY_FLAG_1_TARGET_ENEMIES = 0x02,
    BATTLE_AI_ABILITY_FLAG_1_AFFECTS_HP = 0x80,
};

enum {
    BATTLE_AI_ABILITY_FLAG_2_EVADEABLE = 0x02,
    BATTLE_AI_ABILITY_FLAG_2_AFFECTED_BY_FAITH = 0x04,
    BATTLE_AI_ABILITY_FLAG_2_UNDEAD_REVERSE = 0x20,
    BATTLE_AI_ABILITY_FLAG_2_TARGET_MAP_TILES = 0x80,
};

enum {
    BATTLE_AI_ABILITY_FLAG_3_PHYSICAL_ATTACK = 0x01,
    BATTLE_AI_ABILITY_FLAG_3_MAGICAL_ATTACK = 0x02,
    /* Four area-shape bits, tested only together: 0x20 a line from the user
     * (BATTLE_AI_ABILITY_LINEAR_TRAJECTORY in the 32-bit view), 0x10 the eight
     * breath and bracelet cones, 0x08 Triple Attack alone, 0x04 Triple Bracelet
     * alone. With any of them set the AI tries every origin tile. */
    BATTLE_AI_ABILITY_FLAG_3_EXHAUSTIVE_ORIGIN_MASK = 0x3c,
    BATTLE_AI_ABILITY_FLAG_3_DIRECT_TRAJECTORY = 0x40,
    BATTLE_AI_ABILITY_FLAG_3_ARC_TRAJECTORY = 0x80,
};

enum {
    BATTLE_AI_ABILITY_FLAG_4_EVADE_WITH_MOTION = 0x01, /* the static table's bit; the AI working copy reuses byte 4 */
    BATTLE_AI_ABILITY_FLAG_4_USABLE_BY_AI = 0x80,
};

/* 32-bit flags of the considered ability. Bytes 1..3 retain the static table
 * meanings, but byte 4 is replaced by battle_ai_ability_entry_flag_e: its
 * low bit means CONSUME_TARGET_TILES, not the static EVADE_WITH_MOTION.
 * Team preferences remain distinct from the absolute ONLY restrictions. */
enum {
    BATTLE_AI_ABILITY_TARGET_ALLIES = 0x00000001,
    BATTLE_AI_ABILITY_TARGET_ENEMIES = 0x00000002,
    BATTLE_AI_ABILITY_UNEQUIP = 0x00000004,
    BATTLE_AI_ABILITY_AFFECTS_STATS = 0x00000008,
    BATTLE_AI_ABILITY_ADD_STATUS = 0x00000010,
    BATTLE_AI_ABILITY_CANCEL_STATUS = 0x00000020,
    BATTLE_AI_ABILITY_AFFECTS_MP = 0x00000040,
    BATTLE_AI_ABILITY_AFFECTS_HP = 0x00000080,
    BATTLE_AI_ABILITY_AFFECTED_BY_SILENCE = 0x00000100,
    BATTLE_AI_ABILITY_EVADEABLE = 0x00000200,
    BATTLE_AI_ABILITY_AFFECTED_BY_FAITH = 0x00000400,
    BATTLE_AI_ABILITY_RANDOM_USE = 0x00000800,
    BATTLE_AI_ABILITY_FOLLOW_TARGET = 0x00001000,
    BATTLE_AI_ABILITY_UNDEAD_REVERSE = 0x00002000,
    BATTLE_AI_ABILITY_REFLECTABLE = 0x00004000,
    BATTLE_AI_ABILITY_TARGET_MAP_TILES = 0x00008000,
    BATTLE_AI_ABILITY_PHYSICAL_ATTACK = 0x00010000,
    BATTLE_AI_ABILITY_MAGICAL_ATTACK = 0x00020000,
    BATTLE_AI_ABILITY_LINEAR_TRAJECTORY = 0x00200000,
    BATTLE_AI_ABILITY_DIRECT_TRAJECTORY = 0x00400000,
    BATTLE_AI_ABILITY_ARC_TRAJECTORY = 0x00800000,
    BATTLE_AI_ABILITY_CONSUME_TARGET_TILES = 0x01000000,
    BATTLE_AI_ABILITY_EXHAUSTIVE_ORIGIN_SCAN = 0x04000000,
    BATTLE_AI_ABILITY_REQUIRES_MONSTER_SKILL = 0x08000000,
    BATTLE_AI_ABILITY_ENEMIES_ONLY = 0x20000000,
    BATTLE_AI_ABILITY_ALLIES_ONLY = 0x40000000,
    BATTLE_AI_ABILITY_USABLE_BY_AI = 0x80000000,
};

/* The setup dispatcher writes this at runtime +0x158 and copies 24 bytes into
 * each history slot. ACT copies the 20-byte payload from AI action +0x08;
 * MOVE writes only x/layer/y, and END_TURN leaves the payload unchanged. */
typedef struct battle_ai_command {
    battle_ai_command_kind_e kind;
    union {
        map_tile_coordinates_t move;
        action_command_t action;
    } data;
} battle_ai_command_t;
typedef char battle_ai_command_size_must_be_24[sizeof(battle_ai_command_t) == 24 ? 1 : -1];

/* X / map elevation / Y / zero, written in that order by battle_ai_transfer_unit_coordinates
 * and read as one 32-bit value for equality tests. */
typedef union battle_ai_coords {
    u32 word;
    struct {
        u8 x;
        u8 elevation;
        u8 y;
        u8 zero;
    } fields;
} battle_ai_coords_t;

/* Ability AI behavior flags 1..4. Consumers read the 32-bit value, two 16-bit values,
 * or individual bytes; use the corresponding masks above for each view. */
typedef union battle_ai_ability_flags {
    u32 word;
    ability_ai_flags_t fields;
    struct {
        u16 target_effect_flags;    /* bytes 1..2 */
        u16 trajectory_usage_flags; /* bytes 3..4 */
    } groups;
} battle_ai_ability_flags_t;

/* Shared prefix of the baseline and suspended AI unit snapshots.
 * Byte 2 holds Death Sentence CT only in the compact suspended record;
 * the baseline uses status_ct[15] and leaves this byte untouched. */
typedef struct battle_ai_status_snapshot {
    u8 entd_slot;
    u8 death_counter;
    u8 death_sentence_ct; /* see 0x8019e214 */
    u8 current_status[BATTLE_STATUS_BYTE_COUNT];
} battle_ai_status_snapshot_t;
typedef char battle_ai_status_snapshot_must_be_8[sizeof(battle_ai_status_snapshot_t) == 8 ? 1 : -1];

/* Selected unit state saved at 0x8019e160 and restored at 0x8019e378.
 * HP occupies unaligned bytes 0x1b..0x1c; max HP/MP are not saved. */
typedef struct battle_ai_extended_snapshot {
    battle_ai_status_snapshot_t status;       /* 0x00 */
    u8 status_ct[BATTLE_TIMED_STATUS_COUNT];  /* 0x08 */
    u8 team_flags;                            /* 0x18 */
    u8 faith;                                 /* 0x19 */
    u8 transparent_removal_flag;              /* 0x1a */
    u8 hp_bytes[2];                           /* 0x1b */
    u8 _padding_1d;                           /* aligns mp */
    u16 mp;                                   /* 0x1e */
    u8 charged_ability_ct;                    /* 0x20 */
    u8 base_attributes[UNIT_ATTRIBUTE_COUNT]; /* 0x21; unit_attribute_index_e */
    u8 attributes[UNIT_ATTRIBUTE_COUNT];      /* 0x24; unit_attribute_index_e */
    u8 ct;                                    /* 0x27 */
    u8 x;                                     /* 0x28 */
    u8 position_bytes[2];                     /* 0x29; packed unit position */
    u8 has_turn;                              /* 0x2b */
    u8 movement_taken;
    u8 action_taken;
    u8 auto_battle_setting; /* 0x2e */
    u8 auto_battle_target;
    u8 initial_team_flags;
    u8 inflicted_status[BATTLE_STATUS_BYTE_COUNT]; /* 0x31 */
    u8 equipment[UNIT_EQUIPMENT_SLOT_COUNT];       /* 0x36 */
    u8 brave;                                      /* 0x3d */
    u8 mount_info;
    u8 _padding_3f; /* tail padding to 2-byte alignment */
} battle_ai_extended_snapshot_t;
typedef char battle_ai_snapshot_equipment_must_be_0x36
    [((unsigned long)&((battle_ai_extended_snapshot_t*)0)->equipment == 0x36) ? 1 : -1];
typedef char battle_ai_snapshot_hp_must_be_0x1b[((unsigned long)&((battle_ai_extended_snapshot_t*)0)->hp_bytes == 0x1b)
        ? 1
        : -1];
typedef char battle_ai_extended_snapshot_must_be_64[sizeof(battle_ai_extended_snapshot_t) == 64 ? 1 : -1];

/* AI ability +0x04..0x0b: byte parameters and an aligned 32-bit status tail.
 * 0x8019e9f4 loads status bytes 1..4 together; reading only one byte would
 * silently lose the high status bits tested by the decision evaluator. */
typedef union battle_ai_ability_parameters {
    struct {
        u8 range;
        u8 aoe;
        u8 item_id;
        u8 status_infliction[BATTLE_STATUS_BYTE_COUNT];
    } fields;
    struct {
        u32 _first;
        u32 status_tail;
    } packed;
} battle_ai_ability_parameters_t;

/* Pack canonical status IDs 8..39 into the aligned 32-bit value covering status
 * bytes 1..4 of battle_ai_ability_parameters_t. */
#define BATTLE_AI_STATUS_TAIL_PACKED_MASK(id)                                                                          \
    ((u32)BATTLE_STATUS_BYTE_MASK(id) << ((BATTLE_STATUS_BYTE_INDEX(id) - 1) * 8))

/* Considered ability, base+0x00. */
typedef struct battle_ai_considered_ability {
    u8 skillset;                               /* 0x00 */
    u8 ct;                                     /* 0x01 */
    s16 ability_id;                            /* 0x02; read with both lh and lhu */
    battle_ai_ability_parameters_t parameters; /* 0x04 */
    battle_ai_ability_flags_t ai_flags;        /* 0x0c */
    u8 element;                                /* 0x10 */
    u8 mp_cost;                                /* 0x11 */
    u8 _padding_12[2];                         /* tail padding to 4-byte alignment */
} battle_ai_considered_ability_t;
typedef char battle_ai_considered_ability_size_must_be_20[(sizeof(battle_ai_considered_ability_t) == 20) ? 1 : -1];

/* battle_ai_action_rank_t.decided of the selected action once it is chosen. */
enum {
    BATTLE_AI_ACTION_DECIDED = 0xff,
};

/* How an action follows its target, in the AI/menu command and stored unit command. */
typedef enum battle_targeting_type {
    BATTLE_TARGETING_TYPE_TILE = 5, /* aims at target_position */
    BATTLE_TARGETING_TYPE_UNIT = 6, /* follows the unit target_id */
} battle_targeting_type_e;

/* The ranking tail of an action, compared by
 * battle_ai_compare_action_rank. */
typedef struct battle_ai_action_rank {
    /* BATTLE_AI_ACTION_DECIDED on the selected action once the turn's action is
     * chosen, 0 on every candidate; the first comparison key at 0x80196db0. */
    u8 decided;
    u8 base_hit_percent;
    s16 priority;
} battle_ai_action_rank_t;
typedef char battle_ai_action_rank_size_must_be_4[(sizeof(battle_ai_action_rank_t) == 4) ? 1 : -1];

/*
 * One considered/recorded action, 0x20 bytes.  The current action lives at
 * base+0x14, the best so far at +0x34, eight ranked outcomes at +0x54, the
 * inverted-priority scratch at +0x154 and the selected action at +0x17fc.
 */
typedef struct battle_ai_action_data {
    u8 act_before_move;  /* 0x00; 1: act from the unit's own tile, then move to coords; 0: move to coords first */
    u8 reflected_action; /* 0x01; set while evaluating reflected-origin candidates */
    u8 _unused_02;
    u8 wait_facing_hint;          /* 0x03; direction 0..3, 4 keeps current, 5 chooses during Wait */
    battle_ai_coords_t coords;    /* 0x04 */
    action_command_t command;     /* 0x08; copied as the ACT payload */
    battle_ai_action_rank_t rank; /* 0x1c */
} battle_ai_action_data_t;
typedef char battle_ai_action_data_size_must_be_32[(sizeof(battle_ai_action_data_t) == 32) ? 1 : -1];

/* The packed 16-bit value combines an ability ID and a physical unit slot. */
enum {
    BATTLE_AI_ABILITY_ENTRY_ABILITY_MASK = 0x03ff,
    BATTLE_AI_ABILITY_ENTRY_UNIT_SHIFT = 10,
};

/* Four-byte entries in each compact unit's AI ability list. Whole usage
 * bytes are copied into considered flags4; policy updates address its bits. */
typedef struct battle_ai_ability_entry {
    s16 packed_id; /* bits 0..9: ability ID; bits 10..15: physical unit slot */
    u8 skillset;
    union {
        u8 flags;
        struct {
            u8 consume_target_tiles : 1;
            u8 force_consume_target_tiles : 1;
            u8 exhaustive_origin_scan : 1;
            u8 requires_monster_skill : 1;
            u8 _unknown_00_bit4 : 1;
            u8 enemies_only : 1;
            u8 allies_only : 1;
            u8 usable : 1;
        } policy;
    } usage;
} battle_ai_ability_entry_t;
typedef char battle_ai_ability_entry_must_be_4[sizeof(battle_ai_ability_entry_t) == 4 ? 1 : -1];

/* OR-combined categories gathered from both equipped weapon formulas. They
 * are flags, not ability_formula_e IDs; mixed categories bypass the builder
 * switch's single-category cases. */
typedef enum battle_ai_weapon_formula_flags {
    BATTLE_AI_WEAPON_FORMULA_NONE = 0x00, /* no weapon in either hand */
    BATTLE_AI_WEAPON_FORMULA_DAMAGE = 0x01,
    BATTLE_AI_WEAPON_FORMULA_ABSORB_HP = 0x02,
    BATTLE_AI_WEAPON_FORMULA_HEAL = 0x04,
} battle_ai_weapon_formula_flags_e;

/* Per-compact-unit weapon summary. The builder at 0x8019a2f0 clears two
 * 32-bit values, then merges weapon properties through named fields. */
typedef union battle_ai_weapon_data {
    u32 words[2];
    struct {
        u8 range;
        u8 flags_1;
        u8 flags_2;
        u8 flags_3;
        u8 weapon_id;
        u8 element;
        u8 _unused_06[2];
    } fields;
} battle_ai_weapon_data_t;
typedef char battle_ai_weapon_data_must_be_8[sizeof(battle_ai_weapon_data_t) == 8 ? 1 : -1];

/* Per-unit AI behavior flags copied from ENTD data to decision +0x04. The
 * three transient status bits are rebuilt during action simulation. */
typedef enum battle_ai_decision_flags {
    BATTLE_AI_DECISION_DEAD_WITHOUT_RERAISE = 0x01,
    BATTLE_AI_DECISION_PETRIFIED = 0x02,
    /* Built from Jump; Protect allies reads it as Blood Suck (QUIRKS.md). */
    BATTLE_AI_DECISION_JUMPING = 0x04,
    BATTLE_AI_DECISION_SPECIAL_BEHAVIOR = 0x08,
    BATTLE_AI_DECISION_AGGRESSIVE = 0x10,
    BATTLE_AI_DECISION_STAY_NEAR_COORDINATES = 0x20,
    BATTLE_AI_DECISION_FOCUS_TARGET = 0x40,
    /* Worth acting on: every non-enemy and every enemy with an action charging;
     * the refresh simulation adds each enemy that ends it alive (or dead with
     * Reraise), unpetrified and without Death Sentence. Target searches skip a
     * unit without it unless the unit is confused. The copy from the ENTD
     * record leaves the stay-near tile's level here until the first refresh. */
    BATTLE_AI_DECISION_WORTH_TARGETING = 0x80,
} battle_ai_decision_flags_e;

/* Targeting policy copied from ENTD AI Flags 2 to decision +0x07. The low
 * policy bits survive battle_ai_refresh_unit_decision_flags; the remaining
 * target-state bits are rebuilt there before action simulation. */
typedef enum battle_ai_targeting_flags_2 {
    BATTLE_AI_TARGET_HP_BELOW_HALF = 0x01,
    BATTLE_AI_TARGET_MP_CONSTRAINED = 0x02,
    BATTLE_AI_TARGET_RUN_LIKE_A_RABBIT = 0x04,
    BATTLE_AI_TARGET_TEAM_SUPPORT_UNAVAILABLE = 0x08,
    BATTLE_AI_TARGET_UNTARGETABLE = 0x10,
    BATTLE_AI_TARGET_DEAD_WITHOUT_RERAISE = 0x20,
    BATTLE_AI_TARGET_DEAD_WITH_RERAISE = 0x40,
    BATTLE_AI_TARGET_HP_BELOW_THREE_QUARTERS = 0x80,
} battle_ai_targeting_flags_2_e;

typedef enum battle_ai_target_setting_flags {
    BATTLE_AI_TARGET_SETTING_REFRESH_UNIT_STATUS_FLAGS = 0x01000000,
    BATTLE_AI_TARGET_SETTING_CONSUME_TARGET_TILES = 0x02000000,
    BATTLE_AI_TARGET_SETTING_HAS_HOLY_WATER = 0x20000000, /* the acting unit can use the Holy Water item ability */
    BATTLE_AI_TARGET_SETTING_RETREAT_SUPPORT_AVAILABLE = 0x40000000,
} battle_ai_target_setting_flags_e;

/*
 * Per-unit AI decision record, 0x10 bytes at base+0x182c (unit*16 + 0x182c).
 * acting_unit_decision points at the acting unit's record.
 */
typedef struct battle_ai_unit_decision {
    battle_ai_coords_t target; /* 0x00..0x03; x, elevation, y, 0 */
    u8 flags;                  /* 0x04; battle_ai_decision_flags_e */
    u8 main_target_id;         /* 0x05 */
    u8 targeting_flags_1;      /* 0x06; battle_ai_targeting_flags_1_e */
    u8 targeting_flags_2;      /* 0x07; battle_ai_targeting_flags_2_e */
    u8 enemy_flag;             /* 0x08 */
    u8 highest_mp_cost;        /* 0x09 */
    u8 lowest_mp_cost;         /* 0x0a */
    u8 mp_ability_mod;         /* 0x0b; quarters (0-4) of its abilities that cost MP */
    u8 silence_mod;            /* 0x0c; quarters of its abilities that Silence stops */
    u8 evade_mod;              /* 0x0d; quarters of its abilities that can be evaded */
    u8 enemy_reach;            /* 0x0e; Move + the least range + AoE of its enemy-targeting abilities */
    u8 ally_reach;             /* 0x0f; Move + the least range + AoE of its ally-targeting abilities (items halved) */
} battle_ai_unit_decision_t;
typedef char battle_ai_unit_decision_size_must_be_16[(sizeof(battle_ai_unit_decision_t) == 16) ? 1 : -1];

/* AI +0x0c78..0x0cb7. The target's 21-byte save at +0xca2 overlaps
 * the low three bytes of target_setting_flags at +0xcb4. Loops at
 * 0x8019b184..0x8019b1b8 and 0x8019b260..0x8019b274 establish the span.
 * The CONSUME_TARGET_TILES flag lies in the fourth byte, outside the saved array.
 * Preserve both views and 32-bit alignment; this is not a 16-byte backup. */
typedef union battle_ai_targetability {
    struct {
        u8 unit_active[BATTLE_UNIT_SLOT_COUNT];
        u8 unit_targetable[BATTLE_UNIT_SLOT_COUNT];
        u8 saved_prefix[16];
        u8 _padding_3a[2]; /* aligns target_setting_flags */
        u32 target_setting_flags;
    } live;
    struct {
        u8 _unused_00[42];
        u8 unit_targetable_saved[BATTLE_UNIT_SLOT_COUNT];
        u8 _unused_3f;
    } snapshot;
} battle_ai_targetability_t;
typedef char battle_ai_targetability_size_must_be_64[(sizeof(battle_ai_targetability_t) == 64) ? 1 : -1];

/* Fixed workspace capacities and tile-grid storage, independent of map bounds. */
enum {
    BATTLE_AI_COMPACT_UNIT_COUNT = 16,
    BATTLE_AI_ABILITY_LIST_LENGTH = 34,
    BATTLE_AI_RANKED_ACTION_COUNT = 8,
    BATTLE_AI_MOVEMENT_SCENARIO_COUNT = 3,
    BATTLE_AI_KNOWN_TRAP_COUNT = 4,
    BATTLE_AI_GRID_ROW_COUNT = 18,
    BATTLE_AI_GRID_COLUMN_COUNT = 16,
};

enum {
    BATTLE_AI_MATH_SOURCE_SKILLSET_COUNT = 4,
    BATTLE_AI_MATH_EXTRA_KNOWN_ABILITY_COUNT = 10,
};

/* Scanlines since vertical blank at which each search yields. */
enum {
    BATTLE_AI_TURN_SIMULATION_SCANLINE_LIMIT = 313,
    BATTLE_AI_PATHFIND_SCANLINE_LIMIT = 325,
    BATTLE_AI_DECISION_SCANLINE_LIMIT = 441,
};

/* Turn-event iteration limit for one simulated action. */
enum {
    BATTLE_AI_SIMULATED_TURN_LIMIT = 256,
};

/* Shares of usable abilities are measured in 128ths. */
enum {
    BATTLE_AI_ABILITY_SHARE_SCALE = 128,
    BATTLE_AI_SUPPORT_SHARE_THRESHOLD = 77,
};

typedef struct battle_ai_data {
    battle_ai_considered_ability_t considered_ability;                     /* 0x0000 */
    battle_ai_action_data_t current_action;                                /* 0x0014 */
    battle_ai_action_data_t best_action;                                   /* 0x0034 */
    battle_ai_action_data_t ranked_actions[BATTLE_AI_RANKED_ACTION_COUNT]; /* 0x0054 */
    battle_ai_action_data_t movement_baseline_action; /* 0x0154; only moving: a ranked action must beat it */
    /* Per reachable tile, 10 - distance summed over the imminent enemies within 10 tiles
     * (battle_ai_fill_foe_proximity_grid). */
    u8 tile_foe_proximity[MAP_TERRAIN_LAYER_COUNT][BATTLE_AI_GRID_ROW_COUNT]
                         [BATTLE_AI_GRID_COLUMN_COUNT]; /* 0x0174; level/y/x, strides
                                                         * 288/16/1 at 0x80196e68..0x80196ebc */
    /* Per tile, the movement cost from the decision target (battle_ai_fill_target_distance_grid). */
    u8 tile_target_distance[MAP_TERRAIN_LAYER_COUNT][BATTLE_AI_GRID_ROW_COUNT]
                           [BATTLE_AI_GRID_COLUMN_COUNT]; /* 0x03b4; level/y/x, strides 288/16/1 */
    /* Per reachable tile, the negated score of standing there (battle_ai_score_reachable_tiles): lower is better. */
    s16 tile_priority[MAP_TERRAIN_LAYER_COUNT][BATTLE_AI_GRID_ROW_COUNT]
                     [BATTLE_AI_GRID_COLUMN_COUNT]; /* 0x05f4; level/y/x, byte strides 576/32/2 */
    /* Scenario/level/y; row bits 15..0 represent x=0..15. Scenario 0 holds every reachable tile
     * (battle_ai_build_reachable_tiles); battle_ai_build_movement_scenarios and battle_ai_build_targeted_movement
     * derive scenarios 1 and 2 from it. */
    u16 reachable_tiles[BATTLE_AI_MOVEMENT_SCENARIO_COUNT][MAP_TERRAIN_LAYER_COUNT]
                       [BATTLE_AI_GRID_ROW_COUNT]; /* 0x0a74; byte strides 72/36/2 */
    /* Scratch: a range diamond (battle_ai_build_targetable_tile_mask), then the target tiles in range of the origin
     * (battle_ai_evaluate_target_tiles). */
    u16 targetable_tiles[MAP_TERRAIN_LAYER_COUNT][BATTLE_AI_GRID_ROW_COUNT]; /* 0x0b4c */
    u16 origin_tiles[MAP_TERRAIN_LAYER_COUNT][BATTLE_AI_GRID_ROW_COUNT]; /* 0x0b94; the reachable tiles to act from */
    u16 target_tiles[MAP_TERRAIN_LAYER_COUNT]
                    [BATTLE_AI_GRID_ROW_COUNT]; /* 0x0bdc; the tiles to aim at, dropped once settled */
    u16 walkable_tiles[MAP_TERRAIN_LAYER_COUNT]
                      [BATTLE_AI_GRID_ROW_COUNT]; /* 0x0c24; not blocked, not a cross section */
    u8 _unused_0c6c;
    u8 initial_selection_stage; /* 0x0c6d; battle_ai_initial_selection_stage_e */
    u8 action_selection_phase;  /* 0x0c6e; status, targeted and recovery selection stages */
    u8 _unused_0c6f;
    u8 outcome_evaluation_state;               /* 0x0c70; battle_ai_ability_evaluation_stage_e */
    u8 movement_decision_state;                /* 0x0c71; battle_ai_movement_stage_e */
    u8 search_resume_flag;                     /* 0x0c72; distance-search initialization/resume flag
                                                * at 0x801994f8; reused as a phase flag at 0x8019cd9c. */
    u8 _padding_0c73;                          /* aligns considered_unit_coords */
    battle_ai_coords_t considered_unit_coords; /* 0x0c74 */
    battle_ai_targetability_t targetability;   /* 0x0c78 */
    /* Included target/strike pairs, not unique units; 0xff disables counting. */
    u16 total_hit_percent;      /* 0x0cb8 */
    u8 hit_counter;             /* 0x0cba */
    u8 target_tile_count;       /* 0x0cbb */
    u8 origin_tile_count;       /* 0x0cbc */
    u8 ability_counter;         /* 0x0cbd */
    u8 has_mp_recovery_ability; /* 0x0cbe; a usable ability affects MP without targeting enemies */
    u8 tile_pickup_kind;        /* 0x0cbf; battle_ai_tile_pickup_kind_e */
    u8 _unused_0cc0[4];
    battle_ai_coords_t candidate_coords[BATTLE_AI_MOVEMENT_SCENARIO_COUNT]; /* 0x0cc4; indexed by movement_scenario */
    u8 _unused_0cd0[4];
    action_command_t unit_action_records[BATTLE_AI_COMPACT_UNIT_COUNT]; /* 0x0cd4 */
    u16 throw_ability_id;                                               /* 0x0e14 */
    u8 throw_weapon_id;                                                 /* 0x0e16 */
    /* All slots with entd_slot != 0xff, capped at 255 by 0x80199d20. */
    u8 present_unit_average_max_hp;                  /* 0x0e17 */
    u8 unit_acts_before_me[BATTLE_UNIT_SLOT_COUNT];  /* 0x0e18..0x0e2c; all slots cleared at 0x8019d408–0x8019d41c */
    u8 movement_scenario;                            /* 0x0e2d; 0..2, the reachable_tiles in use */
    u8 acting_unit_id;                               /* 0x0e2e */
    u8 acting_unit_compact_index;                    /* 0x0e2f; unit_compact_indices[acting_unit_id] */
    battle_ai_coords_t acting_unit_coords;           /* 0x0e30 */
    battle_ai_unit_decision_t* acting_unit_decision; /* 0x0e34 */
    u8 acting_unit_move;                             /* 0x0e38 */
    u8 acting_unit_team;                             /* 0x0e39 */
    u8 map_max_x;                                    /* 0x0e3a */
    u8 map_max_y;                                    /* 0x0e3b */
    u16 team_golem[BATTLE_TEAM_COUNT];               /* 0x0e3c; 16-bit scoring at 0x8019d37c */
    u8 status_to_cancel[BATTLE_STATUS_BYTE_COUNT]; /* 0x0e44; usable actor abilities' aggregate cancellation mask, built
                                                      at 0x80195410 */
    /* Resumable reflected-origin search, 0x8019b7b8–0x8019bb20.
     * Candidate x/y = 2 * reflector position - intended target position. */
    u8 reflector_unit_index;        /* 0x0e49 */
    u8 reflected_target_unit_index; /* 0x0e4a */
    u8 reflected_candidate_level;   /* 0x0e4b */
    u8 ability_effect_on_self;      /* 0x0e4c; battle_ai_ability_effect_e on the acting unit */
    /* 1: take the origins best first (battle_ai_take_best_origin). */
    u8 ranked_origin_search; /* 0x0e4d; consume the ranked origin tiles */
    /* Drop the target tile just evaluated: always with CONSUME_TARGET_TILES; in the ranked search also when the
     * caster's own part came out as wanted (battle_ai_simulate_strikes). */
    u8 target_tile_settled;                                                        /* 0x0e4e */
    u8 acting_unit_remaining_clockticks;                                           /* 0x0e4f */
    battle_ai_status_snapshot_t unit_status_records[BATTLE_AI_COMPACT_UNIT_COUNT]; /* 0x0e50 */
    u8 saved_ability_ct;                                                           /* 0x0ed0 */
    u8 saved_current_status;                                                       /* 0x0ed1 */
    u8 saved_inflicted_status;                                                     /* 0x0ed2 */
    u8 _unused_0ed3;
    u8 main_ai_state;          /* 0x0ed4; battle_ai_command_build_stage_e */
    u8 autobattle_state;       /* 0x0ed5; battle_ai_autobattle_stage_e */
    u8 charging_state;         /* 0x0ed6; battle_ai_charging_stage_e */
    u8 cast_origin_stage;      /* 0x0ed7; battle_ai_cast_origin_stage_e: the resume point of
                                  battle_ai_evaluate_before_and_after_move */
    u8 reflected_origin_phase; /* 0x0ed8; cleared at 0x8019ba40, set at 0x8019ba78 */
    u8 origin_search_state;    /* 0x0ed9; battle_ai_origin_evaluation_stage_e: the resume point of
                                  battle_ai_evaluate_origins */
    u8 max_possibilities;      /* 0x0eda; byte-truncated combination count, then scan x at 0x8019bf2c */
    u8 y_counter;              /* 0x0edb */
    u8 map_level_counter;      /* 0x0edc */
    /* Calculator search counters survive suspension at 0x8019bbbc–0x8019bf2c. */
    u8 math_ability_id;       /* 0x0edd; low byte of the selected ability entry */
    u8 math_type_index;       /* 0x0ede; 0..3 */
    u8 math_multiplier_index; /* 0x0edf; 4..7 */
    /* Turn-controller globals saved/restored by 0x8019d37c–0x8019db80. */
    s32 saved_between_turn_state;        /* 0x0ee0; g_battle_turn_clock_state during the simulation */
    s32 saved_between_turn_resume_state; /* 0x0ee4; g_battle_turn_clock_resume_state */
    s32 saved_turn_unit_id;              /* 0x0ee8; g_battle_turn_unit_id */
    u16 simulated_turn_events;           /* 0x0eec; interturn loop stops at 256 */
    u8 action_simulation_phase;          /* 0x0eee; battle_ai_simulation_phase_e */
    u8 _padding_0eef;                    /* aligns ability_lists */
    battle_ai_ability_entry_t ability_lists[BATTLE_AI_COMPACT_UNIT_COUNT]
                                           [BATTLE_AI_ABILITY_LIST_LENGTH]; /* 0x0ef0; 0x88 bytes per unit */
    /* Blood Suck/Frog IDs start without unit bits; 0x8019729c/0x80197510
     * OR the acting physical slot into bits 10..15 before loading the entry. */
    battle_ai_ability_entry_t hardcoded_status_abilities[2];                /* 0x1770 */
    battle_ai_weapon_data_t unit_weapon_data[BATTLE_AI_COMPACT_UNIT_COUNT]; /* 0x1778 */
    battle_stats_t* acting_unit;                                            /* 0x17f8 */
    battle_ai_action_data_t selected_action;                                /* 0x17fc */
    /* Up to four tiles where a unit sprang a trap this battle (battle_ai_record_sprung_trap); unless the acting unit is
     * Confused or Berserk, the AI leaves them out of its reachable tiles (battle_ai_build_reachable_tiles). x == 0xff
     * ends it. */
    battle_ai_coords_t sprung_trap_tiles[BATTLE_AI_KNOWN_TRAP_COUNT]; /* 0x181c; x == 0xff terminates the list */
    battle_ai_unit_decision_t unit_decisions[BATTLE_UNIT_SLOT_COUNT]; /* 0x182c */
    /* 0x80195ed4 saves the complete decision before retreat; 0x80195efc
     * restores it only after the child finishes, not when it suspends. */
    battle_ai_unit_decision_t saved_acting_unit_decision; /* 0x197c */
    u8 unit_compact_indices[BATTLE_UNIT_SLOT_COUNT];      /* 0x198c; slot -> index among present units (0..15) */
    u8 unit_autobattle_modes[BATTLE_UNIT_SLOT_COUNT];     /* 0x19a1; battle_auto_battle_setting_e from the ENTD flags */
    u8 autobattle_setting;                                /* 0x19b6; battle_auto_battle_setting_e of this plan */
    u8 resume_pending;                                    /* 0x19b7; an AI routine suspended on its frame budget */
    u8 random_use_threshold;                              /* 0x19b8; 0x80195778..0x80195788 stores
                                                           * floor(128 * random-use entries / eligible entries).
                                                           * A zero denominator leaves the prior byte unchanged. */
    u8 water_penalty;                                     /* 0x19b9 */
    u8 valuable_target_hit;                               /* 0x19ba; a counted hit on unit_targetable sets this
                                                           * at 0x8019e010..0x8019e020. */
    u8 single_enemy_remaining;                            /* 0x19bb; exactly one enemy counted */
    u8 ability_processing_done;                           /* 0x19bc */
    u8 action_taken;                                      /* 0x19bd */
    u8 movement_taken;                                    /* 0x19be */
    u8 _unused_19bf[8];
    u8 _padding_19c7; /* 0x19c7; last documented byte; tail padding to 4-byte alignment */
} battle_ai_data_t;
typedef char battle_ai_data_size_must_be_6600[(sizeof(battle_ai_data_t) == 6600) ? 1 : -1];

/* Runtime policy byte in battle_ai_ability_entry_t. The low three bits control
 * how the AI searches and consumes candidate action origins; initialization
 * recomputes them while preserving the remaining bits. */
typedef enum battle_ai_ability_entry_flag {
    BATTLE_AI_ABILITY_ENTRY_CONSUME_TARGET_TILES = 0x01,
    BATTLE_AI_ABILITY_ENTRY_FORCE_CONSUME_TARGET_TILES = 0x02,
    BATTLE_AI_ABILITY_ENTRY_EXHAUSTIVE_ORIGIN_SCAN = 0x04,
    BATTLE_AI_ABILITY_ENTRY_REQUIRES_MONSTER_SKILL = 0x08,
    BATTLE_AI_ABILITY_ENTRY_ENEMIES_ONLY = 0x20,
    BATTLE_AI_ABILITY_ENTRY_ALLIES_ONLY = 0x40,
    BATTLE_AI_ABILITY_ENTRY_USABLE = 0x80,
    BATTLE_AI_ABILITY_ENTRY_INIT_PRESERVE_MASK = 0xf8,
} battle_ai_ability_entry_flag_e;

typedef enum battle_ai_rank_order {
    BATTLE_AI_RANK_BELOW = 0,
    BATTLE_AI_RANK_EQUAL = 1,
    BATTLE_AI_RANK_ABOVE = 2,
} battle_ai_rank_order_e;

typedef enum battle_ai_unit_filter {
    BATTLE_AI_UNIT_FILTER_ANY = 0,
    BATTLE_AI_UNIT_FILTER_ENEMIES = 1,
    BATTLE_AI_UNIT_FILTER_ALLIES = 2,
    BATTLE_AI_UNIT_FILTER_SPECIFIC = 3,
    BATTLE_AI_UNIT_FILTER_STATUS = 4,
} battle_ai_unit_filter_e;

typedef enum battle_ai_nearest_target_mode {
    BATTLE_AI_NEAREST_WITHOUT_BLOOD_SUCK = 0,
    BATTLE_AI_NEAREST_IMMINENT_ENEMY = 1,
    BATTLE_AI_NEAREST_ENEMY = 2,
    BATTLE_AI_NEAREST_UNIT = 3,
    BATTLE_AI_NEAREST_ELIGIBLE_ENEMY = 4,
    BATTLE_AI_NEAREST_HEALER_OR_CRYSTAL = 5,
} battle_ai_nearest_target_mode_e;

typedef enum battle_ai_movement_stage {
    BATTLE_AI_MOVEMENT_TARGET_DISTANCE = 0,
    BATTLE_AI_MOVEMENT_SCORE_TILES = 1,
} battle_ai_movement_stage_e;

typedef enum battle_ai_cowardly_movement_stage {
    BATTLE_AI_COWARDLY_MOVEMENT_SCORE_TILES = 0,
    BATTLE_AI_COWARDLY_MOVEMENT_TARGET_DISTANCE = 1,
} battle_ai_cowardly_movement_stage_e;

typedef enum battle_ai_destination_order {
    BATTLE_AI_DESTINATION_PRIORITY_FIRST = 0,
    BATTLE_AI_DESTINATION_DISTANCE_FIRST = 1,
} battle_ai_destination_order_e;

typedef enum battle_ai_tile_mask_mode {
    BATTLE_AI_TILE_MASK_RANGE = 0,
    BATTLE_AI_TILE_MASK_AOE = 1,
    BATTLE_AI_TILE_MASK_RANGE_PLUS_AOE = 2,
    BATTLE_AI_TILE_MASK_EXPLICIT = 3,
} battle_ai_tile_mask_mode_e;

typedef enum battle_ai_command_build_stage {
    BATTLE_AI_COMMAND_BUILD_REFRESH = 0,
    BATTLE_AI_COMMAND_BUILD_MOVEMENT = 1,
    BATTLE_AI_COMMAND_BUILD_CHOOSE_ACTION = 2,
} battle_ai_command_build_stage_e;

typedef enum battle_ai_direction {
    BATTLE_AI_DIRECTION_SOUTH = 0,
    BATTLE_AI_DIRECTION_WEST = 1,
    BATTLE_AI_DIRECTION_NORTH = 2,
    BATTLE_AI_DIRECTION_EAST = 3,
    BATTLE_AI_DIRECTION_OVERLAP = 4,
    BATTLE_AI_DIRECTION_CHOOSE_AT_WAIT = 5,
} battle_ai_direction_e;

typedef enum battle_ai_targeting_flags_1 {
    BATTLE_AI_TARGETING_FLAG_1_HAS_RANGED_ABILITY = 0x01, /* a usable ability has a direct or arc trajectory */
    BATTLE_AI_TARGETING_FLAG_1_CAN_HEAL_ALLIES = 0x02,    /* a usable ability affects HP and targets allies */
    BATTLE_AI_TARGETING_FLAG_1_MOSTLY_SUPPORT = 0x04,     /* ally-targeting share of the usable abilities >= 77/128 */
    BATTLE_AI_TARGETING_FLAG_1_REFLECTED_ACTION = 0x08,
} battle_ai_targeting_flags_1_e;

/* Classification from the acting unit's side, not the sign of HP damage.
 * A useful effect may include deliberately hitting an ally to affect status. */
typedef enum battle_ai_ability_effect {
    BATTLE_AI_ABILITY_EFFECT_NONE = 0,
    BATTLE_AI_ABILITY_EFFECT_USEFUL = 1,
    BATTLE_AI_ABILITY_EFFECT_ADVERSE = 2,
} battle_ai_ability_effect_e;

/* Byte-sized resume labels for initial action selection; the gaps are not dispatched. */
typedef enum battle_ai_initial_selection_stage {
    BATTLE_AI_INITIAL_SELECTION_STATUS = 0,
    BATTLE_AI_INITIAL_SELECTION_RETREAT = 2,
    BATTLE_AI_INITIAL_SELECTION_AUTOBATTLE = 3,
    BATTLE_AI_INITIAL_SELECTION_GENERAL = 7,
} battle_ai_initial_selection_stage_e;

typedef enum battle_ai_status_selection_stage {
    BATTLE_AI_STATUS_SELECTION_CHICKEN = 0,
    BATTLE_AI_STATUS_SELECTION_TARGET_DISTANCE = 1,
} battle_ai_status_selection_stage_e;

/* These three suspend points use their own byte; other values resume abilities. */
typedef enum battle_ai_autobattle_stage {
    BATTLE_AI_AUTOBATTLE_CHARGING = 2,
    BATTLE_AI_AUTOBATTLE_MOVEMENT = 4,
    BATTLE_AI_AUTOBATTLE_ABILITIES = 7,
} battle_ai_autobattle_stage_e;

typedef enum battle_ai_targeted_selection_stage {
    BATTLE_AI_TARGETED_SELECTION_SCORE_TILES = 0,
    BATTLE_AI_TARGETED_SELECTION_MOVEMENT = 1,
    BATTLE_AI_TARGETED_SELECTION_CHARGING = 2,
    BATTLE_AI_TARGETED_SELECTION_TARGET_ABILITIES = 3,
    BATTLE_AI_TARGETED_SELECTION_OTHER_ABILITIES = 4,
} battle_ai_targeted_selection_stage_e;

/* No pass saves stage 5; it follows the same ally-search fallback as stage 6. */
typedef enum battle_ai_recovery_selection_stage {
    BATTLE_AI_RECOVERY_SELECTION_SCORE_TILES = 0,
    BATTLE_AI_RECOVERY_SELECTION_MOVEMENT = 1,
    BATTLE_AI_RECOVERY_SELECTION_CHARGING = 2,
    BATTLE_AI_RECOVERY_SELECTION_TARGET_ABILITIES = 3,
    BATTLE_AI_RECOVERY_SELECTION_ENEMY_ABILITIES = 4,
    BATTLE_AI_RECOVERY_SELECTION_ALLY_ABILITIES = 6,
    BATTLE_AI_RECOVERY_SELECTION_CANCEL_STATUS = 7,
} battle_ai_recovery_selection_stage_e;

typedef enum battle_ai_retreat_stage {
    BATTLE_AI_RETREAT_MOVEMENT = 0,
    BATTLE_AI_RETREAT_SELF_ABILITIES = 1,
    BATTLE_AI_RETREAT_CHARGING = 2,
    BATTLE_AI_RETREAT_OTHER_ABILITIES = 3,
} battle_ai_retreat_stage_e;

enum {
    BATTLE_AI_RECOVERY_STATUS_COUNT = 3,
};

extern s32 g_battle_ai_propagation_resume_pass;
extern u16 g_battle_ai_range_row_masks[];

typedef enum battle_ai_ability_evaluation_stage {
    BATTLE_AI_ABILITY_EVALUATION_REFLECTED_TARGETS = 0,
    BATTLE_AI_ABILITY_EVALUATION_SELF_TARGET_RANGE = 1,
    BATTLE_AI_ABILITY_EVALUATION_UNIT_TARGETS = 2,
    BATTLE_AI_ABILITY_EVALUATION_FIRST_CT_GROUP = 3,
    BATTLE_AI_ABILITY_EVALUATION_SECOND_CT_GROUP = 4,
    BATTLE_AI_ABILITY_EVALUATION_TILE_TARGETS = 5,
    BATTLE_AI_ABILITY_EVALUATION_MATH_TARGETS = 6,
    BATTLE_AI_ABILITY_EVALUATION_PREPARE = 7,
    BATTLE_AI_ABILITY_EVALUATION_MAP_TARGETS = 8,
} battle_ai_ability_evaluation_stage_e;

typedef enum battle_ai_origin_evaluation_stage {
    BATTLE_AI_ORIGIN_EVALUATION_PREPARE = 0,
    BATTLE_AI_ORIGIN_EVALUATION_RANKED = 1,
    BATTLE_AI_ORIGIN_EVALUATION_SCANNED = 2,
} battle_ai_origin_evaluation_stage_e;

typedef enum battle_ai_cast_origin_stage {
    BATTLE_AI_CAST_ORIGIN_ACT_BEFORE_MOVE = 0,
    BATTLE_AI_CAST_ORIGIN_MOVE_BEFORE_ACT = 1,
} battle_ai_cast_origin_stage_e;

typedef enum battle_ai_charging_stage {
    BATTLE_AI_CHARGING_SONG_DANCE = 0,
    BATTLE_AI_CHARGING_WITH_CHARGE = 1,
    BATTLE_AI_CHARGING_WITHOUT_CHARGE = 2,
} battle_ai_charging_stage_e;

extern s32 g_battle_ai_self_target_origin_stage;
extern battle_ai_command_t g_battle_ai_command_history[2]; /* 0x801a0d94, 0x801a0dac */
extern battle_ai_cowardly_movement_stage_e g_battle_ai_retreat_movement_step;

/* BATTLE-resident workspace; EVENT, EFFECT and WORLD overlays also use its pointer. */
extern battle_ai_data_t g_battle_ai_data_base;

extern s32 g_battle_ai_math_ability_index;
extern u16* g_battle_ai_math_ability_list; /* borrowed from main's temporary skillset list */
extern u8 g_battle_ai_math_extra_known_ability_ids[BATTLE_AI_MATH_EXTRA_KNOWN_ABILITY_COUNT];
extern s32 g_battle_ai_math_skillset_id;
extern s32 g_battle_ai_math_skillset_index;
extern u8 g_battle_ai_recovery_status_ids[BATTLE_AI_RECOVERY_STATUS_COUNT];
extern s32 g_battle_ai_recovery_status_index; /* 0x8019f380; status-list cursor preserved for resume */
extern s32 g_battle_ai_recovery_status_retry; /* 0x8019f37c; skip status pass after one failed attempt */
extern s32 g_battle_ai_retreat_phase;         /* 0x8019f390; battle_ai_retreat_stage_e */

/* Cancellation-search accounting survives suspension. */
extern s32 g_battle_ai_status_candidate_count;

extern s32 g_battle_ai_status_insufficient_mp;

/* Pickup occupant on the current action tile; treasure takes precedence. */
typedef enum battle_ai_tile_pickup_kind {
    BATTLE_AI_TILE_PICKUP_NONE = 0,
    BATTLE_AI_TILE_PICKUP_CRYSTAL = 1,
    BATTLE_AI_TILE_PICKUP_TREASURE = 2,
} battle_ai_tile_pickup_kind_e;

/* Signed scoring weights, indexed by status ID. */
extern s16 g_battle_ai_status_priority_weights[BATTLE_STATUS_COUNT]; /* 0x8019f308..0x8019f357 */

extern s32 g_battle_ai_status_sufficient_mp;

/* Unit snapshots share the scratchpad-save arena used during movement propagation. */
extern battle_ai_extended_snapshot_t g_battle_ai_unit_snapshot_storage[];

extern battle_ai_extended_snapshot_t* g_battle_ai_unit_snapshots; /* 0x8019f3c0 */
extern battle_ai_data_t* g_battle_ai_workspace;                   /* 0x8019f3ac */
extern world_event_work_t* g_ai_workspace_ptr;                    /* pointer cell reloaded per subsystem */
extern u8 g_battle_ai_math_source_skillsets[BATTLE_AI_MATH_SOURCE_SKILLSET_COUNT];
s32 battle_ai_add_usable_skillset_abilities(s32 unit_id, s32 skillset, s32 count);
void battle_ai_begin_simulation(void);
s32 battle_ai_build_command(s32 unit_id, battle_ai_command_t* command);
void battle_ai_build_monster_skill_tile_mask(void);
s32 battle_ai_build_movement_scenarios(void);
void battle_ai_build_origin_and_target_tiles(void);
s32 battle_ai_build_reachable_tiles(s32 movement_taken);
s32 battle_ai_build_retreat_movement(void);
void battle_ai_build_targetable_tile_mask(battle_ai_tile_mask_mode_e mode, s32 radius);
s32 battle_ai_build_targeted_movement(void);
void battle_ai_build_unit_ability_list(s32 unit_id);
s32 battle_ai_calculate_clockticks_until_death_counter_expires();
s32 battle_ai_calculate_clockticks_until_unit_acts(const battle_stats_t* unit);
s32 battle_ai_calculate_distance_between_units(const battle_stats_t* a, const battle_stats_t* b);
s32 battle_ai_calculate_ratio_times_4(s32 divisor, s32 dividend);
s32 battle_ai_can_cancel_current_status(const battle_stats_t* unit, s32 status_bit);
s32 battle_ai_can_elemental_ability_be_used(const battle_ai_coords_t* coords);
s32 battle_ai_can_unit_move_within(s32 limit, s32 unit_id);
s32 battle_ai_can_use_ability_at_coords(const battle_ai_coords_t* coordinates);
void battle_ai_update_tile_pickup(void);

/* Only the four cardinal hints index the tile-delta table. */
enum {
    BATTLE_AI_CARDINAL_DIRECTION_COUNT = 4,
};

/* Cardinal map-tile deltas stored as wrapping unsigned bytes. */
typedef struct battle_ai_facing_tile_offsets {
    u8 by_direction[BATTLE_AI_CARDINAL_DIRECTION_COUNT][2];
} battle_ai_facing_tile_offsets_t;

extern battle_ai_facing_tile_offsets_t g_battle_ai_facing_tile_offsets; /* 0x8019f358 */
s32 battle_ai_select_random_action(void);
s32 battle_ai_select_status_action(void);
s32 battle_ai_choose_wait_facing(void);
battle_ai_ability_effect_e battle_ai_classify_ability_effect(s32 unit_id, s32 check_reflect);
void battle_ai_clear_current_action_decided(void);
battle_ai_rank_order_e battle_ai_compare_action_rank(
    const battle_ai_action_rank_t* a, const battle_ai_action_rank_t* b);
s32 battle_ai_count_flagged_tiles_within_map_bounds(u16 rows[MAP_TERRAIN_LAYER_COUNT][BATTLE_AI_GRID_ROW_COUNT]);
s32 battle_ai_count_target_tiles_in_range(void);
void battle_ai_disable_ability_if_only_inflicts_status(battle_ai_ability_entry_t* entry, s32 status_id);
void battle_ai_end_simulation(void);
s32 battle_ai_evaluate_ability_list(void);
s32 battle_ai_evaluate_ability_outcome(void);
s32 battle_ai_evaluate_before_and_after_move(void);
s32 battle_ai_evaluate_charged_action(void);
s32 battle_ai_evaluate_math_targets(void);
s32 battle_ai_evaluate_origins(void);
s32 battle_ai_evaluate_reflected_target_origins(void);
s32 battle_ai_evaluate_self_target_origins(void);
s32 battle_ai_evaluate_status_cancellation(s32 unit_id, s32 status_id);
s32 battle_ai_evaluate_target_tiles(void);
s32 battle_ai_exclude_enemies_acting_before_ct(void);
void battle_ai_fill_foe_proximity_grid(void);
s32 battle_ai_fill_target_distance_grid(const battle_ai_coords_t* target);
s32 battle_ai_filter_reachable_tiles_by_target_distance(s32 distance_limit);
battle_ai_direction_e battle_ai_find_direction_of_target(
    const battle_ai_coords_t* target, const battle_ai_coords_t* origin);
s32 battle_ai_find_nearest_target(battle_ai_nearest_target_mode_e mode);
s32 battle_ai_find_unit_at_coordinates(const battle_ai_coords_t* coords);
s32 battle_ai_find_unit_on_target_tile(void);
s32 battle_ai_get_target_reach(void);
s32 battle_ai_get_unit_height_above_coords(const battle_ai_coords_t* coords, const battle_stats_t* unit);
s32 battle_ai_select_auto_battle_action(void);
void battle_ai_init_acting_unit_data(void);
void battle_ai_init_selected_action(void);
void battle_ai_init_target_consideration(void);
void battle_ai_init_unit_abilities(s32 unit_id);
void battle_ai_init_workspace(void);
void battle_ai_insert_ranked_action(void);
s32 battle_ai_is_action_higher_ranked(const battle_ai_action_data_t* a, const battle_ai_action_data_t* b);
s32 battle_ai_is_facing_tile_blocked(const battle_ai_coords_t* source, const battle_ai_coords_t* target);
s32 battle_ai_is_propagation_budget_spent(void);
s32 battle_ai_is_status_active_through_delay(s32 delay, const battle_stats_t* unit, s32 status_id);
s32 battle_ai_is_unit_out_of_play(s32 unit_id);
s32 battle_ai_is_unit_untargetable(s32 unit_id);
s32 battle_ai_keep_useful_targets(void);
void battle_ai_load_ability_entry(const battle_ai_ability_entry_t* entry);
s32 battle_ai_is_ability_known(s32 unit_id, s32 skillset_id, s32 bit);
void battle_ai_move_temp_unit_to_coords(const battle_ai_coords_t* coordinates);
s32 battle_ai_moves_before_charge_lands(s32 unit_index);

s32 battle_ai_propagate_target_movement(
    s32 unit_id, s32 target_x, s32 target_y, s32 target_elevation, s32 initialize, s32* suspended, s32 check_budget);

void battle_ai_record_sprung_trap(s32 x, s32 y, s32 level);
s32 battle_ai_refresh_unit_decision_flags(void);
void battle_ai_reset_action_ranking_fields(battle_ai_action_data_t* action);
void battle_ai_restore_unit_state(battle_ai_snapshot_mode_e mode);
void battle_ai_save_unit_state(battle_ai_snapshot_mode_e mode);
s32 battle_ai_score_current_action(void);
s32 battle_ai_score_reachable_tiles(void);
s32 battle_ai_select_action(void);
s32 battle_ai_select_active_units(battle_ai_unit_filter_e mode, s32 target_id);
void battle_ai_finalize_action_destination(void);
s32 battle_ai_select_destination(s32 max_distance, battle_ai_destination_order_e order);
void battle_ai_select_destination_with_nearest_fallback(s32 range);
void battle_ai_select_destination_with_range_floor(s32 range);
s32 battle_ai_select_fight_for_life_action(void);
s32 battle_ai_select_general_action(void);
void battle_ai_select_move_only_action(void);
s32 battle_ai_select_peril_target(void);
s32 battle_ai_select_protect_allies_action(void);
s32 battle_ai_select_ranked_action(void);
s32 battle_ai_select_retreat_action(void);
s32 battle_ai_select_save_fading_life_action(void);
void battle_ai_set_ability_considerations(s32 action_taken);
void battle_ai_set_candidate_and_actor_reachable_tiles(void);
void battle_ai_set_movement_baseline(void);
s32 battle_ai_should_exclude_from_imminent_enemies(s32 unit_id);
s32 battle_ai_simulate_ability_use(action_command_t* action);
s32 battle_ai_simulate_action_and_score(void);
void battle_ai_simulate_movement_and_pickup(void);
void battle_ai_simulate_strikes(action_command_t* action, s32 reaction_mode);
s32 battle_ai_status_lasts_until_turn(const battle_stats_t* unit, s32 status);
void battle_ai_focus_on_target(s32 unit_id);
void battle_ai_store_weapon_attack_data(battle_ai_weapon_data_t* out, s32 item_id, s32* flags_out);
void battle_ai_take_best_origin(void);
s32 battle_ai_take_random_tile(
    u16 tiles[MAP_TERRAIN_LAYER_COUNT][BATTLE_AI_GRID_ROW_COUNT], battle_ai_coords_t* coordinates, s32 count);
void battle_ai_transfer_unit_coordinates(s32 unit_index, battle_ai_coords_t* destination);
void battle_ai_update_team_targeting_flags(void);

#endif
