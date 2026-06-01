#include "lt2rb/bzip2_stream.h"
#include "lt2rb/constants.h"
#include "lt2rb/error.h"
#include "lt2rb/md5.h"
#include "lt2rb/rb.h"
#include "lt2rb/util.h"

#include <errno.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

typedef struct Config {
    const char *input;
    const char *output;
    const char *extract_images;
    uint64_t offset;
    uint64_t length;
    bool output_set;
    bool compress_rb;
    bool rb_input;
    bool no_transparency;
    bool roundtrip_md5;
    bool validate_rb;
    bool scan;
    bool quiet;
} Config;

static void usage(FILE *stream, const char *argv0)
{
    fprintf(stream, "Usage: %s [flags] installer.exe [output.rb]\n\n", argv0);
    fprintf(stream,
        "Decompresses and creates Lemonade Tycoon 2 resource-bundle payloads.\n");
    fprintf(stream, "Defaults target Lemonade2.rb at offset 0x%X with length %llu.\n\n",
        LT2RB_DEFAULT_RB_OFFSET, (unsigned long long)LT2RB_DEFAULT_RB_LENGTH);
    fprintf(stream, "You can get the installer from:\n  %s\n\n", LT2RB_SOURCE_URL);
    fprintf(stream, "Flags:\n");
    fprintf(stream,
        "  -offset N           compressed stream byte offset; decimal or 0x-prefixed hex\n");
    fprintf(stream,
        "  -length N           compressed byte count; use 0 to read to end of input\n");
    fprintf(stream,
        "  -compress-rb        treat input as a decompressed .rb file and write a bzip2 stream\n");
    fprintf(stream,
        "  -extract-images D   directory where bitmap records should be written as PNGs\n");
    fprintf(stream,
        "  -rb-input           treat input as an already decompressed Lemonade2.rb file\n");
    fprintf(stream,
        "  -no-transparency    preserve chroma-key pixels instead of making them transparent\n");
    fprintf(stream, "  -output PATH        output .rb or compressed stream path\n");
    fprintf(stream,
        "  -roundtrip-md5      decompress, recompress, and require compressed MD5 match\n");
    fprintf(stream,
        "  -validate-rb        validate Lemonade2.rb container structure after reading/decompressing\n");
    fprintf(stream, "  -scan               list bzip2 stream candidates and exit\n");
    fprintf(stream, "  -quiet              suppress success output\n");
}

static int parse_u64(const char *text, uint64_t *out)
{
    char *end = NULL;
    unsigned long long value;

    if (text == NULL || text[0] == '\0' || text[0] == '-') {
        return lt2rb_set_error("invalid byte count: %s", text == NULL ? "" : text);
    }
    errno = 0;
    value = strtoull(text, &end, 0);
    if (errno != 0 || end == text || *end != '\0') {
        return lt2rb_set_error("invalid byte count: %s", text);
    }
    *out = (uint64_t)value;
    return 1;
}

static int parse_bool(const char *text, bool *out)
{
    if (strcmp(text, "1") == 0 || strcmp(text, "true") == 0 ||
        strcmp(text, "TRUE") == 0 || strcmp(text, "yes") == 0) {
        *out = true;
        return 1;
    }
    if (strcmp(text, "0") == 0 || strcmp(text, "false") == 0 ||
        strcmp(text, "FALSE") == 0 || strcmp(text, "no") == 0) {
        *out = false;
        return 1;
    }
    return lt2rb_set_error("invalid boolean value: %s", text);
}

static int option_value(int *index, int argc, char **argv, const char *arg,
    const char *name, const char **value)
{
    size_t name_len = strlen(name);

    if (strcmp(arg, name) == 0) {
        if (*index + 1 >= argc) {
            lt2rb_set_error("%s requires a value", name);
            return -1;
        }
        *index += 1;
        *value = argv[*index];
        return 1;
    }
    if (strncmp(arg, name, name_len) == 0 && arg[name_len] == '=') {
        *value = arg + name_len + 1;
        return 1;
    }
    return 0;
}

static int option_bool(const char *arg, const char *name, bool *value)
{
    size_t name_len = strlen(name);

    if (strcmp(arg, name) == 0) {
        *value = true;
        return 1;
    }
    if (strncmp(arg, name, name_len) == 0 && arg[name_len] == '=') {
        return parse_bool(arg + name_len + 1, value) ? 1 : -1;
    }
    return 0;
}

