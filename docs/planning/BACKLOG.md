# Adamic — Feature Backlog

The ordered feature queue. "What should I work on next?" is answered by the
topmost `backlog` item whose dependencies are `shipped`. Each row maps a
requirement from [docs/PRODUCT.md](../PRODUCT.md) to a feature folder under
`docs/planning/<feature-name>/`, where its spec → critical-path → design-review
→ implementation trail lives (the planning flow finds in-flight work by the
critical-path docs; this table holds the queue).

Status values: `backlog` (not started) · `in-flow` (has a spec, moving through
the planning flow) · `shipped` (delivered and verified).

## Dependency note

REQ-2 (text extraction & mapping) is the **root dependency for every language
feature** (REQ-4/5/6/7/8/9) — nothing that resolves words can be correct until
extraction produces correctly ordered Unicode mapped to page coordinates. REQ-1
(the reader) is the founder's first priority and the surface REQ-2 attaches to;
it is specced first. The critical path through a usable product is:
**REQ-2 → REQ-3 → REQ-4 → REQ-5 → REQ-7 → REQ-8** (extraction → pack runtime →
lookup → familiarity → coverage → study export), per the Project Management
Plan. Off-critical-path work (REQ-10 OCR, REQ-11 PDF productivity, REQ-12
second pack, REQ-9 aids) parallelizes once the REQ-5 data model is stable.

## Queue

| REQ | Feature (folder name) | Outcome | Status |
|---|---|---|---|
| REQ-1 | `pdf-reader-core` | Open and render a PDF in faithful fixed layout with navigation, zoom, and restored per-document reading position. | **in-flow** (spec written) |
| REQ-2 | `text-extraction-mapping` | Extract correctly ordered Unicode across LTR/RTL/vertical/unspaced scripts, mapped to on-page coordinates; correct selection and copy. *(Root dependency; highest risk — front-loaded with a Stage 0 extraction spike.)* | backlog |
| REQ-3 | `language-pack-runtime` | Load offline pack bundles via manifest; expose stable capability interfaces; disable only dependent features when a capability is absent. | backlog |
| REQ-4 | `word-lookup` | Look up a word by tap/click/selection, resolve surface form → lemma, show definition and reading via the active pack. | backlog |
| REQ-5 | `familiarity-model` | Per-(language,lemma) familiarity state with in-document coloring and single-interaction changes, persisted in SQLite. | backlog |
| REQ-6 | `vocabulary-bank` | Capture looked-up/marked words with surface form, lemma, reading, definition, source document and sentence; view/search/edit. | backlog |
| REQ-7 | `coverage-progress` | Report known-word coverage (page and document) and per-document reading progress. | backlog |
| REQ-8 | `study-export` | Mine a sentence into a study card; export cards and vocabulary to Anki `.apkg` and CSV. | backlog |
| REQ-9 | `reading-aids` | Furigana/pinyin/romanization overlays, offline TTS, grammar/morphology, and diacritization, where the active pack provides them. | backlog |
| REQ-10 | `ocr` | Detect image-only documents and OCR them into a selectable text layer, offline, with per-region review. | backlog |
| REQ-11 | `pdf-productivity` | Annotation as standard PDF objects, page organization/editing, forms, and export/conversion. | backlog |
| REQ-12 | `pack-extensibility` | Documented pack format, conformance suite, pack management, and the Latin pack added with no core changes. | backlog |
| REQ-13 | `app-shell` | First-run onboarding, settings, keyboard shortcuts, themes, accessibility, and localization structure. | backlog |

## Cleanup items (carried from kickoff)

- **Retire the greeting scaffold.** `src/main.go` still carries the template's
  `--name`/`--greeting` command. Remove it once REQ-1 (`pdf-reader-core`) lands
  a real entry point. The `settings` and `update` packages stay — they are live
  features of Adamic.
- **Adopt the Wails shell.** The current binary is a pure-Go CLI scaffold; the
  first UI-bearing feature introduces the Wails v3 desktop shell
  ([ADR-0005](../architecture/ADR-0005-platform-stack.md)). Expect the
  build/release scripts to grow a cgo/packaging path at that point (risk R-03).
- ~~Configure the update repository.~~ Done — `update.Repo` is set to
  `shrigum/Adamic` (public), so `adamic update` checks its Releases
  ([ADR-0003](../architecture/ADR-0003-update-check.md)). It reports no newer
  release until the first release is cut.
