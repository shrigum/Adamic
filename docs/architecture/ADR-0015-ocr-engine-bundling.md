# ADR-0015: Bundle the pinned OCR engine and MVP model in release artifacts

- **Status**: Accepted
- **Date**: 2026-07-20
- **Feature/trigger**: [docs/planning/ocr-engine-bundling/](../planning/ocr-engine-bundling/)
- **Deciders**: founder + architecture review

## Context

[ADR-0014](ADR-0014-ocr-engine.md) chose Tesseract behind the Recognizer seam
and left delivery open ("packaging a stripped Tesseract per platform" as a
later task). Today the app only *detects* an engine (env override or PATH), so
a fresh install cannot recognize scanned books — the product's core promise —
without a manual, technical Tesseract install. Delivery must be decided.

The candidate mechanisms conflict with different standing decisions:
an in-app downloader touches [ADR-0003](ADR-0003-update-check.md)'s invariant
(*no network request except the explicit update check*); bundling grows the
release artifact (founder tolerance: 200 MB/platform); doing nothing fails the
product promise. Multi-language is in view: additional OCR models belong to
Language Packs ([ADR-0011](ADR-0011-language-pack-boundary.md); ocr spec A10),
so this decision covers the **engine** and the **MVP Dutch model** only.

## Decision

**Each platform's release artifact ships the desktop app with a
self-contained, version-pinned, checksum-verified Tesseract engine plus the
MVP Dutch model (`tessdata_best` `nld`) beside it.** Specifics:

- **Discovery precedence**: explicit user override (`ADAMIC_TESSERACT` env,
  then the settings entry) → bundled engine → system PATH. The bundle beats a
  system install so the product runs the engine it was tested with; an
  explicit override beats everything.
- **Pinning**: engine and model versions + SHA-256 checksums are committed
  build inputs; a release cannot silently float. Sources must be pinned,
  checksummed, and license-verified (both components are Apache-2.0,
  NFR-LIC-01); the attribution manifest records them.
- **Assembly at build time**: bundles are produced by committed scripts
  during the release build (CI or local). Build-time fetching is outside
  ADR-0003's jurisdiction, which governs the **application at runtime** —
  the shipped app still makes no network request. The invariant stands
  unmodified.
- **Sourcing per platform**: Windows extracts and trims the UB-Mannheim
  build (proven by the ocr T2 spike). macOS/Linux sourcing is settled by the
  feature's T1 spike in preference order: pinned upstream binaries if a
  relocatable set exists, else a CI source build; the chosen pins are
  recorded in the spike findings and become the committed build inputs. A
  platform that cannot be satisfied re-opens this ADR's scope for that
  platform rather than shipping an unpinned or system-dependent engine.
- **Soft absence stays**: a missing/corrupt bundle degrades to the existing
  "recognition unavailable" posture; the reader is never impaired.

## Alternatives considered

### One-click in-app download install (founder's initial direction)
Advantage: smallest base download; consent-gated; a natural home for many
optional models later. Lost because it adds a second runtime network surface
(supersede/extend ADR-0003), a hosting + integrity + resume/failure UX the
project would carry forever, and — decisive — the per-platform engine
sourcing work is identical anyway, while the 200 MB artifact tolerance
removes bundling's only real cost. Revisit condition: when Language Pack
distribution is designed, pack delivery (which may include OCR models) may
adopt a download flow; engine delivery may join it then.

### Detect-only (require a system Tesseract)
Advantage: zero artifact growth, zero redistribution obligations. Lost
because a fresh install cannot OCR — the default experience of the flagship
use case would be a manual dependency install, unacceptable for a consumer
reading app. Retained only as the PATH fallback at lowest precedence.

### Bundle every language model
Advantage: all languages offline out of the box. Lost on size (~1.5 GB for
tessdata_best) and on architecture: language-specific data is Language Pack
territory (ADR-0011); the core bundles exactly the MVP language
([ADR-0013](ADR-0013-mvp-language-dutch.md)).

### Ship the engine inside the Go binary (embed + extract at runtime)
Advantage: single-file artifact. Lost because extracting executables to user
dirs at first run is exactly the pattern AV/quarantine heuristics punish,
complicates macOS signing (nested executables must be signed in place), and
buys nothing over a directory beside the app.

## Consequences

- **Positive**: OCR works offline out of the box on all three platforms;
  ADR-0003's invariant survives untouched; versions are reproducible and
  attributable; the advanced override formalizes the dev workflow that
  already exists.
- **Negative**: artifacts grow to roughly 50–130 MB (ceiling 200 MB,
  enforced loudly at release build); the release pipeline must build per-OS
  (the desktop shell cannot cross-compile) — a matrix restructure; the
  project now redistributes third-party binaries, making the attribution
  manifest (ocr T10) load-bearing for compliance; macOS
  signing/notarization of a nested engine executable joins the existing
  pre-1.0 code-signing question (ADR-0003 follow-ups).
- **Follow-ups**: Language Pack distribution design decides model delivery
  for further languages (revisit hook above); the T1 spike findings append
  the concrete macOS/Linux pins to the feature folder.
