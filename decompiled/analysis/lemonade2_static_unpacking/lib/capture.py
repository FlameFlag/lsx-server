from __future__ import annotations

import csv
import hashlib
import re
import zipfile


def normalized_member_name(name: str) -> str:
    return name.replace("\\", "/")


def read_zip_text(zf: zipfile.ZipFile, suffix: str) -> str:
    for name in zf.namelist():
        if normalized_member_name(name).endswith(suffix):
            data = zf.read(name)
            if data.startswith((b"\xff\xfe", b"\xfe\xff")):
                return data.decode("utf-16", errors="replace")
            return data.decode("utf-8", errors="replace")
    return ""


def zip_members_by_suffix(zf: zipfile.ZipFile, suffix: str) -> list[str]:
    return [name for name in zf.namelist() if normalized_member_name(name).endswith(suffix)]


def parse_summary(zf: zipfile.ZipFile) -> list[dict[str, str]]:
    rows: list[dict[str, str]] = []
    for name in zip_members_by_suffix(zf, "summary.csv"):
        text = zf.read(name).decode("utf-8", errors="replace")
        for row in csv.DictReader(text.splitlines()):
            row["summary_path"] = name
            rows.append(row)
    return rows


def sha256_member(zf: zipfile.ZipFile, name: str) -> str:
    h = hashlib.sha256()
    with zf.open(name) as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def snapshot_section_name(filename: str) -> str:
    normalized = normalized_member_name(filename)
    if "/preinit_data_" in normalized or normalized.startswith("preinit_data_"):
        return "data"
    match = re.search(r"snap_\d+_([^_]+)_", normalized)
    return match.group(1) if match else ""


def snapshot_seq(filename: str) -> int:
    normalized = normalized_member_name(filename)
    if "/preinit_data_" in normalized or normalized.startswith("preinit_data_"):
        return -1
    match = re.search(r"snap_(\d+)_", normalized)
    return int(match.group(1)) if match else 0


def section_for_capture_member(name: str) -> str:
    section = snapshot_section_name(name)
    return f".{section}" if section in ("text", "rdata", "data") else ""


def first_member_containing(zf: zipfile.ZipFile, needle: str) -> bytes | None:
    candidates = [name for name in zf.namelist() if needle.lower() in name.lower()]
    if not candidates:
        return None
    candidates.sort()
    return zf.read(candidates[0])
