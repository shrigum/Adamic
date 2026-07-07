# ADR-0009: Familiarity state keyed by lemma, not surface form

- **Status**: Accepted
- **Date**: 2026-07-07
- **Feature/trigger**: project kickoff; baselined Architecture and Design Document §6; FR-FAM-02
- **Deciders**: project maintainers (technical lead)

## Context

Familiarity coloring and the vocabulary bank need a stable key for "this word."
A word appears in many surface forms — inflected, conjugated, declined — and the
reader's knowledge is of the word, not each form. Keying familiarity to the
surface form would mark every inflection of a known word as a separate unknown,
which is wrong for inflected languages (Latin, Japanese verbs/adjectives) and
would make coverage and coloring misleading. This is a core-loop correctness
decision, and it creates a dependency on the Language Pack's lemmatizer.

## Decision

We key familiarity state to the **(language, lemma)** pair, not the surface
form. States are unknown, learning, known, and ignored. Where lemmatization is
ambiguous, we record the candidate lemmas and resolve display by context where
the pack provides it, defaulting to the most frequent candidate otherwise.

## Alternatives considered

### Key by surface form
Simpler and independent of lemmatizer quality — no morphological analysis
needed to record or look up a state. It lost because it is incorrect for
inflected languages: marking one inflection "known" would leave every other
inflection of the same word "unknown," undermining coverage, coloring, and the
whole core loop. Rejected.

## Consequences

- **Positive**: marking one inflection known correctly covers all inflections of
  the lemma; coverage and coloring reflect real knowledge; the vocabulary bank
  keys cleanly to headwords.
- **Negative**: the feature depends on the pack's lemmatizer, so lemmatizer
  quality is now a tested, per-pack concern (risk R-02); ambiguous lemmas
  require the defined resolution rule rather than being left undefined.
  **Revisit if** a target language's lemmatization is too unreliable to key on
  — the contingency is user-correctable segmentation/lemmas (R-02).
- **Follow-ups**: measure lemmatizer accuracy per pack against a reference text;
  implement the ambiguity default (most-frequent candidate) and
  user-correction path.
