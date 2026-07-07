# Adamic

**A local-first, open-source PDF reader and editor with an integrated
language-learning layer** — read full-length books in a foreign language, look
up and track vocabulary, annotate, and hear pronunciation, entirely offline.
All user data stays on your device in open, portable formats. Shipping means a
**versioned GitHub Release with binaries attached**, not a hosted service.

Adamic is defined by two capabilities held at once: a competent general-purpose
PDF application (an Adobe Acrobat alternative for common tasks), and a
best-in-class tool for reading a full book in a language you are still learning.

## What Adamic does

- **Reads PDFs** in a faithful fixed layout with navigation, zoom, search,
  bookmarks, themes, and per-document reading position.
- **Extracts and selects text** correctly across left-to-right, right-to-left,
  vertical, and unspaced scripts, mapped to on-page coordinates.
- **Looks up words** by tap/click/selection, resolving each surface form to its
  lemma and showing the definition and reading via the active **Language Pack**.
- **Tracks familiarity per lemma** with in-document coloring and single-tap
  state changes, plus a personal vocabulary bank and known-word coverage.
- **Mines sentences and exports study cards** to Anki (`.apkg`) and CSV.
- **Adds reading aids** — furigana/pinyin/romanization overlays, offline TTS,
  grammar/morphology, and diacritization — where the active pack provides them.
- **OCRs** image-only documents into a selectable text layer, offline.
- **Annotates, organizes, fills forms, and exports/converts** documents, with
  annotations written as standard PDF objects so they open in other readers.
- **Extends to new languages** as self-contained Language Packs — no core
  changes — validated by a conformance suite.

The MVP launches with a **Japanese** Language Pack (the highest-risk, most
differentiating language path); **Latin** is the second pack, validating that
the pack model generalizes to a spaced, heavily inflected language.

## Tech stack

**Go core + web-technology frontend, packaged with [Wails](https://wails.io/)
v3**, desktop only (Windows, macOS, Linux). The Go core owns document handling,
text extraction, Language Pack execution, and the local SQLite data store; the
frontend owns rendering and interaction over a defined command interface.
Native libraries (PDF engine, OCR, voices) are integrated via cgo; pure-Go
libraries are preferred where they exist (the MVP Japanese tokenizer is pure Go).

See the architecture decisions for the full rationale:
[ADR-0005](docs/architecture/ADR-0005-platform-stack.md) (platform/stack, which
amends the template's Go-CLI posture in
[ADR-0001](docs/architecture/ADR-0001-tech-stack.md)) and
[ADR-0008](docs/architecture/ADR-0008-local-data-storage.md) (SQLite, which
supersedes the template's settings-file model in
[ADR-0002](docs/architecture/ADR-0002-settings-file-format.md) and
[ADR-0004](docs/architecture/ADR-0004-settings-schema-version.md)).

> **Status:** early instantiation. The reader, Language Pack runtime, and data
> store are on the backlog and not yet built; the repository currently carries
> the template's settings-file and update-check scaffold plus a greeting command
> that the first real feature (REQ-1) replaces. See
> [docs/planning/BACKLOG.md](docs/planning/BACKLOG.md).

## Quick start

```bash
# Prerequisite: Go 1.24+ (https://go.dev/dl/ — free, no account)
go version

# Run the current scaffold
go run ./src --name "World"

# The inherited settings feature (a live feature of Adamic)
go run ./src config set greeting "Hey"
go run ./src config get greeting
go run ./src --name "World"        # now greets with "Hey, World!"

# Opt-in update check against github.com/shrigum/Adamic releases; the app
# never touches the network otherwise (ADR-0003)
go run ./src update

# Run all tests
go test ./...

# Build a local binary for your platform
./scripts/build.sh                  # or scripts\build.ps1 on Windows -> dist/adamic
```

### Run the desktop reader

The PDF reader (REQ-1) is a Wails v3 desktop app. It needs the
[WebView2 runtime](https://developer.microsoft.com/microsoft-edge/webview2/)
(preinstalled on current Windows) — no other dependency; the PDF engine runs on
WebAssembly, so there is no cgo and no C toolchain.

```bash
./scripts/build-desktop.sh          # -> dist/Adamic.exe (windowed, single file)
```

Then run `dist/Adamic.exe`, click **Open PDF…**, and read: page navigation,
zoom, fit-to-width/page, a thumbnail rail, and per-document reading position.
Regenerating the frontend bindings (only when a bound Go method signature
changes) needs the `wails3` CLI:
`go install github.com/wailsapp/wails/v3/cmd/wails3@latest`.

## Building a release

Releases follow [SemVer](https://semver.org/) and the exact steps in
[docs/process/RELEASE_PROCESS.md](docs/process/RELEASE_PROCESS.md). In brief:

```bash
./scripts/release.sh 1.2.0
```

That script verifies a clean tree and passing tests, checks the changelog has
been rotated, cross-compiles binaries for `windows/amd64`, `darwin/amd64`,
`darwin/arm64`, `linux/amd64`, and `linux/arm64` into `dist/`, and tags the
commit. Pushing the tag triggers
[release.yml](.github/workflows/release.yml), which builds the same artifacts
and attaches them to a GitHub Release. (You can also attach `dist/` manually
with `gh release create` — CI is a convenience, not a dependency.)

> The cgo native-library dependencies (PDF engine, OCR, voices) mean release
> builds will require a C toolchain and container-based cross-compilation once
> those components land; this is tracked as risk **R-03**. The scaffold above
> still builds as a pure-Go static binary today.

## Repository map

| Path | What lives there |
|---|---|
| [docs/PRODUCT.md](docs/PRODUCT.md) | **Read first.** Problem, users, numbered requirements, non-goals, what 1.0 means. |
| [docs/planning/BACKLOG.md](docs/planning/BACKLOG.md) | The ordered feature queue — what to work on next. |
| [docs/onboarding/](docs/onboarding/) | Zero-to-shipped-feature guide, glossary. |
| [docs/process/](docs/process/) | Planning flow, Definition of Done, release process. |
| [docs/planning/](docs/planning/) | One folder per feature: spec, critical path, design review. |
| [docs/architecture/](docs/architecture/) | ADRs — every significant decision, with index. |
| [docs/CODING_STANDARDS.md](docs/CODING_STANDARDS.md) | How code is written here. |
| [docs/CRITICAL_PATH_METHOD.md](docs/CRITICAL_PATH_METHOD.md) | How task graphs and critical paths are computed and tracked. |
| [.claude/skills/](.claude/skills/) | Claude Skills for each planning-flow stage. |
| [src/](src/) | Application code. |
| [tests/](tests/) | Cross-package integration tests. Unit tests live next to the code. |
| [scripts/](scripts/) | Local build and release scripts. There are no deploy scripts, by design. |
| [CHANGELOG.md](CHANGELOG.md) | Keep a Changelog format. |

## The one rule

> **If a decision isn't written down, it doesn't count as decided.**

Every feature leaves a paper trail in `docs/planning/<feature-name>/` and, if it
changed the architecture, an ADR. Someone with no context — human or Claude —
must be able to reconstruct *why* anything is the way it is from the repo alone.

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md). The planning flow there is a **required
checklist**, not a suggestion: PRs that implement features without a committed
spec and critical-path doc are declined regardless of code quality.

## License

MIT — see [LICENSE](LICENSE).
