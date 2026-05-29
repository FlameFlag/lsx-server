from __future__ import annotations

import struct
import unittest

from lib.binary import changed_byte_ranges, contiguous_ranges
from lib.generated_crypto import prng_byte, recover_prng_seed, rle_decompress
from lib.mapper import mapper_magic_from_data1, summarize_mapper_metadata, validation_entries


class BinaryHelpersTest(unittest.TestCase):
    def test_changed_byte_ranges(self) -> None:
        total, ranges = changed_byte_ranges(b"aaXXbbY", b"aaYYbbZ")
        self.assertEqual(total, 3)
        self.assertEqual(ranges, [(2, 4), (6, 7)])

    def test_contiguous_ranges(self) -> None:
        self.assertEqual(contiguous_ranges([3, 4, 8, 9, 10], 1), (2, [(3, 5)]))


class GeneratedCryptoTest(unittest.TestCase):
    def test_recover_prng_seed(self) -> None:
        seed = 0x031E1692
        state = seed
        out = []
        for _ in range(6):
            state, value = prng_byte(state)
            out.append(value)
        self.assertEqual(recover_prng_seed(bytes(out)), seed)

    def test_rle_decompress(self) -> None:
        self.assertEqual(rle_decompress(b"\x00stored", 16), b"stored")
        self.assertEqual(rle_decompress(b"\x01A\xff\x02", 16), b"AAAAAA")


class MapperHelpersTest(unittest.TestCase):
    def test_mapper_magic_from_data1(self) -> None:
        data = bytearray(0x400)
        struct.pack_into("<I", data, 0x310 + 0x38, 0x11111111)
        struct.pack_into("<I", data, 0x310 + 0x24, 0x22222222)
        struct.pack_into("<I", data, 0x310 + 0x10, 0x33333333)
        self.assertEqual(mapper_magic_from_data1(bytes(data)), 0)

    def test_validation_entries(self) -> None:
        metadata = bytearray(0x120)
        struct.pack_into("<IB3s", metadata, 0xFE, 3, 7, b"abc")
        struct.pack_into("<I", metadata, 0x106, 0)
        entries = validation_entries(bytes(metadata))
        self.assertEqual(len(entries), 1)
        self.assertEqual(entries[0].offset, 0xFE)
        self.assertEqual(entries[0].tag, 7)
        self.assertEqual(entries[0].payload, b"abc")

    def test_summarize_mapper_metadata(self) -> None:
        metadata = bytearray(0x80)
        struct.pack_into("<IIIII", metadata, 0, 0xA012, 1, 2, 3, 4)
        struct.pack_into("<H4s", metadata, 0x18, 4, b"Test")
        cursor = 0x1E
        metadata[cursor] = 0
        cursor += 1
        struct.pack_into("<H", metadata, cursor, 8)
        cursor += 2
        metadata[cursor : cursor + 8] = b"a.dll\0b\0"
        cursor += 8
        metadata[cursor : cursor + 2] = b"\0\0"
        cursor += 2
        struct.pack_into("<IB3s", metadata, cursor, 3, 2, b"xyz")
        cursor += 8
        struct.pack_into("<I", metadata, cursor, 0)
        lines = summarize_mapper_metadata(bytes(metadata))
        self.assertIn("- metadata flags: `0x0000A012`", lines)
        self.assertIn("- dependency `0`: `a.dll`", lines)
        self.assertIn("- metadata data entries: `1`", lines)


if __name__ == "__main__":
    unittest.main()
