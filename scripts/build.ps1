# Build the app binary for the current platform (Windows convenience wrapper).
# For cross-platform release builds, use scripts/build.sh (runs in Git Bash),
# which is the canonical script — keep any logic changes there.
$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..")
New-Item -ItemType Directory -Force dist | Out-Null

$env:CGO_ENABLED = "0"
go build -trimpath -ldflags "-s -w -X main.version=dev" -o dist\adamic.exe .\src
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Output "Built dist\adamic.exe"
