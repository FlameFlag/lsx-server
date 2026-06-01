#include "lt2rb/rb.h"

#include "lt2rb/constants.h"
#include "lt2rb/error.h"

#include <png.h>
#include <zlib.h>
#include <errno.h>
#include <limits.h>
#include <setjmp.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

typedef struct RbSegment {
    uint32_t type;
    size_t offset;
    size_t next_offset;
    uint32_t count;
} RbSegment;

typedef struct Color {
    uint8_t r;
    uint8_t g;
    uint8_t b;
    uint8_t a;
} Color;

static int checked_add_size(size_t a, size_t b, size_t *out)
{
    if (b > SIZE_MAX - a) {
        return 0;
    }
    *out = a + b;
    return 1;
}

static int checked_u32_to_size(uint32_t value, size_t *out)
{
    *out = (size_t)value;
    return (uint32_t)*out == value;
}

static int read_segment(Lt2rbBuffer rb, size_t offset, RbSegment *out)
{
    uint32_t next32;
    size_t next;

    if (!lt2rb_range_ok(rb.size, offset, LT2RB_SEGMENT_HEADER_SIZE)) {
        return lt2rb_set_error("truncated segment header at 0x%zX", offset);
    }
    next32 = lt2rb_u32le(rb.data + offset + 4);
    if (!checked_u32_to_size(next32, &next)) {
        return lt2rb_set_error("segment next offset is too large at 0x%zX", offset);
    }
    if (next <= offset || next > rb.size) {
        return lt2rb_set_error("bad segment chain at 0x%zX: next offset 0x%zX",
            offset, next);
    }

    out->type = lt2rb_u32le(rb.data + offset);
    out->next_offset = next;
    out->count = lt2rb_u32le(rb.data + offset + 8);
    out->offset = offset;
    return 1;
}

static int find_segment(Lt2rbBuffer rb, uint32_t type, RbSegment *out)
{
    size_t offset = LT2RB_SEGMENT_TABLE_OFFSET;

    if (!lt2rb_range_ok(rb.size, offset, LT2RB_SEGMENT_HEADER_SIZE)) {
        return lt2rb_set_error("rb input is too small for the segment table");
    }

    for (;;) {
        RbSegment segment;

        if (!read_segment(rb, offset, &segment)) {
            return 0;
        }
        if (segment.type == type) {
            *out = segment;
            return 1;
        }
        offset = segment.next_offset;
        if (offset == rb.size) {
            break;
        }
    }
    return lt2rb_set_error("segment type %u not found", type);
}

static int segment_payload(RbSegment segment, size_t *out_offset, size_t *out_size)
{
    size_t start;

    if (!checked_add_size(segment.offset, LT2RB_SEGMENT_HEADER_SIZE, &start) ||
        start > segment.next_offset) {
        return lt2rb_set_error("bad segment payload bounds at 0x%zX", segment.offset);
    }
    *out_offset = start;
    *out_size = segment.next_offset - start;
    return 1;
}

static int bytes_per_pixel(uint32_t format, size_t *out)
{
    switch (format & 0xFFFFu) {
    case LT2RB_FORMAT_RGB565:
        *out = 2;
        return 1;
    case LT2RB_FORMAT_RGB565_MASK:
        *out = 3;
        return 1;
    case LT2RB_FORMAT_GRAY8:
        *out = 1;
        return 1;
    default:
        return lt2rb_set_error("unknown bitmap format 0x%X", format);
    }
}

static void bitmap_format_name(uint32_t format, char *out, size_t out_size)
{
    switch (format & 0xFFFFu) {
    case LT2RB_FORMAT_RGB565:
        snprintf(out, out_size, "rgb565");
        break;
    case LT2RB_FORMAT_RGB565_MASK:
        snprintf(out, out_size, "rgb565a8");
        break;
    case LT2RB_FORMAT_GRAY8:
        snprintf(out, out_size, "gray8");
        break;
    default:
        snprintf(out, out_size, "fmt%X", format);
        break;
    }
}

static Color color_from_rgb565(uint16_t value)
{
    uint16_t r = (value >> 11) & 0x1F;
    uint16_t g = (value >> 5) & 0x3F;
    uint16_t b = value & 0x1F;
    return (Color){
        .r = (uint8_t)((r << 3) | (r >> 2)),
        .g = (uint8_t)((g << 2) | (g >> 4)),
        .b = (uint8_t)((b << 3) | (b >> 2)),
        .a = 255,
    };
}

