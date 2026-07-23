#!/usr/bin/env python3
"""Validate the canonical Chinese manual and its local screenshot set."""

from __future__ import annotations

import re
import struct
import sys
from pathlib import Path
from urllib.parse import unquote, urlsplit


ROOT = Path(__file__).resolve().parents[1]
MANUAL = ROOT / "docs" / "manual" / "zh-CN"
SCREENSHOTS = MANUAL / "assets" / "screenshots"
LINK_RE = re.compile(r"!?\[[^\]]*\]\(([^)]+)\)")
FORBIDDEN = (
    "X-App-Key",
    "Go 1.21",
    "cmd/migrate",
    "/metrics",
    "/debug/pprof",
)


def png_size(path: Path) -> tuple[int, int]:
    with path.open("rb") as source:
        header = source.read(24)
    if len(header) != 24 or header[:8] != b"\x89PNG\r\n\x1a\n":
        raise ValueError("not a PNG")
    return struct.unpack(">II", header[16:24])


def main() -> int:
    errors: list[str] = []
    markdown_files = sorted(MANUAL.rglob("*.md"))
    referenced_images: set[Path] = set()

    for markdown in markdown_files:
        text = markdown.read_text(encoding="utf-8")
        for stale in FORBIDDEN:
            if stale in text:
                errors.append(f"{markdown.relative_to(ROOT)}: contains stale text {stale!r}")
        for raw_target in LINK_RE.findall(text):
            target = raw_target.strip().split(maxsplit=1)[0].strip("<>")
            parsed = urlsplit(target)
            if parsed.scheme or target.startswith("#"):
                continue
            resolved = (markdown.parent / unquote(parsed.path)).resolve()
            if not resolved.exists():
                errors.append(
                    f"{markdown.relative_to(ROOT)}: missing relative target {target}"
                )
            if resolved.suffix.lower() == ".png":
                referenced_images.add(resolved)

    screenshots = sorted(SCREENSHOTS.rglob("*.png"))
    if len(screenshots) != 39:
        errors.append(f"expected 39 screenshots, found {len(screenshots)}")
    for screenshot in screenshots:
        try:
            size = png_size(screenshot)
        except ValueError as exc:
            errors.append(f"{screenshot.relative_to(ROOT)}: {exc}")
            continue
        if size != (1440, 900):
            errors.append(f"{screenshot.relative_to(ROOT)}: size {size}, want (1440, 900)")
        if screenshot.resolve() not in referenced_images:
            errors.append(f"{screenshot.relative_to(ROOT)}: not referenced by the manual")

    if errors:
        print("Manual validation failed:", file=sys.stderr)
        for error in errors:
            print(f"- {error}", file=sys.stderr)
        return 1
    print(
        f"Manual validation passed: {len(markdown_files)} Markdown files, "
        f"{len(screenshots)} screenshots at 1440x900."
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
