/*
 * Lemonade2.rb offsets:
 * 0x00000000: ff ff 20 03 58 02 10 00 ...
 * 0x00001038: segment chain starts
 *
 * Segment chain:
 * - 0x001038 type 1  next 0x007288 count_or_flags 0x200
 * - 0x007288 type 2  next 0x1472D22 count_or_flags 233
 * - 0x1472D22 type 3  next 0x14F4ABA count_or_flags 22
 * - 0x14F4ABA type 4  next 0x14F92C8 count_or_flags 1
 * - 0x14F92C8 type 5  next 0x14FBC82 count_or_flags 4
 * - 0x14FBC82 type 6  next 0x14FBC8E count_or_flags 0
 * - 0x14FBC8E type 7  next 0x14FBC9A count_or_flags 0
 * - 0x14FBC9A type 8  next 0x1A98452 count_or_flags 15
 * - 0x1A98452 type 13 next 0x1A9845E count_or_flags 0
 * - 0x1A9845E type 11 next 0x1A9846A count_or_flags 0
 * - 0x1A9846A type 12 next 0x1A98476 count_or_flags 0
 * - 0x1A98476 type 10 next 0x1A9848E count_or_flags 1
 *
 * The type-1 string segment has an extra header:
 * - 0x1044: string_count = 704
 * - 0x1048: data_offset = 0x1050
 * - 0x104C: data_size = 0x6238
 * - 0x1050: first record, uint32 length + CP1252 bytes
 */
#include "formats/lemonade2_rb_container.h"

static int rb_range_ok(Lt2RbSlice rb, uint32_t offset, uint32_t length)
{
    return offset <= rb.size && length <= rb.size - offset;
}

static uint32_t rb_u32le(Lt2RbSlice rb, uint32_t offset, int *ok)
{
    const uint8_t *p;

    if (!rb_range_ok(rb, offset, 4)) {
        *ok = 0;
        return 0;
    }
    p = rb.data + offset;
    return (uint32_t)p[0] |
        ((uint32_t)p[1] << 8) |
        ((uint32_t)p[2] << 16) |
        ((uint32_t)p[3] << 24);
}

/* Rejects malformed backward/out-of-file next offsets. */
int lt2_rb_parse_segments(Lt2RbSlice rb, Lt2RbSegment *out_segments,
    size_t max_segments, size_t *out_count)
{
    uint32_t offset = LT2_RB_SEGMENT_TABLE_OFFSET;
    size_t count = 0;
    int ok = 1;

    if (out_count == NULL) {
        return 0;
    }
    *out_count = 0;

    while (rb_range_ok(rb, offset, 12)) {
        Lt2RbSegment segment;

        segment.type = rb_u32le(rb, offset, &ok);
        segment.next_offset = rb_u32le(rb, offset + 4, &ok);
        segment.count_or_flags = rb_u32le(rb, offset + 8, &ok);
        segment.offset = offset;
        if (!ok || segment.next_offset <= offset ||
            segment.next_offset > rb.size) {
            return 0;
        }

        if (out_segments != NULL && count < max_segments) {
            out_segments[count] = segment;
        }
        count++;
        offset = segment.next_offset;

        if (offset == rb.size) {
            break;
        }
    }

    *out_count = count;
    return count != 0;
}

/* Type-1 strings are length-prefixed CP1252 byte slices. */
int lt2_rb_parse_string_segment(Lt2RbSlice rb,
    Lt2RbStringSegment *out_segment)
{
    Lt2RbSegment segments[16];
    size_t count = 0;
    size_t i;
    int ok = 1;

    if (out_segment == NULL ||
        !lt2_rb_parse_segments(rb, segments, 16, &count)) {
        return 0;
    }

    for (i = 0; i < count && i < 16; i++) {
        if (segments[i].type == LT2_RB_STRING_SEGMENT_TYPE) {
            uint32_t string_count = rb_u32le(rb, segments[i].offset + 12, &ok);
            uint32_t data_offset = rb_u32le(rb, segments[i].offset + 16, &ok);
            uint32_t data_size = rb_u32le(rb, segments[i].offset + 20, &ok);

            if (!ok || data_offset < segments[i].offset ||
                data_offset + data_size != segments[i].next_offset ||
                !rb_range_ok(rb, data_offset, data_size)) {
                return 0;
            }

            out_segment->header = segments[i];
            out_segment->string_count = string_count;
            out_segment->data_offset = data_offset;
            out_segment->data_size = data_size;
            return 1;
        }
    }

    return 0;
}

/* Returned slices are not NUL-terminated unless the RB record includes it. */
int lt2_rb_get_string(Lt2RbSlice rb, const Lt2RbStringSegment *segment,
    uint32_t index, Lt2RbSlice *out_string)
{
    uint32_t cursor;
    uint32_t i;
    int ok = 1;

    if (segment == NULL || out_string == NULL ||
        index >= segment->string_count) {
        return 0;
    }

    cursor = segment->data_offset;
    for (i = 0; i < segment->string_count; i++) {
        uint32_t length = rb_u32le(rb, cursor, &ok);
        uint32_t data = cursor + 4;

        if (!ok || !rb_range_ok(rb, data, length) ||
            data + length > segment->data_offset + segment->data_size) {
            return 0;
        }

        if (i == index) {
            out_string->data = rb.data + data;
            out_string->size = length;
            return 1;
        }

        cursor = data + length;
    }

    return 0;
}
