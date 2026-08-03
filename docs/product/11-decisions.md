# Decisions Log

Resolutions to the "must decide before any build starts" questions in
[`10-open-questions.md`](10-open-questions.md). Each entry: decision, date,
rationale, and what it unblocks/still leaves open.

## 1. Platform order

**Status: not revisited yet.** Standing assumption carried over from prior
discussion: desktop first, browser extension later. Still open whether the
MCP/agent angle changes this — revisit at Phase 0 kickoff since an
agent-driven session leans more on terminal + browser automation than
native-app capture.

## 2. Capture engine language — **Go**

Decided 2026-08-03. Single static binary fits the CLI-first distribution
model, iteration speed matters more than Rust's performance ceiling at this
stage, and the MCP SDK story is straightforward. Revisit only if a specific
`robotgo`-class library proves unmaintained during Phase 0 spike.

**Unblocks:** repo structure (`cmd/`, `internal/` Go layout), CI tooling
choice, packaging plan (Homebrew/GitHub Releases per
[`08-technical-considerations.md`](08-technical-considerations.md)).

## 3. Accounts/login — **Optional, local-first**

Decided 2026-08-03. The tool works fully with zero login for Phases 0-2
(record, export, paste into a PR/ticket). Login is only required to unlock
hosted share links / team workspace in Phase 3. Matches the
Scribe/Tango pattern per
[`05-collaboration-and-sharing.md`](05-collaboration-and-sharing.md).

**Unblocks:** confirms Phases 0-2 need no backend/auth work at all — local
CLI + local storage is sufficient until the Phase 3 go/no-go checkpoint in
[`09-roadmap.md`](09-roadmap.md).

## 4. Editing/versioning model — **Mutable with activity log**

Decided 2026-08-03. Editing is allowed (reorder, redact, edit captions) but
every change to a story is logged, and the rendered output can show an
"edited" indicator. Original raw capture is preserved separately so a
revert is always possible. Per
[`06-editing-and-versioning.md`](06-editing-and-versioning.md) Option 2.

**Unblocks:** data model needs to separate **capture-time data**
(screenshot, click coordinates, raw terminal output — immutable once
written) from **presentation-time data** (caption, order, redaction,
visibility) from day one, even before sharing/accounts exist in Phase 3 —
so the log has something to log retroactively instead of being bolted on
later.

## 5. Screenshot capture scope — **window-specific, not full-screen**

Decided 2026-08-03. Every capture is scoped to a single window (the
frontmost/active window, or an explicitly chosen one) rather than the whole
display. Narrows the surface for accidental sensitive-data capture (other
apps, other tabs, menu bar/notifications never appear), and makes the
resulting screenshots more relevant per step. Element/region-level capture
(cropping to a specific control, not just the window) is a stretch goal for
the same spike — attempt it, but window-level is the acceptance bar for
Phase 0, not element-level.

Implementation note (macOS first, per platform-order default): use the
built-in `screencapture -l <windowID>` rather than a full-display grab, with
window ID resolved via `osascript`/System Events for the frontmost window,
or from a `story windows list` picker for a specific one. No CGO/Accessibility
global-hook dependency needed for this — keeps the Phase 0 spike buildable
without extra OS permission prompts beyond Screen Recording.

**Unblocks:** Phase 0 capture engine scope — see
[`../../CONTRIBUTING.md`](../../CONTRIBUTING.md) for where this lands
(`feature/phase-0-capture-engine`).

## 6. Terminal capture scope — **explicit wrapper, not full shell session**

