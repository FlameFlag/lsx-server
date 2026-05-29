#!/usr/bin/env python3
"""Compare captured original-section snapshots against the canonical payload."""

from __future__ import annotations

import argparse
import zipfile
from dataclasses import dataclass
from pathlib import Path

from lib.binary import contiguous_ranges, sha256_hex
from lib.capture import snapshot_section_name, snapshot_seq
from lib.common import EXPECTED_PAYLOADS, load_pe_sections, repo_root
from lib.reporting import write_markdown


@dataclass(frozen=True)
class Snapshot:
    seq: int
    name: str
    data: bytes


def dword_delta_groups(expected: bytes, observed: bytes, missing: list[int], limit: int = 8) -> list[str]:
    groups: dict[int, list[tuple[int, int, int]]] = {}
    for offset in sorted({item & ~3 for item in missing}):
        if offset + 4 > len(expected) or offset + 4 > len(observed):
            continue
        want = int.from_bytes(expected[offset : offset + 4], "little")
        got = int.from_bytes(observed[offset : offset + 4], "little")
        delta = (got - want) & 0xFFFFFFFF
        groups.setdefault(delta, []).append((offset, want, got))
    rows: list[str] = []
    for delta, values in sorted(groups.items(), key=lambda item: len(item[1]), reverse=True)[:limit]:
        examples = ", ".join(
            f"0x{offset:X}:0x{got:08X}->0x{want:08X}" for offset, want, got in values[:4]
        )
        rows.append(f"- delta `0x{delta:08X}` dwords `{len(values)}` examples {examples}")
    return rows


def normalize_by_repeated_dword_deltas(expected: bytes, observed: bytes, missing: list[int], min_group: int = 2) -> bytes:
    out = bytearray(observed[: len(expected)])
    if len(out) < len(expected):
        out.extend(b"\0" * (len(expected) - len(out)))
    groups: dict[int, list[tuple[int, int]]] = {}
    for offset in sorted({item & ~3 for item in missing}):
        if offset + 4 > len(expected) or offset + 4 > len(out):
            continue
        want = int.from_bytes(expected[offset : offset + 4], "little")
        got = int.from_bytes(out[offset : offset + 4], "little")
        delta = (got - want) & 0xFFFFFFFF
        groups.setdefault(delta, []).append((offset, got))
    for delta, values in groups.items():
        if len(values) < min_group:
            continue
        for offset, got in values:
            corrected = (got - delta) & 0xFFFFFFFF
            out[offset : offset + 4] = corrected.to_bytes(4, "little")
    return bytes(out)


def load_snapshots(capture_zip: Path) -> dict[str, list[Snapshot]]:
    snapshots: dict[str, list[Snapshot]] = {"text": [], "rdata": [], "data": []}
    with zipfile.ZipFile(capture_zip) as zf:
        for name in zf.namelist():
            if not name.lower().endswith(".bin"):
                continue
            section = snapshot_section_name(name)
            if section in snapshots:
                snapshots[section].append(Snapshot(snapshot_seq(name), name, zf.read(name)))
    for values in snapshots.values():
        values.sort(key=lambda item: (item.seq, item.name))
    return snapshots


def best_snapshot(expected: bytes, snapshots: list[Snapshot]) -> tuple[Snapshot, int] | None:
    best: tuple[Snapshot, int] | None = None
    for snapshot in snapshots:
        compare_size = min(len(expected), len(snapshot.data))
        diff_count = sum(1 for left, right in zip(snapshot.data[:compare_size], expected[:compare_size]) if left != right)
        diff_count += abs(len(snapshot.data) - len(expected))
        if best is None or (diff_count, snapshot.seq, snapshot.name) < (best[1], best[0].seq, best[0].name):
            best = (snapshot, diff_count)
    return best


