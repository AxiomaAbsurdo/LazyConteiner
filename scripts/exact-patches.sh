#!/usr/bin/env bash
set -euo pipefail
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
checkout=${EXACT:-"$root/.runtime/exact"}
for name in exact-linux-x11 exact-linux-scrollbars; do
  patch="$root/patches/$name.patch"
  if git -C "$checkout" apply --reverse --check "$patch" 2>/dev/null; then
    continue
  fi
  git -C "$checkout" apply --check "$patch"
  git -C "$checkout" apply "$patch"
done
