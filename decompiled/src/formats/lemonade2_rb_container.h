/*
 * Lemonade Tycoon 2 Lemonade2.rb container reconstruction.
 *
 * This is not a PE function decompile. It is a data-format reconstruction from
 * the extracted Lemonade2.rb payload at decompiled/local/lt2_install/Lemonade2.rb.
 * Offsets here are file offsets inside that RB file unless otherwise stated.
 */
#ifndef LT2_LEMONADE2_RB_CONTAINER_H
#define LT2_LEMONADE2_RB_CONTAINER_H

#include <stddef.h>
#include <stdint.h>

#define LT2_RB_SEGMENT_TABLE_OFFSET 0x1038u
#define LT2_RB_STRING_SEGMENT_TYPE 1u
#define LT2_RB_BITMAP_SEGMENT_TYPE 2u
#define LT2_RB_XM_SEGMENT_TYPE 8u

typedef struct Lt2RbSlice {
    const uint8_t *data;
    size_t size;
} Lt2RbSlice;

typedef struct Lt2RbSegment {
    uint32_t type;
    uint32_t offset;
    uint32_t next_offset;
    uint32_t count_or_flags;
} Lt2RbSegment;

typedef struct Lt2RbStringSegment {
    Lt2RbSegment header;
    uint32_t string_count;   /* observed 704 at 0x1044 */
    uint32_t data_offset;    /* observed 0x1050 */
    uint32_t data_size;      /* observed 0x6238 */
} Lt2RbStringSegment;

int lt2_rb_parse_segments(Lt2RbSlice rb, Lt2RbSegment *out_segments,
    size_t max_segments, size_t *out_count);
int lt2_rb_parse_string_segment(Lt2RbSlice rb,
    Lt2RbStringSegment *out_segment);
int lt2_rb_get_string(Lt2RbSlice rb, const Lt2RbStringSegment *segment,
    uint32_t index, Lt2RbSlice *out_string);

#endif /* LT2_LEMONADE2_RB_CONTAINER_H */
