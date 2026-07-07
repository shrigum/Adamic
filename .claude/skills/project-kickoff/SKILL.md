---
name: project-kickoff
description: One-prompt instantiation of this template into a real project — collects the required inputs (identity, repo, requirements list), performs every mechanical rename, writes the product brief and feature backlog, runs the first feature through intake, verifies the result builds and tests green, and makes the initial commit. Use when someone says "start a new project", "kick off <name>", "instantiate the template", or pastes the kickoff prompt from docs/onboarding/KICKOFF.md. Runs once per project, immediately after duplicating the template.
---

# project-kickoff

You turn a fresh copy of the template into the start of a real project in one
session, ending in **full dev mode**: a repo where the very next action anyone
takes is picking a feature off the backlog and running the normal
[planning flow](../../docs/process/PLANNING_FLOW.md) — no leftover
placeholders, no unwritten decisions, no setup debt.

You run **once**. If `go.mod` no longer says `example.com/app`, kickoff has
already happened — say so and stop (the planning flow, not this skill, handles
everything after kickoff).

## Phase 0 — Collect inputs (the contract)

The human-facing explanation of these inputs, with a copy-paste prompt, is
[docs/onboarding/KICKOFF.md](../../docs/onboarding/KICKOFF.md). Gather all of
them **before touching any file**. Ask for missing REQUIRED items once, as a
single batched list — never drip questions, and never guess these:

**REQUIRED (no defaults possible):**
| # | Input | Why it's needed |
|---|---|---|
| R1 | **Project name** (human-readable) and one-sentence purpose | README identity, product brief |
| R2 | **Problem statement** (a paragraph: who has what problem, why now) | Product brief; the root of every future spec |
| R3 | **Initial requirements list** (bullet list; each item one user-visible capability, roughly one feature each) | Becomes the backlog; the first item becomes the first spec |
| R4 | **GitHub `owner/repo`** (or explicit "none yet") | Module path, changelog links, update-check target, release process |

**OPTIONAL (apply this default and record it as an assumption if absent):**
| # | Input | Default |
|---|---|---|
| O1 | Binary/CLI name | kebab-cased project name |
| O2 | Go module path | `github.com/<owner>/<repo>` |
| O3 | Target platforms | the template's five (win/mac/linux, amd64+arm64) |
| O4 | License + copyright holder | MIT + owner name |
| O5 | Settings dir name / env-var prefix | binary name / SCREAMING_SNAKE of it |
| O6 | Product form (CLI / desktop GUI / self-hosted service) | CLI |
| O7 | Non-goals the founder already knows | none recorded |

**Gate before proceeding:** requirements sanity. Read R3/O6 against the
standing ADRs and flag conflicts *now*, when changing course is free:
- Needs a rich native **GUI** → ADR-0001 says revisit the Go decision; stop
  and resolve (possibly: wrong template) before renaming a hundred things.
- Needs **structured/queryable data** → note that ADR-0002/0004 cover
  preferences only; a data store will be its own feature + ADR.
- Needs **network/cloud** anywhere → ADR-0003's invariant (no network outside
  explicit `app update`) must be consciously superseded, not ignored.
- Any requirement that is an implementation ("use SQLite") → convert to the
  underlying need and note the original as a constraint candidate.

Present conflicts and assumptions, get one confirmation, then execute all
phases without further questions.

## Phase 1 — Mechanical instantiation

The automated version of the README's manual checklist. Order matters; verify
each with the stated check:

1. **Module rename**: `go.mod`, imports in `src/main.go`, build target in
   `tests/cli_test.go` `TestMain`. Check: `grep -r "example.com/app" .`
   returns nothing.
2. **Repo links**: `OWNER/REPO` in `CHANGELOG.md` link refs; `update.Repo` in
   `src/update/update.go` (leave the placeholder + a `TODO(#issue)` only if
   R4 was "none yet"). Check: `grep -rn "OWNER/REPO" .` hits at most the
   documented placeholder default in `update.go`'s comment.
