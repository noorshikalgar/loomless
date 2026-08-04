# Story

> Turn a working session into a readable story — not a video.

Story watches a session (clicking through a feature, debugging an issue, a
demo) and turns it into a structured, skimmable "story": ordered steps, each
with a screenshot and a caption, exported as Markdown so it can be dropped
straight into a PR, a ticket, or a repo's `/docs` folder. It's built to be
driven by a human recording themselves **or by an AI coding agent** (e.g.
Claude Code) narrating its own work through an MCP interface.

## Status

**Phase 1.5 (agent path) done. Human path on macOS: click-capture
(with a configurable highlight ring and optional auto-generated
captions), terminal capture, secret-masking, idle-timeout nudge, a
local editor (reorder/hide/caption/redact/highlight), and
Markdown/PDF/slides export all work. Linux: window/terminal capture
works (X11 only), automatic click-capture doesn't yet.** The agent path
(MCP server) works end to end on macOS. See
[`docs/product/00-README.md`](docs/product/00-README.md) for the full
product write-up, [`docs/product/10-open-questions.md`](docs/product/10-open-questions.md)
for what's still undecided, and [`docs/product/11-decisions.md`](docs/product/11-decisions.md)
for what's already resolved and why (capture engine: **Go**; accounts:
**optional/local-first**; editing model: **mutable with activity log**;
screenshots: **window-scoped, not full-screen**; terminal capture: **explicit
`story run` wrapper, not a full shell hook**; secrets: **pattern-masked at
capture time**; Linux: **X11 via standard tools, no click-capture yet**;
forgotten recordings: **desktop notification after 20min idle, not
auto-stop**; PDF/slides: **rendered directly from session data, pure Go,
no headless-browser/CDN dependency**; local editor: **web UI, not
TUI**; highlights: **numbered, editable annotations shared by the
automatic click-ring and manual highlights, not baked into pixels —
see decisions #14/#16/#18**; auto-captions: **local template by
default, macOS Accessibility API grounding, optional LLM rewrite only
if `ANTHROPIC_API_KEY` is set — see decision #33**). Not done: Windows,
Wayland, sharing — all later phases.

## Why

Explaining "what I did" today means choosing between a video (fast to
record, painful to consume, not diffable) or a manual write-up (skimmable,
but tedious enough that most people skip it). Story aims to be fast to
produce *and* fast to read — and to be usable by an AI agent narrating its
own reasoning, not just a human clicking through after the fact.

See [`docs/product/01-vision-and-goals.md`](docs/product/01-vision-and-goals.md)
for the full problem statement.

## Documentation

All product thinking lives in [`docs/product/`](docs/product/), read in this
order:

1. [Vision & goals](docs/product/01-vision-and-goals.md)
2. [Users & personas](docs/product/02-users-and-personas.md)
3. [Features (MVP / Phase 2 / Phase 3+)](docs/product/03-features.md)
4. [Usage scenarios](docs/product/04-usage-scenarios.md)
5. [Collaboration & sharing](docs/product/05-collaboration-and-sharing.md)
6. [Editing & versioning](docs/product/06-editing-and-versioning.md)
7. [Edge cases](docs/product/07-edge-cases.md)
8. [Technical considerations](docs/product/08-technical-considerations.md)
9. [Roadmap](docs/product/09-roadmap.md)
10. [**Open questions**](docs/product/10-open-questions.md) — the decisions
    that gate everything else

## Try it

The core loop: on macOS, `record start` watches your clicks automatically
(a listen-only event tap — it observes, never intercepts) and captures a
window-scoped, click-highlighted screenshot per click (not full-screen —
see [`docs/product/11-decisions.md`](docs/product/11-decisions.md) #5). Add
notes without a click via `story step`, or capture a command + its output
via `story run` — both work the same way on Linux (X11), just without the
automatic click part (decision #10) — use `story step`/`story run` there
instead.

```bash
go build -o story ./cmd/story
./story record start my-first-story           # add --highlight for a click ring, --auto-caption for generated captions
# macOS: click around normally — each click is captured automatically
./story step "optional note, no click needed"
./story run -- go test ./...     # captures the command + its output as a step
./story record stop
./story edit my-first-story                   # reorder/hide/caption, redact (undo/redo), or add numbered highlight callouts
./story export my-first-story                 # Markdown (default)
./story export my-first-story --format pdf    # or a PDF, screenshots embedded
./story export my-first-story --format slides # or a self-contained HTML slide deck
./story tui                                   # or: browse all sessions, edit/export from a terminal dashboard
```

**Linux (X11) dependencies:** `xdotool`, `wmctrl`, `xprop` (usually
preinstalled), and [`maim`](https://github.com/naelstrof/maim) for
screenshots — e.g. `apt install xdotool wmctrl x11-utils maim`. Wayland
isn't supported (these are all X11-protocol tools).

**Forgot a recording was running?** After ~20 minutes of no activity
(`story step`/`story run`/clicks), you'll get a desktop notification
nudging you to stop it — it never auto-stops for you (see decision #11 in
[`docs/product/11-decisions.md`](docs/product/11-decisions.md)).

Notes, window titles, and `story run` commands/output are pattern-matched
for common secret shapes (API keys, tokens, `password=...`-style
assignments) and masked as `«masked»` before anything is written to disk —
see decision #7 in
[`docs/product/11-decisions.md`](docs/product/11-decisions.md). This is a
safety net, not a guarantee: it won't catch every shape, and it doesn't
hide anything from your *live* terminal — only from what gets saved.
Review before sharing a story publicly.

Output lands in `~/.story/sessions/my-first-story/`. First run prompts for
**Accessibility** (to watch clicks and resolve window info) and **Screen
Recording** (to take the screenshots) — System Settings → Privacy &
Security, granted to whichever terminal app you're running it from. Both
are one-time macOS system prompts, expected per
[`docs/product/07-edge-cases.md`](docs/product/07-edge-cases.md).

## Agent-driven capture (MCP)

`story mcp` runs an MCP server (stdio) exposing `start_recording`,
`capture_step`, `run_command`, `stop_recording`, `generate_story`, and
`list_windows` — so an agent like Claude Code can narrate its own work
instead of a human clicking through it (Scenario B in
[`docs/product/04-usage-scenarios.md`](docs/product/04-usage-scenarios.md)).
Same session data, same masking, same CLI underneath — see decision #8 in
[`docs/product/11-decisions.md`](docs/product/11-decisions.md).

To use it from Claude Code:

```bash
claude mcp add story -- /path/to/story mcp
```

## Contributing

Not yet open for external contributions — the product direction is still
being decided (see open questions above). Once Phase 0 starts, see
[`CONTRIBUTING.md`](CONTRIBUTING.md) for the branching model and PR process.

## License

[MIT](LICENSE)
