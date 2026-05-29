#pragma once

#include <cstdint>

namespace keyvis {

struct Xoshiro256ss {
  uint64_t state[4];

  static uint64_t rotl(uint64_t x, int k) { return (x << k) | (x >> (64 - k)); }

  static uint64_t splitmix64(uint64_t &x) {
    uint64_t z = (x += 0x9e3779b97f4a7c15ull);
    z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9ull;
    z = (z ^ (z >> 27)) * 0x94d049bb133111ebull;
    return z ^ (z >> 31);
  }

  explicit Xoshiro256ss(uint64_t seed) {
    for (uint64_t &value : state) {
      value = splitmix64(seed);
    }
  }

  uint64_t next() {
    const uint64_t result = rotl(state[1] * 5, 7) * 9;
    const uint64_t t = state[1] << 17;

    state[2] ^= state[0];
    state[3] ^= state[1];
    state[1] ^= state[2];
    state[0] ^= state[3];
    state[2] ^= t;
    state[3] = rotl(state[3], 45);

    return result;
  }
};

} // namespace keyvis
