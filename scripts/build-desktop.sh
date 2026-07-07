#!/usr/bin/env bash
# Builds the Adamic desktop app (Wails v3 shell) into dist/.
#
# Prereq: the frontend viewer model is shared with the pure-logic tests in
# frontend/; keep desktop/assets/viewer.js in sync with frontend/src/viewer.js
# (this script copies it so the tested version is what ships).
#
# Regenerating bindings (only needed when a bound Go method's signature changes):
#   wails3 generate bindings -b -d desktop/assets/bindings ./desktop/...
set -euo pipefail

cd "$(dirname "$0")/.."

# Ship the unit-tested viewer model verbatim.
cp frontend/src/viewer.js desktop/assets/viewer.js

mkdir -p dist
# Windowed (-H windowsgui: no console), stripped, no cgo (PDFium runs on wasm).
CGO_ENABLED=0 go build -ldflags "-H windowsgui -s -w" -o dist/Adamic.exe ./desktop/
echo "Built dist/Adamic.exe"
