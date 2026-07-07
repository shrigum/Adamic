# Architecture

## System overview

Adamic is a **Go core with a web-technology frontend, packaged with Wails v3**,
desktop only (Windows, macOS, Linux). The Go core owns document handling, text
extraction, Language Pack execution, and the local data store; the frontend
owns rendering and interaction over a defined command interface. This shape,
and the language-agnostic-core / Language-Pack split, are set by
[ADR-0005](ADR-0005-platform-stack.md) (platform/stack),
[ADR-0008](ADR-0008-local-data-storage.md) (SQLite store), and
[ADR-0011](ADR-0011-language-pack-boundary.md) (pack boundary). The full
component decomposition lives in the baselined Architecture and Design
Document; the intended shape is:

```
┌──────────────────────────────────────────────────────────────┐
│ Frontend (web tech, in the Wails webview)                     │
│   Reader View · Interaction · Lookup Panel · Overlays ·       │
│   Vocabulary/Study UI · Annotation/Editing UI · Progress ·    │
│   Shell (onboarding, settings, pack mgmt)                     │
└──────────────────────────────────────────────────────────────┘
        │  command interface (open doc, get page model, look up,
        ▼  set familiarity, export, …) — no language logic here
┌──────────────────────────────────────────────────────────────┐
│ Core (Go)                                                     │
│   Document Engine · Text Extraction & Mapping (root dep) ·    │
│   OCR Service · Language Pack Runtime · Lookup Service ·      │
│   Familiarity & Vocabulary Store · Study · Annotation ·      │
│   Document Operations · Data Store · Library Manager         │
└──────────────────────────────────────────────────────────────┘
        │ SQLite + file artifacts   │ native libs via cgo
        ▼                           ▼ (PDF engine, OCR, voices)
  <os-user-data-dir>/adamic/   Language Packs (offline bundles)
```

> **Status:** the first real feature, **PDF reader core** (REQ-1), is largely
> built — the Go core, its command interface, and the frontend logic are in
> place and tested; the live Wails desktop window is the remaining step. The
> inherited template scaffold (settings file, update check, greeting command)
> also remains live. Later features arrive through the
> [backlog](../planning/BACKLOG.md) and the planning flow.

Components landed so far (feature `pdf-reader-core`):

- `src/reader` — the **command interface** (`reader.Reader`): the stable
  core↔frontend boundary (open, page count, render page at a scale, thumbnail,
  get/set reading position, close) with its request/response types, a typed soft
  open-error shape, and an in-memory stub.
- `src/document` — the **Document Engine**: renders PDF pages via PDFium on the
  no-cgo WebAssembly backend ([ADR-0012](ADR-0012-pdf-engine.md)), with a
  virtualized, LRU-bounded render window for large documents and committed
  performance budgets. The PDF binding is confined to this package.
- `src/library` — the interim file-backed **reading-position store**
  (`Store`/`FileStore`), a narrow `Save`/`Load` seam that the SQLite store
  ([ADR-0008](ADR-0008-local-data-storage.md)) replaces later without an
  interface change.
- `src/app` — the **binding layer** the Wails shell exposes to the frontend:
  the command interface wrapped in JSON-serializable methods (page images as PNG
  data URLs, open failures as displayable results).
- `frontend/` — the framework-agnostic **viewer/navigation/zoom model** and the
  typed client for `src/app`, unit-tested under `node --test`.

Inherited scaffold (live features of Adamic, retained from the template):

- `src/` `package main` — CLI wiring only (thin entry point).
- `src/settings/` — the local settings file. **Note:** Adamic's structured user
  data (vocabulary, familiarity, SRS state) moves to SQLite per
  [ADR-0008](ADR-0008-local-data-storage.md), which supersedes the
  settings-file model in [ADR-0002](ADR-0002-settings-file-format.md) and
  [ADR-0004](ADR-0004-settings-schema-version.md); the settings file remains
  for simple preferences.
- `src/update/` — the opt-in GitHub Releases check ([ADR-0003](ADR-0003-update-check.md)),
  the only network code, checking github.com/shrigum/Adamic releases.

Rules that carry forward (see [CODING_STANDARDS.md](../CODING_STANDARDS.md#module-boundaries)):

- `package main` contains **wiring only** — anything worth testing lives in a
  subpackage.
- Domain/core packages never import the CLI/frontend layer and never print to
  stdout/stderr; they return values and errors.
- Language-specific behavior lives **only** in Language Packs behind stable
  interfaces; the core contains none ([ADR-0011](ADR-0011-language-pack-boundary.md)).

This overview is the first page of architecture a new contributor reads after
[onboarding](../onboarding/ONBOARDING.md); keep it current as components land.

## Architecture Decision Records

Every significant decision is an ADR: numbered, immutable once accepted
(supersede, don't edit), written from [ADR-TEMPLATE.md](ADR-TEMPLATE.md).
What counts as "significant" is defined in the
[Definition of Done](../process/DEFINITION_OF_DONE.md#documentation); the
[architecture-reviewer skill](../../.claude/skills/architecture-reviewer/SKILL.md)
makes the call during design review.

### Index

| ADR | Title | Status | Date |
|-----|-------|--------|------|
| [0001](ADR-0001-tech-stack.md) | Go as the implementation language | Accepted (GUI posture amended by 0005) | 2026-07-07 |
| [0002](ADR-0002-settings-file-format.md) | JSON in the OS user-config dir for settings | Accepted (amended by 0004); superseded for structured data by 0008 | 2026-07-07 |
| [0003](ADR-0003-update-check.md) | Opt-in update check via the GitHub Releases API | Accepted (reinforced by local-first) | 2026-07-07 |
| [0004](ADR-0004-settings-schema-version.md) | Settings file carries a schema version (envelope) | Accepted; superseded for structured data by 0008 | 2026-07-07 |
| [0005](ADR-0005-platform-stack.md) | Platform/stack: Go + web frontend + Wails v3, desktop only | Accepted (supersedes 0001's CLI/no-GUI posture) | 2026-07-07 |
| [0006](ADR-0006-mvp-language.md) | MVP launch language: Japanese | Accepted | 2026-07-07 |
| [0007](ADR-0007-reader-layout.md) | Faithful fixed-layout reader for MVP; reflow later | Accepted | 2026-07-07 |
| [0008](ADR-0008-local-data-storage.md) | Local data storage: SQLite; annotations as standard PDF objects | Accepted (supersedes 0002 and 0004) | 2026-07-07 |
| [0009](ADR-0009-familiarity-by-lemma.md) | Familiarity keyed by lemma | Accepted | 2026-07-07 |
| [0010](ADR-0010-spaced-repetition.md) | Spaced repetition: export-first, in-app review later | Accepted | 2026-07-07 |
| [0011](ADR-0011-language-pack-boundary.md) | Language Pack plugin boundary | Accepted | 2026-07-07 |
| [0012](ADR-0012-pdf-engine.md) | PDF rendering engine: PDFium via klippa-app/go-pdfium | Accepted (resolves A&D §4.1; refines 0005's cgo assumption) | 2026-07-07 |

*(Keep this table sorted by number. Adding a row here is part of merging any ADR —
the docs-maintainer skill checks it.)*

### Lifecycle

`Proposed` → `Accepted` | `Rejected`; later possibly `Superseded by ADR-NNNN`
(both ADRs link to each other). Rejected ADRs stay in the repo — knowing what we
decided *not* to do, and why, is half the value.
