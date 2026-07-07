# Project Kickoff: One Prompt from Template to Dev Mode

How to turn a fresh duplicate of this template into the start of a real
project with a single prompt. The work is done by the
[project-kickoff skill](../../.claude/skills/project-kickoff/SKILL.md) (or a
human following it — as always, the committed files are the interface). It
runs **once**, right after duplicating the template, and ends with the repo in
**full dev mode**: no placeholders, a product brief, a prioritized backlog,
the first feature already specced, everything building and testing green, and
an initial commit. From then on, all work goes through the normal
[planning flow](../process/PLANNING_FLOW.md).

## What you must provide (and why)

Four inputs are **required** — the skill will stop and ask rather than guess:

1. **Project name + one-sentence purpose.** Becomes the repo's identity
   (README, product brief). *Example: "Ledgerline — a CLI that tracks
   personal spending from bank CSV exports, entirely offline."*
2. **Problem statement** — a short paragraph: who has the problem, what it
   is, why existing options don't cut it. Every future spec traces back to
   this, so vagueness here becomes vagueness everywhere.
3. **Requirements list** — a bullet list of user-visible capabilities, each
   roughly one feature, **in priority order**. These become the backlog
   verbatim (numbered `REQ-1…`), and the top item becomes the first spec.
   Write outcomes, not implementations: "import transactions from a CSV
   file" (good) vs "use a CSV parsing library" (an implementation — the
   skill will push back and ask what the user actually needs).
4. **GitHub `owner/repo`** — or an explicit "none yet". Drives the Go module
   path, changelog links, the `app update` release check, and the release
   process. "None yet" is fine; the update check stays dormant with a TODO.
   Note: the repo must be **public** for `app update` to work — the check is
   unauthenticated, and private repos' releases 404 (ADR-0003).

Optional inputs, with the defaults applied if you say nothing (each applied
default is recorded as a written assumption you can amend later):

| Input | Default if omitted |
|---|---|
| Binary/CLI name | kebab-cased project name |
| Go module path | `github.com/<owner>/<repo>` |
| Target platforms | Windows/macOS/Linux, amd64 + arm64 |
| License / copyright holder | MIT / the repo owner |
| Settings dir + env-var prefix | binary name / its SCREAMING_SNAKE form |
| Product form (CLI, GUI, self-hosted service) | CLI |
| Known non-goals | none recorded |

**Fair warning about the gate:** before renaming anything, the skill checks
your requirements against the standing ADRs. If your product needs a rich
native GUI (ADR-0001 flags Go as weak there), structured queryable data
(beyond ADR-0002's preferences file), or network access beyond the opt-in
update check (ADR-0003's invariant), it will surface the conflict and make
you decide — possibly "this is the wrong template" — *before* an hour of
renaming, not after.

## The prompt (copy, fill, paste)

```text
Run project-kickoff to turn this fresh copy of the template into a real project.

Project name: <name>
Purpose: <one sentence>
Problem statement: <who has what problem, why now, why existing options fail>

Requirements (priority order):
- <REQ-1: user-visible capability>
- <REQ-2: ...>
- <REQ-3: ...>

GitHub repo: <owner/repo, or "none yet">

Optional (delete lines you're happy to default):
Binary name: <name>
Module path: <path>
Platforms: <list>
License: <license, holder>
Product form: <CLI | desktop GUI | self-hosted service>
Non-goals: <anything you already know is out of scope>

Apply defaults for anything I left out and record them as assumptions. Flag
any conflicts with the standing ADRs before making changes. When you're done,
tell me the assumptions you recorded and the single next action.
```

## What happens, in order

1. **Input check + ADR gate** — missing required inputs are asked for once,
   in one batch; requirement/ADR conflicts are surfaced for a decision.
2. **Mechanical instantiation** — module rename, `OWNER/REPO` replacement,
   update-check target, binary/settings/env-var identity, license, README
   rewritten to the product, changelog reset. Each step has a grep/build
   check; nothing is "probably renamed".
3. **Product brief** — `docs/PRODUCT.md`: problem, users, numbered
   requirements, non-goals, recorded assumptions, what 1.0 means.
4. **Backlog + first intake** — `docs/planning/BACKLOG.md` (the ordered
   feature queue) and a real spec for REQ-1 via the spec-writer skill, so
   the pipeline is proven before anyone relies on it.
5. **Verification** — full test/build/smoke pass with the new identity, no
   placeholder strings anywhere, links resolve.
6. **Initial commit** — one commit; pushing/creating the remote only happens
   if you ask.

## What "full dev mode" means (the exit bar)

You (or any dev, or any Claude instance) can immediately:

- run `go test ./...` green and build a working binary under the new name;
- open `docs/PRODUCT.md` and know what is being built and for whom;
- open `docs/planning/BACKLOG.md` and know what's next and in what order;
- open `docs/planning/<first-feature>/spec.md` and start stage 2
  (critical-path-planner) **right now** — that is always the reported
  "single next action";
- trust that anything not written down is genuinely undecided, because the
  kickoff recorded every default it applied.

The template's settings and update-check features remain as live, tested
features of your app (their planning trails and ADRs stay as history); the
greeting command remains as scaffold and is queued in the backlog for
replacement by your first real feature.

## After kickoff

The kickoff skill retires itself — if `go.mod` no longer says
`example.com/app`, it refuses to run again. Everything else is the ordinary
flow: [ONBOARDING.md](ONBOARDING.md) for new contributors,
[PLANNING_FLOW.md](../process/PLANNING_FLOW.md) for features,
[RELEASE_PROCESS.md](../process/RELEASE_PROCESS.md) when there's something
worth shipping.
