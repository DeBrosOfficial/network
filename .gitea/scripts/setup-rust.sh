#!/usr/bin/env bash
# Installs the chain's pinned Rust on a Gitea Actions runner. The runner image
# (docker.gitea.com/runner-images:ubuntu-latest) ships no rustup, and may lack
# make and rsync, which the chain's builds need.
#
#   setup-rust.sh [--nightly] [--component NAME]... [--target TRIPLE]...
#
# The default toolchain is RUST_TOOLCHAIN from chain/ci.mk. --nightly also
# installs RUST_NIGHTLY, and the components and targets then go to it instead.
# ~/.cargo/bin is added to GITHUB_PATH for the job's later steps, and the
# step's output `toolchain` is RUST_TOOLCHAIN (the workflows key their cargo
# caches on it).
set -euo pipefail

nightly=false
components=()
targets=()
while [ $# -gt 0 ]; do
  case "$1" in
    --nightly) nightly=true ;;
    --component) components+=("$2"); shift ;;
    --target) targets+=("$2"); shift ;;
    *) echo "setup-rust.sh: unknown argument $1" >&2; exit 2 ;;
  esac
  shift
done

missing=()
for tool in make rsync curl; do
  command -v "$tool" >/dev/null || missing+=("$tool")
done
if [ ${#missing[@]} -gt 0 ]; then
  sudo=""
  [ "$(id -u)" -eq 0 ] || sudo="sudo"
  $sudo apt-get update -qq
  $sudo apt-get install -y -qq "${missing[@]}"
fi

repo="$(git rev-parse --show-toplevel)"
stable="$(make -s -C "$repo/chain" print-RUST_TOOLCHAIN)"

export PATH="$HOME/.cargo/bin:$PATH"
if ! command -v rustup >/dev/null; then
  curl -fsSL https://sh.rustup.rs | sh -s -- -y --no-modify-path --profile minimal --default-toolchain "$stable"
fi
rustup toolchain install "$stable" --profile minimal
rustup default "$stable"

toolchain="$stable"
if $nightly; then
  toolchain="$(make -s -C "$repo/chain" print-RUST_NIGHTLY)"
  rustup toolchain install "$toolchain" --profile minimal
fi
if [ ${#components[@]} -gt 0 ]; then
  rustup component add --toolchain "$toolchain" "${components[@]}"
fi
if [ ${#targets[@]} -gt 0 ]; then
  rustup target add --toolchain "$toolchain" "${targets[@]}"
fi

echo "$HOME/.cargo/bin" >> "$GITHUB_PATH"
echo "toolchain=$stable" >> "$GITHUB_OUTPUT"
