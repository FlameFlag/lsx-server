#include "keyvis/crc.hpp"

#include "keyvis/common.hpp"

#include <algorithm>
#include <array>

#if defined(__aarch64__) && defined(__ARM_FEATURE_CRC32)
#include <arm_acle.h>
#define KEYVIS_HW_CRC32 1
#else
#define KEYVIS_HW_CRC32 0
#endif

namespace keyvis {
namespace {

std::array<uint32_t, 256> g_crc_table{};

} // namespace

void init_crc_table() {
  for (uint32_t i = 0; i < 256; i++) {
    uint32_t c = i;
    for (int j = 0; j < 8; j++) {
      c = c & 1 ? 0xedb88320 ^ (c >> 1) : c >> 1;
    }
    g_crc_table[i] = c;
  }
}

uint32_t crc_step(uint32_t crc, uint8_t byte) {
#if KEYVIS_HW_CRC32
  return __crc32b(crc, byte);
#else
  return g_crc_table[(crc ^ byte) & 255] ^ (crc >> 8);
#endif
}

uint32_t crc(const char *s, size_t n) {
  uint32_t value = ~0u;
  for (size_t i = 0; i < n; i++) {
    value = crc_step(value, uint8_t(s[i]));
  }
  return value;
}

uint32_t hex_crc_group(uint32_t value, int group_size) {
  constexpr char kHex[] = "0123456789ABCDEF";

  uint32_t c = ~0u;
  for (int i = 0; i < 8; i += group_size) {
    for (int j = std::min(8, i + group_size) - 1; j >= i; j--) {
      const uint8_t ch = kHex[(value >> (4 * (7 - j))) & 15];
      c = crc_step(c, ch);
    }
  }
  return c;
}

} // namespace keyvis
