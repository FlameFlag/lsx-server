#!/usr/bin/env python3
"""Correlate generated PE state changes with table-driven decrypt rows."""

from __future__ import annotations

import argparse
import re
from dataclasses import dataclass
from pathlib import Path

from lib.binary import changed_byte_ranges, read_u32, sha256_hex
from lib.reporting import write_markdown
from inspectors.generated.pe import (
    GUARD_TEXT,
    find_call_sites,
    parse_base,
    parse_pe,
    runtime_to_offset,
    summarize_guard_call,
)


@dataclass(frozen=True)
class Range:
    start: int
    end: int

    @property
    def size(self) -> int:
        return self.end - self.start


@dataclass(frozen=True)
class State:
    seq: int
    path: Path
    base: int
    data: bytes


@dataclass(frozen=True)
class DecryptRow:
    call: int
    hint: int
    sources: tuple[Range, ...]
    source_text: str
    length_text: str


def parse_state(path: Path) -> State:
    seq_match = re.search(r"seq(\d+)", path.name)
    seq = int(seq_match.group(1)) if seq_match else 0
    return State(seq=seq, path=path, base=parse_base(path, None), data=path.read_bytes())


def range_overlap(a: Range, b: Range) -> int:
    return max(0, min(a.end, b.end) - max(a.start, b.start))


def decrypt_rows(state: State) -> list[DecryptRow]:
    image_base, _entry, sections = parse_pe(state.data)

    def global_u32(address: int) -> int | None:
        return read_u32(state.data, runtime_to_offset(address, state.base))

    rows: list[DecryptRow] = []
    for call_site in find_call_sites(state.data, sections, GUARD_TEXT, image_base):
        row = summarize_guard_call(state.data, sections, image_base, state.base, call_site)
        source_addresses = row.source_offsets
        length_addresses = row.length_offsets
        sources: list[Range] = []
        source_text: list[str] = []
        length_text: list[str] = []
        for index, source_addr in enumerate(source_addresses):
            source = global_u32(source_addr)
            length_addr = length_addresses[index] if index < len(length_addresses) else None
            length = global_u32(length_addr) if length_addr is not None else None
            if source is not None:
                source_text.append(f"0x{source:X}")
            if length is not None:
                length_text.append(f"0x{length:X}")
            if source is not None and length is not None and length > 0:
                sources.append(Range(source, source + length))
        rows.append(
            DecryptRow(
                call=row.call,
                hint=row.function_hint,
                sources=tuple(sources),
                source_text=", ".join(source_text),
                length_text=", ".join(length_text),
            )
        )
    return rows


def rank_matches(changes: list[Range], rows: list[DecryptRow]) -> list[tuple[int, DecryptRow]]:
    ranked: list[tuple[int, DecryptRow]] = []
    for row in rows:
        score = 0
        for change in changes:
            for source in row.sources:
                score += range_overlap(change, source)
        if score:
            ranked.append((score, row))
    ranked.sort(key=lambda item: item[0], reverse=True)
    return ranked


def merge_rows(primary: list[DecryptRow], fallback: list[DecryptRow]) -> list[DecryptRow]:
    seen: set[tuple[int, tuple[tuple[int, int], ...]]] = set()
    merged: list[DecryptRow] = []
    for row in primary + fallback:
        key = (row.call, tuple((item.start, item.end) for item in row.sources))
        if key in seen:
            continue
        seen.add(key)
        merged.append(row)
    return merged


def format_ranges(ranges: list[Range], limit: int) -> str:
    return ", ".join(f"0x{item.start:X}-0x{item.end:X}" for item in ranges[:limit]) or "none"


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("run_dir", type=Path)
    parser.add_argument("--output", type=Path)
    parser.add_argument("--min-run", type=int, default=8)
    parser.add_argument("--range-limit", type=int, default=10)
    args = parser.parse_args()

    states = [parse_state(path) for path in sorted(args.run_dir.glob("generated_island_state_*.bin"))]
    if len(states) < 2:
        raise SystemExit("need at least two generated_island_state_*.bin files")

    final_rows = decrypt_rows(states[-1])
    lines = [
        "# Generated PE Timeline Analysis",
        "",
        f"Run directory: `{args.run_dir}`",
        f"States: `{len(states)}`",
        f"Decrypt rows in final state: `{len(final_rows)}`",
        "",
        "## State Transitions",
        "",
    ]

    for previous, current in zip(states, states[1:]):
        rows = merge_rows(decrypt_rows(current), final_rows)
        _total, raw_changes = changed_byte_ranges(previous.data, current.data, min_run=args.min_run)
        changes = [Range(start, end) for start, end in raw_changes]
        total = sum(item.size for item in changes)
        ranked = rank_matches(changes, rows)
        lines.append(
            f"- seq `{previous.seq}` -> `{current.seq}` sha `{sha256_hex(current.data)[:16]}` changed_runs `{len(changes)}` changed_bytes `0x{total:X}` ranges `{format_ranges(changes, args.range_limit)}`"
        )
        if ranked:
            for score, row in ranked[:4]:
                row_ranges = ", ".join(f"0x{item.start:X}-0x{item.end:X}" for item in row.sources) or "none"
                lines.append(
                    f"  match score `0x{score:X}` call `0x{row.call:08X}` hint `0x{row.hint:08X}` sources `{row_ranges}` lengths `{row.length_text or '-'}`"
                )
        else:
            lines.append("  no matching decrypt-row source range")

    if args.output:
        write_markdown(args.output, lines)
    else:
        print("\n".join(lines) + "\n", end="")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
