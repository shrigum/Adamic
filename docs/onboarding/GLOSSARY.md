# Glossary

One name per concept, everywhere — specs, code, docs, and conversation
([CODING_STANDARDS.md](../CODING_STANDARDS.md#naming)). Extend this file in the
same PR that introduces a new domain concept; the docs-maintainer skill checks.

| Term | Meaning here |
|---|---|
| **ADR** | Architecture Decision Record — an immutable, numbered record of a significant decision in [docs/architecture/](../architecture/). Superseded, never edited. |
| **Acceptance criterion** | A testable statement in a spec that must hold for the feature to be done. Each one names its covering automated test before the feature closes. |
| **Artifact (release)** | A binary or checksum file attached to a GitHub Release. The only deployable this project produces. |
| **Artifact (stage)** | The committed file a planning-flow stage must produce (spec.md, critical-path.md, …). |
| **Backlog** | `docs/planning/BACKLOG.md` (created at kickoff) — the ordered queue of features (`REQ-n` → feature name → status). In-flight work lives in feature folders; the queue lives here. |
| **Command interface** | The defined boundary across which the web frontend asks the Go core for data (open document, render page, look up word, set familiarity, …). The frontend holds no PDF, persistence, or language logic ([ADR-0005](../architecture/ADR-0005-platform-stack.md)). In code: `reader.Reader` (`src/reader`), exposed to the frontend as JSON-friendly methods by `src/app`. |
| **Document Engine** | The Go component that opens PDFs and rasterizes pages, wrapping the PDFium binding so nothing else imports it (`src/document`, [ADR-0012](../architecture/ADR-0012-pdf-engine.md)). It satisfies the command interface. |
| **Familiarity state** | The reader's declared knowledge of a lemma — one of unknown, learning, known, ignored — keyed by (language, lemma), not surface form ([ADR-0009](../architecture/ADR-0009-familiarity-by-lemma.md)). |
| **Language Pack** | A versioned, offline bundle providing one language's capabilities (segmenter, lemmatizer, dictionary, transliterator, TTS, grammar parser, script profile) behind stable interfaces ([ADR-0011](../architecture/ADR-0011-language-pack-boundary.md)). The core holds no language logic. |
| **Lemma / surface form** | A **surface form** is a word as printed; a **lemma** is its dictionary headword. Lookup resolves surface form → lemma before dictionary lookup; familiarity keys on the lemma. |
| **Reader** | The component that opens and displays a PDF in faithful fixed layout, with navigation, zoom, and thumbnails (feature `pdf-reader-core`, REQ-1). |
| **Reading position** | The persisted per-document viewport location (page index plus within-page offset/zoom) that restores where a reader left off ([ADR-0007](../architecture/ADR-0007-reader-layout.md) layout model; stored per [ADR-0008](../architecture/ADR-0008-local-data-storage.md)). Interim home: `library.FileStore`. |
| **Render window** | The Document Engine's virtualization: it renders only the visible pages plus a small look-ahead and keeps rendered pages in an LRU bounded by a page budget, so a long document stays responsive and memory stays bounded (`document.RenderWindow`). |
| **Blocking finding** | A code-review finding that must be resolved before merge: spec violation, Definition-of-Done failure, correctness bug, or standards breach on the critical path. |
| **Critical path** | The dependency chain of tasks with the largest total estimate in a feature's task DAG. Determines rigor and build order. See [CRITICAL_PATH_METHOD.md](../CRITICAL_PATH_METHOD.md). |
| **CP task / `[CP]`** | A task on the critical path. Gets error-path tests, strict review, no TODOs. |
| **Definition of Done (DoD)** | The objective checklist in [docs/process/DEFINITION_OF_DONE.md](../process/DEFINITION_OF_DONE.md). |
| **Feature folder** | `docs/planning/<feature-name>/` — the complete committed state of one feature's planning and review. |
| **Full dev mode** | The state a repo is in after kickoff: no placeholders, product brief + backlog written, first feature specced, tests green, initial commit made — the next action is always a planning-flow stage. |
| **Kickoff** | The one-time instantiation of the template into a real project via the [project-kickoff skill](../../.claude/skills/project-kickoff/SKILL.md); inputs and prompt in [KICKOFF.md](KICKOFF.md). |
| **Local-first** | The app runs fully on the user's machine; state lives in user-owned, user-readable files; any network capability is optional and degrades gracefully offline. |
| **Off-path task** | A task not on the critical path; parallelizable, lighter review, suitable for new contributors. |
| **Product brief** | `docs/PRODUCT.md` (created at kickoff) — problem, users, numbered requirements (`REQ-n`), founder non-goals, recorded assumptions. The document every spec traces back to. |
| **Release** | A SemVer git tag plus a GitHub Release with binaries attached. Not a deployment — there is nothing to deploy to. |
| **Settings** | The user's persistent preferences, stored per [ADR-0002](../architecture/ADR-0002-settings-file-format.md). Always "settings" — never "config", "prefs", or "options" — except in the CLI command name `app config`, which is user-facing convention. |
| **Skill** | A Claude Skill in [.claude/skills/](../../.claude/skills/): a written methodology for one planning-flow stage, executable by a Claude instance or followable by a human. |
| **Spec** | The stage-1 document: problem, non-goals, constraints, assumptions, acceptance criteria. The contract implementation is reviewed against. |
| **Schema version** | The `schemaVersion` field in the settings file identifying its on-disk layout, so future migrations are deterministic ([ADR-0004](../architecture/ADR-0004-settings-schema-version.md)). |
| **Spike** | A time-boxed, throwaway implementation to retire a High risk before real work depends on it. |
| **Update check** | The opt-in `app update` command — the only code in the app that touches the network ([ADR-0003](../architecture/ADR-0003-update-check.md)). Reports newer releases; never downloads or auto-runs. |
| **Worked example** | The settings-file feature — a real feature kept in the repo with its complete planning trail as living documentation of the flow. |
