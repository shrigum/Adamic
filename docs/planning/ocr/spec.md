# Spec: OCR text layer

- **Stage**: 1 — Intake ([planning flow](../../process/PLANNING_FLOW.md))
- **Author**: spec-writer skill, from REQ-10 (docs/PRODUCT.md), pulled ahead of
  REQ-2 per the [backlog](../BACKLOG.md) reordering decision (2026-07-07)
- **Date**: 2026-07-07
- **Status**: Draft — ready for stage 2 (critical-path-planner)
- **Requirement**: REQ-10 (SRS area OCR)

## Problem

Many of the books a language learner actually wants to read are **scanned
image-only PDFs**: every page is a picture with no underlying text. The reader
(REQ-1) can display them, but nothing else can work — you cannot select a word,
look it up, track familiarity, or build vocabulary, because there is no text.
The project's own test fixture,
[taalcompleet-a1-sample.pdf](../../../src/document/testdata/taalcompleet-a1-sample.pdf),
is exactly this: a scanned Dutch A1 coursebook whose pages carry an image and an
empty text layer.

So the whole language-learning layer is blocked on a precondition: **turn the
pixels of a scanned page into recognized text positioned on the page**. That is
this feature. It runs optical character recognition on page images, offline, and
attaches the recognized text — each unit tied to the on-page rectangle it came
from — to the document, so that later features (text selection/mapping in REQ-2,
then lookup/familiarity/vocabulary) have something correct to attach to. It also
lets the user **review** what OCR produced, because OCR is never perfect and a
wrong reading would silently poison every downstream feature.

## Non-goals

Each is something a reasonable person might expect from "OCR"; fenced out here
with the requirement/feature that owns it or the reason it is deferred.

- **No text selection, ordering, copy, or search.** This feature *produces* the
  text-plus-boxes layer; **consuming** it — correct reading order across
  LTR/RTL/vertical/unspaced scripts, selection, and copy — is **REQ-2**
  (`text-extraction-mapping`). The OCR/REQ-2 boundary is stated precisely in
  Constraints and A2. OCR emits units with boxes; it does not guarantee global
  reading order.
- **No word lookup, lemmatization, familiarity, or vocabulary.** Those are
  REQ-4/5/6 and attach to the text layer *after* REQ-2 maps it. OCR knows
  nothing about words as language.
- **No layout analysis beyond what OCR needs to place text** — no detection of
  columns, tables, figures, captions, or headings as semantic structures. If the
  OCR engine returns line/block groupings we keep them as opaque hints, but this
  feature does not build a document-structure model (later work if ever needed).
- **No editing of the original PDF.** OCR output is stored *alongside* the
  document (A4), not written back into the PDF as a text layer inside the file.
  Writing a standards-compliant invisible text layer into the PDF itself is a
  possible later enhancement tied to REQ-11 (annotation/export as PDF objects);
  it is out of scope here and called out as revisitable.
- **No handwriting recognition.** Printed text only. Handwriting is a different
  model class and accuracy problem; revisit only if a requirement demands it.
- **No automatic per-word correction, spell-check, or dictionary snapping** of
  OCR output. Review is manual (A6). Auto-correction needs language resources
  that live behind the Language Pack boundary and is not in this feature.
  *Clarified 2026-07-18*: suppressing obviously **non-textual junk units**
  (icon/graphic misreads) by confidence/charset/geometry (A11) is *filtering*,
  not correction, and is in scope; changing a unit's recognized *text* is not.
- **No semantic understanding of exercise structure.** The fixture contains
  letter-scramble exercises (single spaced glyphs to be reordered into a word);
  recognizing that a region is "letters, not words" is document-structure
  semantics (see the layout-analysis non-goal) and is out. Mitigation instead
  (A11): such glyphs come through as ordinary single-character units with the
  engine's (typically low) confidence, the unit filter does not delete real
  glyphs, and the per-region review (A6) is the correction path. Revisit only
  if reviewing scrambles proves too noisy in practice.
