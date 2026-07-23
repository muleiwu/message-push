#!/usr/bin/env python3
"""Publish the canonical Chinese manual to M-Doc without deleting articles."""

from __future__ import annotations

import argparse
import hashlib
import json
import mimetypes
import os
import re
import sys
import time
import urllib.error
import urllib.request
import uuid
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[1]
MANUAL = ROOT / "docs" / "manual" / "zh-CN"
CONFIG_PATH = MANUAL / "mdoc-manifest.json"
STATE_DIR = ROOT / ".local"
ARTICLE_LINK_RE = re.compile(
    r"\[(?P<title>[^\]]+)]\([^)]*/articles/(?P<id>\d+)/content\.md[^)]*\)"
)
IMAGE_RE = re.compile(r"!\[(?P<alt>[^\]]*)]\((?P<target>[^)]+)\)")
MD_LINK_RE = re.compile(r"(?<!!)\[(?P<label>[^\]]+)]\((?P<target>[^)]+\.md(?:#[^)]+)?)\)")


class MDocClient:
    def __init__(self, token: str) -> None:
        self.token = token

    def request(
        self,
        url: str,
        *,
        method: str = "GET",
        payload: dict[str, Any] | None = None,
        data: bytes | None = None,
        headers: dict[str, str] | None = None,
        expect_json: bool = True,
    ) -> Any:
        request_headers = {"Authorization": f"Bearer {self.token}"}
        if headers:
            request_headers.update(headers)
        if payload is not None:
            data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
            request_headers["Content-Type"] = "application/json"
        request = urllib.request.Request(
            url, data=data, method=method, headers=request_headers
        )
        last_error: Exception | None = None
        for attempt in range(4):
            try:
                with urllib.request.urlopen(request, timeout=90) as response:
                    body = response.read().decode("utf-8")
                return json.loads(body) if expect_json else body
            except urllib.error.HTTPError as error:
                detail = error.read().decode("utf-8", errors="replace")
                last_error = RuntimeError(f"M-Doc HTTP {error.code}: {detail}")
                if error.code not in {429, 500, 502, 503, 504}:
                    raise last_error
            except urllib.error.URLError as error:
                last_error = error
            time.sleep(2**attempt)
        raise RuntimeError(f"M-Doc request failed: {last_error}")

    def upload(self, path: Path) -> str:
        boundary = f"----mdoc-{uuid.uuid4().hex}"
        content_type = mimetypes.guess_type(path.name)[0] or "application/octet-stream"
        body = bytearray()
        body.extend(f"--{boundary}\r\n".encode())
        body.extend(
            (
                f'Content-Disposition: form-data; name="file"; filename="{path.name}"\r\n'
                f"Content-Type: {content_type}\r\n\r\n"
            ).encode()
        )
        body.extend(path.read_bytes())
        body.extend(f"\r\n--{boundary}--\r\n".encode())
        response = self.request(
            "https://mdoc.cc/openapi/upload",
            method="POST",
            data=bytes(body),
            headers={"Content-Type": f"multipart/form-data; boundary={boundary}"},
        )
        try:
            return str(response["data"]["url"])
        except (KeyError, TypeError) as error:
            raise RuntimeError(f"Unexpected upload response for {path}: {response}") from error


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--dry-run", action="store_true")
    parser.add_argument(
        "--only",
        action="append",
        default=[],
        metavar="ARTICLE_KEY",
        help="only update and publish the selected article key (repeatable)",
    )
    return parser.parse_args()


def load_json(path: Path, default: Any) -> Any:
    if not path.exists():
        return default
    return json.loads(path.read_text(encoding="utf-8"))


def save_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )


def extract_article_id(response: Any) -> int:
    data = response.get("data") if isinstance(response, dict) else None
    candidates = [data]
    if isinstance(data, dict):
        candidates.extend([data.get("id"), data.get("article_id"), data.get("article")])
    for candidate in candidates:
        if isinstance(candidate, int):
            return candidate
        if isinstance(candidate, str) and candidate.isdigit():
            return int(candidate)
        if isinstance(candidate, dict):
            value = candidate.get("id") or candidate.get("article_id")
            if isinstance(value, int) or (isinstance(value, str) and value.isdigit()):
                return int(value)
    raise RuntimeError(f"Cannot find created article id in response: {response}")


