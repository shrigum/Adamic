# ADR-0005: Platform and technology stack — Go + web frontend + Wails v3, desktop only

- **Status**: Accepted (supersedes the CLI/no-rich-GUI posture of [ADR-0001](ADR-0001-tech-stack.md); retains Go)
- **Date**: 2026-07-07
- **Feature/trigger**: project kickoff; baselined Architecture and Design Document §3
- **Deciders**: project maintainers (technical lead)

## Context

Adamic must render PDFs at high quality, run several native libraries offline
(PDF engine, OCR, voices), perform well on large documents (500+ pages), and
remain local-first with no network dependency. The target is desktop only —
Windows, macOS, and Linux; tablet is out of scope for the MVP.

A founding constraint of the project is fast onboarding for a small, rotating
team, so the stack must keep the contributor floor low. The template already
fixes Go as the implementation language ([ADR-0001](ADR-0001-tech-stack.md));
its only conflict with Adamic was the assumption that a Go application here
would be a command-line tool with no rich GUI. Adamic needs a rich reader GUI,
so that one assumption — not the language choice — must be revisited.

## Decision

We build Adamic on a **Go core with a web-technology frontend, packaged with
Wails v3**, targeting desktop only (Windows, macOS, Linux). The Go core owns
document handling, text extraction, Language Pack execution, and data storage.
The frontend owns rendering and interaction over a defined command interface
and holds no language logic and no persistence. Native libraries (PDF engine,
OCR, voices) are integrated through cgo; pure-Go libraries are preferred where
they exist (the MVP Japanese tokenizer is pure Go).

## Alternatives considered

### Rust core + Tauri
A stronger native-library story and real mobile support. It lost because it
carries a higher onboarding floor that runs directly against the project's
founding reason to exist (fast onboarding for a rotating team), and it would
discard the template's Go language decision entirely. Once tablet was dropped
from MVP scope, Tauri's main remaining advantage (mature mobile packaging) no
longer applied.

### Go CLI, per the template as written
Preserves everything about the template unchanged and has the lowest possible
floor. It cannot provide the rich reader GUI Adamic requires. Rejected — this
is precisely the conflict this ADR resolves.

## Consequences

- **Positive**: retains Go and the low onboarding floor that motivated the
  whole template; gives the wanted core/web-frontend shape on the language
  already chosen; most feature work happens in the frontend and Language Pack
  layers, where the floor is lowest. Restricting to desktop removes the one
  area where Wails is weak (mobile support is alpha).
- **Negative**: cgo native libraries remove the template's single-static-binary
  property and require a C toolchain plus container-based cross-compilation for
  all three desktop targets. The core/frontend command boundary must be defined
  early and kept stable. Tablet is not delivered; FR-APP-04 and NFR-XPLAT are
  scoped to desktop. **Revisit if** Wails v3 desktop packaging proves
  inadequate during the Stage 0 stack spike (invoke a fallback stack), or if
  tablet re-enters scope.
- **Follow-ups**: Stage 0 stack-and-packaging spike validating the cgo build
  for all three desktop targets (risk R-03); define and version the
  core/frontend command interface before dependent work.
