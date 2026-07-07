# ADR-0013: MVP launch language is Dutch (supersedes ADR-0006)

- **Status**: Accepted (supersedes [ADR-0006](ADR-0006-mvp-language.md))
- **Date**: 2026-07-07
- **Feature/trigger**: founder decision during REQ-10 (OCR) intake; the concrete
  target corpus is a scanned Dutch coursebook
- **Deciders**: project maintainers (technical lead)

## Context

[ADR-0006](ADR-0006-mvp-language.md) chose **Japanese** as the MVP language to
front-load the hardest language work (no-space segmentation, multi-reading
lookup). That reasoning was sound in the abstract, but the project's actual
first work has diverged from it in two ways that make Dutch the better MVP now:

- **The real target documents are scanned image-only PDFs**, which pushed OCR
  (REQ-10) ahead of text extraction (REQ-2) in the [backlog](../planning/BACKLOG.md).
  The concrete fixture is a **Dutch** A1 coursebook. Building the MVP around the
  language we can actually test end-to-end — from scanned page to OCR to lookup —
  beats building it around a language with no fixture in hand.
- **The founder prioritizes recognition/pipeline accuracy over front-loading the
  hardest typology.** Dutch is Latin-script, spaced, and inflected: it exercises
  OCR, lemmatization, dictionary lookup, familiarity, vocabulary capture, and
  study export — the whole product spine — on a language where an accurate,
  license-clean OCR path exists. The uniquely hard Japanese capabilities
  (segmentation of unspaced text, furigana/reading overlays) are deferred to a
  later, harder pack rather than gating the MVP.

Prior ADRs that used Japanese only as a *typology example* (inflection in
[ADR-0009](ADR-0009-familiarity-by-lemma.md); "typologically hard" in
[ADR-0011](ADR-0011-language-pack-boundary.md)) remain correct and are not
changed by this decision — Japanese is still a language the pack model must
eventually handle. This ADR changes only which language the **MVP ships and is
tested against first**.

## Decision

We launch the MVP with **Dutch**. Dutch is the first Language Pack and the
language every MVP feature (OCR text layer, text extraction, lookup, familiarity,
vocabulary, coverage, study export) is built and tested against. Its properties
— Latin script, word-spaced, inflected — exercise the full product spine while
keeping OCR and lemmatization on a path where accurate, MIT-compatible
open-source components exist. The high-typology-difficulty languages (Japanese
and other unspaced/multi-reading scripts) become **later** packs, added over the
stable Language Pack boundary ([ADR-0011](ADR-0011-language-pack-boundary.md))
without core changes.

## Alternatives considered

### Keep Japanese as MVP (ADR-0006's decision)
Genuine advantage: front-loads the highest-risk language typology, so the
hardest technical work is validated at MVP rather than deferred, and its
open-source toolchain (pure-Go tokenizer, JMdict, KANJIDIC) is mature. It lost
because there is **no Japanese fixture or corpus in play**, while the actual
first feature (OCR) and the actual test document are Dutch; and because the
founder's stated priority is accuracy of the working pipeline over proving the
hardest typology first. Front-loading risk is only valuable if you are building
that risky part now — we are building OCR + the Latin-script spine now.

### Latin (classical) as MVP
Genuine advantage: abundant public-domain corpora, no segmenter needed, and it
was already designated the generalization-proving second pack in ADR-0006/0011.
It lost to Dutch on **concreteness**: the in-hand fixture and the founder's
immediate reading goal are Dutch, a living language with everyday text; classical
Latin's value is as a *typological contrast* pack, a role it still keeps.

### Ship language-agnostic and pick later
Defers the choice. Rejected because the MVP must be tested against *some*
language's lemmatizer/dictionary/OCR model to prove the Language Pack interface
end to end; "no language" leaves the core thesis unexercised, the same reason
ADR-0006 rejected deferring.

## Consequences

- **Positive**: the MVP is built and tested against the language we can actually
  exercise today (a real Dutch fixture, an accurate OCR path, mature Latin-script
  lemmatization/dictionary data); the full product spine is proven on one
  coherent language; the Language Pack boundary is still validated by a real pack.
- **Negative**: the hardest language typology (unspaced segmentation, multiple
  readings) is **no longer proven at MVP** — the risk ADR-0006 wanted to
  front-load (R-02) is deferred to when the first CJK pack is built. We accept
  that segmentation/reading-overlay capabilities are unexercised until then.
  **Revisit if** a later pack reveals the core baked in Latin-script or
  word-spacing assumptions that the pack boundary was supposed to keep out — that
  would be a boundary bug to fix, not a reason to change MVP language.
- **Amends prior decisions**: supersedes [ADR-0006](ADR-0006-mvp-language.md)
  (Japanese → Dutch as MVP). Latin's designated role as a typology-contrast pack
  ([ADR-0011](ADR-0011-language-pack-boundary.md)) stands; Japanese moves from
  "MVP" to "a later, harder pack." Typology examples in ADR-0009/0011 are
  unaffected.
- **Follow-ups**: the OCR feature ([docs/planning/ocr/spec.md](../planning/ocr/spec.md))
  targets Dutch/Latin script first (its assumption A1, now confirmed by this
  ADR); the first Language Pack is Dutch; lemmatization/dictionary data sourcing
  for Dutch (license-compatible, NFR-LIC-01) is an early task for the
  language-pack features (REQ-3/REQ-4).
