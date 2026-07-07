# Onboarding: Zero to Shipped Feature

Audience: a brand-new contributor — human or Claude instance — with **no prior
context**. Time budget: ~30 minutes reading + setup, then you can ship. If
anything here is wrong or missing, that is a bug of the highest priority in this
project's value system: fix it in your first PR.

**What this project is:** Adamic — a local-first, offline PDF reader and editor
with an integrated language-learning layer. Start with
[docs/PRODUCT.md](../PRODUCT.md) (problem, users, requirements) and
[docs/planning/BACKLOG.md](../planning/BACKLOG.md) (what's next). Adamic was
instantiated from a local-first application template; the settings-file and
update-check features you'll see below are inherited scaffold that became real
features of Adamic, and the process machinery in `docs/process/` and
`.claude/skills/` transfers unchanged.

## 1. Environment setup (10 minutes)

Everything is free, open source, and account-free (GitHub itself excepted).

1. **Go 1.24+** — [go.dev/dl](https://go.dev/dl/). Verify: `go version`.
2. **Git** — verify: `git --version`. On Windows, install Git for Windows; its
   Git Bash runs the project's `.sh` scripts.
3. **`gh` CLI** *(optional — only for cutting releases without CI)* —
   [cli.github.com](https://cli.github.com/).
4. Clone, then verify the world works before touching anything:

```bash
git clone <repo-url> && cd <repo>
go test ./...          # must pass
go run ./src --name You
```

If `go test ./...` fails on a fresh clone, stop and report it — a broken main
branch outranks whatever you came here to do.

## 2. Mental model of the project (10 minutes)

Three ideas explain everything else:

**Idea 1 — The repo is the memory.** Nothing counts as decided, planned, or
known unless it's committed. Consequence: you never need to ask anyone what the
state of a feature is; you read its folder (and if you *did* have to ask, you
fix the docs after). Where things live:

| Question | Read |
|---|---|
| What is this project / how do I run it? | [README.md](../../README.md) |
| How is work supposed to flow? | [docs/process/PLANNING_FLOW.md](../process/PLANNING_FLOW.md) |
| What does "done" mean, exactly? | [docs/process/DEFINITION_OF_DONE.md](../process/DEFINITION_OF_DONE.md) |
| Why is the code/stack the way it is? | [docs/architecture/README.md](../architecture/README.md) + ADRs |
| What's the state of feature X? | `docs/planning/<x>/` — which files exist = which stage it's in |
| How should code be written? | [docs/CODING_STANDARDS.md](../CODING_STANDARDS.md) |
| What does this term mean here? | [GLOSSARY.md](GLOSSARY.md) |

**Idea 2 — Plan, then code, with a paper trail.** Every feature passes through
spec → critical path → design review → implementation → review → docs sync,
each stage leaving a committed file. The stages have matching Claude Skills in
[.claude/skills/](../../.claude/skills/) that read each other's outputs, so the
flow works identically whether a human or an agent drives each stage.

**Idea 3 — Rigor follows the critical path.** Each feature's plan identifies
the longest dependency chain of tasks; tasks on it get the strictest tests and
review and are built first, tasks off it can be parallelized with lighter
review (and are ideal first tasks). Method:
[docs/CRITICAL_PATH_METHOD.md](../CRITICAL_PATH_METHOD.md).

**The codebase itself** (2 minutes, it's small on purpose): read
[docs/architecture/README.md](../architecture/README.md#system-overview), then
[src/main.go](../../src/main.go) top to bottom, then skim
[src/settings/settings.go](../../src/settings/settings.go) and
[src/update/update.go](../../src/update/update.go). `main` is thin wiring;
domain logic lives in subpackages; state is plain files in user dirs; the
update package is the only code allowed to touch the network (ADR-0003).

## 3. Where to find the current critical path

Each active feature keeps its critical path in a blockquote **at the top of**
`docs/planning/<feature>/critical-path.md`, with a task table below showing
per-task status and owners. To see what the project is working on right now:

```bash
grep -r "Critical path" docs/planning/*/critical-path.md
```

Tasks with `Status = todo` and no owner are up for grabs; off-critical-path
ones are the safest to start with.

## 4. Study the worked example (5 minutes)

The feature "local settings file" (inherited from the template, now a live
Adamic feature) was pushed through the entire flow and left its full trail.
Read the files **in this order** — this is the project's whole process taught
by example:

1. [docs/planning/settings-file/spec.md](../planning/settings-file/spec.md) —
   note the Assumptions section (ambiguity converted to explicit assumptions)
   and testable acceptance criteria with their covering tests named.
2. [docs/planning/settings-file/critical-path.md](../planning/settings-file/critical-path.md) —
   the task DAG, the highlighted path, risk flags, final statuses.
3. [docs/planning/settings-file/design-review.md](../planning/settings-file/design-review.md) —
   an approval **with conditions**, and the trigger that demanded
   [ADR-0002](../architecture/ADR-0002-settings-file-format.md).
4. The result: [src/settings/settings.go](../../src/settings/settings.go),
   its unit tests beside it, integration tests in
   [tests/](../../tests/), and the entry in [CHANGELOG.md](../../CHANGELOG.md).

## 5. First-feature exercise

A deliberately small feature to walk the flow yourself (or with the skills).
Suggested exercise — **`app config unset <key>`**: remove a key so the built-in
default applies again.

1. **Intake**: create `docs/planning/config-unset/spec.md` (use the
   [spec-writer skill](../../.claude/skills/spec-writer/SKILL.md) or copy the
   structure from the worked example). Questions you must answer or convert to
   assumptions: what happens when unsetting a key that isn't set? An unknown key?
2. **Critical path**: `critical-path.md`. Yes, even for a feature this small —
   it will have ~4 tasks and take ten minutes, and the point is the habit.
3. **Design review**: run the architecture-reviewer. Expected outcome: approved,
   *no ADR needed* (no new format, dependency, or boundary) — confirming that a
   feature *doesn't* rate an ADR is a decision worth recording too.
4. **Implement + test**: extend `src/settings/`, tests first or alongside.
   Match the existing table-driven test style.
5. **Review**: run the code-reviewer against your diff, spec, and the
   [Definition of Done](../process/DEFINITION_OF_DONE.md). Fix BLOCKING findings.
6. **Docs sync + changelog**: `app config` commands are documented in the README
   quick start — update it; add your Unreleased changelog entry.

When that PR merges, you are onboarded — you've touched every part of the system.

## 6. Getting unstuck

- Process question → [docs/process/](../process/). Term → [GLOSSARY.md](GLOSSARY.md).
- "Why is X like this?" → `git log --follow` on the file, the ADR index, and the
  feature's planning folder, in that order.
- Genuinely undocumented? Decide something reasonable, **write it down** (spec
  assumption, ADR, or doc fix), and proceed. Documented-but-wrong beats
  undocumented-and-stalled; review will catch the former.
