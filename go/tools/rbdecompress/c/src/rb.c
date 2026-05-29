#include "lt2rb/rb.h"

#include "lt2rb/constants.h"
#include "lt2rb/error.h"

#include <png.h>
#include <errno.h>
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

static int find_segment(Lt2rbBuffer rb, uint32_t type, RbSegment *out)
{
    size_t offset = LT2RB_SEGMENT_TABLE_OFFSET;

    if (!lt2rb_range_ok(rb.size, offset, 12)) {
        return lt2rb_set_error("rb input is too small for the segment table");
    }

    for (;;) {
        RbSegment segment;

        if (!lt2rb_range_ok(rb.size, offset, 12)) {
            return lt2rb_set_error("truncated segment header at 0x%zX", offset);
        }
        segment.type = lt2rb_u32le(rb.data + offset);
        segment.next_offset = lt2rb_u32le(rb.data + offset + 4);
        segment.count = lt2rb_u32le(rb.data + offset + 8);
        segment.offset = offset;
        if (segment.next_offset <= offset || segment.next_offset > rb.size) {
            return lt2rb_set_error("bad segment chain at 0x%zX: next offset 0x%zX",
                offset, segment.next_offset);
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

int lt2rb_extract_bitmap_pngs(Lt2rbBuffer rb, const char *output_dir,
    bool transparency, size_t *out_count)
{
    RbSegment segment;
    size_t offset;
    size_t parsed = 0;

    if (!find_segment(rb, LT2RB_BITMAP_SEGMENT_TYPE, &segment) || !lt2rb_mkdir_all(output_dir)) {
        return 0;
    }

    offset = segment.offset + 12;
    while (offset < segment.next_offset) {
        uint16_t width;
        uint16_t height;
        uint32_t format;
        uint32_t flags;
        size_t bpp;
        size_t pixels;
        size_t data_size;
        size_t data_start;
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
            !lt2rb_checked_mul_size(pixels, bpp, &data_size)) {
            return 0;
        }
        data_start = offset + 12;
        if (data_size > segment.next_offset - data_start) {
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

        offset = data_start + data_size;
        parsed++;
    }

    if (parsed != segment.count) {
        return lt2rb_set_error("bitmap count mismatch: parsed %zu, segment says %u",
            parsed, segment.count);
    }
    *out_count = parsed;
    return 1;
}
