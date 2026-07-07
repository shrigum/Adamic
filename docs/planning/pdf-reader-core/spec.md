# Spec: PDF reader core

- **Stage**: 1 — Intake ([planning flow](../../process/PLANNING_FLOW.md))
- **Author**: spec-writer skill, from REQ-1 (docs/PRODUCT.md) at project kickoff
- **Date**: 2026-07-07
- **Status**: Draft — ready for stage 2 (critical-path-planner)
- **Requirement**: REQ-1 (SRS areas DOC + NAV core: FR-DOC-01/02, FR-NAV-01..05)

## Problem

Adamic cannot yet open or show a document — the whole product presupposes a
reader, and every later feature (text extraction, lookup, familiarity coloring,
annotation) attaches to a rendered page. A reader must open a book-length PDF
from the local disk, show it faithfully as it was laid out on the page, let the
user move through it (scroll or page-by-page, zoom, jump to a page, use
thumbnails), and — because people read books over many sittings — reopen it
exactly where they left off. It must do this fully offline, stay responsive on a
500-page book, and never crash on a broken file.

This feature also establishes two foundations everything else depends on: the
**Wails desktop shell** (the first GUI surface, per
[ADR-0005](../../architecture/ADR-0005-platform-stack.md)) and the
**core/frontend command interface** — the boundary across which the web frontend
asks the Go core for page images and document facts and holds no PDF logic
itself.

## Non-goals

Each is a thing a reasonable reader would expect, fenced out here with the
requirement/feature that owns it:

- **No text extraction, selection, or copy.** That is REQ-2
  (`text-extraction-mapping`), the root language dependency; this feature
  renders pages as images and knows nothing about their text. A page here is a
  picture, not characters.
- **No in-document search** (FR-NAV-08) — depends on text extraction (REQ-2).
- **No bookmarks or document outline/TOC** (FR-NAV-06/07) — later NAV feature.
- **No reading themes / dark mode** (FR-NAV-09) — later NAV feature.
- **No right-to-left or vertical reading order** (FR-NAV-10). Page *rendering*
  is faithful regardless (the engine draws the page as-is); what is deferred is
  *navigation/scroll direction* semantics for RTL/vertical documents. Revisit
  with REQ-2/I18N work.
- **No reflowed view** (FR-NAV-11) — deferred by
  [ADR-0007](../../architecture/ADR-0007-reader-layout.md).
- **No multi-document tabs, recents, collections, drag-and-drop import, or
  language detection** (FR-DOC-03..08). This feature opens **one** document at a
  time and stores the **minimum** library metadata needed to restore position
  (see A4); the full Library Manager is later DOC work.
- **No annotation, forms, OCR, page editing** — separate requirements
  (REQ-10/11).
- **No printing or export.**

## Constraints

- **Stack**: Go core + web frontend + Wails v3, desktop only
  ([ADR-0005](../../architecture/ADR-0005-platform-stack.md)). Language logic
  and PDF logic live in the Go core; the frontend renders and interacts over the
  command interface and holds neither.
- **Rendering engine is native, via cgo.** A native PDF engine (MuPDF or PDFium)
  renders page images (Architecture & Design Document §4.1). This is Adamic's
  **first cgo dependency** and triggers risk **R-03**: it removes the template's
  single-static-binary property and requires a C toolchain plus container-based
  cross-compilation for the three desktop targets. **The engine choice
  (MuPDF vs PDFium), and the cgo build/packaging path, require an ADR and must
  be settled at design review (stage 3)** before implementation — flagged here,
  not decided in this spec (it is solution space).
- **Persistence is local and offline.** Reading position and minimal document
  metadata persist locally with no network (NFR-OFFLINE-01). The intended home
  is the SQLite store
  ([ADR-0008](../../architecture/ADR-0008-local-data-storage.md)), which is
  **not yet built** — see assumption A3 for how this feature proceeds without
  blocking on it.
- **Performance** (numeric budgets are **to be set** during design after the
  Stage 0 spike — this spec references them, does not invent them):
  opening a typical book PDF becomes interactive within the startup budget
  (NFR-PERF-03); scrolling/zooming a 500-page document does not visibly degrade
  (the fixed-layout analogue of NFR-PERF-02). Rendering is therefore
  incremental/virtualized — only visible (and near-visible) pages are rendered,
  not the whole document up front.
- **Reliability**: a malformed, truncated, encrypted-without-password, or
  non-PDF file is reported as a clear user-facing error and never crashes the
  app (NFR-REL-02).
- **Local-first / privacy**: no network path anywhere in this feature
  (NFR-OFFLINE-01, NFR-SEC-01); no telemetry.

## Assumptions

Ambiguities resolved as explicit, overridable assumptions (amend this spec to
change one). Low-confidence items are flagged for confirmation before stage 4.

- **A1** — "Faithful fixed layout" means the frontend displays **page images
  rendered by the native engine** at a requested scale; the frontend does not
  re-lay-out or re-flow content. Fidelity is the engine's raster of the page.
- **A2** — "Reading position" is stored as **{page index, and a within-page
  scroll offset or zoom/fit state}** sufficient to return the viewport to where
  the user was, per document. In continuous-scroll mode it resolves to the
  top-most visible page plus offset. *(low confidence — confirm the exact
  position model before stage 4; it affects the data shape and AC7.)*
