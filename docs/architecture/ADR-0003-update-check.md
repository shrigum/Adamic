# ADR-0003: Opt-in update check via the GitHub Releases API

- **Status**: Accepted
- **Date**: 2026-07-07
- **Feature/trigger**: [docs/planning/update-check/](../planning/update-check/)
- **Deciders**: project maintainers

## Context

Binaries distributed as GitHub Release artifacts have no channel back to the
user: once downloaded, users never learn that newer versions exist. Every
product built from this template inherits that gap, and it was flagged as a
top future risk ("no update mechanism") in review of the template itself.

The tension: this is a **local-first** project — the standing promise is that
the app is fully functional offline and never phones home. Any update
mechanism necessarily touches the network, making this the first network
access in the codebase and therefore a cross-cutting policy decision, not just
a feature. Free-infrastructure and zero-dependency constraints (ADR-0001)
apply as always.

## Decision

We ship an **explicitly invoked** `app update` command that makes exactly one
HTTP request (5-second timeout) to the GitHub Releases "latest" endpoint of
the product's repository, compares the published version with the version
baked into the binary, and reports the result with the release-page URL. It
does not download anything, write anything, or run unless the user asks.

Standing invariant this ADR establishes: **the application makes no network
request except during an explicit `app update` invocation.** Any future
feature that wants network access must supersede or extend this ADR;
reviewers enforce the invariant at stage 3/5.

Supporting decisions: the repository identity is a build-time variable
(`update.Repo`, ldflags-settable) with a placeholder that degrades to a clear
"not configured" error; `APP_UPDATE_URL` overrides the endpoint for tests and
self-hosted forges; local `dev` builds report the latest release but never
claim comparability.

## Alternatives considered

### Do nothing (status quo)
Advantage: zero network surface, purest local-first posture. Lost because
stranded users on buggy versions is a real, recurring product harm, and an
opt-in check preserves the offline guarantee in full — the app remains
100% functional without it.

### Automatic check on startup (with cache/throttle)
Advantage: users actually learn about updates without remembering a command —
better reach. Lost because it phones home by default (a GitHub-visible IP
ping per user per day is telemetry, whatever the intent), adds startup
latency, and needs cache/throttle state. Could be layered later **only** as
an opt-in setting, which would extend this ADR.

### Full self-update (download and replace the binary)
Advantage: closes the loop; best user experience. Lost for now: binary
replacement is platform-specific (locked files on Windows, quarantine
attributes on macOS), security-sensitive (must verify checksums/signatures —
see the unresolved code-signing question), and large. Revisit as its own
feature+ADR on demonstrated demand; this command is its natural foundation.

### Static version file on a project website
Advantage: forge-agnostic. Lost because it requires infrastructure this
project refuses to have (a website to keep updated); the Releases API is
already the canonical, zero-maintenance source of truth. The `APP_UPDATE_URL`
override recovers forge-agnosticism where needed.

## Consequences

- **Positive**: users can discover updates with one command; products built
  from the template inherit the mechanism by setting one variable; the
  local-first offline guarantee is unchanged; the network policy is now
  explicit and enforceable instead of implicit.
- **Negative**: GitHub becomes an optional runtime dependency (degrades to a
  clear error offline or if the API changes shape); an explicit command has
  limited reach compared to auto-check — accepted trade-off, revisit condition
  above; unauthenticated API rate limits (60/hr/IP) could affect heavy shared
  IPs — accepted (spec A3); **the release repository must be public** — the
  request is unauthenticated, so a private repo's releases return 404 and the
  check fails (loudly, per design). Accepted: products built from this
  template are published as public repos (only the template itself is
  private). Revisit with token support if a private-repo product ever appears.
- **Follow-ups**: release verification (checksums/signing) becomes more
  valuable once users act on update prompts — tracked as part of the
  code-signing question in the template's known-issues list (README of a
  derived product should address it before 1.0).
