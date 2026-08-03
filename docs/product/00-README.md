# Story — Product Doc (v0.1, draft for review)

This is a first-pass product analysis, written to hand to an LLM (or a human PM)
for review, questioning, and decision-making. It intentionally does **not**
make final calls on open questions — those are flagged explicitly so whoever
picks this up next can challenge assumptions, ask clarifying questions, and
confirm direction before engineering starts.

## How to use this doc

Read in this order:

1. `01-vision-and-goals.md` — what problem this solves and why it should exist
2. `02-users-and-personas.md` — who it's for
3. `03-features.md` — what it does, split into MVP / later phases
4. `04-usage-scenarios.md` — 3 real-world walkthroughs of how it gets used
5. `05-collaboration-and-sharing.md` — accounts, teams, sharing, permissions
6. `06-editing-and-versioning.md` — what happens after capture, history/audit
7. `07-edge-cases.md` — things that can quietly break trust in the tool
8. `08-technical-considerations.md` — architecture options, Go vs Rust, etc.
9. `09-roadmap.md` — phased plan
10. `10-open-questions.md` — the single most important file — every unresolved
    decision, collected in one place, meant to be answered before build starts

## One-line pitch

A tool that watches you work (clicking through a feature, debugging an issue,
walking through a demo) and turns that session into a structured, readable
"story" — screenshots, steps, and notes — instead of a video, so it can be
skimmed, edited, diffed, and dropped into a PR, ticket, or doc. It's built to
be driven by a human **or by an AI coding agent** through an MCP interface.
