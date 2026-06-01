#ifndef LT2RB_BZIP2_STREAM_H
#define LT2RB_BZIP2_STREAM_H

#include <stdint.h>

int lt2rb_decompress_bzip2_section(const char *input_path,
    const char *output_path, uint64_t offset, uint64_t length,
    uint64_t *out_written);
int lt2rb_compress_rb_file(const char *input_path, const char *output_path,
    uint64_t *out_written, uint8_t digest[16]);
int lt2rb_md5_compressed_section(const char *input_path, uint64_t offset,
    uint64_t length, uint8_t digest[16]);
int lt2rb_print_bzip2_offsets(const char *input_path);

#endif
