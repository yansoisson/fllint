#!/bin/bash
set -euo pipefail

# build-app.sh — Creates the macOS distribution folder.
#
# Usage: ./packaging/macos/build-app.sh [output-dir]
#
# Output structure:
#   {output-dir}/Fllint/
#     Fllint.app/
#       Contents/
#         Info.plist
#         MacOS/fllint
#         Resources/
#           icon.icns
#           bin/llama-server + dylibs
#     Data/
#       models/
#       conversations/

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
OUTPUT_DIR="${1:-$PROJECT_ROOT/dist}"

DIST="$OUTPUT_DIR/Fllint"
APP="$DIST/Fllint.app"

echo "=== Fllint macOS Distribution Builder ==="
echo "Project root: $PROJECT_ROOT"
echo "Output:       $DIST"
echo ""

# Step 1: Build the Go binary
echo "--- Building Go binary ---"
cd "$PROJECT_ROOT"
make build
echo ""

# Step 2: Create .app bundle structure
echo "--- Creating .app bundle ---"
rm -rf "$DIST"
mkdir -p "$APP/Contents/MacOS"
mkdir -p "$APP/Contents/Resources/bin"

# Step 3: Copy Info.plist
cp "$SCRIPT_DIR/Info.plist" "$APP/Contents/Info.plist"

# Step 4: Copy Go binary
cp "$PROJECT_ROOT/fllint" "$APP/Contents/MacOS/fllint"
chmod +x "$APP/Contents/MacOS/fllint"

# Step 5: Copy icon (if exists)
if [ -f "$SCRIPT_DIR/icon.icns" ]; then
    cp "$SCRIPT_DIR/icon.icns" "$APP/Contents/Resources/icon.icns"
    echo "Icon: copied"
else
    echo "WARNING: No icon.icns found at $SCRIPT_DIR/icon.icns"
    echo "         The app will work but won't have a custom icon."
fi

# Step 6: Copy Sparkle.framework (optional — enables auto-update)
SPARKLE_SRC="$SCRIPT_DIR/Sparkle.framework"
FRAMEWORKS_DST="$APP/Contents/Frameworks"

if [ -d "$SPARKLE_SRC" ]; then
    mkdir -p "$FRAMEWORKS_DST"
    cp -R "$SPARKLE_SRC" "$FRAMEWORKS_DST/Sparkle.framework"
    echo "Sparkle.framework: copied"
else
    echo "NOTE: Sparkle.framework not found at $SPARKLE_SRC"
    echo "      Auto-update will not be available. Run ./download-sparkle.sh to fetch it."
fi

# Step 7: Build and copy sparkle-helper (optional — requires Sparkle.framework)
HELPER_SRC="$SCRIPT_DIR/sparkle-helper"

if [ -d "$SPARKLE_SRC" ] && [ -f "$HELPER_SRC/main.m" ]; then
    echo "--- Building sparkle-helper ---"
    make -C "$HELPER_SRC" SPARKLE_FRAMEWORK="$SPARKLE_SRC" build
    cp "$HELPER_SRC/sparkle-helper" "$APP/Contents/MacOS/sparkle-helper"
    chmod +x "$APP/Contents/MacOS/sparkle-helper"
    echo "sparkle-helper: built and copied"
else
    echo "NOTE: sparkle-helper not built (Sparkle.framework or source not found)"
fi

# Step 8: Copy llama-server binary and shared libraries
BIN_SRC="$PROJECT_ROOT/bin"
BIN_DST="$APP/Contents/Resources/bin"

if [ -f "$BIN_SRC/llama-server" ]; then
    cp "$BIN_SRC/llama-server" "$BIN_DST/llama-server"
    chmod +x "$BIN_DST/llama-server"

    # Copy all .dylib files (shared libraries needed by llama-server)
    DYLIB_COUNT=0
    for dylib in "$BIN_SRC"/*.dylib; do
        [ -e "$dylib" ] || continue
        if [ -L "$dylib" ]; then
            # Preserve symlinks
            cp -P "$dylib" "$BIN_DST/"
        else
            cp "$dylib" "$BIN_DST/"
        fi
        DYLIB_COUNT=$((DYLIB_COUNT + 1))
    done
    echo "llama-server: copied with $DYLIB_COUNT shared libraries"
else
    echo "WARNING: llama-server not found at $BIN_SRC/llama-server"
    echo "         The app will run but cannot load models until llama-server is provided."
fi

# Step 7: Create Data directory structure
DATA="$DIST/Data"
mkdir -p "$DATA/models"
mkdir -p "$DATA/conversations"

# Copy default system prompt files so advanced users can edit them
PROMPTS_SRC="$PROJECT_ROOT/internal/prompt/defaults"
cp "$PROMPTS_SRC/system-prompt.md" "$DATA/system-prompt.md"
cp "$PROMPTS_SRC/summary-prompt.md" "$DATA/summary-prompt.md"
echo "Prompts: copied default system-prompt.md and summary-prompt.md"

# Step 8: Copy models (if any exist in project models dir)
MODELS_SRC="$PROJECT_ROOT/models"
if [ -d "$MODELS_SRC" ]; then
    MODEL_COPIED=false

    # Copy model subdirectories (e.g., models/Lite/model.gguf)
    for model_dir in "$MODELS_SRC"/*/; do
        [ -d "$model_dir" ] || continue
        if ls "$model_dir"*.gguf 1>/dev/null 2>&1; then
            dir_name="$(basename "$model_dir")"
            cp -r "$model_dir" "$DATA/models/$dir_name"
            echo "Model: copied $dir_name"
            MODEL_COPIED=true
        fi
    done

    # Copy loose .gguf files at top level
    for gguf in "$MODELS_SRC"/*.gguf; do
        [ -f "$gguf" ] || continue
        cp "$gguf" "$DATA/models/"
        echo "Model: copied $(basename "$gguf")"
        MODEL_COPIED=true
    done

    if [ "$MODEL_COPIED" = false ]; then
        echo "WARNING: No .gguf model files found in $MODELS_SRC"
        echo "         The app will run but users will need to add a model manually."
    fi
else
    echo "WARNING: No models directory found at $MODELS_SRC"
fi

echo ""
echo "=== Build complete ==="
echo "Distribution folder: $DIST"
echo ""
echo "To test: open \"$APP\""
echo "To distribute: create a DMG or zip the $DIST folder"
