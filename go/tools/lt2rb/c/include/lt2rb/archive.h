#ifndef LT2RB_ARCHIVE_H
#define LT2RB_ARCHIVE_H

#include <stddef.h>
#include <stdint.h>

int lt2rb_pack_file_archive(const char *input_path, const char *output_path,
    size_t *out_count, uint64_t *out_written);
int lt2rb_unpack_file_archive(const char *input_path, const char *output_dir,
    size_t *out_count);

#endif
