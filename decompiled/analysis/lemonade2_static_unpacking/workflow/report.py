#!/usr/bin/env python3
"""Generate a compact Markdown report from a Lemonade2 capture run."""

from __future__ import annotations

import argparse
import re
import zipfile
from pathlib import Path
from typing import Any

from capstone import CS_ARCH_X86, CS_MODE_32, Cs
from capstone.x86 import X86_OP_IMM, X86_OP_MEM

from lib.binary import changed_byte_ranges, sha256_hex
from lib.capture import parse_summary, read_zip_text, sha256_member, snapshot_section_name
from lib.common import EXPECTED_PAYLOADS, load_pe_sections, repo_root
from lib.pe import parse_pe_metadata
from lib.reporting import write_markdown


def parse_hex_fields(name: str) -> dict[str, int]:
    fields: dict[str, int] = {}
    for key in ("ex", "at", "base", "size"):
        match = re.search(rf"{key}([0-9A-Fa-f]{{8}})", name)
        if match:
            fields[key] = int(match.group(1), 16)
    return fields


def interesting_ranges(base: int, size: int) -> list[tuple[str, int, int]]:
    return [
        ("image", 0x00400000, 0x00500000),
        ("island", base, base + size),
        ("island+next", base, base + 0x40000),
    ]


def format_disassembly(data: bytes, base: int, address: int, radius: int = 0x20, size: int = 0x90) -> list[str]:
    offset = address - base
    if offset < 0 or offset >= len(data):
        return [f"Address `0x{address:08X}` is outside dump base `0x{base:08X}`."]
    start = max(0, offset - radius)
    md = Cs(CS_ARCH_X86, CS_MODE_32)
    rows: list[str] = []
    for ins in md.disasm(data[start : start + size], base + start):
        mark = ">>" if ins.address == address else "  "
        rows.append(f"{mark} {ins.address:08X}  {ins.bytes.hex().upper():<18} {ins.mnemonic:<8} {ins.op_str}")
        if len(rows) >= 18:
            break
    return rows or [f"No linear disassembly at `0x{address:08X}`."]


def collect_refs(data: bytes, base: int, size: int, limit: int = 12) -> list[str]:
    md = Cs(CS_ARCH_X86, CS_MODE_32)
    md.detail = True
    ranges = interesting_ranges(base, size)
    refs: dict[int, tuple[str, int, str]] = {}
    for ins in md.disasm(data, base):
        for op in ins.operands:
            values: list[int] = []
            if op.type == X86_OP_IMM:
                values.append(op.imm & 0xFFFFFFFF)
            elif op.type == X86_OP_MEM and op.mem.disp:
                values.append(op.mem.disp & 0xFFFFFFFF)
            for value in values:
                for label, lo, hi in ranges:
                    if lo <= value < hi and value not in refs:
                        refs[value] = (label, ins.address, f"{ins.mnemonic} {ins.op_str}".strip())
                        break
        if len(refs) >= limit:
            break
    return [f"`0x{addr:08X}` {label} from `0x{src:08X}` `{text}`" for addr, (label, src, text) in refs.items()]


def island_analysis(zf: zipfile.ZipFile, info: zipfile.ZipInfo) -> dict[str, Any]:
    fields = parse_hex_fields(info.filename)
    data = zf.read(info.filename)
    base = fields.get("base", 0)
    at = fields.get("at", base)
    return {
        "filename": info.filename,
        "size": info.file_size,
        "sha": sha256_member(zf, info.filename),
        "fields": fields,
        "mz": data[:2] == b"MZ",
        "fault_disassembly": format_disassembly(data, base, at),
        "refs": collect_refs(data, base, info.file_size),
    }


def parse_pe_sections(data: bytes) -> dict[str, Any]:
    if len(data) < 0x40 or data[:2] != b"MZ":
        return {}
    try:
        pe = parse_pe_metadata(data)
    except Exception:
        return {}
    sections = [
        {
            "name": section.name,
            "va": section.va,
            "vs": section.vs,
            "raw_size": section.raw_size,
            "raw_pointer": section.raw_pointer,
            "flags": section.flags,
            "complete": section.raw_pointer + section.raw_size <= len(data),
        }
        for section in pe.sections
    ]
    return {
        "machine": pe.machine,
        "section_count": len(sections),
        "characteristics": pe.characteristics,
        "entry_rva": pe.entry_rva,
        "image_base": pe.image_base,
        "size_of_image": pe.size_of_image,
        "sections": sections,
    }


