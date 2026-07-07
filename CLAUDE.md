# CLAUDE.md

Entry point for Claude instances working in this repo. Humans: start at
[docs/onboarding/ONBOARDING.md](docs/onboarding/ONBOARDING.md) instead.

## What this is

**Adamic** — a local-first, open-source PDF reader and editor with an
integrated language-learning layer for reading full-length books in a foreign
language, fully offline. Read [docs/PRODUCT.md](docs/PRODUCT.md) first for the
problem, users, and numbered requirements, then
[docs/planning/BACKLOG.md](docs/planning/BACKLOG.md) for what's next.

Stack: **Go core + web frontend, packaged with Wails v3, desktop only**
(Windows/macOS/Linux) — see [ADR-0005](docs/architecture/ADR-0005-platform-stack.md),
which amends the template's Go-CLI posture in
[ADR-0001](docs/architecture/ADR-0001-tech-stack.md). Structured user data lives
in **SQLite** ([ADR-0008](docs/architecture/ADR-0008-local-data-storage.md),
superseding the settings-file model of ADR-0002/0004). Language-specific
behavior lives only in **Language Packs** behind stable interfaces
([ADR-0011](docs/architecture/ADR-0011-language-pack-boundary.md)); the core has
none. Shipping = GitHub Release with binaries attached; no deployments, no
cloud/SaaS dependencies, no network beyond the opt-in update check (ADR-0003).

This project was instantiated from the local-first application template. The
inherited settings-file and update-check features remain live (with their
planning trails in [docs/planning/](docs/planning/)); the greeting command in
`src/main.go` is scaffold to be replaced by the first real feature (REQ-1).
Kickoff has already run — do not run the project-kickoff skill again.

## The non-negotiable process

**No feature gets implemented without a committed spec and critical-path
analysis.** Every feature passes through the seven stages in
[docs/process/PLANNING_FLOW.md](docs/process/PLANNING_FLOW.md); each stage has
a matching skill in [.claude/skills/](.claude/skills/) and a required committed
artifact in `docs/planning/<feature>/`. The skills chain — each reads its
predecessor's file — so to work on a feature, first check which artifacts exist
in its folder; that tells you the current stage. If asked to "just implement"
something with no spec, run the flow from stage 1 (it scales down: small
features get small specs), or cite the exemption table in
[CONTRIBUTING.md](CONTRIBUTING.md#when-can-you-skip-stages) if it truly qualifies.

## Commands (all local, all free)

```bash
go test ./...                      # full test suite (unit + integration; builds the binary)
go vet ./... && gofmt -l ./src ./tests   # must be clean before any commit
go run ./src --name You            # run the current scaffold
./scripts/build.sh                 # build for current platform → dist/adamic
./scripts/release.sh X.Y.Z         # cut a release (verifies, builds all targets, tags — never pushes)
```

## Load-bearing rules (full versions in the linked docs)

- If a decision isn't written down, it doesn't count as decided. Undocumented
  discoveries → doc fix in the same PR.
- [Definition of Done](docs/process/DEFINITION_OF_DONE.md) is objective; check
  it, don't approximate it. Changelog entry under Unreleased in every
  behavior-changing PR.
- Critical-path tasks (marked ✅/`[CP]` in the feature's critical-path.md) get
  error-path tests and strict review; TODOs on them are forbidden
  ([docs/CRITICAL_PATH_METHOD.md](docs/CRITICAL_PATH_METHOD.md)).
- Stdlib first; any new dependency needs an ADR. New on-disk format, module
  boundary, or CLI surface → likely needs an ADR: the architecture-reviewer
  skill makes the call at stage 3.
- `package main` is wiring only; logic lives in importable subpackages; tests
  never touch real user directories (use the `ADAMIC_CONFIG_DIR` override).
- Code style: [docs/CODING_STANDARDS.md](docs/CODING_STANDARDS.md); names for
  concepts: [docs/onboarding/GLOSSARY.md](docs/onboarding/GLOSSARY.md).
