#!/usr/bin/env bash
set -euo pipefail

REPO="${CALLMEBACK_REPO:-tungnguyensipher/callmeback}"
INSTALL_DIR="${CALLMEBACK_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${CALLMEBACK_VERSION:-}"

need() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Missing dependency: $1" >&2
    exit 1
  }
}

need uname
need mktemp
need rm
need mkdir
need chmod
need printf
need sed
need install
need curl
need tar

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
  darwin) GOOS="darwin" ;;
  linux) GOOS="linux" ;;
  *) echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

case "$ARCH" in
  arm64|aarch64) GOARCH="arm64" ;;
  x86_64|amd64) GOARCH="amd64" ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

if [[ -z "$VERSION" ]]; then
  latest_url="$(curl -fsSL -o /dev/null -w "%{url_effective}" -L "https://github.com/$REPO/releases/latest" || true)"
  VERSION="$(printf "%s" "$latest_url" | sed -n 's#.*/tag/v\([^/][^/]*\)$#\1#p' | head -n1)"

  if [[ -z "$VERSION" ]]; then
    VERSION="$(curl -fsSL \
      -H "Accept: application/vnd.github+json" \
      -H "User-Agent: callmeback-installer" \
      "https://api.github.com/repos/$REPO/releases/latest" \
      | sed -n 's/.*"tag_name": "v\([0-9][^"]*\)".*/\1/p' \
      | head -n1)"
  fi
fi

if [[ -z "$VERSION" ]]; then
  echo "Failed to detect latest version. Set CALLMEBACK_VERSION=1.2.3 and retry." >&2
  exit 1
fi

TAG="v$VERSION"
ASSET="callmeback_${VERSION}_${GOOS}_${GOARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/$TAG/$ASSET"

TMPDIR="$(mktemp -d)"
cleanup() { rm -rf "$TMPDIR"; }
trap cleanup EXIT

echo "Installing callmeback $TAG from $URL"

curl -fL --retry 3 --retry-delay 1 -o "$TMPDIR/$ASSET" "$URL"
tar -xzf "$TMPDIR/$ASSET" -C "$TMPDIR"

mkdir -p "$INSTALL_DIR"
install -m 0755 "$TMPDIR/callmeback" "$INSTALL_DIR/callmeback"

echo "Installed: $INSTALL_DIR/callmeback"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    echo
    echo "Adding $INSTALL_DIR to PATH..."
    if [[ -n "${ZSH_VERSION:-}" ]]; then
      PROFILE="$HOME/.zshrc"
    else
      PROFILE="$HOME/.bashrc"
      [[ -f "$HOME/.bash_profile" ]] && PROFILE="$HOME/.bash_profile"
    fi
    LINE="export PATH=\"$INSTALL_DIR:\$PATH\""
    if [[ -f "$PROFILE" ]] && grep -qF "$LINE" "$PROFILE"; then
      :
    else
      printf "\n# callmeback\n%s\n" "$LINE" >> "$PROFILE"
    fi
    echo "Updated: $PROFILE"
    echo "Restart your shell or run: export PATH=\"$INSTALL_DIR:\$PATH\""
    ;;
esac

echo
echo "Run: callmeback --help"