static Color color_from_rgb_flag(uint32_t value)
{
    return (Color){
        .r = (uint8_t)(value >> 16),
        .g = (uint8_t)(value >> 8),
        .b = (uint8_t)value,
        .a = 255,
    };
}

static bool same_rgb(Color left, Color right)
{
    return left.r == right.r && left.g == right.g && left.b == right.b;
}

static void set_rgba(uint8_t *pixels, size_t pixel, Color color)
{
    size_t base = pixel * 4;
    pixels[base] = color.r;
    pixels[base + 1] = color.g;
    pixels[base + 2] = color.b;
    pixels[base + 3] = color.a;
}

static int decode_bitmap(uint16_t width, uint16_t height, uint32_t format,
    uint32_t flags, const uint8_t *data, bool transparency, uint8_t **out_pixels)
{
    size_t pixel_count;
    size_t pixel_bytes;
    uint8_t *pixels;

    if (!lt2rb_checked_mul_size(width, height, &pixel_count) ||
        !lt2rb_checked_mul_size(pixel_count, 4, &pixel_bytes)) {
        return lt2rb_set_error("bitmap dimensions are too large");
    }
    pixels = malloc(pixel_bytes == 0 ? 1 : pixel_bytes);
    if (pixels == NULL) {
        return lt2rb_set_error("out of memory");
    }

    switch (format & 0xFFFFu) {
    case LT2RB_FORMAT_RGB565_MASK:
        for (size_t i = 0; i < pixel_count; i++) {
            size_t base = i * 3;
            Color color = color_from_rgb565(lt2rb_u16le(data + base));
            if (transparency) {
                color.a = data[base + 2];
            }
            set_rgba(pixels, i, color);
        }
        break;
    case LT2RB_FORMAT_RGB565: {
        Color key = color_from_rgb_flag(flags);
        for (size_t i = 0; i < pixel_count; i++) {
            size_t base = i * 2;
            Color color = color_from_rgb565(lt2rb_u16le(data + base));
            if (transparency && flags != 0 && same_rgb(color, key)) {
                color.a = 0;
            }
            set_rgba(pixels, i, color);
        }
        break;
    }
    case LT2RB_FORMAT_GRAY8:
        for (size_t i = 0; i < pixel_count; i++) {
            uint8_t value = data[i];
            set_rgba(pixels, i, (Color){.r = value, .g = value, .b = value, .a = 255});
        }
        break;
    default:
        free(pixels);
        return lt2rb_set_error("unknown bitmap format 0x%X", format);
    }

    *out_pixels = pixels;
    return 1;
}

static int write_png_rgba(const char *path, const uint8_t *pixels,
    uint16_t width, uint16_t height)
{
    FILE *file = fopen(path, "wb");
    png_structp png = NULL;
    png_infop info = NULL;
    png_bytep *rows = NULL;
    int result = 0;

    if (file == NULL) {
        return lt2rb_set_error("create %s: %s", path, strerror(errno));
    }
    png = png_create_write_struct(PNG_LIBPNG_VER_STRING, NULL, NULL, NULL);
    if (png == NULL) {
        lt2rb_set_error("create png writer: out of memory");
        goto done;
    }
    info = png_create_info_struct(png);
    if (info == NULL) {
        lt2rb_set_error("create png info: out of memory");
        goto done;
    }
    if (setjmp(png_jmpbuf(png))) {
        lt2rb_set_error("encode %s: libpng error", path);
        goto done;
    }

    rows = malloc((size_t)height * sizeof(rows[0]));
    if (rows == NULL) {
        lt2rb_set_error("out of memory");
        goto done;
    }
    for (uint16_t y = 0; y < height; y++) {
        rows[y] = (png_bytep)(pixels + (size_t)y * width * 4);
    }

    png_init_io(png, file);
    png_set_IHDR(png, info, width, height, 8, PNG_COLOR_TYPE_RGBA,
        PNG_INTERLACE_NONE, PNG_COMPRESSION_TYPE_DEFAULT, PNG_FILTER_TYPE_DEFAULT);
    png_write_info(png, info);
    png_write_image(png, rows);
    png_write_end(png, info);
    result = 1;

done:
    free(rows);
    if (png != NULL) {
        png_destroy_write_struct(&png, &info);
    }
    if (fclose(file) != 0 && result) {
        return lt2rb_set_error("close %s: %s", path, strerror(errno));
    }
    return result;
}

