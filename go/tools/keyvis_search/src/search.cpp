#include "keyvis/search.hpp"

#include "keyvis/crc.hpp"
#include "keyvis/keygen.hpp"
#include "keyvis/random.hpp"

#include <atomic>
#include <chrono>
#include <cstdio>
#include <cstring>
#include <memory>
#include <mutex>
#include <thread>
#include <vector>

namespace keyvis {
namespace {

enum class CandidateStatus {
  exhausted,
  invalid,
  valid,
};

uint32_t fast_bounded(uint64_t x, uint32_t bound) {
  return uint32_t((u128)x * bound >> 64);
}

void generate_random_name(Xoshiro256ss &rng, int max_len, char name[64],
                          int &len, uint32_t &name_crc) {
  len = 1 + fast_bounded(rng.next(), uint32_t(max_len));
  name_crc = ~0u;

  uint64_t bits = 0;
  int available_bits = 0;

  for (int i = 0; i < len; i++) {
    if (available_bits < 5) {
      bits = rng.next();
      available_bits = 60;
    }

    const char ch = kAlphabet[bits & 31];
    name[i] = ch;
    name_crc = crc_step(name_crc, uint8_t(ch));
    bits >>= 5;
    available_bits -= 5;
  }
}

CandidateStatus generate_candidate(const Options &options,
                                   const uint8_t fixed_kb6[6], u128 fixed_msg,
                                   Xoshiro256ss &rng, int thread_index,
                                   uint64_t step, char name[64], int &len,
                                   uint32_t &seed, char key[64]) {
  if (options.has_fixed_name) {
    const uint64_t idx =
        uint64_t(thread_index) + step * uint64_t(options.threads);
    if (options.seed_count && idx >= options.seed_count) {
      return CandidateStatus::exhausted;
    }

    len = int(options.fixed_name.size());
    seed = uint32_t(options.seed_start + idx);
    return sign_prepared(fixed_kb6, fixed_msg, seed, key)
               ? CandidateStatus::valid
               : CandidateStatus::invalid;
  }

  uint32_t name_crc = 0;
  generate_random_name(rng, options.max_len, name, len, name_crc);
  seed = uint32_t(rng.next());
  return keygen_crc(name, len, options.day, name_crc, seed, key)
             ? CandidateStatus::valid
             : CandidateStatus::invalid;
}

void merge_hit(const Hit &local, Hit &global, std::mutex &mutex, int day) {
  std::lock_guard<std::mutex> lock(mutex);
  if (better(local, global)) {
    global = local;
    print_hit("best", global, day);
  }
}

void worker(const Options &options, const uint8_t fixed_kb6[6], u128 fixed_msg,
            int thread_index, std::atomic<bool> &stop,
            std::atomic<uint64_t> &tried, std::atomic<uint64_t> &valid,
            Hit &global, std::mutex &global_mutex) {
  Xoshiro256ss rng(0xC0FFEE1234ull + uint64_t(thread_index));
  Hit local;
  char name[64];
  char key[64];
  uint64_t local_tried = 0;
  uint64_t local_valid = 0;

  if (options.has_fixed_name) {
    memcpy(name, options.fixed_name.c_str(), options.fixed_name.size());
  }

  for (uint64_t step = 0; !stop.load(std::memory_order_relaxed); step++) {
    int len = 0;
    uint32_t seed = 0;

    const CandidateStatus status =
        generate_candidate(options, fixed_kb6, fixed_msg, rng, thread_index,
                           step, name, len, seed, key);
    if (status == CandidateStatus::exhausted) {
      break;
    }

    local_tried++;
    if (status == CandidateStatus::invalid) {
      continue;
    }

    local_valid++;

    Counts counts;
    count_key(key, counts);

    if (better_counts(counts, key, local)) {
      set_hit(local, name, len, key, seed, counts);
      if (local.counts[0] >= options.print_zero) {
        print_hit("hit", local, options.day);
      }
    }
  }

  tried.fetch_add(local_tried, std::memory_order_relaxed);
  valid.fetch_add(local_valid, std::memory_order_relaxed);
  merge_hit(local, global, global_mutex, options.day);
}

} // namespace

bool better(const Hit &a, const Hit &b) {
  for (size_t i = 0; i < a.counts.size(); i++) {
    if (a.counts[i] != b.counts[i]) {
      return a.counts[i] > b.counts[i];
    }
  }
  return a.key < b.key;
}

bool better_counts(const Counts &counts, const char *key, const Hit &best) {
  for (size_t i = 0; i < counts.size(); i++) {
    if (counts[i] != best.counts[i]) {
      return counts[i] > best.counts[i];
    }
  }
  return best.key.empty() || strcmp(key, best.key.c_str()) < 0;
}

void set_hit(Hit &hit, const char *name, int nlen, const char *key,
             uint32_t seed, const Counts &counts) {
  hit.name.assign(name, nlen);
  hit.key = key;
  hit.seed = seed;
  hit.counts = counts;
}

void print_hit(const char *tag, const Hit &hit, int day) {
  printf("%s zeros=%d ones=%d twos=%d threes=%d As=%d day=%d seed=%u name=%s "
         "key=%s\n",
         tag, hit.counts[0], hit.counts[1], hit.counts[2], hit.counts[3],
         hit.counts[10], day, hit.seed, hit.name.c_str(), hit.key.c_str());
  fflush(stdout);
}

void run_search(const Options &options, const uint8_t fixed_kb6[6],
                u128 fixed_msg) {
  Hit global;
  std::mutex global_mutex;
  std::atomic<uint64_t> tried{0};
  std::atomic<uint64_t> valid{0};
  auto stop = std::make_shared<std::atomic<bool>>(false);
  auto start = std::chrono::steady_clock::now();
  const int seconds = options.seconds;

  std::thread([stop, seconds] {
    std::this_thread::sleep_for(std::chrono::seconds(seconds));
    stop->store(true, std::memory_order_relaxed);
  }).detach();

  std::vector<std::thread> threads;
  threads.reserve(options.threads);

  for (int t = 0; t < options.threads; t++) {
    threads.emplace_back(worker, std::ref(options), fixed_kb6, fixed_msg, t,
                         std::ref(*stop), std::ref(tried), std::ref(valid),
                         std::ref(global), std::ref(global_mutex));
  }

  for (std::thread &thread : threads) {
    thread.join();
  }

  print_hit("final", global, options.day);

  const double elapsed =
      std::chrono::duration<double>(std::chrono::steady_clock::now() - start)
          .count();
  const uint64_t tried_count = tried.load(std::memory_order_relaxed);
  const uint64_t valid_count = valid.load(std::memory_order_relaxed);

  printf("stats tried=%llu valid=%llu elapsed=%.3fs tried_per_sec=%.0f "
         "valid_per_sec=%.0f\n",
         (unsigned long long)tried_count, (unsigned long long)valid_count,
         elapsed, tried_count / elapsed, valid_count / elapsed);
}

} // namespace keyvis
