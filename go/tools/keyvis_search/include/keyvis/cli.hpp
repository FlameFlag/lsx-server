#pragma once

#include "keyvis/common.hpp"

#include <cstdio>

namespace keyvis {

int current_day();
void version();
void usage(FILE *f, const char *program);
bool parse_options(int argc, char **argv, Options &options);
bool prepare_fixed_name(const char *program, const Options &options,
                        uint8_t kb6[6], u128 &msg);
void print_config(const Options &options);

} // namespace keyvis