def combined_island_images(zf: zipfile.ZipFile, islands: list[zipfile.ZipInfo], output_dir: Path) -> list[dict[str, Any]]:
    parts = []
    for info in islands:
        fields = parse_hex_fields(info.filename)
        base = fields.get("base")
        if base is None:
            continue
        seq_match = re.search(r"island_(\d+)_", info.filename)
        seq = int(seq_match.group(1)) if seq_match else len(parts) + 1
        parts.append((seq, base, zf.read(info.filename), info.filename))
    parts.sort(key=lambda item: item[0])

    pages: dict[int, tuple[bytes, str]] = {}
    mz_base = 0
    rows: list[dict[str, Any]] = []
    last_digest = ""
    for seq, page_base, data, name in parts:
        pages[page_base] = (data, name)
        if data[:2] == b"MZ":
            mz_base = page_base
        if not mz_base or mz_base + 0x20000 not in pages:
            continue
        group = [(mz_base, pages[mz_base][0], pages[mz_base][1]), (mz_base + 0x20000, pages[mz_base + 0x20000][0], pages[mz_base + 0x20000][1])]
        base = mz_base
        end = max(part_base + len(part_data) for part_base, part_data, _name in group)
        blob = bytearray(end - base)
        for part_base, part_data, _name in group:
            blob[part_base - base : part_base - base + len(part_data)] = part_data
        island_image = bytes(blob)
        digest = sha256_hex(island_image)
        if digest == last_digest:
            continue
        last_digest = digest
        index = len(rows) + 1
        path = output_dir / f"generated_island_state_{index:02d}_seq{seq:04d}_{base:08X}.bin"
        path.write_bytes(island_image)
        rows.append(
            {
                "path": path.name,
                "base": base,
                "seq": seq,
                "size": len(blob),
                "sha": digest,
                "parts": [name for _part_base, _data, name in group],
                "pe": parse_pe_sections(island_image),
            }
        )
        if len(rows) >= 100:
            break
    return rows


def parse_evctx(line: str) -> dict[str, int]:
    if not line.startswith("EVCTX "):
        return {}
    values: dict[str, int] = {}
    for key, value in re.findall(r"([a-z0-9]+)=([0-9A-Fa-f]+)", line):
        try:
            values[key] = int(value, 16)
        except ValueError:
            continue
    return values


def island_context_rows(trace_lines: list[str], island_rows: list[dict[str, Any]], limit: int = 40) -> list[str]:
    ranges: list[tuple[int, int]] = []
    for row in island_rows:
        fields = row["fields"]
        base = fields.get("base", 0)
        size = fields.get("size", row.get("size", 0))
        if base and size:
            ranges.append((base, base + size))
    rows: list[str] = []
    seen: set[tuple[int, int, int, int, int]] = set()
    for line in trace_lines:
        ctx = parse_evctx(line)
        if not ctx:
            continue
        addr = ctx.get("addr", 0)
        for base, end in ranges:
            if base <= addr < end:
                key = (addr, ctx.get("esp", 0), ctx.get("ecx", 0), ctx.get("edx", 0), ctx.get("esi", 0))
                if key in seen:
                    break
                seen.add(key)
                rows.append(
                    "- addr `0x{addr:08X}` off `0x{off:X}` ex `0x{ex:08X}` eax `0x{eax:08X}` ebx `0x{ebx:08X}` ecx `0x{ecx:08X}` edx `0x{edx:08X}` esi `0x{esi:08X}` edi `0x{edi:08X}`".format(
                        addr=addr,
                        off=addr - base,
                        ex=ctx.get("ex", 0),
                        eax=ctx.get("eax", 0),
                        ebx=ctx.get("ebx", 0),
                        ecx=ctx.get("ecx", 0),
                        edx=ctx.get("edx", 0),
                        esi=ctx.get("esi", 0),
                        edi=ctx.get("edi", 0),
                    )
                )
                break
        if len(rows) >= limit:
            break
    return rows