static int validate_preamble(Lt2rbBuffer rb)
{
    if (!lt2rb_range_ok(rb.size, 0, LT2RB_SEGMENT_TABLE_OFFSET)) {
        return lt2rb_set_error("rb input is too small for the preamble");
    }
    if (lt2rb_u16le(rb.data) != 0xFFFFu ||
        lt2rb_u16le(rb.data + 2) != LT2RB_PREAMBLE_WIDTH ||
        lt2rb_u16le(rb.data + 4) != LT2RB_PREAMBLE_HEIGHT ||
        lt2rb_u16le(rb.data + 6) != LT2RB_PREAMBLE_DEPTH ||
        lt2rb_u32le(rb.data + 8) != LT2RB_PREAMBLE_SIGNATURE ||
        lt2rb_u32le(rb.data + 12) != 0 ||
        lt2rb_u32le(rb.data + 16) != LT2RB_SEGMENT_TABLE_OFFSET ||
        lt2rb_u32le(rb.data + 20) != LT2RB_PREAMBLE_DWORD_COUNT) {
        return lt2rb_set_error("rb preamble does not match Lemonade2.rb header");
    }
    return 1;
}

static int validate_string_segment(Lt2rbBuffer rb, RbSegment segment)
{
    size_t payload_offset;
    size_t payload_size;
    uint32_t string_count;
    uint32_t data_offset32;
    uint32_t data_size32;
    size_t data_offset;
    size_t data_size;
    size_t data_end;
    size_t cursor;

    if (!segment_payload(segment, &payload_offset, &payload_size)) {
        return 0;
    }
    if (payload_size < 12) {
        return lt2rb_set_error("string segment at 0x%zX is too small", segment.offset);
    }
    string_count = lt2rb_u32le(rb.data + payload_offset);
    data_offset32 = lt2rb_u32le(rb.data + payload_offset + 4);
    data_size32 = lt2rb_u32le(rb.data + payload_offset + 8);
    if (!checked_u32_to_size(data_offset32, &data_offset) ||
        !checked_u32_to_size(data_size32, &data_size) ||
        !checked_add_size(data_offset, data_size, &data_end)) {
        return lt2rb_set_error("string segment at 0x%zX has oversized offsets", segment.offset);
    }
    if (data_offset < payload_offset + 12 || data_end != segment.next_offset ||
        !lt2rb_range_ok(rb.size, data_offset, data_size)) {
        return lt2rb_set_error("string segment at 0x%zX has bad data bounds", segment.offset);
    }

    cursor = data_offset;
    for (uint32_t i = 0; i < string_count; i++) {
        uint32_t len32;
        size_t len;
        size_t data_start;
        size_t next;

        if (!lt2rb_range_ok(data_end, cursor, 4)) {
            return lt2rb_set_error("truncated string %u at 0x%zX", i, cursor);
        }
        len32 = lt2rb_u32le(rb.data + cursor);
        if (!checked_u32_to_size(len32, &len) ||
            !checked_add_size(cursor, 4, &data_start) ||
            !checked_add_size(data_start, len, &next) || next > data_end) {
            return lt2rb_set_error("string %u at 0x%zX exceeds string segment", i, cursor);
        }
        cursor = next;
    }
    if (cursor != data_end) {
        return lt2rb_set_error("string segment has %zu trailing byte(s)", data_end - cursor);
    }
    return 1;
}

static int validate_bitmap_records(Lt2rbBuffer rb, RbSegment segment)
{
    size_t offset;
    size_t parsed = 0;

    if (!segment_payload(segment, &offset, &(size_t){0})) {
        return 0;
    }
    while (offset < segment.next_offset) {
        uint16_t width;
        uint16_t height;
        uint32_t format;
        size_t bpp;
        size_t pixels;
        size_t data_size;
        size_t data_start;
        size_t data_end;

        if (!lt2rb_range_ok(segment.next_offset, offset, 12)) {
            return lt2rb_set_error("truncated bitmap header at 0x%zX", offset);
        }
        width = lt2rb_u16le(rb.data + offset);
        height = lt2rb_u16le(rb.data + offset + 2);
        format = lt2rb_u32le(rb.data + offset + 4);
        if (width == 0 || height == 0) {
            return lt2rb_set_error("bad bitmap dimensions %ux%u at 0x%zX",
                width, height, offset);
        }
        if (!bytes_per_pixel(format, &bpp) ||
            !lt2rb_checked_mul_size(width, height, &pixels) ||
            !lt2rb_checked_mul_size(pixels, bpp, &data_size) ||
            !checked_add_size(offset, 12, &data_start) ||
            !checked_add_size(data_start, data_size, &data_end)) {
            return 0;
        }
        if (data_end > segment.next_offset) {
            return lt2rb_set_error("bitmap %zu at 0x%zX exceeds segment bounds", parsed, offset);
        }
        parsed++;
        offset = data_end;
    }
    if (parsed != segment.count) {
        return lt2rb_set_error("bitmap count mismatch: parsed %zu, segment says %u",
            parsed, segment.count);
    }
    return 1;
}

