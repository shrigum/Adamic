# ADR-0006: MVP launch language is Japanese

- **Status**: Superseded by [ADR-0013](ADR-0013-mvp-language-dutch.md) (MVP language changed to Dutch)
- **Date**: 2026-07-07
- **Feature/trigger**: project kickoff; baselined Architecture and Design Document §5
- **Deciders**: project maintainers (technical lead, Language Pack author)

## Context

The MVP proves the product thesis with one language, exercising the Language
Pack interface end to end. Two profiles were considered: a no-space,
multiple-reading language (Japanese) that exercises segmentation, lemmatization,
and reading overlays; or an inflected, spaced, public-domain language (Latin)
that exercises lemmatization and OCR cheaply. The project prioritizes tackling
the highest-risk path first, so that the riskiest technical work is validated at
MVP rather than deferred.

## Decision

We launch with **Japanese**. It exercises the segmentation and reading pipeline
that is the most differentiating and highest-risk capability, and its
open-source toolchain is mature: a pure-Go tokenizer for segmentation, and
JMdict, furigana data, and KANJIDIC for lookup and character breakdown.

## Alternatives considered

### Latin first
Cheaper to build — spaced text needs no segmenter, and public-domain classical
sources are abundant for testing. It lost because it under-exercises
segmentation, leaving the core thesis (reading an unspaced, multi-reading
language) unproven at MVP. Adopted instead as the second pack (see
[ADR-0011](ADR-0011-language-pack-boundary.md)), where its opposite typology
validates that the pack model generalizes.

## Consequences

- **Positive**: the MVP handles no-space segmentation and multi-granularity
  lookup from the start, deliberately front-loading the riskiest language work;
  the toolchain is mature and openly licensed.
- **Negative**: segmentation/lemmatization quality for Japanese is now on the
  critical path and must be measured against a reference text (risk R-02).
  **Revisit if** the Japanese open-source data proves license-incompatible or
  inadequate (fall back per FR-LP-04 by shipping only the capabilities whose
  data is available).
- **Follow-ups**: Latin designated as pack #2 to validate generalization;
  segmentation/lemmatization accuracy measured during Stage 1.
