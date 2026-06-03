#include "lt2rb/archive.h"
#include "lt2rb/error.h"
#include "lt2rb/rb.h"
#include "lt2rb/util.h"

#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static const unsigned char archive_magic[8] = {'L', 'T', '2', 'R', 'B', 'F', 'S', '1'};

typedef struct Config {
    const char *command;
    const char *input;
    const char *output;
    bool quiet;
} Config;

static void usage(FILE *stream, const char *argv0)
{
    fprintf(stream, "Usage:\n");
    fprintf(stream, "  %s unpack input.rb output-dir\n", argv0);
    fprintf(stream, "  %s pack input-file-or-dir output.rb\n\n", argv0);
    fprintf(stream, "Unpacks an .rb into usable assets, or packs a file/folder into an .rb.\n\n");
    fprintf(stream, "Flags:\n");
    fprintf(stream, "  %-20s %s\n", "-h, --help", "show this help and exit");
    fprintf(stream, "  %-20s %s\n", "-quiet", "suppress success output");
}

static int parse_args(int argc, char **argv, Config *cfg)
{
    const char *positionals[3] = {0};
    int positional_count = 0;

    *cfg = (Config){0};
    for (int i = 1; i < argc; i++) {
        const char *arg = argv[i];

        if (strcmp(arg, "-h") == 0 || strcmp(arg, "--help") == 0) {
            usage(stdout, argv[0]);
            exit(0);
        }
        if (strcmp(arg, "-quiet") == 0 || strcmp(arg, "--quiet") == 0) {
            cfg->quiet = true;
            continue;
        }
        if (arg[0] == '-') {
            return lt2rb_set_error("unknown option: %s", arg);
        }
        if (positional_count == 3) {
            return lt2rb_set_error("expected command, input, and output");
        }
        positionals[positional_count++] = arg;
    }

    if (positional_count != 3) {
        usage(stderr, argv[0]);
        return lt2rb_set_error("expected command, input, and output; got %d argument(s)",
            positional_count);
    }
    cfg->command = positionals[0];
    cfg->input = positionals[1];
    cfg->output = positionals[2];
    return 1;
}

static int is_archive_file(Lt2rbBuffer input)
{
    return input.size >= sizeof(archive_magic) &&
        memcmp(input.data, archive_magic, sizeof(archive_magic)) == 0;
}

static int unpack_assets(const char *input_path, const char *output_dir, size_t *out_count)
{
    Lt2rbBuffer input;
    char *bitmap_dir;
    int ok;

    if (!lt2rb_read_file_all(input_path, &input)) return 0;
    if (is_archive_file(input)) {
        lt2rb_free_buffer(&input);
        return lt2rb_unpack_file_archive(input_path, output_dir, out_count);
    }

    bitmap_dir = lt2rb_join_path(output_dir, "bitmaps");
    if (bitmap_dir == NULL) {
        lt2rb_free_buffer(&input);
        return 0;
    }
    ok = lt2rb_extract_bitmap_pngs(input, bitmap_dir, true, out_count);
    free(bitmap_dir);
    lt2rb_free_buffer(&input);
    return ok;
}

static int run(int argc, char **argv)
{
    Config cfg;
    size_t count = 0;
    uint64_t written = 0;

    if (!parse_args(argc, argv, &cfg)) return 0;
    if (strcmp(cfg.command, "pack") == 0 || strcmp(cfg.command, "compress") == 0) {
        if (!lt2rb_pack_file_archive(cfg.input, cfg.output, &count, &written)) return 0;
        if (!cfg.quiet) {
            printf("packed %zu asset(s) into %s (%llu bytes)\n",
                count, cfg.output, (unsigned long long)written);
        }
        return 1;
    }
    if (strcmp(cfg.command, "unpack") == 0 || strcmp(cfg.command, "decompress") == 0 ||
        strcmp(cfg.command, "extract") == 0) {
        if (!unpack_assets(cfg.input, cfg.output, &count)) return 0;
        if (!cfg.quiet) printf("unpacked %zu asset(s) to %s\n", count, cfg.output);
        return 1;
    }
    usage(stderr, argv[0]);
    return lt2rb_set_error("unknown command: %s", cfg.command);
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