static int looks_like_pcm_header(Lt2rbBuffer rb, size_t offset, size_t end)
{
    return lt2rb_range_ok(end, offset, 8) &&
        lt2rb_u32le(rb.data + offset) == LT2RB_PCM_SAMPLE_RATE &&
        lt2rb_u32le(rb.data + offset + 4) == LT2RB_PCM_TAG;
}

static int validate_pcm_segment(Lt2rbBuffer rb, RbSegment segment)
{
    size_t cursor;
    size_t payload_size;
    size_t samples = 0;

    if (!segment_payload(segment, &cursor, &payload_size)) {
        return 0;
    }
    if (payload_size == 0) {
        return lt2rb_set_error("pcm segment at 0x%zX is empty", segment.offset);
    }
    while (cursor < segment.next_offset) {
        size_t payload_start;
        size_t next = segment.next_offset;

        if (!looks_like_pcm_header(rb, cursor, segment.next_offset)) {
            return lt2rb_set_error("pcm record %zu missing header at 0x%zX", samples, cursor);
        }
        payload_start = cursor + 8;
        for (size_t probe = payload_start; probe + 8 <= segment.next_offset; probe += 2) {
            if (looks_like_pcm_header(rb, probe, segment.next_offset)) {
                next = probe;
                break;
            }
        }
        if (((next - payload_start) & 1u) != 0) {
            return lt2rb_set_error("pcm record %zu has odd byte length", samples);
        }
        samples++;
        cursor = next;
    }
    if (samples == 0) {
        return lt2rb_set_error("pcm segment at 0x%zX has no records", segment.offset);
    }
    return 1;
}

static int validate_image_info_segment(Lt2rbBuffer rb, RbSegment segment)
{
    size_t payload_offset;
    size_t payload_size;
    uint16_t max_handle;
    size_t table_bytes;
    (void)rb;

    if (!segment_payload(segment, &payload_offset, &payload_size)) {
        return 0;
    }
    if (payload_size < 2) {
        return lt2rb_set_error("image-info segment at 0x%zX is too small", segment.offset);
    }
    max_handle = lt2rb_u16le(rb.data + payload_offset);
    if (!lt2rb_checked_mul_size(max_handle, LT2RB_IMAGE_INFO_RECORD_SIZE, &table_bytes) ||
        table_bytes != payload_size - 2) {
        return lt2rb_set_error("image-info segment at 0x%zX has bad table size", segment.offset);
    }
    return 1;
}

static int validate_glyph_segment(Lt2rbBuffer rb, RbSegment segment)
{
    size_t cursor;
    size_t payload_size;

    if (!segment_payload(segment, &cursor, &payload_size)) {
        return 0;
    }
    (void)payload_size;
    for (uint32_t i = 0; i < segment.count; i++) {
        uint16_t glyph_count;
        size_t glyph_bytes;
        size_t next;

        if (!lt2rb_range_ok(segment.next_offset, cursor, LT2RB_GLYPH_TABLE_HEADER_SIZE)) {
            return lt2rb_set_error("glyph table %u truncated at 0x%zX", i, cursor);
        }
        glyph_count = lt2rb_u16le(rb.data + cursor + 2);
        if (!lt2rb_checked_mul_size(glyph_count, LT2RB_GLYPH_RECORD_SIZE, &glyph_bytes) ||
            !checked_add_size(cursor, LT2RB_GLYPH_TABLE_HEADER_SIZE, &next) ||
            !checked_add_size(next, glyph_bytes, &next) || next > segment.next_offset) {
            return lt2rb_set_error("glyph table %u at 0x%zX exceeds segment bounds", i, cursor);
        }
        cursor = next;
    }
    if (cursor != segment.next_offset) {
        return lt2rb_set_error("glyph segment has %zu trailing byte(s)", segment.next_offset - cursor);
    }
    return 1;
}

