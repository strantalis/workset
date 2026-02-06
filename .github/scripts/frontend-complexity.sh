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

echo "frontend-complexity: checking ${#rel_files[@]} file(s)"
(
  cd "$frontend_root"
  npx eslint \
    --rule 'complexity:["error",15]' \
    --rule 'max-lines:["error",{"max":1000,"skipBlankLines":true,"skipComments":true}]' \
    "${rel_files[@]}"
)
