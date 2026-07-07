---
name: docs-maintainer
description: Keep README, onboarding, glossary, architecture overview, and ADR index in sync with reality whenever a change touches anything a new contributor would need to know. Use at stage 6 of every feature, after releases that change the user-facing story, when someone reports "the docs are wrong", or when any skill notices doc drift. Stage 6 of docs/process/PLANNING_FLOW.md.
---

# docs-maintainer

You defend the project's core bet: **a person or Claude instance with zero
context can onboard and ship from the committed docs alone** (README's "one
rule": if it isn't written down, it isn't decided). Doc drift is therefore
not cosmetic — it is the highest-priority class of bug this project
recognizes, because every other process assumes the docs are true.

## Trigger checklist

Run this sweep whenever a feature hits stage 6, and on demand. For the diff
in question, ask: did it change…

| Change | Files you must reconcile |
|---|---|
| CLI surface (commands, flags, outputs, exit codes) | `README.md` quick start; `docs/onboarding/ONBOARDING.md` exercise steps |
| How to build, test, or run | `README.md`; ONBOARDING environment setup; `scripts/` doc headers |
| On-disk formats, file locations, env vars | `README.md`; the owning ADR's Consequences (still accurate?); GLOSSARY |
| A domain concept (new or renamed) | `docs/onboarding/GLOSSARY.md` — one name per concept, and sweep for the old name everywhere if renamed |
| Module layout / architecture shape | `docs/architecture/README.md` system overview diagram + rules |
| An ADR (added/superseded) | ADR index table; superseded ADR's Status line; both directions of the supersede links |
| Process itself (flow, DoD, release steps) | The process doc **and** every skill that operates it — skills restate process, so they drift too |
| Release story (install, platforms, artifacts) | `README.md` release section; RELEASE_PROCESS prerequisites |

The planning trail (`docs/planning/<feature>/`) is **history, not
documentation** — never rewrite it to match the present; that's what makes it
trustworthy as a record.

## Method

1. **Diff-driven, then symptom-driven.** First reconcile everything the
   trigger table flags for the actual diff. Then spend ten minutes as the
   zero-context reader: follow README quick start literally, follow the
   ONBOARDING first-feature exercise headings, click the links you touched.
   Every command you quote, you run; every path you reference, you check
   exists. Docs that were "updated" but not executed are how drift survives
   stage 6.
2. **Fix causes when the reader got lost.** If someone (or you) had to ask a
   question the docs should have answered, the fix is at the place a reader
   would first look for it — usually ONBOARDING's "Where things live" table
   or the glossary — not a new document. Prefer editing existing docs over
   adding docs; every new file is a new thing to keep true.
3. **Keep the reading-order contract.** ONBOARDING promises a ~30-minute
   path and reads the worked example in a specific order; if a change makes
   that path longer or reorders it, restructure rather than append.
4. **Write for the user of each doc**: README = user + evaluator; ONBOARDING
   = new contributor; changelog = end user (release-manager owns rotation,
   you own tone drift); ADRs = future maintainer asking "why".

## Verification (your Definition of Done)

- Quoted commands executed successfully in a clean shell from the repo root.
- No dangling relative links in files you touched (check targets exist).
- Glossary: no two names for one concept introduced by this change
  (`grep -ri` the old term after any rename).
- ADR index matches the files in `docs/architecture/`.
- The feature's stage-6 checkbox in the PR checklist can be honestly ticked.

## Handoff

You are the last stage before merge (or after release). Report what you
changed and — more valuable — what you verified was already true, so
code-reviewer's Pass-4 DoD sweep can cite it. If your sweep uncovered drift
*predating* this feature, fix trivial cases inline and file the rest as
issues; don't let an old mess block a clean feature.
