# ADR-0007: Faithful fixed-layout reader for the MVP; reflow deferred

- **Status**: Accepted
- **Date**: 2026-07-07
- **Feature/trigger**: project kickoff; baselined Architecture and Design Document §2, principle 5
- **Deciders**: project maintainers (technical lead)

## Context

Adamic reads book PDFs, and readers value both a faithful reproduction of the
printed page and adjustable readability (reflow). These pull in opposite
directions: fixed layout is reliable and maps cleanly to annotation and lookup
coordinates, while reflow re-derives a linear text flow and is far harder to get
right against script-correctness (RTL, vertical, unspaced) and complex layouts.
The MVP must ship a correct, reliable reader before it optimizes readability.

## Decision

The MVP uses a **faithful fixed-layout reader**. Reflow is deferred to a later
milestone as an optional view layered on the same extracted-text model, so it
does not require reworking the reader core when added.

## Alternatives considered

### Reflow-first
Higher learner value — adjustable font size and spacing help readers of long
texts, and it aids accessibility. It lost on risk and cost: reflow against the
full range of target scripts is high-risk and disproportionately expensive for
the MVP, and getting it wrong would undermine the reading experience the product
depends on. Deferred, not dropped.

## Consequences

- **Positive**: simpler, more reliable rendering and annotation/lookup
  coordinate mapping for the MVP; the extracted-text model built now is reused
  by reflow later.
- **Negative**: adjustable readability is limited to zoom until reflow is added
  (NFR-A11Y-01 is partially met at MVP for fixed layout only). **Revisit** when
  the fixed-layout reader and text-extraction model are stable — reflow is a
  planned later milestone (FR-NAV-11).
- **Follow-ups**: keep the extracted-text model independent of the fixed-layout
  renderer so reflow layers on without a rewrite.