static int parse_args(int argc, char **argv, Config *cfg)
{
    const char *positionals[3] = {0};
    int positional_count = 0;
    bool end_options = false;

    *cfg = (Config){
        .output = LT2RB_DEFAULT_OUTPUT,
        .offset = LT2RB_DEFAULT_RB_OFFSET,
        .length = LT2RB_DEFAULT_RB_LENGTH,
    };

    for (int i = 1; i < argc; i++) {
        const char *arg = argv[i];
        const char *value = NULL;
        int matched;

        if (!end_options && strcmp(arg, "--") == 0) {
            end_options = true;
            continue;
        }
        if (!end_options && (strcmp(arg, "-h") == 0 || strcmp(arg, "--help") == 0)) {
            usage(stdout, argv[0]);
            exit(0);
        }
        if (!end_options && arg[0] == '-') {
            matched = option_value(&i, argc, argv, arg, "-offset", &value);
            if (matched == 0) matched = option_value(&i, argc, argv, arg, "--offset", &value);
            if (matched < 0) return 0;
            if (matched > 0) {
                if (!parse_u64(value, &cfg->offset)) return 0;
                continue;
            }

            matched = option_value(&i, argc, argv, arg, "-length", &value);
            if (matched == 0) matched = option_value(&i, argc, argv, arg, "--length", &value);
            if (matched < 0) return 0;
            if (matched > 0) {
                if (!parse_u64(value, &cfg->length)) return 0;
                continue;
            }

            matched = option_value(&i, argc, argv, arg, "-extract-images", &value);
            if (matched == 0) matched = option_value(&i, argc, argv, arg, "--extract-images", &value);
            if (matched < 0) return 0;
            if (matched > 0) {
                cfg->extract_images = value;
                continue;
            }

            matched = option_value(&i, argc, argv, arg, "-output", &value);
            if (matched == 0) matched = option_value(&i, argc, argv, arg, "--output", &value);
            if (matched < 0) return 0;
            if (matched > 0) {
                cfg->output = value;
                cfg->output_set = true;
                continue;
            }

            matched = option_bool(arg, "-compress-rb", &cfg->compress_rb);
            if (matched == 0) matched = option_bool(arg, "--compress-rb", &cfg->compress_rb);
            if (matched < 0) return 0;
            if (matched > 0) continue;

            matched = option_bool(arg, "-rb-input", &cfg->rb_input);
            if (matched == 0) matched = option_bool(arg, "--rb-input", &cfg->rb_input);
            if (matched < 0) return 0;
            if (matched > 0) continue;

            matched = option_bool(arg, "-roundtrip-md5", &cfg->roundtrip_md5);
            if (matched == 0) matched = option_bool(arg, "--roundtrip-md5", &cfg->roundtrip_md5);
            if (matched < 0) return 0;
            if (matched > 0) continue;

            matched = option_bool(arg, "-no-transparency", &cfg->no_transparency);
            if (matched == 0) matched = option_bool(arg, "--no-transparency", &cfg->no_transparency);
            if (matched < 0) return 0;
            if (matched > 0) continue;

            matched = option_bool(arg, "-validate-rb", &cfg->validate_rb);
            if (matched == 0) matched = option_bool(arg, "--validate-rb", &cfg->validate_rb);
            if (matched < 0) return 0;
            if (matched > 0) continue;

            matched = option_bool(arg, "-scan", &cfg->scan);
            if (matched == 0) matched = option_bool(arg, "--scan", &cfg->scan);
            if (matched < 0) return 0;
            if (matched > 0) continue;

            matched = option_bool(arg, "-quiet", &cfg->quiet);
            if (matched == 0) matched = option_bool(arg, "--quiet", &cfg->quiet);
            if (matched < 0) return 0;
            if (matched > 0) continue;

            return lt2rb_set_error("unknown option: %s", arg);
        }

        if (positional_count == 3) {
            return lt2rb_set_error("expected at most input and output paths, got too many arguments");
        }
        positionals[positional_count++] = arg;
    }

    if (positional_count == 0) {
        usage(stderr, argv[0]);
        return lt2rb_set_error("missing installer input path");
    }
    if (positional_count > 2) {
        return lt2rb_set_error("expected at most input and output paths, got %d arguments",
            positional_count);
    }
    cfg->input = positionals[0];
    if (positional_count == 2) {
        if (cfg->rb_input) {
            return lt2rb_set_error("-rb-input accepts only one positional input path");
        }
        cfg->output = positionals[1];
        cfg->output_set = true;
    }
    if (cfg->compress_rb && strcmp(cfg->output, LT2RB_DEFAULT_OUTPUT) == 0 && !cfg->output_set) {
        char *default_compressed = lt2rb_append_suffix(cfg->input, ".bz2");
        if (default_compressed == NULL) return 0;
        cfg->output = default_compressed;
    }
    return 1;
}

