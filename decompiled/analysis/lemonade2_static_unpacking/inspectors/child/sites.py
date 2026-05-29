#!/usr/bin/env python3
from capstone import *

from lib.common import first_layer_adata, load_pe_sections, packed_exe_path, runtime_adata_path, seh_adata_path

adata_section = load_pe_sections(packed_exe_path())[".adata"]
adata_va = adata_section.va
adata_xor, key = first_layer_adata(adata_section.data)
runtime_adata = runtime_adata_path()
seh_adata = seh_adata_path()

choices = [
    ("packed_xor1", adata_xor),
    ("seh_latest", seh_adata.read_bytes() if seh_adata.exists() else b""),
    ("runtime_good", runtime_adata.read_bytes() if runtime_adata.exists() else b""),
]

md = Cs(CS_ARCH_X86, CS_MODE_32)
md.detail = True

faults = [
    0x004D31B3, 0x004D3888, 0x004D6A84,
    0x004DC5C3, 0x004DC5C5, 0x004DC51F, 0x004DB020,
    0x004DAFA1, 0x004D9BDF, 0x004D8BB2, 0x004D8CEB,
]

def blob_at(blob, addr, size):
    off = addr - adata_va
    if off < 0 or off >= len(blob):
        return b""
    return blob[off:off+size]

def dis(blob, start, before=0x40, size=0xc0):
    lo = max(adata_va, start - before)
    code = blob_at(blob, lo, size)
    for ins in md.disasm(code, lo):
        mark = ">>" if ins.address == start else "  "
        print(f"{mark}{ins.address:08X}  {ins.bytes.hex().upper():<20} {ins.mnemonic:<8} {ins.op_str}")

for label, blob in choices:
    if not blob:
        continue
    print(f"\n===== {label} len=0x{len(blob):x} key=0x{key:08x} =====")
    for f in faults:
        print(f"\n-- around {f:08X} --")
        dis(blob, f)

    print("\n-- fs:[0] / SEH-ish instructions --")
    count = 0
    for ins in md.disasm(blob, adata_va):
        op = ins.op_str.lower()
        bs = ins.bytes
        if "fs:" in op or bs.startswith(b"\x64") or (ins.mnemonic in ("int", "into") or bs.startswith(b"\xf0\xf0")):
            print(f"{ins.address:08X}  {bs.hex().upper():<20} {ins.mnemonic:<8} {ins.op_str}")
            count += 1
            if count >= 140:
                break