def matching_regions(zf: zipfile.ZipFile) -> list[str]:
    matches: list[str] = []
    expected_by_size = {size: (section, digest) for section, (size, digest) in EXPECTED_PAYLOADS.items()}
    for info in zf.infolist():
        if not info.filename.lower().endswith(".bin"):
            continue
        expected = expected_by_size.get(info.file_size)
        if not expected:
            continue
        section, digest = expected
        actual = sha256_member(zf, info.filename)
        if actual == digest:
            matches.append(f"- `{section}` matched `{info.filename}`")
        else:
            matches.append(f"- `{section}` size matched but hash differed: `{info.filename}` `{actual}`")
    return matches


def best_expected_snapshot_rows(zf: zipfile.ZipFile, bins: list[zipfile.ZipInfo]) -> list[str]:
    expected_path = repo_root() / "decompiled/local/unpacked/Lemonade2.unpacked.exe"
    if not expected_path.exists():
        return [f"- Canonical unpacked EXE not found at `{expected_path}`."]
    expected_sections = load_pe_sections(expected_path)
    rows: list[str] = []
    for section in (".text", ".rdata", ".data"):
        expected = expected_sections.get(section)
        if expected is None:
            rows.append(f"- `{section}` missing from canonical unpacked EXE.")
            continue
        snapshot_name = section[1:]
        candidates = [info for info in bins if snapshot_section_name(info.filename) == snapshot_name]
        if not candidates:
            rows.append(f"- `{section}` had no runtime snapshots.")
            continue
        best: tuple[int, str, bytes] | None = None
        for info in candidates:
            data = zf.read(info.filename)
            compare_size = min(len(data), len(expected.data))
            diff_count = sum(1 for left, right in zip(data[:compare_size], expected.data[:compare_size]) if left != right)
            diff_count += abs(len(data) - len(expected.data))
            if best is None or (diff_count, info.filename) < (best[0], best[1]):
                best = (diff_count, info.filename, data)
        if best is None:
            continue
        diff_count, filename, data = best
        status = "matched expected" if diff_count == 0 else f"diff bytes `{diff_count}`"
        _total, ranges = changed_byte_ranges(data, expected.data)
        ranges = ranges[:8]
        ranges_text = ", ".join(f"0x{start:X}-0x{end:X}" for start, end in ranges) or "none"
        rows.append(f"- `{section}` best `{filename}` {status}; first ranges `{ranges_text}`")
    return rows