- **No cloud/hosted OCR.** Non-negotiable: fully offline (NFR-OFFLINE-01). An
  online engine is not an alternative to be weighed at stage 3.
- **No translation.** Out of product scope entirely.

## Constraints

- **Fully offline.** No network path anywhere in OCR — model files and engine
  are bundled or installed locally; recognition runs on-device (NFR-OFFLINE-01,
  NFR-SEC-01). This is a hard filter on the engine choice, not a preference.
- **License-compatible components only.** Adamic is MIT; any bundled OCR engine
  and any bundled model/training data must use licenses compatible with MIT and
  with free redistribution (NFR-LIC-01). **The OCR engine choice is
  license-load-bearing and is an ADR at stage 3** — flagged here, not decided in
  this spec (it is solution space). Note that model/weights licenses are a
  *separate* license surface from the engine code and both must clear NFR-LIC-01.
- **cgo / single-binary tension (R-03).** The Document Engine is deliberately
  **no-cgo** today: PDFium runs on a WebAssembly backend, preserving a single
  static binary with no C toolchain
  ([ADR-0012](../../architecture/ADR-0012-pdf-engine.md)). Common OCR engines
  (e.g. Tesseract via bindings) are **native/cgo**, which would reintroduce
  R-03 (C toolchain, per-platform cross-compilation, loss of the single-binary
  property). Whether OCR is allowed to reintroduce cgo, or must stay no-cgo
  (pure-Go / wasm / subprocess engine), is a **decision for the stage-3 ADR**;
  the spec records it as a first-order constraint the engine choice must weigh,
  not as decided.
- **Runs on the existing rendered page image.** OCR input is a rasterized page
  the Document Engine already produces (`document` renders pages to images).
  This feature does not add a second PDF/image decoder; it consumes the engine's
  page raster at a resolution suitable for recognition (A7).
