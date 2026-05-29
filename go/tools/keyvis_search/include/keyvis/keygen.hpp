#pragma once

#include "keyvis/common.hpp"

#include <cstdint>

namespace keyvis {

void init_alphabet_index();
void init_powtab();
void init_keygen();

bool prepare_name_crc(const char *name, int nlen, uint16_t day,
                      uint32_t name_crc, uint8_t kb6[6], u128 &msg);
bool prepare_name(const char *name, int nlen, uint16_t day, uint8_t kb6[6],
                  u128 &msg);
bool sign_prepared(const uint8_t kb6[6], u128 msg, uint32_t seed,
                   char key[64]);
bool keygen(const char *name, int nlen, uint16_t day, uint32_t seed,
            char key[64]);
bool keygen_crc(const char *name, int nlen, uint16_t day, uint32_t name_crc,
                uint32_t seed, char key[64]);

void count_key(const char *key, Counts &counts);
bool self_test();

} // namespace keyvis
