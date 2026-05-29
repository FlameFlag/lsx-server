#!/usr/bin/env python3
"""Summarize optional PAGE_GUARD hits on unresolved .data pages."""

from __future__ import annotations

import argparse
import collections
import re
import zipfile
from pathlib import Path

from lib.capture import read_zip_text
from lib.reporting import write_markdown


DATA_BASE = 0x0049C000
DATA_END = DATA_BASE + 0x7000


def read_trace(capture_zip: Path) -> str:
    with zipfile.ZipFile(capture_zip) as zf:
        return read_zip_text(zf, "trace/trace.log")


def parse_hits(trace: str) -> list[dict[str, int]]:
    hits: list[dict[str, int]] = []
    pattern = re.compile(
        r"DATAGUARD_HIT tid=(?P<tid>\d+) eip=(?P<eip>[0-9A-Fa-f]+) access=(?P<access>\d+) "
        r"addr=(?P<addr>[0-9A-Fa-f]+) page=(?P<page>[0-9A-Fa-f]+)(?: got=(?P<got>[0-9A-Fa-f]+) pre=(?P<pre>[0-9A-Fa-f]+))?.*?"
        r"eax=(?P<eax>[0-9A-Fa-f]+) ebx=(?P<ebx>[0-9A-Fa-f]+) ecx=(?P<ecx>[0-9A-Fa-f]+) "
        r"edx=(?P<edx>[0-9A-Fa-f]+) esi=(?P<esi>[0-9A-Fa-f]+) edi=(?P<edi>[0-9A-Fa-f]+)"
    )
    for match in pattern.finditer(trace):
        row = {
            key: (0 if value is None else (int(value, 16) if key not in ("access", "tid") else int(value)))
            for key, value in match.groupdict().items()
        }
        if DATA_BASE <= row["addr"] < DATA_END:
            hits.append(row)
    return hits


def parse_posts(trace: str) -> list[dict[str, int]]:
    posts: list[dict[str, int]] = []
    pattern = re.compile(
        r"DATAGUARD_POST tid=(?P<tid>\d+) eip=(?P<eip>[0-9A-Fa-f]+) access=(?P<access>\d+) "
        r"addr=(?P<addr>[0-9A-Fa-f]+) page=(?P<page>[0-9A-Fa-f]+) got=(?P<got>[0-9A-Fa-f]+) value=(?P<value>[0-9A-Fa-f]+)"
    )
    for match in pattern.finditer(trace):
        row = {key: int(value, 16) if key not in ("access", "tid") else int(value) for key, value in match.groupdict().items()}
        if DATA_BASE <= row["addr"] < DATA_END:
            posts.append(row)
    return posts


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("run_dir", type=Path)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()

    trace = read_trace(args.run_dir / "capture.zip")
    hits = parse_hits(trace)
    posts = parse_posts(trace)
    writes = [hit for hit in hits if hit["access"] == 1]
    write_posts = [post for post in posts if post["access"] == 1]
    lines = ["# Data Guard Analysis", "", f"Capture archive: `{(args.run_dir / 'capture.zip').name}`", ""]
    lines.append(f"- Total `.data` guard hits: `{len(hits)}`")
    lines.append(f"- Write hits: `{len(writes)}`")
    lines.append(f"- Read hits: `{len(hits) - len(writes)}`")
    lines.append(f"- Post-write samples: `{len(write_posts)}`")
    if writes:
        by_eip = collections.Counter(hit["eip"] for hit in writes)
        lines.extend(["", "## Top Write EIPs", ""])
        for eip, count in by_eip.most_common(30):
            examples = [hit for hit in writes if hit["eip"] == eip][:4]
            addrs = ", ".join(f"0x{hit['addr']:08X}(+0x{hit['addr'] - DATA_BASE:X})" for hit in examples)
            lines.append(f"- `0x{eip:08X}` hits `{count}` examples {addrs}")
        by_page = collections.Counter(hit["page"] for hit in writes)
        lines.extend(["", "## Write Pages", ""])
        for page, count in sorted(by_page.items()):
            lines.append(f"- `0x{page:08X}` hits `{count}`")
        if write_posts:
            lines.extend(["", "## Post-Write Values", ""])
            for post in write_posts[:80]:
                lines.append(
                    f"- eip `0x{post['eip']:08X}` addr `0x{post['addr']:08X}` (+`0x{post['addr'] - DATA_BASE:X}`) got `{post['got']}` value `0x{post['value']:08X}`"
                )
        pre_hits = [hit for hit in writes if hit.get("got")]
        if pre_hits:
            lines.extend(["", "## Pre-Write Values", ""])
            for hit in pre_hits[:80]:
                lines.append(
                    f"- eip `0x{hit['eip']:08X}` addr `0x{hit['addr']:08X}` (+`0x{hit['addr'] - DATA_BASE:X}`) got `{hit['got']}` value `0x{hit['pre']:08X}`"
                )
    else:
        lines.extend(["", "No guard writes were captured. Set `LEMONADE2_DATA_GUARD=1` for an owner-tracing run."])
    write_markdown(args.output, lines)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
