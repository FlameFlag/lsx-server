#ifndef LT2RB_UTIL_H
#define LT2RB_UTIL_H

#include <stddef.h>
#include <stdint.h>
#include <stdio.h>

typedef struct Lt2rbBuffer {
    uint8_t *data;
    size_t size;
} Lt2rbBuffer;

int lt2rb_file_size(FILE *file, uint64_t *out);
int lt2rb_seek_u64(FILE *file, uint64_t offset);
char *lt2rb_append_suffix(const char *path, const char *suffix);
char *lt2rb_join_path(const char *dir, const char *name);
int lt2rb_mkdir_all(const char *path);
int lt2rb_read_file_all(const char *path, Lt2rbBuffer *out);
void lt2rb_free_buffer(Lt2rbBuffer *buffer);
int lt2rb_range_ok(size_t size, size_t offset, size_t length);
int lt2rb_checked_mul_size(size_t a, size_t b, size_t *out);
uint16_t lt2rb_u16le(const uint8_t *p);
uint32_t lt2rb_u32le(const uint8_t *p);

#endif
