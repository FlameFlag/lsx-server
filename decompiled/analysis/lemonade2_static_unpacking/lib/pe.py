from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path

import pefile


DEFAULT_IMAGE_BASE = 0x400000


@dataclass(frozen=True)
class Section:
    name: str
    va: int
    data: bytes
    vs: int = 0
    raw_size: int = 0
    raw_pointer: int = 0
    flags: int = 0


@dataclass(frozen=True)
class PEImage:
    machine: int
    characteristics: int
    image_base: int
    entry_rva: int
    size_of_image: int
    sections: list[Section]


def _section_name(section: pefile.SectionStructure) -> str:
    name = getattr(section, "Name")
    return name.split(b"\0")[0].decode("latin1", errors="replace")


def load_pe_sections(path: Path, image_base: int = DEFAULT_IMAGE_BASE) -> dict[str, Section]:
    data = path.read_bytes()
    pe = pefile.PE(data=data, fast_load=True)
    sections: dict[str, Section] = {}
    for section in pe.sections:
        virtual_size = section.Misc_VirtualSize
        raw_size = section.SizeOfRawData
        raw_offset = section.PointerToRawData
        blob = bytearray(max(virtual_size, raw_size))
        if raw_size:
            blob[:raw_size] = data[raw_offset : raw_offset + raw_size]
        sections[_section_name(section)] = Section(
            name=_section_name(section),
            va=image_base + section.VirtualAddress,
            data=bytes(blob),
            vs=virtual_size,
            raw_size=raw_size,
            raw_pointer=raw_offset,
            flags=section.Characteristics,
        )
    return sections


def parse_pe_metadata(data: bytes) -> PEImage:
    pe = pefile.PE(data=data, fast_load=True)
    sections = [
        Section(
            name=_section_name(section),
            va=section.VirtualAddress,
            data=b"",
            vs=section.Misc_VirtualSize,
            raw_size=section.SizeOfRawData,
            raw_pointer=section.PointerToRawData,
            flags=section.Characteristics,
        )
        for section in pe.sections
    ]
    return PEImage(
        machine=pe.FILE_HEADER.Machine,
        characteristics=pe.FILE_HEADER.Characteristics,
        image_base=pe.OPTIONAL_HEADER.ImageBase,
        entry_rva=pe.OPTIONAL_HEADER.AddressOfEntryPoint,
        size_of_image=pe.OPTIONAL_HEADER.SizeOfImage,
        sections=sections,
    )


def parse_pe_image(data: bytes) -> tuple[int, int, list[Section]]:
    pe = parse_pe_metadata(data)
    return pe.image_base, pe.entry_rva, pe.sections


def runtime_to_offset(address: int, loaded_base: int) -> int | None:
    offset = address - loaded_base
    return offset if offset >= 0 else None


def preferred_to_offset(address: int, sections: list[Section], image_base: int) -> int | None:
    rva = address - image_base
    for section in sections:
        span = max(section.vs, section.raw_size)
        if section.va <= rva < section.va + span:
            return section.raw_pointer + (rva - section.va)
    if sections and 0 <= rva < min(section.raw_pointer for section in sections):
        return rva
    return None
