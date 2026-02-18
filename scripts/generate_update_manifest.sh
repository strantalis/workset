#!/usr/bin/env bash
set -euo pipefail

channel=""
version=""
asset_url=""
sha256=""
team_id=""
notes_url=""
minimum_version=""
output=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --channel)
      channel="$2"
      shift 2
      ;;
    --version)
      version="$2"
      shift 2
      ;;
    --asset-url)
      asset_url="$2"
      shift 2
      ;;
    --sha256)
      sha256="$2"
      shift 2
      ;;
    --team-id)
      team_id="$2"
      shift 2
      ;;
    --notes-url)
      notes_url="$2"
      shift 2
      ;;
    --minimum-version)
      minimum_version="$2"
      shift 2
      ;;
    --output)
      output="$2"
      shift 2
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

if [[ -z "$channel" || -z "$version" || -z "$asset_url" || -z "$sha256" || -z "$team_id" || -z "$output" ]]; then
  cat >&2 <<'EOF'
usage:
  scripts/generate_update_manifest.sh \
    --channel stable|alpha \
    --version 0.3.0 \
    --asset-url https://.../workset-v0.3.0-macos-update.zip \
    --sha256 <hex> \
    --team-id <APPLE_TEAM_ID> \
    [--notes-url https://.../releases/tag/v0.3.0] \
    [--minimum-version v0.2.0] \
    --output docs/updates/stable.json
EOF
  exit 1
fi

if [[ "$channel" != "stable" && "$channel" != "alpha" ]]; then
  echo "--channel must be stable or alpha" >&2
  exit 1
fi

if [[ "$version" != v* ]]; then
  version="v$version"
fi

generated_at="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
asset_name="$(basename "$asset_url")"
minimum_version_json='""'
if [[ -n "$minimum_version" ]]; then
  if [[ "$minimum_version" != v* ]]; then
    minimum_version="v$minimum_version"
  fi
  minimum_version_json="\"$minimum_version\""
fi

notes_url_json='""'
if [[ -n "$notes_url" ]]; then
  notes_url_json="\"$notes_url\""
fi

mkdir -p "$(dirname "$output")"
cat >"$output" <<EOF
{
  "schemaVersion": 1,
  "generatedAt": "$generated_at",
  "channel": "$channel",
  "disabled": false,
  "message": "",
  "latest": {
    "version": "$version",
    "pubDate": "$generated_at",
    "notesUrl": $notes_url_json,
    "minimumVersion": $minimum_version_json,
    "asset": {
      "name": "$asset_name",
      "url": "$asset_url",
      "sha256": "$sha256"
    },
    "signing": {
      "teamId": "$team_id"
    }
  }
}
EOF

echo "wrote $output"
