#!/usr/bin/env sh
#
# Fetches the two Vosk models the in-browser transcription runs (the same
# vosk-model-small-fr-0.22 family the old server transcriber used) and repacks
# them as the tar.gz that vosk-browser loads, one top-level folder each.
# Pinned by sha256: a changed archive aborts the build.
#
#   sh scripts/vosk-models.sh apps/client/static/models
#
# Models: https://alphacephei.com/vosk/models (Apache 2.0)

set -eu

out="${1:?usage: $0 <output dir>}"
base="https://alphacephei.com/vosk/models"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mkdir -p "$out"

fetch() {
  lang="$1"
  model="$2"
  sum="$3"
  curl -sSfL -o "$work/$model.zip" "$base/$model.zip"
  echo "$sum  $work/$model.zip" | sha256sum -c - >/dev/null
  unzip -q "$work/$model.zip" -d "$work"
  tar -czf "$out/$lang.tar.gz" -C "$work" "$model"
}

fetch fr vosk-model-small-fr-0.22 cabf6180e177eb9b3a9a9d43a437bd5e549f3a7d09525e5d69a3fed787be12ad
fetch en vosk-model-small-en-us-0.15 30f26242c4eb449f948e42cb302dd7a686cb29a3423a8367f99ff41780942498
