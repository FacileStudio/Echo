#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
printf 'v%s\n' "$(sed -n 's/^[[:space:]]*"version":[[:space:]]*"\([^"]*\)".*/\1/p' apps/client/package.json | head -n 1)"
