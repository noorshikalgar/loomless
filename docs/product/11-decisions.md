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

## GitHub remote

Decided 2026-08-03: staying local-only for now. `git init` is done, commits
are landing on `main` per [`../../CONTRIBUTING.md`](../../CONTRIBUTING.md).
Create the actual `github.com` remote (public, once there's real code)
when ready — `gh repo create` from an authenticated session.
