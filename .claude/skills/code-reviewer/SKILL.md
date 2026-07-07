---
name: code-reviewer
description: Review a diff against the Definition of Done, the coding standards, and the originating spec's acceptance criteria; produce a findings list split into BLOCKING and NON-BLOCKING. Use when stage 4 implementation is complete, when a PR needs review, or when someone says "review this against the spec". Stage 5 of docs/process/PLANNING_FLOW.md.
---

# code-reviewer

You review **against documents, not taste**. Every finding cites its source:
a spec AC, a Definition-of-Done item, a coding-standards rule, or a design-
review condition. A finding you can't source is an opinion — either propose it
as a standards change (separate PR) or drop it. This is what keeps review
consistent across humans and agents and keeps authors from relitigating.

## Inputs (gather all before reading the diff)

1. The diff (PR or working tree).
2. `docs/planning/<feature>/spec.md` — the AC table with covering tests.
3. `docs/planning/<feature>/critical-path.md` — which touched code is `[CP]`.
4. `docs/planning/<feature>/design-review.md` — conditions to verify.
5. [docs/process/DEFINITION_OF_DONE.md](../../../docs/process/DEFINITION_OF_DONE.md)
   and [docs/CODING_STANDARDS.md](../../../docs/CODING_STANDARDS.md).

No spec for a non-trivial change? That **is** the review: one BLOCKING
finding, route to **spec-writer**, stop. Don't review code whose contract
doesn't exist.

## Method — four passes, in this order

### Pass 1: Mechanical (run, don't eyeball)
`go build ./...`, `go test ./...`, `go vet ./...`,
`gofmt -l ./src ./tests`, `grep -rn "TODO\|FIXME" src/` (CP code: any hit is
BLOCKING; elsewhere: must be `TODO(#issue):`). Any failure is BLOCKING and
you still continue to the other passes — authors deserve the full list in one
round, not a drip-feed.

### Pass 2: Spec conformance (both directions)
- **Under-delivery**: walk the AC table. Each AC: is its covering test named,
  does that test exist, does it actually assert the criterion (read the test
  body — a test named for the AC that asserts something weaker is the classic
  false green)?
- **Over-delivery**: anything in the diff serving no AC and no task in the
  plan — drive-by features, speculative hooks, "while I was in there"
  refactors. BLOCKING, not because the code is bad but because unspecced
  behavior is unowned behavior. (Route: amend spec, or split it out.)

### Pass 3: Standards, rigor-weighted
Line-by-line on `[CP]`-task code; brisker elsewhere. Hunt in order of damage:
1. Swallowed errors / missing context wrapping / user-facing messages with no
   action to take.
2. Failure-mode claims: does the module doc's failure-modes paragraph match
   what the code does, and do tests pin it? (Atomicity claimed but no
   leftover-temp-file check = unverified claim.)
3. Boundary leaks (exported internals, domain code printing to stdout,
   dependency direction).
4. Abstraction violations (rule of three, single-implementation interfaces,
   forwarding layers).
5. Naming/glossary drift (one name per concept — check GLOSSARY.md).

### Pass 4: Definition of Done sweep
Walk every DoD checkbox not already covered above: changelog entry present
and user-worded; ADR present if design review said one was needed; planning
docs' statuses final; docs sync done or explicitly queued to
**docs-maintainer**.

## Output

Findings list (PR review, or `docs/planning/<feature>/review.md` for
local-only work):

```markdown
## Review: <feature> — <date>
Verdict: CHANGES-REQUIRED | APPROVED

### BLOCKING
1. [spec AC6] Corrupt-file path: file is rewritten on error — spec requires
   it be left intact. src/settings/settings.go:88. Suggest: …

### NON-BLOCKING
1. [standards: naming] `cfgMgr` → glossary says "settings". src/main.go:31.
```

BLOCKING = spec violation, DoD failure, correctness bug, or standards breach
in CP code. NON-BLOCKING = everything else; each gets fixed now or becomes a
real issue — review comments don't have an afterlife.

Every finding: source citation, file:line, and a concrete suggested fix.
Findings without all three are not findings.

## Verdict discipline

APPROVED means you would own this code's failure modes yourself. Re-review
after fixes checks the fixes *and* re-runs Pass 1 — fixes regress things.
Two consecutive rounds discovering new BLOCKING findings in the same area is
a signal to stop reviewing and send it back to stage 3 — the design, not the
diff, is the problem.
