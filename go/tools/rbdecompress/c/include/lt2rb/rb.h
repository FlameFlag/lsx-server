#ifndef LT2RB_RB_H
#define LT2RB_RB_H

#include <stdbool.h>
#include <stddef.h>

#include "lt2rb/util.h"

int lt2rb_extract_bitmap_pngs(Lt2rbBuffer rb, const char *output_dir,
    bool transparency, size_t *out_count);

#endif
