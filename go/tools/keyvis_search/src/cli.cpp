#include "keyvis/cli.hpp"

#include "keyvis/keygen.hpp"

#include <array>
#include <charconv>
#include <chrono>
#include <cstdlib>
#include <string>
#include <string_view>
#include <system_error>

namespace keyvis {
namespace {

enum class OptionKind {
  threads,
  seconds,
  day,
  max_len,
  print_zero,
  name,
  seed_start,
  seed_count,
  version,
  help,
};

struct OptionSpec {
  std::string_view short_name;
  std::string_view long_name;
  std::string_view value_name;
  std::string_view description;
  OptionKind kind;

  [[nodiscard]] constexpr bool takes_value() const {
    return !value_name.empty();
  }
};

constexpr std::array kOptionSpecs = {
    OptionSpec{"-t", "--threads", "N",
               "worker threads (default: hardware threads)",
               OptionKind::threads},
    OptionSpec{"-s", "--seconds", "N", "search duration (default: 60)",
               OptionKind::seconds},
    OptionSpec{"-d", "--day", "N", "Armadillo day (default: today)",
               OptionKind::day},
    OptionSpec{"-l", "--max-len", "N",
               "max random username length, 1..49 (default: 40)",
               OptionKind::max_len},
    OptionSpec{"-p", "--print-zero", "N",
               "print hits with at least N zeroes (default: 12)",
               OptionKind::print_zero},
    OptionSpec{"-n", "--name", "NAME",
               "fixed registration name, 1..49 chars; sweep seeds",
               OptionKind::name},
    OptionSpec{"", "--seed-start", "N",
               "first seed for fixed-name sweep (default: 1)",
               OptionKind::seed_start},
    OptionSpec{"", "--seed-count", "N",
               "seeds to test; 0 means until time expires (default: 0)",
               OptionKind::seed_count},
    OptionSpec{"", "--version", "", "print version information and exit",
               OptionKind::version},
    OptionSpec{"-h", "--help", "", "display this help and exit",
               OptionKind::help},
};

constexpr std::array kOutputRecords = {
    "hit    per-thread candidate meeting --print-zero",
    "best   best candidate merged from a worker",
    "final  best candidate for this run; enter name/key in register prompt",
};

const OptionSpec *find_option(std::string_view name) {
  for (const OptionSpec &option : kOptionSpecs) {
    if (name == option.short_name || name == option.long_name) {
      return &option;
    }
  }
  return nullptr;
}

struct ParsedArg {
  std::string name;
  std::string value;
  bool has_inline_value = false;
};

ParsedArg split_arg(std::string_view arg) {
  ParsedArg parsed{std::string(arg), {}, false};
  const size_t eq = arg.find('=');
  if (eq != std::string_view::npos) {
    parsed.name.assign(arg.substr(0, eq));
    parsed.value.assign(arg.substr(eq + 1));
    parsed.has_inline_value = true;
  }
  return parsed;
}

std::string_view take_value(const std::string &value, bool has_inline_value,
                            int &index, int argc, char **argv) {
  if (has_inline_value) {
    return value;
  }

  if (index + 1 >= argc) {
    usage(stderr, argv[0]);
    std::exit(2);
  }

  return argv[++index];
}

[[noreturn]] void invalid_option_value(const char *program,
                                       std::string_view option,
                                       std::string_view value) {
  fprintf(stderr, "%s: invalid value '%.*s' for %.*s\n", program,
          int(value.size()), value.data(), int(option.size()), option.data());
  std::exit(2);
}

void parse_int_option(int &out, const std::string &value, bool has_inline_value,
                      int &index, int argc, char **argv,
                      std::string_view option) {
  const std::string_view text =
      take_value(value, has_inline_value, index, argc, argv);
  int parsed = 0;
  const auto [ptr, ec] =
      std::from_chars(text.data(), text.data() + text.size(), parsed);
  if (ec != std::errc{} || ptr != text.data() + text.size()) {
    invalid_option_value(argv[0], option, text);
  }
  out = parsed;
}

bool consume_prefix(std::string_view &text, std::string_view prefix) {
  if (!text.starts_with(prefix)) {
    return false;
  }
  text.remove_prefix(prefix.size());
  return true;
}

void parse_uint64_option(uint64_t &out, const std::string &value,
                         bool has_inline_value, int &index, int argc,
                         char **argv, std::string_view option) {
  const std::string_view original =
      take_value(value, has_inline_value, index, argc, argv);
  std::string_view text = original;
  int base = 10;

  if (consume_prefix(text, "0x") || consume_prefix(text, "0X")) {
    base = 16;
  } else if (text.size() > 1 && text.starts_with('0')) {
    base = 8;
  }

  uint64_t parsed = 0;
  const auto [ptr, ec] =
      std::from_chars(text.data(), text.data() + text.size(), parsed, base);
  if (text.empty() || ec != std::errc{} || ptr != text.data() + text.size()) {
    invalid_option_value(argv[0], option, original);
  }
  out = parsed;
}

} // namespace

int current_day() {
  using days = std::chrono::duration<int64_t, std::ratio<86400>>;
  const auto epoch_days =
      std::chrono::duration_cast<days>(
          std::chrono::system_clock::now().time_since_epoch())
          .count();
  return int(epoch_days - 10592);
}

void version() {
  puts("keyvis-search 0.6");
  puts("License: public domain / no warranty.");
}

void usage(FILE *f, const char *program) {
  fprintf(f, "Usage: %s [OPTION...]\n", program);
  fputs("Search Armadillo ShortV3 keygen outputs for visually low "
        "activation keys.\n\n",
        f);
  fputs("Options:\n", f);

  for (const OptionSpec &option : kOptionSpecs) {
    std::string names;

    if (!option.short_name.empty()) {
      names += option.short_name;
      if (option.takes_value()) {
        names += ' ';
        names += option.value_name;
      }
    }

    if (!option.long_name.empty()) {
      if (!names.empty()) {
        names += ", ";
      }
      names += option.long_name;
      if (option.takes_value()) {
        names += '=';
        names += option.value_name;
      }
    }

    fprintf(f, "  %-24s %.*s\n", names.c_str(), int(option.description.size()),
            option.description.data());
  }

  fprintf(f, "\nCurrent default day: %d\n\n", current_day());
  fputs("Output records:\n", f);
  for (std::string_view record : kOutputRecords) {
    fprintf(f, "  %.*s\n", int(record.size()), record.data());
  }
}

bool parse_options(int argc, char **argv, Options &options) {
  options.day = current_day();

  for (int i = 1; i < argc; i++) {
    const ParsedArg arg = split_arg(argv[i]);
    const OptionSpec *option = find_option(arg.name);

    if (!option) {
      fprintf(stderr, "%s: unrecognized option '%s'\n", argv[0], argv[i]);
      usage(stderr, argv[0]);
      std::exit(2);
    }

    if (!option->takes_value() && arg.has_inline_value) {
      fprintf(stderr, "%s: option '%.*s' does not take a value\n", argv[0],
              int(option->long_name.size()), option->long_name.data());
      usage(stderr, argv[0]);
      std::exit(2);
    }

    switch (option->kind) {
    case OptionKind::threads:
      parse_int_option(options.threads, arg.value, arg.has_inline_value, i,
                       argc, argv, arg.name);
      break;
    case OptionKind::seconds:
      parse_int_option(options.seconds, arg.value, arg.has_inline_value, i,
                       argc, argv, arg.name);
      break;
    case OptionKind::day:
      parse_int_option(options.day, arg.value, arg.has_inline_value, i, argc,
                       argv, arg.name);
      break;
    case OptionKind::max_len:
      parse_int_option(options.max_len, arg.value, arg.has_inline_value, i,
                       argc, argv, arg.name);
      break;
    case OptionKind::print_zero:
      parse_int_option(options.print_zero, arg.value, arg.has_inline_value, i,
                       argc, argv, arg.name);
      break;
    case OptionKind::name:
      options.fixed_name =
          take_value(arg.value, arg.has_inline_value, i, argc, argv);
      options.has_fixed_name = true;
      break;
    case OptionKind::seed_start:
      parse_uint64_option(options.seed_start, arg.value, arg.has_inline_value,
                          i, argc, argv, arg.name);
      break;
    case OptionKind::seed_count:
      parse_uint64_option(options.seed_count, arg.value, arg.has_inline_value,
                          i, argc, argv, arg.name);
      break;
    case OptionKind::version:
      version();
      return false;
    case OptionKind::help:
      usage(stdout, argv[0]);
      return false;
    }
  }

  if (options.threads < 1) {
    options.threads = 1;
  }
  if (options.max_len < 1) {
    options.max_len = 1;
  }
  if (options.max_len > 49) {
    options.max_len = 49;
  }

  return true;
}

bool prepare_fixed_name(const char *program, const Options &options,
                        uint8_t kb6[6], u128 &msg) {
  if (!options.has_fixed_name) {
    return true;
  }

  const int fixed_len = int(options.fixed_name.size());
  if (fixed_len > 49) {
    fprintf(stderr, "%s: --name is limited to 49 characters\n", program);
    return false;
  }

  if (!prepare_name(options.fixed_name.c_str(), fixed_len, options.day, kb6,
                    msg)) {
    fprintf(stderr, "%s: --name must not be empty\n", program);
    return false;
  }

  return true;
}

void print_config(const Options &options) {
  printf("config threads=%d seconds=%d day=%d max_len=%d print_zero=%d",
         options.threads, options.seconds, options.day, options.max_len,
         options.print_zero);

  if (options.has_fixed_name) {
    printf(" name=%s seed_start=%llu seed_count=%llu",
           options.fixed_name.c_str(), (unsigned long long)options.seed_start,
           (unsigned long long)options.seed_count);
  }

  printf("\n");
}

} // namespace keyvis
