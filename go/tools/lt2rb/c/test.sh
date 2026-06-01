#!/usr/bin/env bash
set -Eeuo pipefail

root=$(cd "$(dirname "$0")" && pwd)
tmp=${TMPDIR:-/tmp}/lt2rb-c-test-$$
mkdir -p "$tmp"
trap 'rm -rf "$tmp"' EXIT

build_bin() {
  local out=$1
  (cd "$root" && ./build.sh "$out")
}

bin="$tmp/lt2rb"
build_bin "$bin"
(cd "$root" && CFLAGS_EXTRA='-Werror' ./build.sh "$tmp/lt2rb-werror") >/dev/null

if [[ ${LT2RB_USE_SANITIZER:-0} == 1 ]]; then
  (cd "$root" && \
    CFLAGS_EXTRA='-fsanitize=address,undefined -fno-omit-frame-pointer -g -Werror' \
    LDFLAGS_EXTRA='-fsanitize=address,undefined' \
    ./build.sh "$tmp/lt2rb-asan") >/dev/null
  bin="$tmp/lt2rb-asan"
  echo "using sanitizer build"
fi

python3 - "$tmp/synthetic.rb" <<'PY'
from pathlib import Path
import struct, sys
out = Path(sys.argv[1])
seg_off = 0x1038
rb = bytearray(seg_off)
struct.pack_into('<hhhhIIII', rb, 0, -1, 800, 600, 16, 0x848EDDC3, 0, seg_off, 0x408)
records = bytearray()
# RGB565 + alpha: red opaque, white transparent-alpha byte.
records += struct.pack('<HHII', 2, 1, 0x8760, 0) + bytes([0x00, 0xF8, 0xFF, 0xFF, 0xFF, 0x00])
# RGB565 with high format bits and magenta chroma-key flag.
records += struct.pack('<HHII', 2, 1, 0x20660, 0x00FF00FF) + bytes([0xE0, 0x07, 0x1F, 0xF8])
# 8-bit grayscale.
records += struct.pack('<HHII', 1, 1, 0xC300, 0) + bytes([0x80])
end = seg_off + 12 + len(records)
rb += struct.pack('<III', 2, end, 3) + records
out.write_bytes(rb)
PY

"$bin" --rb-input --validate-rb --quiet "$tmp/synthetic.rb"
"$bin" --rb-input --validate-rb --extract-images "$tmp/png" --quiet "$tmp/synthetic.rb"
[[ $(find "$tmp/png" -type f -name '*.png' | wc -l | tr -d ' ') == 3 ]]

python3 - "$tmp/synthetic.rb" "$tmp/truncated.rb" "$tmp/badnext.rb" <<'PY'
from pathlib import Path
import struct, sys
src = Path(sys.argv[1]).read_bytes()
Path(sys.argv[2]).write_bytes(src[:-1])
bad = bytearray(src)
struct.pack_into('<I', bad, 0x1038 + 4, 0x1000)
Path(sys.argv[3]).write_bytes(bad)
PY
if "$bin" --rb-input --validate-rb --quiet "$tmp/truncated.rb" >/dev/null 2>&1; then
  echo "truncated rb unexpectedly validated" >&2
  exit 1
fi
if "$bin" --rb-input --validate-rb --quiet "$tmp/badnext.rb" >/dev/null 2>&1; then
  echo "bad next_offset unexpectedly validated" >&2
  exit 1
fi

printf 'hello rb\n' > "$tmp/hello.rb"
bzip2 -c -9 "$tmp/hello.rb" > "$tmp/hello.rb.bz2"
python3 - "$tmp/installer.bin" "$tmp/hello.rb.bz2" <<'PY'
from pathlib import Path
import sys
Path(sys.argv[1]).write_bytes(b'JUNK' + Path(sys.argv[2]).read_bytes() + b'TRAILER')
PY
"$bin" --scan --quiet "$tmp/installer.bin" | grep -q '0x4 (4)'
"$bin" --offset 4 --length "$(wc -c < "$tmp/hello.rb.bz2" | tr -d ' ')" --output "$tmp/out.rb" --quiet "$tmp/installer.bin"
cmp "$tmp/hello.rb" "$tmp/out.rb"
"$bin" --compress-rb --output "$tmp/recompressed.bz2" --quiet "$tmp/out.rb"
"$bin" --offset 0 --length 0 --output "$tmp/recompressed.out" --quiet "$tmp/recompressed.bz2"
cmp "$tmp/hello.rb" "$tmp/recompressed.out"

python3 - "$tmp/fuzz" <<'PY'
from pathlib import Path
import os, random, struct, sys
root = Path(sys.argv[1]); root.mkdir()
rng = random.Random(0x1A9848E)
for i in range(256):
    n = rng.randrange(0, 9000)
    data = bytearray(os.urandom(n))
    # Sometimes make the data look close to an RB to exercise deeper paths.
    if n >= 0x1038 + 12 and i % 4 == 0:
        struct.pack_into('<hhhhIIII', data, 0, -1, 800, 600, 16, 0x848EDDC3, 0, 0x1038, 0x408)
        nxt = rng.randrange(0, n + 1024)
        struct.pack_into('<III', data, 0x1038, rng.randrange(0, 16), nxt, rng.randrange(0, 300))
    (root / f'{i:03}.rb').write_bytes(data)
PY
for f in "$tmp"/fuzz/*.rb; do
  "$bin" --rb-input --validate-rb --quiet "$f" >/dev/null 2>&1 || true
done

real_rb="$root/../../../../decompiled/local/lt2_install/Lemonade2.rb"
if [[ -f "$real_rb" ]]; then
  "$bin" --rb-input --validate-rb --quiet "$real_rb"
  rm -rf "$tmp/real-png"
  "$bin" --rb-input --validate-rb --extract-images "$tmp/real-png" --quiet "$real_rb"
  [[ $(find "$tmp/real-png" -type f -name '*.png' | wc -l | tr -d ' ') == 233 ]]
fi

echo "lt2rb C tests passed"
