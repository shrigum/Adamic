# T2 spike findings: Tesseract recognition of the Dutch fixture

- **Task**: T2 of [critical-path.md](critical-path.md); gate C1 of
  [design-review.md](design-review.md); executes the measurement follow-up of
  [ADR-0014](../../architecture/ADR-0014-ocr-engine.md).
- **Date**: 2026-07-18
- **Verdict**: **PASS — ADR-0014 stands.** Tesseract 5.4.0 with the
  `tessdata_best` Dutch (`nld`) model recognizes the scanned fixture offline to
  per-word text + boxes + confidence, well within acceptable latency, via the
  **subprocess** integration. No grounds to re-open the engine ADR.
- **Durable proof**: [src/ocr/spike_test.go](../../../src/ocr/spike_test.go) —
  renders fixture page 1 through the real Document Engine at ~300 DPI, runs
  `tesseract … tsv` as a subprocess, maps TSV onto the T1 contract with the
  pixel→point transform, and asserts contract validity (AC2), expected Dutch
  words (AC1), and confidence sanity. It skips when no Tesseract is present
  (set `ADAMIC_TESSERACT` or have `tesseract` + `nld` on PATH); a *failure* on
  a supported platform re-opens ADR-0014.

## Setup measured

- Tesseract **5.4.0.20240606** (UB Mannheim build,
  digi.bib.uni-mannheim.de/tesseract), run portably — extracted from the
  installer, no system install.
- Dutch model **`nld.traineddata` from `tessdata_best`** (github.com/tesseract-ocr/tessdata_best),
  8.5 MB, Apache-2.0 (as is the engine — NFR-LIC-01 holds, feeds T10).
- Input: fixture pages rendered by the Document Engine (PDFium/wasm) at zoom
  300/72 ≈ 4.17 → 2488×3516 px for the 597×844 pt A4 pages (spec A7).
- Hardware: AMD Ryzen 5 5600X (6-core), 32 GB RAM, Windows 11 — a desktop CPU;
  laptop-class hardware will be slower, which the T15 budget must allow for.
  CPU-only throughout; no GPU involved.

## Recognition quality (AC1/AC2 evidence)

- Page 1: 182 word units. Exercise headings and vocabulary recognized
  correctly — "Welk woord is weg?", "Nederlands", "Goedemorgen", "Mevrouw",
  "Luister", "docent" — at 93–97 % per-word confidence. All 4 pages return
  substantial, plausible word sets (84–228 words/page).
- Every unit passes the T1 contract check: non-empty text, confidence
  normalized to [0,1], positive box inside the page bounds after the
  pixel→point transform. TSV word rows carry block/paragraph/line numbers,
  kept as the opaque `Group` id (spec A2).

## Per-page latency (input to T15's budget)

One cold `tesseract` process per page (subprocess model), TSV to stdout,
300 DPI input:

| Page | Words | Latency |
|---|---|---|
| 1 | 182 | 2.0 s |
| 2 | 84 | 1.3 s |
| 3 | 126 | 1.9 s |
| 4 | 228 | 2.1 s |

≈ 0.3–0.4 s of each run is process start + model load, amortizable later by
batching many pages into one invocation (Tesseract accepts an image-list file)
— an optimization T7 can pick up if the budget demands it, not needed for the
spike. Suggested starting point for T15: a **10 s/page** ceiling gives ~5×
headroom for laptop-class CPUs and denser pages.

## Install size (Windows)

| What | Size |
|---|---|
| Full UB-Mannheim extraction (incl. training tools, docs) | 246.5 MB |
| Trimmed runtime bundle: `tesseract.exe` + 25 DLLs + `tessdata/` (`nld` + configs) | **128.6 MB** |
| — of which `libtesseract-5.dll` (unstripped MinGW build) | 96.8 MB |
| — `nld.traineddata` (tessdata_best) | 8.5 MB |

Verified by deletion + re-run: ICU (34 MB) and the pango/cairo/freetype group
are droppable; glib/gio/crypto are required by this build. The 96.8 MB
`libtesseract-5.dll` is an **unstripped** debug-info build — the packaging
task should build/source a stripped lean Tesseract (typical stripped size is
in the 5–15 MB range), putting a realistic shipped bundle at **~25–40 MB**.
The UB-Mannheim trim is the *upper bound* proven here, not the target.

## Integration pick (per platform)

- **Windows: subprocess.** Proven end to end here. Keeps our Go code cgo-free
  (`CGO_ENABLED=0` build preserved; ADR-0012's posture undisturbed outside the
  bundled binary), failure isolation for free (an engine crash is an exited
  process → clean per-page `PageFailure`, AC8), and per-page cancellation is
  process kill. Cost: bundle a per-platform binary + model (packaging task).
- **cgo (gosseract) leg: not measurable on this machine** — no C toolchain
  (no gcc/MSYS2) is installed, which is itself the finding: the cgo path
  imposes that toolchain on every Windows contributor and release builder,
  exactly the R-03 cost ADR-0012 avoided. With subprocess latency already at
  ~2 s/page and its startup overhead (~0.3–0.4 s) both tolerable and
  amortizable, cgo's win would be marginal. **Pick: subprocess on Windows;
  macOS/Linux expected to follow (same trade-offs, `tesseract` also
  distributable/installable there) — confirm at the packaging task with a run
  of the spike test on those platforms.**

## Follow-ups fed by this spike

- T15: set the per-page budget constant from the table above (suggest 10 s).
- T10: license manifest — engine Apache-2.0, `nld` (tessdata_best) Apache-2.0.
- T4: wrap exactly this subprocess path behind the `Recognizer` seam; reuse
  the spike's TSV→`RecognizedUnit` mapping and single pixel→point transform.
- Packaging (later task): source or build a **stripped** Tesseract per
  platform; version-pin engine + model (ADR-0014 consequence).
