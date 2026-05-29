#!/usr/bin/env python3
"""Verify generated PE state transitions against the TEA-like block routine."""

from __future__ import annotations

import argparse
import re
import zipfile
from dataclasses import dataclass
from pathlib import Path

from inspectors.generated.pe import GUARD_TEXT, find_call_sites, parse_base, parse_pe, runtime_to_offset, summarize_guard_call
from lib.binary import grouped_by_gap, read_u32
from lib.generated_crypto import crc32_seed, seed_tea_region as crypt_region
from lib.reporting import write_markdown


@dataclass(frozen=True)
class State:
    seq: int
    path: Path
    base: int
    data: bytes


@dataclass(frozen=True)
class Row:
    call: int
    hint: int
    source: int
    length: int


@dataclass(frozen=True)
class SeedCall:
    seq: int
    buf: int
    length: int
    seed_arg: int
    ret: int | None
    data: bytes = b""


@dataclass(frozen=True)
class TeaCall:
    seq: int
    seed: int
    ptr: int
    length: int
    mode: int
    pre: bytes = b""
    post: bytes = b""


def parse_state(path: Path) -> State:
    seq_match = re.search(r"seq(\d+)", path.name)
    seq = int(seq_match.group(1)) if seq_match else 0
    return State(seq, path, parse_base(path, None), path.read_bytes())


def rows_for_state(state: State) -> list[Row]:
    image_base, _entry, sections = parse_pe(state.data)

    def global_u32(address: int) -> int | None:
        return read_u32(state.data, runtime_to_offset(address, state.base))

    rows: list[Row] = []
    for call_site in find_call_sites(state.data, sections, GUARD_TEXT, image_base):
        summary = summarize_guard_call(state.data, sections, image_base, state.base, call_site)
        sources = summary.source_offsets
        lengths = summary.length_offsets
        for index, source_addr in enumerate(sources):
            length_addr = lengths[index] if index < len(lengths) else None
            source = global_u32(source_addr)
            length = global_u32(length_addr) if length_addr is not None else None
            if source is not None and length is not None and length:
                rows.append(Row(summary.call, summary.function_hint, source, length))
    return rows


def parse_seed_calls(run_dir: Path) -> list[SeedCall]:
    archive = run_dir / "capture.zip"
    if not archive.exists():
        return []
    with zipfile.ZipFile(archive) as zf:
        try:
            log = zf.read("trace\\trace.log").decode("latin1", errors="replace")
        except KeyError:
            return []
        names = set(zf.namelist())
        raw_calls = []
        for match in re.finditer(
            r"SEEDCALL#(\d+).*?buf=([0-9A-Fa-f]+) len=([0-9A-Fa-f]+) seed=([0-9A-Fa-f]+)", log
        ):
            seq = int(match.group(1))
            buf = int(match.group(2), 16)
            length = int(match.group(3), 16)
            sample_size = min(length, 0x10000)
            sample_name = f"trace\\seedbuf_{seq:05d}_src{buf:08X}_size{sample_size:08X}.bin"
            data = zf.read(sample_name) if sample_name in names else b""
            raw_calls.append((seq, buf, length, int(match.group(4), 16), data))
    returns = {
        int(match.group(1)): int(match.group(2), 16)
        for match in re.finditer(r"SEEDRET#(\d+).*?eax=([0-9A-Fa-f]+)", log)
    }
    calls: list[SeedCall] = []
    for seq, buf, length, seed_arg, data in raw_calls:
        calls.append(
            SeedCall(
                seq=seq,
                buf=buf,
                length=length,
                seed_arg=seed_arg,
                ret=returns.get(seq),
                data=data,
            )
        )
    return calls


def read_trace_log(run_dir: Path) -> tuple[str, zipfile.ZipFile | None]:
    archive = run_dir / "capture.zip"
    if not archive.exists():
        return "", None
    zf = zipfile.ZipFile(archive)
    try:
        return zf.read("trace\\trace.log").decode("latin1", errors="replace"), zf
    except KeyError:
        zf.close()
        return "", None


