#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BINARY_PATH="$ROOT/bin/courseforge"
FRONTEND_DIR="$ROOT/frontend/dist"
HOST="127.0.0.1"
PORT="8080"
COURSES_DIR="$ROOT/courses"
DATA_DIR="$ROOT/data"
BUILD=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --host)
      HOST="$2"
      shift 2
      ;;
    --port)
      PORT="$2"
      shift 2
      ;;
    --courses-dir)
      COURSES_DIR="$2"
      shift 2
      ;;
    --data-dir)
      DATA_DIR="$2"
      shift 2
      ;;
    --build)
      BUILD=1
      shift
      ;;
    *)
      echo "Unknown option: $1" >&2
      exit 1
      ;;
  esac
done

if [[ $BUILD -eq 1 || ! -x "$BINARY_PATH" || ! -f "$FRONTEND_DIR/index.html" ]]; then
  "$ROOT/scripts/build.sh"
fi

exec "$BINARY_PATH" \
  "--host=$HOST" \
  "--port=$PORT" \
  "--courses-dir=$COURSES_DIR" \
  "--data-dir=$DATA_DIR" \
  "--frontend-dir=$FRONTEND_DIR"
