# Adamic — Product Brief

The one document every spec traces back to. This is a *brief*, not a
specification: it states the problem, who it is for, the numbered requirements,
what is out of scope, the assumptions this project runs on, and what 1.0 means.
Acceptance criteria live in per-feature specs under
[docs/planning/](planning/), not here.

The full requirements baseline (SRS), architecture, and project plan exist as a
separate baselined document set; this brief summarizes them for day-to-day work
and gives specs a stable set of `REQ-n` handles to cite.

## Problem

Language learners who want to read real books — full-length works, as PDFs — in
a foreign language have no single offline tool that combines competent PDF
reading and annotation with per-word lookup, lemma-keyed familiarity tracking,
vocabulary capture, and study export. Existing options are either
cloud-dependent, single-language, or split the reading and learning workflows
across separate apps, so the reader constantly context-switches and their data
is scattered and non-portable. Adamic keeps the whole loop — read, look up,
mark, capture, review-export — in one place and entirely on the user's device,
in open, portable formats.

## Users (personas)

- **Immersion reader** — reads a Japanese book, wants segmentation, reading aids,
  lookup, familiarity marking, sentence mining, and Anki export, then reopens
  with prior words colored by knowledge.
- **Classics student** — opens a scanned Latin source, OCRs it, looks up
  inflected forms resolved to headwords, and annotates, fully offline.
- **Heritage learner** — opens right-to-left unvocalized text, selects and
  highlights correctly, applies diacritization and transliteration, looks up by
  resolved lemma.
- **Practical newcomer** — opens an official document, fills and saves a form,
  quick-looks-up terms, captures vocabulary, and confirms nothing leaves the
  device.
- **Polyglot power user** — manages multiple languages and packs, uses
  keyboard-driven lookup, exports vocabulary, and keeps all data local and
  portable.

## Requirements

These are the user-visible capabilities, in priority order. Each becomes one or
more features in [docs/planning/BACKLOG.md](planning/BACKLOG.md); specs cite the
`REQ-n` handle. They map onto the SRS functional areas noted in parentheses.

- **REQ-1** — Open and render PDF documents in a faithful fixed layout with
  navigation, zoom, and per-document reading position (DOC + NAV core).
- **REQ-2** — Extract and select text as correctly ordered Unicode across
  LTR/RTL/vertical/unspaced scripts, mapped to page coordinates (TXT). *Root
  dependency for all language features.*
- **REQ-3** — Language Pack runtime with stable capability interfaces
  (segmenter, lemmatizer, dictionary, transliterator, TTS, grammar, script
  profile) loaded from offline bundles (LP).
- **REQ-4** — Word lookup by tap/click/selection, resolving surface form to
  lemma and showing definition and reading via the active pack (LKP).
- **REQ-5** — Per-lemma familiarity model with in-document coloring and
  single-interaction state changes, persisted locally (FAM).
- **REQ-6** — Personal vocabulary bank capturing surface form, lemma, reading,
  definition, source document, and source sentence (VOC).
- **REQ-7** — Known-word coverage and reading-progress reporting (PRG).
- **REQ-8** — Sentence mining and study export to Anki package and CSV
  (SRS, export-first per [ADR-0010](architecture/ADR-0010-spaced-repetition.md)).
- **REQ-9** — Reading aids, offline TTS, grammar/morphology, and diacritization
  overlays (AID/TTS/GRM/DIA).
- **REQ-10** — OCR for image-only documents producing a selectable text layer,
  offline (OCR).
- **REQ-11** — Annotation as standard PDF objects, page organization/editing,
  forms, and export/conversion (ANN/ORG/FRM/EXP).
- **REQ-12** — Language Pack extensibility: documented format, conformance
  suite, pack management, and a second pack (Latin) added without core changes
  (EXT).
- **REQ-13** — Application shell: first-run onboarding, settings, keyboard
  shortcuts, themes, accessibility, and localization structure (APP).

## Non-goals

- No cloud or SaaS; no hosted service to deploy.
- No account, license server, or remote service; nothing required to sign in.
- No network access beyond the **opt-in** update check
  ([ADR-0003](architecture/ADR-0003-update-check.md)); the app is fully
  functional with networking disabled.
- No default telemetry; no user documents or data leave the device.
- No tablet or mobile target for the MVP — desktop only, Windows/macOS/Linux
  ([ADR-0005](architecture/ADR-0005-platform-stack.md)).
- No in-app spaced-repetition scheduler for the MVP — study is export-first
  ([ADR-0010](architecture/ADR-0010-spaced-repetition.md)).
- No reflowed reading view in the MVP — fixed layout first, reflow later
  ([ADR-0007](architecture/ADR-0007-reader-layout.md)).

## Assumptions recorded at kickoff

Defaults applied where an input was omitted; each is a decision that can be
amended by a normal doc change, not settled fact.

- **Module path**: `github.com/shrigum/adamic` (lowercase per Go convention).
  The GitHub repository is [github.com/shrigum/Adamic](https://github.com/shrigum/Adamic),
  public; `update.Repo` is set to `shrigum/Adamic`, so `adamic update` checks
  its Releases (the unauthenticated check requires a public repo —
  [ADR-0003](architecture/ADR-0003-update-check.md)). No releases exist yet, so
  the check reports the latest as none until the first release is cut.
- **Binary / CLI name**: `adamic`. **Settings dir**: `adamic` under the OS
  user-config dir. **Env-var prefix**: `ADAMIC_` (`ADAMIC_CONFIG_DIR`,
  `ADAMIC_UPDATE_URL`).
- **Platforms**: Windows, macOS, Linux (desktop only). **License**: MIT,
  copyright secuiu stefan.
- **ADR numbering**: the template already occupies ADR-0001–0004, so Adamic's
  decisions were renumbered from the baselined document set's three-digit scheme
  to **ADR-0005–0011** on top of the template's, rather than colliding with
  ADR-0004. Crosswalk from the baselined set: platform/stack → 0005, MVP
  language → 0006, reader layout → 0007, SQLite storage → 0008, familiarity by
  lemma → 0009, spaced repetition → 0010, pack boundary → 0011.
- **Inherited scaffold**: the template's settings-file and update-check features
  are retained as live features; the greeting command in `src/main.go` is
  scaffold, queued for replacement by REQ-1 (see the backlog).
- **cgo native libraries** (PDF engine, OCR, voices) will remove the template's
  single-static-binary property and require a C toolchain plus container-based
  cross-compilation; carried as risk **R-03**, to be validated in a Stage 0
  stack spike before dependent work is scheduled.

## What "1.0" means

A reader can open a full-length book PDF in a supported language and complete the
**core reading loop entirely offline**: read in a faithful layout, look up a word
resolved to its lemma with definition and reading, mark its familiarity in one
interaction, capture it to the vocabulary bank with its source sentence, see
known-word coverage, and export mined sentences to Anki/CSV. Annotated and
form-filled documents remain valid in other PDF readers. A **second language
(Latin) is demonstrably added as a self-contained Language Pack with no core
changes**, proving the extensibility architecture. All user data is stored
locally in documented open formats and is exportable without loss. The product
ships as versioned local builds published as attached release artifacts.

The requirement baseline is delivered across sequential stages (foundation →
language core → study & aids → PDF productivity → extensibility → full
verification); the backlog reflects that ordering.
