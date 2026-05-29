#include "keyvis/cli.hpp"
#include "keyvis/keygen.hpp"
#include "keyvis/search.hpp"

int main(int argc, char **argv) {
  keyvis::init_keygen();

  if (!keyvis::self_test()) {
    return 1;
  }

  keyvis::Options options;
  if (!keyvis::parse_options(argc, argv, options)) {
    return 0;
  }

  uint8_t fixed_kb6[6]{};
  keyvis::u128 fixed_msg = 0;
  if (!keyvis::prepare_fixed_name(argv[0], options, fixed_kb6, fixed_msg)) {
    return 2;
  }

  keyvis::print_config(options);
  keyvis::run_search(options, fixed_kb6, fixed_msg);
  return 0;
}
