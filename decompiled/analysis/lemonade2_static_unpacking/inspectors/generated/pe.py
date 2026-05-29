#!/usr/bin/env python3
"""Analyze the generated Armadillo PE dumped by the capture hook."""

from __future__ import annotations

import argparse
import re
import struct
from dataclasses import dataclass
from pathlib import Path

from capstone import CS_ARCH_X86, CS_MODE_32, Cs
from capstone.x86 import X86_OP_IMM, X86_OP_MEM
from lib.binary import read_u32
from lib.pe import Section, parse_pe_image as parse_pe, preferred_to_offset, runtime_to_offset
from lib.reporting import write_markdown


PREFERRED_BASE = 0x10000000
GUARD_TEXT = 0x1002951F
TEA_WRAPPER = 0x100014AC

ORIGINAL_IMAGE_BASE_GLOBAL_OFF = 0x3EB9C
LOADED_BASE_GLOBAL_OFF = 0x3E6D0
DELTA_GLOBAL_OFF = 0x3EBA0
HASH_OR_SEED_GLOBAL_OFF = 0x3EBA8
CONTEXT_RECORD_GLOBAL_OFF = 0x3E6CC
SCOPE_TABLE_A_OFF = 0x2F820
SCOPE_TABLE_B_OFF = 0x2F830
SOURCE_TABLE_START_OFF = 0x35200
SOURCE_TABLE_END_OFF = 0x35400
LENGTH_TABLE_START_OFF = 0x35400
LENGTH_TABLE_END_OFF = 0x35600
GLOBAL_WINDOW_START_OFF = 0x3E000
GLOBAL_WINDOW_END_OFF = 0x3F000


@dataclass(frozen=True)
class GuardCallSummary:
    call: int
    function_hint: int
    source_offsets: list[int]
    length_offsets: list[int]
    globals: list[int]
    tea_calls: list[int]


def parse_base(path: Path, explicit: int | None) -> int:
    if explicit is not None:
        return explicit
    match = re.search(r"_([0-9A-Fa-f]{8})\.bin$", path.name)
    if match:
        return int(match.group(1), 16)
    raise SystemExit("generated PE base not supplied and not present in filename")


def collect_operands(insns) -> tuple[list[int], list[int]]:
    immediates: list[int] = []
    displacements: list[int] = []
    for ins in insns:
        for op in ins.operands:
            if op.type == X86_OP_IMM:
                immediates.append(op.imm & 0xFFFFFFFF)
            elif op.type == X86_OP_MEM and op.mem.disp:
                displacements.append(op.mem.disp & 0xFFFFFFFF)
    return immediates, displacements


def find_call_sites(data: bytes, sections: list[Section], target: int, image_base: int) -> list[int]:
    text = next(section for section in sections if section.name == ".text")
    blob = data[text.raw_pointer : text.raw_pointer + text.raw_size]
    sites: list[int] = []
    for index in range(0, max(0, len(blob) - 5)):
        if blob[index] != 0xE8:
            continue
        va = image_base + text.va + index
        rel = struct.unpack_from("<i", blob, index + 1)[0]
        if (va + 5 + rel) & 0xFFFFFFFF == target:
            sites.append(va)
    return sites


def summarize_guard_call(
    data: bytes, sections: list[Section], image_base: int, loaded_base: int, call_site: int
) -> GuardCallSummary:
    md = Cs(CS_ARCH_X86, CS_MODE_32)
    md.detail = True
    start = max(image_base, call_site - 0x90)
    end = call_site + 0x120
    start_off = preferred_to_offset(start, sections, image_base)
    if start_off is None:
        return GuardCallSummary(call_site, call_site, [], [], [], [])
    code = data[start_off : start_off + (end - start)]
    insns = list(md.disasm(code, start))
    before = [ins for ins in insns if ins.address < call_site]
    after = [ins for ins in insns if ins.address > call_site]
    immediates, displacements = collect_operands(before + after[:80])
    source_lo = loaded_base + SOURCE_TABLE_START_OFF
    source_hi = loaded_base + SOURCE_TABLE_END_OFF
    length_lo = loaded_base + LENGTH_TABLE_START_OFF
    length_hi = loaded_base + LENGTH_TABLE_END_OFF
    global_lo = loaded_base + GLOBAL_WINDOW_START_OFF
    global_hi = loaded_base + GLOBAL_WINDOW_END_OFF
    source_offsets = sorted({value for value in immediates + displacements if source_lo <= value < source_hi})
    length_offsets = sorted({value for value in immediates + displacements if length_lo <= value < length_hi})
    globals_used = sorted({value for value in immediates + displacements if global_lo <= value < global_hi})
    tea_calls = [ins.address for ins in after[:120] if ins.mnemonic == "call" and ins.op_str == f"0x{TEA_WRAPPER:x}"]
    return GuardCallSummary(
        call=call_site,
        function_hint=before[0].address if before else call_site,
        source_offsets=source_offsets[:6],
        length_offsets=length_offsets[:6],
        globals=globals_used[:16],
        tea_calls=tea_calls[:4],
    )


