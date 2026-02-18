from __future__ import annotations

import json
import os
import re
from datetime import datetime, timezone
from typing import Any

import mkdocs_gen_files
import requests

OWNER = "strantalis"
REPO = "workset"
API_URL = f"https://api.github.com/repos/{OWNER}/{REPO}/releases"
USER_AGENT = "workset-docs-updater-manifests"
CHANNELS = ("stable", "alpha")
SHA256_RE = re.compile(r"\b([a-fA-F0-9]{64})\b")
TEAM_ID_RE = re.compile(r"\b([A-Z0-9]{10})\b")


def _now_rfc3339() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def _base_headers() -> dict[str, str]:
    headers = {
        "Accept": "application/vnd.github+json",
        "User-Agent": USER_AGENT,
    }
    token = os.getenv("GITHUB_TOKEN") or os.getenv("GH_TOKEN")
    if token:
        headers["Authorization"] = f"Bearer {token}"
    return headers


def _fetch_releases() -> tuple[list[dict[str, Any]], str | None]:
    try:
        response = requests.get(
            API_URL,
            headers=_base_headers(),
            timeout=20,
            params={"per_page": 100},
        )
        response.raise_for_status()
        data = response.json()
    except Exception as exc:  # pragma: no cover - best-effort docs generation
        return [], str(exc)
    return [r for r in data if not r.get("draft")], None


def _select_release(releases: list[dict[str, Any]], channel: str) -> dict[str, Any] | None:
    prerelease = channel == "alpha"
    for release in releases:
        if bool(release.get("prerelease")) == prerelease:
            return release
    return None


def _fetch_text(url: str) -> tuple[str, str | None]:
    try:
        response = requests.get(url, headers=_base_headers(), timeout=30)
        response.raise_for_status()
        return response.text, None
    except Exception as exc:  # pragma: no cover - best-effort docs generation
        return "", str(exc)


def _default_latest(version: str, notes_url: str, asset_url: str, generated_at: str) -> dict[str, Any]:
    return {
        "version": version,
        "pubDate": generated_at,
        "notesUrl": notes_url,
        "minimumVersion": "",
        "asset": {
            "name": "disabled",
            "url": asset_url,
            "sha256": "disabled",
        },
        "signing": {
            "teamId": "DISABLED",
        },
    }


def _release_tag(release: dict[str, Any] | None) -> str:
    if not release:
        return "v0.0.0"
    tag = str(release.get("tag_name") or "").strip()
    if not tag:
        return "v0.0.0"
    if not tag.startswith("v"):
        return f"v{tag}"
    return tag


def _build_manifest(
    channel: str,
    release: dict[str, Any] | None,
    error: str | None,
) -> dict[str, Any]:
    generated_at = _now_rfc3339()
    notes_url = str((release or {}).get("html_url") or "https://workset.dev/releases/")
    default = {
        "schemaVersion": 1,
        "generatedAt": generated_at,
        "channel": channel,
        "disabled": True,
        "message": "",
        "latest": _default_latest(_release_tag(release), notes_url, notes_url, generated_at),
    }

    if error:
        default["message"] = f"Update manifest generation failed: {error}"
        return default

    if not release:
        default["message"] = f"No {channel} release available yet."
        return default

    assets = release.get("assets") or []
    assets_by_name = {
        str(asset.get("name") or ""): asset for asset in assets if asset.get("name")
    }

    tag = _release_tag(release)
    default["latest"]["version"] = tag
    default["latest"]["pubDate"] = str(release.get("published_at") or generated_at)
    default["latest"]["notesUrl"] = notes_url

    expected_zip_name = f"workset-{tag}-macos-update.zip"
    update_asset = assets_by_name.get(expected_zip_name)
    if not update_asset:
        default["message"] = (
            f"Latest {channel} release {tag} does not include a macOS auto-update package yet."
        )
        return default

    update_asset_url = str(update_asset.get("browser_download_url") or "").strip()
    if not update_asset_url:
        default["message"] = f"Latest {channel} release {tag} has an invalid update asset URL."
        return default

    sha_asset = assets_by_name.get(f"{expected_zip_name}.sha256")
    team_asset = assets_by_name.get(f"{expected_zip_name}.teamid")
    if not sha_asset or not team_asset:
        default["latest"]["asset"]["name"] = expected_zip_name
        default["latest"]["asset"]["url"] = update_asset_url
        default["message"] = (
            f"Latest {channel} release {tag} is missing updater metadata assets (.sha256/.teamid)."
        )
        return default

    sha_text, sha_error = _fetch_text(str(sha_asset.get("browser_download_url") or ""))
    team_text, team_error = _fetch_text(str(team_asset.get("browser_download_url") or ""))
    if sha_error or team_error:
        default["latest"]["asset"]["name"] = expected_zip_name
        default["latest"]["asset"]["url"] = update_asset_url
        default["message"] = (
            f"Failed to load updater metadata for {tag}: "
            f"sha256={sha_error or 'ok'}, teamid={team_error or 'ok'}."
        )
        return default

    sha_match = SHA256_RE.search(sha_text)
    team_match = TEAM_ID_RE.search(team_text)
    if not sha_match or not team_match:
        default["latest"]["asset"]["name"] = expected_zip_name
        default["latest"]["asset"]["url"] = update_asset_url
        default["message"] = f"Updater metadata for {tag} is invalid."
        return default

    default["disabled"] = False
    default["message"] = ""
    default["latest"]["asset"]["name"] = expected_zip_name
    default["latest"]["asset"]["url"] = update_asset_url
    default["latest"]["asset"]["sha256"] = sha_match.group(1).lower()
    default["latest"]["signing"]["teamId"] = team_match.group(1)
    return default


releases, release_error = _fetch_releases()

for channel in CHANNELS:
    release = _select_release(releases, channel)
    manifest = _build_manifest(channel, release, release_error)
    with mkdocs_gen_files.open(f"updates/{channel}.json", "w") as fd:
        json.dump(manifest, fd, indent=2)
        fd.write("\n")
