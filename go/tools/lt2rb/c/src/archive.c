#include "lt2rb/archive.h"

#include "lt2rb/error.h"
#include "lt2rb/util.h"

#include <errno.h>
#include <limits.h>
#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <zlib.h>

#ifdef _WIN32
#include <direct.h>
#include <sys/stat.h>
#define LT2RB_STAT _stat
#define LT2RB_MODE_T int
#define LT2RB_PATH_SEP '\\'
#define lt2rb_chmod(path, mode) _chmod((path), (mode))
#else
#include <dirent.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <unistd.h>
#define LT2RB_STAT stat
#define LT2RB_MODE_T mode_t
#define LT2RB_PATH_SEP '/'
#define lt2rb_chmod(path, mode) chmod((path), (mode))
#endif

enum {
    LT2RB_ARCHIVE_TYPE_FILE = 1,
    LT2RB_ARCHIVE_TYPE_DIR = 2,
    LT2RB_ARCHIVE_MAX_PATH = 1024 * 1024,
};

static const uint8_t archive_magic[8] = {'L', 'T', '2', 'R', 'B', 'F', 'S', '1'};

typedef struct ArchiveEntry {
    char *fs_path;
    char *archive_path;
    uint32_t mode;
    uint64_t raw_size;
    bool is_dir;
} ArchiveEntry;

typedef struct EntryList {
    ArchiveEntry *items;
    size_t count;
    size_t cap;
} EntryList;

static void free_entries(EntryList *list)
{
    for (size_t i = 0; i < list->count; i++) {
        free(list->items[i].fs_path);
        free(list->items[i].archive_path);
    }
    free(list->items);
    list->items = NULL;
    list->count = 0;
    list->cap = 0;
}

static char *dup_str(const char *s)
{
    size_t len = strlen(s);
    char *copy = malloc(len + 1);
    if (copy == NULL) {
        lt2rb_set_error("out of memory");
        return NULL;
    }
    memcpy(copy, s, len + 1);
    return copy;
}

static const char *base_name(const char *path)
{
    const char *base = path;
    for (const char *p = path; *p != '\0'; p++) {
        if (*p == '/' || *p == '\\') base = p + 1;
    }
    return base;
}

static int checked_add(size_t a, size_t b, size_t *out)
{
    if (b > SIZE_MAX - a) return 0;
    *out = a + b;
    return 1;
}

static char *join_archive_path(const char *base, const char *name)
{
    size_t base_len = strlen(base);
    size_t name_len = strlen(name);
    size_t total;
    char *out;

    if (base_len == 0) return dup_str(name);
    if (!checked_add(base_len, 1, &total) || !checked_add(total, name_len, &total) ||
        !checked_add(total, 1, &total)) {
        lt2rb_set_error("archive path is too long");
        return NULL;
    }
    out = malloc(total);
    if (out == NULL) {
        lt2rb_set_error("out of memory");
        return NULL;
    }
    memcpy(out, base, base_len);
    out[base_len] = '/';
    memcpy(out + base_len + 1, name, name_len + 1);
    return out;
}

static int archive_path_is_safe(const char *path)
{
    const char *part = path;

    if (path == NULL || path[0] == '\0' || path[0] == '/' || path[0] == '\\') {
        return 0;
    }
    for (const char *p = path;; p++) {
        if (*p == '\\') return 0;
        if (*p == '/' || *p == '\0') {
            size_t len = (size_t)(p - part);
            if (len == 0) return 0;
            if ((len == 1 && part[0] == '.') ||
                (len == 2 && part[0] == '.' && part[1] == '.')) {
                return 0;
            }
            if (*p == '\0') break;
            part = p + 1;
        }
    }
    return 1;
}

