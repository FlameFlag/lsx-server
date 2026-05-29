from __future__ import annotations

import os
import struct
from pathlib import Path

from lib.binary import sha256_hex
from lib.pe import Section, load_pe_sections


IMAGE_BASE = 0x400000
ADATA_BASE = 0x4D3000

EXPECTED_PAYLOADS = {
    ".text": (0x91000, "d5855b7313b16e6d1b1e234a48e6c8a099e2908f27a11df17dc29ef31d3bb4f8"),
    ".rdata": (0x0A000, "615c86178ba724a4e84951ab0bc28acc59de65f06106d97e69e2ed821d8eaf5c"),
    ".data": (0x07000, "0b5252485f399379e4094ccd83a999e9f14f9b7aa909b5797d069622c726d496"),
}


def analysis_root() -> Path:
    return Path(__file__).resolve().parents[1]


def repo_root() -> Path:
    return Path(__file__).resolve().parents[4]


def env_path(name: str, default: str | Path) -> Path:
    return Path(os.environ.get(name, str(default))).expanduser()


def packed_exe_path() -> Path:
    return env_path("LEMONADE2_PACKED", repo_root() / "decompiled/local/lemonade2_install/Lemonade2.exe")


def trace_path() -> Path:
    return env_path("LEMONADE2_TRACE", analysis_root() / "traces/evctx/2026-05-30.log")


def runtime_adata_path() -> Path:
    return env_path("LEMONADE2_RUNTIME_ADATA", "/tmp/lemonade2_dumps_now/pid_8344/Lemonade2.adata.bin")


def seh_adata_path() -> Path:
    return env_path("LEMONADE2_SEH_ADATA", "/tmp/lemonade2_adata_seh.bin")


def first_layer_adata(adata: bytes) -> tuple[bytes, int]:
    blob = bytearray(adata)
    marker = 0x17B
    key = struct.unpack_from("<I", blob, marker)[0] ^ 0x5478
    struct.pack_into("<I", blob, marker, 0x5478)
    for i in range(0x17F, min(len(blob), 0x17F + 0x9B4F)):
        blob[i] ^= key & 0xFF
    blob[0x10A] = 0x90
    return bytes(blob), key


def rol8(value: int, shift: int) -> int:
    shift &= 7
    value &= 0xFF
    return value if not shift else ((value << shift) | (value >> (8 - shift))) & 0xFF


def ror8(value: int, shift: int) -> int:
    shift &= 7
    value &= 0xFF
    return value if not shift else ((value >> shift) | (value << (8 - shift))) & 0xFF
