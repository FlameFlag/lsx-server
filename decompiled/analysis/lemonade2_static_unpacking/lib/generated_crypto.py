from __future__ import annotations

import struct


PRNG_MULTIPLIER = 0x01DF5E0D
PRNG_MODULUS = 100000000
PRNG_MULTIPLIER_INVERSE = pow(PRNG_MULTIPLIER, -1, PRNG_MODULUS)
TEA_DELTA = 0x61C88647
TEA_DECRYPT_SUM = 0xC6EF3720


def u32(value: int) -> int:
    return value & 0xFFFFFFFF


def s32(value: int) -> int:
    value &= 0xFFFFFFFF
    return value - 0x100000000 if value & 0x80000000 else value


def idiv_qr(value: int, divisor: int) -> tuple[int, int]:
    value = s32(value)
    divisor = s32(divisor)
    quotient = abs(value) // abs(divisor)
    if (value < 0) ^ (divisor < 0):
        quotient = -quotient
    remainder = value - quotient * divisor
    return s32(quotient), s32(remainder)


def prng_mul(a: int, b: int) -> int:
    return (((((b // 10000) * (a % 10000)) + ((a // 10000) * (b % 10000))) % 10000) * 10000 + ((b % 10000) * (a % 10000))) % PRNG_MODULUS


def prng_mul_signed(a: int, b: int) -> int:
    q1, r1 = idiv_qr(a, 10000)
    q2, r2 = idiv_qr(b, 10000)
    high = u32(u32(q2 * r1) + u32(q1 * r2)) % 10000
    low = u32(r2 * r1)
    return u32(u32(high * 10000) + low) % PRNG_MODULUS


def prng_byte(state: int) -> tuple[int, int]:
    state = ((state * PRNG_MULTIPLIER) + 1) % PRNG_MODULUS
    return state, ((state // 10000) << 8) // 10000


def prng_byte_signed(state: int) -> tuple[int, int]:
    state = u32(prng_mul_signed(state, PRNG_MULTIPLIER) + 1) % PRNG_MODULUS
    return state, ((((state // 10000) << 8) // 10000) & 0xFF)


def prng_byte_interval(value: int) -> tuple[int, int]:
    lo = (value * PRNG_MODULUS + 255) // 256
    hi = ((value + 1) * PRNG_MODULUS + 255) // 256 - 1
    return lo, hi


def recover_prng_seed(bytes_out: bytes) -> int | None:
    if not bytes_out:
        return None
    lo, hi = prng_byte_interval(bytes_out[0])
    for first_state in range(lo, hi + 1):
        state = first_state
        ok = True
        for value in bytes_out[1:]:
            state, got = prng_byte(state)
            if got != value:
                ok = False
                break
        if ok:
            return ((first_state - 1) * PRNG_MULTIPLIER_INVERSE) % PRNG_MODULUS
    return None


def prng_sequence_has_seed(bytes_out: bytes) -> bool:
    return recover_prng_seed(bytes_out) is not None


def generated_prng_dword(state: int) -> tuple[int, int]:
    out = []
    for _ in range(4):
        state, value = prng_byte(state)
        out.append(value)
    return state, out[3] | (out[0] << 24) | (out[1] << 16) | (out[2] << 8)


def prng_dword_signed(state: int) -> tuple[int, int]:
    out = []
    for _ in range(4):
        state, value = prng_byte_signed(state)
        out.append(value)
    return state, out[3] | (out[0] << 24) | (out[1] << 16) | (out[2] << 8)


def xor_dwords_with_prng(data: bytes, seed: int, size: int) -> bytes:
    out = bytearray(data[:size])
    state = seed
    for offset in range(0, len(out) & ~3, 4):
        state, key = generated_prng_dword(state)
        value = struct.unpack_from("<I", out, offset)[0] ^ key
        struct.pack_into("<I", out, offset, value)
    return bytes(out)


def dword_xor_to_prng_bytes(xor_stream: bytes) -> bytes:
    out = bytearray()
    for offset in range(0, len(xor_stream) & ~3, 4):
        value = struct.unpack_from("<I", xor_stream, offset)[0]
        out.extend([(value >> 24) & 0xFF, (value >> 16) & 0xFF, (value >> 8) & 0xFF, value & 0xFF])
    return bytes(out)


def seed_key_words(seed: int) -> tuple[int, int, int, int]:
    seed &= 0xFFFFFFFF
    k1 = ((seed << 24) | (seed >> 8)) & 0xFFFFFFFF
    k2 = (((seed >> 8) << 24) | (k1 >> 8)) & 0xFFFFFFFF
    k3 = (((k1 >> 8) << 24) | (k2 >> 8)) & 0xFFFFFFFF
    return seed, k1, k2, k3


def seed_tea_block(v0: int, v1: int, seed: int, mode: int) -> tuple[int, int]:
    k0, k1, k2, k3 = seed_key_words(seed)
    if mode <= 0:
        sum_value = TEA_DECRYPT_SUM
        for _ in range(32):
            v1 = (v1 - ((((v0 >> 5) + k3) ^ ((v0 << 4) + k2) ^ (sum_value + v0)) & 0xFFFFFFFF)) & 0xFFFFFFFF
            combined = (sum_value + v1) & 0xFFFFFFFF
            sum_value = (sum_value + TEA_DELTA) & 0xFFFFFFFF
            v0 = (v0 - ((((v1 >> 5) + k1) ^ ((v1 << 4) + k0) ^ combined) & 0xFFFFFFFF)) & 0xFFFFFFFF
    else:
        sum_value = 0
        for _ in range(32):
            sum_value = (sum_value - TEA_DELTA) & 0xFFFFFFFF
            v0 = (v0 + ((((v1 >> 5) + k1) ^ ((v1 << 4) + k0) ^ (sum_value + v1)) & 0xFFFFFFFF)) & 0xFFFFFFFF
            v1 = (v1 + ((((v0 >> 5) + k3) ^ ((v0 << 4) + k2) ^ (sum_value + v0)) & 0xFFFFFFFF)) & 0xFFFFFFFF
    return v0, v1


def seed_tea_region(data: bytes, seed: int, mode: int) -> bytes:
    out = bytearray(data)
    end = (len(out) >> 2 & 0x3FFFFFFE) * 4
    prev0 = seed_key_words(seed)[1]
    prev1 = seed_key_words(seed)[3]
    for offset in range(0, end, 8):
        old0, old1 = struct.unpack_from("<II", out, offset)
        new0, new1 = seed_tea_block(old0, old1, seed, mode)
        struct.pack_into("<II", out, offset, new0, new1)
        if mode < 0:
            prev0, prev1 = new0, new1
        elif mode > 1:
            prev0, prev1 = old0, old1
        _ = (prev0, prev1)
    return bytes(out)


def tea_transform(data: bytes, key: tuple[int, int, int, int], chaining: bool) -> bytes:
    out = bytearray(data)
    k0, k1, k2, k3 = key
    carry0 = k1
    carry1 = k3
    for offset in range(0, len(out) & ~7, 8):
        v0, v1 = struct.unpack_from("<II", out, offset)
        total = TEA_DECRYPT_SUM
        for _ in range(32):
            v1 = (v1 - (((v0 >> 5) + carry1) ^ (((v0 << 4) + k2) & 0xFFFFFFFF) ^ ((total + v0) & 0xFFFFFFFF))) & 0xFFFFFFFF
            mix = (total + v1) & 0xFFFFFFFF
            total = (total + TEA_DELTA) & 0xFFFFFFFF
            v0 = (v0 - (((v1 >> 5) + carry0) ^ (((v1 << 4) + k0) & 0xFFFFFFFF) ^ mix)) & 0xFFFFFFFF
        struct.pack_into("<II", out, offset, v0, v1)
        if chaining:
            carry0 = v0
            carry1 = v1
    return bytes(out)


def build_crc32_table() -> tuple[int, ...]:
    values = []
    for item in range(256):
        value = item
        for _ in range(8):
            value = (value >> 1) ^ 0xEDB88320 if value & 1 else value >> 1
        values.append(value & 0xFFFFFFFF)
    return tuple(values)


CRC32_TABLE = build_crc32_table()


def crc32_seed(data: bytes, initial: int) -> int:
    value = initial & 0xFFFFFFFF
    for item in data:
        value = CRC32_TABLE[(value ^ item) & 0xFF] ^ (value >> 8)
    return value ^ 0xFFFFFFFF


def rle_decompress(source: bytes, limit: int) -> bytes | None:
    if not source:
        return None
    if source[0] == 0:
        return source[1:] if len(source) - 1 <= limit else None
    out = bytearray()
    last = 0xFF
    cursor = 1
    while cursor < len(source):
        item = source[cursor]
        cursor += 1
        if item != 0xFF:
            out.append(item)
            last = item
            if len(out) > limit:
                return None
            continue
        if cursor >= len(source):
            return None
        esc = source[cursor]
        cursor += 1
        if esc == 0xFF:
            out.append(0xFF)
            last = 0xFF
        else:
            repeat = esc + 3
            if len(out) + repeat >= limit:
                return None
            out.extend([last] * repeat)
    return bytes(out)
