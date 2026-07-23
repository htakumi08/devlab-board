#!/usr/bin/env bash

set -euo pipefail

usage() {
  echo "Usage: $0 <keyword> [repo_dir]" >&2
}

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
  usage
  exit 1
fi

keyword="$1"
repo_dir="${2:-$(pwd)}"

if [ ! -d "$repo_dir" ]; then
  echo "Directory not found: $repo_dir" >&2
  exit 1
fi

if rg -n -F --hidden \
  --glob '!**/.git/**' \
  --glob '!**/node_modules/**' \
  --glob '!**/dist/**' \
  --glob '!**/build/**' \
  --glob '!**/.terraform/**' \
  --glob '!**/*.tfstate' \
  --glob '!**/*.tfstate.*' \
  -- "$keyword" "$repo_dir"; then
  exit 0
else
  search_status=$?
fi

if [ "$search_status" -eq 1 ]; then
  echo "No related usages found: $keyword"
  exit 0
fi

exit "$search_status"
