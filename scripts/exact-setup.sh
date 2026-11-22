#!/usr/bin/env bash
set -euo pipefail
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
revision=d7b6871218430ed57c5a0fbf34860b28f523d5df
checkout=${EXACT:-"$root/.runtime/exact"}

command -v bun >/dev/null || { echo 'Install Bun 1.4.2 or newer first.' >&2; exit 1; }
command -v rustup >/dev/null || { echo 'Install Rust through rustup first.' >&2; exit 1; }
if [[ ! -d "$checkout/.git" ]]; then
  mkdir -p -- "$(dirname -- "$checkout")"
  git clone https://github.com/expo/exact.git "$checkout"
  git -C "$checkout" checkout --detach "$revision"
fi
if [[ $(git -C "$checkout" rev-parse HEAD) != "$revision" ]]; then
  echo "Exact checkout must be pinned to $revision: $checkout" >&2
  exit 1
fi

EXACT="$checkout" bash "$root/scripts/exact-patches.sh"
cd -- "$checkout"
bun install --frozen-lockfile
# Native Linux needs neither the game SDK nor Android/Apple cross toolchains.
rustup toolchain install 1.99.1 --profile minimal --component rustfmt --component clippy --target wasm32-unknown-unknown
cargo run --locked --manifest-path vendor/ibex/crates/hermes-lean-sys-installer/Cargo.toml --
cargo fetch --locked --manifest-path Cargo.toml
cd -- "$root/fleet"
EXACT="$checkout" bun exact.mjs --help >/dev/null
bun install
printf 'Exact ready at %s\nRun: EXACT=%q ./scripts/exact-run.sh build\n' "$checkout" "$checkout"
