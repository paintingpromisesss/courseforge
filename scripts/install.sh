#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BINARY_PATH="$ROOT/bin/courseforge"
INSTALL_DIR="${HOME}/.local/bin"
SKIP_DEPS=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --install-dir)
      INSTALL_DIR="$2"
      shift 2
      ;;
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

if [[ $SKIP_DEPS -eq 1 ]]; then
  "$ROOT/scripts/build.sh" --skip-deps
else
  "$ROOT/scripts/build.sh"
fi

mkdir -p "$INSTALL_DIR"
cp "$BINARY_PATH" "$INSTALL_DIR/courseforge"
chmod +x "$INSTALL_DIR/courseforge"

echo "Installed to $INSTALL_DIR/courseforge"
echo "If needed, add $INSTALL_DIR to PATH."
