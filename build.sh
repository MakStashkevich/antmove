#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
VERSION="$(sed -n 's/^var version = "\([^"]*\)"/\1/p' "$ROOT/src/model.go")"
if [[ -z "$VERSION" ]]; then
  echo "Could not read version from src/model.go" >&2
  exit 1
fi

mkdir -p "$ROOT/dist"
rm -f "$ROOT"/dist/antmove_"$VERSION"_*.tar.gz "$ROOT/dist/checksums.txt"
for ARCH in arm64 amd64; do
  case "$ARCH" in
    arm64) ARCH_NAME="apple-silicon" ;;
    amd64) ARCH_NAME="intel" ;;
  esac
  TEMP_DIR="$(mktemp -d)"
  trap 'rm -rf "$TEMP_DIR"' EXIT
  echo "Building macOS $ARCH..."
  (cd "$ROOT" && CGO_ENABLED=0 GOOS=darwin GOARCH="$ARCH" go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o "$TEMP_DIR/antmove" ./src)
  cp "$ROOT/install-local.sh" "$ROOT/LICENSE" "$ROOT/llms.txt" "$TEMP_DIR/"
  cp "$ROOT"/README*.md "$TEMP_DIR/"
  chmod +x "$TEMP_DIR/install-local.sh"
  (cd "$TEMP_DIR" && tar -czf "$ROOT/dist/antmove_${VERSION}_macos_${ARCH_NAME}_${ARCH}.tar.gz" antmove install-local.sh LICENSE llms.txt README*.md)
  rm -rf "$TEMP_DIR"
  trap - EXIT
done
echo "Release archives written to $ROOT/dist"
