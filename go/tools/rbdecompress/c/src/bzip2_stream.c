#include "lt2rb/bzip2_stream.h"

#include "lt2rb/error.h"
#include "lt2rb/md5.h"
#include "lt2rb/util.h"

#include <bzlib.h>
#include <errno.h>
#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

int lt2rb_decompress_bzip2_section(const char *input_path,
    const char *output_path, uint64_t offset, uint64_t length,
    uint64_t *out_written)
{
    FILE *input = NULL;
    FILE *output = NULL;
    char *temp_path = NULL;
    uint8_t input_buf[64 * 1024];
    uint8_t output_buf[64 * 1024];
    uint8_t magic[3];
    bz_stream stream = {0};
    uint64_t input_size = 0;
    uint64_t section_length;
    uint64_t remaining;
    uint64_t written = 0;
    int bzerr;
    int result = 0;

    input = fopen(input_path, "rb");
    if (input == NULL) {
        lt2rb_set_error("open input: %s", strerror(errno));
        goto done;
    }
    if (!lt2rb_file_size(input, &input_size)) {
        goto done;
    }
    if (offset >= input_size) {
        lt2rb_set_error("offset 0x%llX is past end of input (%llu bytes)",
            (unsigned long long)offset, (unsigned long long)input_size);
        goto done;
    }

    section_length = length == 0 ? input_size - offset : length;
    if (section_length > input_size - offset) {
        lt2rb_set_error("section 0x%llX+%llu exceeds input size %llu",
            (unsigned long long)offset, (unsigned long long)section_length,
            (unsigned long long)input_size);
        goto done;
    }
    if (!lt2rb_seek_u64(input, offset) || fread(magic, 1, sizeof(magic), input) != sizeof(magic)) {
        lt2rb_set_error("read bzip2 magic: %s", ferror(input) ? strerror(errno) : "short read");
        goto done;
    }
    if (memcmp(magic, "BZh", 3) != 0) {
        lt2rb_set_error("section at 0x%llX does not start with bzip2 magic BZh",
            (unsigned long long)offset);
        goto done;
    }
    if (!lt2rb_seek_u64(input, offset)) {
        goto done;
    }

    temp_path = lt2rb_append_suffix(output_path, ".tmp");
    if (temp_path == NULL) {
        goto done;
    }
    output = fopen(temp_path, "wb");
    if (output == NULL) {
        lt2rb_set_error("create temp output: %s", strerror(errno));
        goto done;
    }

    bzerr = BZ2_bzDecompressInit(&stream, 0, 0);
    if (bzerr != BZ_OK) {
        lt2rb_set_error("initialize bzip2 decompressor: %d", bzerr);
        goto done;
    }

    remaining = section_length;
    bzerr = BZ_OK;
    while (bzerr != BZ_STREAM_END) {
        if (stream.avail_in == 0 && remaining > 0) {
            size_t want = sizeof(input_buf);
            size_t got;

            if (remaining < want) {
                want = (size_t)remaining;
            }
            got = fread(input_buf, 1, want, input);
            if (got == 0) {
                lt2rb_set_error("read compressed section: %s",
                    ferror(input) ? strerror(errno) : "short read");
                goto done_bz;
            }
            stream.next_in = (char *)input_buf;
            stream.avail_in = (unsigned int)got;
            remaining -= got;
        }

        stream.next_out = (char *)output_buf;
        stream.avail_out = sizeof(output_buf);
        bzerr = BZ2_bzDecompress(&stream);
        if (bzerr != BZ_OK && bzerr != BZ_STREAM_END) {
            lt2rb_set_error("decompress bzip2 stream: %d", bzerr);
            goto done_bz;
        }

        size_t produced = sizeof(output_buf) - stream.avail_out;
        if (produced > 0) {
            if (fwrite(output_buf, 1, produced, output) != produced) {
                lt2rb_set_error("write output: %s", strerror(errno));
                goto done_bz;
            }
            written += produced;
        }
        if (remaining == 0 && stream.avail_in == 0 && bzerr == BZ_OK && produced == 0) {
            lt2rb_set_error("decompress bzip2 stream: unexpected end of section");
            goto done_bz;
        }
    }

    result = 1;

done_bz:
    BZ2_bzDecompressEnd(&stream);
done:
    if (output != NULL && fclose(output) != 0 && result) {
        lt2rb_set_error("close temp output: %s", strerror(errno));
        result = 0;
    }
    if (input != NULL) {
        fclose(input);
    }
    if (result && rename(temp_path, output_path) != 0) {
        lt2rb_set_error("replace output: %s", strerror(errno));
        result = 0;
    }
    if (!result && temp_path != NULL) {
        remove(temp_path);
    }
    free(temp_path);
    if (result && out_written != NULL) {
        *out_written = written;
    }
    return result;
}