def snapshot_diffs(zf: zipfile.ZipFile, bins: list[zipfile.ZipInfo]) -> list[str]:
    snapshots = [info for info in bins if "snap_" in info.filename]
    snapshots.sort(key=lambda info: info.filename)
    by_section: dict[str, list[zipfile.ZipInfo]] = {}
    for info in snapshots:
        section = snapshot_section_name(info.filename)
        if section:
            by_section.setdefault(section, []).append(info)
    lines: list[str] = []
    for section, infos in sorted(by_section.items()):
        previous_name = ""
        previous_data = b""
        for info in infos:
            data = zf.read(info.filename)
            digest = sha256_hex(data)
            expected = EXPECTED_PAYLOADS.get(f".{section}")
            status = "matched expected" if expected and digest == expected[1] else "changed"
            if previous_name:
                total, ranges = changed_byte_ranges(previous_data, data)
                ranges = ranges[:12]
                ranges_text = ", ".join(f"0x{start:X}-0x{end:X}" for start, end in ranges) or "none"
                lines.append(
                    f"- `{section}` `{previous_name}` -> `{info.filename}` {status}; changed bytes `{total}` first ranges `{ranges_text}`"
                )
            else:
                lines.append(f"- `{section}` initial `{info.filename}` sha `{digest}`")
            previous_name = info.filename
            previous_data = data
    return lines


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("capture_zip", type=Path)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()

    with zipfile.ZipFile(args.capture_zip) as zf:
        capture_log = read_zip_text(zf, "capture.log")
        trace_log = read_zip_text(zf, "trace/trace.log")
        summary = parse_summary(zf)
        bins = [info for info in zf.infolist() if info.filename.lower().endswith(".bin")]
        islands = [info for info in bins if "island_" in info.filename]
        island_rows = [island_analysis(zf, info) for info in islands]
        combined_islands = combined_island_images(zf, islands, args.output.parent)
        matches = matching_regions(zf)
        best_snapshots = best_expected_snapshot_rows(zf, bins)
        diffs = snapshot_diffs(zf, bins)

    trace_lines = [line for line in trace_log.splitlines() if line.strip()]
    log_lines = [line for line in capture_log.splitlines() if line.strip()]
    interesting = [row for row in summary if row.get("contains_interesting")]

    report = [
        "# Lemonade2 Capture Report",
        "",
        f"Capture archive: `{args.capture_zip.name}`",
        "",
        "## Summary",
        "",
        f"- Trace lines: {len(trace_lines)}",
        f"- Dumped memory regions: {len(bins)}",
        f"- Generated island dumps: {len(islands)}",
        f"- Summary rows with interesting addresses: {len(interesting)}",
        "",
        "## Expected Payload Hash Checks",
        "",
    ]
    report.extend(matches or ["- No dumped region matched an expected staged payload size."])
    report.extend(["", "## Best Runtime Payload Snapshots", ""])
    report.extend(best_snapshots or ["- No runtime payload snapshots found."])
    report.extend(["", "## Snapshot Diffs", ""])
    report.extend(diffs[:80] or ["- No comparable snapshots found."])
    report.extend(["", "## Generated Island Dumps", ""])
    if island_rows:
        for row in island_rows[:25]:
            fields = row["fields"]
            base = fields.get("base", 0)
            at = fields.get("at", 0)
            offset = at - base if base else 0
            report.append(
                f"- `{row['filename']}` size `0x{row['size']:X}` sha `{row['sha']}` base `0x{base:08X}` fault `0x{at:08X}` offset `0x{offset:X}` mz `{row['mz']}`"
            )
    else:
        report.append("- None recorded.")
    if island_rows:
        context_rows = island_context_rows(trace_lines, island_rows)
        report.extend(["", "## Island Exception Contexts", ""])
        report.extend(context_rows or ["- No EVCTX rows fell inside captured island ranges."])
    if combined_islands:
        report.extend(["", "## Combined Generated Images", ""])
        for image in combined_islands:
            report.append(
                f"- `{image['path']}` seq `{image['seq']}` base `0x{image['base']:08X}` size `0x{image['size']:X}` sha `{image['sha']}` parts `{len(image['parts'])}`"
            )
            pe = image["pe"]
            if pe:
                report.append(
                    f"- PE image_base `0x{pe['image_base']:08X}` entry_rva `0x{pe['entry_rva']:X}` size_of_image `0x{pe['size_of_image']:X}` sections `{pe['section_count']}` chars `0x{pe['characteristics']:X}`"
                )
                for section in pe["sections"]:
                    report.append(
                        f"- section `{section['name']}` rva `0x{section['va']:X}` vsize `0x{section['vs']:X}` raw `0x{section['raw_pointer']:X}`+`0x{section['raw_size']:X}` flags `0x{section['flags']:X}` complete `{section['complete']}`"
                    )
            else:
                report.append("- Combined image did not parse as PE.")
            report.append("")
    if island_rows:
        report.extend(["", "## Island Fault Disassembly", ""])
        for row in island_rows[:8]:
            fields = row["fields"]
            report.append(f"### `{row['filename']}`")
            report.append("")
            report.append("```text")
            report.extend(row["fault_disassembly"])
            report.append("```")
            report.append("")
            refs = row["refs"]
            if refs:
                report.append("Notable absolute references:")
                report.extend(f"- {ref}" for ref in refs)
            else:
                report.append("No notable absolute references found in linear scan.")
            report.append("")
    report.extend(["", "## Interesting Regions", ""])
    if interesting:
        for row in interesting[:25]:
            report.append(
                "- `{summary}` pid `{pid}` base `0x{base}` size `0x{size}` contains `{contains}` sha `{sha}`".format(
                    summary=row.get("summary_path", ""),
                    pid=row.get("pid", ""),
                    base=row.get("base", ""),
                    size=row.get("size", ""),
                    contains=row.get("contains_interesting", ""),
                    sha=row.get("sha256", ""),
                )
            )
    else:
        report.append("- None recorded.")
    report.extend(["", "## Trace Tail", "", "```text"])
    report.extend(trace_lines[-80:] or ["No trace log found."])
    report.extend(["```", "", "## Capture Log Tail", "", "```text"])
    report.extend(log_lines[-80:] or ["No capture log found."])
    report.append("```")

    write_markdown(args.output, report)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
