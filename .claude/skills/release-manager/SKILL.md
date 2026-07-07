---
name: release-manager
description: Cut a release locally — decide the SemVer bump from the changelog, rotate CHANGELOG.md, run scripts/release.sh to verify/build/tag, and publish the GitHub Release with binaries attached. Use when someone says "cut a release", "ship vX.Y.Z", "release what's on main", or asks what the next version should be. Stage 7 of docs/process/PLANNING_FLOW.md; executes docs/process/RELEASE_PROCESS.md.
---

# release-manager

You execute [docs/process/RELEASE_PROCESS.md](../../../docs/process/RELEASE_PROCESS.md).
If this skill and that document ever disagree, **the document wins — and
fixing this skill becomes part of the release.** Releases here are artifacts,
never deployments: the output is a git tag plus a GitHub Release with
binaries and checksums, produced entirely from a local machine (CI is a
mirror, not a dependency).

## Hard rules

- **The changelog drives the version, never the reverse.** You read what
  shipped and derive the bump; you don't pick a number and back-fill.
- **Tags are immutable.** You never delete, move, or re-point a pushed tag.
  Bad releases are fixed forward (see "When it goes wrong").
- **You do not push without the human.** `release.sh` deliberately stops
  before pushing; pushing the tag is the publish act and is confirmed with
  the user explicitly (this is the one hard stop in the flow).

## Procedure

### 1. Decide the version
Read the `Unreleased` section of `CHANGELOG.md` and classify each entry
against the SemVer rules in RELEASE_PROCESS.md (user-visible surface: CLI
commands/flags, on-disk formats, documented behavior):
- any breaking change → MAJOR (pre-1.0: MINOR, with a breaking-change callout),
- else any new capability → MINOR,
- else → PATCH.

State your classification per entry when proposing the version — the
reasoning is the deliverable; the number falls out. Empty `Unreleased`?
Nothing to release; say so and stop.

Cross-check completeness: `git log <last-tag>..main --oneline` — merged work
missing from the changelog is a Definition-of-Done escape. Add the missing
entries (that's a fix, not a favor) before continuing.

### 2. Rotate the changelog (normal PR)
Move `Unreleased` → `## [X.Y.Z] - <today>`; fresh empty `Unreleased`; update
the link refs at the bottom; prune empty subsections; edit entries to read as
release notes (users read this — it *is* the release notes, and
release.yml extracts this exact section for the GitHub Release body).

### 3. Verify, build, tag
```bash
git checkout main && git pull
./scripts/release.sh X.Y.Z
```
The script enforces the preconditions (clean tree, on synced main, valid +
increasing SemVer, changelog section present, tests/vet/fmt green), builds
all five platform targets with the version embedded, writes `SHA256SUMS`,
and creates the local tag. **Do not work around a precondition failure** —
each one is a named process violation; fix the cause. Sanity-check the
output: five binaries + checksums in `dist/`, and run the local one with
`--version` to confirm it prints `X.Y.Z`.

### 4. Publish (after explicit user confirmation)
```bash
git push origin vX.Y.Z     # release.yml builds + creates the GitHub Release
```
Offline/no-CI fallback: `gh release create vX.Y.Z dist/* --title "vX.Y.Z"`
with the changelog section as notes (exact command in RELEASE_PROCESS.md).

### 5. Post-release verification
Download one artifact **from the Release page** (not your local build), run
`--version`, confirm. Confirm the Release notes render correctly. Report the
release URL as the final deliverable.

## When it goes wrong

- **Precondition failures** at step 3: report which check failed and what it
  means; the fixes are ordinary PRs, then re-run.
- **Broken artifact, good code**: rebuild from the tag, replace the Release's
  assets. Tag untouched.
- **Broken code discovered post-release**: fix forward as `X.Y.(Z+1)` through
  the normal flow (a regression gets a failing test first — test-engineer's
  regression rule). Mark the bad Release pre-release and prepend
  "Known-bad: see X.Y.Z+1" to its notes. Never delete it.

## Handoff

After publishing, hand anything user-facing that changed in the release story
(install instructions, supported platforms) to **docs-maintainer** for a
README sweep.
