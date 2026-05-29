#pragma once

#include <array>
#include <cstdint>
#include <string>
#include <thread>

#if defined(_MSVC_LANG)
#define KEYVIS_CPLUSPLUS _MSVC_LANG
#else
#define KEYVIS_CPLUSPLUS __cplusplus
#endif

#if KEYVIS_CPLUSPLUS < 202302L
#error "keyvis-search requires C++23; compile with -std=c++23"
#endif

namespace keyvis {

using u128 = unsigned __int128;
using Counts = std::array<int, 32>;

inline constexpr char kAlphabet[] = "0123456789ABCDEFGHJKMNPQRTUVWXYZ";
inline constexpr u128 kModP = (u128(1) << 72) + 3643;
inline constexpr u128 kModQ = kModP - 1;
inline constexpr u128 kModQHalf = kModQ >> 1;
inline constexpr u128 kBase = (u128(0xF3C7E00A4B581552ull) << 8) | 0x99;
inline constexpr u128 kPrivate = (u128(0x70301169DE7C75D6ull) << 8) | 0x6f;

struct Hit {
  std::string name;
  std::string key;
  uint32_t seed = 0;
  Counts counts{};
};

struct Options {
  int threads = int(std::thread::hardware_concurrency());
  int seconds = 60;
  int day = 0;
  int max_len = 40;
  int print_zero = 12;
  bool has_fixed_name = false;
  std::string fixed_name;
  uint64_t seed_start = 1;
  uint64_t seed_count = 0;
};

} // namespace keyvis
