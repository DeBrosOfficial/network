#!/usr/bin/env bash
# Installs a prebuilt release binary of a cargo subcommand into cargo's bin directory, after checking
# the archive against the sha256 pinned in chain/ci.mk. Building cargo-audit and cargo-deny from
# source took 25 minutes of every CI run; the upstream release archives are the same code, built by
# the projects' own release workflows.
#
#   release-tool.sh NAME URL SHA256
#
# The archive is a .tar.gz with one top-level directory holding the NAME binary. An empty SHA256
# means chain/ci.mk pins no build of NAME for this platform: add one there.
set -euo pipefail

if [ $# -ne 3 ]; then
  echo "usage: release-tool.sh NAME URL SHA256" >&2
  exit 2
fi
name="$1"
url="$2"
sum="$3"

if [ -z "$sum" ]; then
  echo "release-tool.sh: chain/ci.mk pins no $name build for this platform" >&2
  exit 1
fi

dir="$(mktemp -d)"
trap 'rm -rf "$dir"' EXIT

curl -fsSL -o "$dir/archive.tar.gz" "$url"
echo "$sum  $dir/archive.tar.gz" | shasum -a 256 -c - >/dev/null
mkdir "$dir/x"
tar -xzf "$dir/archive.tar.gz" -C "$dir/x" --strip-components=1

bin="${CARGO_HOME:-$HOME/.cargo}/bin"
mkdir -p "$bin"
install -m 0755 "$dir/x/$name" "$bin/$name"
echo "installed $name from $url"
