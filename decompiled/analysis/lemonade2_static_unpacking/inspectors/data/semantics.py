#!/usr/bin/env python3
"""Classify the remaining `.data` canonical patch manifest semantically."""

from __future__ import annotations

import argparse
import collections
import json
from pathlib import Path

from lib.binary import grouped_by_gap, int_field
from lib.reporting import write_markdown


IMAGE_LO = 0x00400000
IMAGE_HI = 0x00590000
DATA_BASE = 0x0049C000


def owner_for_offset(offset: int) -> str:
    ranges = [
        (0x12A0, 0x14A0, "startup/runtime scalar table"),
        (0x3820, 0x389C, "static runtime pointer table"),
        (0x4000, 0x40B0, "runtime exception/static state"),
        (0x4180, 0x4209, "object/resource bootstrap state"),
        (0x42A0, 0x443B, "object table 004A02A0"),
        (0x44A0, 0x4623, "object table 004A04A0"),
        (0x4640, 0x4744, "object/render table 004A0640"),
        (0x4760, 0x481B, "render/window state 004A0760"),
        (0x489C, 0x48A0, "render/window state 004A089C"),
        (0x4C20, 0x4C38, "render/window API handles"),
        (0x4E90, 0x4E9C, "CRT runtime globals"),
        (0x4FC8, 0x4FC9, "CRT file stream allocation counter"),
        (0x5424, 0x5428, "ctype/runtime table"),
        (0x6440, 0x6464, "CRT runtime globals"),
        (0x656C, 0x6577, "CRT runtime globals"),
    ]
    for start, end, label in ranges:
        if start <= offset < end:
            return label
    return "unclassified"


def classify_dword(offset: int, value: int) -> str:
    if offset == 0x47A8 and value == 0x848EDDC3:
        return "resource_bundle_magic"
    if offset == 0x4C34:
        return "render_window_handle"
    if offset == 0x4FC8:
        return "crt_file_stream_counter"
    if value == 0:
        return "zero"
    if value == 0xFFFFFFFF:
        return "sentinel_-1"
    if IMAGE_LO <= value < IMAGE_HI:
        return "image_pointer"
    if value < 0x10000:
        return "small_scalar"
    if value < 0x01000000:
        return "packed_scalar_or_flags"
    if value < 0x10000000:
        return "low_runtime_or_heap_pointer"
    return "other_dword"


def contiguous_clusters(patches: list[dict[str, object]], gap: int = 0x10) -> list[tuple[int, int, list[dict[str, object]]]]:
    return grouped_by_gap(patches, lambda patch: (int_field(patch["offset"]), int_field(patch["offset"]) + int_field(patch["length"])), gap)


def dword_changes(patches: list[dict[str, object]], reconstructed_data: bytes) -> list[tuple[int, int, int]]:
    observed = bytearray(reconstructed_data)
    touched: set[int] = set()
    for patch in patches:
        start = int_field(patch["offset"])
        before = bytes.fromhex(str(patch["observed"]))
        observed[start : start + len(before)] = before
        for offset in range(start, start + int_field(patch["length"])):
            touched.add(offset & ~3)
    rows: list[tuple[int, int, int]] = []
    for offset in sorted(touched):
        if offset + 4 > len(reconstructed_data):
            continue
        before_value = int.from_bytes(observed[offset : offset + 4], "little")
        after_value = int.from_bytes(reconstructed_data[offset : offset + 4], "little")
        if before_value != after_value:
            rows.append((offset, before_value, after_value))
    return rows


def byte_only_patch_bytes(patches: list[dict[str, object]], dwords: list[tuple[int, int, int]]) -> list[int]:
    dword_bytes: set[int] = set()
    for offset, _before, _after in dwords:
        dword_bytes.update(range(offset, offset + 4))
    offsets: list[int] = []
    for patch in patches:
        start = int_field(patch["offset"])
        for offset in range(start, start + int_field(patch["length"])):
            if offset not in dword_bytes:
                offsets.append(offset)
    return sorted(offsets)


