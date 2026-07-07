# Spec: Settings file schema version

- **Stage**: 1 — Intake ([planning flow](../../process/PLANNING_FLOW.md))
- **Author**: spec-writer skill, from template hardening review
- **Date**: 2026-07-07
- **Status**: Implemented (Unreleased)

*A deliberately small spec for a small change — the flow scales down
(CONTRIBUTING.md, proportionality).*

## Problem

`settings.json` has no version marker, so future layout changes would have to
guess a file's vintage from its shape. Version markers must exist before the
files they'll need to distinguish; the template has no real user files yet, so
this is the cheapest moment it will ever be.

## Non-goals

- No change to what settings *are* (flat string map, ADR-0002 semantics).
- No migration tooling beyond legacy-file reading — there is only one old
  layout, and it migrates transparently on save.

## Constraints

- Same file, same location, zero dependencies; atomic-write, corrupt-file,
  and unknown-key guarantees of the settings-file feature must all still hold
  (its test suite is the regression net).

## Assumptions

- **A1**: Version 0 = the legacy bare flat map (absence of `schemaVersion`);
  version 1 = the envelope. No other layouts exist anywhere.

## Acceptance criteria

| # | Criterion | Covering test |
|---|---|---|
| AC1 | Every save writes `schemaVersion: 1` in the envelope layout. | `src/settings: TestSaveWritesVersionedEnvelope` |
| AC2 | A legacy flat-map file is read correctly; reads do not modify it; the next save rewrites it as a v1 envelope with all keys (known and unknown) intact. | `src/settings: TestLoadLegacyFlatFileAndMigrateOnSave` |
| AC3 | A file with `schemaVersion` greater than the build understands is a user-facing error naming the path and cause; the file is left intact. | `src/settings: TestLoadNewerSchemaVersionErrors` |
| AC4 | All prior settings-file acceptance criteria (AC1–AC8 of [settings-file/spec.md](../settings-file/spec.md)) still pass unchanged. | existing `src/settings` + `tests/` suites |

## Revision history

- 2026-07-07 — Initial version accepted into stage 2.
