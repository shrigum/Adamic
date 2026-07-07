# ADR-0001: Go as the implementation language

- **Status**: Accepted. The language decision (Go) stands; the "weak at native GUI, therefore command-line" framing is amended by [ADR-0005](ADR-0005-platform-stack.md), which delivers a GUI through a Wails web frontend on Go.
- **Date**: 2026-07-07
- **Feature/trigger**: template bootstrap — the template must ship a concrete,
  worked example rather than an abstract "insert language here" scaffold.
- **Deciders**: project maintainers

## Context

The template targets local-first applications whose end product is an
**executable that runs on Windows, macOS, and Linux**, distributed as binaries
attached to GitHub Releases. That imposes hard requirements on the default stack:

1. **Cross-compilation from one machine** — the release process is local-first;
   a maintainer must be able to produce all platform artifacts without a build
   farm or paid CI.
2. **No runtime for end users to install** — "download one file and run it" is
   the local-first distribution story. Asking users to install Python 3.12 or a
   JVM first fails it.
3. **Free, open, account-free toolchain**, per the project's OSS-first principle.
4. **Common enough that contributors (and LLM agents) are productive immediately** —
   the author's constraint is explicitly "align with the most common languages".

There is tension in requirement 4: the *most* common languages overall
(JavaScript/TypeScript, Python) are the weakest at requirements 1–2, while the
languages strongest at 1–2 (Go, Rust) are somewhat less common. The decision is
about where to take the hit.

## Decision

We use **Go (1.24+)** as the template's implementation language, with a
stdlib-first dependency policy. Cross-compilation is done with
`GOOS`/`GOARCH` environment variables in [scripts/build.sh](../../scripts/build.sh);
the version is injected at link time. All process documentation is written to be
language-agnostic so the stack can be swapped by replacing this ADR, `src/`,
`tests/`, and `scripts/`.

## Alternatives considered

### Rust
Advantages: strongest correctness guarantees, no GC, excellent single-binary
story, first-class cross-compilation via `cross`/`cargo`. Lost because: slower
compile-edit cycle, a steeper learning curve for the median contributor, and
cross-compiling to macOS/Windows from Linux requires more setup than Go's
zero-config `GOOS=` switch. For a template optimized for "productive within the
hour", Go's simplicity wins; a Rust retarget of this template is entirely
reasonable for projects that need Rust's guarantees.

### TypeScript/Node (pkg / Bun compile / Deno compile)
Advantages: the most common language; enormous ecosystem; same language if the
app later grows a web UI. Lost because: single-executable packaging bundles a
~60–90 MB runtime per platform, the bundlers are the least mature link in the
chain, and the ecosystem's dependency sprawl fights the template's
minimal-dependency principle.

### Python (PyInstaller / Nuitka)
Advantages: most common language for tooling; batteries included. Lost because:
executable packaging is per-platform (must build *on* each OS — directly violates
requirement 1), artifacts are large and fragile (antivirus false positives,
slow cold start), and dependency/venv management adds onboarding friction.

### C# (.NET self-contained AOT)
Advantages: genuine cross-compilation, good tooling, common in enterprise. Lost
because: larger artifacts, toolchain is heavier to install, and it is
meaningfully less common in the open-source CLI/tools ecosystem this template
targets, which matters for drive-by contributors.

### Leaving the stack abstract ("bring your own language")
Advantages: maximal reusability of the template. Lost because: the template's
own requirements state that concrete examples are what let a contributor start
immediately, and an abstract template cannot include a runnable worked example —
which is the template's main teaching device.

## Consequences

- **Positive**: one-command cross-compilation for all five release targets from
  any dev machine; ~5 MB static binaries with instant startup; `gofmt`/`go vet`/
  `go test` give formatter, linter, and test runner with zero configuration or
  third-party installs; stdlib covers JSON, file IO, and OS config-dir lookup, so
  the example app has **zero dependencies**.
- **Negative**: contributors who know only Python/JS have a (shallow) learning
  curve; no REPL-style exploratory workflow; GUI applications are not Go's
  strength — a project needing a rich native GUI should revisit this ADR
  (likely toward Rust+Tauri or C#), and that revisit condition is explicitly
  anticipated here.
- **Follow-ups**: keep the Go addendum in
  [CODING_STANDARDS.md](../CODING_STANDARDS.md#go-current-stack--see-adr-0001)
  current with the pinned Go version in CI and `go.mod`.