- **Coordinate model matches the page.** Recognized units carry bounding boxes
  in **PDF page coordinates (points, origin per the page's coordinate system)**,
  not pixels of a particular render, so a box is valid at any zoom and is the
  same coordinate space REQ-2 and the reader already use (A2, A3). The engine's
  per-page point size is already available (`reader.PageSize`).
- **Local, single-user, durable storage.** OCR output persists locally, keyed to
  the document, and survives restarts (NFR-OFFLINE-01, local-first). Structured
  local data's intended home is SQLite
  ([ADR-0008](../../architecture/ADR-0008-local-data-storage.md)); as with the
  reading-position store, this feature may use an interim file-backed store
  behind a narrow interface if the SQLite store is not yet built (A4).
- **Must not crash on bad input** (NFR-REL-02): a page OCR cannot process, a
  corrupt image, an unsupported script, or an engine failure is a reported,
  recoverable condition — the document still opens and reads (OCR is additive).
- **Performance is bounded and cancellable.** OCR is expensive; recognizing a
  whole book must not freeze the app or block reading. Numeric budgets are set
  at design after a measurement spike (A8), as with the reader's budgets.
- **Bounded install size and a modest hardware floor.** Adamic is a desktop app
  distributed as a GitHub Release; the current binary is ~22 MB. An OCR engine
  that adds gigabytes of runtime/model weights or **requires a GPU** to be usable
  changes what machine can run the product. The MVP OCR engine must run
  acceptably on a **typical CPU-only laptop** with a bundle size a desktop user
  will tolerate. A heavier, higher-accuracy engine (e.g. a VLM) is acceptable
  only as an **optional** backend, not the baseline. This constraint is a
  first-order input to the stage-3 engine ADR (see open question 1).

## Assumptions

Ambiguities resolved as explicit, overridable assumptions (amend this spec to
change one). Low-confidence items are flagged for confirmation before stage 4.

- **A1 — Scope is Latin-script printed OCR (Dutch), confirmed as the MVP
  language by [ADR-0013](../../architecture/ADR-0013-mvp-language-dutch.md)
  (which supersedes ADR-0006's Japanese choice).** The engine must be architected
  so additional scripts (incl. Japanese/CJK) can be added later without redesign,
  but CJK OCR (different models, vertical text, no word spaces) is **out of scope
  for this feature** and comes with a later, harder Language Pack. This resolves
  what was previously the spec's biggest open scope lever: the founder chose
  accuracy on the in-hand Dutch corpus over front-loading the hardest typology.
- **A2 — OCR emits "recognized units" = {text string, bounding box, confidence,
  and the engine's line/block grouping id if any}; it does NOT own global
  reading order or selection.** Correct ordering across scripts, selection, and
  copy are REQ-2. This is the precise OCR↔REQ-2 boundary: OCR answers "what text
  is in this rectangle, and how sure are we"; REQ-2 answers "in what order do
  these units read, and what happens when the user drags a selection." A unit's
  granularity (word vs. line) follows what the engine provides; the stored model
  keeps both the text and its box so REQ-2 can re-segment if needed.
- **A3 — A document is "image-only / needs OCR" when its pages carry (near-)no
  extractable text.** Detection heuristic (exact threshold set at design): a page
  whose embedded text layer yields effectively empty text but which contains a
  full-page (or dominant) image is an OCR candidate. Born-digital pages with a
  real text layer are **not** OCR'd (that is REQ-2's input directly). Detection
  is per page, not per document, so mixed documents are handled. *(low confidence
  — confirm the detection heuristic and whether the user can force-OCR a page
  that has a poor native text layer.)*
- **A4 — OCR results persist in a narrow, document-keyed local store, file-backed
  for now, swappable for SQLite ([ADR-0008](../../architecture/ADR-0008-local-data-storage.md))
  with no interface change** — mirroring the reading-position store pattern
  ([library.FileStore](../../../src/library/library.go)). Document identity reuses
  the existing path+content-hash key (`library.Identify`) so re-opening the same
  file finds its OCR without re-running. Re-OCR is explicit (A5), not automatic.
- **A5 — OCR is triggered per document/page and its result is cached; it does not
  re-run automatically on every open.** Because OCR is expensive, once a page is
  recognized the result is stored and reused. Re-running (e.g. after the user
  is dissatisfied, or a better engine ships) is a deliberate user action.
  Whether the first OCR of a candidate document is automatic-on-open or
  user-initiated is a design detail bounded by A8's performance rules. *(low
  confidence — confirm auto-OCR-on-open vs. an explicit "Recognize text"
  action; the perf budget and UX both hinge on this.)*
- **A6 — "Per-region review" for this feature = the user can, per page, see the
  recognized text overlaid on/next to its region and correct a unit's text.**
  The reviewable/correctable unit is the recognized unit from A2 (word or line).
  Corrections are stored as user overrides alongside the OCR result (A4) and take
  precedence over the engine output; the original OCR text is retained so a
  correction can be undone. Bulk tooling, find-replace across a document, and
  re-flowing corrections are **out** (later enhancement). *(low confidence —
  confirm the correction granularity and whether review is required for the
  first release or can be a fast-follow while OCR-without-review ships first.)*
- **A7 — OCR runs on the page rendered at a recognition-appropriate resolution
  (e.g. ~300 DPI equivalent), independent of the on-screen zoom.** Recognition
  quality depends on input resolution; the display render window (REQ-1) is
  tuned for the viewport, not for OCR. This feature requests its own render
  scale from the Document Engine. Exact DPI is a design/measurement detail.
- **A8 — Numeric performance budgets (per-page recognition time ceiling; whole
  app stays responsive during OCR) are set at design after a measurement spike,
  and OCR of multiple pages is cancellable and runs off the UI thread.** As with
  the reader, this spec references budgets symbolically; it does not invent
  numbers. A book must be recognizable without freezing the reader or requiring
  the whole document be done before any page is usable.
- **A9 — A recognized-text overlay is *not* required to be visually rendered on
  the page in this feature.** Making OCR text selectable/visible over the image
  is REQ-2's job (it owns selection). This feature's user-visible surface is:
  triggering OCR, seeing progress, and reviewing/correcting per region (A6). If
  a minimal "show recognized boxes" debug overlay helps review, it is allowed but
  not an acceptance requirement. *(low confidence — confirm the minimal UI; it
  depends on A5/A6 answers.)*
- **A10 — Language/script-specific OCR models are candidates to live behind the
  Language Pack boundary ([ADR-0011](../../architecture/ADR-0011-language-pack-boundary.md)),
  but the OCR *engine/runtime* is core.** The core runs recognition; a pack may
  supply the model/weights and script profile for its language. Whether the
  first (Latin) model ships in core or as/with a pack is an architecture-review
  question at stage 3; the spec flags the boundary so the design does not bake
  language data into the core. *(low confidence — confirm at design review.)*
- **A11 — The recognizer owns a measured recognition-quality pipeline:
  raster preprocessing before the engine and unit filtering after it, with
  every technique adopted or rejected by measurement against ground truth
  (A12), never by eye.** Added 2026-07-18 after founder review of the T2 spike
  output found systematic error classes on the fixture: (1) colored answer-chip
  *borders* touching glyphs break words ("Tot"→"Toi", "Dag"→"dz", correct
  words at <30 % confidence); (2) **reversed text** (white numerals on solid
  blue circles) misread as letter junk ("mn", "pd" for 6/7/8/9); (3) solid
  icons (speech bubbles, pencils) emitted as junk units; (4) single scrambled
  letters read with word-model bias. Candidate techniques the measurement task
  weighs: render-DPI choice, grayscale/binarization strategy, saturation-based
  masking or line-removal of colored decorations, Tesseract config (page
  segmentation mode, inversion handling, per-region re-recognition of
  low-confidence single glyphs), and post-filters on confidence, charset, and
  box geometry. Constraint: preprocessing operates on the rendered raster with
  stdlib-level image ops or engine-native options — a new imaging dependency
  is a stage-3 ADR question, not a task-level choice. Filtering must be
  conservative: it deletes only units that fail *both* a confidence floor and
  a plausibility check (no letters/digits, or degenerate geometry), so real
  low-confidence words survive to be reviewed (A6), not silently dropped.
- **A12 — Recognition quality is a measured, regression-guarded number, not an
  impression.** At least one fixture page gets a word-level ground truth
  (text + approximate region class: real text vs. decoration); an automated
  harness computes (a) the fraction of ground-truth words recognized correctly
  and (b) the count of junk units in non-text regions, and the acceptance
  budgets (`OCR_ACCURACY_MIN`, `OCR_JUNK_MAX` — named constants, values set
  from the measured baseline + achievable improvement, mirroring the A8/perf
  budget pattern) are asserted in tests (AC13, AC14). Techniques from A11 are
  adopted only if they move these numbers.
- **A14 — Recognized-unit boxes are the mouse-over/selection targets of every
  later feature, so they must stay tight to the printed word and any rendered
  text overlay must be fitted into its unit's box (both dimensions).** Founder
  requirement (2026-07-18): hovering/selecting a word for translation must hit
  the word where it sits in the book. For *this* feature that binds the
  per-region review UI (A6/T12): it renders a unit's text sized to its box,
  not at a fixed font size. For REQ-2 (selection) and REQ-4 (tap-to-look-up)
  it is the handoff guarantee: the box in the T1 contract *is* the hit target;
  no re-derivation downstream. (A9 unchanged — a full visible text layer is
  still REQ-2's surface.)

## Acceptance criteria

Each is observable and testable; the covering test is filled in at close-out
(Definition of Done). Where a criterion depends on a budget or threshold set at
design, it is written against the symbol, not an invented number. Criteria are
scoped to A1 (Latin-script printed OCR) unless stated.

| # | Criterion | Covering test |
|---|---|---|
| AC1 | Given the scanned Dutch fixture (image-only), OCR of a page returns a non-empty set of recognized units, and the recognized text of a known page contains expected words from that page (e.g. the fixture's lesson headings) — case/whitespace-normalized substring match. | |
| AC2 | Each recognized unit carries a bounding box in **page-point coordinates** within the page's size, a non-empty text string, and a confidence value in a defined range; boxes lie inside the page bounds. | |
| AC3 | A born-digital page **with** a real text layer is detected as **not** an OCR candidate and is skipped (no OCR run), while an image-only page **is** detected as a candidate (A3 heuristic). | |
| AC4 | OCR results for a document persist locally and are reused on reopen: OCR'ing the fixture, closing, and reopening returns the stored units **without** re-running recognition (observable via a run-count/timestamp or an injected engine spy). | |
| AC5 | Re-running OCR on a page is an explicit operation that replaces the stored result for that page; it is not triggered automatically on open (A5). | |
| AC6 | A user correction to a recognized unit's text is stored, takes precedence over the engine text on subsequent reads, and the original engine text remains retrievable (A6). | |
| AC7 | OCR of a multi-page document is cancellable mid-run; after cancel, pages already recognized are persisted and usable, and the app remains responsive throughout (no UI freeze) (A8). | |
| AC8 | A page image OCR cannot process (corrupt/blank/engine error) yields a per-page reported failure, not a crash; other pages still OCR, and the document still opens and reads (NFR-REL-02). | |
| AC9 | Per-page recognition completes within the configured time budget on the reference fixture/hardware; the test asserts against the budget constant set at design (A8). | |
| AC10 | No code path in this feature performs network I/O (inspection/test): OCR works fully with networking disabled (NFR-OFFLINE-01). | |
| AC11 | The bundled OCR engine and any bundled model/weights are recorded with their licenses in the project's component/attribution manifest, and each is compatible with MIT free-redistribution (NFR-LIC-01). Test/inspection asserts the manifest entry exists for the shipped engine+model. | |
| AC12 | OCR output uses the same page-point coordinate space and document-identity key as the reader/position store, so a unit's box is valid at any zoom and re-opening the same file (by path+content-hash) finds its OCR (A2, A4). | |
| AC13 | On the ground-truthed fixture page (A12), at least `OCR_ACCURACY_MIN` of ground-truth words are recognized correctly (case-normalized match) after the A11 pipeline; the harness test asserts against the named constant. | |
| AC14 | On the ground-truthed fixture page, at most `OCR_JUNK_MAX` recognized units fall in regions ground-truthed as decoration (icons, chips, ornaments), and **no** ground-truth word is deleted by the unit filter — low-confidence real words survive with their confidence, junk does not (A11, A12). | |
| AC15 | The per-region review UI renders each unit's text fitted to that unit's box (scaled in both dimensions to the box in page coordinates), so the displayed word visually coincides with the printed word at any zoom (A14, A6). | |

**Error behavior summary** (per
[CODING_STANDARDS.md](../../CODING_STANDARDS.md#error-handling)): OCR is
*additive and soft*. A page that cannot be recognized, an unsupported script, a
corrupt image, or an engine/model failure are **soft, per-page, user-facing
conditions** — reported, recoverable, never crashing the reader (the document
still opens and reads). A missing/failed OCR store read is soft: the page simply
has no OCR yet. Programmer errors (OCR requested for a page index out of range,
engine used before init) are loud.

## Open questions for design review (stage 3)

Recorded so critical-path-planner and architecture-reviewer pick them up; none
block stage 2:

1. **OCR engine ADR** (the load-bearing decision): which offline OCR engine,
   under the filters of **license** (engine *and* model/weights compatible with
   MIT free-redistribution, NFR-LIC-01), **local-first fit** (bundle/runtime
   size and the *hardware floor* — see new constraint below), and
   **cgo/single-binary** (cgo/R-03 is now acceptable per the founder; the cost is
   still weighed, not vetoed). Founder priority is **accuracy**. The decision has
   a real axis — **classical OCR vs. VLM (vision-language-model) OCR** — with
   honest trade-offs to weigh:
   - **Tesseract** — classical; small (tens of MB), CPU-only, mature, integrates
     via cgo or as a bundled subprocess. Apache-2.0 engine; per-language model
     (traineddata) licenses vary and must be checked. Good—not-SOTA accuracy;
     strong on clean Latin print (the Dutch fixture's profile).
   - **PaddleOCR-VL** (0.9B VLM, Apache-2.0, ~109 languages, ~96% OmniDocBench) —
     SOTA accuracy, but Python/PyTorch, GB-scale weights, realistically wants a
     GPU. Fits only as a heavy bundled subprocess with a real hardware floor.
   - **MinerU 2.5** (1.2B VLM) — SOTA document parsing; **relicensed off AGPLv3**
     to a *custom* Apache-2.0-based license — its exact text and the model-weights
     license must be read against NFR-LIC-01 (custom ≠ Apache). Same Python/GPU
     weight class as PaddleOCR-VL.
   - **pure-Go / wasm OCR** — keeps the single-binary property but accuracy is
     weak/immature; likely fails the accuracy priority.
   The reviewer should also weigh a **two-tier** design: a light default engine
   (e.g. Tesseract) that satisfies the MVP on CPU, with a VLM engine as an
   **optional high-accuracy backend behind the same OCR interface** for users
   with the hardware — so the desktop app's size and hardware floor are not set
   by a 1B-param model. **Must be resolved before implementation (T-* recognition
   tasks).**
2. **Language scope — RESOLVED** by
   [ADR-0013](../../architecture/ADR-0013-mvp-language-dutch.md): Dutch
   (Latin script) is the MVP language; CJK/Japanese OCR is a later pack, out of
   scope here (spec A1). No longer open.
3. **Trigger & UX (A5/A6/A9)**: auto-OCR-on-open vs. explicit "Recognize text";
   whether per-region **review/correction** is in the first release or a
   fast-follow; the minimal reviewable-unit UI.
4. **Store now vs. SQLite slice (A4)**: interim file-backed OCR store vs. a
   minimal slice of the SQLite store (ADR-0008), and the OCR result schema
   (units, boxes, confidence, corrections) — this schema is a near-term SemVer
   surface for the on-disk data.
5. **Language Pack boundary (A10)**: does the Latin OCR model ship in core or via
   a pack; where does the script profile live (ADR-0011)?
6. **Detection heuristic (A3)**: exact "needs OCR" threshold, and whether the
   user can force-OCR a page with a poor native text layer.

## Revision history

- 2026-07-07 — Initial version, written from REQ-10 after the backlog reordering
  pulled OCR ahead of REQ-2 (scanned books are the real use case and REQ-2 has
  nothing to consume without OCR). Ready for stage 2.
- 2026-07-07 — Founder decisions folded in (no AC changes): MVP language is
  **Dutch** ([ADR-0013](../../architecture/ADR-0013-mvp-language-dutch.md)),
  resolving A1 and open question 2; **accuracy** is the stated priority and
  **cgo is acceptable** (R-03 no longer a veto), reframing the engine ADR (open
  question 1) around a classical-vs-VLM choice with **Tesseract**,
  **PaddleOCR-VL**, and **MinerU 2.5** named as candidates. Added a hardware-floor
  / install-size constraint so a GPU-class VLM can only be an optional backend,
  not the baseline. Scope and criteria otherwise unchanged.
- 2026-07-18 — **Recognition-quality amendment** from founder review of the T2
  spike output (docs/planning/ocr/spike-t2-findings.md and its visual review):
  systematic error classes on the fixture (chip borders breaking words,
  reversed white-on-blue numerals, icon junk units, scrambled-letter
  exercises) are now in scope as a *measured* pipeline — new assumptions
  A11 (preprocessing + conservative unit filtering, technique-by-measurement),
  A12 (ground truth + accuracy/junk budgets), A14 (unit boxes are the
  selection/hover targets; review UI fits text to the box), new criteria
  AC13–AC15. Non-goals clarified: junk *filtering* is in scope, text
  *correction* and semantic exercise detection remain out. **Acceptance
  criteria changed and scope changed materially → critical-path-planner must
  re-run (stage 2) and the design review must be re-checked for the new
  tasks.**