def observed_overlay(expected: bytes, snapshots: list[Snapshot]) -> tuple[bytes, list[int], list[int]]:
    if not snapshots:
        return b"", list(range(len(expected))), []
    overlay = bytearray(snapshots[-1].data[: len(expected)])
    if len(overlay) < len(expected):
        overlay.extend(b"\0" * (len(expected) - len(overlay)))
    missing: list[int] = []
    replaced: list[int] = []
    for offset, want in enumerate(expected):
        if overlay[offset] == want:
            continue
        source = next((snapshot for snapshot in reversed(snapshots) if offset < len(snapshot.data) and snapshot.data[offset] == want), None)
        if source is None:
            missing.append(offset)
            continue
        overlay[offset] = want
        replaced.append(offset)
    return bytes(overlay), missing, replaced


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("run_dir", type=Path)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()

    capture_zip = args.run_dir / "capture.zip"
    expected_path = repo_root() / "decompiled/local/unpacked/Lemonade2.unpacked.exe"
    expected_sections = load_pe_sections(expected_path)
    snapshots = load_snapshots(capture_zip)

    lines = [
        "# Payload Snapshot Analysis",
        "",
        f"Capture archive: `{capture_zip.name}`",
        f"Canonical payload: `{expected_path}`",
        "",
        "| Section | Snapshots | Best Snapshot | Best Diff Bytes | Observed Overlay | Missing Bytes |",
        "| --- | ---: | --- | ---: | --- | ---: |",
    ]
    details: list[str] = []
    args.output.parent.mkdir(parents=True, exist_ok=True)
    for section in (".text", ".rdata", ".data"):
        expected = expected_sections[section].data[: EXPECTED_PAYLOADS[section][0]]
        section_snapshots = snapshots[section[1:]]
        best = best_snapshot(expected, section_snapshots)
        overlay, missing, replaced = observed_overlay(expected, section_snapshots)
        overlay_name = f"payload_observed_{section[1:]}.bin"
        overlay_path = args.output.parent / overlay_name
        overlay_path.write_bytes(overlay)
        overlay_hash = sha256_hex(overlay)
        expected_hash = EXPECTED_PAYLOADS[section][1]
        overlay_status = "matched" if overlay_hash == expected_hash else f"sha `{overlay_hash[:12]}`"
        best_name = best[0].name if best else "none"
        best_diff = best[1] if best else len(expected)
        lines.append(
            f"| `{section}` | {len(section_snapshots)} | `{best_name}` | {best_diff} | {overlay_status} | {len(missing)} |"
        )
        if missing:
            range_count, ranges = contiguous_ranges(missing, 16)
            ranges_text = ", ".join(f"0x{start:X}-0x{end:X}" for start, end in ranges)
            details.extend(
                [
                    "",
                    f"## `{section}` Missing Bytes",
                    "",
                    f"- Missing ranges: `{range_count}`",
                    f"- First ranges: {ranges_text}",
                ]
            )
            if best is not None:
                delta_rows = dword_delta_groups(expected, best[0].data, missing)
                if delta_rows:
                    details.extend(["", "Dominant dword deltas from best snapshot:"])
                    details.extend(delta_rows)
                normalized = normalize_by_repeated_dword_deltas(expected, best[0].data, missing)
                normalized_hash = sha256_hex(normalized)
                normalized_path = args.output.parent / f"payload_delta_normalized_{section[1:]}.bin"
                normalized_path.write_bytes(normalized)
                normalized_status = "matched" if normalized_hash == EXPECTED_PAYLOADS[section][1] else f"sha `{normalized_hash[:12]}`"
                details.extend(["", f"- Repeated-delta normalized candidate: {normalized_status} `{normalized_path.name}`"])
        if replaced:
            range_count, ranges = contiguous_ranges(replaced, 16)
            ranges_text = ", ".join(f"0x{start:X}-0x{end:X}" for start, end in ranges)
            details.extend(
                [
                    "",
                    f"## `{section}` Recovered From Earlier Snapshots",
                    "",
                    f"- Recovered byte ranges: `{range_count}`",
                    f"- First ranges: {ranges_text}",
                ]
            )
    lines.extend(details)
    write_markdown(args.output, lines)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
