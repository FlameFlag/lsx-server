from __future__ import annotations

import hashlib
import struct
from dataclasses import dataclass
from typing import Callable, Generic, Iterable, TypeVar


T = TypeVar("T")


@dataclass
class _Cluster(Generic[T]):
    start: int
    end: int
    items: list[T]


def sha256_hex(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def read_u32(data: bytes, offset: int | None) -> int | None:
    if offset is None or offset < 0 or offset + 4 > len(data):
        return None
    return struct.unpack_from("<I", data, offset)[0]


def u32le(data: bytes, offset: int) -> int:
    return struct.unpack_from("<I", data, offset)[0]


def changed_byte_ranges(before: bytes, after: bytes, min_run: int = 1) -> tuple[int, list[tuple[int, int]]]:
    total = 0
    ranges: list[tuple[int, int]] = []
    index = 0
    size = min(len(before), len(after))
    while index < size:
        if before[index] == after[index]:
            index += 1
            continue
        start = index
        while index < size and before[index] != after[index]:
            index += 1
        end = index
        total += end - start
        if end - start >= min_run:
            ranges.append((start, end))
    total += abs(len(before) - len(after))
    return total, ranges


def contiguous_ranges(offsets: Iterable[int], limit: int | None = None) -> tuple[int, list[tuple[int, int]]]:
    ranges: list[tuple[int, int]] = []
    for offset in sorted(offsets):
        if not ranges or offset != ranges[-1][1]:
            ranges.append((offset, offset + 1))
        else:
            ranges[-1] = (ranges[-1][0], offset + 1)
    return len(ranges), ranges if limit is None else ranges[:limit]


def grouped_by_gap(items: Iterable[T], bounds: Callable[[T], tuple[int, int]], gap: int) -> list[tuple[int, int, list[T]]]:
    clusters: list[_Cluster[T]] = []
    for item in items:
        start, end = bounds(item)
        if not clusters or start - clusters[-1].end > gap:
            clusters.append(_Cluster(start, end, [item]))
        else:
            clusters[-1].end = max(clusters[-1].end, end)
            clusters[-1].items.append(item)
    return [(cluster.start, cluster.end, cluster.items) for cluster in clusters]


def int_field(value: object) -> int:
    if isinstance(value, int):
        return value
    if isinstance(value, str | bytes | bytearray):
        return int(value)
    raise TypeError(f"expected integer-like field, got {type(value).__name__}")


def patch_ranges(source: bytes, target: bytes) -> list[dict[str, object]]:
    patches: list[dict[str, object]] = []
    index = 0
    size = min(len(source), len(target))
    while index < size:
        if source[index] == target[index]:
            index += 1
            continue
        start = index
        while index < size and source[index] != target[index]:
            index += 1
        end = index
        patches.append(
            {
                "offset": start,
                "length": end - start,
                "observed": source[start:end].hex(),
                "canonical": target[start:end].hex(),
            }
        )
    return patches


def printable_strings(data: bytes, minimum: int = 8) -> list[tuple[int, str]]:
    strings: list[tuple[int, str]] = []
    start = -1
    for offset, value in enumerate(data + b"\0"):
        printable = value in (9, 10, 13) or 32 <= value < 127
        if printable and start < 0:
            start = offset
        elif not printable and start >= 0:
            if offset - start >= minimum:
                strings.append((start, data[start:offset].decode("latin1")))
            start = -1
    return strings
