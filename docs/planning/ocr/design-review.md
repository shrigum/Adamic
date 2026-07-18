# Design review: OCR text layer

- **Stage**: 3 — Design review ([planning flow](../../process/PLANNING_FLOW.md))
- **Reviewer**: architecture-reviewer skill
- **Inputs**: [spec.md](spec.md), [critical-path.md](critical-path.md),
  [docs/architecture/README.md](../../architecture/README.md), full ADR index
  (0001–0013), [CODING_STANDARDS.md](../../CODING_STANDARDS.md),
  [CONTRIBUTING.md](../../../CONTRIBUTING.md)
- **Date**: 2026-07-07

## Verdict: APPROVED-WITH-CONDITIONS

Proceed to stage 4 **once C1 (engine spike) succeeds and C2–C6 are folded into
the plan**. C1 is a true gate: if the spike fails (chosen integration can't meet
the accuracy/latency/packaging bar on the Dutch fixture), the engine ADR
re-opens before any recognizer code (T4+) is written. The plan is well-shaped —
the contract-first, two-track decomposition is right and the seams reuse the
reader's existing ones cleanly. Findings are about **guarding** two allowed
abstractions and **cutting** one scope item, not restructuring.

## Fit with existing architecture

- **Module shape** ✔. OCR adds importable core packages (a `Recognizer` +
  engine binding, an OCR-result store) with the engine binding confined to one
  package — the same discipline that keeps PDFium inside `document`
  ([ADR-0012](../../architecture/ADR-0012-pdf-engine.md)). One-way dependencies
  hold: OCR depends on `document` (page images) and `library` (identity); nothing
  in `document`/`reader`/`library` depends back on OCR. Consistent with the
  [system overview](../../architecture/README.md#system-overview).
- **Local-first / offline invariant** ✔. AC10 tasks an explicit no-network
  inspection (T13/T14); the engine runs on-device. Respects
  [ADR-0003](../../architecture/ADR-0003-update-check.md)'s standing invariant
  and CONTRIBUTING's "no network at runtime" rule. **Guard added** (C5): a
  *subprocess* engine must be invoked with no network access and bundled models
  only — the offline inspection must cover the subprocess, not just Go imports.
- **Coordinate model & identity reuse** ✔, verified not duplicated. Boxes are in
  **page points** matching `reader.PageSize` (`WidthPt/HeightPt`), and the OCR
  store is keyed by `library.Identify` (path+content-hash) — the same `DocID`
  the reading-position store uses. AC12 pins this. The plan reuses these seams
  rather than reinventing them; do **not** introduce a second coordinate space
  or identity scheme (C4).
- **Persistence destination** ✔ with the same caveat approved for the reader.
  The OCR result is local and file-backed for now, destined for SQLite per
  [ADR-0008](../../architecture/ADR-0008-local-data-storage.md). Not a
  contradiction — 0008 is the destination, not a precondition.
- **Language Pack boundary** ✔ untouched for the MVP. The engine/runtime is core;
  the Dutch model ships with the app for now. Spec A10 leaves "model behind a
  pack" as a *later* question — correctly not built now
  ([ADR-0011](../../architecture/ADR-0011-language-pack-boundary.md) unchanged).
- **cgo posture** — a real interaction with [ADR-0012](../../architecture/ADR-0012-pdf-engine.md),
  handled by the new ADR. 0012 established a no-cgo preference; OCR may need cgo
  or a bundled native subprocess. That is a **refinement of an accepted ADR**,
  which has exactly two legal outcomes — change the design, or write a
  superseding/refining ADR. We took the second: **ADR-0014** records it. No quiet
  contradiction remains.

## ADR decision

**An ADR is required** — the design introduces a **major third-party dependency**
(an OCR engine + model), a **new persistent on-disk format** (the OCR result
store), a **new top-level component** (the Recognizer), and **changes a
cross-cutting policy** (may reintroduce cgo, refining ADR-0012). Every trigger on
the list fires.

→ Produced: [ADR-0014](../../architecture/ADR-0014-ocr-engine.md) — **Tesseract
(baseline) behind a Recognizer seam**. I verified the reviewer-supplied facts
independently and they hold: the Tesseract **engine is Apache-2.0 and the Dutch
`nld` model (tessdata/tessdata_best) is Apache-2.0** — both clear NFR-LIC-01,
which is the decisive point and removes the model-license trap that keeps the
VLMs risky. Tesseract is CPU-only and small (meets the hardware-floor
constraint), and its TSV output is per-word `{text, box, confidence}` — a direct
map onto the `RecognizedUnit` contract. The VLMs (PaddleOCR-VL, MinerU 2.5) are
SOTA but GB-scale/GPU-class, so they fail the *baseline* hardware floor and are
recorded as the **foreseen optional backend**, not built now. The no-cgo wasm
binding (`gogosseract`) is rejected on maintenance grounds (abandoned, broken by
wazero 1.8.0, ~6× slower) — which is exactly why the **subprocess** integration
is attractive (keeps our Go code cgo-free without that binding). Index row added.

## Complexity check (simplest correct design?)

- **T1 contract-first + two-track (recognition vs. persistence) — approved.**
  This is the correct seam, not over-decomposition: the `RecognizedUnit` contract
  is what lets the engine track (T4/T5) and the store track (T6/T8) proceed in
  parallel and rejoin at T9, and the contract must exist regardless because it is
  the on-disk schema *and* the recognizer output *and* what REQ-2 will consume.
  Three consumers of one contract is the opposite of speculative. Kept.
- **A4 file-backed OCR store — approved, not premature (C3-equivalent).** This is
  the **same** justified pattern approved as condition C3 for pdf-reader-core:
  one real implementation now (file), one **already-decided** second
  (SQLite, ADR-0008), a narrow `Save`/`Load` seam. The rule-of-three exception is
  explicit — the second implementation is *scheduled*, not imagined. **Condition
  C3**: keep it to `Save`/`Load` of one document's OCR result keyed by `DocID`;
  do **not** grow a general "document metadata repository." Mirror
  `library.FileStore` (atomic temp+rename, versioned envelope) rather than
  inventing a new persistence style.
- **T4 `Recognizer` interface — approved as a guarded seam, with the rule-of-three
  line drawn explicitly (C2).** An interface with one implementation is normally
  a smell. It is allowed **here** because ADR-0014 commits to a *specific,
  foreseen second implementation* (the optional VLM backend) with a concrete
  reason it can't be the baseline (hardware floor) — i.e. the second consumer is
  named and scheduled-if-pursued, not "for testability" or "for flexibility."
  **Condition C2**: the interface exposes only what recognition needs (page image
  + page size → units); ship **exactly one** engine (Tesseract) in the MVP;
  **do not build the VLM backend now**, and do not add engine-selection config,
  capability registries, or a second engine's plumbing. If the VLM backend is
  never pursued, this stays a one-implementation interface and that is a finding
  to revisit, not a disaster — but the ADR's named second impl clears the bar to
  *start* with the seam.
- **Per-region review UI (T12) — recommend DEFER to a fast-follow (C6).** This is
  the largest, most speculative slice of UI and the plan already flags it as the
  deferral valve. Recommendation: **ship OCR-without-review in v1**; land the
  review/correction *UI* as a fast-follow. Crucially, the correction **model +
  store** (T8) and its **binding** (T11) still ship in v1, so AC6's *storage
  behavior* (a correction is stored, takes precedence, original retained) is
  satisfied and testable at the model level now; only the on-screen review is
  deferred. This drops the CP from 49h to 47h and removes the riskiest UI scope
  from the first release. **Condition C6**: build T8 (model) in v1; defer T12
  (UI) unless the founder wants it in v1 — mark the choice in the plan.
- **Concurrency ownership (T7) — under-designed as written; must name an owner
  (C4-adjacent, folded into notes).** CODING_STANDARDS: "no goroutines without an
  owner." T7 runs OCR off the UI thread and must be cancellable. The plan says
  *what* but not *who owns shutdown*. Not a rejection — a required detail: T7's
  outcome must state that a single component owns the worker's lifecycle and
  cancellation (a `context.Context`), and that each page's result is persisted as
  it completes so a cancel leaves a consistent partial result. Folded into the
  implementer notes and C4.
- **No premature abstraction over "engines" beyond the one seam justified above,
  and no second coordinate/identity scheme** (C4). Use `document`'s page raster
  and `reader.PageSize` directly; use `library.Identify` directly. Do not wrap
  them in OCR-specific re-abstractions.
- **Detection (T3) — right-sized.** A per-page heuristic serving AC3, no
  document-structure model built. No scope creep.

## Conditions

| # | Condition | Rationale | Status |
|---|---|---|---|
| C1 | **Spike T2 (engine + integration) before any recognizer code (T4+).** Prove the chosen Tesseract integration recognizes the **Dutch fixture** offline to text+boxes, and record per-page latency, bundle size, and the subprocess-vs-cgo pick **per platform**. A failure re-opens [ADR-0014](../../architecture/ADR-0014-ocr-engine.md). | High risk on the critical path → build first ([rigor rule](../../CRITICAL_PATH_METHOD.md)). | **Done — PASS** (2026-07-18, [spike-t2-findings.md](spike-t2-findings.md)): Dutch fixture recognized offline to text+boxes at 93–97 % word confidence, ~2 s/page; **subprocess** picked on Windows (cgo leg blocked by absent C toolchain — recorded as an R-03 cost datum). T4+ unblocked. |
| C2 | `Recognizer` interface exposes only (page image + page size → units); **exactly one** engine (Tesseract) ships in the MVP. Do **not** build the VLM backend, engine-selection config, or a capability registry now. | Rule of three: the seam is allowed only because ADR-0014 names a scheduled-if-pursued second impl — it is not license to build it. | todo |
| C3 | OCR store stays a narrow `Save`/`Load` of one document's OCR result keyed by `library.DocID`; versioned on-disk envelope; mirrors `library.FileStore`. No general repository. | Justified seam (SQLite is scheduled per ADR-0008), guarded against growing into speculative generality. | todo |
| C4 | Reuse `document`'s page raster, `reader.PageSize` (page-point coords), and `library.Identify` directly — no second coordinate space, identity scheme, or engine re-abstraction. | Avoid duplicating decided, working seams. | todo |
| C5 | The offline inspection (AC10) must cover a **subprocess** engine if chosen: it is invoked with no network and bundled models only, not just "no `net` imports in Go." | A native subprocess is a code path the Go-import scan alone would miss. | todo |
| C6 | Build the correction **model + store + binding** (T8, T11) in v1; **defer the per-region review UI (T12)** to a fast-follow unless the founder wants it in v1. Record the choice in the plan. | Cuts the riskiest, most speculative UI scope while still satisfying AC6's storage behavior; drops CP 49h→47h. | **Resolved — founder chose review UI IN v1** (2026-07-07). T12 is on the critical path; active CP = 49h. The C6 *guard* still applies: the review UI is scoped to per-region view + inline text correction (spec A6), not bulk find/replace or document-wide re-flow. |

## Notes for the implementer

- The engine binding is confined to the Recognizer package; nothing else imports
  the Tesseract binding/subprocess wrapper. Its failure modes (engine missing,
  model missing, page unreadable, timeout) are normalized into the OCR result's
  typed per-page error (AC8, T13).
- **Own your failure modes** in the package doc comment (CODING_STANDARDS): what
  recognition/store errors occur and that OCR is *additive and soft* — a page
  that can't be recognized leaves the document fully readable, and a store
  failure means "no OCR yet," never a crash.
- **T7 concurrency**: one component owns the OCR worker and its cancellation via
  `context.Context`; persist each page's result as it completes so cancel leaves
  a consistent partial result (per C4-adjacent finding above). No orphaned
  goroutines.
- Keep the pixel→point transform in **one place**, derived from the render scale
  and `reader.PageSize`, and assert boxes-inside-page on the fixture from the
  first recognizer commit (AC2).
- Do not start T4 until C1 is `Done`. T3 (detection), T6 (store), and T8
  (corrections model) can proceed in parallel with the T2 spike — none depends
  on the engine.
- `package main`/`desktop` stays wiring-only; the recognizer, store, and worker
  are packages.
