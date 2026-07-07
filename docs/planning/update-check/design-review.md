# Design review: Update check command

- **Stage**: 3 — Design review ([planning flow](../../process/PLANNING_FLOW.md))
- **Reviewer**: architecture-reviewer skill
- **Inputs**: [spec.md](spec.md), [critical-path.md](critical-path.md), ADR index
- **Date**: 2026-07-07

## Verdict: APPROVED-WITH-CONDITIONS

All conditions satisfied before merge (statuses below).

## Fit with existing architecture

- New `src/update/` domain package, no imports from `main`, returns
  values/errors, never prints — matches the established shape. ✔
- Zero new dependencies (`net/http`, `encoding/json` stdlib). ✔
- **This is the first network access in the codebase.** That is a cross-cutting
  local-first policy change, not just a feature — ADR required regardless of
  the implementation's size.

## ADR decision

**Required.** Triggers: external interface (GitHub Releases API becomes an
optional runtime dependency) and cross-cutting policy (under what conditions
this app may touch the network at all).
→ Produced: [ADR-0003](../../architecture/ADR-0003-update-check.md). **Done.**

## Complexity check

- ✔ No HTTP client abstraction, no retry/backoff machinery, no response
  caching — one request, one timeout, report or fail. Correct scale.
- ✔ SemVer comparison hand-rolled (~40 lines) rather than importing a semver
  library: consistent with stdlib-first; the spec pins the two cases that
  actually matter (numeric fields, pre-release ordering) with tests.
- ⚠ Watch item, not a finding: `release-manager`/`release.sh` also reason
  about SemVer (in bash). Two implementations of version comparison now exist
  in different languages. Acceptable at current scale; if a third appears,
  consolidate (rule of three) — likely by having the release script call the
  Go binary.

## Conditions

| # | Condition | Rationale | Status |
|---|---|---|---|
| C1 | The check must be **explicitly invoked only** — no auto-check on startup, and ADR-0003 must record "the app makes no network request except during `app update`" as a standing invariant reviewers enforce. | Local-first: an app that phones home on startup violates the template's core promise; also startup latency. | Done — recorded in ADR-0003 and in the package doc comment. |
| C2 | Repo identity is a build-time `var` (ldflags-settable), defaulting to a placeholder that degrades to a clear "not configured" error. | The template must behave sensibly before instantiation (spec AC4); products must not let end users redirect the check. | Done — `update.Repo` + `ErrNotConfigured`. |
| C3 | Endpoint override via `APP_UPDATE_URL` env var so the test suite never touches the real network. | Same precedent as `APP_CONFIG_DIR` (settings-file C3); suite must pass offline. | Done — unit + integration tests use httptest servers only. |

## Notes for the implementer

- Failure messages must carry the reassurance that the app itself is
  unaffected (spec AC3) — users seeing a network error from a local-first app
  need to know nothing is broken.
- Exit codes: 0 for a completed check (regardless of outcome), 1 for a failed
  check, consistent with the config commands.
