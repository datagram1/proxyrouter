#!/bin/bash

# ProxyRouter Firefox Extension Build Script
# Creates a distributable package of the extension

set -e

echo "=== Building ProxyRouter Firefox Extension ==="

# Configuration
EXTENSION_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD_DIR="$EXTENSION_DIR/build"
PACKAGE_NAME="proxyrouter-extension-$(date +%Y%m%d).zip"

# Clean previous build
if [ -d "$BUILD_DIR" ]; then
    echo "Cleaning previous build..."
    rm -rf "$BUILD_DIR"
fi

# Create build directory
echo "Creating build directory..."
mkdir -p "$BUILD_DIR"

# Copy extension files
echo "Copying extension files..."
cp "$EXTENSION_DIR/manifest.json" "$BUILD_DIR/"
cp "$EXTENSION_DIR/background.js" "$BUILD_DIR/"
cp "$EXTENSION_DIR/popup.html" "$BUILD_DIR/"
cp "$EXTENSION_DIR/popup.js" "$BUILD_DIR/"
cp "$EXTENSION_DIR/options.html" "$BUILD_DIR/"
cp "$EXTENSION_DIR/options.js" "$BUILD_DIR/"
cp "$EXTENSION_DIR/README.md" "$BUILD_DIR/"

# Create icons directory and copy icons if they exist
if [ -d "$EXTENSION_DIR/icons" ]; then
    echo "Copying icons..."
    cp -r "$EXTENSION_DIR/icons" "$BUILD_DIR/"
else
    echo "Warning: icons directory not found, creating placeholder..."
    mkdir -p "$BUILD_DIR/icons"
    
    # Create simple placeholder icons (you should replace these with real icons)
    echo "Creating placeholder icons..."
    
    # Create a simple SVG icon for different sizes
    for size in 16 32 48 128; do
        cat > "$BUILD_DIR/icons/icon${size}.svg" << EOF
<svg width="${size}" height="${size}" xmlns="http://www.w3.org/2000/svg">
  <rect width="${size}" height="${size}" fill="#3498db" rx="4"/>
  <text x="50%" y="50%" text-anchor="middle" dy="0.35em" fill="white" font-family="Arial" font-size="${size/2}">P</text>
</svg>
EOF
    done
fi

# Create package
echo "Creating package..."
cd "$BUILD_DIR"
zip -r "$PACKAGE_NAME" . -x "*.DS_Store" "*.git*" "*.zip"

echo "=== Build Complete ==="
echo "Package created: $BUILD_DIR/$PACKAGE_NAME"
echo
echo "To install in Firefox:"
echo "1. Open Firefox"
echo "2. Navigate to about:debugging"
echo "3. Click 'This Firefox' tab"
echo "4. Click 'Load Temporary Add-on'"
echo "5. Select the manifest.json file from: $BUILD_DIR"
echo
echo "For production distribution, you can:"
echo "1. Replace placeholder icons with real icons"
echo "2. Update version in manifest.json"
echo "3. Test thoroughly"
echo "4. Submit to Firefox Add-ons store (optional)"
