#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

if [[ ! -f "$SCRIPT_DIR/antmove" ]]; then
  echo "antmove binary not found next to install-local.sh" >&2
  exit 1
fi

exec "$SCRIPT_DIR/antmove" install