def policy_for_class(label: str) -> str:
    if label in ("low_runtime_or_heap_pointer", "render_window_handle", "crt_file_stream_counter"):
        return "canonical_dump_artifact"
    if label in ("small_scalar", "packed_scalar_or_flags", "sentinel_-1", "resource_bundle_magic", "zero"):
        return "portable_initializer_state"
    return "subsystem_byte_field"


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("reconstruction_dir", type=Path)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()

    manifest = json.loads((args.reconstruction_dir / "reconstruction_manifest.json").read_text())
    data_info = manifest["sections"][".data"]
    patches = list(data_info["patches"])
    reconstructed_data = (args.reconstruction_dir / data_info["output"]).read_bytes()
    dwords = dword_changes(patches, reconstructed_data)
    byte_only_offsets = byte_only_patch_bytes(patches, dwords)

    class_counts = collections.Counter(classify_dword(off, after) for off, _before, after in dwords)
    byte_owner_counts = collections.Counter(owner_for_offset(offset) for offset in byte_only_offsets)
    owner_counts = collections.Counter(owner_for_offset(off) for off, _before, _after in dwords)
    policy_counts = collections.Counter(policy_for_class(classify_dword(off, after)) for off, _before, after in dwords)
    if byte_only_offsets:
        policy_counts["subsystem_byte_field"] += len(byte_only_offsets)
    high_word_counts = collections.Counter(after >> 16 for off, _before, after in dwords if classify_dword(off, after) in ("low_runtime_or_heap_pointer", "packed_scalar_or_flags"))

    lines = [
        "# Data Patch Semantics",
        "",
        f"Manifest: `{args.reconstruction_dir / 'reconstruction_manifest.json'}`",
        "",
        f"- Patch ranges: `{len(patches)}`",
        f"- Patch bytes: `{sum(int_field(patch['length']) for patch in patches)}`",
        f"- Changed dwords: `{len(dwords)}`",
        f"- Byte-only patch bytes: `{len(byte_only_offsets)}`",
        "",
        "## Dword Classes",
        "",
    ]
    for label, count in class_counts.most_common():
        lines.append(f"- `{label}`: `{count}`")
    lines.extend(["", "## Owner Groups", ""])
    for label, count in owner_counts.most_common():
        lines.append(f"- `{label}`: `{count}` dwords")
    if byte_owner_counts:
        lines.extend(["", "## Byte-Only Fields", ""])
        for label, count in byte_owner_counts.most_common():
            examples = [offset for offset in byte_only_offsets if owner_for_offset(offset) == label][:8]
            example_text = ", ".join(f"+0x{offset:X}" for offset in examples)
            lines.append(f"- `{label}`: `{count}` bytes examples {example_text}")
    lines.extend(["", "## Reconstruction Policy", ""])
    for label, count in policy_counts.most_common():
        lines.append(f"- `{label}`: `{count}` patched units")
    if high_word_counts:
        lines.extend(["", "## Non-Image High Words", ""])
        for high, count in high_word_counts.most_common(20):
            examples = [row for row in dwords if (row[2] >> 16) == high][:5]
            example_text = ", ".join(f"+0x{off:X}:0x{before:08X}->0x{after:08X}" for off, before, after in examples)
            lines.append(f"- `0x{high:04X}0000`: `{count}` examples {example_text}")
    lines.extend(["", "## Clusters", ""])
    for start, end, items in contiguous_clusters(patches):
        cluster_offsets = {offset for offset, _before, _after in dwords if start <= offset < end}
        cluster_classes = collections.Counter(
            classify_dword(offset, after) for offset, _before, after in dwords if offset in cluster_offsets
        )
        classes = ", ".join(f"{name}:{count}" for name, count in cluster_classes.most_common()) or "byte_fields"
        owner = owner_for_offset(start)
        lines.append(
            f"- `+0x{start:X}-+0x{end:X}` VA `0x{DATA_BASE + start:08X}-0x{DATA_BASE + end:08X}` "
            f"len `0x{end - start:X}` owner `{owner}` ranges `{len(items)}` classes `{classes}`"
        )
    lines.extend(["", "## Interpretation", ""])
    lines.append(
        "- `low_runtime_or_heap_pointer` values are outside the image range and are likely process-specific dump artifacts, not portable disk data."
    )
    lines.append(
        "- `small_scalar`, `packed_scalar_or_flags`, and `sentinel_-1` values line up with CRT/runtime table initialization state."
    )
    lines.append(
        "- The former generic dwords are now accounted for as the resource bundle magic and render/window handle state."
    )
    lines.append(
        "- For byte-for-byte canonical matching, preserve all manifest entries. For portable reconstruction, treat canonical_dump_artifact entries as runtime-created state and let startup rebuild them."
    )
    lines.append(
        "- These categories explain why steady runtime snapshots cannot naturally equal the canonical staged `.data` dump."
    )
    write_markdown(args.output, lines)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
