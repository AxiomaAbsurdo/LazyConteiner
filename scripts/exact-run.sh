#!/usr/bin/env bash
set -euo pipefail
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
export EXACT=${EXACT:-"$root/.runtime/exact"}
if [[ ! -f "$EXACT/scripts/exact.mjs" ]]; then
  echo 'Run ./scripts/exact-setup.sh first, or set EXACT to the pinned checkout.' >&2
  exit 1
fi
bash "$root/scripts/exact-patches.sh"
cd -- "$root/fleet"
mode=${1:-run}
if (($#)); then shift; fi
case "$mode" in
  check) exec bun exact.mjs contract build app.contract --json --errors-only "$@" ;;
  build) exec bun exact.mjs linux "$@" ;;
  test) if [[ ${EXACT_SKIP_BUILD:-0} != 1 ]]; then bun exact.mjs linux; fi; exec bun exact.mjs test linux "$@" ;;
  agent) if [[ ${EXACT_SKIP_BUILD:-0} != 1 ]]; then bun exact.mjs linux; fi; exec bun exact.mjs agent linux "$@" ;;
  web) exec bun exact.mjs web "$@" ;;
  run)
    if [[ ${EXACT_SKIP_BUILD:-0} != 1 ]]; then bun exact.mjs linux; fi
    export EXACT_SIZE=${EXACT_SIZE:-1440x900}
    binary=$(bun -e 'const {resolveApp,linuxBinary}=await import(process.env.EXACT + "/scripts/app.mjs"); console.log(linuxBinary(resolveApp(process.cwd())));')
    if [[ -n ${WAYLAND_DISPLAY:-}${WAYLAND_SOCKET:-}${DISPLAY:-} ]]; then
      exec "$binary" "$@"
    else
      echo 'Native GUI needs an X11 or Wayland desktop session.' >&2
      echo 'Use ./scripts/exact-run.sh agent "tree" for headless verification.' >&2
      exit 1
    fi
    ;;
  *) echo "Usage: $0 [run|build|test|agent|web|check] [arguments]" >&2; exit 2 ;;
esac
