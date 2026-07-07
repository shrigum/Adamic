# Design review: Local settings file

- **Stage**: 3 — Design review ([planning flow](../../process/PLANNING_FLOW.md))
- **Reviewer**: architecture-reviewer skill
- **Inputs**: [spec.md](spec.md), [critical-path.md](critical-path.md),
  [docs/architecture/README.md](../../architecture/README.md), ADR index
- **Date**: 2026-07-07

## Verdict: APPROVED-WITH-CONDITIONS

All conditions were satisfied before implementation began (statuses below).

## Fit with existing architecture

- New `src/settings/` package respects the established shape: domain logic in a
  subpackage, no imports from `main`, no printing — returns values and errors.
  ✔ Consistent with [system overview](../../architecture/README.md#system-overview).
- State as a plain user-owned file matches the local-first rule. ✔
- Zero new dependencies. ✔ (ADR-0001 policy.)

## ADR decision

**An ADR is required.** Trigger: the design introduces a **persistent on-disk
format**, which becomes part of the SemVer-major surface — squarely inside the
"significant decision" definition in the
[Definition of Done](../../process/DEFINITION_OF_DONE.md#documentation).
The format choice (JSON vs TOML vs INI vs SQLite) and location choice are the
decisions to record.

→ Produced: [ADR-0002](../../architecture/ADR-0002-settings-file-format.md). **Done.**

## Complexity check (simplest correct design?)

- ✔ Flat `map[string]string` — no schema structs, no per-key types. Matches
  spec assumption A4; anything richer would be speculative.
- ✔ No settings-watcher, no caching layer, no interface abstraction over
  "storage backends". One concrete implementation; rule of three applies.
- ⚠ Condition C2 (below) trimmed an over-design found in the draft plan: an
  originally proposed `Settings` type with getter methods per key was rejected
  as premature abstraction — callers use plain map access with defaults applied
  at load.

## Conditions

| # | Condition | Rationale | Status |
|---|---|---|---|
| C1 | The T4 Windows-rename risk must be spiked **before** T4 is considered started, since it can invalidate the atomicity design. | High risk on the critical path → build first ([rigor rule](../../CRITICAL_PATH_METHOD.md#rigor-rule)). | Done — spike result recorded in critical-path.md; risk retired. |
| C2 | Drop the per-key getter API; expose `Load() (map[string]string, error)`, `Set`, `Path`, `Defaults`. | Premature abstraction; no second consumer exists. | Done — reflected in implementation. |
| C3 | Config-dir override must be an env var (`APP_CONFIG_DIR`), not a test-only build flag, and must be documented. | Also serves portable installs; test-only hooks in prod code are a smell. | Done — implemented and documented in code + README. |

## Notes for the implementer

- Corrupt-file error text is user-facing: include the absolute path and the
  fix-or-delete suggestion verbatim from AC6.
- Keep `main.go` dispatch flat (`switch` on subcommand) — do not introduce a CLI
  framework for four subcommands.
