#!/usr/bin/env python3

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path


HEADER_RE = re.compile(r"^#\s+Code\s+Switch\s+(v[^\s]+)\s*$", re.IGNORECASE)
SECTION_SEPARATOR = "---"


def normalize_version(raw: str) -> str:
    value = raw.strip()
    if not value:
        raise ValueError("version is empty")
    if not value.lower().startswith("v"):
        value = f"v{value}"
    return f"v{value[1:]}"


def extract_version_section(text: str, version: str) -> str:
    normalized_target = normalize_version(version)
    normalized_text = text.lstrip("\ufeff").replace("\r\n", "\n").replace("\r", "\n")
    lines = normalized_text.split("\n")

    start_index: int | None = None
    end_index: int | None = None

    for index, line in enumerate(lines):
        match = HEADER_RE.match(line.strip())
        if not match:
            continue

        header_version = normalize_version(match.group(1))
        if start_index is None:
            if header_version == normalized_target:
                start_index = index
            continue

        end_index = index
        break

    if start_index is None:
        raise ValueError(f"version section not found: {normalized_target}")

    section_lines = lines[start_index:end_index]
    while section_lines and not section_lines[-1].strip():
        section_lines.pop()
    if section_lines and section_lines[-1].strip() == SECTION_SEPARATOR:
        section_lines.pop()
    while section_lines and not section_lines[-1].strip():
        section_lines.pop()

    section = "\n".join(section_lines).strip()
    if not section:
        raise ValueError(f"version section is empty: {normalized_target}")

    return f"{section}\n"


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Extract a single version section from RELEASE_NOTES.md style markdown.",
    )
    parser.add_argument("--version", required=True, help="Version or tag, for example 2.7.20 or v2.7.20")
    parser.add_argument("--notes-file", required=True, help="Path to the markdown release notes file")
    parser.add_argument("--output", help="Optional output path. Prints to stdout when omitted.")
    args = parser.parse_args()

    notes_path = Path(args.notes_file)
    if not notes_path.is_file():
        print(f"release notes file not found: {notes_path}", file=sys.stderr)
        return 1

    try:
        content = notes_path.read_text(encoding="utf-8-sig")
        section = extract_version_section(content, args.version)
    except Exception as exc:
        print(str(exc), file=sys.stderr)
        return 1

    if args.output:
        output_path = Path(args.output)
        output_path.parent.mkdir(parents=True, exist_ok=True)
        with output_path.open("w", encoding="utf-8", newline="\n") as f:
            f.write(section)
    else:
        sys.stdout.write(section)

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
