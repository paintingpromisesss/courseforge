#!/usr/bin/env bash
# Installs a prebuilt courseforge release binary into ~/.local/bin
# and adds it to PATH if needed. No Go/Node toolchain required.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/paintingpromisesss/courseforge/main/scripts/install-release.sh | sh
#   ./scripts/install-release.sh --tag v1.2.3 --install-dir ~/bin
set -euo pipefail

REPO="paintingpromisesss/courseforge"
TAG="latest"
INSTALL_DIR="${HOME}/.local/bin"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --tag)
      TAG="$2"
      shift 2
      ;;
    --install-dir)
      INSTALL_DIR="$2"
      shift 2
      ;;
    *)
      echo "Unknown option: $1" >&2
      exit 1
      ;;
  esac
done

case "$(uname -s)" in
  Linux) OS="linux" ;;
  Darwin) OS="macos" ;;
  *)
    echo "Unsupported OS: $(uname -s)" >&2
    exit 1
    ;;
esac

case "$(uname -m)" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

ASSET="courseforge-${OS}-${ARCH}"

if [[ "$TAG" == "latest" ]]; then
  URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"
else
  URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"
fi

mkdir -p "$INSTALL_DIR"
BINARY_PATH="${INSTALL_DIR}/courseforge"

echo "Downloading ${ASSET} (${TAG})..."
curl -fsSL "$URL" -o "$BINARY_PATH"

CHECKSUM_URL="${URL%/*}/checksums.txt"
CHECKSUM_FILE="${INSTALL_DIR}/checksums.txt"
if curl -fsSL "$CHECKSUM_URL" -o "$CHECKSUM_FILE" 2>/dev/null; then
  echo "Verifying SHA256 checksum..."
  EXPECTED_HASH="$(grep -w "${ASSET}" "$CHECKSUM_FILE" 2>/dev/null | awk '{print $1}')"
  if [[ -n "$EXPECTED_HASH" ]]; then
    if command -v sha256sum >/dev/null 2>&1; then
      ACTUAL_HASH="$(sha256sum "$BINARY_PATH" | awk '{print $1}')"
    elif command -v shasum >/dev/null 2>&1; then
      ACTUAL_HASH="$(shasum -a 256 "$BINARY_PATH" | awk '{print $1}')"
    else
      ACTUAL_HASH=""
    fi

    if [[ -n "$ACTUAL_HASH" ]]; then
      if [[ "$EXPECTED_HASH" != "$ACTUAL_HASH" ]]; then
        rm -f "$BINARY_PATH" "$CHECKSUM_FILE"
        echo "Error: Checksum mismatch! Expected ${EXPECTED_HASH}, got ${ACTUAL_HASH}" >&2
        exit 1
      fi
      echo "Checksum verified: SHA256 matches."
    fi
  fi
  rm -f "$CHECKSUM_FILE"
fi

chmod +x "$BINARY_PATH"

echo "Installed to $BINARY_PATH"

case ":${PATH}:" in
  *":${INSTALL_DIR}:"*)
    ;;
  *)
    SHELL_NAME="$(basename "${SHELL:-sh}")"
    case "$SHELL_NAME" in
      zsh) SHELL_RC="$HOME/.zshrc" ;;
      bash) SHELL_RC="$HOME/.bashrc" ;;
      *) SHELL_RC="$HOME/.profile" ;;
    esac
    printf '\nexport PATH="%s:$PATH"\n' "$INSTALL_DIR" >> "$SHELL_RC"
    echo "Added $INSTALL_DIR to PATH in $SHELL_RC (restart your shell or run: source $SHELL_RC)"
    ;;
esac

echo "Run: courseforge"
