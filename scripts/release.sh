#!/usr/bin/env bash
# Cut a release locally: verify, build all artifacts, tag.
#
# Usage: ./scripts/release.sh X.Y.Z
#
# Implements steps of docs/process/RELEASE_PROCESS.md ("Verify, build, and
# tag"). It deliberately does NOT push — publishing is a separate human action.
# Rotate the changelog BEFORE running this (the script checks).
set -euo pipefail

cd "$(dirname "$0")/.."

VERSION="${1:-}"
fail() { echo "release: $*" >&2; exit 1; }

# semver_gt A B: exit 0 iff A > B per SemVer precedence. Handles numeric
# fields (1.10 > 1.9) and pre-releases ranking below their release
# (1.0.0-rc1 < 1.0.0). Two pre-releases of the same core version compare as
# plain strings — sufficient for this project's tagging scheme (rc1 < rc2).
# Build metadata (+...) is ignored. Mirrors semverGreater in src/update.
semver_gt() {
  local acore="${1%%-*}" bcore="${2%%-*}" a1 a2 a3 b1 b2 b3
  acore="${acore%%+*}"; bcore="${bcore%%+*}"
  IFS=. read -r a1 a2 a3 <<<"$acore"
  IFS=. read -r b1 b2 b3 <<<"$bcore"
  if (( a1 != b1 )); then (( a1 > b1 )); return; fi
  if (( a2 != b2 )); then (( a2 > b2 )); return; fi
  if (( a3 != b3 )); then (( a3 > b3 )); return; fi
  local apre="" bpre=""
  [[ "$1" == *-* ]] && { apre="${1#*-}"; apre="${apre%%+*}"; }
  [[ "$2" == *-* ]] && { bpre="${2#*-}"; bpre="${bpre%%+*}"; }
  [[ "$apre" == "$bpre" ]] && return 1     # equal versions: not greater
  [[ -z "$apre" ]] && return 0             # release > pre-release
  [[ -z "$bpre" ]] && return 1
  [[ "$apre" > "$bpre" ]]
}

# latest_tag GLOB: highest existing tag (without the v) matching GLOB, by SemVer.
latest_tag() {
  local best="" t
  while IFS= read -r t; do
    t="${t#v}"
    if [[ -z "$best" ]] || semver_gt "$t" "$best"; then best="$t"; fi
  done < <(git tag --list "$1")
  echo "$best"
}

# --- Preconditions ----------------------------------------------------------
[[ -n "$VERSION" ]] || fail "usage: ./scripts/release.sh X.Y.Z"
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.-]+)?$ ]] \
  || fail "'$VERSION' is not valid SemVer (expected X.Y.Z)"

[[ -z "$(git status --porcelain)" ]] || fail "working tree is not clean"

# Releases are cut from main; hotfixes to an older series are cut from a
# release/X.Y branch (docs/process/RELEASE_PROCESS.md, "Hotfixing an older
# series"), where the version must belong to that series and only tags of
# that series bound it.
BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [[ "$BRANCH" == "main" ]]; then
  TAG_SCOPE='v*'
elif [[ "$BRANCH" =~ ^release/([0-9]+\.[0-9]+)$ ]]; then
  SERIES="${BASH_REMATCH[1]}"
  [[ "$VERSION" == "$SERIES".* ]] \
    || fail "version $VERSION does not belong to series $SERIES (branch $BRANCH)"
  TAG_SCOPE="v${SERIES}.*"
else
  fail "releases are cut from main or a release/X.Y branch (on '$BRANCH')"
fi

git fetch origin "$BRANCH" --quiet 2>/dev/null || echo "release: warning: could not fetch origin (offline?); continuing with local $BRANCH"
if git rev-parse --verify "origin/$BRANCH" >/dev/null 2>&1; then
  [[ "$(git rev-parse HEAD)" == "$(git rev-parse "origin/$BRANCH")" ]] \
    || fail "local $BRANCH is not in sync with origin/$BRANCH"
fi

git rev-parse "v$VERSION" >/dev/null 2>&1 && fail "tag v$VERSION already exists"

LATEST="$(latest_tag "$TAG_SCOPE")"
if [[ -n "$LATEST" ]]; then
  semver_gt "$VERSION" "$LATEST" \
    || fail "$VERSION is not greater than latest tag v$LATEST (scope: $TAG_SCOPE)"
fi

grep -q "^## \[$VERSION\]" CHANGELOG.md \
  || fail "CHANGELOG.md has no '## [$VERSION]' section — rotate the changelog first (docs/process/RELEASE_PROCESS.md, step 2)"

# --- Verification -----------------------------------------------------------
echo "release: running tests..."
go test ./...
go vet ./...
UNFORMATTED="$(gofmt -l ./src ./tests)"
[[ -z "$UNFORMATTED" ]] || fail "unformatted files: $UNFORMATTED"

# --- Build ------------------------------------------------------------------
rm -rf dist
./scripts/build.sh --all "$VERSION"

echo "release: writing checksums..."
(cd dist && { command -v sha256sum >/dev/null && sha256sum adamic_* || shasum -a 256 adamic_*; } > SHA256SUMS)

# --- Tag --------------------------------------------------------------------
git tag -a "v$VERSION" -m "Release v$VERSION"

cat <<EOF

release: v$VERSION ready.
  Artifacts:  dist/ ($(ls dist | wc -l | tr -d ' ') files, incl. SHA256SUMS)
  Tag:        v$VERSION (local only — nothing has been pushed)

To publish (docs/process/RELEASE_PROCESS.md, step 4):
  git push origin v$VERSION        # CI builds + creates the GitHub Release
Or, without CI:
  gh release create v$VERSION dist/* --title "v$VERSION"
EOF