static int validate_zlib_screen_record(const uint8_t *record, size_t size, size_t record_index)
{
    size_t cursor;
    uint32_t max_handle;
    uint32_t entries = 0;

    if (size < LT2RB_ZLIB_SCREEN_HEADER_SIZE) {
        return lt2rb_set_error("type-8 record %zu too small for zlib screen", record_index);
    }
    if (lt2rb_u32le(record) != 1 || lt2rb_u32le(record + 4) != 1 ||
        lt2rb_u32le(record + 8) != LT2RB_PREAMBLE_WIDTH ||
        lt2rb_u32le(record + 12) != LT2RB_PREAMBLE_HEIGHT ||
        lt2rb_u32le(record + 16) != 4) {
        return lt2rb_set_error("type-8 record %zu zlib screen header mismatch", record_index);
    }
    max_handle = lt2rb_u32le(record + 20);
    if (max_handle == 0) {
        return lt2rb_set_error("type-8 record %zu has invalid max handle", record_index);
    }
    for (size_t i = 24; i < LT2RB_ZLIB_SCREEN_HEADER_SIZE; i += 4) {
        if (lt2rb_u32le(record + i) != 0) {
            return lt2rb_set_error("type-8 record %zu has nonzero screen reserved field", record_index);
        }
    }

    cursor = LT2RB_ZLIB_SCREEN_HEADER_SIZE;
    while (cursor < size) {
        uint32_t type;
        uint32_t handle;
        uint32_t width;
        uint32_t height;
        uint32_t total_size;
        size_t data_header;
        size_t data_start;
        size_t comp_len;
        size_t raw_len;
        size_t raw_expected;
        size_t data_end;
        size_t aligned_end;
        uint8_t *raw;
        uLongf raw_dest_len;
        int zerr;

        if (!lt2rb_range_ok(size, cursor, LT2RB_ZLIB_ENTRY_HEADER_SIZE)) {
            return lt2rb_set_error("type-8 record %zu has truncated zlib entry", record_index);
        }
        type = lt2rb_u32le(record + cursor);
        handle = lt2rb_u32le(record + cursor + 4);
        width = lt2rb_u32le(record + cursor + 16);
        height = lt2rb_u32le(record + cursor + 20);
        total_size = lt2rb_u32le(record + cursor + 24);
        if (type != 2 || handle == 0 || handle >= max_handle) {
            return lt2rb_set_error("type-8 record %zu bad zlib entry header", record_index);
        }
        for (size_t i = cursor + 28; i < cursor + LT2RB_ZLIB_ENTRY_HEADER_SIZE; i += 4) {
            if (lt2rb_u32le(record + i) != 0) {
                return lt2rb_set_error("type-8 record %zu has nonzero zlib entry reserved field", record_index);
            }
        }
        entries++;
        cursor += LT2RB_ZLIB_ENTRY_HEADER_SIZE;
        if (total_size == 0) {
            continue;
        }
        if (total_size < 12 || !lt2rb_range_ok(size, cursor, 12)) {
            return lt2rb_set_error("type-8 record %zu has bad zlib payload header", record_index);
        }
        comp_len = lt2rb_u32le(record + cursor);
        raw_len = lt2rb_u32le(record + cursor + 4);
        if (lt2rb_u32le(record + cursor + 8) != LT2RB_ZLIB_CHECKSUM_OR_KEY) {
            return lt2rb_set_error("type-8 record %zu has bad zlib checksum/key", record_index);
        }
        if (!lt2rb_checked_mul_size(width, height, &raw_expected) ||
            !lt2rb_checked_mul_size(raw_expected, 2, &raw_expected) ||
            raw_len != raw_expected || comp_len != (size_t)total_size - 12) {
            return lt2rb_set_error("type-8 record %zu has inconsistent zlib lengths", record_index);
        }
        data_header = cursor;
        if (!checked_add_size(data_header, 12, &data_start) ||
            !checked_add_size(data_start, comp_len, &data_end) || data_end > size ||
            !checked_add_size(data_header, ((size_t)total_size + 3u) & ~(size_t)3u, &aligned_end) ||
            aligned_end > size) {
            return lt2rb_set_error("type-8 record %zu zlib payload exceeds record", record_index);
        }
        for (size_t i = data_end; i < aligned_end; i++) {
            if (record[i] != 0) {
                return lt2rb_set_error("type-8 record %zu has nonzero zlib padding", record_index);
            }
        }
        raw = malloc(raw_len == 0 ? 1 : raw_len);
        if (raw == NULL) {
            return lt2rb_set_error("out of memory");
        }
        raw_dest_len = (uLongf)raw_len;
        if ((size_t)raw_dest_len != raw_len) {
            free(raw);
            return lt2rb_set_error("type-8 record %zu raw zlib payload too large", record_index);
        }
        zerr = uncompress(raw, &raw_dest_len, record + data_start, (uLong)comp_len);
        free(raw);
        if (zerr != Z_OK || (size_t)raw_dest_len != raw_len) {
            return lt2rb_set_error("type-8 record %zu zlib inflate failed: %d", record_index, zerr);
        }
        cursor = aligned_end;
    }
    if (entries + 1 != max_handle) {
        return lt2rb_set_error("type-8 record %zu entry count mismatch: parsed %u, max handle %u",
            record_index, entries, max_handle);
    }
    return 1;
}

