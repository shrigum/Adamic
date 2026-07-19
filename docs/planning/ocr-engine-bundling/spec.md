# Spec: OCR engine bundling

- **Stage**: 1 — Intake ([planning flow](../../process/PLANNING_FLOW.md))
- **Author**: spec-writer skill, from founder direction (2026-07-20)
- **Date**: 2026-07-20
- **Status**: Draft — ready for stage 2 (critical-path-planner)
- **Requirement**: REQ-10 (OCR) — delivery/packaging slice; follows the
  [ocr feature](../ocr/spec.md), whose T2 spike findings left "packaging a
  stripped Tesseract" as a later task. This spec is that task, decided in
  favor of **bundling** after weighing an in-app download installer (see
  Non-goals and Revision history).

## Problem

The OCR feature (REQ-10) treats the recognition engine as optional equipment:
the app looks for a Tesseract installation at startup and, finding none,
reports text recognition as unavailable. That is the right *failure* posture,
but today it is also the *default experience* — nothing installs the engine,
so a user who downloads an Adamic release and opens a scanned book gets no
OCR unless they install and configure Tesseract themselves (a multi-step,
technical task nobody should need). The engine the product depends on for its
core promise — reading scanned books — must arrive with the product.

At the same time, advanced users may want to run a *different* engine build
or model than the one we choose (a newer Tesseract, a differently trained
model). Today that works only via an undocumented environment variable.

So: **a fresh install of Adamic must be able to recognize a scanned Dutch
book offline, out of the box, on every supported platform, with zero
additional steps — while an advanced user can still point the app at their
own engine.**

## Non-goals

- **No in-app download/installer for the engine.** Considered and set aside
  (founder decision 2026-07-20): with a ~200 MB artifact tolerance, bundling
  removes the entire download/consent/integrity/hosting surface and — the
  decisive point — keeps [ADR-0003](../../architecture/ADR-0003-update-check.md)'s
  standing invariant ("no network request except the explicit update check")
  intact, so OCR works offline from the first launch. *Revisit condition*:
  when Language Pack distribution is designed (see next item), if packs adopt
  an in-app download flow, engine/model delivery may join it.
- **No additional language models.** V1 bundles exactly the MVP Dutch model
  ([ADR-0013](../../architecture/ADR-0013-mvp-language-dutch.md)). Further
  languages arrive as **Language Packs**
  ([ADR-0011](../../architecture/ADR-0011-language-pack-boundary.md)); the
  ocr spec (A10) already anticipates a pack supplying the OCR model for its
  language. Bundling every tessdata model (~1.5 GB) is excluded by size.
- **No engine auto-update.** The bundled engine/model are version-pinned and
  change only with an app release (ADR-0014 consequence). No separate update
  channel.
- **No VLM backend packaging.** The optional high-accuracy backend named in
  [ADR-0014](../../architecture/ADR-0014-ocr-engine.md) stays unbuilt
  (design-review condition C2); nothing here prepares for it.
- **No system-wide install.** The engine lives inside the app's own
  directory; no registry entries, PATH edits, admin elevation, or shared
  system locations. Uninstalling the app removes the engine with it.
- **No engine-management UI.** The advanced override is a setting an advanced
  user edits deliberately; there is no browse/detect/manage engines screen.
  Revisit if support burden proves otherwise.

## Constraints

- **Offline invariant unchanged.** The app makes no network request at
  runtime beyond ADR-0003's explicit update check. Bundling must not add any
  runtime network path; whatever fetching happens occurs at **release build
  time** in CI (build-time network is not constrained by ADR-0003).
- **License compliance (NFR-LIC-01).** The bundled engine and model must be
  redistribution-compatible with the MIT product. Both are Apache-2.0
  (Tesseract engine; `nld.traineddata` from tessdata_best — verified in
  [ADR-0014](../../architecture/ADR-0014-ocr-engine.md) and the T2 spike),
  and both must appear in the component/attribution manifest the ocr
  feature's T10 establishes (its AC11 test then covers them).
- **Size ceiling.** The founder accepts up to **200 MB** per platform
  artifact; the working target is far lower (stripped engine ~15–40 MB +
  8.5 MB model + ~29 MB app ≈ **50–80 MB**). The T2 spike measured the
  un-stripped UB-Mannheim trim at 128.6 MB — an upper bound that still fits
  the ceiling, so a stripped build is an optimization, not a gate.
- **Per-platform engine builds.** Windows, macOS, and Linux each need a
  self-contained engine bundle (founder: all three in v1). Only Windows has a
  proven portable bundle today (T2 spike); macOS/Linux bundles must be proven
  the same way the spike proved Windows — the existing real-engine tests run
  against them in CI (spike follow-up note).
- **The release artifact is the desktop app.** `scripts/build.sh` currently
  builds the CLI scaffold (`./src`), not the Wails desktop shell
  (`./desktop`). Bundling presupposes fixing this: release artifacts must
  ship the desktop app with the engine beside it. This spec owns that
  packaging change for the desktop artifact; the CLI's fate is stage-2/3's
  call to keep or drop from releases.
- **Engine discovery stays soft.** The existing failure posture is retained:
  a missing/corrupt engine (bundled or overridden) means OCR reports itself
  unavailable and the reader works fully; never a crash (ocr spec error
  summary).
- **Pinned versions.** Engine and model versions are pinned in the build
  (ADR-0014 consequence) and recorded in the attribution manifest; a release
  cannot silently float to a newer engine.

## Assumptions

- **A1 — Discovery precedence: explicit override, then bundled engine, then
  system PATH.** `ADAMIC_TESSERACT` (and the settings entry, A4) wins over
  the bundled engine, which wins over a `tesseract` found on PATH. Reasoning:
  an explicit user choice must never be shadowed; the bundle must beat
  whatever random system version exists so the product runs the engine it
  was tested with; PATH remains a useful last resort on platforms/builds
  without a bundle.