static int append_entry(EntryList *list, const char *fs_path, const char *archive_path,
    uint32_t mode, uint64_t raw_size, bool is_dir)
{
    ArchiveEntry entry;

    if (!archive_path_is_safe(archive_path)) {
        return lt2rb_set_error("unsafe archive path: %s", archive_path);
    }
    if (list->count == list->cap) {
        size_t next_cap = list->cap == 0 ? 16 : list->cap * 2;
        ArchiveEntry *next;
        if (next_cap < list->cap) return lt2rb_set_error("too many archive entries");
        next = realloc(list->items, next_cap * sizeof(*next));
        if (next == NULL) return lt2rb_set_error("out of memory");
        list->items = next;
        list->cap = next_cap;
    }

    entry.fs_path = dup_str(fs_path);
    if (entry.fs_path == NULL) return 0;
    entry.archive_path = dup_str(archive_path);
    if (entry.archive_path == NULL) {
        free(entry.fs_path);
        return 0;
    }
    entry.mode = mode & 0777u;
    entry.raw_size = raw_size;
    entry.is_dir = is_dir;
    list->items[list->count++] = entry;
    return 1;
}

#ifndef _WIN32
static int collect_entries_rec(const char *fs_path, const char *archive_path,
    EntryList *list, bool include_self)
{
    struct stat st;

    if (stat(fs_path, &st) != 0) {
        return lt2rb_set_error("stat archive input %s: %s", fs_path, strerror(errno));
    }
    if (S_ISLNK(st.st_mode)) {
        return lt2rb_set_error("archive input contains symlink %s", fs_path);
    }
    if (S_ISDIR(st.st_mode)) {
        DIR *dir;
        struct dirent *ent;

        if (include_self &&
            !append_entry(list, fs_path, archive_path, (uint32_t)st.st_mode, 0, true)) {
            return 0;
        }
        dir = opendir(fs_path);
        if (dir == NULL) {
            return lt2rb_set_error("read archive input directory %s: %s", fs_path, strerror(errno));
        }
        while ((ent = readdir(dir)) != NULL) {
            char *child_fs;
            char *child_archive;
            int ok;

            if (strcmp(ent->d_name, ".") == 0 || strcmp(ent->d_name, "..") == 0) continue;
            child_fs = lt2rb_join_path(fs_path, ent->d_name);
            if (child_fs == NULL) {
                closedir(dir);
                return 0;
            }
            child_archive = join_archive_path(archive_path, ent->d_name);
            if (child_archive == NULL) {
                free(child_fs);
                closedir(dir);
                return 0;
            }
            ok = collect_entries_rec(child_fs, child_archive, list, true);
            free(child_fs);
            free(child_archive);
            if (!ok) {
                closedir(dir);
                return 0;
            }
        }
        closedir(dir);
        return 1;
    }
    if (!S_ISREG(st.st_mode)) {
        return lt2rb_set_error("archive input contains unsupported file type %s", fs_path);
    }
    return append_entry(list, fs_path, archive_path, (uint32_t)st.st_mode,
        (uint64_t)st.st_size, false);
}
#else
static int collect_entries_rec(const char *fs_path, const char *archive_path,
    EntryList *list, bool include_self)
{
    struct _stat st;

    (void)include_self;
    if (_stat(fs_path, &st) != 0) {
        return lt2rb_set_error("stat archive input %s: %s", fs_path, strerror(errno));
    }
    if ((st.st_mode & _S_IFDIR) != 0) {
        return lt2rb_set_error("directory packing is not implemented on this C runtime");
    }
    if ((st.st_mode & _S_IFREG) == 0) {
        return lt2rb_set_error("archive input contains unsupported file type %s", fs_path);
    }
    return append_entry(list, fs_path, archive_path, (uint32_t)st.st_mode,
        (uint64_t)st.st_size, false);
}
#endif

static int compare_entries(const void *a, const void *b)
{
    const ArchiveEntry *ea = (const ArchiveEntry *)a;
    const ArchiveEntry *eb = (const ArchiveEntry *)b;
    return strcmp(ea->archive_path, eb->archive_path);
}

static int collect_entries(const char *input_path, EntryList *list)
{
    struct LT2RB_STAT st;
    const char *root_archive;

    if (LT2RB_STAT(input_path, &st) != 0) {
        return lt2rb_set_error("stat archive input %s: %s", input_path, strerror(errno));
    }
    if ((st.st_mode & S_IFDIR) != 0) {
        return collect_entries_rec(input_path, "", list, false);
    }
    root_archive = base_name(input_path);
    if (!archive_path_is_safe(root_archive)) {
        return lt2rb_set_error("unsafe archive path: %s", root_archive);
    }
    return collect_entries_rec(input_path, root_archive, list, true);
}