def read_scope_table(data: bytes, loaded_base: int, address: int) -> list[tuple[int, int, int]]:
    offset = runtime_to_offset(address, loaded_base)
    if offset is None:
        return []
    rows: list[tuple[int, int, int]] = []
    cursor = offset
    for _ in range(8):
        if cursor + 12 > len(data):
            break
        previous, filter_addr, handler_addr = struct.unpack_from("<III", data, cursor)
        if previous != 0xFFFFFFFF or filter_addr < loaded_base or handler_addr < loaded_base:
            break
        rows.append((previous, filter_addr, handler_addr))
        cursor += 12
        if cursor + 4 <= len(data) and struct.unpack_from("<I", data, cursor)[0] == 0:
            cursor += 4
            break
    return rows


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("generated_pe", type=Path)
    parser.add_argument("--base", type=lambda value: int(value, 0), default=None)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()

    data = args.generated_pe.read_bytes()
    loaded_base = parse_base(args.generated_pe, args.base)
    image_base, entry_rva, sections = parse_pe(data)

    def global_u32(address: int) -> int | None:
        return read_u32(data, runtime_to_offset(address, loaded_base))

    lines = [
        "# Generated Armadillo PE Analysis",
        "",
        f"Input: `{args.generated_pe}`",
        f"Loaded base: `0x{loaded_base:08X}`",
        f"Preferred image base: `0x{image_base:08X}`",
        f"Entry RVA: `0x{entry_rva:X}`",
        "",
        "## Sections",
        "",
    ]
    for section in sections:
        lines.append(
            f"- `{section.name}` rva `0x{section.va:X}` vsize `0x{section.vs:X}` raw `0x{section.raw_pointer:X}`+`0x{section.raw_size:X}` flags `0x{section.flags:X}`"
        )

    lines.extend(["", "## Runtime Globals", ""])
    globals_to_read = [
        ("original_image_base", loaded_base + ORIGINAL_IMAGE_BASE_GLOBAL_OFF),
        ("generated_loaded_base", loaded_base + LOADED_BASE_GLOBAL_OFF),
        ("relocation_delta", loaded_base + DELTA_GLOBAL_OFF),
        ("hash_or_mapper_seed", loaded_base + HASH_OR_SEED_GLOBAL_OFF),
        ("context_record", loaded_base + CONTEXT_RECORD_GLOBAL_OFF),
        ("callback_hash_seed", loaded_base + 0x38114),
        ("callback_virtual_protect", loaded_base + 0x38118),
        ("callback_misc_a", loaded_base + 0x38100),
    ]
    for label, address in globals_to_read:
        value = global_u32(address)
        text = "unreadable" if value is None else f"0x{value:08X}"
        lines.append(f"- `{label}` @ `0x{address:08X}` = `{text}`")

    lines.extend(["", "## SEH Scope Tables", ""])
    for table in (loaded_base + SCOPE_TABLE_A_OFF, loaded_base + SCOPE_TABLE_B_OFF):
        rows = read_scope_table(data, loaded_base, table)
        if not rows:
            lines.append(f"- `0x{table:08X}` unreadable or no rows")
            continue
        for index, (_previous, filter_addr, handler_addr) in enumerate(rows):
            lines.append(
                f"- table `0x{table:08X}` row `{index}` filter `0x{filter_addr:08X}` combined_off `0x{filter_addr - loaded_base:X}` handler `0x{handler_addr:08X}` combined_off `0x{handler_addr - loaded_base:X}`"
            )

    guard_sites = find_call_sites(data, sections, GUARD_TEXT, image_base)
    lines.extend(["", "## Guard/Decrypt Call Rows", "", f"Found `{len(guard_sites)}` calls to `0x{GUARD_TEXT:08X}`.", ""])
    for row in [summarize_guard_call(data, sections, image_base, loaded_base, site) for site in guard_sites]:
        source_values = []
        for address in row.source_offsets:
            value = global_u32(address)
            source_values.append(f"0x{address:08X}->0x{value:08X}" if value is not None else f"0x{address:08X}->?")
        length_values = []
        for address in row.length_offsets:
            value = global_u32(address)
            length_values.append(f"0x{address:08X}->0x{value:08X}" if value is not None else f"0x{address:08X}->?")
        lines.append(
            f"- call `0x{row.call:08X}` hint `0x{row.function_hint:08X}` tea_calls `{','.join(f'0x{x:08X}' for x in row.tea_calls) or '-'}`"
        )
        if source_values:
            lines.append(f"  source table: `{', '.join(source_values)}`")
        if length_values:
            lines.append(f"  length table: `{', '.join(length_values)}`")
        if row.globals:
            lines.append(f"  globals: `{', '.join(f'0x{x:08X}' for x in row.globals)}`")

    if args.output:
        write_markdown(args.output, lines)
    else:
        print("\n".join(lines) + "\n", end="")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
