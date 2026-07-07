# Planning Flow

Every feature moves through seven stages. Each stage has a **required committed
artifact**, an **owner skill** (a human can substitute at any stage — the artifact
is the interface), and an **exit criterion** that must be true before the next
stage starts. The full rationale for why we plan before coding is in the project
principles ([CONTRIBUTING.md](../../CONTRIBUTING.md#ground-rules-from-the-project-principles));
this document is the operational reference.

A fully worked example of the entire flow lives in
[docs/planning/settings-file/](../planning/settings-file/) — read it alongside
this document.

## Stage overview

| # | Stage | Skill | Committed artifact | Exit criterion |
|---|-------|-------|--------------------|----------------|
| 1 | Intake | [spec-writer](../../.claude/skills/spec-writer/SKILL.md) | `docs/planning/<feature>/spec.md` | No open ambiguities: every open question is answered or converted to an explicit, labeled assumption. Acceptance criteria are testable. |
| 2 | Critical path analysis | [critical-path-planner](../../.claude/skills/critical-path-planner/SKILL.md) | `docs/planning/<feature>/critical-path.md` | Task graph is a valid DAG, critical path identified, every task ≤ 1 day of work, risks flagged with mitigations. |
| 3 | Design review | [architecture-reviewer](../../.claude/skills/architecture-reviewer/SKILL.md) | `docs/planning/<feature>/design-review.md` (+ ADR in `docs/architecture/` if architectural) | Verdict is APPROVED or APPROVED-WITH-CONDITIONS with all conditions folded into the plan. ADR merged if one was required. |
| 4 | Implementation | [implementer](../../.claude/skills/implementer/SKILL.md) + [test-engineer](../../.claude/skills/test-engineer/SKILL.md) | Code + tests, committed per task | All critical-path tasks done with tests; task statuses updated in critical-path.md. |
| 5 | Review | [code-reviewer](../../.claude/skills/code-reviewer/SKILL.md) | Findings list (PR review or `review.md` in the feature folder) | Zero unresolved BLOCKING findings. Diff satisfies spec acceptance criteria and [Definition of Done](DEFINITION_OF_DONE.md). |
| 6 | Docs sync | [docs-maintainer](../../.claude/skills/docs-maintainer/SKILL.md) | Updated README/onboarding/ADR index/etc. as needed | A new contributor reading only the docs would not be misled by anything this feature changed. |
| 7 | Release (batched, not per-feature) | [release-manager](../../.claude/skills/release-manager/SKILL.md) | Rotated CHANGELOG.md, git tag, GitHub Release with artifacts | Release checklist in [RELEASE_PROCESS.md](RELEASE_PROCESS.md) complete. |

## Rules that make the flow work

1. **Artifacts are committed, not chatted.** A spec that exists only in a
   conversation, an issue thread, or someone's head does not exist. This is what
   lets anyone — including a Claude instance with zero prior context — resume a
   feature mid-flight: the entire state of the feature is on disk in its
   planning folder.

2. **Stages can iterate, but not silently.** Discovering during implementation
   that the spec was wrong is normal. The response is: amend `spec.md` (with a
   dated note in its Revision history section), re-check whether the critical
   path changed, then continue. The response is never "quietly build something
   different from the spec".

3. **Stage 3 is a gate, not a rubber stamp.** The architecture-reviewer's job
   includes rejecting designs for being *more* elaborate than the spec requires.
   REJECTED sends the feature back to stage 1 or 2 with written reasons.

4. **Critical-path tasks first.** Within stage 4, tasks on the critical path are
   implemented before off-path tasks whenever dependencies allow, and they get
   the elevated rigor defined in
   [CRITICAL_PATH_METHOD.md](../CRITICAL_PATH_METHOD.md#rigor-rule). Off-path
   tasks may proceed in parallel (good first tasks for new contributors).

5. **Proportionality.** A half-day feature gets a half-page spec and a
   five-task graph. The flow scales down; it does not switch off. The exemption
   table for truly trivial changes is in
   [CONTRIBUTING.md](../../CONTRIBUTING.md#when-can-you-skip-stages).

## Feature folder layout

```
docs/planning/<feature-name>/
  spec.md              # stage 1 — problem, non-goals, constraints, acceptance criteria
  critical-path.md     # stage 2 — task graph, DAG, critical path, risks, task status
  design-review.md     # stage 3 — verdict, conditions, ADR link if any
  review.md            # stage 5 — only if review happened outside a PR (e.g. local-only work)
```

`<feature-name>` is kebab-case, short, and stable (it will be referenced from
commits, ADRs, and the changelog): `settings-file`, not
`add-support-for-a-local-settings-file-v2`.

## Resuming a feature with no context

To pick up a feature cold (the test this whole system is designed to pass):

1. `ls docs/planning/<feature-name>/` — which artifacts exist tells you which
   stage it is in.
2. Read `spec.md` for *what and why*, `critical-path.md` for *what's left*
   (the task table has per-task status), `design-review.md` for *constraints on how*.
3. Continue at the first incomplete stage. That's it — if you needed information
   not in those files, that is a process bug: fix the docs, then continue.
