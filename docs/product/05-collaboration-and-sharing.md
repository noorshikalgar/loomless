# Collaboration & Sharing

This is one of the biggest open decision areas in the whole doc — it
determines whether this stays a local CLI tool or becomes a hosted product
with accounts. Laying out the options rather than picking one.

## Do we need accounts / login at all?

**Option 1 — No accounts, ever. Pure local tool.**
Everything stays on the recorder's machine. Sharing = sending a file
(Markdown + images, or a self-contained PDF/HTML bundle) through whatever
channel they already use (Slack, email, pasted into a PR). Zero
infrastructure, zero auth surface, zero privacy/compliance burden. Downside:
no shareable links, no "who viewed this," no team library — feels
lightweight but limited.

**Option 2 — Optional accounts, local-first by default.**
Tool works fully without login (Scenario A/C above need nothing). Logging
in unlocks: hosted share links, team workspace, comments, history sync
across machines. This is the common pattern (Scribe/Tango both work this
way — free local capture, account needed to share/host). Probably the
safest middle ground, but adds real build cost (auth, hosting, a backend).

**Option 3 — Always requires an account (full SaaS from day one).**
Simplifies the product story (one way to use it) but adds friction to the
exact moment (solo dev, quick capture) where this tool needs to feel
instant. Likely wrong for the primary personas in this doc, but noting it
for completeness.

*No recommendation made here — flagging as the single highest-leverage
open question in the whole doc (see `10-open-questions.md`).*

## Sharing model, if/when it exists

- **Private share link**: only people with the link (and, if logged in,
  on the right team/workspace) can view. Default assumption if links exist
  at all.
- **Public share link**: anyone with the link can view, no login required
  on the viewer's side — useful for open-source PRs, public bug reports,
  external stakeholders. Should be opt-in per story, not default.
- **No link at all**: file stays local / gets attached directly (git,
  Slack upload) — always available regardless of the above, doesn't
  require any backend.
- Expiry / revocation for share links — worth having even in a v1 sharing
  feature, since screenshots can leak sensitive data (see edge cases doc).

## Team workspace (Phase 3, if pursued)

- Shared library of stories per team/org.
- Basic roles: who can create, who can edit others' stories, who can
  delete.
- Whether this becomes a "knowledge base" (Scribe's territory) or stays
  narrowly scoped to "things attached to PRs/tickets" is itself a
  product-direction question, not just a feature checkbox — going broad
  here means competing with Scribe directly; staying narrow keeps the dev-
  workflow focus that differentiates this tool.
