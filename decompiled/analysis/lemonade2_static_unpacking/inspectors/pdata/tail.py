#!/usr/bin/env python3
"""Analyze the .pdata tail selected by the generated mapper."""

from __future__ import annotations

import argparse
import struct
import zipfile
import zlib
from pathlib import Path

from lib.binary import printable_strings, sha256_hex
from lib.common import load_pe_sections, repo_root
from lib.capture import first_member_containing
from lib.generated_crypto import (
    dword_xor_to_prng_bytes,
    prng_sequence_has_seed,
    recover_prng_seed,
    xor_dwords_with_prng,
)
from lib.mapper import mapper_magic_from_data1, summarize_mapper_metadata
from lib.reporting import write_markdown


PDATA_SIGNATURE = b"PDATA000"
FIRST_METADATA_SIZE = 11
RECOVERED_TAIL_SIZE = 0x2804
TAIL_WINDOW_RVA = 0x12B52A
MAPPER_SELECTED_VA = 0x0052B52E
MAPPER_FIRST_CURSOR_VA = 0x0052C1B9
MAPPER_FINAL_CURSOR_VA = 0x00572CD8
def inflate_stream(data: bytes, offset: int) -> tuple[bytes, int] | None:
    try:
        obj = zlib.decompressobj()
        out = obj.decompress(data[offset:]) + obj.flush()
    except zlib.error:
        return None
    consumed = len(data[offset:]) - len(obj.unused_data)
    if consumed <= 0:
        return None
    return out, consumed


def load_capture_mapper_buffer(capture: Path | None) -> bytes | None:
    if capture is None or not capture.exists():
        return None
    with zipfile.ZipFile(capture) as archive:
        return first_member_containing(archive, "mapper_global_f2e4a0")


