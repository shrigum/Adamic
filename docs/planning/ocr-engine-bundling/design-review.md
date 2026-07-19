# Design review: OCR engine bundling

- **Stage**: 3 — Design review ([planning flow](../../process/PLANNING_FLOW.md))
- **Reviewer**: architecture-reviewer skill
- **Inputs**: [spec.md](spec.md), [critical-path.md](critical-path.md),
  [docs/architecture/README.md](../../architecture/README.md), full ADR index
  (0001–0014), [CODING_STANDARDS.md](../../CODING_STANDARDS.md),
  [CONTRIBUTING.md](../../../CONTRIBUTING.md)
- **Date**: 2026-07-20

## Verdict: APPROVED-WITH-CONDITIONS

Proceed to stage 4 **once the T1 spike passes on all three platforms** (C1 —
it is the gate, exactly as the ocr feature's engine spike was). The plan is
well-shaped: two independent tracks, the High risk scheduled first, every
task serving an AC. Findings below are guards on scope, not restructuring.

## Fit with existing architecture

- **Matches the intended shape** ✔. The baselined architecture diagram
  already places "native libs (PDF engine, OCR, voices)" beside the core as
  shipped components; bundling the OCR engine implements that box. No new
  runtime module appears: discovery stays inside `src/ocr/tesseract` (the
  engine binding's existing confinement), the override is a plain settings
  preference (the settings file's remit per ADR-0002/0008), packaging lives
  in `scripts/` + CI.
- **ADR-0003 invariant survives untouched** ✔ — the decisive coherence
  point. Bundling moves all fetching to **build time**; the shipped app
  still makes no network request outside the explicit update check. No
  supersession, no extension. (The rejected installer alternative would have
  required one; that comparison is recorded in ADR-0015.)
- **ADR-0014 is executed, not contradicted** ✔. Its consequences named
  "bundle a per-platform binary + model (packaging task)" and "version-pin
  engine + model" — this feature is that task. ADR-0015 records the delivery
  decision and cross-links.
- **Language Pack boundary respected** ✔. Only the MVP Dutch model ships in
  core (ADR-0013); further models are explicitly fenced to packs
  (ADR-0011), with the revisit hook recorded in ADR-0015.
- **License surface** ✔ handled: redistribution makes the attribution
  manifest load-bearing; AC3 + the cross-feature dependency on ocr T10 are
  correctly recorded in the plan.

## ADR decision

**Required** — the design redistributes third-party binaries in release
artifacts (dependency-class change), changes the release-artifact layout and
build policy (cross-cutting), and executes a delivery question ADR-0014 left
open. → Produced: **[ADR-0015](../../architecture/ADR-0015-ocr-engine-bundling.md)**,
with the installer, detect-only, bundle-everything, and embed-in-binary
alternatives honestly weighed. Index row added.

## Rulings on the spec's open questions

1. **Sourcing (Q1)**: policy in ADR-0015 — pinned + checksummed +
   license-verified sources only; Windows = UB-Mannheim extract-and-trim
   (proven); macOS/Linux settled **by the T1 spike** in preference order
   (pinned upstream binaries, else CI source build), pins recorded in the
   spike findings. An unsatisfiable platform re-opens ADR-0015's scope for
   that platform — it does not ship unpinned.
2. **macOS layout (Q2)**: not decidable from the armchair — Gatekeeper and
   codesigning constrain where a nested executable may live
   (`Contents/MacOS`-adjacent vs `Resources`). The **T1 spike on macOS must
   produce the evidence** and T4 adopts it; recorded as condition C6. The
   Windows/Linux answer stays "engine dir beside the executable" (already
   exercised daily via the dev override).
3. **CLI release fate (Q3)**: **drop the CLI scaffold from release
   artifacts** — the desktop app is the product, and shipping two binaries
   confuses the release story. `./src` keeps building and testing in CI (it
   is still the module's main package) until REQ-1's scaffold retirement
   removes it with its own trail. Condition C5; founder may override by
   spec amendment.
4. **Settings entry (Q4)**: one additive key, `ocrEnginePath` (absolute path
   to the engine executable; empty = unset), env var still winning;
   additive-only per the settings store's existing envelope discipline
   (ADR-0004) — no version bump, no migration. Condition C4.

## Complexity check (simplest correct design?)

- **No new abstractions proposed — keep it that way (C2).** Discovery
  remains `tesseract.Find` with a constant per-platform relative-path table.
  Do **not** grow an "engine manager", engine registry, or multi-engine
  configuration surface — one bundled engine, one override, per ADR-0014's
  C2 discipline next door.
- **A5's "re-check when inputs change" is a watchpoint (C3).** The honest
  minimum is: discovery runs at launch and when the override setting
  changes. No filesystem watchers, no polling — a user who deletes the
  bundle mid-session gets the soft failure on next use, which is enough.
- **T5 correctly resists a second build system** — `release.sh` stays the
  source of truth CI mirrors. Hold that line during implementation.
- **No speculative scope found**: every task maps to an AC (T1→AC1/7,
  T2→AC4/6, T3→AC4/5, T4/T5→AC1/2/7, T6→AC3, T7→AC8); T8/T9 are stage
  requirements. Nothing to cut.
- **Under-design check**: soft corrupt-bundle behavior (AC6), loud release
  gates (AC2), per-platform proof (AC7/A6), and error-path coverage are all
  tasked. Artifact **signing** is *not* in scope — correctly so; it stays
  the tracked pre-1.0 code-signing question (ADR-0003 follow-ups, restated
  in ADR-0015's consequences).

## Conditions

| # | Condition | Rationale | Status |
|---|---|---|---|
| C1 | **T1 spike passes on all three platforms before T4/T5 start.** A failing platform goes back to this review/ADR-0015 with the measured finding (source-build cost or platform deferral) — no unpinned or system-dependent fallback. | High risk on the CP → built first ([rigor rule](../../CRITICAL_PATH_METHOD.md)); the ADR's sourcing policy depends on its evidence. | todo |
| C2 | Discovery stays `tesseract.Find` + a constant per-platform path table; no engine-manager/registry/multi-engine config. | Rule of three; ADR-0014 C2's one-engine discipline extends to delivery. | todo |
| C3 | Override re-check = launch + setting change only; no fs watchers or polling. | Simplest behavior satisfying A5/AC5; watchers are speculative machinery. | todo |
| C4 | Settings key `ocrEnginePath`, additive only, env var precedence preserved, existing envelope discipline (no version bump). | SemVer surface (ADR-0004) touched minimally. | todo |
| C5 | Release artifacts ship the desktop app only; the CLI scaffold leaves the release (keeps building/testing until REQ-1 retires it). | One product story per artifact; scaffold's removal gets its own trail. | todo |
| C6 | The macOS engine location is adopted from T1-spike evidence (codesign/quarantine-compatible) and recorded in the spike findings + ADR-0015 follow-up. | Gatekeeper constraints are empirical; guessing risks unlaunchable artifacts. | todo |

## Notes for the implementer

- The T1 spike is a **script + CI proof**, not throwaway: the committed
  assembly script is the deliverable, the existing real-engine tests are its
  harness (point them at the produced bundle — no new test framework).
- Keep the pin file (versions + URLs + SHA-256) a single committed source of
  truth that the assembly script, the attribution manifest (T6), and the
  size gate (T5) all read — one place to bump an engine version.
- `tesseract.Find`'s bundle probe must resolve **relative to the running
  executable** (`os.Executable`), not the working directory — the desktop
  app is launched from anywhere.
- The settings re-check (C3) wires through the existing `app.EnableOCR`
  seam; do not add a second enable path.
- T7's inspection language should be written together with ocr T13's owner
  (same C5-style subprocess concern there) so one inspection covers both
  features' claims.
