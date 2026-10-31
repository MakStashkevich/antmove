#!/bin/bash
set -euo pipefail

REPOSITORY="MakStashkevich/antmove"

if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
  echo "Run install.sh as your user, without sudo." >&2
  exit 1
fi

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "antmove currently supports macOS only." >&2
  exit 1
fi

case "$(uname -m)" in
  arm64) ARCH="arm64"; ARCH_NAME="apple-silicon" ;;
  x86_64) ARCH="amd64"; ARCH_NAME="intel" ;;
  *) echo "Unsupported Mac architecture: $(uname -m)" >&2; exit 1 ;;
esac

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/antmove-install.XXXXXX")"
trap 'rm -rf "$WORK_DIR"' EXIT

RELEASE_JSON="$WORK_DIR/release.json"
curl -fsSL "https://api.github.com/repos/$REPOSITORY/releases/latest" -o "$RELEASE_JSON"
TAG="$(awk -F '"' '/"tag_name"/ { print $4; exit }' "$RELEASE_JSON")"
[[ -n "$TAG" ]] || { echo "No antmove release found." >&2; exit 1; }

VERSION="${TAG#v}"
ARCHIVE="antmove_${VERSION}_macos_${ARCH_NAME}_${ARCH}.tar.gz"
URL="https://github.com/$REPOSITORY/releases/download/$TAG/$ARCHIVE"
echo "Downloading antmove $VERSION for macOS $ARCH_NAME ($ARCH)..."
curl -fL "$URL" -o "$WORK_DIR/$ARCHIVE"
tar -xzf "$WORK_DIR/$ARCHIVE" -C "$WORK_DIR"
[[ -x "$WORK_DIR/antmove" ]] || { echo "Release archive is missing antmove." >&2; exit 1; }
"$WORK_DIR/antmove" install
