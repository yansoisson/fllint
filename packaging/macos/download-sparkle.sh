#!/bin/bash
set -euo pipefail

# download-sparkle.sh — Downloads Sparkle 2.x framework for macOS auto-update support.
# Usage: ./download-sparkle.sh [version]
# Idempotent: skips download if Sparkle.framework already exists.

SPARKLE_VERSION="${1:-2.6.4}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DEST="$SCRIPT_DIR/Sparkle.framework"

if [ -d "$DEST" ]; then
    echo "Sparkle.framework already exists at $DEST"
    echo "Delete it first to re-download."
    exit 0
fi

TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT

URL="https://github.com/sparkle-project/Sparkle/releases/download/${SPARKLE_VERSION}/Sparkle-${SPARKLE_VERSION}.tar.xz"

echo "Downloading Sparkle ${SPARKLE_VERSION}..."
curl -L --fail -o "$TEMP_DIR/sparkle.tar.xz" "$URL"

echo "Extracting..."
tar -xf "$TEMP_DIR/sparkle.tar.xz" -C "$TEMP_DIR"

if [ ! -d "$TEMP_DIR/Sparkle.framework" ]; then
    echo "ERROR: Sparkle.framework not found in the archive."
    exit 1
fi

cp -R "$TEMP_DIR/Sparkle.framework" "$DEST"
echo "Sparkle.framework installed at $DEST"
