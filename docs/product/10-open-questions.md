# Open Questions

The single most important file in this doc. Collected here so whoever
(human or LLM) picks this up next can go through them explicitly rather
than have assumptions baked in silently elsewhere. Grouped roughly by
how early they need to be resolved.

## Must decide before any build starts

1. **Platform order** — confirmed: desktop first, browser extension later
   (per earlier discussion). Does that still hold, or does the MCP/agent
   angle change the calculus (an agent mostly needs terminal + browser
   automation, less native-app capture)?
2. **Language for the capture engine** — Go vs Rust vs something else.
   See `08-technical-considerations.md`. Depends partly on team
   familiarity, which isn't captured in this doc.
3. **Accounts/login** — none, optional, or required. This single decision
   determines whether this stays a local tool or becomes a hosted product
   with a backend. See `05-collaboration-and-sharing.md`.
4. **Editing/versioning model** — immutable, mutable-with-history, or
   mutable-with-no-history. See `06-editing-and-versioning.md`. Affects
   data model from day one if history needs to be captured retroactively.

## Should decide before Phase 1 (MVP) ships

5. Where does secret-masking happen, and how aggressive is the default?
   Over-masking (false positives) vs under-masking (real leaks) is a
   real trade-off, not a solved problem.
6. What counts as a "step" for agent-driven capture — is there a minimum
   granularity guideline, or is it left entirely to the agent's judgment?
7. Does v1 need multi-monitor and high-DPI handling correctly, or is
   single-monitor/standard-DPI an acceptable v1 constraint?
8. Scope of terminal capture — full shell session, or only commands run
   through an explicit wrapper/PTY the tool controls? Full-session capture
   is more powerful but a bigger surface for accidentally capturing
   secrets or irrelevant noise.

## Can defer until Phase 2/3, but worth flagging now

9. If sharing exists — public links: on by default per-story, or strictly
   opt-in? (Doc leans opt-in, not decided.)
10. Team workspace scope — does this stay narrowly "attach to a PR/ticket"
    or grow into a searchable knowledge base (i.e., how directly do we
    want to compete with Scribe)?
11. Does redaction (blurring a screenshot) need its own audit trail, or is
    that overkill for v1 of versioning?
12. Local editor: web UI (localhost, browser-rendered) vs TUI vs neither
    in early phases?

## Meta-question for the next reviewer

Is the framing in this doc — "dev-focused, agent-drivable story capture,
differentiated from Scribe/Tango by the MCP/agent angle" — actually the
right wedge, or should the product go broader (competing more directly on
general SOP/documentation) or narrower (e.g. purely a bug-repro tool, drop
the "feature documentation" use case entirely)? Worth explicitly
confirming the target wedge before scoping Phase 0, since several feature
and technical decisions above depend on it.