3. **Identity**: binary name in `scripts/build.sh` + `build.ps1` (artifact
   names and `-o` targets), `dirName` in `src/settings/settings.go`, env-var
   prefix (`APP_CONFIG_DIR`, `APP_UPDATE_URL` → `<PREFIX>_…`) in both
   packages **and** every doc/test that names them (grep, don't recall).
4. **License**: year + holder in `LICENSE` (swap the license file itself if
   O4 says non-MIT).
5. **README rewrite**: replace the template's self-description with the
   project's (name, purpose, quick start with real binary name). **Keep**:
   repository map, the one rule, planning-flow links, release instructions.
   **Delete**: the "Starting a new project from this template" section —
   it's done its job.
6. **Changelog reset**: fresh `Unreleased` only; no versions yet (the
   template's history belongs to the template).
7. **Example code disposition**: keep `src/settings/`, `src/update/`, and
   their planning trails/ADRs — they are live features of the new app, not
   examples anymore. The greeting command stays as scaffold until the first
   real feature replaces it (note this in the backlog as a cleanup item).
   ADR numbering continues from 0005.

## Phase 2 — Product brief

Write `docs/PRODUCT.md` — the one document specs trace back to:
problem statement (R2) · users · the requirements list verbatim (numbered
`REQ-1…`, so specs can cite them) · founder non-goals (O7) · explicitly
recorded assumptions from Phase 0 · what "1.0" plausibly means. This is a
*brief*, not a spec — one page, no acceptance criteria.

If Phase 0 surfaced an architectural deviation the founder confirmed
(GUI, data store, network), record it now as ADR-0005+ rather than leaving it
oral. Update the ADR index.

## Phase 3 — Backlog and first intake

1. Write `docs/planning/BACKLOG.md`: the requirements as an ordered feature
   table — `REQ | feature-name (kebab, future folder name) | one-line outcome |
   status (backlog/in-flow/shipped)`. Order by founder priority, then by
   dependency. This file is where "what should I work on?" gets answered;
   the critical-path grep in ONBOARDING finds in-flight work, BACKLOG.md
   holds the queue.
2. Run **spec-writer** on the first backlog item, for real — producing
   `docs/planning/<first-feature>/spec.md`. This is not ceremony: it proves
   the pipeline works end-to-end in the new repo, and it means "full dev
   mode" ends with a concrete next action (stage 2 of feature #1), not a
   blank page.

## Phase 4 — Verification (all must pass before you claim done)

- `grep -r "example.com/app\|OWNER/REPO" .` — clean (per Phase-1 caveats).
- `go test -count=1 ./...`, `go vet ./...`, `gofmt -l ./src ./tests` — green.
- `./scripts/build.sh` — binary builds; `<binary> --version` prints `dev`;
  the settings round-trip works with the new identity
  (`config set` / `config get` / `config path` shows the renamed dir).
- `<binary> update` — reports "not configured" only if R4 was "none yet";
  otherwise it must attempt the real repo (a 404 before the first release is
  the correct answer — say so in your report).
- Docs link check on files you touched: relative links resolve.
- `docs/PRODUCT.md`, `docs/planning/BACKLOG.md`, and the first spec exist and
  cross-reference (spec cites its `REQ-n`).

## Phase 5 — Commit and hand off

1. Initial commit: `Initialize <project> from local-first template` — one
   commit containing the whole instantiation, so `git log` starts with an
   honest record of what was generated vs. authored.
2. If R4 exists and the human wants it: `gh repo create` / add remote / push —
   **ask before pushing**; publishing is theirs.
3. Report, in this order: what was renamed, assumptions recorded (with where),
   ADR conflicts resolved and how, the backlog, and **the single next action**:
   "run critical-path-planner on `docs/planning/<first-feature>/`".

From that point the project is in full dev mode: this skill never runs again,
and every subsequent change goes through the ordinary planning flow.
