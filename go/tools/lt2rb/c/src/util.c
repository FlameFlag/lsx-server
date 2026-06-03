#include "lt2rb/util.h"

#include "lt2rb/error.h"

#include <errno.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

#ifdef _WIN32
#include <direct.h>
#define LT2RB_PATH_SEP '\\'
static int rb_mkdir(const char *path) { return _mkdir(path); }
#else
#include <sys/stat.h>
#include <sys/types.h>
#define LT2RB_PATH_SEP '/'
static int rb_mkdir(const char *path) { return mkdir(path, 0755); }
#endif

int lt2rb_file_size(FILE *file, uint64_t *out)
{
    long size;

    if (fseek(file, 0, SEEK_END) != 0) {
        return lt2rb_set_error("seek input: %s", strerror(errno));
    }
    size = ftell(file);
    if (size < 0) {
        return lt2rb_set_error("tell input: %s", strerror(errno));
    }
    if (fseek(file, 0, SEEK_SET) != 0) {
        return lt2rb_set_error("rewind input: %s", strerror(errno));
    }
    *out = (uint64_t)size;
    return 1;
}

char *lt2rb_append_suffix(const char *path, const char *suffix)
{
    size_t path_len = strlen(path);
    size_t suffix_len = strlen(suffix);
    char *result = malloc(path_len + suffix_len + 1);

    if (result == NULL) {
        lt2rb_set_error("out of memory");
        return NULL;
    }
    memcpy(result, path, path_len);
    memcpy(result + path_len, suffix, suffix_len + 1);
    return result;
}

char *lt2rb_join_path(const char *dir, const char *name)
{
    size_t dir_len = strlen(dir);
    size_t name_len = strlen(name);
    bool needs_sep = dir_len > 0 && dir[dir_len - 1] != '/' && dir[dir_len - 1] != '\\';
    char *path = malloc(dir_len + (needs_sep ? 1 : 0) + name_len + 1);

    if (path == NULL) {
        lt2rb_set_error("out of memory");
        return NULL;
    }
    memcpy(path, dir, dir_len);
    if (needs_sep) {
        path[dir_len] = LT2RB_PATH_SEP;
    }
    memcpy(path + dir_len + (needs_sep ? 1 : 0), name, name_len + 1);
    return path;
}

int lt2rb_mkdir_all(const char *path)
{
    char *copy;
    size_t len;

    if (path == NULL || path[0] == '\0') {
        return lt2rb_set_error("image output directory is empty");
    }
    len = strlen(path);
    copy = malloc(len + 1);
    if (copy == NULL) {
        return lt2rb_set_error("out of memory");
    }
    memcpy(copy, path, len + 1);

    for (size_t i = 1; i < len; i++) {
        if (copy[i] == '/' || copy[i] == '\\') {
            char saved = copy[i];
            copy[i] = '\0';
            if (copy[0] != '\0' && rb_mkdir(copy) != 0 && errno != EEXIST) {
                int saved_errno = errno;
                free(copy);
                return lt2rb_set_error("create image output directory: %s", strerror(saved_errno));
            }
            copy[i] = saved;
        }
    }
    if (rb_mkdir(copy) != 0 && errno != EEXIST) {
        int saved_errno = errno;
        free(copy);
        return lt2rb_set_error("create image output directory: %s", strerror(saved_errno));
    }
    free(copy);
    return 1;
}

int lt2rb_read_file_all(const char *path, Lt2rbBuffer *out)
{
    FILE *file = fopen(path, "rb");
    uint64_t size64;
    uint8_t *data;

    out->data = NULL;
    out->size = 0;
    if (file == NULL) {
        return lt2rb_set_error("read input: %s", strerror(errno));
    }
    if (!lt2rb_file_size(file, &size64)) {
        fclose(file);
        return 0;
    }
    if (size64 > SIZE_MAX) {
        fclose(file);
        return lt2rb_set_error("input is too large for this C runtime");
    }
    data = malloc((size_t)size64 == 0 ? 1 : (size_t)size64);
    if (data == NULL) {
        fclose(file);
        return lt2rb_set_error("out of memory");
    }
    if (fread(data, 1, (size_t)size64, file) != (size_t)size64) {
        free(data);
        fclose(file);
        return lt2rb_set_error("read input: %s", ferror(file) ? strerror(errno) : "short read");
    }
    fclose(file);
    out->data = data;
    out->size = (size_t)size64;
    return 1;
}

void lt2rb_free_buffer(Lt2rbBuffer *buffer)
{
    free(buffer->data);
    buffer->data = NULL;
    buffer->size = 0;
}

int lt2rb_range_ok(size_t size, size_t offset, size_t length)
{
    return offset <= size && length <= size - offset;
}

int lt2rb_checked_mul_size(size_t a, size_t b, size_t *out)
{
    if (a != 0 && b > SIZE_MAX / a) {
        return 0;
    }
    *out = a * b;
    return 1;
}

uint16_t lt2rb_u16le(const uint8_t *p)
{
    return (uint16_t)((uint16_t)p[0] | ((uint16_t)p[1] << 8));
}

uint32_t lt2rb_u32le(const uint8_t *p)
{
    return (uint32_t)p[0] | ((uint32_t)p[1] << 8) |
        ((uint32_t)p[2] << 16) | ((uint32_t)p[3] << 24);
}
