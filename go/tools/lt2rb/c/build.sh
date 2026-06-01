#!/usr/bin/env nix-shell
#! nix-shell -i bash -p bash clang pkg-config bzip2 libpng zlib coreutils gnused
set -Eeuo pipefail

out=${1:-lt2rb}
std=${STD:-c23}
cc_bin=${CC:-cc}
cflags=(-Iinclude)
ldflags=()
libs=(-lbz2 -lpng -lz)
sources=(src/lt2rb.c src/bzip2_stream.c src/rb.c src/md5.c src/util.c src/error.c)

if [[ -n ${CFLAGS_EXTRA:-} ]]; then
  # shellcheck disable=SC2206 # intentional simple word splitting for compiler flags
  extra_cflags=(${CFLAGS_EXTRA})
  cflags+=("${extra_cflags[@]}")
fi
if [[ -n ${LDFLAGS_EXTRA:-} ]]; then
  # shellcheck disable=SC2206 # intentional simple word splitting for linker flags
  extra_ldflags=(${LDFLAGS_EXTRA})
  ldflags+=("${extra_ldflags[@]}")
fi

if command -v pkg-config >/dev/null && pkg-config --exists libpng; then
  # bzip2 commonly ships without a pkg-config file; libpng carries zlib flags.
  while IFS= read -r flag; do cflags+=("$flag"); done < <(pkg-config --cflags-only-I libpng | tr ' ' '\n' | sed '/^$/d')
  while IFS= read -r flag; do ldflags+=("$flag"); done < <(pkg-config --libs-only-L libpng | tr ' ' '\n' | sed '/^$/d')
fi

"$cc_bin" -std="$std" -Wall -Wextra -O2 "${cflags[@]}" "${sources[@]}" "${ldflags[@]}" "${libs[@]}" -o "$out"

abs_dir=$(pwd)
{
  printf '[\n'
  for i in "${!sources[@]}"; do
    source=${sources[$i]}
    comma=','
    [[ $i == $((${#sources[@]} - 1)) ]] && comma=''
    printf '  {"directory":"%s","command":"%s -std=%s -Wall -Wextra -O2' "$abs_dir" "$cc_bin" "$std"
    for flag in "${cflags[@]}"; do printf ' %s' "$flag"; done
    printf ' -c %s","file":"%s/%s"}%s\n' "$source" "$abs_dir" "$source" "$comma"
  done
  printf ']\n'
} > compile_commands.json