def backup_online(
    client: MDocClient, base: str, version: str, manifest: str
) -> Path:
    timestamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    destination = STATE_DIR / "mdoc-backups" / timestamp
    destination.mkdir(parents=True, exist_ok=False)
    (destination / "manifest.md").write_text(manifest, encoding="utf-8")
    article_ids = sorted({int(match.group("id")) for match in ARTICLE_LINK_RE.finditer(manifest)})
    for article_id in article_ids:
        content = client.request(
            f"{base}/articles/{article_id}/content.md?version={version}",
            expect_json=False,
        )
        (destination / f"article-{article_id}.md").write_text(content, encoding="utf-8")
    return destination


def collect_sources(config: dict[str, Any]) -> dict[Path, dict[str, Any]]:
    sources: dict[Path, dict[str, Any]] = {}
    for article in config["articles"]:
        source = (MANUAL / article["source"]).resolve()
        if not source.is_file():
            raise RuntimeError(f"Missing article source: {source}")
        if source in sources:
            raise RuntimeError(f"Duplicate article source: {source}")
        sources[source] = article
    all_markdown = {path.resolve() for path in MANUAL.rglob("*.md")}
    expected_unpublished = {(MANUAL / "README.md").resolve()}
    if set(sources) | expected_unpublished != all_markdown:
        missing = sorted(str(path.relative_to(MANUAL)) for path in all_markdown - set(sources) - expected_unpublished)
        raise RuntimeError(f"Markdown files missing from M-Doc manifest: {missing}")
    return sources


def image_paths(sources: dict[Path, dict[str, Any]]) -> set[Path]:
    result: set[Path] = set()
    for source in sources:
        text = source.read_text(encoding="utf-8")
        for match in IMAGE_RE.finditer(text):
            target = match.group("target").strip().split(maxsplit=1)[0].strip("<>")
            if target.startswith(("http://", "https://", "data:")):
                continue
            path = (source.parent / target).resolve()
            if not path.is_file():
                raise RuntimeError(f"Missing image referenced by {source}: {target}")
            result.add(path)
    return result


def render_online_content(
    source: Path,
    sources: dict[Path, dict[str, Any]],
    article_ids: dict[str, int],
    image_urls: dict[str, str],
    public_base: str,
) -> str:
    text = source.read_text(encoding="utf-8")

    def replace_image(match: re.Match[str]) -> str:
        target = match.group("target").strip().split(maxsplit=1)[0].strip("<>")
        if target.startswith(("http://", "https://", "data:")):
            return match.group(0)
        path = (source.parent / target).resolve()
        digest = hashlib.sha256(path.read_bytes()).hexdigest()
        return f"![{match.group('alt')}]({image_urls[digest]})"

    def replace_markdown_link(match: re.Match[str]) -> str:
        target = match.group("target")
        path_part, separator, anchor = target.partition("#")
        path = (source.parent / path_part).resolve()
        article = sources.get(path)
        if not article:
            raise RuntimeError(f"Unmapped Markdown link in {source}: {target}")
        url = f"{public_base}/{article_ids[article['key']]}"
        if separator:
            url += f"#{anchor}"
        return f"[{match.group('label')}]({url})"

    return MD_LINK_RE.sub(replace_markdown_link, IMAGE_RE.sub(replace_image, text))


