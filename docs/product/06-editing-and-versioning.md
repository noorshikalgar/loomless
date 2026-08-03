# Editing & Versioning (post-capture trust)

## The core tension

Once a story can be edited after capture (reorder steps, delete a step,
change a caption, blur/crop a screenshot), it stops being a "recording" and
becomes a "document someone assembled." That's often desirable (you want to
delete an accidental step, fix a typo in a caption) — but it directly
undermines the trust property that makes a story useful for bug repros and
PR reviews in the first place: **"this is exactly what happened."**

This needs a deliberate answer, not a default. Options:

## Option 1 — No editing after capture (immutable stories)
Simplest, highest trust. Steps are exactly what was recorded; if it's
wrong, re-record. Downside: no way to redact a screenshot you forgot to
blur, no way to remove an irrelevant step, no way to fix a bad
auto-caption. Probably too rigid for real use, but the trust baseline
worth measuring other options against.

## Option 2 — Editing allowed, but every story keeps a visible activity log
Every change (step deleted, caption edited, order changed, image redacted)
is recorded with what changed and when — and, if accounts exist, by whom.
The rendered story can show a subtle "edited" indicator (per step or
overall) so a reader knows they're not looking at a raw, untouched capture.
Doesn't block edits, but preserves the ability to "backtrack" and see the
original. This is the most balanced option and probably the direction to
lean, but see open questions — worth the LLM/next reviewer confirming.

## Option 3 — Editing allowed, no history at all (fully mutable, no audit)
Fastest to build, most flexible for the user, but a reader has no way to
know if a repro story was quietly altered after the bug was "fixed" to make
it look cleaner, or whether a step was removed that mattered. Weakest trust
model — probably wrong for the bug-repro and PR-review use cases, but worth
naming explicitly since it's the "do nothing extra" default if versioning
isn't deliberately designed in.

## What "activity history" would need to capture, if we go with Option 2

- Original raw capture, preserved separately from the edited/published
  version (so "revert to original" is always possible).
- A diff-able change log: field changed, previous value, new value,
  timestamp, actor (if known).
- Distinguish **capture-time data** (screenshot, click coordinates, raw
  terminal output — should never silently change) from **presentation-time
  data** (caption text, step order, redaction/blur, which steps are
  hidden from the exported view) — edits to the latter are expected and
  fine; edits to the former are the ones that should be flagged loudly.
- Redaction is a special case worth calling out: blurring a screenshot to
  remove a secret should be allowed and encouraged, but the tool should
  probably still retain (locally, unshared) the fact that a redaction
  happened and roughly where, in case of dispute — without retaining the
  actual sensitive content anywhere it could leak.

## Related open question

Does this apply to solo/local use too, or only once sharing/teams exist?
A solo dev doesn't necessarily need an audit log for their own local
edits — this may only matter once a story is shared with someone who
wasn't in the room for the capture. Worth deciding explicitly rather than
building history tracking everywhere by default.
