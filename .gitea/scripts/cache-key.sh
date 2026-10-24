#!/usr/bin/env bash
# Writes NAME=<hash of FILES> to the step's outputs, for an actions/cache key.
# act_runner evaluates hashFiles() on the runner host, where the checkout does
# not exist (the job runs in a container), so hashFiles() returns an empty
# string there and every key built on it would never change. The files are
# hashed here, inside the job, instead.
#
#   cache-key.sh NAME FILE...
#
# A missing file is an error, not an empty hash.
set -euo pipefail

if [ $# -lt 2 ]; then
  echo "usage: cache-key.sh NAME FILE..." >&2
  exit 2
fi
name="$1"
shift

hash="$(sha256sum -- "$@" | sha256sum | cut -c1-32)"
echo "$name=$hash" >> "$GITHUB_OUTPUT"
echo "$name=$hash ($*)"