def main() -> int:
    args = parse_args()
    config = load_json(CONFIG_PATH, None)
    if not config:
        raise RuntimeError(f"Missing config: {CONFIG_PATH}")
    sources = collect_sources(config)
    images = sorted(image_paths(sources))
    if len(images) != 39:
        raise RuntimeError(f"Expected 39 referenced screenshots, found {len(images)}")
    reused = [item for item in config["articles"] if item.get("article_id")]
    known_keys = {item["key"] for item in config["articles"]}
    unknown_keys = set(args.only) - known_keys
    if unknown_keys:
        raise RuntimeError(f"Unknown article keys: {sorted(unknown_keys)}")
    selected_articles = [
        item
        for item in config["articles"]
        if not args.only or item["key"] in set(args.only)
    ]
    print(
        f"Prepared {len(config['articles'])} articles and {len(images)} images; "
        f"{len(reused)} article IDs will be reused; "
        f"{len(selected_articles)} articles selected for publication."
    )
    if args.dry_run:
        return 0

    token = os.environ.get("MDOC_TOKEN")
    if not token:
        raise RuntimeError("MDOC_TOKEN is required")
    client = MDocClient(token)
    org = config["organization"]
    document = config["document"]
    version = config["version"]
    version_id = int(config["version_id"])
    base = f"https://mdoc.cc/openapi/organizations/{org}/documents/{document}"
    public_base = f"https://mdoc.cc/{org}/{document}/{version}"

    document_info = client.request(base)
    if int(document_info["data"]["default_version_id"]) != version_id:
        raise RuntimeError(
            f"Version mismatch: API default is {document_info['data']['default_version_id']}, "
            f"config is {version_id}"
        )
    manifest = client.request(f"{base}/manifest?version={version}", expect_json=False)
    backup = backup_online(client, base, version, manifest)
    print(f"Backed up online manifest and articles to {backup}")

    title_ids = {
        match.group("title"): int(match.group("id"))
        for match in ARTICLE_LINK_RE.finditer(manifest)
    }
    state_path = STATE_DIR / "mdoc-article-ids.json"
    state_ids = {key: int(value) for key, value in load_json(state_path, {}).items()}
    article_ids: dict[str, int] = {}
    for article in config["articles"]:
        article_id = article.get("article_id") or state_ids.get(article["key"]) or title_ids.get(article["title"])
        if article_id:
            article_ids[article["key"]] = int(article_id)

    for article in config["articles"]:
        key = article["key"]
        parent_key = article.get("parent")
        parent_id = article_ids[parent_key] if parent_key else None
        if key in article_ids:
            continue
        response = client.request(
            f"https://mdoc.cc/openapi/versions/{version_id}/articles",
            method="POST",
            payload={
                "title": article["title"],
                "content": f"# {article['title']}\n\n正在发布仓库手册内容。",
                "parent_article_id": parent_id,
                "sort_order": article["sort_order"],
            },
        )
        article_ids[key] = extract_article_id(response)
        state_ids[key] = article_ids[key]
        save_json(state_path, state_ids)
        print(f"Created [{article_ids[key]}] {article['title']}")

    upload_state_path = STATE_DIR / "mdoc-uploads.json"
    image_urls: dict[str, str] = load_json(upload_state_path, {})
    for index, image in enumerate(images, start=1):
        digest = hashlib.sha256(image.read_bytes()).hexdigest()
        if digest not in image_urls:
            image_urls[digest] = client.upload(image)
            save_json(upload_state_path, image_urls)
            print(f"Uploaded image {index}/{len(images)}: {image.relative_to(MANUAL)}")
    active_digests = {
        hashlib.sha256(image.read_bytes()).hexdigest() for image in images
    }
    image_urls = {
        digest: url
        for digest, url in image_urls.items()
        if digest in active_digests
    }
    save_json(upload_state_path, image_urls)

    for index, article in enumerate(selected_articles, start=1):
        article_id = article_ids[article["key"]]
        parent_key = article.get("parent")
        content = render_online_content(
            (MANUAL / article["source"]).resolve(),
            sources,
            article_ids,
            image_urls,
            public_base,
        )
        client.request(
            f"https://mdoc.cc/openapi/versions/{version_id}/articles/{article_id}",
            method="PUT",
            payload={
                "title": article["title"],
                "content": content,
                "parent_article_id": article_ids[parent_key] if parent_key else None,
                "sort_order": article["sort_order"],
            },
        )
        client.request(
            f"https://mdoc.cc/openapi/versions/{version_id}/articles/{article_id}/publish",
            method="POST",
            payload={"commit_message": "重整 v1.0.0 中文手册、SQLite 演示与页面截图"},
        )
        print(f"Published {index}/{len(selected_articles)}: [{article_id}] {article['title']}")

    final_manifest = client.request(
        f"{base}/manifest?version={version}", expect_json=False
    )
    (backup / "manifest-after.md").write_text(final_manifest, encoding="utf-8")
    final_ids = {int(match.group("id")) for match in ARTICLE_LINK_RE.finditer(final_manifest)}
    if set(article_ids.values()) - final_ids:
        raise RuntimeError("Final manifest is missing one or more published articles")
    stale = ("cmd/migrate", "X-App-Key", "Go 1.21", "/metrics", "/debug/pprof")
    for article_id in article_ids.values():
        content = client.request(
            f"{base}/articles/{article_id}/content.md?version={version}",
            expect_json=False,
        )
        if any(value in content for value in stale):
            raise RuntimeError(f"Published article {article_id} still contains stale text")
        if "assets/screenshots/" in content:
            raise RuntimeError(f"Published article {article_id} still contains a local image path")
    print(f"Verified final manifest: {len(final_ids)} published articles")
    print(public_base)
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as error:
        print(f"publish_mdoc.py: {error}", file=sys.stderr)
        raise SystemExit(1)
