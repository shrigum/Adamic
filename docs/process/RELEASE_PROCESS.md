# Release Process

Shipping here means: **changelog rotated → version bumped → tag pushed → binaries
built → attached to a GitHub Release.** There are no servers, no deployments, and
no paid services anywhere in this process. Everything below runs on a developer
machine; the GitHub Actions workflow is a free convenience that mirrors the local
steps, not a requirement.

The [release-manager skill](../../.claude/skills/release-manager/SKILL.md)
executes this document. If the skill and this document ever disagree, this
document wins — fix the skill.

## Versioning

[SemVer 2.0.0](https://semver.org/spec/v2.0.0.html), applied to the app's
**user-visible surface**: CLI commands/flags, on-disk file formats, and documented
behavior.

- **MAJOR** — a user must change something to upgrade (removed/renamed command or
  flag, settings-file format change without automatic migration).
- **MINOR** — new backward-compatible capability (new command, new optional flag,
  new settings key).
- **PATCH** — bug fixes and internal changes only.

Pre-1.0, MINOR may break (per SemVer), but breaking changes still get a
changelog callout and, where feasible, a migration note.

The single source of truth for the version is the **git tag** (`v<semver>`).
Binaries embed it at build time via `-ldflags "-X main.version=..."` — there is
no version constant to keep in sync in the source.

## Prerequisites (one-time)

- Push access to the repository.
- [`gh` CLI](https://cli.github.com/) authenticated (`gh auth status`) — only
  needed if you attach artifacts manually instead of letting CI do it.
- Go 1.24+, bash (Git Bash is fine on Windows).

## Cutting a release

### 1. Pre-flight (manual judgment)

- [ ] `main` is green in CI and contains everything intended for this release.
- [ ] Read the `Unreleased` section of [CHANGELOG.md](../../CHANGELOG.md).
      Decide the version bump from its *content* using the SemVer rules above —
      the changelog drives the version, never the other way around.
- [ ] No open BLOCKING issues labeled `release-blocker`.

### 2. Rotate the changelog (a normal PR)

- [ ] Move the `Unreleased` contents into a new `## [X.Y.Z] - YYYY-MM-DD` section;
      leave a fresh empty `Unreleased` behind.
- [ ] Update the link references at the bottom of the changelog.
- [ ] Prune empty subsections; sanity-edit entries so they read as release notes
      (users read this file — it *is* the release notes).
- [ ] Merge this PR. The release tag will point at this merge commit.

### 3. Verify, build, and tag (scripted)

```bash
git checkout main && git pull
./scripts/release.sh X.Y.Z
```

`release.sh` refuses to proceed unless all of the following hold, so you don't
have to remember them: working tree clean; on `main` (or a `release/X.Y`
branch for hotfixes — see below) and up to date with origin; `X.Y.Z` is valid
SemVer and greater than the latest existing tag in scope; the changelog
contains a `[X.Y.Z]` section; `go test ./...`, `go vet ./...`, and `gofmt` all
clean. It then:

1. Cross-compiles `dist/app_X.Y.Z_<os>_<arch>[.exe]` for windows/amd64,
   darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, embedding the version.
2. Writes `dist/SHA256SUMS`.
3. Creates the annotated tag `vX.Y.Z` and prints the exact push command —
   it deliberately does **not** push for you.

### 4. Publish

```bash
git push origin vX.Y.Z
```

Pushing the tag triggers [.github/workflows/release.yml](../../.github/workflows/release.yml),
which runs the **full three-OS test matrix** (per-push CI is Linux-only; the
Windows/macOS verification happens here, gating publication), then rebuilds
the binaries and creates the GitHub Release with artifacts and the changelog
section as release notes.

**Offline / no-CI fallback** (the process must not depend on CI): after step 3,

```bash
gh release create vX.Y.Z dist/* \
  --title "vX.Y.Z" \
  --notes-file <(awk '/^## \[X.Y.Z\]/{f=1;next} /^## \[/{f=0} f' CHANGELOG.md)
```

### 5. Post-release

- [ ] Download one artifact from the Release page, run `app --version`, confirm
      it prints `X.Y.Z`. (Smoke-testing the published artifact, not your local
      build, is the point.)
- [ ] Announce wherever the project announces (if anywhere).

## Hotfixing an older series

Once a new MAJOR (or MINOR) has shipped, users may still be on the previous
series and need a fix that can't wait for them to upgrade (e.g. 2.x is out,
but 1.2.x users hit a data-loss bug). Cut the hotfix from a **release branch**:

```bash
git checkout -b release/1.2 v1.2.3     # branch from the tag being fixed
git cherry-pick <fix-commits>           # the fix, tests included, from main
# rotate CHANGELOG.md on this branch: add a [1.2.4] section for the fix
git push -u origin release/1.2
./scripts/release.sh 1.2.4              # script validates against the 1.2.* series only
git push origin v1.2.4
```

Rules:
- The fix lands on `main` first (normal flow, with its regression test), then
  is cherry-picked back — never fixed only on the release branch.
- `release.sh` accepts `release/X.Y` branches and bounds the version check to
  that series' tags, so shipping `1.2.4` after `2.0.0` exists is supported.
- The branch is kept only while its series is supported; delete it when
  support ends (tags carry the history).

## Fixing a bad release

- **Broken artifact, correct code**: delete the Release's assets, rebuild from the
  tag, re-upload. Never re-point or delete a pushed tag — tags are immutable
  history.
- **Broken code**: ship `X.Y.(Z+1)` with the fix. Yank the bad release by editing
  its notes to say **"Known-bad: see X.Y.Z+1"** and marking it as a pre-release so
  it stops being "latest". Do not delete it; users may need to identify what they
  are running.
