#ifndef LT2RB_CONSTANTS_H
#define LT2RB_CONSTANTS_H

#include <stdint.h>

enum {
    LT2RB_DEFAULT_RB_OFFSET = 0xFEA4C,
    LT2RB_SEGMENT_TABLE_OFFSET = 0x1038,
    LT2RB_BITMAP_SEGMENT_TYPE = 2,
    LT2RB_FORMAT_RGB565 = 0x0660,
    LT2RB_FORMAT_RGB565_MASK = 0x8760,
    LT2RB_FORMAT_GRAY8 = 0xC300,
};

static const uint64_t LT2RB_DEFAULT_RB_LENGTH = 8072295;
static const char *const LT2RB_DEFAULT_OUTPUT = "Lemonade2.rb";
static const char *const LT2RB_SOURCE_URL =
    "https://www.myabandonware.com/game/lemonade-tycoon-2-c4g";

#endif