def find_next_zlib(data: bytes, start: int) -> int:
    for offset in range(start, len(data) - 1):
        if data[offset] == 0x78 and data[offset + 1] in (0x9C, 0xDA):
            return offset
    return -1


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--packed", type=Path, default=repo_root() / "decompiled/local/lt2_install/Lemonade2.exe")
    parser.add_argument(
        "--asset",
        type=Path,
        default=repo_root() / "tools/lt2normalize/internal/normalizer/assets/pdata/recovered-tail-payload.bin",
    )
    parser.add_argument("--capture", type=Path, default=Path("runs/latest/capture.zip"))
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()

    sections = load_pe_sections(args.packed)
    pdata = sections[".pdata"]
    data = pdata.data
    if not data.startswith(PDATA_SIGNATURE):
        raise SystemExit(".pdata signature mismatch")

    stream_offset = len(PDATA_SIGNATURE) + FIRST_METADATA_SIZE
    last_end = stream_offset
    streams: list[tuple[int, int, int, str]] = []
    while stream_offset >= 0 and stream_offset < len(data) - 1:
        inflated = inflate_stream(data, stream_offset)
        if inflated is None:
            if not streams:
                raise SystemExit(f"first zlib stream at 0x{stream_offset:X} failed")
            stream_offset = find_next_zlib(data, stream_offset + 1)
            continue
        payload, consumed = inflated
        streams.append((stream_offset, consumed, len(payload), sha256_hex(payload)))
        last_end = stream_offset + consumed
        stream_offset = find_next_zlib(data, last_end)

    tail = data[last_end:]
    recovered_payload = args.asset.read_bytes()
    if len(recovered_payload) != RECOVERED_TAIL_SIZE - 4:
        raise SystemExit(f"unexpected recovered payload size 0x{len(recovered_payload):X}")
    recovered_header = tail[:4] + recovered_payload

    selected_tail_offset = MAPPER_SELECTED_VA - pdata.va - last_end
    first_cursor_tail_offset = MAPPER_FIRST_CURSOR_VA - pdata.va - last_end
    final_cursor_tail_offset = MAPPER_FINAL_CURSOR_VA - pdata.va - last_end
    selected_window_offset = MAPPER_SELECTED_VA - pdata.va
    first_cursor_window_offset = MAPPER_FIRST_CURSOR_VA - pdata.va
    final_cursor_window_offset = MAPPER_FINAL_CURSOR_VA - pdata.va

    encrypted_first = struct.unpack_from("<I", tail, selected_tail_offset)[0]
    recovered_first = struct.unpack_from("<I", recovered_header, selected_tail_offset)[0]
    xor_first = encrypted_first ^ recovered_first
    xor_bytes_little = xor_first.to_bytes(4, "little")
    xor_bytes_big = xor_first.to_bytes(4, "big")
    captured_mapper = load_capture_mapper_buffer(args.capture)
    data1 = sections[".data1"].data
    mapper_magic = mapper_magic_from_data1(data1)

    lines = [
        "# .pdata Tail Mapper Analysis",
        "",
        f"Input: `{args.packed}`",
        f"Recovered payload asset: `{args.asset}`",
        "",
        "## Zlib Streams",
        "",
    ]
    for index, (offset, consumed, size, digest) in enumerate(streams, start=1):
        lines.append(f"- `{index}` section `0x{offset:X}` compressed `0x{consumed:X}` out `0x{size:X}` sha256 `{digest}`")
    lines.extend(
        [
            "",
            "## Tail",
            "",
            f"- tail section offset: `0x{last_end:X}`",
            f"- tail VA: `0x{pdata.va + last_end:08X}`",
            f"- tail size: `0x{len(tail):X}`",
            f"- recovered header size: `0x{len(recovered_header):X}`",
            f"- recovered header sha256: `{sha256_hex(recovered_header)}`",
            "",
            "## Mapper Pointer Mapping",
            "",
            f"- `f2e4a0` selected VA `0x{MAPPER_SELECTED_VA:08X}` -> section offset `0x{selected_window_offset:X}`, tail offset `0x{selected_tail_offset:X}`",
            f"- first observed `f2e6e0` cursor VA `0x{MAPPER_FIRST_CURSOR_VA:08X}` -> section offset `0x{first_cursor_window_offset:X}`, tail offset `0x{first_cursor_tail_offset:X}`",
            f"- final island `f2e6e0` cursor VA `0x{MAPPER_FINAL_CURSOR_VA:08X}` -> section offset `0x{final_cursor_window_offset:X}`, tail offset `0x{final_cursor_tail_offset:X}`",
            f"- selected recovered bytes: `{recovered_header[selected_tail_offset:selected_tail_offset + 16].hex()}`",
            f"- first cursor packed bytes: `{tail[first_cursor_tail_offset:first_cursor_tail_offset + 16].hex()}`",
            f"- final cursor packed bytes: `{tail[final_cursor_tail_offset:final_cursor_tail_offset + 16].hex()}`",
            "",
            "## First Dword Check",
            "",
            f"- packed dword at selected VA: `0x{encrypted_first:08X}`",
            f"- recovered dword at selected VA: `0x{recovered_first:08X}`",
            f"- xor dword: `0x{xor_first:08X}`",
            f"- PRNG-compatible little-byte order: `{prng_sequence_has_seed(xor_bytes_little)}`",
            f"- PRNG-compatible big-byte order: `{prng_sequence_has_seed(xor_bytes_big)}`",
            "",
            "The canonical recovered-header first dword is not the immediate `FUN_1001F498` output; the captured mapper buffer below is the correct post-PRNG-XOR state for this stage.",
        ]
    )
    if captured_mapper is not None:
        lines.extend(["", "## Captured Mapper Buffer", ""])
        lines.append(f"- capture: `{args.capture}`")
        lines.append(f"- size: `0x{len(captured_mapper):X}`")
        lines.append(f"- sha256: `{sha256_hex(captured_mapper)}`")
        lines.append(f"- first bytes: `{captured_mapper[:32].hex()}`")
        captured_magic = struct.unpack_from("<I", captured_mapper, 0)[0]
        lines.append(f"- captured magic: `0x{captured_magic:08X}`")
        lines.append(f"- expected magic from `.data1`: `0x{mapper_magic:08X}`")
        lines.append(f"- magic matches `FUN_10019183` guard: `{captured_magic == mapper_magic}`")
        selected_raw = data[selected_window_offset : selected_window_offset + min(0x2800, len(captured_mapper))]
        xor_stream = bytes(left ^ right for left, right in zip(selected_raw[:8], captured_mapper[:8]))
        recovered_seed = recover_prng_seed(dword_xor_to_prng_bytes(xor_stream))
        if recovered_seed is not None:
            decoded_window = xor_dwords_with_prng(selected_raw, recovered_seed, min(0x2800, len(captured_mapper)))
            match_len = 0
            for left, right in zip(decoded_window, captured_mapper):
                if left != right:
                    break
                match_len += 1
            lines.append(f"- recovered PRNG seed from first two dwords: `0x{recovered_seed:08X}`")
            lines.append(f"- PRNG-XOR decoded match length: `0x{match_len:X}`")
            lines.append(f"- full `0x2800` window matches capture: `{match_len >= min(0x2800, len(captured_mapper))}`")
            first_cursor_in_window = first_cursor_window_offset - selected_window_offset
            lines.append(f"- first cursor offset inside decoded window: `0x{first_cursor_in_window:X}`")
            lines.append(
                f"- first cursor decoded bytes: `{decoded_window[first_cursor_in_window:first_cursor_in_window + 16].hex()}`"
            )
        zlib_offset = find_next_zlib(captured_mapper, 0)
        if zlib_offset >= 0:
            inflated = inflate_stream(captured_mapper, zlib_offset)
            if inflated is not None:
                payload, consumed = inflated
                lines.append(f"- first zlib offset: `0x{zlib_offset:X}`")
                lines.append(f"- first zlib compressed size: `0x{consumed:X}`")
                lines.append(f"- first zlib output size: `0x{len(payload):X}`")
                lines.append(f"- first zlib output sha256: `{sha256_hex(payload)}`")
                lines.extend(summarize_mapper_metadata(payload))
                for offset, text in printable_strings(payload)[:12]:
                    escaped = text.replace("`", "'")
                    lines.append(f"- string `0x{offset:X}`: `{escaped}`")
    write_markdown(args.output, lines)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
