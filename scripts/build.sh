#!/usr/bin/env bash
# Build the app binary.
#
# Usage:
#   ./scripts/build.sh                 # current platform -> dist/adamic[.exe]
#   ./scripts/build.sh --all [VERSION] # all release targets -> dist/adamic_VERSION_os_arch[.exe]
#
# Runs on Linux, macOS, and Windows (Git Bash). Requires only Go.
# There are no deploy steps anywhere in this repo, by design: the release
# artifact is the binary itself (docs/process/RELEASE_PROCESS.md).
set -euo pipefail

cd "$(dirname "$0")/.."
mkdir -p dist

# Release targets. Keep in sync with .github/workflows/release.yml.
TARGETS=(
  "windows/amd64"
  "darwin/amd64"
  "darwin/arm64"
  "linux/amd64"
  "linux/arm64"
)

build_one() {
  local goos="$1" goarch="$2" version="$3" out="$4"
  echo "  ${goos}/${goarch} -> ${out}"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags "-s -w -X main.version=${version}" \
    -o "$out" ./src
}

if [[ "${1:-}" == "--all" ]]; then
  VERSION="${2:-dev}"
  echo "Building all release targets (version ${VERSION})..."
  for target in "${TARGETS[@]}"; do
    goos="${target%/*}"
    goarch="${target#*/}"
    ext=""
    [[ "$goos" == "windows" ]] && ext=".exe"
    build_one "$goos" "$goarch" "$VERSION" "dist/adamic_${VERSION}_${goos}_${goarch}${ext}"
  done
  echo "Done. Artifacts in dist/"
else
  ext=""
  [[ "$(go env GOOS)" == "windows" ]] && ext=".exe"
  echo "Building for current platform..."
  build_one "$(go env GOOS)" "$(go env GOARCH)" "dev" "dist/adamic${ext}"
fi