- **A2 — The bundle lives beside the app executable** (e.g.
  `<app dir>/tesseract/` with `tessdata/` inside, per-platform layout
  refined at design — macOS app bundles have their own conventions).
  Reasoning: no separate install location to manage or migrate; deleting the
  app deletes the engine (non-goal: system-wide install). *(low confidence —
  confirm the macOS .app layout at design review.)*
- **A3 — Engine bundles are produced/fetched at release build time in CI
  from pinned, checksummed sources, committed to the repo as build scripts,
  not binaries.** The exact sourcing per platform (extract the UB-Mannheim
  build as the spike did; a pinned upstream release; or building Tesseract
  from source in CI) is solution space for stage 3. Reasoning: no large
  binaries in git; reproducible by pin+checksum; ADR-0003 untouched since
  it's build-time.
- **A4 — The advanced override is the existing `ADAMIC_TESSERACT`
  environment variable plus a persistent setting** (settings store,
  ADR-0002/0008) naming the engine executable, with the env var taking
  precedence over the setting for one-off/testing use. The override points
  at an engine *installation* (executable + its `tessdata/`), which is also
  how an advanced user runs a custom model. Reasoning: founder explicitly
  asked for the advanced escape hatch; env-only is undiscoverable.
- **A5 — Engine availability is (re-)checked when discovery inputs change,
  not only at process start.** At minimum: launch and when the override
  setting changes. Reasoning: a user fixing their override shouldn't need a
  restart; full hot-plug watching of the filesystem is not required. *(low
  confidence — confirm the exact re-check trigger set at design.)*
- **A6 — macOS/Linux bundles are proven by CI running the existing
  real-engine tests (T2 spike test, T4/T5 integration tests) against the
  bundled engine on those platforms.** Reasoning: the tests already encode
  "the engine recognizes the fixture"; pointing them at the bundle is the
  cheapest honest proof, per the spike's own follow-up note.
- **A7 — The one-click install direction is fully superseded for v1.** No
  consent dialog, no download UI ships. Reasoning: founder decision
  2026-07-20 after the bundling-vs-install comparison; recorded so the
  earlier direction isn't half-implemented.

## Acceptance criteria

| # | Criterion | Covering test |
|---|---|---|
| AC1 | On a machine with **no system Tesseract and no network**, a fresh release artifact opens the scanned Dutch fixture and "Recognize text" is offered and succeeds (units recognized), on each supported platform. | |
| AC2 | Each platform's release artifact, including the bundled engine + model, is ≤ the committed size-ceiling constant (200 MB founder ceiling; the constant records the actual target). A release build fails the check loudly if exceeded. | |
| AC3 | The bundled engine and model versions are pinned in the build inputs and recorded, with licenses, in the component/attribution manifest; the ocr feature's manifest test (AC11) passes against the bundled pair. | |
| AC4 | Discovery precedence holds: with an override set (env or setting), the overridden engine is used even when a bundle exists; with no override, the bundle is used even when a different `tesseract` is on PATH. Observable via which engine path recognition reports/uses. | |
| AC5 | An invalid override (path missing or not a usable engine) degrades softly: OCR reports itself unavailable with a message naming the override and what to fix; the reader is unaffected; clearing the override restores the bundled engine per A5's re-check. | |
| AC6 | Deleting/corrupting the bundled engine directory degrades softly to the existing "recognition unavailable" posture — never a crash, reader unaffected. | |
| AC7 | The release build produces **desktop-app** artifacts (not the CLI scaffold) for Windows/macOS/Linux with the engine bundled, via the standard release script; CI proves AC1's recognition path on each platform per A6. | |
| AC8 | No runtime network: the shipped artifact performs no network request in any OCR path (bundled discovery included) — the ocr feature's AC10 inspection extended to the bundled layout. | |

**Error behavior summary** (per
[CODING_STANDARDS.md](../../CODING_STANDARDS.md#error-handling)): every
failure in this feature is a **soft user/environment condition** — missing or
corrupt bundle, invalid override, unusable engine — reported as "text
recognition unavailable" with a message that names the cause and the fix
(e.g. the override path to correct); the reader never degrades. Loud failures
belong only to the **release build** (size ceiling exceeded, checksum
mismatch when fetching pinned sources, manifest entry missing), where a human
is present to fix them.

## Open questions for design review (stage 3)

1. **Per-platform sourcing** (A3): extract-and-trim the UB-Mannheim build
   (proven on Windows), pinned upstream binaries, or build Tesseract from
   source in CI? License and reproducibility both bear on this; likely an
   ADR (new third-party redistribution + release-artifact layout), or an
   extension of ADR-0014.
2. **macOS artifact layout** (A2): where does the engine sit in a .app
   bundle, and does notarization/quarantine affect a bundled executable?
3. **CLI scaffold's release fate**: keep shipping `dist/adamic` alongside
   the desktop app or drop it from releases (it is scaffold pending REQ-1's
   full replacement)?
4. **Settings surface for the override** (A4): exact setting name/shape in
   the settings schema (a SemVer surface per ADR-0004).

## Revision history

- 2026-07-20 — Initial version. Founder direction began as a **one-click
  in-app engine install** (consent → download → done); after comparison
  (network-invariant impact, hosting/integrity surface, identical
  per-platform sourcing work either way, ~200 MB artifact tolerance, and
  multi-language models belonging to Language Packs per ADR-0011/ocr-A10),
  the founder chose **bundling** for v1 with the advanced local-engine
  override kept. The install idea is preserved as a non-goal with its
  revisit condition (Language Pack distribution design).
