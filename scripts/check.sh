#!/usr/bin/env sh
#
# The repository quality gate. Reports, never rewrites (except --format).
#
#   sh scripts/check.sh             gofmt + vet + test on every Go module, then the client
#   sh scripts/check.sh --go-only   Go modules only
#   sh scripts/check.sh --format    rewrite Go sources in place
#
# Deliberately depends on nothing but a `go` on PATH and `bun` for the client.
# It is NOT invoked through mise: `mise run` resolves every tool in the merged
# config before running any task body, so an unrelated broken tool in the
# user's global config would take this gate down with it.

set -eu

GO_MODULES="apps/api"
CLIENT_DIR="apps/client"

mode="all"
case "${1:-}" in
--go-only) mode="go" ;;
--format) mode="format" ;;
"") ;;
*)
  echo "usage: $0 [--go-only|--format]" >&2
  exit 2
  ;;
esac

cd "$(git rev-parse --show-toplevel)"

if [ -z "${GO:-}" ]; then
  if [ -n "${GOROOT:-}" ] && [ -x "$GOROOT/bin/go" ]; then GO="$GOROOT/bin/go"; else GO=go; fi
fi
if [ -z "${GOFMT:-}" ]; then
  if [ -n "${GOROOT:-}" ] && [ -x "$GOROOT/bin/gofmt" ]; then GOFMT="$GOROOT/bin/gofmt"; else GOFMT=gofmt; fi
fi

status=0

for module in $GO_MODULES; do
  echo "== $module =="
  if [ "$mode" = "format" ]; then
    (cd "$module" && "$GOFMT" -w .)
    continue
  fi
  unformatted="$(cd "$module" && "$GOFMT" -l .)" || status=1
  if [ -n "$unformatted" ]; then
    echo "gofmt needed on:" >&2
    echo "$unformatted" >&2
    status=1
  fi
  if ! (cd "$module" && GOFLAGS="-mod=readonly" "$GO" vet ./...); then
    status=1
  fi
  if ! (cd "$module" && GOFLAGS="-mod=readonly" "$GO" test ./...); then
    status=1
  fi
done

if [ "$mode" != "go" ] && [ "$mode" != "format" ]; then
  if [ -d "$CLIENT_DIR" ]; then
    echo "== $CLIENT_DIR =="
    if [ ! -d "$CLIENT_DIR/node_modules" ]; then
      echo "node_modules missing — run: bun install in $CLIENT_DIR" >&2
      exit 1
    fi
    if ! (cd "$CLIENT_DIR" && bun run check); then
      status=1
    fi
  fi
fi

exit "$status"
