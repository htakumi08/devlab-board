#!/usr/bin/env bash

set -euo pipefail

usage() {
  echo "Usage: $0 <base_ref> <review_ref> [repo_dir]" >&2
}

if [ "$#" -lt 2 ] || [ "$#" -gt 3 ]; then
  usage
  exit 1
fi

base_ref="$1"
review_ref="$2"
repo_dir="${3:-$(pwd)}"

if ! git -C "$repo_dir" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "Git repository not found: $repo_dir" >&2
  exit 1
fi

git -C "$repo_dir" rev-parse --verify "${base_ref}^{commit}" >/dev/null
git -C "$repo_dir" rev-parse --verify "${review_ref}^{commit}" >/dev/null

merge_base="$(git -C "$repo_dir" merge-base "$base_ref" "$review_ref")"

echo "== Repository =="
echo "$repo_dir"
echo

echo "== Base Ref =="
echo "$base_ref"
echo

echo "== Review Ref =="
echo "$review_ref"
echo

echo "== Merge Base =="
echo "$merge_base"
echo

echo "== Commit Range (${base_ref}...${review_ref}) =="
git -C "$repo_dir" log --oneline --decorate --left-right "${base_ref}...${review_ref}"
echo

echo "== Diff Stat =="
git -C "$repo_dir" diff --stat --find-renames "${base_ref}...${review_ref}"
echo

echo "== Changed Files =="
git -C "$repo_dir" diff --name-status --find-renames "${base_ref}...${review_ref}"
echo

echo "== Diff =="
git -C "$repo_dir" diff --find-renames "${base_ref}...${review_ref}"
