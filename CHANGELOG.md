# Changelog

All notable changes to this project are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Entries are added to **Unreleased** as part of each PR (see
[Definition of Done](docs/process/DEFINITION_OF_DONE.md)). The release manager
rotates Unreleased into a versioned section when cutting a release (see
[Release Process](docs/process/RELEASE_PROCESS.md)).

## [Unreleased]

### Added
- Project initialized from the local-first application template: product brief
  ([docs/PRODUCT.md](docs/PRODUCT.md)), feature backlog
  ([docs/planning/BACKLOG.md](docs/planning/BACKLOG.md)), and the Adamic
  architecture decisions ([ADR-0005](docs/architecture/ADR-0005-platform-stack.md)
  through [ADR-0011](docs/architecture/ADR-0011-language-pack-boundary.md)).
- Inherited, working template features retained as live features of Adamic: a
  persistent local settings file (`adamic config get|set|list|path`) and an
  opt-in update check (`adamic update`) against
  [github.com/shrigum/Adamic](https://github.com/shrigum/Adamic) releases — see
  [ADR-0003](docs/architecture/ADR-0003-update-check.md).
- PDF reader core (REQ-1), work started: the PDF engine is proven end-to-end
  behind the new `src/document` Document Engine package. It renders real PDF
  pages via PDFium ([ADR-0012](docs/architecture/ADR-0012-pdf-engine.md)) using
  the no-cgo WebAssembly backend, cross-compiling for all six desktop targets
  with `CGO_ENABLED=0` — see the
  [spike result](docs/planning/pdf-reader-core/critical-path.md#t2-spike-result-c1-gate).

<!--
Versioned release link references (e.g. [Unreleased]: .../compare/v0.1.0...HEAD)
are added once the first release is cut. No releases exist yet.
-->
