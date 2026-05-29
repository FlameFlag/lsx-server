from __future__ import annotations

from dataclasses import dataclass

from construct import Byte, Int16ul, Int32ul, Struct

from lib.binary import read_u32


MAPPER_METADATA_DATA_OFFSET = 0xFE
MetadataHeader = Struct("flags" / Int32ul, "keys" / Int32ul[4])
Length16 = Struct("value" / Int16ul)
RecordHeader = Struct("kind" / Byte, "length" / Byte, "record_id" / Int32ul)
ValidationEntryHeader = Struct("size" / Int32ul, "tag" / Byte)


@dataclass(frozen=True)
class MetadataRecord:
    kind: int
    record_id: int
    payload: bytes


@dataclass(frozen=True)
class ValidationEntry:
    offset: int
    tag: int
    payload: bytes


def mapper_magic_from_data1(data1: bytes, base: int = 0x310) -> int:
    a = read_u32(data1, base + 0x38)
    b = read_u32(data1, base + 0x24)
    c = read_u32(data1, base + 0x10)
    if a is None or b is None or c is None:
        raise ValueError("data1 mapper magic fields are out of range")
    return a ^ b ^ c


def validation_entries(metadata: bytes, data_offset: int = MAPPER_METADATA_DATA_OFFSET) -> list[ValidationEntry]:
    entries: list[ValidationEntry] = []
    cursor = data_offset
    while cursor + ValidationEntryHeader.sizeof() <= len(metadata):
        header = ValidationEntryHeader.parse(metadata[cursor : cursor + ValidationEntryHeader.sizeof()])
        if header["size"] == 0:
            break
        start = cursor + ValidationEntryHeader.sizeof()
        end = start + header["size"]
        if end > len(metadata):
            break
        entries.append(ValidationEntry(cursor, int(header["tag"]), metadata[start:end]))
        cursor = end
    return entries


def summarize_mapper_metadata(metadata: bytes) -> list[str]:
    lines: list[str] = []
    if len(metadata) < 0x1A:
        return [f"- metadata too short: `0x{len(metadata):X}`"]
    header = MetadataHeader.parse(metadata[: MetadataHeader.sizeof()])
    cursor = 0x18
    product_len = Length16.parse(metadata[cursor : cursor + 2]).value
    cursor += 2
    product = metadata[cursor : cursor + product_len].decode("latin1", errors="replace")
    cursor += product_len
    lines.append(f"- metadata flags: `0x{int(header['flags']):08X}`")
    lines.append(f"- metadata key dwords: `{', '.join(f'0x{int(item):08X}' for item in header['keys'])}`")
    lines.append(f"- product name: `{product}`")
    if cursor < len(metadata) and metadata[cursor] == 0:
        cursor += 1
    if cursor + 2 > len(metadata):
        return lines + ["- dependency blob missing"]
    dependency_blob_len = Length16.parse(metadata[cursor : cursor + 2]).value
    cursor += 2
    dependency_blob = metadata[cursor : cursor + dependency_blob_len]
    dependencies = [item.decode("latin1", errors="replace") for item in dependency_blob.split(b"\0") if item]
    lines.append(f"- dependency blob size: `0x{dependency_blob_len:X}`")
    for index, dependency in enumerate(dependencies[:16]):
        lines.append(f"- dependency `{index}`: `{dependency}`")
    cursor += dependency_blob_len
    record_count = 0
    while cursor + RecordHeader.sizeof() <= len(metadata) and metadata[cursor + 1] != 0:
        header = RecordHeader.parse(metadata[cursor : cursor + RecordHeader.sizeof()])
        end = cursor + RecordHeader.sizeof() + header["length"]
        if end > len(metadata):
            break
        payload = metadata[cursor + RecordHeader.sizeof() : end]
        lines.append(
            f"- record `{record_count}`: kind `0x{int(header['kind']):02X}` id `0x{int(header['record_id']):08X}` payload `{payload.hex()}`"
        )
        cursor = end
        record_count += 1
    lines.append(f"- metadata records: `{record_count}`")
    cursor += 2
    entry_count = 0
    while cursor + ValidationEntryHeader.sizeof() <= len(metadata):
        header = ValidationEntryHeader.parse(metadata[cursor : cursor + ValidationEntryHeader.sizeof()])
        if header["size"] == 0:
            break
        start = cursor + ValidationEntryHeader.sizeof()
        end = start + header["size"]
        if end > len(metadata):
            lines.append(f"- data entry `{entry_count}` at `0x{cursor:X}` exceeds metadata size")
            break
        tail = metadata[end - 8 : end].hex()
        lines.append(f"- data entry `{entry_count}`: offset `0x{cursor:X}` size `0x{int(header['size']):X}` tag `0x{int(header['tag']):02X}` tail `{tail}`")
        cursor = end
        entry_count += 1
    lines.append(f"- metadata data entries: `{entry_count}`")
    return lines
