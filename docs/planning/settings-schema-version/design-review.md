# Design review: Settings file schema version

- **Stage**: 3 — Design review ([planning flow](../../process/PLANNING_FLOW.md))
- **Reviewer**: architecture-reviewer skill
- **Inputs**: [spec.md](spec.md), [critical-path.md](critical-path.md), ADR-0002
- **Date**: 2026-07-07

## Verdict: APPROVED-WITH-CONDITIONS

All conditions satisfied before merge.

## Fit / ADR decision

Changes a persistent on-disk format → **ADR required**, and since it revises
an Accepted decision (ADR-0002's layout), the record must link both ways
rather than editing ADR-0002's substance.
→ [ADR-0004](../../architecture/ADR-0004-settings-schema-version.md); ADR-0002
status line now reads "Accepted (amended by ADR-0004)". **Done.**

## Complexity check

- ✔ One envelope level and a two-branch read path — the minimum that buys
  deterministic future migrations. No migration framework, no version
  registry: YAGNI still applies to machinery, just not to the marker itself.
- ✔ Rejected during review: a generic `Migrate(from, to)` interface — there
  is exactly one legacy layout; a fallback branch is the right size.

## Conditions

| # | Condition | Rationale | Status |
|---|---|---|---|
| C1 | Reads must never rewrite a legacy file; migration happens only on save. | Preserves "reads don't write" (settings-file AC2) and never touches user files without a user action. | Done — pinned by `TestLoadLegacyFlatFileAndMigrateOnSave`. |
| C2 | Future-version files are an error, never best-effort parsed. | Misreading a newer layout is silent data corruption; loud failure is the only safe behavior. | Done — pinned by `TestLoadNewerSchemaVersionErrors`. |
