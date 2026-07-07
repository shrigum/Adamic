# ADR-NNNN: <decision as a short assertive phrase>

- **Status**: Proposed | Accepted | Rejected | Superseded by [ADR-MMMM](ADR-MMMM-slug.md)
- **Date**: YYYY-MM-DD
- **Feature/trigger**: link to `docs/planning/<feature>/` or the issue that forced the decision
- **Deciders**: who accepted this (names or "project maintainers")

## Context

What situation forces a decision? State the forces in tension (requirements,
constraints from prior ADRs, the local-first/OSS-first principles, effort,
risk). Write it so a reader in two years understands the pressure without
any other context. 2–4 paragraphs; if it needs more, the decision is probably
several decisions.

## Decision

One paragraph, active voice, present tense: "We use X to do Y." State the
decision precisely enough that someone could check whether the codebase
conforms to it.

## Alternatives considered

For each serious alternative (including "do nothing"):

### <Alternative>
What it is, its genuine advantages, and the specific reason it lost. An
alternative with no listed advantages wasn't seriously considered — go back
and think, or delete it.

## Consequences

The honest bill, both directions:

- **Positive**: what gets easier or safer.
- **Negative**: what gets harder, what we're locked into, what we'll have to
  revisit and under what conditions ("revisit if X" is the most useful sentence
  in an ADR).
- **Follow-ups**: concrete tasks this decision creates, with issue links.

<!--
Usage:
1. Copy to ADR-NNNN-short-slug.md (next free number, zero-padded).
2. Fill in every section — an empty "Alternatives" section means it's not an
   architecture decision, it's a description.
3. Add the row to the index in README.md in the same PR.
4. Once Accepted, never edit substance; supersede with a new ADR instead.
   Typo fixes and link repairs are fine.
-->
