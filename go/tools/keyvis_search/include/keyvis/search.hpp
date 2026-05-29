#pragma once

#include "keyvis/common.hpp"

#include <cstdint>

namespace keyvis {

bool better(const Hit &a, const Hit &b);
bool better_counts(const Counts &counts, const char *key, const Hit &best);
void set_hit(Hit &hit, const char *name, int nlen, const char *key,
             uint32_t seed, const Counts &counts);
void print_hit(const char *tag, const Hit &hit, int day);
void run_search(const Options &options, const uint8_t fixed_kb6[6],
                u128 fixed_msg);

} // namespace keyvis
