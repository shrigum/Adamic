# ADR-0012: PDF rendering engine is PDFium (via klippa-app/go-pdfium)

- **Status**: Accepted
- **Date**: 2026-07-07
- **Feature/trigger**: [docs/planning/pdf-reader-core/](../planning/pdf-reader-core/) — spec open question 1; critical-path task T2
- **Deciders**: project maintainers (technical lead), via architecture-reviewer stage 3

## Context

The reader (REQ-1) must rasterize PDF pages faithfully; text extraction (REQ-2)
and later OCR/annotation build on the same engine. Go has no
production-quality pure-Go PDF *renderer*, so a native engine integrated via
cgo is required — this is Adamic's first cgo dependency and the root of risk
R-03 (removes the single-static-binary property; needs a C toolchain and
container cross-compilation). The Architecture and Design Document §4.1 named
"MuPDF or PDFium" and deferred the choice; [ADR-0005](ADR-0005-platform-stack.md)
assumed cgo. This ADR makes the call.

Two forces dominate. **Licensing (NFR-LIC-01)**: Adamic is MIT-licensed and
requires bundled components to use licenses mutually compatible and compatible
with free redistribution. **Build/packaging cost (R-03)**: whatever we pick, we
must build it for Windows, macOS, and Linux from the project's local, free
tooling. Rendering fidelity and maintenance health are secondary but real.

## Decision

We use **PDFium** as the PDF rendering (and text-extraction) engine, integrated
through **`github.com/klippa-app/go-pdfium`**. PDFium is BSD-licensed —
permissive and compatible with Adamic's MIT license and free-redistribution
requirement. The binding is actively maintained, wraps the same PDFium library
that drives Chrome's PDF viewer, and exposes single-threaded cgo, subprocess
multi-threading, and a WebAssembly (`purego`, no-cgo) backend behind one
interface. The engine sits behind Adamic's own Document Engine interface (the
`pdf-reader-core` command contract), so the binding is not referenced outside
that package.

The `wazero`/WebAssembly backend is explicitly kept in scope as the **preferred
default** where its performance is acceptable, because it needs **no cgo and no
C toolchain** — which directly attacks R-03 and can partially restore the
single-binary property. The cgo backend is the fallback where wasm performance
is inadequate. Task T2 (spike) measures both and records which backend each
platform ships.

## Alternatives considered

### MuPDF (via gen2brain/go-fitz)
Genuine advantages: marginally higher rendering fidelity in published
comparisons, and a mature, single library covering render + extraction + more.
It lost decisively on **licensing**: MuPDF is **AGPL** (or a paid commercial
license from Artifex). AGPL is viral copyleft — statically linking it into
Adamic would force the entire distributed application under AGPL terms,
contradicting the chosen MIT license and the NFR-LIC-01 "compatible with free
redistribution" requirement, and creating obligations (network-use source
disclosure) inappropriate for this project. Fidelity does not outweigh a
license incompatibility. Rejected.

### Pure-Go PDF libraries (e.g. ledongthuc/pdf, pdfcpu, unipdf)
Genuine advantage: no cgo — preserves the single-static-binary property and
sidesteps R-03 entirely. It lost because none provides production-quality
**page rasterization** of arbitrary real-world PDFs (complex fonts, shading,
subset fonts, CJK) at the fidelity FR-NAV-01 demands; they target manipulation
or text extraction, not faithful rendering. unipdf is additionally
commercially licensed. Rejected for the rendering requirement — though the
PDFium **wasm** backend recovers most of the no-cgo benefit these promised.

### Poppler
Advantage: high fidelity, widely used. Lost on license (GPL) — same class of
problem as MuPDF — and a heavier, less Go-friendly integration story than
PDFium. Rejected.

## Consequences

- **Positive**: license-clean (BSD) under Adamic's MIT; a single engine serves
  rendering now and text extraction (REQ-2) later; Chrome-grade rendering and
  active upstream. The wasm/purego backend offers a **no-cgo path** that can
  reduce or eliminate R-03 for the platforms where it performs adequately.
- **Negative**: PDFium is a large native/wasm blob to bundle and version-pin
  (recorded as a configuration item, NFR-LIC-01 attribution retained); the cgo
  backend, where used, still carries R-03 (C toolchain, container
  cross-compilation). The binding is a significant third-party dependency, now
  formally accepted here. **Revisit if** the wasm backend proves too slow on
  large documents *and* the cgo cross-compilation cost proves unsustainable —
  the fallback is to reduce supported platforms per release, not to change
  engine.
- **Amends prior decisions (no rewrite)**: resolves the "MuPDF or PDFium"
  wording in the Architecture and Design Document §4.1 in favor of PDFium, and
  refines [ADR-0005](ADR-0005-platform-stack.md)'s blanket cgo assumption — cgo
  is now the *fallback* backend, not the only path. ADR-0005 stands; this
  narrows one of its consequences.
- **Follow-ups**: T2 spike proves render + tri-platform build and picks the
  per-platform backend; T11 sets perf budgets from those measurements; the
  bundled-component license/attribution manifest lists PDFium (BSD).
