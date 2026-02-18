#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <base-sha>" >&2
  exit 2
fi

base_sha="$1"
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
frontend_root="$repo_root/wails-ui/workset/frontend"

changed="$(git -C "$repo_root" diff --name-only --diff-filter=ACMRT "$base_sha"...HEAD)"
filtered="$(
  printf '%s\n' "$changed" \
    | grep -E '^wails-ui/workset/frontend/.*\.(ts|svelte)$' \
    | grep -Ev '^wails-ui/workset/frontend/(dist|node_modules|wailsjs)/' || true
)"

if [[ -z "$filtered" ]]; then
  echo "frontend-complexity: no changed frontend .ts/.svelte files"
  exit 0
fi

rel_files=()
while IFS= read -r file; do
  [[ -z "$file" ]] && continue
  rel_files+=("${file#wails-ui/workset/frontend/}")
done <<< "$filtered"

allowlist_file="$repo_root/.github/frontend-complexity-allowlist.txt"
filtered_files=()
allowlisted_count=0
allowlist_tmp=""

if [[ -f "$allowlist_file" ]]; then
  allowlist_tmp="$(mktemp)"
  trap 'rm -f "$allowlist_tmp"' EXIT
  while IFS= read -r raw; do
    entry="${raw%%#*}"
    entry="${entry#"${entry%%[![:space:]]*}"}"
    entry="${entry%"${entry##*[![:space:]]}"}"
    [[ -z "$entry" ]] && continue
    printf '%s\n' "$entry" >> "$allowlist_tmp"
  done < "$allowlist_file"

  for file in "${rel_files[@]}"; do
    if grep -Fxq "$file" "$allowlist_tmp"; then
      allowlisted_count=$((allowlisted_count + 1))
      continue
    fi
    filtered_files+=("$file")
  done
else
  filtered_files=("${rel_files[@]}")
fi

if (( allowlisted_count > 0 )); then
  echo "frontend-complexity: skipped $allowlisted_count allowlisted file(s)"
fi

if [[ ${#filtered_files[@]} -eq 0 ]]; then
  echo "frontend-complexity: no non-allowlisted changed frontend .ts/.svelte files"
  exit 0
fi

echo "frontend-complexity: checking ${#filtered_files[@]} file(s)"
(
  cd "$frontend_root"
  npx eslint \
    --rule 'complexity:["error",15]' \
    --rule 'max-lines:["error",{"max":1000,"skipBlankLines":true,"skipComments":true}]' \
    "${filtered_files[@]}"
)
