#!/bin/sh
set -eu
cd "$(dirname "$0")/../apps/api"
go test -count=1 -run TestVersionMatchesPackageJSON -ldflags "-X main.version=$(sh ../../scripts/version.sh)" .