def parse_tea_calls(run_dir: Path) -> list[TeaCall]:
    log, zf = read_trace_log(run_dir)
    if not log or zf is None:
        return []
    try:
        names = set(zf.namelist())
        calls: dict[int, tuple[int, int, int, int, bytes]] = {}
        for match in re.finditer(
            r"TEACALL#(\d+).*?seed=([0-9A-Fa-f]+) ptr=([0-9A-Fa-f]+) len=([0-9A-Fa-f]+) mode=([0-9A-Fa-f]+)",
            log,
        ):
            seq = int(match.group(1))
            seed = int(match.group(2), 16)
            ptr = int(match.group(3), 16)
            length = int(match.group(4), 16)
            mode = int(match.group(5), 16)
            sample_size = min(length, 0x10000)
            name = f"trace\\teapre_{seq:05d}_src{ptr:08X}_size{sample_size:08X}.bin"
            calls[seq] = (seed, ptr, length, mode, zf.read(name) if name in names else b"")
        posts: dict[int, bytes] = {}
        for match in re.finditer(r"TEARET#(\d+).*?ptr=([0-9A-Fa-f]+) len=([0-9A-Fa-f]+)", log):
            seq = int(match.group(1))
            ptr = int(match.group(2), 16)
            length = int(match.group(3), 16)
            sample_size = min(length, 0x10000)
            name = f"trace\\teapost_{seq:05d}_src{ptr:08X}_size{sample_size:08X}.bin"
            if name in names:
                posts[seq] = zf.read(name)
        return [
            TeaCall(seq, seed, ptr, length, mode, pre, posts.get(seq, b""))
            for seq, (seed, ptr, length, mode, pre) in sorted(calls.items())
        ]
    finally:
        zf.close()


def changed_bytes(a: bytes, b: bytes, start: int, length: int) -> int:
    return sum(1 for x, y in zip(a[start : start + length], b[start : start + length]) if x != y)


def best_state_for_seed_window(states: list[State], base: int, call: SeedCall) -> tuple[int, int] | None:
    if not call.data or call.buf < base:
        return None
    offset = call.buf - base
    if offset < 0:
        return None
    best: tuple[int, int] | None = None
    for state in states:
        if offset + len(call.data) > len(state.data):
            continue
        diff = sum(1 for x, y in zip(call.data, state.data[offset : offset + len(call.data)]) if x != y)
        candidate = (diff, state.seq)
        if best is None or candidate < best:
            best = candidate
    return best


def merge_ranges(ranges: list[tuple[int, int, int]]) -> list[tuple[int, int, list[int]]]:
    return [(start, end, [seq for _start, _end, seq in items]) for start, end, items in grouped_by_gap(sorted(ranges), lambda item: (item[0], item[1]), 0)]


def build_plain_overlay(base_state: State, tea_calls: list[TeaCall]) -> tuple[bytes, list[tuple[int, int, list[int]]]]:
    image = bytearray(base_state.data)
    ranges: list[tuple[int, int, int]] = []
    for call in tea_calls:
        if call.mode != 0 or not call.post:
            continue
        if not (base_state.base <= call.ptr < base_state.base + len(image)):
            continue
        offset = call.ptr - base_state.base
        if offset + len(call.post) > len(image):
            continue
        image[offset : offset + len(call.post)] = call.post
        ranges.append((offset, offset + len(call.post), call.seq))
    return bytes(image), merge_ranges(ranges)


