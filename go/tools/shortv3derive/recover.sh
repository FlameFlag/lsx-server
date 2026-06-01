#!/usr/bin/env nix-shell
#! nix-shell -i bash -p bash coreutils go_1_26 sage
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "$script_dir/../../.." && pwd)"
tmp_dir="${TMPDIR:-/tmp}/lt2-shortv3-recovery"

mkdir -p "$tmp_dir"

cd "$repo_root/go"

echo "[1/3] Deriving mapper metadata and validation entry from packed Lemonade2.exe"
go run ./tools/lt2normalize \
  -derive-mapper-window "$tmp_dir/mapper_window.bin" \
  -derive-validation-entry "$tmp_dir/validation_entry.bin"

echo
echo "[2/3] Verifying recovered ShortV3 parameters and checked-in exponent"
go run ./tools/shortv3derive

echo
echo "[3/3] Recovering the private exponent from the public certificate with Sage"
if ! command -v sage >/dev/null 2>&1; then
  cat >&2 <<'EOF'
Sage is required for the full independent discrete-log recovery.

Install Sage separately and rerun:
  make recover-shortv3
EOF
  exit 127
fi

sage "$script_dir/recover.sage"
