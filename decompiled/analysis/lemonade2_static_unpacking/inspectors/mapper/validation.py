#!/usr/bin/env python3
"""Model FUN_10019D2B validation-entry decryption candidates."""

from __future__ import annotations

import argparse
import hashlib
import struct
import zlib
from pathlib import Path

from lib.binary import sha256_hex
from lib.common import load_pe_sections, repo_root
from lib.generated_crypto import (
    PRNG_MODULUS,
    PRNG_MULTIPLIER,
    prng_dword_signed,
    prng_mul,
    prng_mul_signed,
    tea_transform,
    u32,
    xor_dwords_with_prng,
)
from lib.mapper import mapper_magic_from_data1, validation_entries
from lib.reporting import write_markdown


MAPPER_WINDOW_SEED = 0x031E1692
MAPPER_WINDOW_SIZE = 0x2800
MAPPER_SELECTED_TAIL_OFFSET = 4
MAPPER_METADATA_OFFSET = 0x0C
SHORTV3_ALPHABET = "0123456789ABCDEFGHJKMNPQRTUVWXYZ"
KNOWN_REGISTRATION_NAME = "TestName"
KNOWN_ACTIVATION_KEY = "0000PP-FZYKGQ-JABWAK-Q6XMT6-U0U72Q-CD4Y50-JTAV0G"


