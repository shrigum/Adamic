# ADR-0008: Local data storage — SQLite; annotations as standard PDF objects

- **Status**: Accepted (supersedes [ADR-0002](ADR-0002-settings-file-format.md) and [ADR-0004](ADR-0004-settings-schema-version.md) for structured user data)
- **Date**: 2026-07-07
- **Feature/trigger**: project kickoff; baselined Architecture and Design Document §6
- **Deciders**: project maintainers (technical lead)

## Context

Adamic stores per-user structured data — vocabulary, familiarity state,
spaced-repetition state, library metadata, and reading positions — plus
per-document annotations. This data is relational and query-driven: computing
known-word coverage and scheduling reviews require queries (joins, aggregates,
filters by language/state/document) that the template's preferences-file model
([ADR-0002](ADR-0002-settings-file-format.md), a flat JSON map with a schema
envelope from [ADR-0004](ADR-0004-settings-schema-version.md)) cannot serve. The
product is local-first, so the store must be a local, open, portable format the
user owns, with no server.

## Decision

We use **SQLite** as the single local structured store for user data, and we
store **annotations as standard PDF annotation objects written back into the
document** where representable, with Adamic-specific annotation metadata
referenced in SQLite. SQLite is a single portable open-format file, serves the
required relational queries, and is trivially backed up with the user's own
tooling. Writing annotations into the PDF keeps annotated and form-filled
documents usable in other readers (NFR-PORT-02). The schema is documented and
versioned, with forward-compatible migrations from the first release.

## Alternatives considered

### Preferences file only (ADR-0002 as written)
Simplest, human-readable, already implemented. It cannot serve the relational,
queried data that coverage and review scheduling require. Retained only as a
simple-preferences store and an export format — not the structured store. This
is the reason ADR-0002/0004 are superseded here for structured data.

### Cloud/hosted store
Would offload sync and backup. Rejected as directly contrary to the local-first
constraint (NFR-OFFLINE, NFR-SEC): no user data leaves the device.

## Consequences

- **Positive**: relational queries for coverage and scheduling are native;
  one portable file the user can back up and move between platforms
  (NFR-XPLAT-02); annotations round-trip in other PDF readers.
- **Negative**: a documented, versioned schema with forward-compatible
  migrations is required from the first release (FR-DAT-05, NFR-REL-01); the
  annotation layer must map Adamic annotation types onto standard PDF objects
  and degrade predictably for types PDF cannot represent (risk R-05).
  Documented export to CSV, JSON, and Anki is required so users retain
  ownership (FR-DAT-03, NFR-PORT-01). **Revisit if** a future capability needs
  data that does not fit a single-file relational model.
- **Follow-ups**: schema + migration definitions as versioned configuration
  items; export paths (CSV/JSON/Anki); annotation round-trip testing in
  independent readers during Stage 3.
