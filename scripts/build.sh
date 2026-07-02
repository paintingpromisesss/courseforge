#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND_DIR="$ROOT/frontend"
BACKEND_DIR="$ROOT/backend"
BIN_DIR="$ROOT/bin"
BINARY_PATH="$BIN_DIR/courseforge"
GO_CACHE_DIR="$ROOT/.cache/go-build"
SKIP_DEPS=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-deps)
      SKIP_DEPS=1
      shift
      ;;
    *)
      echo "Unknown option: $1" >&2
      exit 1
      ;;
  esac
done

mkdir -p "$GO_CACHE_DIR"
export GOCACHE="$GO_CACHE_DIR"

if [[ $SKIP_DEPS -eq 0 || ! -d "$FRONTEND_DIR/node_modules" ]]; then
  (cd "$FRONTEND_DIR" && npm ci)
fi

(cd "$FRONTEND_DIR" && npm run build)

mkdir -p "$BIN_DIR"
(cd "$BACKEND_DIR" && go run github.com/swaggo/swag/cmd/swag init -g main.go -d ./cmd/server,./internal/api/handlers,./internal/api/dto -o ./docs --exclude ./courses)
(cd "$BACKEND_DIR" && go build -tags swagger -o "$BINARY_PATH" ./cmd/courseforge)

echo "Built $BINARY_PATH"
