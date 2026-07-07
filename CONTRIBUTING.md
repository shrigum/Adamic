# Contributing

This document is the contract for getting changes merged. The planning flow below
is **required** — it is enforced by review, and the Claude Skills in
[.claude/skills/](.claude/skills/) automate it stage by stage. If you are brand
new, read [docs/onboarding/ONBOARDING.md](docs/onboarding/ONBOARDING.md) first;
it walks the whole flow with a worked example.

## The required planning flow

Every **feature** (and any bug fix that changes behavior or touches architecture)
passes through these stages. Each stage's output is a **committed file** — that is
the enforcement mechanism. A PR whose feature has no spec and no critical-path doc
in `docs/planning/<feature-name>/` is declined without code review.

Copy this checklist into your PR description and check items off:

```markdown
## Planning-flow checklist
- [ ] 1. Intake — `docs/planning/<feature-name>/spec.md` committed
        (skill: spec-writer). Ambiguities resolved or listed as explicit assumptions.
- [ ] 2. Critical path — `docs/planning/<feature-name>/critical-path.md` committed
        (skill: critical-path-planner). Task graph, dependencies, critical path,
        risk flags.
- [ ] 3. Design review — `docs/planning/<feature-name>/design-review.md` committed
        (skill: architecture-reviewer). If the design is architectural, the ADR it
        produced is linked here.
- [ ] 4. Implementation — code + tests, task-by-task off the critical-path doc,
        critical-path tasks first (skills: implementer, test-engineer).
- [ ] 5. Review — findings list resolved; all BLOCKING findings closed
        (skill: code-reviewer). Reviewed against the spec's acceptance criteria
        and the Definition of Done.
- [ ] 6. Docs sync — README/onboarding/ADRs updated for anything a new
        contributor would need to know (skill: docs-maintainer).
- [ ] 7. Changelog — entry added under Unreleased in CHANGELOG.md.
```

Stage 7 (release) is not per-PR; releases batch merged work — see
[docs/process/RELEASE_PROCESS.md](docs/process/RELEASE_PROCESS.md).

### When can you skip stages?

Almost never, but proportionality is allowed for **trivial changes**:

| Change | Required trail |
|---|---|
| Typo/doc-only fix | None. PR description suffices. |
| Bug fix, no behavior/interface change | Failing test that reproduces the bug + fix. No spec needed; the test *is* the spec. |
| Bug fix that changes documented behavior | Full flow (it's a feature in disguise). |
| New feature, any size | Full flow. "It's small" is not an exemption — small features get small specs (a spec can be half a page). |
| Architectural change | Full flow + ADR. |

If you're unsure whether something is "trivial", it isn't.

## PR expectations

- **One feature (or one coherent fix) per PR.** Planning docs for a feature may be
  committed in the same PR as the implementation or in an earlier docs-only PR —
  the latter is better for anything that takes more than a day, because it lets
  design review happen before code exists.
- **Tests pass locally before you push**: `go test ./...` and `gofmt -l ./src ./tests`
  must both be clean. CI runs the same commands; it is a backstop, not your test runner.
- **The diff matches the spec.** Scope that grew during implementation goes back
  through intake (amend the spec, note the change) — silent scope growth is the
  main thing reviewers hunt for.
- **Meet the [Definition of Done](docs/process/DEFINITION_OF_DONE.md).** Every item
  there is objectively checkable; the code-reviewer skill checks them mechanically.
- **Write PR descriptions for the archaeologist.** Someone reading this PR in two
  years with no context should understand what changed and why from the description
  plus the linked planning folder.

## Commit style

- Imperative subject line, ≤ 72 chars: `Add settings file persistence`, not
  `Added some settings stuff`.
- Reference the feature's planning folder in the body when relevant:
  `See docs/planning/settings-file/`.
- No merge-commit noise on feature branches; rebase before opening the PR.

## How a brand-new contributor picks a first task

1. Read [docs/onboarding/ONBOARDING.md](docs/onboarding/ONBOARDING.md) end-to-end
   (~30 minutes, includes environment setup).
2. Look at open issues labeled `good-first-issue`, or at any feature folder in
   `docs/planning/` whose critical-path doc has unclaimed **off-critical-path**
   tasks — those are safe to pick up in parallel and get lighter review
   (see [docs/CRITICAL_PATH_METHOD.md](docs/CRITICAL_PATH_METHOD.md)).
3. Claim the task by commenting on the issue or editing the critical-path doc's
   task table (`Owner` column) in a small PR.
4. Do the work through the flow above. For your first PR, expect the reviewer to
   be pedantic about process — that is the template working as intended.

## Working with the Claude Skills

Each stage of the flow has a skill in [.claude/skills/](.claude/skills/). They are
designed to chain: each one reads its predecessor's committed output, so you can
tell a fresh Claude instance "run the planning flow for feature X" and it will
find the state on disk. Humans are welcome to do any stage by hand — the skills
define the *methodology*, and the committed files are the interface, so a
hand-written spec and a spec-writer spec are interchangeable.

## Ground rules (from the project principles)

- **Local-first, OSS-first.** New dependencies must be open source, widely adopted,
  and must not require an account, API key, or network access at runtime. A cloud
  integration, if ever justified, must be optional and degrade gracefully offline —
  and needs an ADR.
- **No speculative abstraction.** Build for the current spec, not imagined futures.
  The rule of three applies: abstract on the third concrete use, not before.
- **Simplest correct design wins ties.** If two designs meet the acceptance
  criteria, the one with fewer moving parts is the right one.
- **Explicit tradeoffs.** "We chose X" without "over Y, because Z" is an
  incomplete decision — in specs, ADRs, and PR descriptions alike.