static int write_u32(FILE *file, uint32_t value)
{
    uint8_t b[4] = {
        (uint8_t)value,
        (uint8_t)(value >> 8),
        (uint8_t)(value >> 16),
        (uint8_t)(value >> 24),
    };
    return fwrite(b, 1, sizeof(b), file) == sizeof(b);
}

static int write_u64(FILE *file, uint64_t value)
{
    uint8_t b[8];
    for (size_t i = 0; i < sizeof(b); i++) b[i] = (uint8_t)(value >> (i * 8));
    return fwrite(b, 1, sizeof(b), file) == sizeof(b);
}

static int read_exact(FILE *file, void *data, size_t size)
{
    return fread(data, 1, size, file) == size;
}

static int read_u32(FILE *file, uint32_t *out)
{
    uint8_t b[4];
    if (!read_exact(file, b, sizeof(b))) return 0;
    *out = (uint32_t)b[0] | ((uint32_t)b[1] << 8) |
        ((uint32_t)b[2] << 16) | ((uint32_t)b[3] << 24);
    return 1;
}

static int read_u64(FILE *file, uint64_t *out)
{
    uint8_t b[8];
    if (!read_exact(file, b, sizeof(b))) return 0;
    *out = 0;
    for (size_t i = 0; i < sizeof(b); i++) *out |= ((uint64_t)b[i]) << (i * 8);
    return 1;
}

static int read_file_bytes(const char *path, uint8_t **out, size_t *out_size)
{
    Lt2rbBuffer buf;
    if (!lt2rb_read_file_all(path, &buf)) return 0;
    *out = buf.data;
    *out_size = buf.size;
    return 1;
}

static int write_entry(FILE *out, const ArchiveEntry *entry)
{
    uint8_t *raw = NULL;
    uint8_t *compressed = NULL;
    size_t raw_size = 0;
    uLongf compressed_size = 0;
    uint32_t type = entry->is_dir ? LT2RB_ARCHIVE_TYPE_DIR : LT2RB_ARCHIVE_TYPE_FILE;
    uint64_t comp_size64 = 0;
    int zerr;
    int ok = 0;

    if (!entry->is_dir) {
        if (!read_file_bytes(entry->fs_path, &raw, &raw_size)) goto done;
        if (raw_size > UINT64_MAX || raw_size > ULONG_MAX) {
            lt2rb_set_error("archive input is too large: %s", entry->fs_path);
            goto done;
        }
        compressed_size = compressBound((uLong)raw_size);
        compressed = malloc(compressed_size == 0 ? 1 : compressed_size);
        if (compressed == NULL) {
            lt2rb_set_error("out of memory");
            goto done;
        }
        zerr = compress2(compressed, &compressed_size, raw, (uLong)raw_size, Z_BEST_COMPRESSION);
        if (zerr != Z_OK) {
            lt2rb_set_error("compress archive input %s: %d", entry->fs_path, zerr);
            goto done;
        }
        comp_size64 = (uint64_t)compressed_size;
    }

    if (!write_u32(out, (uint32_t)strlen(entry->archive_path)) ||
        !write_u32(out, type) ||
        !write_u32(out, entry->mode) ||
        !write_u64(out, entry->is_dir ? 0 : (uint64_t)raw_size) ||
        !write_u64(out, comp_size64) ||
        fwrite(entry->archive_path, 1, strlen(entry->archive_path), out) != strlen(entry->archive_path)) {
        lt2rb_set_error("write archive entry: %s", strerror(errno));
        goto done;
    }
    if (compressed_size > 0 && fwrite(compressed, 1, compressed_size, out) != compressed_size) {
        lt2rb_set_error("write archive entry data: %s", strerror(errno));
        goto done;
    }
    ok = 1;

done:
    free(raw);
    free(compressed);
    return ok;
}

