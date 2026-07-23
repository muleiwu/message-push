#!/usr/bin/env python3
"""Read back the published M-Doc tree and uploaded screenshots."""

from __future__ import annotations

import html
import json
import os
import re
import sys
import urllib.request
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

from publish_mdoc import ARTICLE_LINK_RE, IMAGE_RE, MDocClient, ROOT


CONFIG_PATH = ROOT / "docs" / "manual" / "zh-CN" / "mdoc-manifest.json"
STATE_DIR = ROOT / ".local"


def load(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def check_image(url: str) -> None:
    request = urllib.request.Request(url, method="HEAD")
    with urllib.request.urlopen(request, timeout=30) as response:
        if response.status != 200:
            raise RuntimeError(f"image returned HTTP {response.status}: {url}")


def main() -> int:
    token = os.environ.get("MDOC_TOKEN")
    if not token:
        raise RuntimeError("MDOC_TOKEN is required")
    config = load(CONFIG_PATH)
    state_ids = {key: int(value) for key, value in load(STATE_DIR / "mdoc-article-ids.json").items()}
    article_ids = {
        item["key"]: int(item.get("article_id") or state_ids[item["key"]])
        for item in config["articles"]
    }
    client = MDocClient(token)
    version_id = int(config["version_id"])
    base = (
        f"https://mdoc.cc/openapi/organizations/{config['organization']}"
        f"/documents/{config['document']}"
    )

    manifest = client.request(
        f"{base}/manifest?version={config['version']}", expect_json=False
    )
    manifest_ids = {
        int(match.group("id")) for match in ARTICLE_LINK_RE.finditer(manifest)
    }
    if manifest_ids != set(article_ids.values()):
        raise RuntimeError("M-Doc manifest article IDs do not match the repository manifest")

    for item in config["articles"]:
        article_id = article_ids[item["key"]]
        detail = client.request(
            f"https://mdoc.cc/openapi/versions/{version_id}/articles/{article_id}"
        )["data"]
        parent_key = item.get("parent")
        expected_parent = article_ids[parent_key] if parent_key else 0
        actual_parent = detail.get("parent_article_id") or 0
        if detail["article"]["title"] != item["title"]:
            raise RuntimeError(f"title mismatch for article {article_id}")
        if actual_parent != expected_parent:
            raise RuntimeError(f"parent mismatch for article {article_id}")
        if int(detail["sort_order"]) != int(item["sort_order"]):
            raise RuntimeError(f"sort order mismatch for article {article_id}")
        if detail["current_version"]["status"] != "published":
            raise RuntimeError(f"article {article_id} is not published")

    public_url = (
        f"https://mdoc.cc/{config['organization']}/{config['document']}"
        f"/{config['version']}"
    )
    public_html = client.request(public_url, expect_json=False)
    tree_pattern = re.compile(
        rf'<a href="/{config["organization"]}/{config["document"]}/'
        rf'{re.escape(config["version"])}/(\d+)" class="toc-overview-item-content".*?'
        r'<span class="toc-overview-item-title"[^>]*>(.*?)</span>',
        re.DOTALL,
    )
    public_order = [int(article_id) for article_id, _ in tree_pattern.findall(public_html)]
    expected_order = [article_ids[item["key"]] for item in config["articles"]]
    if public_order != expected_order:
        raise RuntimeError("public M-Doc table-of-contents order does not match")
    public_titles = [
        html.unescape(re.sub(r"<[^>]+>", "", title)).strip()
        for _, title in tree_pattern.findall(public_html)
    ]
    if public_titles != [item["title"] for item in config["articles"]]:
        raise RuntimeError("public M-Doc table-of-contents titles do not match")

    image_urls = list(load(STATE_DIR / "mdoc-uploads.json").values())
    if len(image_urls) != 39:
        raise RuntimeError(f"expected 39 uploaded images, found {len(image_urls)}")
    published_image_urls: set[str] = set()
    for article_id in article_ids.values():
        content = client.request(
            f"{base}/articles/{article_id}/content.md?version={config['version']}",
            expect_json=False,
        )
        for match in IMAGE_RE.finditer(content):
            target = match.group("target").strip().split(maxsplit=1)[0].strip("<>")
            if target.startswith(("http://", "https://")):
                published_image_urls.add(target)
    if published_image_urls != set(image_urls):
        missing = sorted(set(image_urls) - published_image_urls)
        unexpected = sorted(published_image_urls - set(image_urls))
        raise RuntimeError(
            "published image references do not match repository upload state: "
            f"missing={len(missing)}, unexpected={len(unexpected)}"
        )
    with ThreadPoolExecutor(max_workers=8) as executor:
        list(executor.map(check_image, image_urls))

    print(
        f"M-Doc verification passed: {len(expected_order)} articles in order, "
        f"{len(image_urls)} images load successfully."
    )
    print(public_url)
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as error:
        print(f"verify_mdoc.py: {error}", file=sys.stderr)
        raise SystemExit(1)
