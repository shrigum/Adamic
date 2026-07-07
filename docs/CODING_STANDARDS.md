# Coding Standards

Language-agnostic principles first; language-specific addenda at the bottom.
These are the standards the [implementer skill](../.claude/skills/implementer/SKILL.md)
writes to and the [code-reviewer skill](../.claude/skills/code-reviewer/SKILL.md)
reviews against. When a standard here conflicts with the surrounding code, match
the surrounding code and open an issue — consistency beats local correctness.

## Naming

- Names say **what a thing is or does in the domain**, not how it's implemented:
  `settingsPath()`, not `getStringFromOSDirs()`.
- Length proportional to scope: `i` in a three-line loop is fine; a package-level
  variable gets a real name.
- No abbreviations that aren't universal (`cfg`, `ctx`, `err` fine; `stgs`, `mgr` not).
- Booleans read as assertions: `isDefault`, `hasChanges` — never `flag`, `check`.
- One name per concept across the codebase. If specs say "settings", code says
  `settings`, not `config` in one file and `prefs` in another. The
  [glossary](onboarding/GLOSSARY.md) is the arbiter; extend it when you introduce
  a concept.

## Error handling

- **Every error is either handled or propagated with context — never swallowed.**
  Empty catch blocks and ignored return values are review-blocking.
- Add context at each boundary where you have information the caller lacks:
  "load settings: open C:\...\settings.json: permission denied" — the chain reads
  like a story, so wrap with *what you were doing*, not "error occurred".
- **Fail loud and early on programmer errors** (violated invariants); **fail soft
  with a clear message on user/environment errors** (missing file, bad input).
  Deciding which kind an error is, is part of the design, and specs should say
  (the spec template has an "Error behavior" prompt in acceptance criteria).
- Error messages tell the user what to *do*, not just what broke, when an action
  exists: `settings file is corrupt (invalid JSON at line 3); fix it or delete it
  to reset to defaults: <path>`.
- **Own your failure modes.** Every module's doc comment lists how it can fail
  and what state it leaves behind (e.g. "Save is atomic: on any error the
  previous file is intact"). If you can't write that sentence, the design isn't
  done.

## Module boundaries

- Modules expose **capabilities, not internals**: `settings.Load()` is a boundary;
  exporting the path-resolution helper so callers can peek is a leak.
- Dependencies point one way. The domain core imports nothing from CLI/UI layers.
  A cycle between packages means the boundary is drawn wrong.
- Boundaries are where errors gain context, inputs get validated, and tests
  concentrate. A boundary that's hard to test from outside is a wrong boundary.
- New top-level packages/modules are architectural — they need at least a
  design-review note, possibly an ADR (the
  [architecture-reviewer](../.claude/skills/architecture-reviewer/SKILL.md) decides).

## When to abstract vs. inline

- **Rule of three**: extract shared code on the third concrete usage, not the
  second, and never for imagined future ones.
- Duplication is cheaper than the wrong abstraction. Two similar-looking blocks
  that serve different masters will evolve apart; forcing them together creates
  the parameter-flag swamp.
- No layers that only forward calls, no interfaces with one implementation
  "for testability" when a concrete type tests fine, no config options nobody
  asked for. Speculative generality is the most common review finding — expect it.
- The inverse rule: when a function needs a comment to separate its phases,
  those phases want to be functions.

## Comments

- Comments explain **why**, or state constraints code can't express (units,
  invariants, links to specs/ADRs). Comments that restate the code are deleted
  in review.
- Public API gets doc comments (what, failure modes, concurrency expectations).
- `TODO(#123):` with a real issue number, off the critical path only — see the
  [Definition of Done](process/DEFINITION_OF_DONE.md).

## Tests

Detailed methodology lives in the [test-engineer skill](../.claude/skills/test-engineer/SKILL.md).
The standards-level rules:

- Tests accompany the code in the same commit/PR — never "tests to follow".
- Test behavior through public boundaries, not private internals; a test that
  breaks under a pure refactor is testing the wrong thing.
- Table-driven where the language supports it; each case says in its name what
  behavior it pins.
- Tests touch real files only under a temp directory the test owns; never the
  user's actual config/home directories, never the network.

## Dead code and dependencies

- No commented-out code, no unreachable branches, no "keeping it around" —
  git remembers.
- Dependencies: stdlib first. A third-party dependency needs to be OSS, widely
  adopted, actively maintained, account-free — and gets an ADR
  ([Definition of Done](process/DEFINITION_OF_DONE.md)). Copying 30 lines with
  attribution beats importing a 30,000-line library for them.

---

## Language addenda

Add a section per language used in the project. Addenda refine the principles
above for one language; they never contradict them.

### Go (current stack — see [ADR-0001](architecture/ADR-0001-tech-stack.md))

- `gofmt` and `go vet` clean is a merge requirement, enforced by CI and the
  Definition of Done. Formatting is therefore never a review topic.
- Follow [Effective Go](https://go.dev/doc/effective_go) and
  [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) where this
  document is silent.
- Errors: wrap with `fmt.Errorf("doing thing: %w", err)`; sentinel errors as
  package-level `var ErrX = errors.New(...)` only when callers genuinely branch
  on them (`errors.Is`).
- No panics across package boundaries; `panic` only for violated invariants
  that indicate a bug, with a message saying so.
- No goroutines without an owner: whoever starts one is responsible for its
  shutdown and its error. (The example app has none — concurrency is a design
  decision, not a default.)
- Package layout: `src/` holds `package main` (thin CLI wiring only);
  domain logic lives in subpackages (`src/settings/`), each importable and
  testable without the CLI. Integration tests that exercise the built binary
  live in `tests/`.
- Table-driven tests with subtests (`t.Run`); use `t.TempDir()` for all
  filesystem work; use `t.Setenv` to redirect config-dir lookups.

### Adding another language

Copy this heading structure: formatter/linter (must be free + local), canonical
style guide, error-handling idiom, module layout, test idiom. Then record the
language addition itself as an ADR — a second language is an architectural
decision.