def candidate_seeds(previous: State, current: State, row: Row) -> list[int]:
    # These are the visible seed-adjacent globals/values in the generated PE.
    # The real seed is usually produced by the external callback, so this list is
    # mostly a sanity check and a way to detect rows that do not need the callback.
    values = {
        0,
        0xFFFFFFFF,
        read_u32(current.data, row.source),
        read_u32(previous.data, row.source),
        read_u32(current.data, current.base + 0x3EBA8 - current.base),
        read_u32(current.data, current.base + 0x3EBA0 - current.base),
    }
    for seed_source in rows_for_state(current):
        if seed_source.source == row.source:
            continue
        if 0 <= seed_source.source < len(previous.data) and seed_source.source + seed_source.length <= len(previous.data):
            for initial in (0, 0xFFFFFFFF, 0xCE19463F):
                values.add(crc32_seed(previous.data[seed_source.source : seed_source.source + seed_source.length], initial))
    return sorted(value for value in values if value is not None)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("run_dir", type=Path)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()

    states = [parse_state(path) for path in sorted(args.run_dir.glob("generated_island_state_*.bin"))]
    if len(states) < 2:
        raise SystemExit("need at least two generated_island_state_*.bin files")

    seed_calls = parse_seed_calls(args.run_dir)
    tea_calls = parse_tea_calls(args.run_dir)
    latest_base = states[-1].base

    lines = ["# Generated TEA Replay Check", "", f"Run directory: `{args.run_dir}`", ""]
    generated_seed_calls = [call for call in seed_calls if latest_base <= call.buf < latest_base + len(states[-1].data)]
    if generated_seed_calls:
        lines.extend(["## Live Seed Callback Windows", ""])
        for call in generated_seed_calls:
            ret = "?" if call.ret is None else f"0x{call.ret:08X}"
            best_state = best_state_for_seed_window(states, latest_base, call)
            state_text = "?" if best_state is None else f"seq {best_state[1]} diff 0x{best_state[0]:X}"
            lines.append(
                f"- seedcall `{call.seq}` window `0x{call.buf - latest_base:X}-0x{call.buf - latest_base + call.length:X}` len `0x{call.length:X}` seed_arg `0x{call.seed_arg:08X}` ret `{ret}` best_state `{state_text}`"
            )
        lines.append("")
    if tea_calls:
        validated = 0
        failed = []
        for call in tea_calls:
            if not call.pre or not call.post:
                continue
            actual = crypt_region(call.pre, call.seed, call.mode)
            diff = sum(1 for x, y in zip(actual, call.post) if x != y)
            if diff == 0:
                validated += 1
            else:
                failed.append((call.seq, diff))
        lines.extend(["## Live TEA Wrapper Validation", ""])
        lines.append(f"- validated pairs: `{validated}`")
        if failed:
            lines.append(f"- failed pairs: `{', '.join(f'#{seq}:0x{diff:X}' for seq, diff in failed[:12])}`")
        else:
            lines.append("- failed pairs: `none`")
        lines.append("")
        generated_tea = [call for call in tea_calls if latest_base <= call.ptr < latest_base + len(states[-1].data)]
        if generated_tea:
            lines.extend(["## Live Seed To TEA Mapping", ""])
            for call in generated_tea:
                producers = [seed for seed in generated_seed_calls if seed.ret == call.seed and seed.seq <= call.seq + 200]
                producer_text = ", ".join(f"#{seed.seq}:0x{seed.buf - latest_base:X}-0x{seed.buf - latest_base + seed.length:X}" for seed in producers[-3:]) or "?"
                lines.append(
                    f"- tea `#{call.seq}` target `0x{call.ptr - latest_base:X}-0x{call.ptr - latest_base + call.length:X}` seed `0x{call.seed:08X}` mode `{call.mode}` producers `{producer_text}`"
                )
            lines.append("")
        plain_overlay, plain_ranges = build_plain_overlay(states[-1], tea_calls)
        if plain_ranges:
            covered = sum(end - start for start, end, _seqs in plain_ranges)
            lines.extend(["## Max-Cleartext Overlay", ""])
            lines.append(f"- mode-0 decrypted ranges: `{len(plain_ranges)}`")
            lines.append(f"- covered bytes: `0x{covered:X}`")
            if args.output:
                overlay_path = args.output.with_name("generated_tea_plain_overlay.bin")
                overlay_path.write_bytes(plain_overlay)
                lines.append(f"- overlay image: `{overlay_path.name}`")
            for start, end, seqs in plain_ranges[:32]:
                lines.append(
                    f"- range `0x{start:X}-0x{end:X}` len `0x{end - start:X}` teacalls `{','.join(str(seq) for seq in seqs[:8])}`"
                )
            lines.append("")
    for previous, current in zip(states, states[1:]):
        rows = rows_for_state(current)
        matches = [row for row in rows if changed_bytes(previous.data, current.data, row.source, row.length)]
        if not matches:
            continue
        lines.append(f"## seq `{previous.seq}` -> `{current.seq}`")
        lines.append("")
        for row in matches[:6]:
            changed = changed_bytes(previous.data, current.data, row.source, row.length)
            live_windows = [
                call
                for call in generated_seed_calls
                if call.buf - current.base <= row.source
                and row.source + row.length <= call.buf - current.base + call.length
            ]
            best = []
            for seed in candidate_seeds(previous, current, row):
                expected = current.data[row.source : row.source + row.length]
                for mode in (0, 1):
                    actual = crypt_region(previous.data[row.source : row.source + row.length], seed, mode)
                    diff = sum(1 for x, y in zip(actual, expected) if x != y)
                    best.append((diff, seed, mode))
            best.sort()
            if best:
                diff, seed, mode = best[0]
                lines.append(
                    f"- row call `0x{row.call:08X}` hint `0x{row.hint:08X}` range `0x{row.source:X}-0x{row.source + row.length:X}` changed `0x{changed:X}` best_candidate_seed `0x{seed:08X}` mode `{mode}` diff `0x{diff:X}`"
                )
            else:
                lines.append(
                    f"- row call `0x{row.call:08X}` hint `0x{row.hint:08X}` range `0x{row.source:X}-0x{row.source + row.length:X}` changed `0x{changed:X}` no visible seed candidates"
                )
            if live_windows:
                details = []
                for call in live_windows[:4]:
                    ret = "?" if call.ret is None else f"0x{call.ret:08X}"
                    details.append(
                        f"#{call.seq}:off=0x{call.buf - current.base:X},len=0x{call.length:X},ret={ret}"
                    )
                lines.append(f"  live seed windows: `{', '.join(details)}`")
        lines.append("")

    if args.output:
        write_markdown(args.output, lines)
    else:
        print("\n".join(lines))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
