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
