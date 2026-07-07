# Definition of Done

A change is **done** when every applicable item below is true. Every item is
objectively checkable — most by a command, the rest by pointing at a committed
file. The [code-reviewer skill](../../.claude/skills/code-reviewer/SKILL.md)
verifies this list mechanically during stage 5 of the
[planning flow](PLANNING_FLOW.md); "it's basically done" is not a status.

## The checklist

### Code
- [ ] **Builds cleanly**: `go build ./...` exits 0 with no new warnings.
- [ ] **All tests pass**: `go test ./...` exits 0. No skipped tests without a
      linked issue explaining why.
- [ ] **Formatted and vetted**: `gofmt -l ./src ./tests` prints nothing;
      `go vet ./...` exits 0.
- [ ] **Critical-path code meets elevated rigor**: every task marked `[CP]` in the
      feature's critical-path doc has tests covering its error paths, not just its
      happy path (see [CRITICAL_PATH_METHOD.md](../CRITICAL_PATH_METHOD.md#rigor-rule)).
- [ ] **No unresolved TODOs on the critical path**: `grep -rn "TODO\|FIXME" src/`
      returns nothing attributable to this change's critical-path tasks. Off-path
      TODOs are allowed only in the form `TODO(#issue):` with a real issue number.
- [ ] **No dead code**: nothing added that is unreachable, commented-out, or
      "for later". Version control remembers deleted code; the working tree doesn't
      have to.
- [ ] **No new dependencies without an ADR** (per the OSS-first rule in
      [CONTRIBUTING.md](../../CONTRIBUTING.md#ground-rules-from-the-project-principles)).

### Spec conformance
- [ ] **Every acceptance criterion in the spec passes**, and each one is covered by
      at least one automated test (name the test next to the criterion when closing
      the feature — the spec template has a column for this).
- [ ] **Nothing beyond the spec shipped**: no drive-by features, no speculative
      hooks. If scope changed, the spec was amended first.

### Documentation
- [ ] **Changelog entry** added under `Unreleased` in [CHANGELOG.md](../../CHANGELOG.md),
      written for users ("Settings persist across runs"), not for developers
      ("Refactored config loader").
- [ ] **ADR written and indexed** if the change is architectural — new dependency,
      new persistent format, new module boundary, new external interface, or a
      reversal of a previous ADR. When in doubt, the
      [architecture-reviewer skill](../../.claude/skills/architecture-reviewer/SKILL.md)
      decides at stage 3, and its decision is recorded in the design review.
- [ ] **Docs sync complete**: README, onboarding, and glossary updated for anything
      a new contributor would need (stage 6; the
      [docs-maintainer skill](../../.claude/skills/docs-maintainer/SKILL.md) has the
      full trigger list).
- [ ] **Planning trail complete**: the feature folder contains spec, critical-path
      doc (with all task statuses final), and design review.

### Process
- [ ] **Review findings resolved**: zero open BLOCKING findings from stage 5.
      NON-BLOCKING findings either fixed or converted to issues.
- [ ] **CI green** on the PR (CI runs the same build/test/format commands —
      it should never be the first place you learn something fails).

## What "done" is not

- Not "works on my machine" — the tests are the arbiter, and they run in CI on
  Linux regardless of what OS you developed on.
- Not "merged" — merged with a missing changelog entry is not done; it is debt
  with a timestamp.
- Not "the happy path works" — for critical-path code, the error paths are the
  acceptance bar.