static int run(int argc, char **argv)
{
    Config cfg;
    Lt2rbBuffer rb;
    uint64_t written;
    size_t count;
    uint8_t original_md5[16];
    uint8_t recompressed_md5[16];

    if (!parse_args(argc, argv, &cfg)) return 0;
    if (cfg.scan) {
        if (cfg.rb_input) return lt2rb_set_error("-scan expects an installer input, not -rb-input");
        if (cfg.compress_rb || cfg.roundtrip_md5) return lt2rb_set_error("-scan cannot be combined with compression modes");
        return lt2rb_print_bzip2_offsets(cfg.input);
    }
    if (cfg.compress_rb) {
        if (cfg.rb_input || cfg.roundtrip_md5) return lt2rb_set_error("-compress-rb cannot be combined with -rb-input or -roundtrip-md5");
        if (!lt2rb_compress_rb_file(cfg.input, cfg.output, &written, recompressed_md5)) return 0;
        if (!cfg.quiet) {
            printf("wrote %s (%llu bytes, md5 ", cfg.output, (unsigned long long)written);
            lt2rb_print_md5(stdout, recompressed_md5);
            printf(")\n");
        }
        return 1;
    }
    if (cfg.rb_input) {
        if (cfg.roundtrip_md5) return lt2rb_set_error("-roundtrip-md5 expects an installer input, not -rb-input");
        if (!cfg.validate_rb && (cfg.extract_images == NULL || cfg.extract_images[0] == '\0')) return lt2rb_set_error("-rb-input requires -extract-images or -validate-rb");
        if (!lt2rb_read_file_all(cfg.input, &rb)) return 0;
        if (cfg.validate_rb && !lt2rb_validate_rb(rb)) {
            lt2rb_free_buffer(&rb);
            return 0;
        }
        if (!cfg.quiet && cfg.validate_rb) printf("validated %s\n", cfg.input);
        if (cfg.extract_images != NULL && cfg.extract_images[0] != '\0') {
            int ok = lt2rb_extract_bitmap_pngs(rb, cfg.extract_images, !cfg.no_transparency, &count);
            lt2rb_free_buffer(&rb);
            if (!ok) return 0;
            if (!cfg.quiet) printf("wrote %zu bitmap PNG(s) to %s\n", count, cfg.extract_images);
            return 1;
        }
        lt2rb_free_buffer(&rb);
        return 1;
    }

    if (!lt2rb_decompress_bzip2_section(cfg.input, cfg.output, cfg.offset,
            cfg.length, &written)) {
        return 0;
    }
    if (!cfg.quiet) printf("wrote %s (%llu bytes)\n", cfg.output, (unsigned long long)written);
    if (cfg.validate_rb) {
        if (!lt2rb_read_file_all(cfg.output, &rb)) return 0;
        int ok = lt2rb_validate_rb(rb);
        lt2rb_free_buffer(&rb);
        if (!ok) return 0;
        if (!cfg.quiet) printf("validated %s\n", cfg.output);
    }
    if (cfg.roundtrip_md5) {
        char *compressed_path = lt2rb_append_suffix(cfg.output, ".bz2");
        if (compressed_path == NULL) return 0;
        if (!lt2rb_md5_compressed_section(cfg.input, cfg.offset, cfg.length, original_md5) ||
            !lt2rb_compress_rb_file(cfg.output, compressed_path, &written, recompressed_md5)) {
            free(compressed_path);
            return 0;
        }
        if (!cfg.quiet) {
            printf("recompressed %s\n", compressed_path);
            printf("original compressed md5:     ");
            lt2rb_print_md5(stdout, original_md5);
            printf("\nrecompressed stream md5:    ");
            lt2rb_print_md5(stdout, recompressed_md5);
            printf("\n");
        }
        if (memcmp(original_md5, recompressed_md5, 16) != 0) {
            free(compressed_path);
            return lt2rb_set_error("round-trip compressed MD5 mismatch");
        }
        if (!cfg.quiet) printf("round-trip compressed MD5 matches\n");
        free(compressed_path);
    }
    if (cfg.extract_images != NULL && cfg.extract_images[0] != '\0') {
        if (!lt2rb_read_file_all(cfg.output, &rb)) return 0;
        int ok = lt2rb_extract_bitmap_pngs(rb, cfg.extract_images, !cfg.no_transparency, &count);
        lt2rb_free_buffer(&rb);
        if (!ok) return 0;
        if (!cfg.quiet) printf("wrote %zu bitmap PNG(s) to %s\n", count, cfg.extract_images);
    }
    return 1;
}

int main(int argc, char **argv)
{
    lt2rb_clear_error();
    if (!run(argc, argv)) {
        fprintf(stderr, "lt2rb: %s\n", lt2rb_error());
        return 1;
    }
    return 0;
}
