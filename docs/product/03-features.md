# Features

Grouped by phase. "MVP" = needed for the tool to be useful at all. "Phase 2"
= makes it good. "Phase 3+" = makes it a product, not just a tool. None of
this phasing is final — see `09-roadmap.md` and `10-open-questions.md`.

## MVP — capture

- Start / stop a recording session from the CLI (`loomless record start /
  stop`).
- Capture on every mouse click: screenshot + click coordinates + (where
  resolvable) the UI element under the cursor.
- Capture terminal activity during the session: commands run + their
  output, associated with the surrounding steps.
- Manual step insertion: user (or agent) can add a note/step without a
  click (e.g. "this is where I noticed the bug").
- Local-only storage of raw session data (screenshots + step metadata as
  JSON) — no server dependency to record.

## MVP — agent interface

- MCP server exposing: `start_recording`, `capture_step(note)`,
  `stop_recording`, `generate_story(format)`.
- Agent-supplied captions take priority over (or replace the need for)
  auto-generated ones, since the agent already knows *why* it did
  something.

## MVP — output

- Render a session into a Markdown file: ordered steps, each with an
  embedded (or linked) screenshot and a caption.
- Basic click-highlight annotation on screenshots (circle/box at the click
  location) — no manual annotation editor needed yet.

## Phase 2 — editing & polish

- A lightweight editor (local, e.g. a small local web UI or TUI) to:
  reorder steps, delete steps, edit captions, redact/blur parts of a
  screenshot, merge two steps into one.
- Auto-caption generation via an LLM call for steps that weren't
  explicitly narrated (i.e. pure click-recording without an agent) — using
  the clicked element's label/DOM text as grounding context, not raw pixel
  guessing.
- PDF export (render the Markdown/HTML to PDF).
- Slide-deck export (HTML, e.g. reveal.js-style, step-through view).
- Basic redaction: auto-blur likely-sensitive fields (password inputs,
  tokens in terminal output) before anything is saved or exported.

## Phase 3 — sharing & collaboration

*(See `05-collaboration-and-sharing.md` for the full discussion — this is
just the feature list.)*

- Generate a shareable link for a story (public or restricted).
- Team workspace: shared library of stories, basic access control.
- Commenting on individual steps.
- Accounts / login (see open questions — this is a genuinely open decision,
  not assumed).

## Phase 3 — versioning & trust

*(See `06-editing-and-versioning.md`.)*

- Edit history per story: who changed what, when, and (ideally) why a step
  was modified or removed after the original capture.
- A way to tell, at a glance, "this screenshot/step was edited after
  capture" vs "this is exactly what was recorded" — important for repro
  reports and PR trust.

## Later / speculative (not committed)

- Desktop app for native (non-browser) app capture, reusing the same
  engine as the CLI.
- In-app "Guide Me"-style overlays (Tango-style) — explicitly a stretch,
  possibly never.
- Org-wide searchable story library (Scribe-style knowledge base).
- Video export (stitch the steps into a narrated video) as an alternate
  output format, for people who still want video for some audiences.
