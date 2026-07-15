#!/bin/sh
set -e

REPO="heartleo/hn-cli"
BIN="hn"

warn() {
  echo "warning: $1" >&2
}

# Resolve install directory.
# Precedence: HN_INSTALL_DIR > $XDG_BIN_HOME > $XDG_DATA_HOME/../bin > $HOME/.local/bin
if [ -n "${HN_INSTALL_DIR:-}" ]; then
  INSTALL_DIR="$HN_INSTALL_DIR"
elif [ -n "${XDG_BIN_HOME:-}" ]; then
  INSTALL_DIR="$XDG_BIN_HOME"
elif [ -n "${XDG_DATA_HOME:-}" ]; then
  INSTALL_DIR="$XDG_DATA_HOME/../bin"
elif [ -n "${HOME:-}" ]; then
  INSTALL_DIR="$HOME/.local/bin"
else
  echo "Cannot determine an install directory: set HN_INSTALL_DIR" >&2
  exit 1
fi
INSTALL_DIR="${INSTALL_DIR%/}"

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  darwin) OS="darwin" ;;
  linux)  OS="linux" ;;
  *)      echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
  x86_64 | amd64) ARCH="x86_64" ;;
  arm64 | aarch64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

# Get latest version
VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed 's/.*"tag_name": *"\(.*\)".*/\1/')
if [ -z "$VERSION" ]; then
  echo "Failed to fetch latest version" >&2
  exit 1
fi

ARCHIVE="${BIN}_${VERSION#v}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

# Canonicalize so the PATH comparison below sees the same string as $PATH does
# ($XDG_DATA_HOME/../bin would otherwise never match a literal PATH entry).
mkdir -p "$INSTALL_DIR"
INSTALL_DIR=$(cd "$INSTALL_DIR" && pwd)

echo "Installing ${BIN} ${VERSION} (${OS}/${ARCH}) to ${INSTALL_DIR}..."

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

curl -fsSL "$URL" -o "$TMP/$ARCHIVE"
tar -xzf "$TMP/$ARCHIVE" -C "$TMP"

mv "$TMP/$BIN" "$INSTALL_DIR/$BIN"
chmod +x "$INSTALL_DIR/$BIN"

echo "Installed to ${INSTALL_DIR}/${BIN}"

# Warn if the install directory is not on PATH.
in_path=false
set -f
IFS=:
for entry in $PATH; do
  if [ "${entry%/}" = "$INSTALL_DIR" ]; then
    in_path=true
    break
  fi
done
unset IFS
set +f

if [ "$in_path" = false ]; then
  warn "${INSTALL_DIR} is not in your PATH. Add it to your shell profile:"
  echo "    export PATH=\"${INSTALL_DIR}:\$PATH\"" >&2
fi

# Warn if a different hn is ahead on PATH (e.g. an older sudo-installed copy).
shadow=$(command -v "$BIN" 2>/dev/null || true)
if [ -n "$shadow" ] && [ "$shadow" != "${INSTALL_DIR}/${BIN}" ]; then
  warn "another ${BIN} takes precedence on your PATH: ${shadow}"
  warn "remove it, or put ${INSTALL_DIR} earlier in PATH"
fi

"${INSTALL_DIR}/${BIN}" --version