Decided 2026-08-03 (resolves open question #8). `story run -- <command>`
runs one command as a subprocess and captures its output as a step —
it does not attach to or tee the user's actual interactive shell.

Consistent with every capture-surface decision so far (window-scoped not
full-screen, listen-only click tap, optional accounts): default to the
narrower, explicit surface. A full-session shell wrapper would capture
every command the user runs for the entire recording — including ones
that have nothing to do with the story and are more likely to contain
secrets never meant for the output. An explicit per-command wrapper only
captures what's deliberately routed through it, at the cost of the user
needing to type `story run --` in front of commands they want captured.

Long output is truncated to a head/tail preview in the rendered story
(per `07-edge-cases.md`); full raw output is always saved alongside the
session so nothing is silently lost, just not inlined.

Secret-masking (decision #7, below) now runs on this path before anything
is written to disk.

## 7. Secret-masking — **pattern-based, text fields only, unconditional**

Decided 2026-08-03 (resolves open question #5). `internal/mask` pattern-
matches common secret shapes (AWS keys, `sk-`/`gh_`/`xox`-prefixed API
tokens, JWTs, PEM private-key blocks, and `label=value`/`--flag value`
style assignments) and replaces matches with `«masked»` before any text
touches disk. Applied unconditionally inside `session.AddStep` /
`AddTerminalStep` — not something a caller opts into — so every current
and future capture path (manual step, click daemon, `story run`, and the
Phase 1.5 MCP server later) gets it automatically. Covers: step notes,
window titles, terminal commands, and terminal output.

**Scope, deliberately narrow:** text only. Screenshot pixels are not
scanned — per `07-edge-cases.md`, reliable secret detection in arbitrary
pixel content needs OCR, which is real Phase 2 "auto-blur" scope, not a
Phase 1 pattern-match. And masking only governs what Story *saves*: a
command run via `story run` still prints its real, unmasked output live to
your actual terminal exactly as it would without Story — masking can't
and shouldn't suppress that, only what lands in `session.json` and the
output log.

Default aggressiveness leans toward **fewer false positives over catching
everything** (the doc's stated trade-off, left undecided until now): an
early version of the `label=value` rule also matched on bare whitespace
(`password reset`, `a secret plan`) and flagged ordinary prose caught by
the test suite — tightened to require an explicit `:`/`=` separator, plus
a separate `--flag value` rule for CLI-style args, which are unambiguous
without a separator. This will still miss real secrets in unrecognized
shapes; per the edge-cases doc, that's expected — this is a safety net,
not a guarantee, and the human/agent reviewing before sharing is still
the real backstop.

Each step records *which* rules fired (`Masked: []string`, rule names
only, never matched content) — visible in the CLI output and in
`session.json` — so masking is never silent.

## 8. MCP tool surface — the doc's 4, plus 2

Decided 2026-08-03 (Phase 1.5, per `09-roadmap.md`). `story mcp` runs an
MCP server over stdio exposing `start_recording`, `capture_step`,
`stop_recording`, `generate_story` — the exact 4 named in
`03-features.md`/`04-usage-scenarios.md` Scenario B — plus two more:

- `run_command`: the agent-facing equivalent of `story run`. Scenario B's
  own example ("ran the failing test to see the exact error", "re-ran the
  test suite, all green") is literally an agent running commands and
  wanting them captured — without this tool the agent would have to use
  its own separate shell tool and then manually describe the command via
  `capture_step`'s note, losing the actual output. Included because the
  scenario the doc describes needs it, not because it was asked for by
  name.
- `list_windows`: lets an agent sanity-check what a capture would resolve
  as the frontmost window before calling it — cheap, read-only, avoids
  capturing the wrong window blind.

Implementation: `cmd/story/mcp.go` + a new `internal/engine` package.
Business logic (session lifecycle, click-daemon start/stop, step capture,
command execution, export) moved out of `cmd/story/main.go` into
`internal/engine`, shared unchanged between the CLI and the MCP server —
per `08-technical-considerations.md`'s "thin wrapper around the same
engine's internal API." One hard invariant enforced there: nothing in
`internal/engine` writes to `os.Stdout` — the MCP stdio transport uses
stdout as the protocol stream, so a stray print would corrupt it. The
CLI's live terminal echo for `story run` is a tee the CLI adds on top
(`engine.RunCommand` takes optional tee writers, nil from the MCP path).

Verified against the real SDK client (`github.com/modelcontextprotocol/
go-sdk`, not just manual JSON-RPC): connected, listed all 6 tools, drove a
full start_recording → capture_step → run_command → list_windows →
stop_recording → generate_story sequence, and confirmed a fake secret
in `run_command`'s output came back masked in both the tool result and
the final generated Markdown.

## GitHub remote

Decided 2026-08-03: staying local-only for now. `git init` is done, commits
are landing on `main` per [`../../CONTRIBUTING.md`](../../CONTRIBUTING.md).
Create the actual `github.com` remote (public, once there's real code)
when ready — `gh repo create` from an authenticated session.
