#include "lt2rb/bzip2_stream.h"
#include "lt2rb/constants.h"
#include "lt2rb/error.h"
#include "lt2rb/md5.h"
#include "lt2rb/rb.h"
#include "lt2rb/util.h"

#include <errno.h>
#include <stdbool.h>
#include <stddef.h>
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

typedef enum OptionKind {
    OPTION_BOOL,
    OPTION_STRING,
    OPTION_OUTPUT,
    OPTION_U64,
} OptionKind;

typedef struct CliOption {
    const char *name;
    const char *metavar;
    const char *help;
    OptionKind kind;
    size_t field_offset;
} CliOption;

static const CliOption cli_options[] = {
    {"offset", "N", "compressed stream byte offset; decimal or 0x-prefixed hex", OPTION_U64, offsetof(Config, offset)},
    {"length", "N", "compressed byte count; use 0 to read to end of input", OPTION_U64, offsetof(Config, length)},
    {"compress-rb", NULL, "treat input as a decompressed .rb file and write a bzip2 stream", OPTION_BOOL, offsetof(Config, compress_rb)},
    {"extract-images", "D", "directory where bitmap records should be written as PNGs", OPTION_STRING, offsetof(Config, extract_images)},
    {"rb-input", NULL, "treat input as an already decompressed Lemonade2.rb file", OPTION_BOOL, offsetof(Config, rb_input)},
    {"no-transparency", NULL, "preserve chroma-key pixels instead of making them transparent", OPTION_BOOL, offsetof(Config, no_transparency)},
    {"output", "PATH", "output .rb or compressed stream path", OPTION_OUTPUT, offsetof(Config, output)},
    {"roundtrip-md5", NULL, "decompress, recompress, and require compressed MD5 match", OPTION_BOOL, offsetof(Config, roundtrip_md5)},
    {"validate-rb", NULL, "validate Lemonade2.rb container structure after reading/decompressing", OPTION_BOOL, offsetof(Config, validate_rb)},
    {"scan", NULL, "list bzip2 stream candidates and exit", OPTION_BOOL, offsetof(Config, scan)},
    {"quiet", NULL, "suppress success output", OPTION_BOOL, offsetof(Config, quiet)},
};

static void usage(FILE *stream, const char *argv0)
{
    fprintf(stream, "Usage: %s [flags] installer.exe [output.rb]\n\n", argv0);
    fprintf(stream,
        "Reads, writes, and validates Lemonade Tycoon 2 resource-bundle payloads.\n");
    fprintf(stream, "Defaults target Lemonade2.rb at offset 0x%X with length %llu.\n\n",
        LT2RB_DEFAULT_RB_OFFSET, (unsigned long long)LT2RB_DEFAULT_RB_LENGTH);
    fprintf(stream, "You can get the installer from:\n  %s\n\n", LT2RB_SOURCE_URL);
    fprintf(stream, "Flags:\n");
    fprintf(stream, "  %-20s %s\n", "-h, --help", "show this help and exit");
    for (size_t i = 0; i < sizeof(cli_options) / sizeof(cli_options[0]); i++) {
        const CliOption *opt = &cli_options[i];
        char display[64];

        if (opt->metavar != NULL) {
            snprintf(display, sizeof(display), "-%s %s", opt->name, opt->metavar);
        } else {
            snprintf(display, sizeof(display), "-%s", opt->name);
        }
        fprintf(stream, "  %-20s %s\n", display, opt->help);
    }
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

static void *config_field(Config *cfg, size_t offset)
{
    return (void *)((unsigned char *)cfg + offset);
}

static const CliOption *find_option(const char *arg, const char **inline_value)
{
    const char *name;
    const char *equals;
    size_t name_len;

    *inline_value = NULL;
    if (arg[0] != '-') return NULL;

    name = arg + 1;
    if (name[0] == '-') name++;
    if (name[0] == '\0') return NULL;

    equals = strchr(name, '=');
    name_len = equals == NULL ? strlen(name) : (size_t)(equals - name);
    if (equals != NULL) *inline_value = equals + 1;

    for (size_t i = 0; i < sizeof(cli_options) / sizeof(cli_options[0]); i++) {
        const CliOption *opt = &cli_options[i];
        if (strlen(opt->name) == name_len && strncmp(opt->name, name, name_len) == 0) {
            return opt;
        }
    }
    return NULL;
}

static int apply_option(Config *cfg, const CliOption *opt, const char *arg,
    const char *value)
{
    switch (opt->kind) {
    case OPTION_BOOL:
        if (value == NULL) {
            *(bool *)config_field(cfg, opt->field_offset) = true;
            return 1;
        }
        return parse_bool(value, (bool *)config_field(cfg, opt->field_offset));
    case OPTION_STRING:
        *(const char **)config_field(cfg, opt->field_offset) = value;
        return 1;
    case OPTION_OUTPUT:
        *(const char **)config_field(cfg, opt->field_offset) = value;
        cfg->output_set = true;
        return 1;
    case OPTION_U64:
        return parse_u64(value, (uint64_t *)config_field(cfg, opt->field_offset));
    }
    return lt2rb_set_error("internal option table error for %s", arg);
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

        if (!end_options && strcmp(arg, "--") == 0) {
            end_options = true;
            continue;
        }
        if (!end_options && (strcmp(arg, "-h") == 0 || strcmp(arg, "--help") == 0)) {
            usage(stdout, argv[0]);
            exit(0);
        }
        if (!end_options && arg[0] == '-') {
            const CliOption *opt = find_option(arg, &value);

            if (opt == NULL) {
                return lt2rb_set_error("unknown option: %s", arg);
            }
            if (opt->kind != OPTION_BOOL && value == NULL) {
                if (i + 1 >= argc) {
                    return lt2rb_set_error("%s requires a value", arg);
                }
                value = argv[++i];
            }
            if (!apply_option(cfg, opt, arg, value)) return 0;
            continue;
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
