# Vision & Goals

## Problem

Explaining "what I did" — a new feature, a debugging session, a demo, a repro
of a bug — currently forces a choice between two bad options:

- **Video (Loom-style):** fast to record, but slow to consume, impossible to
  skim, hard to edit (one wrong sentence means re-recording), not diffable,
  not searchable, and awkward to embed in a PR or ticket.
- **Manual write-up:** accurate and skimmable, but tedious — screenshot,
  crop, annotate, write a caption, repeat, for every step. Most people skip
  it or do it badly under time pressure.

Existing "auto step recorder" tools (Scribe, Tango, Guidde, Snagit) solve
this reasonably well for **business SOPs and onboarding docs**, aimed at
non-technical ops/enablement teams, produced by a **human clicking through a
browser**. None of them are built for:

- Developer workflows that span the browser, terminal, IDE, and native apps
  in the same session (a debug session might touch all four).
- Being **driven by an AI agent**, not just a human — an agent narrating its
  own steps as it works, rather than a human clicking through after the
  fact.
- Output that's meant to live in a PR description, a ticket, or a repo
  (Markdown, git-friendly), not just a hosted SaaS page.

## Goal

Build a tool that captures a working session (clicks, screenshots, terminal
commands, notes) and turns it into a structured "story" — a sequence of
steps with screenshots and captions — that can be exported as Markdown, a
PDF, or a simple slide deck, and can be produced by a human recording
themselves **or by an AI agent narrating its own actions**.

## Non-goals (for now — up for debate, see open questions)

- Not trying to replace Loom/video entirely — video still wins for
  narrated walkthroughs where tone/voice matters (e.g. sales demos).
- Not trying to be a full enterprise SOP/knowledge-base platform (Scribe's
  territory) — at least not in v1. No plans for a searchable org-wide
  content library on day one.
- Not (initially) trying to do in-app "Guide Me" style live overlays like
  Tango's Nuggets.

## Why now / why this can win

- AI coding agents (Claude Code, Cursor, etc.) are increasingly doing the
  work themselves — they're well positioned to *narrate* what they did in a
  way no passive click-recorder can, because they already know intent, not
  just coordinates.
- Developers already live in Markdown/git — a story that renders as a clean
  `.md` file with images is something they'll actually use in a PR, unlike a
  hosted SaaS link.