static int validate_blob_segment(Lt2rbBuffer rb, RbSegment segment)
{
    size_t cursor;
    size_t payload_size;

    if (!segment_payload(segment, &cursor, &payload_size)) {
        return 0;
    }
    (void)payload_size;
    for (uint32_t i = 0; i < segment.count; i++) {
        uint32_t len32;
        size_t len;
        size_t data_start;
        size_t data_end;

        if (!lt2rb_range_ok(segment.next_offset, cursor, 4)) {
            return lt2rb_set_error("type-8 record %u length truncated at 0x%zX", i, cursor);
        }
        len32 = lt2rb_u32le(rb.data + cursor);
        if (!checked_u32_to_size(len32, &len) ||
            !checked_add_size(cursor, 4, &data_start) ||
            !checked_add_size(data_start, len, &data_end) || data_end > segment.next_offset) {
            return lt2rb_set_error("type-8 record %u at 0x%zX exceeds segment bounds", i, cursor);
        }
        if (i == 1 && len >= LT2RB_ZLIB_SCREEN_HEADER_SIZE &&
            lt2rb_u32le(rb.data + data_start) == 1 &&
            !validate_zlib_screen_record(rb.data + data_start, len, i)) {
            return 0;
        }
        cursor = data_end;
    }
    if (cursor != segment.next_offset) {
        return lt2rb_set_error("type-8 segment has %zu trailing byte(s)", segment.next_offset - cursor);
    }
    return 1;
}

static int validate_empty_segment(RbSegment segment)
{
    size_t payload_offset;
    size_t payload_size;

    if (!segment_payload(segment, &payload_offset, &payload_size)) {
        return 0;
    }
    (void)payload_offset;
    if (payload_size != 0 || segment.count != 0) {
        return lt2rb_set_error("segment type %u at 0x%zX should be empty",
            segment.type, segment.offset);
    }
    return 1;
}

static int validate_footer_segment(Lt2rbBuffer rb, RbSegment segment)
{
    size_t payload_offset;
    size_t payload_size;

    if (!segment_payload(segment, &payload_offset, &payload_size)) {
        return 0;
    }
    if (segment.count != 1 || payload_size != 12) {
        return lt2rb_set_error("footer segment at 0x%zX has bad size/count", segment.offset);
    }
    if (rb.size > UINT32_MAX) {
        return lt2rb_set_error("rb input too large for 32-bit footer");
    }
    if (lt2rb_u32le(rb.data + payload_offset) != 0 ||
        lt2rb_u32le(rb.data + payload_offset + 4) != (uint32_t)rb.size ||
        lt2rb_u32le(rb.data + payload_offset + 8) != 0) {
        return lt2rb_set_error("footer segment at 0x%zX has bad payload", segment.offset);
    }
    return 1;
}

