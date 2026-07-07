# ADR-0004: Settings file carries a schema version (envelope layout)

- **Status**: Accepted (amends [ADR-0002](ADR-0002-settings-file-format.md)). Superseded for structured user data by [ADR-0008](ADR-0008-local-data-storage.md): the file-based schema-version model applies only to the simple preferences file; Adamic's structured data uses SQLite with its own versioned schema and migrations.
- **Date**: 2026-07-07
- **Feature/trigger**: template hardening review — "no settings schema version"
  flagged as a cheap-now/impossible-later defect before real users exist.
  Planning trail: [docs/planning/settings-schema-version/](../planning/settings-schema-version/)
- **Deciders**: project maintainers

## Context

ADR-0002 chose flat JSON (`{"greeting": "Hey"}`) for the settings file and
noted revisit conditions (nesting, typed values). What it did not provide is a
way for future code to **identify which layout a given file uses**. The moment
a layout change becomes necessary, migration code must guess a file's vintage
from its shape — fragile, and impossible to do confidently once files that
were hand-edited by users exist in the wild. On-disk formats are part of the
SemVer-major surface, so this defect compounds with every release shipped.

The template has shipped no real user data yet (v0.1.0 is the template's own
example), making this the cheapest possible moment to fix it.

## Decision

The settings file becomes a versioned envelope:

```json
{
  "schemaVersion": 1,
  "settings": { "greeting": "Hey" }
}
```

Rules, implemented in `src/settings/`:
- Save always writes the current schema version.
- A file **without** `schemaVersion` is read as the legacy flat map
  (version 0) and is rewritten in the current layout on its next save —
  reads never modify the file.
- A file with a schema version **greater** than the build understands is a
  user-facing error ("written by a newer version of the app"), never
  reinterpreted, downgraded, or reset.
- Bumping `schemaVersion` in the future requires a migration path from every
  older version and is a MAJOR-relevant change per the release process.

Everything else in ADR-0002 (JSON, string map, OS config dir, atomic writes,
corrupt-file behavior, unknown-key preservation — now scoped to the inner
`settings` map) stands unchanged.

## Alternatives considered

### Reserved key in the flat map (`"schema_version": "1"` alongside settings)
Advantage: no layout change at all. Lost because it pollutes the settings
namespace (must be filtered from `list`, blocked from `set`, special-cased
everywhere) — more ongoing complexity than one envelope level, in exchange
for avoiding a one-time change made while no user files exist.

### Version in the filename (`settings.v1.json`)
Advantage: version visible without opening the file. Lost because it turns
every future migration into a multi-file dance (which file wins if several
exist?), breaks the "one obvious file" property `config path` relies on, and
encodes metadata in a place hand-editors won't maintain.

### Do nothing until a migration is actually needed
Advantage: zero work now; YAGNI on its face. Lost because this is the
known exception to YAGNI: version markers must exist **before** the files
they'll need to distinguish. Deferring makes the eventual migration guess
at file vintage — the exact failure this ADR exists to prevent.

## Consequences

- **Positive**: any future layout change is a deterministic
  `switch schemaVersion` migration; files written by a newer app version fail
  loudly instead of being misread; the envelope is where nesting/typed values
  (ADR-0002's revisit conditions) can later live without another layout break.
- **Negative**: the file is one level deeper for hand-editors; legacy-file
  reading code exists essentially only for pre-template-1.0 files (small,
  tested, cheap to keep). Products instantiated from the template start at
  version 1 with no legacy path ever exercised.
- **Follow-ups**: none. ADR-0002's status line now points here.