int lt2rb_pack_file_archive(const char *input_path, const char *output_path,
    size_t *out_count, uint64_t *out_written)
{
    EntryList entries = {0};
    FILE *output = NULL;
    char *temp_path = NULL;
    size_t entry_count = 0;
    uint64_t written = 0;
    long pos;
    int result = 0;

    if (input_path == NULL || input_path[0] == '\0') {
        return lt2rb_set_error("input path is empty");
    }
    if (output_path == NULL || output_path[0] == '\0') {
        return lt2rb_set_error("output path is empty");
    }
    if (!collect_entries(input_path, &entries)) goto done;
    if (entries.count > UINT32_MAX) {
        lt2rb_set_error("too many archive entries");
        goto done;
    }
    qsort(entries.items, entries.count, sizeof(entries.items[0]), compare_entries);
    entry_count = entries.count;

    temp_path = lt2rb_append_suffix(output_path, ".tmp");
    if (temp_path == NULL) goto done;
    output = fopen(temp_path, "wb");
    if (output == NULL) {
        lt2rb_set_error("create temp output: %s", strerror(errno));
        goto done;
    }
    if (fwrite(archive_magic, 1, sizeof(archive_magic), output) != sizeof(archive_magic) ||
        !write_u32(output, (uint32_t)entries.count)) {
        lt2rb_set_error("write archive header: %s", strerror(errno));
        goto done;
    }
    for (size_t i = 0; i < entries.count; i++) {
        if (!write_entry(output, &entries.items[i])) goto done;
    }
    pos = ftell(output);
    if (pos < 0) {
        lt2rb_set_error("measure archive output: %s", strerror(errno));
        goto done;
    }
    written = (uint64_t)pos;
    result = 1;

done:
    if (output != NULL && fclose(output) != 0 && result) {
        lt2rb_set_error("close temp output: %s", strerror(errno));
        result = 0;
    }
    if (result && rename(temp_path, output_path) != 0) {
        lt2rb_set_error("replace output: %s", strerror(errno));
        result = 0;
    }
    if (!result && temp_path != NULL) remove(temp_path);
    free(temp_path);
    free_entries(&entries);
    if (result && out_count != NULL) *out_count = entry_count;
    if (result && out_written != NULL) *out_written = written;
    return result;
}

static int mkdir_parent(const char *path)
{
    char *copy = dup_str(path);
    char *slash;
    int ok;

    if (copy == NULL) return 0;
    slash = strrchr(copy, '/');
    if (slash == NULL) slash = strrchr(copy, '\\');
    if (slash == NULL) {
        free(copy);
        return 1;
    }
    *slash = '\0';
    ok = lt2rb_mkdir_all(copy);
    free(copy);
    return ok;
}

static char *archive_output_path(const char *output_dir, const char *archive_path)
{
    char *path = lt2rb_join_path(output_dir, archive_path);
    if (path == NULL) return NULL;
    for (char *p = path; *p != '\0'; p++) {
        if (*p == '/') *p = LT2RB_PATH_SEP;
    }
    return path;
}

