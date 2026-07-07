# ADR-0010: Spaced repetition is export-first for the MVP; in-app review later

- **Status**: Accepted
- **Date**: 2026-07-07
- **Feature/trigger**: project kickoff; baselined Architecture and Design Document §5 (Study Service); FR-SRS-03/04/05/06
- **Deciders**: project maintainers (technical lead, product owner)

## Context

Collected vocabulary and mined sentences must be reviewable to be useful.
Building a spaced-repetition scheduler (a review queue, an interval algorithm,
review-outcome bookkeeping, a review UI) is significant work. A mature,
open-source spaced-repetition tool — Anki — already exists and is widely used by
exactly Adamic's audience of language learners. The MVP should not reinvent a
solved tool before it has proven the reading-and-capture loop.

## Decision

The MVP is **export-first**: Adamic captures vocabulary and mined sentences and
exports them to Anki package format (`.apkg`) and CSV; it does not build an
in-app scheduler for the MVP. An in-app spaced-repetition review mode is a
planned later milestone that reuses the same captured data model.

## Alternatives considered

### In-app spaced-repetition in the MVP
Higher stickiness — the reader never leaves Adamic to review, and review data
stays in one place. It lost because it reinvents a mature, widely adopted tool
and materially expands MVP scope, competing with the higher-risk reading and
language-core work that must land first. Deferred, not dropped (FR-SRS-05/06
remain in the baseline).

## Consequences

- **Positive**: MVP scope stays focused on the differentiating reading loop;
  learners keep their existing Anki workflow immediately; the later in-app
  review reuses the same data model with no rework.
- **Negative**: export fidelity (card templates, sentence context, attached
  audio, deck naming) is a first-class MVP requirement, not an afterthought;
  users who do not use Anki have no in-app review until the later milestone.
  **Revisit** when the reading and study-capture loop is stable and in-app
  review is scheduled (FR-SRS-05/06).
- **Follow-ups**: high-fidelity `.apkg` and CSV export; ensure captured cards
  carry everything the later in-app scheduler will need.
