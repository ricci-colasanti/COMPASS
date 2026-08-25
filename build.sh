#!/bin/bash
# build.sh – Cross‑compile COMPASS for Linux, Windows, macOS

set -e  # exit on error

# ---------- Configuration ----------
BINARY_NAME="compass"
BUILD_DIR="build"
VERSION="${1:-0.80}"          # optional version tag, default 0.80

# ---------- Clean previous builds ----------
rm -rf "${BUILD_DIR}"
mkdir -p "${BUILD_DIR}"

# ---------- Build for each platform ----------
echo "🚀 Building COMPASS version ${VERSION}..."

# Linux (64-bit)
echo "📦 Building for Linux (amd64)..."
GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=${VERSION}" -o "${BUILD_DIR}/${BINARY_NAME}_linux_amd64" .

# Windows (64-bit)
echo "📦 Building for Windows (amd64)..."
GOOS=windows GOARCH=amd64 go build -ldflags "-X main.version=${VERSION}" -o "${BUILD_DIR}/${BINARY_NAME}_windows_amd64.exe" .

# macOS (Intel / amd64)
echo "📦 Building for macOS (amd64)..."
GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.version=${VERSION}" -o "${BUILD_DIR}/${BINARY_NAME}_darwin_amd64" .

# macOS (Apple Silicon / arm64) – optional but recommended
echo "📦 Building for macOS (arm64)..."
GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.version=${VERSION}" -o "${BUILD_DIR}/${BINARY_NAME}_darwin_arm64" .

# ---------- Create a tarball with all binaries ----------
echo "📦 Creating archive..."
cd "${BUILD_DIR}"
tar -czf "compass_${VERSION}_all_platforms.tar.gz" *
cd - > /dev/null

# ---------- Done ----------
echo "✅ Build complete! Binaries are in the '${BUILD_DIR}' directory:"
ls -lh "${BUILD_DIR}"