#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
OUT_DIR="$ROOT_DIR/wails/RateDifferences/resources"
SRC="$ROOT_DIR/backend/tools/extract.swift"

mkdir -p "$OUT_DIR"

if ! command -v swiftc >/dev/null 2>&1; then
  echo "swiftc not found; cannot build extractor" >&2
  exit 1
fi

swiftc "$SRC" -o "$OUT_DIR/extract"