static int read_entry_to_disk(FILE *input, const char *output_dir)
{
    uint32_t path_len;
    uint32_t type;
    uint32_t mode;
    uint64_t raw_size;
    uint64_t compressed_size;
    char *archive_path = NULL;
    char *output_path = NULL;
    uint8_t *compressed = NULL;
    uint8_t *raw = NULL;
    uLongf raw_len;
    FILE *out = NULL;
    int zerr;
    int ok = 0;

    if (!read_u32(input, &path_len) || !read_u32(input, &type) ||
        !read_u32(input, &mode) || !read_u64(input, &raw_size) ||
        !read_u64(input, &compressed_size)) {
        lt2rb_set_error("read archive entry header: %s",
            ferror(input) ? strerror(errno) : "short read");
        goto done;
    }
    if (path_len == 0 || path_len > LT2RB_ARCHIVE_MAX_PATH) {
        lt2rb_set_error("bad archive path length %u", path_len);
        goto done;
    }
    archive_path = malloc((size_t)path_len + 1);
    if (archive_path == NULL) {
        lt2rb_set_error("out of memory");
        goto done;
    }
    if (!read_exact(input, archive_path, path_len)) {
        lt2rb_set_error("read archive entry path: %s",
            ferror(input) ? strerror(errno) : "short read");
        goto done;
    }
    archive_path[path_len] = '\0';
    if (!archive_path_is_safe(archive_path)) {
        lt2rb_set_error("unsafe archive path: %s", archive_path);
        goto done;
    }
    output_path = archive_output_path(output_dir, archive_path);
    if (output_path == NULL) goto done;

    if (type == LT2RB_ARCHIVE_TYPE_DIR) {
        if (raw_size != 0 || compressed_size != 0) {
            lt2rb_set_error("directory archive entry has file payload");
            goto done;
        }
        if (!lt2rb_mkdir_all(output_path)) goto done;
        lt2rb_chmod(output_path, mode == 0 ? 0755 : (int)(mode & 0777u));
        ok = 1;
        goto done;
    }
    if (type != LT2RB_ARCHIVE_TYPE_FILE) {
        lt2rb_set_error("unknown archive entry type %u", type);
        goto done;
    }
    if (raw_size > ULONG_MAX || raw_size > SIZE_MAX || compressed_size > SIZE_MAX) {
        lt2rb_set_error("archive entry is too large for this C runtime");
        goto done;
    }
    compressed = malloc(compressed_size == 0 ? 1 : (size_t)compressed_size);
    raw = malloc(raw_size == 0 ? 1 : (size_t)raw_size);
    if (compressed == NULL || raw == NULL) {
        lt2rb_set_error("out of memory");
        goto done;
    }
    if (!read_exact(input, compressed, (size_t)compressed_size)) {
        lt2rb_set_error("read archive entry data: %s",
            ferror(input) ? strerror(errno) : "short read");
        goto done;
    }
    raw_len = (uLongf)raw_size;
    zerr = uncompress(raw, &raw_len, compressed, (uLong)compressed_size);
    if (zerr != Z_OK || raw_len != (uLongf)raw_size) {
        lt2rb_set_error("decompress archive entry %s: %d", archive_path, zerr);
        goto done;
    }
    if (!mkdir_parent(output_path)) goto done;
    out = fopen(output_path, "wb");
    if (out == NULL) {
        lt2rb_set_error("create archive output %s: %s", output_path, strerror(errno));
        goto done;
    }
    if (fwrite(raw, 1, (size_t)raw_size, out) != (size_t)raw_size) {
        lt2rb_set_error("write archive output %s: %s", output_path, strerror(errno));
        goto done;
    }
    if (fclose(out) != 0) {
        out = NULL;
        lt2rb_set_error("close archive output %s: %s", output_path, strerror(errno));
        goto done;
    }
    out = NULL;
    lt2rb_chmod(output_path, mode == 0 ? 0644 : (int)(mode & 0777u));
    ok = 1;

done:
    if (out != NULL) fclose(out);
    free(archive_path);
    free(output_path);
    free(compressed);
    free(raw);
    return ok;
}

int lt2rb_unpack_file_archive(const char *input_path, const char *output_dir,
    size_t *out_count)
{
    FILE *input = NULL;
    uint8_t magic[8];
    uint32_t count;
    int result = 0;

    if (input_path == NULL || input_path[0] == '\0') {
        return lt2rb_set_error("input path is empty");
    }
    if (output_dir == NULL || output_dir[0] == '\0') {
        return lt2rb_set_error("output directory is empty");
    }
    input = fopen(input_path, "rb");
    if (input == NULL) {
        lt2rb_set_error("open archive input: %s", strerror(errno));
        goto done;
    }
    if (!read_exact(input, magic, sizeof(magic))) {
        lt2rb_set_error("read archive magic: %s", ferror(input) ? strerror(errno) : "short read");
        goto done;
    }
    if (memcmp(magic, archive_magic, sizeof(magic)) != 0) {
        lt2rb_set_error("input is not an lt2rb file archive");
        goto done;
    }
    if (!read_u32(input, &count)) {
        lt2rb_set_error("read archive entry count: %s",
            ferror(input) ? strerror(errno) : "short read");
        goto done;
    }
    if (!lt2rb_mkdir_all(output_dir)) goto done;
    for (uint32_t i = 0; i < count; i++) {
        if (!read_entry_to_disk(input, output_dir)) goto done;
    }
    if (out_count != NULL) *out_count = count;
    result = 1;

done:
    if (input != NULL) fclose(input);
    return result;
}
