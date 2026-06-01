#ifndef LT2RB_MD5_H
#define LT2RB_MD5_H

#include <stddef.h>
#include <stdint.h>
#include <stdio.h>

typedef struct Lt2rbMd5 {
    uint32_t h[4];
    uint64_t length;
    uint8_t buffer[64];
    size_t buffer_len;
} Lt2rbMd5;

void lt2rb_md5_init(Lt2rbMd5 *md5);
void lt2rb_md5_update(Lt2rbMd5 *md5, const uint8_t *data, size_t len);
void lt2rb_md5_final(Lt2rbMd5 *md5, uint8_t out[16]);
void lt2rb_print_md5(FILE *stream, const uint8_t digest[16]);

#endif
