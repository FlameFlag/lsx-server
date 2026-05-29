#include "keyvis/keygen.hpp"

#include "keyvis/crc.hpp"

#include <array>
#include <cstdio>
#include <cstring>

namespace keyvis {
namespace {

std::array<uint8_t, 256> g_alphabet_index{};
std::array<std::array<u128, 16>, 19> g_powtab{};

u128 reduce_pm_positive(u128 x, uint32_t c, u128 mod) {
  constexpr u128 kMask72 = (u128(1) << 72) - 1;

  const u128 lo = x & kMask72;
  const u128 hi = x >> 72;
  __int128 diff = (__int128)lo - (__int128)(hi * c);

  if (diff < 0) {
    diff += mod;
  }

  return (u128)diff;
}

u128 reduce_pm_signed(__int128 x, uint32_t c, u128 mod) {
  if (x >= 0) {
    return reduce_pm_positive((u128)x, c, mod);
  }

  const u128 positive = reduce_pm_positive((u128)-x, c, mod);
  return positive ? mod - positive : 0;
}

u128 mod_qh(u128 x) {
  if (x >= kModQ) {
    x -= kModQ;
  }
  if (x >= kModQHalf) {
    x -= kModQHalf;
  }
  return x;
}

bool inv_mod_odd(u128 a, u128 mod, u128 &out) {
  u128 u = a;
  u128 v = mod;
  u128 x1 = 1;
  u128 x2 = 0;

  while (u != 1 && v != 1) {
    while (!(u & 1)) {
      u >>= 1;
      x1 = (x1 & 1) ? (x1 + mod) >> 1 : x1 >> 1;
    }

    while (!(v & 1)) {
      v >>= 1;
      x2 = (x2 & 1) ? (x2 + mod) >> 1 : x2 >> 1;
    }

    if (u >= v) {
      u -= v;
      x1 = x1 >= x2 ? x1 - x2 : x1 + mod - x2;
    } else {
      v -= u;
      x2 = x2 >= x1 ? x2 - x1 : x2 + mod - x1;
    }

    if (!u || !v) {
      return false;
    }
  }

  out = u == 1 ? x1 : x2;
  return true;
}

bool inv_mod_q(u128 k, u128 &out) {
  if (!(k & 1)) {
    return false;
  }

  u128 inv;
  if (!inv_mod_odd(mod_qh(k), kModQHalf, inv)) {
    return false;
  }

  out = (inv & 1) ? inv : inv + kModQHalf;
  return true;
}

u128 mmul(u128 a, u128 b, uint32_t fold, u128 mod) {
  constexpr u128 kMask36 = (u128(1) << 36) - 1;
  constexpr u128 kMask72 = (u128(1) << 72) - 1;

  const u128 a0 = a & kMask36;
  const u128 a1 = a >> 36;
  const u128 b0 = b & kMask36;
  const u128 b1 = b >> 36;
  const u128 c0 = a0 * b0;
  const u128 c1 = a0 * b1 + a1 * b0;
  const u128 c2 = a1 * b1;

  u128 lo = c0 + ((c1 & kMask36) << 36);
  const u128 hi = c2 + (c1 >> 36) + (lo >> 72);
  lo &= kMask72;

  return reduce_pm_signed((__int128)lo - (__int128)(hi * fold), fold, mod);
}

u128 mmul_p(u128 a, u128 b) { return mmul(a, b, 3643, kModP); }

u128 mmul_q(u128 a, u128 b) { return mmul(a, b, 3642, kModQ); }

u128 mpow_base(u128 exponent) {
  u128 result = 1;

  for (int pos = 0; exponent; pos++, exponent >>= 4) {
    const int nibble = int(exponent & 15);
    if (nibble) {
      result = mmul_p(result, g_powtab[pos][nibble]);
    }
  }

  return result;
}

uint32_t bswap(uint32_t x) { return __builtin_bswap32(x); }

uint32_t load_5_bits(uint64_t l0, uint64_t l1, uint64_t l2, int bitpos) {
  const int limb = bitpos >> 6;
  const int offset = bitpos & 63;
  const uint64_t lo = limb == 0 ? l0 : limb == 1 ? l1 : l2;
  const uint64_t hi = limb == 0 ? l1 : limb == 1 ? l2 : 0;

  uint64_t value = lo >> offset;
  if (offset > 59) {
    value |= hi << (64 - offset);
  }

  return uint32_t(value & 31);
}

uint64_t load_be64(const uint8_t *p) {
  uint64_t value = 0;
  for (int i = 0; i < 8; i++) {
    value = (value << 8) | p[i];
  }
  return value;
}

void enc(const uint8_t *bytes, char *out) {
  char tmp[64];
  int output_len = 0;
  int group_remaining = 6;

  const uint64_t l0 = load_be64(bytes + 16);
  const uint64_t l1 = load_be64(bytes + 8);
  const uint64_t l2 = load_be64(bytes);

  int high_digit = 38;
  while (high_digit > 0 && load_5_bits(l0, l1, l2, high_digit * 5) == 0) {
    high_digit--;
  }

  for (int i = 0; i < high_digit; i++) {
    tmp[output_len++] = kAlphabet[load_5_bits(l0, l1, l2, i * 5)];
    if (!--group_remaining) {
      tmp[output_len++] = '-';
      group_remaining = 6;
    }
  }

  const uint32_t rem = load_5_bits(l0, l1, l2, high_digit * 5);
  if (rem < 16) {
    tmp[output_len++] = kAlphabet[rem + 16];
    group_remaining--;
  } else {
    tmp[output_len++] = kAlphabet[rem];
    if (!--group_remaining) {
      tmp[output_len++] = '-';
      group_remaining = 6;
    }
    tmp[output_len++] = kAlphabet[16];
    group_remaining--;
  }

  while (group_remaining--) {
    tmp[output_len++] = '0';
  }

  for (int i = 0; i < output_len; i++) {
    out[i] = tmp[output_len - 1 - i];
  }
  out[output_len] = 0;
}

uint32_t rol(uint32_t x, int shift) {
  return (x << shift) | (x >> (32 - shift));
}

void md5_key_name(const uint8_t kb6[6], const char *name, int nlen,
                  uint8_t out[16]) {
  constexpr uint32_t kMd5Constants[64] = {
      0xd76aa478, 0xe8c7b756, 0x242070db, 0xc1bdceee, 0xf57c0faf, 0x4787c62a,
      0xa8304613, 0xfd469501, 0x698098d8, 0x8b44f7af, 0xffff5bb1, 0x895cd7be,
      0x6b901122, 0xfd987193, 0xa679438e, 0x49b40821, 0xf61e2562, 0xc040b340,
      0x265e5a51, 0xe9b6c7aa, 0xd62f105d, 0x02441453, 0xd8a1e681, 0xe7d3fbc8,
      0x21e1cde6, 0xc33707d6, 0xf4d50d87, 0x455a14ed, 0xa9e3e905, 0xfcefa3f8,
      0x676f02d9, 0x8d2a4c8a, 0xfffa3942, 0x8771f681, 0x6d9d6122, 0xfde5380c,
      0xa4beea44, 0x4bdecfa9, 0xf6bb4b60, 0xbebfbc70, 0x289b7ec6, 0xeaa127fa,
      0xd4ef3085, 0x04881d05, 0xd9d4d039, 0xe6db99e5, 0x1fa27cf8, 0xc4ac5665,
      0xf4292244, 0x432aff97, 0xab9423a7, 0xfc93a039, 0x655b59c3, 0x8f0ccc92,
      0xffeff47d, 0x85845dd1, 0x6fa87e4f, 0xfe2ce6e0, 0xa3014314, 0x4e0811a1,
      0xf7537e82, 0xbd3af235, 0x2ad7d2bb, 0xeb86d391};
  constexpr int kMd5Shifts[4][4] = {
      {7, 12, 17, 22},
      {5, 9, 14, 20},
      {4, 11, 16, 23},
      {6, 10, 15, 21},
  };

  uint8_t block[64]{};
  memcpy(block, kb6, 6);
  memcpy(block + 6, name, nlen);

  const int len = 6 + nlen;
  block[len] = 128;

  const uint64_t bits = uint64_t(len) * 8;
  memcpy(block + 56, &bits, 8);

  uint32_t words[16];
  for (int i = 0; i < 16; i++) {
    words[i] = uint32_t(block[4 * i]) | (uint32_t(block[4 * i + 1]) << 8) |
               (uint32_t(block[4 * i + 2]) << 16) |
               (uint32_t(block[4 * i + 3]) << 24);
  }

  uint32_t a = 0x67452301;
  uint32_t b = 0xefcdab89;
  uint32_t c = 0x98badcfe;
  uint32_t d = 0x10325476;
  const uint32_t a0 = a;
  const uint32_t b0 = b;
  const uint32_t c0 = c;
  const uint32_t d0 = d;

  for (int i = 0; i < 64; i++) {
    uint32_t f;
    uint32_t g;

    if (i < 16) {
      f = (b & c) | (~b & d);
      g = i;
    } else if (i < 32) {
      f = (d & b) | (~d & c);
      g = (5 * i + 1) & 15;
    } else if (i < 48) {
      f = b ^ c ^ d;
      g = (3 * i + 5) & 15;
    } else {
      f = c ^ (b | ~d);
      g = (7 * i) & 15;
    }

    const uint32_t round = i >> 4;
    const uint32_t z = a + f + kMd5Constants[i] + words[g];
    a = d;
    d = c;
    c = b;
    b += rol(z, kMd5Shifts[round][i & 3]);
  }

  const uint32_t hash[4] = {a0 + a, b0 + b, c0 + c, d0 + d};
  memcpy(out, hash, 16);
}

} // namespace

void init_alphabet_index() {
  g_alphabet_index.fill(255);
  for (int i = 0; i < 32; i++) {
    g_alphabet_index[uint8_t(kAlphabet[i])] = uint8_t(i);
  }
}

void init_powtab() {
  u128 base = kBase;

  for (auto &row : g_powtab) {
    row[0] = 1;

    for (int i = 1; i < 16; i++) {
      row[i] = mmul_p(row[i - 1], base);
    }

    for (int i = 0; i < 4; i++) {
      base = mmul_p(base, base);
    }
  }
}

void init_keygen() {
  init_crc_table();
  init_alphabet_index();
  init_powtab();
}

bool prepare_name_crc(const char *name, int nlen, uint16_t day,
                      uint32_t name_crc, uint8_t kb6[6], u128 &msg) {
  if (!nlen) {
    return false;
  }

  uint8_t md[16];
  kb6[0] = day;
  kb6[1] = day >> 8;

  const uint32_t signature = bswap(0xccf0580a);
  kb6[2] = signature;
  kb6[3] = signature >> 8;
  kb6[4] = signature >> 16;
  kb6[5] = signature >> 24;

  uint64_t r = name_crc;
  for (int i = 0; i < 6; i++) {
    r = (r * 31415821 + 1) % 100000000;
    kb6[i] ^= ((r / 10000) * 256) / 10000;
  }

  md5_key_name(kb6, name, nlen, md);

  msg = 0;
  for (int i = 0; i < 16; i += 4) {
    const uint32_t word = uint32_t(md[i]) | (uint32_t(md[i + 1]) << 8) |
                          (uint32_t(md[i + 2]) << 16) |
                          (uint32_t(md[i + 3]) << 24);
    msg = (msg << 32) + word;
  }

  msg = reduce_pm_positive(msg, 3642, kModQ);
  return true;
}

bool prepare_name(const char *name, int nlen, uint16_t day, uint8_t kb6[6],
                  u128 &msg) {
  return prepare_name_crc(name, nlen, day, crc(name, nlen), kb6, msg);
}

bool sign_prepared(const uint8_t kb6[6], u128 msg, uint32_t seed,
                   char key[64]) {
  uint8_t key_bytes[24];
  memcpy(key_bytes, kb6, 6);

  if (!seed) {
    seed = 1000;
  }

  for (uint32_t z = 0; z < 1000000; z++) {
    const uint32_t src = seed + z;
    u128 k = 0;

    for (int p = 2; p < 7; p++) {
      k = reduce_pm_positive((k << 4) + hex_crc_group(src, p), 3643, kModP);
    }

    u128 k_inverse;
    if (!k || !inv_mod_q(k, k_inverse)) {
      continue;
    }

    const u128 a = mpow_base(k);
    const u128 pa = mmul_q(kPrivate, a);
    const u128 b = mmul_q(msg >= pa ? msg - pa : msg + kModQ - pa, k_inverse);

    if (a >= 256 && b >= 256 && a < (u128(1) << 72) && b < (u128(1) << 72)) {
      for (int i = 0; i < 9; i++) {
        key_bytes[6 + 2 * i] = a >> (8 * i);
        key_bytes[7 + 2 * i] = b >> (8 * i);
      }
      enc(key_bytes, key);
      return true;
    }
  }

  return false;
}

bool keygen(const char *name, int nlen, uint16_t day, uint32_t seed,
            char key[64]) {
  uint8_t kb6[6];
  u128 msg;

  return prepare_name(name, nlen, day, kb6, msg) &&
         sign_prepared(kb6, msg, seed, key);
}

bool keygen_crc(const char *name, int nlen, uint16_t day, uint32_t name_crc,
                uint32_t seed, char key[64]) {
  uint8_t kb6[6];
  u128 msg;

  return prepare_name_crc(name, nlen, day, name_crc, kb6, msg) &&
         sign_prepared(kb6, msg, seed, key);
}

void count_key(const char *key, Counts &counts) {
  counts.fill(0);

  for (const char *p = key; *p; p++) {
    if (*p == '-') {
      continue;
    }

    const uint8_t index = g_alphabet_index[uint8_t(*p)];
    if (index < 32) {
      counts[index]++;
    }
  }
}

bool self_test() {
  char key[64];
  keygen("TESTNAME", 8, 10011, 1000, key);

  if (strcmp(key, "0000PP-FZYKGQ-JABWAK-Q6XMT6-U0U72Q-CD4Y50-JTAV0G")) {
    printf("bad vector %s\n", key);
    return false;
  }

  return true;
}

} // namespace keyvis
