#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
VERSION="$(sed -n 's/^var version = "\([^"]*\)"/\1/p' "$ROOT/model.go")"
if [[ -z "$VERSION" ]]; then
  echo "Could not read version from model.go" >&2
  exit 1
fi

mkdir -p "$ROOT/dist"
for ARCH in arm64 amd64; do
  TEMP_DIR="$(mktemp -d)"
  trap 'rm -rf "$TEMP_DIR"' EXIT
  echo "Building macOS $ARCH..."
  (cd "$ROOT" && CGO_ENABLED=0 GOOS=darwin GOARCH="$ARCH" go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o "$TEMP_DIR/antmove" .)
  cp "$ROOT/install-local.sh" "$ROOT/LICENSE" "$ROOT/llms.txt" "$TEMP_DIR/"
  cp "$ROOT"/README*.md "$TEMP_DIR/"
  chmod +x "$TEMP_DIR/install-local.sh"
  (cd "$TEMP_DIR" && tar -czf "$ROOT/dist/antmove_${VERSION}_darwin_${ARCH}.tar.gz" antmove install-local.sh LICENSE llms.txt README*.md)
  rm -rf "$TEMP_DIR"
  trap - EXIT
done
echo "Release archives written to $ROOT/dist"
