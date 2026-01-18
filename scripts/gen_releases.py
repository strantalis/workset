from __future__ import annotations

import os
from datetime import datetime

import mkdocs_gen_files
import requests

OWNER = "strantalis"
REPO = "workset"
API_URL = f"https://api.github.com/repos/{OWNER}/{REPO}/releases"
MAX_RELEASES = int(os.getenv("WORKSET_RELEASES_LIMIT", "10"))


def _format_date(value: str | None) -> str | None:
    if not value:
        return None
    try:
        return datetime.fromisoformat(value.replace("Z", "+00:00")).date().isoformat()
    except ValueError:
        return None


def _fetch_releases() -> tuple[list[dict], str | None]:
    headers = {
        "Accept": "application/vnd.github+json",
        "User-Agent": "workset-docs",
    }
    token = os.getenv("GITHUB_TOKEN") or os.getenv("GH_TOKEN")
    if token:
        headers["Authorization"] = f"Bearer {token}"

    try:
        response = requests.get(API_URL, headers=headers, timeout=15)
        response.raise_for_status()
        data = response.json()
    except Exception as exc:  # pragma: no cover - best-effort docs generation
        return [], str(exc)

    releases = [r for r in data if not r.get("draft") and not r.get("prerelease")]
    return releases[:MAX_RELEASES], None


with mkdocs_gen_files.open("releases/index.md", "w") as fd:
    fd.write("---\n")
    fd.write("description: Workset release notes pulled from GitHub.\n")
    fd.write("---\n\n")
    fd.write("# Releases\n\n")
    fd.write("Latest release notes from GitHub.\n\n")

    releases, error = _fetch_releases()
    if error:
        fd.write("!!! warning\n")
        fd.write("    Failed to fetch releases from GitHub.\n\n")
        fd.write(f"    Error: `{error}`\n\n")
        fd.write(
            "    If you're running locally, set `GITHUB_TOKEN` to increase rate limits.\n"
        )
        fd.write("\n")
    elif not releases:
        fd.write("_No releases found._\n")
    else:
        for release in releases:
            tag = release.get("tag_name") or "(untagged)"
            name = release.get("name") or tag
            url = release.get("html_url")
            published = _format_date(release.get("published_at") or release.get("created_at"))

            fd.write(f"## {name}\n\n")
            meta = []
            if published:
                meta.append(f"Released {published}")
            if url:
                meta.append(f"[View on GitHub]({url})")
            if meta:
                fd.write(" ".join(meta) + "\n\n")

            body = (release.get("body") or "").strip()
            fd.write(body if body else "_No release notes provided._")
            fd.write("\n\n")
