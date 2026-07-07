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
- PDF reader core (REQ-1), work started. The PDF engine is proven end-to-end
  via PDFium ([ADR-0012](docs/architecture/ADR-0012-pdf-engine.md)) on the no-cgo
  WebAssembly backend, cross-compiling for all six desktop targets with
  `CGO_ENABLED=0` (see the
  [spike result](docs/planning/pdf-reader-core/critical-path.md#t2-spike-result-c1-gate)).
  Landed so far:
  - `src/reader` — the core/frontend **command interface** (`reader.Reader`:
    open, page count, render page at a scale, thumbnail, get/set reading
    position, close) with its request/response models, a typed open-error shape,
    and an in-memory stub the frontend can build against (tasks T1, C2).
  - `src/document` — the **Document Engine** implementing that interface: open a
    PDF and report page count and per-page geometry (T3), render a page at an
    explicit zoom or fit-to-width/page (T4), and classify bad input (missing,
    non-PDF, corrupt/truncated, encrypted) into distinct soft errors that keep
    the app running (T13). The PDFium binding is confined to this package.

<!--
Versioned release link references (e.g. [Unreleased]: .../compare/v0.1.0...HEAD)
are added once the first release is cut. No releases exist yet.
-->
