#!/usr/bin/env python3
"""Enumerate plausible FUN_1000773B chunk chains in packed .data1."""

from __future__ import annotations

import argparse
import struct
from pathlib import Path

from lib.common import load_pe_sections, repo_root
from lib.generated_crypto import rle_decompress
from lib.reporting import write_markdown


def read_chunk(data: bytes, offset: int, key: int, limit: int = 0x20000) -> tuple[int, bool, bytes] | None:
    if offset < 0 or offset + 4 > len(data):
        return None
    raw = struct.unpack_from("<I", data, offset)[0] ^ key
    stored = bool(raw & 0x80000000)
    size = raw & 0x7FFFFFFF
    if size == 0 or size > limit or offset + 4 + size > len(data):
        return None
    source = data[offset + 4 : offset + 4 + size]
    decoded = source if stored else rle_decompress(source, limit)
    if decoded is None:
        return None
    return offset + 4 + size, stored, decoded


def printable_score(data: bytes) -> float:
    if not data:
        return 0.0
    sample = data[: min(len(data), 128)]
    return sum(1 for item in sample if item in (0, 9, 10, 13) or 32 <= item < 127) / len(sample)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--packed", type=Path, default=repo_root() / "decompiled/local/lt2_install/Lemonade2.exe")
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--key", type=lambda value: int(value, 0), default=0)
    args = parser.parse_args()

    sections = load_pe_sections(args.packed)
    data1 = sections[".data1"].data[:0x9000]
    lines = ["# .data1 Chunk Chain Enumeration", "", f"Input: `{args.packed}`", f"Length key: `0x{args.key:08X}`", ""]
    candidates = []
    for offset in range(0, len(data1) - 8):
        cursor = offset
        decoded_total = 0
        chunks = []
        for _ in range(16):
            chunk = read_chunk(data1, cursor, args.key)
            if chunk is None:
                break
            next_cursor, stored, decoded = chunk
            chunks.append((cursor, next_cursor - cursor - 4, stored, len(decoded), decoded[:64]))
            decoded_total += len(decoded)
            cursor = next_cursor
            if next_cursor >= len(data1):
                break
        if not chunks:
            continue
        score = max(printable_score(item[4]) for item in chunks)
        if len(chunks) >= 2 or decoded_total >= 0x400 or score > 0.75:
            candidates.append((offset, len(chunks), decoded_total, score, chunks))
    candidates.sort(key=lambda item: (-item[1], -item[2], item[0]))
    lines.append(f"Candidates: `{len(candidates)}`")
    for offset, count, total, score, chunks in candidates[:100]:
        lines.extend(["", f"## Offset `0x{offset:X}`", "", f"- chunks: `{count}`", f"- decoded_total: `0x{total:X}`", f"- printable_score: `{score:.2f}`"])
        for chunk_offset, source_size, stored, decoded_size, preview in chunks[:12]:
            preview_text = preview.decode("latin1", errors="replace").replace("\0", "\\0")
            lines.append(
                f"- chunk `0x{chunk_offset:X}` src `0x{source_size:X}` {'stored' if stored else 'rle'} out `0x{decoded_size:X}` preview `{preview_text}`"
            )
    write_markdown(args.output, lines)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
