# Users & Personas

## Primary personas

### 1. The individual developer (solo capture)
Wants to document a bug repro, a feature they just built, or a debugging
session — mostly for themselves (future reference) or to attach to a PR /
ticket. Records short sessions (2–15 steps). Cares about speed and low
friction more than polish. Likely records alone, doesn't need
teams/sharing on day one.

### 2. The AI coding agent (agent-driven capture)
Not a human, but a "user" of the tool in a real sense — it calls the MCP
tools to start a recording, capture steps with its own narration, and
generate a story at the end of a task (e.g. "here's what I changed and why,
with screenshots of the before/after"). This is the differentiating persona
— no competitor designs for it.

### 3. The reviewer / teammate (consumer, not recorder)
Doesn't record anything — just reads/reviews a story someone (or some
agent) shared, usually inside a PR, a Slack message, or a ticket. Wants a
fast skim: what changed, why, does it look right. May want to comment on a
specific step.

### 4. The support/debug engineer (repro capture)
Records a bug reproduction to hand to another engineer or to attach to a
bug ticket — "here's exactly how to reproduce this, in 6 steps." Overlaps
heavily with persona 1 but the intent is handoff, not self-reference.

## Secondary / later personas (not core to MVP)

### 5. The team lead / eng manager
Wants a lightweight audit trail of what was done during an incident or a
sprint — "show me the story of how this was debugged." Cares about shared
libraries of stories, maybe searchability. This starts to look like
Scribe's territory — worth being deliberate about how far to go here.

### 6. The technical writer / DevRel
Might use exported stories as raw material for real docs or changelogs.
Cares about export quality (clean Markdown/PDF) more than the live tool.

## Explicitly out of scope (for now)

- Non-technical / ops teams documenting business processes (Scribe/Tango's
  core market) — not the target user, at least initially.
- End customers / external users being walked through a product via
  in-app overlays (Tango's "Guide Me") — different product entirely.
