#!/usr/bin/env bash
set -euo pipefail
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
checkout=${EXACT:-"$root/.runtime/exact"}
patch="$root/patches/exact-linux-x11.patch"
if git -C "$checkout" apply --reverse --check "$patch" 2>/dev/null; then
  exit 0
fi
git -C "$checkout" apply --check "$patch"
git -C "$checkout" apply "$patch"
