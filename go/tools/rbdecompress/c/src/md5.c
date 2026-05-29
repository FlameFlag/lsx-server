#include "lt2rb/md5.h"

#include <string.h>

static uint32_t load_u32le(const uint8_t *p)
{
    return (uint32_t)p[0] | ((uint32_t)p[1] << 8) |
        ((uint32_t)p[2] << 16) | ((uint32_t)p[3] << 24);
}

static uint32_t rotl(uint32_t x, uint32_t n)
{
    return (x << n) | (x >> (32u - n));
}

static void transform(Lt2rbMd5 *md5, const uint8_t block[64])
{
    static const uint32_t s[64] = {
        7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22,
        5, 9, 14, 20, 5, 9, 14, 20, 5, 9, 14, 20, 5, 9, 14, 20,
        4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23,
        6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15, 21,
    };
    static const uint32_t k[64] = {
        0xd76aa478, 0xe8c7b756, 0x242070db, 0xc1bdceee,
        0xf57c0faf, 0x4787c62a, 0xa8304613, 0xfd469501,
        0x698098d8, 0x8b44f7af, 0xffff5bb1, 0x895cd7be,
        0x6b901122, 0xfd987193, 0xa679438e, 0x49b40821,
        0xf61e2562, 0xc040b340, 0x265e5a51, 0xe9b6c7aa,
        0xd62f105d, 0x02441453, 0xd8a1e681, 0xe7d3fbc8,
        0x21e1cde6, 0xc33707d6, 0xf4d50d87, 0x455a14ed,
        0xa9e3e905, 0xfcefa3f8, 0x676f02d9, 0x8d2a4c8a,
        0xfffa3942, 0x8771f681, 0x6d9d6122, 0xfde5380c,
        0xa4beea44, 0x4bdecfa9, 0xf6bb4b60, 0xbebfbc70,
        0x289b7ec6, 0xeaa127fa, 0xd4ef3085, 0x04881d05,
        0xd9d4d039, 0xe6db99e5, 0x1fa27cf8, 0xc4ac5665,
        0xf4292244, 0x432aff97, 0xab9423a7, 0xfc93a039,
        0x655b59c3, 0x8f0ccc92, 0xffeff47d, 0x85845dd1,
        0x6fa87e4f, 0xfe2ce6e0, 0xa3014314, 0x4e0811a1,
        0xf7537e82, 0xbd3af235, 0x2ad7d2bb, 0xeb86d391,
    };
    uint32_t m[16];
    uint32_t a = md5->h[0];
    uint32_t b = md5->h[1];
    uint32_t c = md5->h[2];
    uint32_t d = md5->h[3];

    for (size_t i = 0; i < 16; i++) {
        m[i] = load_u32le(block + i * 4);
    }
    for (uint32_t i = 0; i < 64; i++) {
        uint32_t f;
        uint32_t g;
        uint32_t old_d = d;

        if (i < 16) {
            f = (b & c) | (~b & d);
            g = i;
        } else if (i < 32) {
            f = (d & b) | (~d & c);
            g = (5 * i + 1) & 15;
        } else if (i < 48) {
            f = b ^ c ^ d;
            g = (3 * i + 5) & 15;
        } else {
            f = c ^ (b | ~d);
            g = (7 * i) & 15;
        }
        d = c;
        c = b;
        b += rotl(a + f + k[i] + m[g], s[i]);
        a = old_d;
    }
    md5->h[0] += a;
    md5->h[1] += b;
    md5->h[2] += c;
    md5->h[3] += d;
}

void lt2rb_md5_init(Lt2rbMd5 *md5)
{
    md5->h[0] = 0x67452301;
    md5->h[1] = 0xefcdab89;
    md5->h[2] = 0x98badcfe;
    md5->h[3] = 0x10325476;
    md5->length = 0;
    md5->buffer_len = 0;
}

void lt2rb_md5_update(Lt2rbMd5 *md5, const uint8_t *data, size_t len)
{
    md5->length += (uint64_t)len * 8u;
    while (len > 0) {
        size_t copy_len = 64 - md5->buffer_len;
        if (copy_len > len) {
            copy_len = len;
        }
        memcpy(md5->buffer + md5->buffer_len, data, copy_len);
        md5->buffer_len += copy_len;
        data += copy_len;
        len -= copy_len;
        if (md5->buffer_len == 64) {
            transform(md5, md5->buffer);
            md5->buffer_len = 0;
        }
    }
}

void lt2rb_md5_final(Lt2rbMd5 *md5, uint8_t out[16])
{
    uint64_t bit_length = md5->length;
    uint8_t pad[64] = {0x80};
    uint8_t length_bytes[8];
    size_t pad_len = md5->buffer_len < 56 ? 56 - md5->buffer_len : 120 - md5->buffer_len;

    for (size_t i = 0; i < 8; i++) {
        length_bytes[i] = (uint8_t)(bit_length >> (8 * i));
    }
    lt2rb_md5_update(md5, pad, pad_len);
    lt2rb_md5_update(md5, length_bytes, sizeof(length_bytes));
    for (size_t i = 0; i < 4; i++) {
        out[i * 4] = (uint8_t)md5->h[i];
        out[i * 4 + 1] = (uint8_t)(md5->h[i] >> 8);
        out[i * 4 + 2] = (uint8_t)(md5->h[i] >> 16);
        out[i * 4 + 3] = (uint8_t)(md5->h[i] >> 24);
    }
}

void lt2rb_print_md5(FILE *stream, const uint8_t digest[16])
{
    for (size_t i = 0; i < 16; i++) {
        fprintf(stream, "%02x", digest[i]);
    }
}
