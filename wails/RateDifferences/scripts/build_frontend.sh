#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

npm --prefix "$ROOT_DIR/frontend" run build
rm -rf "$ROOT_DIR/wails/RateDifferences/frontend/dist"
cp -R "$ROOT_DIR/frontend/dist" "$ROOT_DIR/wails/RateDifferences/frontend/dist"
