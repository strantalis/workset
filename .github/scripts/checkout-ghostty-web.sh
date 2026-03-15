#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
target_dir="${repo_root}/../ghostty-web"
ghostty_repo="${GHOSTTY_WEB_REPOSITORY:-strantalis/ghostty-web}"
ghostty_fallback_ref="${GHOSTTY_WEB_REF:-main}"
ghostty_token="${GHOSTTY_WEB_CHECKOUT_TOKEN:-}"
current_ref="${GITHUB_HEAD_REF:-${GITHUB_REF_NAME:-}}"

if [[ -e "${target_dir}" ]]; then
  echo "ghostty-web checkout already present at ${target_dir}"
  exit 0
fi

remote_url="https://github.com/${ghostty_repo}.git"
if [[ -n "${ghostty_token}" ]]; then
  remote_url="https://x-access-token:${ghostty_token}@github.com/${ghostty_repo}.git"
fi

ghostty_ref="${ghostty_fallback_ref}"
if [[ -n "${current_ref}" ]] && git ls-remote --exit-code --heads "${remote_url}" "${current_ref}" >/dev/null 2>&1; then
  ghostty_ref="${current_ref}"
elif [[ -n "${current_ref}" ]]; then
  echo "ghostty-web branch ${current_ref} not found in ${ghostty_repo}; falling back to ${ghostty_fallback_ref}"
fi

echo "Checking out ${ghostty_repo}@${ghostty_ref} into ${target_dir}"
git clone --depth 1 --branch "${ghostty_ref}" "${remote_url}" "${target_dir}"
