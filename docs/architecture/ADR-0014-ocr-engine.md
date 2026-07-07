# ADR-0014: OCR engine is Tesseract (baseline), behind a Recognizer seam

- **Status**: Accepted
- **Date**: 2026-07-07
- **Feature/trigger**: [docs/planning/ocr/](../planning/ocr/) — spec open
  question 1; critical-path task T2
- **Deciders**: project maintainers (technical lead), via architecture-reviewer
  stage 3

## Context

OCR (REQ-10) turns scanned image-only pages into recognized text positioned on
the page, so the language layer has something to attach to. The engine choice is
the load-bearing decision of the feature: it sets accuracy, the app's install
size and hardware floor, the licensing surface, and whether we keep the no-cgo
single-binary property.

Four forces are in tension. **Licensing (NFR-LIC-01)**: both the engine *and* any
bundled model/weights must be MIT-redistribution-compatible — a model-license
trap that is easy to miss. **Accuracy** is the founder's stated priority; a wrong
reading silently poisons every downstream feature (lookup, familiarity,
vocabulary). **Hardware floor / install size** (spec constraint): Adamic is a
desktop app shipped as a GitHub Release, currently ~22 MB, that must run on a
typical CPU-only laptop — a GPU-class multi-GB model changes what machine can run
the product. **Integration / cgo (R-03)**: the Document Engine is deliberately
no-cgo today ([ADR-0012](ADR-0012-pdf-engine.md)); the founder has now accepted
that OCR *may* reintroduce cgo, but the cost is still weighed, not free. The MVP
language is **Dutch** (Latin script, [ADR-0013](ADR-0013-mvp-language-dutch.md)).

## Decision

We use **Tesseract** as the baseline OCR engine for the MVP. Its engine is
Apache-2.0 and its Dutch model (`nld`, from `tessdata`/`tessdata_best`) is
Apache-2.0 — **both clear NFR-LIC-01**, defusing the model-license trap. It is
CPU-only and tens of MB, fitting the hardware-floor constraint, and its
TSV/hOCR output gives exactly the per-word `{text, bounding box, confidence}`
the OCR result contract (spec A2) needs, with Dutch accuracy on clean print that
meets the fixture's profile.

Tesseract sits behind Adamic's own **`Recognizer` interface** (the T4 seam):
input a page image + the page's point size, output `RecognizedUnit`s in
page-point coordinates. The Tesseract binding is confined to one package, exactly
as PDFium is confined to `document`. This seam is deliberately allowed under the
rule of three because a **second, already-foreseen implementation exists**: a
VLM engine (PaddleOCR-VL) as an *optional* high-accuracy backend for users who
want SOTA quality and have the hardware. **Only Tesseract ships in the MVP; the
VLM backend is not built now** — the seam exists so it can be added later without
touching the recognition/persistence/UI code, not as a built abstraction today.

The **integration mechanism** (Tesseract as a bundled **subprocess** invoked over
its CLI/TSV, keeping our Go code cgo-free, vs. **cgo** via `otiai10/gosseract`)
is decided by the T2 spike on measured per-page latency and per-platform build
cost, and recorded per platform. Both are acceptable (cgo is no longer vetoed);
the spike picks. The no-cgo wasm option (`gogosseract`) is rejected below.

## Alternatives considered

### Tesseract via the no-cgo WebAssembly binding (Danlock/gogosseract)
Genuine advantage: keeps the single-binary, no-C-toolchain property we value —
same wazero runtime family already used for PDFium — while still being
Tesseract. It lost on **maintenance and performance**: the library is abandoned,
broken by a backwards-incompatible wazero 1.8.0 change (usable only with pinned
old deps), and ~6× slower than the cgo path. Depending on an unmaintained,
already-broken binding for a load-bearing capability is an unacceptable risk
(CONTRIBUTING requires *actively maintained* dependencies). Rejected — but it is
the reason the *subprocess* path is attractive: it recovers most of the no-cgo
benefit (our Go code stays cgo-free) without depending on this binding.

### PaddleOCR-VL (0.9B vision-language model) as the baseline
Genuine advantage: SOTA accuracy (~96% OmniDocBench), ~109 languages, Apache-2.0
engine — the best recognition quality available. It lost **as the baseline** on
the hardware-floor constraint: it is a ~1B-parameter PyTorch model, gigabytes of
weights, realistically needing a GPU for tolerable speed. Shipping it as the
default would raise Adamic's install size by 1–3 orders of magnitude and impose a
GPU-class floor on a "read a book on your laptop" product. **Retained as the
foreseen optional backend** behind the Recognizer seam (opt-in, for users with
the hardware) — which is precisely what justifies that seam existing.

### MinerU 2.5 (1.2B VLM)
Genuine advantage: SOTA document parsing; recently relicensed off AGPLv3, which
had been a hard blocker. It lost for the **same hardware-floor reason** as
PaddleOCR-VL, plus a **license caveat**: the new license is a *custom*
Apache-based license, not Apache-2.0 itself, and the model weights are a separate
surface — both need reading against NFR-LIC-01 before it could ship. Not a clean
yes; not needed for the MVP. Reconsider only if a VLM backend is pursued and it
beats PaddleOCR-VL on the license/quality trade.

### Pure-Go OCR (no Tesseract, no native engine)
Genuine advantage: preserves the single-binary property with zero native
dependencies. It lost on **accuracy**: no pure-Go OCR is production-competitive
for real-world print, which fails the founder's accuracy priority and AC1.
Rejected.

## Consequences

- **Positive**: license-clean end to end (engine + Dutch model both Apache-2.0,
  AC11 satisfiable); CPU-only and small, meeting the hardware-floor constraint;
  TSV output maps directly onto the `RecognizedUnit` contract (AC1/AC2); mature,
  actively maintained, widely adopted. The Recognizer seam keeps a credible path
  to SOTA accuracy (optional VLM backend) without paying its cost now.
- **Negative**: Tesseract accuracy is good, not SOTA — a noisier scan may
  recognize worse than a VLM would; the per-region review/correction path (spec
  A6) is the mitigation for bad readings. The **subprocess** path means bundling
  a per-platform `tesseract` binary + `nld.traineddata` (packaging work, a
  configuration item to version-pin); the **cgo** path reintroduces R-03 (C
  toolchain, per-platform cross-compilation, loss of the pure-`CGO_ENABLED=0`
  build) — the T2 spike picks and records which per platform. **Revisit if** the
  chosen integration path's build/packaging cost proves unsustainable, or if
  Tesseract accuracy proves inadequate on the target corpus — the fallback is the
  already-designed optional VLM backend, not a different baseline.
- **Amends prior decisions (no rewrite)**: refines
  [ADR-0012](ADR-0012-pdf-engine.md)'s no-cgo posture — OCR is explicitly allowed
  to use cgo or bundle a native subprocess where the PDF engine did not need to.
  ADR-0012 stands for the PDF engine; this narrows the project-wide "prefer
  no-cgo" default for the OCR capability specifically.
- **Follow-ups**: T2 spike proves recognition on the Dutch fixture, measures
  per-page latency + bundle size, and picks subprocess-vs-cgo per platform; T15
  sets the per-page budget from those measurements; T10 records the Tesseract
  engine + `nld` model licenses in the component/attribution manifest.
