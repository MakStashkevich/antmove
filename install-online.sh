#!/bin/bash
set -euo pipefail

REPOSITORY="${ANTMOVE_GITHUB_REPO:-MakStashkevich/antmove}"
if [[ ! "$REPOSITORY" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]]; then
  echo "Set ANTMOVE_GITHUB_REPO to a GitHub owner/repository." >&2
  exit 2
fi

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "antmove currently supports macOS only." >&2
  exit 1
fi

case "$(uname -m)" in
  arm64) ARCH="arm64" ;;
  x86_64) ARCH="amd64" ;;
  *)
    echo "Unsupported Mac architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

for tool in curl tar; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "Required command not found: $tool" >&2
    exit 1
  fi
done

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/antmove-install.XXXXXX")"
trap 'rm -rf "$WORK_DIR"' EXIT

RELEASE_JSON="$WORK_DIR/release.json"
if ! curl -fsSL --retry 2 "https://api.github.com/repos/$REPOSITORY/releases/latest" -o "$RELEASE_JSON"; then
  echo "Could not read the latest GitHub release for $REPOSITORY." >&2
  exit 1
fi

TAG="$(awk -F '"' '/"tag_name"/ {print $4; exit}' "$RELEASE_JSON")"
if [[ -z "$TAG" ]]; then
  echo "The GitHub response did not contain a release tag." >&2
  exit 1
fi
VERSION="${TAG#v}"
ARCHIVE="antmove_${VERSION}_darwin_${ARCH}.tar.gz"
URL="https://github.com/$REPOSITORY/releases/download/$TAG/$ARCHIVE"

echo "Downloading antmove $VERSION for macOS $ARCH..."
if ! curl -fL --retry 2 "$URL" -o "$WORK_DIR/$ARCHIVE"; then
  echo "Could not download $ARCHIVE from the latest release." >&2
  exit 1
fi

tar -xzf "$WORK_DIR/$ARCHIVE" -C "$WORK_DIR"
if [[ ! -x "$WORK_DIR/antmove" || ! -f "$WORK_DIR/install.sh" ]]; then
  echo "The release archive is missing antmove or install.sh." >&2
  exit 1
fi

echo "Installing antmove and am for the current user..."
/bin/bash "$WORK_DIR/install.sh"