int lt2rb_md5_compressed_section(const char *input_path, uint64_t offset,
    uint64_t length, uint8_t digest[16])
{
    FILE *input = NULL;
    Lt2rbMd5 md5;
    uint8_t buffer[64 * 1024];
    uint64_t input_size;
    uint64_t section_length;
    uint64_t remaining;
    int result = 0;

    input = fopen(input_path, "rb");
    if (input == NULL) {
        lt2rb_set_error("open input: %s", strerror(errno));
        goto done;
    }
    if (!lt2rb_file_size(input, &input_size)) {
        goto done;
    }
    if (offset >= input_size) {
        lt2rb_set_error("offset 0x%llX is past end of input (%llu bytes)",
            (unsigned long long)offset, (unsigned long long)input_size);
        goto done;
    }
    section_length = length == 0 ? input_size - offset : length;
    if (section_length > input_size - offset) {
        lt2rb_set_error("section 0x%llX+%llu exceeds input size %llu",
            (unsigned long long)offset, (unsigned long long)section_length,
            (unsigned long long)input_size);
        goto done;
    }
    if (!lt2rb_seek_u64(input, offset)) {
        goto done;
    }

    lt2rb_md5_init(&md5);
    remaining = section_length;
    while (remaining > 0) {
        size_t want = sizeof(buffer);
        size_t got;
        if (remaining < want) {
            want = (size_t)remaining;
        }
        got = fread(buffer, 1, want, input);
        if (got != want) {
            lt2rb_set_error("read compressed section: %s",
                ferror(input) ? strerror(errno) : "short read");
            goto done;
        }
        lt2rb_md5_update(&md5, buffer, got);
        remaining -= got;
    }
    lt2rb_md5_final(&md5, digest);
    result = 1;

done:
    if (input != NULL) {
        fclose(input);
    }
    return result;
}

int lt2rb_compress_rb_file(const char *input_path, const char *output_path,
    uint64_t *out_written, uint8_t digest[16])
{
    FILE *input = NULL;
    FILE *output = NULL;
    char *temp_path = NULL;
    uint8_t input_buf[64 * 1024];
    uint8_t output_buf[64 * 1024];
    bz_stream stream = {0};
    Lt2rbMd5 md5;
    bool eof = false;
    uint64_t written = 0;
    int bzerr;
    int result = 0;

    input = fopen(input_path, "rb");
    if (input == NULL) {
        lt2rb_set_error("open rb input: %s", strerror(errno));
        goto done;
    }
    temp_path = lt2rb_append_suffix(output_path, ".tmp");
    if (temp_path == NULL) {
        goto done;
    }
    output = fopen(temp_path, "wb");
    if (output == NULL) {
        lt2rb_set_error("create temp output: %s", strerror(errno));
        goto done;
    }

    bzerr = BZ2_bzCompressInit(&stream, 9, 0, 30);
    if (bzerr != BZ_OK) {
        lt2rb_set_error("initialize bzip2 compressor: %d", bzerr);
        goto done;
    }
    lt2rb_md5_init(&md5);

    while (bzerr != BZ_STREAM_END) {
        int action;

        if (stream.avail_in == 0 && !eof) {
            size_t got = fread(input_buf, 1, sizeof(input_buf), input);
            if (got == 0) {
                if (ferror(input)) {
                    lt2rb_set_error("read rb input: %s", strerror(errno));
                    goto done_bz;
                }
                eof = true;
            } else {
                stream.next_in = (char *)input_buf;
                stream.avail_in = (unsigned int)got;
            }
        }

        action = eof ? BZ_FINISH : BZ_RUN;
        stream.next_out = (char *)output_buf;
        stream.avail_out = sizeof(output_buf);
        bzerr = BZ2_bzCompress(&stream, action);
        if (bzerr != BZ_RUN_OK && bzerr != BZ_FINISH_OK && bzerr != BZ_STREAM_END) {
            lt2rb_set_error("compress rb with bzip2: %d", bzerr);
            goto done_bz;
        }

        size_t produced = sizeof(output_buf) - stream.avail_out;
        if (produced > 0) {
            if (fwrite(output_buf, 1, produced, output) != produced) {
                lt2rb_set_error("write compressed output: %s", strerror(errno));
                goto done_bz;
            }
            lt2rb_md5_update(&md5, output_buf, produced);
            written += produced;
        }
    }

    lt2rb_md5_final(&md5, digest);
    result = 1;

done_bz:
    BZ2_bzCompressEnd(&stream);
done:
    if (output != NULL && fclose(output) != 0 && result) {
        lt2rb_set_error("close temp output: %s", strerror(errno));
        result = 0;
    }
    if (input != NULL) {
        fclose(input);
    }
    if (result && rename(temp_path, output_path) != 0) {
        lt2rb_set_error("replace output: %s", strerror(errno));
        result = 0;
    }
    if (!result && temp_path != NULL) {
        remove(temp_path);
    }
    free(temp_path);
    if (result && out_written != NULL) {
        *out_written = written;
    }
    return result;
}

int lt2rb_print_bzip2_offsets(const char *input_path)
{
    Lt2rbBuffer input;
    size_t count = 0;

    if (!lt2rb_read_file_all(input_path, &input)) {
        return 0;
    }
    for (size_t i = 0; i + 4 <= input.size; i++) {
        if (memcmp(input.data + i, "BZh", 3) == 0 &&
            input.data[i + 3] >= '1' && input.data[i + 3] <= '9') {
            count++;
        }
    }
    printf("found %zu bzip2 stream candidate(s)\n", count);
    for (size_t i = 0; i + 4 <= input.size; i++) {
        if (memcmp(input.data + i, "BZh", 3) == 0 &&
            input.data[i + 3] >= '1' && input.data[i + 3] <= '9') {
            printf("0x%zX (%zu)\n", i, i);
        }
    }
    lt2rb_free_buffer(&input);
    return 1;
}
