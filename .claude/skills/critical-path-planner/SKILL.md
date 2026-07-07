---
name: critical-path-planner
description: Turn a committed spec into a task graph with dependencies, identify the critical path, flag parallelizable work and risk hotspots, written to docs/planning/<feature>/critical-path.md. Use after spec-writer completes stage 1, when someone says "plan the tasks", "what's the critical path", or when scope changes require re-planning. Stage 2 of docs/process/PLANNING_FLOW.md.
---

# critical-path-planner

You produce stage-2 plans following [docs/CRITICAL_PATH_METHOD.md](../../../docs/CRITICAL_PATH_METHOD.md)
exactly — that document is the methodology; this skill is its operating
procedure. Your output determines build order, rigor allocation, and what a
second contributor or agent can safely pick up in parallel. The worked example
to match: [docs/planning/settings-file/critical-path.md](../../../docs/planning/settings-file/critical-path.md).

## Inputs (all required — refuse if missing)

1. `docs/planning/<feature>/spec.md` — committed and free of unresolved
   ambiguity. If it has open questions or is missing, stop and send the work
   back to **spec-writer**; planning on a vague spec produces confident
   nonsense.
2. The codebase — estimates require looking at what exists. Read the packages
   the feature will touch before estimating anything.
3. `docs/architecture/README.md` — the module shape constrains task boundaries.

## Method

1. **Derive tasks from acceptance criteria, backwards.** Every AC must be
   producible by some task; every task must serve some AC or a stage
   requirement (docs sync, ADR, integration tests). A task serving no AC is
   scope creep — cut it or send the spec back for amendment.
2. **Enforce granularity.** 1–8 hours each; split anything larger. Each task
   states an *outcome* (what is true when done) and names its verification.
   Include non-code tasks (ADR, docs, changelog) — they have dependencies too.
3. **Record only direct dependencies**, then check the graph is a DAG. A cycle
   means a task boundary is wrong — split until it isn't.
4. **Compute the critical path** per the method doc (earliest-finish forward
   pass, walk back from the max). Show your arithmetic in a "Path check" line —
   it lets reviewers verify in ten seconds and catches your own errors.
5. **Rate risk per task** (Low/Med/High per the method doc's definitions).
   Then apply the two rules that make risk ratings actionable:
   - **High risk on the critical path ⇒ scheduled first**, usually as a
     time-boxed spike; say so explicitly in the Risks section, and say what a
     spike failure would invalidate.
   - Every Med/High risk gets a one-line mitigation, not just a label.
6. **Mark parallelization.** Note which off-path tasks unlock when, and which
   are suitable for a new contributor or secondary agent (off-path + Low risk
   + no shared files with in-flight CP tasks).

## Output

`docs/planning/<feature>/critical-path.md` containing, in order:

1. Header (stage, source spec link, date, status).
2. The blockquote at the very top:
   `> **Critical path (Nh): T1 → T3 → …**` — this exact format; other tooling
   and humans grep for it.
3. Mermaid `graph LR` of the DAG.
4. The task table with columns
   `ID | Task (outcome) | Est (h) | Depends on | On CP? | Risk | Status | Owner`,
   all statuses `todo`, owners `—`.
5. Path-check arithmetic line.
6. Risks section (per rule 5 above).
7. Parallelization notes.

Commit it. Hand off to **architecture-reviewer** (stage 3) — it will check
your plan for over-design as well as the design itself. Do not start
implementation; stage 3 is a gate.

## Re-planning

You are re-invoked when: the spec is amended materially, estimates drift
(actuals ≥ 2× estimate on any CP task), or tasks are added/split. Recompute the
path — it moves more often than intuition says — and update the blockquote and
`On CP?` flags in the same commit. Never edit history: statuses and the
Revision-history line in the spec carry the story.

## Anti-patterns to refuse

- A "task" that is a activity without a checkable outcome ("investigate X" —
  make it a spike with a question it must answer and a time box).
- Estimating in days or weeks (split it), or padding estimates instead of
  flagging risk (the Risk column is where uncertainty goes).
- A graph where everything depends on everything — that's a missing
  decomposition insight; find the seam (usually: schema/contract first, then
  producers and consumers in parallel).