static int validate_segment_payload(Lt2rbBuffer rb, RbSegment segment)
{
    switch (segment.type) {
    case LT2RB_STRING_SEGMENT_TYPE:
        return validate_string_segment(rb, segment);
    case LT2RB_BITMAP_SEGMENT_TYPE:
        return validate_bitmap_records(rb, segment);
    case LT2RB_PCM_SEGMENT_TYPE:
        return validate_pcm_segment(rb, segment);
    case LT2RB_IMAGE_INFO_SEGMENT_TYPE:
        return validate_image_info_segment(rb, segment);
    case LT2RB_GLYPH_SEGMENT_TYPE:
        return validate_glyph_segment(rb, segment);
    case LT2RB_EMPTY_SEGMENT_6:
    case LT2RB_EMPTY_SEGMENT_7:
    case LT2RB_EMPTY_SEGMENT_11:
    case LT2RB_EMPTY_SEGMENT_12:
    case LT2RB_EMPTY_SEGMENT_13:
        return validate_empty_segment(segment);
    case LT2RB_BLOB_SEGMENT_TYPE:
        return validate_blob_segment(rb, segment);
    case LT2RB_FOOTER_SEGMENT_TYPE:
        return validate_footer_segment(rb, segment);
    default:
        return lt2rb_set_error("unknown rb segment type %u at 0x%zX",
            segment.type, segment.offset);
    }
}

int lt2rb_validate_rb(Lt2rbBuffer rb)
{
    size_t offset = LT2RB_SEGMENT_TABLE_OFFSET;
    size_t count = 0;

    if (!validate_preamble(rb)) {
        return 0;
    }
    if (!lt2rb_range_ok(rb.size, offset, LT2RB_SEGMENT_HEADER_SIZE)) {
        return lt2rb_set_error("rb input is too small for the segment table");
    }
    for (;;) {
        RbSegment segment;

        if (!read_segment(rb, offset, &segment) ||
            !validate_segment_payload(rb, segment)) {
            return 0;
        }
        count++;
        offset = segment.next_offset;
        if (offset == rb.size) {
            break;
        }
    }
    if (count == 0) {
        return lt2rb_set_error("rb contains no segments");
    }
    return 1;
}

int lt2rb_extract_bitmap_pngs(Lt2rbBuffer rb, const char *output_dir,
    bool transparency, size_t *out_count)
{
    RbSegment segment;
    size_t offset;
    size_t parsed = 0;

    if (!find_segment(rb, LT2RB_BITMAP_SEGMENT_TYPE, &segment) || !lt2rb_mkdir_all(output_dir)) {
        return 0;
    }

    offset = segment.offset + LT2RB_SEGMENT_HEADER_SIZE;
    while (offset < segment.next_offset) {
        uint16_t width;
        uint16_t height;
        uint32_t format;
        uint32_t flags;
        size_t bpp;
        size_t pixels;
        size_t data_size;
        size_t data_start;
        size_t data_end;
        uint8_t *rgba = NULL;
        char format_name[32];
        char filename[128];
        char *path;

        if (!lt2rb_range_ok(segment.next_offset, offset, 12)) {
            return lt2rb_set_error("truncated bitmap header at 0x%zX", offset);
        }
        width = lt2rb_u16le(rb.data + offset);
        height = lt2rb_u16le(rb.data + offset + 2);
        format = lt2rb_u32le(rb.data + offset + 4);
        flags = lt2rb_u32le(rb.data + offset + 8);
        if (width == 0 || height == 0) {
            return lt2rb_set_error("bad bitmap dimensions %ux%u at 0x%zX",
                width, height, offset);
        }
        if (!bytes_per_pixel(format, &bpp) ||
            !lt2rb_checked_mul_size(width, height, &pixels) ||
            !lt2rb_checked_mul_size(pixels, bpp, &data_size) ||
            !checked_add_size(offset, 12, &data_start) ||
            !checked_add_size(data_start, data_size, &data_end)) {
            return 0;
        }
        if (data_end > segment.next_offset) {
            return lt2rb_set_error("bitmap %zu at 0x%zX exceeds segment bounds", parsed, offset);
        }

        if (!decode_bitmap(width, height, format, flags, rb.data + data_start,
                transparency, &rgba)) {
            return 0;
        }
        bitmap_format_name(format, format_name, sizeof(format_name));
        snprintf(filename, sizeof(filename), "bitmap_%03zu_%ux%u_%s.png",
            parsed, width, height, format_name);
        path = lt2rb_join_path(output_dir, filename);
        if (path == NULL) {
            free(rgba);
            return 0;
        }
        if (!write_png_rgba(path, rgba, width, height)) {
            free(path);
            free(rgba);
            return 0;
        }
        free(path);
        free(rgba);

        offset = data_end;
        parsed++;
    }

    if (parsed != segment.count) {
        return lt2rb_set_error("bitmap count mismatch: parsed %zu, segment says %u",
            parsed, segment.count);
    }
    *out_count = parsed;
    return 1;
}