def mutate_validation_words(
    words: list[int],
    local20: int,
    local28: int,
    data1_field3c: int,
    long_count_flag: bool,
) -> tuple[list[int], int, int]:
    local20 = u32(prng_mul_signed(local20, PRNG_MULTIPLIER) + 1) % PRNG_MODULUS
    local28 = u32(prng_mul_signed(local28, PRNG_MULTIPLIER) + 1) % PRNG_MODULUS
    shift = (((local28 // 10000) << 4) // 10000 + ((local20 // 10000) << 4) // 10000) & 0x1F
    count = (data1_field3c & (0xFFFF if long_count_flag else 0x7FFF)) + (10000 if long_count_flag else 5000)
    index = 0
    for iteration in range(count):
        if (iteration & 0xFF) == 0:
            index = 0
        selector = (words[index] >> shift) & 3
        if selector == 0:
            local20, value = prng_dword_signed(local20)
            words[index] = u32(words[index] | value)
            index += 1
            local28, _ = prng_dword_signed(local28)
        elif selector == 1:
            local28, value = prng_dword_signed(local28)
            words[index] = u32(words[index] & value)
            index += 1
            local20, _ = prng_dword_signed(local20)
        else:
            local28, a = prng_dword_signed(local28)
            local20, b = prng_dword_signed(local20)
            words[index] = u32(words[index] ^ a ^ b)
            index += 1
    return words, shift, count


def derive_validation_key_full(seed: int, data1_field38: int, data1_field3c: int, long_count_flag: bool) -> tuple[tuple[int, int, int, int], int, int, int, int, str]:
    local20 = u32(data1_field38 ^ seed)
    local28 = seed
    local60 = u32(local20 * 10 + 1)
    words = []
    for _ in range(256):
        local60, a = prng_dword_signed(local60)
        local28, b = prng_dword_signed(local28)
        local20, c = prng_dword_signed(local20)
        words.append(u32(a ^ b ^ c ^ seed))
    words, shift, count = mutate_validation_words(words, local20, local28, data1_field3c, long_count_flag)
    digest = hashlib.md5(b"".join(struct.pack("<I", word) for word in words)).digest()
    tag = digest[0] & 7
    key = struct.unpack("<IIII", digest)
    local44 = u32((~seed & 0xFFFFFFFF) ^ data1_field38)
    local44 = u32(prng_mul_signed(local44, PRNG_MULTIPLIER) + 1) % PRNG_MODULUS
    repeat_count = (((local44 // 10000) * 400) // 10000) + 0x321
    return key, tag, repeat_count, shift, count, digest.hex()


def decrypt_candidate(payload: bytes, key: tuple[int, int, int, int], repeat_count: int) -> bytes:
    decoded = tea_transform(payload, key, chaining=True)
    for _ in range(repeat_count):
        decoded = tea_transform(decoded, key, chaining=False)
    return tea_transform(decoded, key, chaining=True)


def akt_crc32(data: bytes, crc: int = 0xFFFFFFFF) -> int:
    table = []
    for item in range(256):
        value = reverse_bits(item, 8) << 24
        for _ in range(8):
            if value & 0x80000000:
                value = ((value << 1) ^ 0x04C11DB7) & 0xFFFFFFFF
            else:
                value = (value << 1) & 0xFFFFFFFF
        table.append(reverse_bits(value, 32))
    for item in data:
        crc = table[(crc ^ item) & 0xFF] ^ (crc >> 8)
    return crc & 0xFFFFFFFF


def reverse_bits(value: int, count: int) -> int:
    out = 0
    for _ in range(count):
        out = (out << 1) | (value & 1)
        value >>= 1
    return out


def cook_text(value: str) -> str:
    return "".join(item.upper() for item in value if item not in " \t\r\n")


def decode_shortv3_key(key: str) -> bytes:
    compact = key.replace("-", "").lstrip("0")
    if not compact:
        return b""
    values = [SHORTV3_ALPHABET.index(item) for item in compact]
    # encodeShortV3Key marks the most-significant digit by adding 16.
    values[0] -= 16
    if values[0] < 0:
        raise ValueError("invalid ShortV3 marker digit")
    number = 0
    for value in values:
        number = (number << 5) | value
    return number.to_bytes(24, "big")


def decrypt_shortv3_prefix(name: str, key: str) -> bytes:
    decoded = bytearray(decode_shortv3_key(key))
    state = akt_crc32(cook_text(name).encode("latin1"))
    for index in range(6):
        state = (prng_mul(state, 31415821) + 1) % PRNG_MODULUS
        decoded[index] ^= ((state // 10000) * 256) // 10000
    return bytes(decoded[:8])


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--packed", type=Path, default=repo_root() / "decompiled/local/lt2_install/Lemonade2.exe")
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()

    sections = load_pe_sections(args.packed)
    pdata = sections[".pdata"].data
    data1 = sections[".data1"].data
    last_end = 0x2852A
    selected = last_end + MAPPER_SELECTED_TAIL_OFFSET
    mapper_window = xor_dwords_with_prng(pdata[selected : selected + MAPPER_WINDOW_SIZE], MAPPER_WINDOW_SEED, MAPPER_WINDOW_SIZE)
    metadata = zlib.decompress(mapper_window[MAPPER_METADATA_OFFSET + 0x0 :])
    entries = validation_entries(metadata)
    field38 = mapper_magic_from_data1(data1) ^ struct.unpack_from("<I", data1, 0x310 + 0x24)[0] ^ struct.unpack_from("<I", data1, 0x310 + 0x10)[0]
    field3c = struct.unpack_from("<I", data1, 0x310 + 0x3C)[0]

    known_prefix = decrypt_shortv3_prefix(KNOWN_REGISTRATION_NAME, KNOWN_ACTIVATION_KEY)
    parser_seed = struct.unpack_from(">I", known_prefix, 0)[0]
    embedded_cert_seed = struct.unpack_from(">I", known_prefix, 2)[0]

    candidates = [
        ("known_key_parser_dword", parser_seed),
        ("known_key_embedded_cert_seed", embedded_cert_seed),
        ("shortv3_cert_seed", 0xCCF0580A),
        ("shortv3_payload_dword", 0x0A58F0CC),
        ("public_record_id", 0xD074508D),
        ("public_cert_lo", 0x9CC50E4D),
        ("public_cert_hi", 0x25416464),
        ("mapper_window_seed", MAPPER_WINDOW_SEED),
    ]

    lines = ["# Mapper Validation Entry Analysis", "", f"Input: `{args.packed}`", "", "## Entries", ""]
    for index, entry in enumerate(entries):
        lines.append(f"- `{index}` offset `0x{entry.offset:X}` tag `0x{entry.tag:02X}` size `0x{len(entry.payload):X}` sha256 `{sha256_hex(entry.payload)}`")
    lines.extend(
        [
            "",
            "## Known ShortV3 Key Decode",
            "",
            f"- name: `{KNOWN_REGISTRATION_NAME}`",
            f"- key: `{KNOWN_ACTIVATION_KEY}`",
            f"- decrypted first bytes: `{known_prefix.hex()}`",
            f"- parser first dword candidate: `0x{parser_seed:08X}`",
            f"- embedded cert seed: `0x{embedded_cert_seed:08X}`",
            "",
            "## Seed Candidates",
            "",
        ]
    )
    for name, seed in candidates:
        for long_count_flag in (False, True):
            key, tag, repeat_count, shift, count, digest = derive_validation_key_full(seed, field38, field3c, long_count_flag)
            lines.append(f"- `{name}` seed `0x{seed:08X}` e875 `{int(long_count_flag)}` selects tag `0x{tag:02X}` repeat `{repeat_count}` shift `{shift}` count `{count}` md5 `{digest}` key `{','.join(f'0x{part:08X}' for part in key)}`")
            for index, entry in enumerate(entries):
                if entry.tag != tag:
                    continue
                decoded = decrypt_candidate(entry.payload, key, repeat_count)
                trailer_a, trailer_b = struct.unpack_from("<II", decoded, len(decoded) - 8)
                lines.append(
                    f"  - entry `{index}` trailer `0x{trailer_a:08X}`/`0x{trailer_b:08X}` complement `{trailer_a == (~trailer_b & 0xFFFFFFFF)}` first `{decoded[:32].hex()}`"
                )
    lines.extend(
        [
            "",
            "The naturally valid ShortV3 key reaches `FUN_10019D2B` with seed `0xCCF0580A`; exact signed `FUN_10001071` semantics and the 5k mutation loop decrypt validation entry tag `0x04`.",
        ]
    )
    write_markdown(args.output, lines)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
