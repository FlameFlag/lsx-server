#!/usr/bin/env python3
import hashlib
import re
import struct
from capstone import *
from capstone.x86 import *

from lib.common import first_layer_adata, load_pe_sections, packed_exe_path, runtime_adata_path, trace_path

TRACE = trace_path().read_text(errors="replace").splitlines()
RUNTIME_ADATA = runtime_adata_path().read_bytes()
adata_section = load_pe_sections(packed_exe_path())[".adata"]
adata_va = adata_section.va
adata0 = adata_section.data

events = []
for line in TRACE:
    m = re.search(r"code=EXCEPTION .* ex=([0-9A-F]+) addr=([0-9A-F]+)", line)
    if m:
        ex, addr = m.group(1), int(m.group(2), 16)
        if 0x4d3000 <= addr < 0x4e3000:
            events.append((ex, addr))

md = Cs(CS_ARCH_X86, CS_MODE_32)
md.detail = True

def dis_range(blob, start, size):
    off = start - adata_va
    if off < 0:
        return []
    return list(md.disasm(bytes(blob[off:off+size]), start))

def read32(blob, addr):
    off = addr - adata_va
    if off < 0 or off + 4 > len(blob):
        return None
    return struct.unpack_from("<I", blob, off)[0]

def write_blob(blob, addr, data):
    off = addr - adata_va
    if off < 0 or off + len(data) > len(blob):
        return False
    blob[off:off+len(data)] = data
    return True

def find_handler_call(blob, fault):
    # The invalid op is inside a tiny function reached by a nearby direct call.
    # Search backward for call rel32 whose target is within a few bytes of fault.
    lo = max(adata_va, fault - 0x80)
    calls = []
    start = lo - adata_va
    end = min(len(blob) - 5, fault - adata_va + 0x20)
    for off in range(start, end):
        if blob[off] != 0xE8:
            continue
        rel = struct.unpack_from("<i", blob, off + 1)[0]
        addr = adata_va + off
        tgt = (addr + 5 + rel) & 0xffffffff
        if abs(tgt - fault) < 0x30 or (tgt <= fault <= tgt + 0x30):
            calls.append(addr)
    return calls[-1] if calls else None

def backward_block(blob, call_addr):
    lo = max(adata_va, call_addr - 0x80)
    insns = [i for i in dis_range(blob, lo, call_addr - lo + 5) if i.address < call_addr]
    # Start after the previous ret/jmp/call when possible.
    start = 0
    for idx, ins in enumerate(insns):
        if ins.mnemonic in ("ret", "jmp") or (ins.mnemonic == "call"):
            start = idx + 1
    return insns[start:]

def imm_of(ins, reg_name):
    if len(ins.operands) == 2 and ins.operands[0].type == X86_OP_REG and ins.reg_name(ins.operands[0].reg) == reg_name and ins.operands[1].type == X86_OP_IMM:
        return ins.operands[1].imm & 0xffffffff
    return None

def summarize_block(blob, call_addr):
    block = backward_block(blob, call_addr)
    eax = ecx = None
    ops = []
    for ins in block:
        if ins.mnemonic == "mov":
            v = imm_of(ins, "eax")
            if v is not None: eax = v
            v = imm_of(ins, "ecx")
            if v is not None: ecx = v
        elif ins.mnemonic == "add" and len(ins.operands)==2 and ins.operands[0].type==X86_OP_REG and ins.reg_name(ins.operands[0].reg)=="eax":
            if eax is not None and ins.operands[1].type == X86_OP_REG and ins.reg_name(ins.operands[1].reg) == "ebp":
                eax = (eax + 0x4d3000) & 0xffffffff
            elif eax is not None and ins.operands[1].type == X86_OP_IMM:
                eax = (eax + ins.operands[1].imm) & 0xffffffff
        elif ins.mnemonic in ("xor","dec","rol","ror") or ins.mnemonic.startswith("loop"):
            ops.append(f"{ins.mnemonic} {ins.op_str}")
    return block, eax, ecx, ops

seen = set()
print("events", len(events), "unique", len(set(a for _,a in events)))
print("runtime adata sha", hashlib.sha256(RUNTIME_ADATA).hexdigest())

for ex, fault in events:
    if fault in seen:
        continue
    seen.add(fault)
    blob = bytearray(first_layer_adata(adata0)[0])
    call = find_handler_call(blob, fault)
    call_rt = find_handler_call(RUNTIME_ADATA, fault)
    source = "xor1"
    # Use runtime-good bytes when the first-layer view has not decoded this far yet.
    if call_rt and call_rt != call:
        call = call_rt
        blob = bytearray(RUNTIME_ADATA)
        source = "runtime"
    if not call:
        continue
    block, eax, ecx, ops = summarize_block(blob, call)
    print(f"\nFAULT {ex} {fault:08X} call={call:08X} source={source} eax={eax if eax is None else hex(eax)} ecx={ecx if ecx is None else hex(ecx)}")
    for ins in block[-14:]:
        print(f"  {ins.address:08X} {ins.mnemonic:<7} {ins.op_str}")
    if ops:
        print("  ops:", "; ".join(ops[-6:]))
