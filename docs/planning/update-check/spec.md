# Spec: Update check command

- **Stage**: 1 — Intake ([planning flow](../../process/PLANNING_FLOW.md))
- **Author**: spec-writer skill, from the request "make updating easy — a button
  that checks if there is a newer version available, as part of the template"
- **Date**: 2026-07-07
- **Status**: Implemented (Unreleased)

## Problem

Released binaries are downloaded once and then age silently: nothing tells a
user that a newer version exists, so bug fixes and features never reach users
who don't habitually revisit the Releases page. Every product built from this
template inherits the problem, so the template itself should ship the
mechanism. At the same time, the project is local-first: any solution must not
make the app phone home, depend on the network, or degrade at all when offline.

## Non-goals

- **No self-update** (downloading and replacing the running binary). It drags
  in platform-specific file-locking, privilege, and code-signing concerns;
  the check tells the user where to get the new version, no more. Revisit as
  its own feature+ADR if user demand shows up.
- **No automatic/background checking.** The check runs only when the user
  explicitly invokes it. An opt-in "remind me" setting could layer on later.
- **No update channels** (beta/nightly). Latest published release only.
- **No offline caching of the answer.** The command needs the network by
  definition; when offline it fails clearly, it does not guess from stale data.

## Constraints

- Zero third-party dependencies (ADR-0001); `net/http` + `encoding/json` only.
- This becomes the **only network-touching code in the application**, and it
  must stay that way absent a new ADR — the property "offline-complete except
  explicit `update`" is part of the local-first contract (recorded in ADR-0003).
- Free infrastructure only: the GitHub Releases API of the project's own repo
  (no accounts, no keys for public repos; it is where releases already live).
- Must behave sensibly in the un-instantiated template, where no release
  repository exists yet.
- Request timeout must be short (seconds); a hung check is worse than a failed
  one.

## Assumptions

- **A1**: The version baked into the binary at build time (git tag via
  ldflags) is the truth about "what am I running"; local `dev` builds are not
  comparable and the command reports rather than compares for them.
- **A2**: The repository to check is a build-time property of the product
  (a `var` settable by ldflags or source edit at template instantiation), not
  a user setting — users of a product shouldn't be able to accidentally point
  it elsewhere.
- **A3**: Rate limiting (60 unauthenticated requests/hour/IP for the GitHub
  API) is a non-issue for an explicit, user-invoked command; a 403 surfaces
  as an ordinary failed-check error.
- **A4**: An endpoint override env var (`APP_UPDATE_URL`) is acceptable and
  desirable — it serves tests (no real network in the suite) and self-hosted
  forges, mirroring the `APP_CONFIG_DIR` precedent (settings-file C3).

## Acceptance criteria

| # | Criterion | Covering test |
|---|---|---|
| AC1 | With a release newer than the running version published, `app update` prints the new version and its release-page URL, exit 0. | `src/update: TestCheckReportsNewerVersion`; `tests: TestUpdateCommand/newer release available` |
| AC2 | When the running version equals the latest release, `app update` says so, exit 0, and never claims an update exists. | `src/update: TestCheckUpToDate`, `TestSemverGreater` |
| AC3 | With the network unreachable (or server erroring), `app update` exits 1 with a message that states the check failed **and that the app is unaffected**; no other command is impacted in any way. | `src/update: TestCheckUnreachableServer`, `TestCheckServerErrors`; `tests: TestUpdateCommand/unreachable server` |
| AC4 | In an un-instantiated template (placeholder repo), `app update` exits 1 explaining update checks are not configured for this build. | `src/update: TestCheckUnconfiguredRepo`; `tests: TestUpdateCommand/unconfigured` |
| AC5 | A `dev` (untagged) build never claims "newer version available"; it reports the latest release informationally. | `src/update: TestCheckDevBuildNeverClaimsNewer`; `tests: TestUpdateCommand/newer release available` |
| AC6 | Version comparison is SemVer-correct where it matters: numeric fields (1.10 > 1.9) and pre-release ordering (1.0.0-rc1 < 1.0.0). | `src/update: TestSemverGreater` |

**Error behavior summary**: every failure (offline, HTTP error, unparsable
response, non-SemVer tag, unconfigured) is a soft user-facing error, exit 1,
never a wrong answer, never a crash, never any effect on other functionality.

## Revision history

- 2026-07-07 — Initial version accepted into stage 2.
