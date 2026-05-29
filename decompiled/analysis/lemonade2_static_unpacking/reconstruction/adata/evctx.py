#!/usr/bin/env python3
from pathlib import Path
import hashlib
import re

from lib.common import ADATA_BASE, first_layer_adata, load_pe_sections, packed_exe_path, rol8, ror8, runtime_adata_path, trace_path

BASE = ADATA_BASE
TRACE = trace_path().read_text(errors="replace").splitlines()
RUNTIME = runtime_adata_path().read_bytes()

ctx=[]
for line in TRACE:
    m=re.search(r"EVCTX .* ex=([0-9A-F]+) addr=([0-9A-F]+).* eax=([0-9A-F]+) ebx=([0-9A-F]+) ecx=([0-9A-F]+) edx=([0-9A-F]+)",line)
    if m:
        ex,addr,eax,ebx,ecx,edx=m.groups()
        addr=int(addr,16); ebx=int(ebx,16); ecx=int(ecx,16); edx=int(edx,16)
        if 0x4d3000 <= addr < 0x4e3000:
            ctx.append((ex,addr,ebx,ecx,edx))

# collapse paired invalid-op events with identical state
uniq=[]
last=None
for row in ctx:
    key=(row[2],row[4])
    if key == last:
        continue
    uniq.append(row)
    last=key

adata=bytearray(first_layer_adata(load_pe_sections(packed_exe_path())[".adata"].data)[0])

def op_for(block):
    off=block-BASE
    window=bytes(RUNTIME[off:off+0x60])
    if b"\x30\x08\xd2\x00\x40\x49\x85\xc9\x75\xf6" in window:
        return "xor_rol_cl"
    if b"\xd2\x08\xfe\x00\x40\x49\x85\xc9\x75\xf6" in window:
        return "ror_cl_inc"
    if b"\xd2\x00\x40\x49\x85\xc9\x75\xf8" in window:
        return "rol_cl"
    if b"\xfe\x00\x40\x49\x85\xc9\x75\xf8" in window or b"\xfe\x00\x40\x49\x85\xc9\x75\xf6" in window:
        return "inc"
    if b"\xfe\x08\x40\x49\x85\xc9\x75\xf8" in window:
        return "dec"
    if b"\x30\x08\x40\x49\x85\xc9\x75\xf8" in window:
        return "xor_cl"
    m=re.search(b"\x80\x30(.)\x40\x49\x85\xc9\x75\xf7", window, re.S)
    if m:
        return ("xor_imm", m.group(1)[0])
    return "unknown"

applied=[]
for i,row in enumerate(uniq):
    ex,addr,ebx,ecx,edx=row
    if not (BASE <= edx < BASE+len(adata)):
        continue
    nxt=None
    for j in range(i+1,len(uniq)):
        n=uniq[j][4]
        if BASE <= n <= BASE+len(adata) and n > edx:
            nxt=n
            break
    if not nxt:
        continue
    n=nxt-edx
    if n <= 0 or n > 0x8000:
        continue
    op=op_for(ebx)
    off=edx-BASE
    before=hashlib.sha1(adata[off:off+n]).hexdigest()[:8]
    if op=="inc":
        for k in range(n): adata[off+k]=(adata[off+k]+1)&0xff
    elif op=="dec":
        for k in range(n): adata[off+k]=(adata[off+k]-1)&0xff
    elif op=="xor_cl":
        for k in range(n): adata[off+k]^=(n-k)&0xff
    elif op=="xor_rol_cl":
        for k in range(n):
            cl=(n-k)&0xff
            adata[off+k]=rol8(adata[off+k]^cl, cl)
    elif op=="ror_cl_inc":
        for k in range(n):
            cl=(n-k)&0xff
            adata[off+k]=(ror8(adata[off+k], cl)+1)&0xff
    elif op=="rol_cl":
        for k in range(n):
            cl=(n-k)&0xff
            adata[off+k]=rol8(adata[off+k], cl)
    elif isinstance(op, tuple) and op[0]=="xor_imm":
        imm=op[1]
        for k in range(n): adata[off+k]^=imm
    else:
        applied.append((addr,ebx,edx,n,op,before,"skip"))
        continue
    after=hashlib.sha1(adata[off:off+n]).hexdigest()[:8]
    applied.append((addr,ebx,edx,n,op,before,after))

Path("/tmp/lemonade2_adata_evctx_attempt.bin").write_bytes(adata)
diff=sum(a!=b for a,b in zip(adata,RUNTIME))
print("unique",len(uniq),"applied",sum(1 for x in applied if x[-1]!="skip"),"diff",diff)
print("sha",hashlib.sha256(adata).hexdigest())
for x in applied[:120]:
    print("addr=%08X ebx=%08X edx=%08X len=%04X op=%s %s->%s"%x)