- **A3** — Because the SQLite store (ADR-0008) does not exist yet, this feature
  **defines a narrow persistence interface** (`SaveReadingPosition` /
  `LoadReadingPosition` / minimal document record keyed by a stable document
  identity) and provides a **local file-backed implementation** it owns, to be
  swapped for the SQLite-backed store when `data-store` lands — with **no change
  to the interface or acceptance criteria**. This keeps REQ-1 unblocked and is
  consistent with ADR-0008 (the store is the destination, not a prerequisite).
  *(low confidence — confirm this bridge vs. building a minimal SQLite slice now,
  at design review.)*
- **A4** — Stable **document identity** for the position key is the **absolute
  file path plus a content hash** (so a moved file still restores, and two files
  at the same path but different content do not collide). Metadata stored is the
  minimum for REQ-1: identity, page count, last reading position, last-opened
  timestamp. Title/author/language and the rest of FR-DOC-02 are deferred to the
  Library Manager.
- **A5** — **Zoom** offers at least fit-to-width, fit-to-page, and discrete
  zoom steps (e.g. 50%–400%); the exact step set is a design detail. Fit modes
  recompute on window resize.
- **A6** — The **thumbnail panel** renders low-resolution page thumbnails
  lazily (visible thumbnails first) and is toggleable; clicking a thumbnail
  navigates to that page.
- **A7** — Opening a **password-protected** PDF is treated as a graceful error
  in this feature (reported, not crashed); actually prompting for and applying a
  password is later work (FR-EXP-04 / forms area). *(overridable if a minimal
  password prompt is wanted in REQ-1.)*
- **A8** — Single desktop user, one document open at a time (multi-tab is
  FR-DOC-03, out of scope). The position store is single-writer.

## Acceptance criteria

Each is observable and testable; the covering test is filled in at close-out
(Definition of Done). Where a criterion depends on a numeric budget still to be
set at design, it is written against the budget symbolically.

| # | Criterion | Covering test |
|---|---|---|
| AC1 | Opening a valid multi-page PDF from a local path renders page 1 faithfully (engine raster) and reports the correct total page count. | |
| AC2 | Single-page mode shows exactly one page and next/previous move by one page, clamped at first/last (no wrap, no out-of-range). | |
| AC3 | Continuous-scroll mode renders pages in order and scrolls smoothly across the whole document; only visible/near-visible pages are rendered (verified by rendered-page count staying bounded on a 500-page fixture, not = 500). | |
| AC4 | Zoom supports fit-to-width, fit-to-page, and explicit zoom levels; fit modes recompute correctly after a window resize. | |
| AC5 | "Go to page N" navigates to page N for valid N; an out-of-range N is rejected with a user-visible message and no navigation. | |
| AC6 | The thumbnail panel shows thumbnails for the document and clicking thumbnail N navigates to page N. | |
| AC7 | Closing a document at page N (with a given scroll/zoom per A2) and reopening the same document restores the viewport to that position; a never-opened document opens at page 1. | |
| AC8 | Reading position and minimal metadata persist across full app restarts (process exit), with no network access at any point. | |
| AC9 | A corrupt/truncated PDF, a non-PDF file, and a missing file each produce a distinct, clear user-facing error naming the problem, and the app remains running and usable (no crash). | |
| AC10 | A password-protected PDF is reported as such (graceful error per A7) without crashing. | |
| AC11 | Opening a typical book PDF becomes interactive within the startup budget (NFR-PERF-03, budget set at design); scrolling a 500-page document stays within the responsiveness budget. Test asserts against the configured budget constant. | |
| AC12 | No code path in this feature performs network I/O (inspection/test): the reader works fully with networking disabled. | |

**Error behavior summary** (per
[CODING_STANDARDS.md](../../CODING_STANDARDS.md#error-handling)): a missing file,
non-PDF file, corrupt/truncated PDF, or password-protected PDF are **soft
user-facing errors** — reported clearly, app stays up. A failure to read/write
the position store is a soft error that must not lose the document view (open
still succeeds; position simply isn't restored/saved, with a logged reason).
Programmer errors (e.g. calling render before open) are loud.

## Open questions for design review (stage 3)

Recorded so critical-path-planner and architecture-reviewer pick them up; none
block stage 2:

1. **PDF engine ADR**: MuPDF vs PDFium, and the cgo build/cross-compilation
   strategy (R-03). Licensing (NFR-LIC-01) is part of this call — MuPDF is
   AGPL/commercial, PDFium is BSD-style; this materially affects the decision.
2. **Persistence bridge (A3)**: file-backed interim store vs. a minimal SQLite
   slice now. Interacts with when `data-store` (ADR-0008) is scheduled.
3. **Position model (A2)**: exact fields for a fixed-layout continuous-scroll
   viewport.
4. **Command interface shape**: the stable set of core→frontend commands this
   feature introduces (open, page count, render page at scale, thumbnails, get/set
   position) — kept stable per ADR-0005.

## Revision history

- 2026-07-07 — Initial version, written at kickoff from REQ-1. Ready for stage 2.
