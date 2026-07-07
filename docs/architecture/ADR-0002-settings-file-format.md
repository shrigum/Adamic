# ADR-0002: JSON in the OS user-config dir for settings

- **Status**: Accepted (amended by [ADR-0004](ADR-0004-settings-schema-version.md): the file now carries a schema version in an envelope layout). Superseded for structured user data by [ADR-0008](ADR-0008-local-data-storage.md), which moves Adamic's relational, query-driven data (vocabulary, familiarity, SRS state) to SQLite; the settings file remains for simple preferences.
- **Date**: 2026-07-07
- **Feature/trigger**: [docs/planning/settings-file/](../planning/settings-file/)
- **Deciders**: project maintainers

## Context

The settings-file feature ([spec](../planning/settings-file/spec.md)) needs a
persistent, user-editable store for app preferences. Constraints from the spec
and project principles:

- **Local-first**: a plain file the user owns, can read, edit, back up, and
  delete; no database, no registry, no cloud sync.
- **Zero/minimal dependencies** (ADR-0001's stdlib-first policy).
- **Hand-repairable**: a user with a text editor must be able to fix or reset it;
  error messages must be able to point at what's wrong.
- **Format is forever-ish**: the on-disk format is part of the SemVer-major
  surface ([RELEASE_PROCESS.md](../process/RELEASE_PROCESS.md#versioning)), so
  changing it later is expensive. This is why the decision rates an ADR despite
  the feature being small.

The location question is bundled in because format and location together define
the persistence contract.

## Decision

Settings are stored as **pretty-printed JSON with string keys and string values**
in `<UserConfigDir>/app/settings.json`, where `<UserConfigDir>` is Go's
`os.UserConfigDir()` (`%AppData%` on Windows, `~/Library/Application Support` on
macOS, `$XDG_CONFIG_HOME` or `~/.config` on Linux). Writes are atomic
(write temp file in the same directory, then rename). Unknown keys are preserved
on read-modify-write. A corrupt file is a **user-facing error naming the path**,
never silent reset — silently discarding a user's hand-edited file is data loss.

## Alternatives considered

### TOML
Advantages: nicer to hand-edit (comments!, no trailing-comma traps), the de facto
config standard in modern CLI tools. Lost because it requires a third-party
dependency (`BurntSushi/toml`) for a flat string-to-string map that JSON handles
with zero dependencies. **Revisit if** settings grow nesting or want user-facing
comments — that's the tipping point where TOML's ergonomics outweigh one
well-vetted dependency.

### INI / custom key=value
Advantages: trivially hand-editable, trivially parseable. Lost because: no
standard grammar (escaping, encoding, and multiline values are all ad hoc), so
we'd own a homegrown parser forever — worse than either real option.

### SQLite
Advantages: transactional, scales to real data, single file. Lost because: not
hand-editable or diffable, requires cgo or a driver dependency, and is wildly
over-scaled for a handful of preferences. Revisit only if the app grows actual
structured data — and that would be its own feature and ADR, not a settings change.

### Environment variables / flags only (do nothing)
Advantages: zero persistence code. Lost because the spec's core requirement is
persistence across runs without per-invocation ceremony; env vars push the
persistence problem onto the user's shell profile, which is hostile to
non-technical users and unmanageable on Windows.

## Consequences

- **Positive**: zero dependencies; `encoding/json` is as battle-tested as
  software gets; users can inspect/fix/reset with any editor; atomic writes mean
  a crash can't half-write the file; `app config path` gives support a one-liner.
- **Negative**: JSON has no comments, so we can't annotate the file for users
  (mitigated: `app config list` documents live keys); string-only values push
  type interpretation to the reading code (acceptable at current scale; the
  schema constant in [src/settings/settings.go](../../src/settings/settings.go)
  is the single place it's handled).
- **Follow-ups**: none open. The revisit conditions above are the watch-list.
