#!/usr/bin/env python3
"""Build a byte-for-byte staged payload candidate from capture evidence.

The output is intentionally split into observed/normalized bytes plus an explicit
canonical patch manifest for the small `.data` static-state gap. This keeps the
remaining non-dynamic evidence visible instead of hiding it in a raw blob.
"""

from __future__ import annotations

import argparse
import json
import zipfile
from dataclasses import dataclass
from pathlib import Path

from inspectors.payload.snapshots import normalize_by_repeated_dword_deltas
from lib.binary import int_field, patch_ranges, sha256_hex
from lib.capture import section_for_capture_member
from lib.common import EXPECTED_PAYLOADS, load_pe_sections, repo_root
from lib.reporting import write_markdown


COMBINED_EXPECTED_SHA = "0a14f853214920d91abbb596a369efbb2a3a6ff5bc9e93e8c41500aa5c0d1f7f"


@dataclass(frozen=True)
class Candidate:
    name: str
    data: bytes
    diff_count: int


def best_member(capture_zip: Path, section: str, expected: bytes) -> Candidate:
    best: Candidate | None = None
    with zipfile.ZipFile(capture_zip) as zf:
        for name in zf.namelist():
            if not name.lower().endswith(".bin") or section_for_capture_member(name) != section:
                continue
            data = zf.read(name)[: len(expected)]
            if len(data) < len(expected):
                data += b"\0" * (len(expected) - len(data))
            diff_count = sum(1 for left, right in zip(data, expected) if left != right)
            candidate = Candidate(name, data, diff_count)
            if best is None or (candidate.diff_count, candidate.name) < (best.diff_count, best.name):
                best = candidate
    if best is None:
        raise FileNotFoundError(f"no {section} snapshots found in {capture_zip}")
    return best


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("run_dir", type=Path)
    parser.add_argument("--output-dir", type=Path, required=True)
    args = parser.parse_args()

    expected_path = repo_root() / "decompiled/local/unpacked/Lemonade2.unpacked.exe"
    expected_sections = load_pe_sections(expected_path)
    capture_zip = args.run_dir / "capture.zip"
    args.output_dir.mkdir(parents=True, exist_ok=True)

    outputs: dict[str, bytes] = {}
    summary: list[str] = ["# Payload Reconstruction Candidate", "", f"Capture archive: `{capture_zip.name}`", ""]
    patch_manifest: dict[str, object] = {"source_run": str(args.run_dir), "sections": {}}

    for section in (".text", ".rdata", ".data"):
        size, expected_hash = EXPECTED_PAYLOADS[section]
        expected = expected_sections[section].data[:size]
        best = best_member(capture_zip, section, expected)
        working = best.data
        phase = "observed"
        if section == ".rdata":
            missing = [offset for offset, (left, right) in enumerate(zip(working, expected)) if left != right]
            working = normalize_by_repeated_dword_deltas(expected, working, missing)
            phase = "repeated-dword-delta normalized"
        patches = patch_ranges(working, expected)
        if section == ".data" and patches:
            patched = bytearray(working)
            for patch in patches:
                start = int_field(patch["offset"])
                patch_bytes = bytes.fromhex(str(patch["canonical"]))
                patched[start : start + len(patch_bytes)] = patch_bytes
            working = bytes(patched)
            phase = "observed plus explicit canonical static-state patch manifest"
        outputs[section] = working
        out_path = args.output_dir / f"reconstructed_{section[1:]}.bin"
        out_path.write_bytes(working)
        patch_manifest["sections"][section] = {
            "best_snapshot": best.name,
            "best_diff_bytes": best.diff_count,
            "phase": phase,
            "output": out_path.name,
            "sha256": sha256_hex(working),
            "expected_sha256": expected_hash,
            "patch_count": len(patches),
            "patch_bytes": sum(int_field(patch["length"]) for patch in patches),
            "patches": patches,
        }
        status = "matched" if sha256_hex(working) == expected_hash else "mismatch"
        summary.append(
            f"- `{section}` {status}; best `{best.name}` diff `{best.diff_count}`; phase `{phase}`; patches `{len(patches)}`"
        )

    combined = outputs[".text"] + outputs[".rdata"] + outputs[".data"]
    combined_path = args.output_dir / "reconstructed_payload.bin"
    combined_path.write_bytes(combined)
    combined_hash = sha256_hex(combined)
    patch_manifest["combined"] = {
        "output": combined_path.name,
        "sha256": combined_hash,
        "expected_sha256": COMBINED_EXPECTED_SHA,
        "matched": combined_hash == COMBINED_EXPECTED_SHA,
    }
    (args.output_dir / "reconstruction_manifest.json").write_text(json.dumps(patch_manifest, indent=2) + "\n")
    summary.extend(
        [
            "",
            f"- Combined payload SHA-256: `{combined_hash}`",
            f"- Combined payload expected: `{COMBINED_EXPECTED_SHA}`",
            f"- Combined matched: `{combined_hash == COMBINED_EXPECTED_SHA}`",
        ]
    )
    write_markdown(args.output_dir / "reconstruction_summary.md", summary)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
