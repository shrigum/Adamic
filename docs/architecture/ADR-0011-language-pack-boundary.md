# ADR-0011: Language Pack plugin boundary

- **Status**: Accepted
- **Date**: 2026-07-07
- **Feature/trigger**: project kickoff; baselined Architecture and Design Document §5; FR-LP-01..07, FR-EXT-01..04, NFR-MAINT-01
- **Deciders**: project maintainers (technical lead)

## Context

Adamic must support languages with radically different writing systems and
grammar (Japanese, Latin, and later others), including packs built by external
contributors. Languages differ enough that hard-coding each into the core would
make the core unmaintainable and would prevent a contributor from owning a
single language end to end. A founding objective is that a new contributor can
deliver a whole language without touching the application core (NFR-MAINT-01/02),
and everything must work fully offline (NFR-OFFLINE-01).

## Decision

A **Language Pack** is a versioned, offline bundle with a manifest declaring its
language, version, and provided capabilities, plus the data and code
implementing them. The core defines stable capability interfaces — segmenter,
lemmatizer, dictionary, transliterator, TTS, grammar parser, script profile — and
routes calls to the active pack. A pack implements the subset relevant to its
language and declares the rest absent; the core contains no language-specific
logic and behaves correctly when a capability is absent, disabling only the
dependent features. A conformance suite validates a pack against the interfaces.

## Alternatives considered

### Language logic inside the core behind conditionals
Fastest to start for the first language — no interface to design. It lost
because it becomes unmaintainable across many languages, entangles unrelated
languages in one code path, and makes it impossible for a contributor to own a
language without editing the core. Rejected against NFR-MAINT-01.

### Fully dynamic third-party plugin system at MVP
Maximum extensibility — arbitrary third-party code loaded at runtime. It lost as
premature: the security, versioning, and sandboxing surface is large and
unjustified before the interface has even been shaken out by two packs. The
bundle-and-manifest design deliberately leaves room to open this up later
without a redesign.

## Consequences

- **Positive**: languages are independently versioned, testable, and ownable;
  a contributor delivers a whole language without core changes; every capability
  has an offline implementation path, preserving the local-first guarantee.
- **Negative**: the core must define and version these interfaces **before** the
  first pack, and interface stability across typologically diverse languages is
  a standing risk (R-08); a conformance suite is required (FR-EXT-04). **Revisit
  the interface version** (a superseding ADR + MAJOR bump) if adding a language
  forces an incompatible interface change. The interface is shaken out with
  Japanese (typologically hard) and validated for generalization with Latin
  (typologically opposite).
- **Follow-ups**: define and version the capability interfaces; build the
  conformance suite; document the pack format for external authors (FR-EXT-01).
