# Spec: Local settings file

- **Stage**: 1 — Intake ([planning flow](../../process/PLANNING_FLOW.md))
- **Author**: spec-writer skill, from the request "the app should remember my preferences"
- **Date**: 2026-07-07
- **Status**: Implemented (v0.1.0)

## Problem

The app forgets everything between runs. A user who prefers a different greeting
word must pass `--greeting` on every invocation. There is no persistent,
user-controlled place for preferences, and every future preference-shaped
feature would reinvent one. We need a single settings mechanism that persists
across runs, survives app upgrades, and stays inspectable and repairable by the
user (local-first principle).

## Non-goals

- **No sync** between machines (would violate local-first with a cloud
  dependency; a future *optional* sync would be its own feature and ADR).
- **No secrets storage.** Settings are plaintext preferences. Anything
  credential-shaped is out of scope and should use the OS keychain (future
  feature if ever needed).
- **No structured/nested settings.** Flat string keys → string values only.
  Revisit condition recorded in [ADR-0002](../../architecture/ADR-0002-settings-file-format.md).
- **No settings UI/TUI.** CLI subcommands and a text editor are the interfaces.
- **No live-reload.** Settings are read at startup; changes apply next run.

## Constraints

- Zero third-party dependencies (ADR-0001 stdlib-first policy).
- File must live in the platform-conventional user config location, not the
  working directory or a dotfile in `$HOME` root.
- On-disk format becomes part of the SemVer-major surface — choose accordingly
  (this constraint is what triggered ADR-0002 at design review).
- A partially-written or corrupt file must never be mistaken for valid settings,
  and a crash mid-write must never destroy the previous good file.

## Assumptions

Ambiguities in the original request, resolved as explicit assumptions (per the
spec-writer skill's rules — each is overridable by amending this spec):

- **A1**: Single settings file per OS user; no profiles/workspaces. The request
  mentioned only "my preferences".
- **A2**: Unknown keys in the file are preserved, not rejected — a newer version
  of the app (or the user) may have written keys this version doesn't know.
  Reading tolerates; `set` on an unknown key is allowed for the same reason.
- **A3**: Initial recognized key is `greeting` (default `"Hello"`), consumed by
  the existing greet command. The mechanism, not the key list, is the feature.
- **A4**: Values are not validated beyond being strings; consuming code owns
  interpretation. Keeps the settings package generic.

## Acceptance criteria

Each criterion is testable; the covering test is filled in at close-out
(Definition of Done).

| # | Criterion | Covering test |
|---|---|---|
| AC1 | `app config set greeting Hey` then `app --name World` prints `Hey, World!` — settings persist across process invocations. | `tests/cli_test.go: TestSettingsPersistAcrossRuns` |
| AC2 | With no settings file present, the app runs normally with defaults and does **not** create the file until first `set`. | `src/settings: TestLoadMissingFileReturnsDefaults`, `tests: TestNoFileCreatedOnRead` |
| AC3 | `app config get <key>` prints the stored value, or the default for a known-but-unset key, and exits 0; exits 1 with a clear message for an unknown, unset key. | `tests/cli_test.go: TestConfigGet` |
| AC4 | `app config list` prints all effective settings (defaults overlaid with stored values), one `key=value` per line, sorted. | `tests/cli_test.go: TestConfigList` |
| AC5 | `app config path` prints the absolute settings-file path (so users can find/edit/delete it) whether or not the file exists. | `tests/cli_test.go: TestConfigPath` |
| AC6 | Corrupt settings file → any command that needs settings exits 1 with a message naming the path and suggesting fix-or-delete. It is never silently overwritten or reset. | `src/settings: TestLoadCorruptFile`, `tests: TestCorruptFileErrors` |
| AC7 | Writes are atomic: interrupting a save cannot leave a truncated/mixed file; the previous file remains intact on any save error. | `src/settings: TestSaveIsAtomic` |
| AC8 | Unknown keys already in the file survive a `set` of a different key (read-modify-write preserves them). | `src/settings: TestSetPreservesUnknownKeys` |

**Error behavior summary** (per [CODING_STANDARDS.md](../../CODING_STANDARDS.md#error-handling)):
missing file = normal (defaults); unreadable/corrupt file = loud user-facing
error with path; failed write = error, previous file intact.

## Revision history

- 2026-07-07 — Initial version accepted into stage 2.
