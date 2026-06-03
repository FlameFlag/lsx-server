#!/usr/bin/env bash
set -Eeuo pipefail

root=$(cd "$(dirname "$0")" && pwd)
tmp=${TMPDIR:-/tmp}/lt2rb-c-test-$$
mkdir -p "$tmp"
trap 'rm -rf "$tmp"' EXIT

bin="$tmp/lt2rb"
(cd "$root" && ./build.sh "$bin")
(cd "$root" && CFLAGS_EXTRA='-Werror' ./build.sh "$tmp/lt2rb-werror") >/dev/null

mkdir -p "$tmp/assets/nested/empty"
printf 'root\n' > "$tmp/assets/root.txt"
printf '\000\001\002\003\004' > "$tmp/assets/nested/data.bin"
"$bin" -quiet pack "$tmp/assets" "$tmp/assets.rb"
"$bin" -quiet unpack "$tmp/assets.rb" "$tmp/out"
cmp "$tmp/assets/root.txt" "$tmp/out/root.txt"
cmp "$tmp/assets/nested/data.bin" "$tmp/out/nested/data.bin"
[[ -d "$tmp/out/nested/empty" ]]

printf 'single\n' > "$tmp/single.txt"
"$bin" -quiet compress "$tmp/single.txt" "$tmp/single.rb"
"$bin" -quiet decompress "$tmp/single.rb" "$tmp/single-out"
cmp "$tmp/single.txt" "$tmp/single-out/single.txt"

python3 - "$tmp/synthetic.rb" <<'PY'
from pathlib import Path
import struct, sys
out = Path(sys.argv[1])
seg_off = 0x1038
rb = bytearray(seg_off)
struct.pack_into('<hhhhIIII', rb, 0, -1, 800, 600, 16, 0x848EDDC3, 0, seg_off, 0x408)
records = bytearray()
records += struct.pack('<HHII', 2, 1, 0x8760, 0) + bytes([0x00, 0xF8, 0xFF, 0xFF, 0xFF, 0x00])
records += struct.pack('<HHII', 2, 1, 0x20660, 0x00FF00FF) + bytes([0xE0, 0x07, 0x1F, 0xF8])
records += struct.pack('<HHII', 1, 1, 0xC300, 0) + bytes([0x80])
end = seg_off + 12 + len(records)
rb += struct.pack('<III', 2, end, 3) + records
out.write_bytes(rb)
PY
"$bin" -quiet unpack "$tmp/synthetic.rb" "$tmp/synthetic-out"
[[ $(find "$tmp/synthetic-out/bitmaps" -type f -name '*.png' | wc -l | tr -d ' ') == 3 ]]

python3 - "$tmp/bad-archive.rb" <<'PY'
from pathlib import Path
import struct, sys
Path(sys.argv[1]).write_bytes(
    b'LT2RBFS1' +
    struct.pack('<I', 1) +
    struct.pack('<IIIQQ', len('../bad.txt'), 2, 0o755, 0, 0) +
    b'../bad.txt'
)
PY
if "$bin" -quiet unpack "$tmp/bad-archive.rb" "$tmp/bad-out" >/dev/null 2>&1; then
  echo "unsafe archive path unexpectedly unpacked" >&2
  exit 1
fi

if "$bin" -offset 4 installer.exe out.rb >/dev/null 2>&1; then
  echo "obsolete installer mode unexpectedly accepted" >&2
  exit 1
fi

echo "lt2rb C tests passed"
