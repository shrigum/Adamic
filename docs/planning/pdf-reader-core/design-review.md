# Design review: PDF reader core

- **Stage**: 3 — Design review ([planning flow](../../process/PLANNING_FLOW.md))
- **Reviewer**: architecture-reviewer skill
- **Inputs**: [spec.md](spec.md), [critical-path.md](critical-path.md),
  [docs/architecture/README.md](../../architecture/README.md), full ADR index
  (0001–0012), [CODING_STANDARDS.md](../../CODING_STANDARDS.md)
- **Date**: 2026-07-07

## Verdict: APPROVED-WITH-CONDITIONS

Proceed to stage 4 **once C1 (engine spike) succeeds and C2–C5 are folded into
the plan**. C1 is a true gate: if the spike fails, the design (and possibly
ADR-0005) re-opens before any rendering code is written.

## Fit with existing architecture

- **Module shape** ✔. The design keeps the native engine behind a Go Document
  Engine package and the frontend behind the command interface (T1) — thin
  entry point, domain logic in importable packages, one-way dependencies
  (frontend → core, never the reverse). Consistent with the
  [system overview](../../architecture/README.md#system-overview) and
  [ADR-0005](../../architecture/ADR-0005-platform-stack.md).
- **Local-first / no-network invariant** ✔. The reader touches no network;
  AC12 tasks an explicit inspection. This respects
  [ADR-0003](../../architecture/ADR-0003-update-check.md)'s standing invariant
  (the only network code is `adamic update`).
- **Persistence** ✔ with a caveat (C3). Reading position is local and
  file-backed for now, destined for SQLite per
  [ADR-0008](../../architecture/ADR-0008-local-data-storage.md). This does not
  contradict 0008 — 0008 is the destination, not a precondition.
- **No language logic** ✔. This feature is language-agnostic; it renders page
  images and knows nothing of text or lemmas
  ([ADR-0011](../../architecture/ADR-0011-language-pack-boundary.md) untouched).

## ADR decision

**An ADR is required** — the design introduces a **major third-party
dependency** and a **new top-level component** (the PDF engine + Document
Engine package), squarely on the "significant decision" list. The load-bearing
sub-decision (MuPDF vs PDFium) is licensing-critical.

→ Produced: [ADR-0012](../../architecture/ADR-0012-pdf-engine.md) — **PDFium via
`klippa-app/go-pdfium`**. Verified the reviewer-supplied reasoning
independently: MuPDF is AGPL/commercial, which is **incompatible** with Adamic's
MIT license and NFR-LIC-01 ("compatible with free redistribution") — this is
decisive, not a preference; PDFium is BSD and the binding is actively
maintained with a no-cgo wasm backend that materially helps R-03. Index row
added.

A **second ADR-shaped decision is deferred, correctly**: the Wails desktop
shell (T6) is the concrete realization of ADR-0005, not a new decision — no
separate ADR needed. Recorded here so the next person doesn't re-derive it.

## Complexity check (simplest correct design?)

- **T1 command-interface-first — approved, with a guard (C2).** Contract-first
  is the *right* seam here: it's what lets the engine track (T2–T5) and the
  frontend track (T6) proceed in parallel, and ADR-0005 already commits us to a
  stable core/frontend boundary, so this interface must exist regardless. The
  risk is over-specifying it before its shape is known. **Condition C2**: T1
  defines *only* the commands the ACs need (open, page count, render page at
  scale, thumbnails, get/set position) — no speculative commands, no
  generalized "capability registry." It is allowed to change during T3–T7; it
  is not a frozen public API until the feature closes.
- **A3 file-backed persistence interface — approved, not premature (C3).** This
  is a one-method-pair interface with **one** real implementation now and a
  **known, already-decided** second one (SQLite, ADR-0008). That is not
  speculative generality — the rule-of-three exception is explicit: the second
  implementation is scheduled, not imagined. The interface is the seam that
  keeps REQ-1 from blocking on the unbuilt data store. **Condition C3**: keep it
  to `Save`/`Load` for reading position + the minimal document record — do
  **not** grow a general "repository" abstraction; when SQLite lands it
  implements this narrow interface or replaces it outright.
- **Virtualized rendering (T5) — necessary, not gold-plating.** AC3 and AC11
  require bounded rendering on a 500-page document; rendering all pages would
  fail the ACs. Kept.
- **Thumbnails (T8), zoom/fit (T9) — each traces to an AC** (AC6, AC4). No
  scope creep found.
- **No premature abstraction over "storage backends" or "engines"** beyond the
  two seams justified above. The PDFium binding is used directly inside the
  Document Engine package; we do **not** build an engine-abstraction layer for a
  second engine that does not exist (C4).

## Conditions

| # | Condition | Rationale | Status |
|---|---|---|---|
| C1 | **Spike T2 (engine + cgo/wasm build) before any rendering code (T3+).** Prove one real PDF page renders via `go-pdfium` **and** builds for win/mac/linux (record which backend per platform). A failure re-opens ADR-0012/0005. | High risk on the critical path → build first ([rigor rule](../../CRITICAL_PATH_METHOD.md)). | **Done** — PASS 2026-07-07; render + all-6-target `CGO_ENABLED=0` build green ([spike result](critical-path.md#t2-spike-result-c1-gate)). |
| C2 | T1 interface exposes only AC-required commands; may evolve through T7; not frozen until close. | Avoid designing a boundary's details before its shape is known, without losing the parallelizing seam. | todo |
| C3 | Reading-position store stays a narrow `Save`/`Load` + minimal record; no general repository layer. Swappable for SQLite (ADR-0008) with no AC change. | Justified seam, but guard against it growing into speculative generality. | todo |
| C4 | Use the `go-pdfium` binding directly inside the Document Engine package; do **not** add an engine-abstraction interface for a hypothetical second engine. | Rule of three: one engine, no second consumer. | todo |
| C5 | Prefer the **wasm/purego backend** where its measured performance meets the T11 budgets; fall back to cgo only where it doesn't. Record the choice per platform in T2. | Directly reduces R-03; keep the single-binary property where feasible. | **Done (pending T11 perf)** — wasm/purego chosen for all 6 targets in T2; cgo not needed. Perf validation deferred to T11. |

## Notes for the implementer

- The engine binding is confined to the Document Engine package; nothing else
  imports `go-pdfium`. Its failure modes (bad handle, encrypted, malformed) are
  normalized into the typed error shape from T1 (see AC9/AC10, T13).
- Own your failure modes in the package doc comment (CODING_STANDARDS.md): what
  render/open errors occur and that a position-store failure is soft (open still
  succeeds; position simply isn't restored).
- Do not start T3 until C1 is `Done`. Stand up T6 (Wails shell) in parallel with
  the T2 spike so both High risks surface together.
- Keep `package main` wiring-only; the Wails app bootstrap is wiring, the engine
  and store are packages.
