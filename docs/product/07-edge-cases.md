# Edge Cases

For each: whether it's realistically catchable early (at capture time) vs.
only fixable later (at edit/export time), and roughly how.

## Sensitive data exposure

- **Passwords / secrets typed during a session.** Catchable early: detect
  `type="password"` fields in the browser, and common secret-looking
  patterns (API keys, tokens) in typed text or terminal output, and
  auto-mask before the screenshot/keystroke is even saved to disk — not
  just before export. Waiting until export to redact means the raw
  sensitive data still sat on disk (and possibly got uploaded) in the
  meantime.
- **Sensitive data already visible on screen** (an API key already
  rendered on a settings page, a customer's PII in a data table) — much
  harder to catch automatically, since nothing distinguishes it from
  ordinary UI text. Realistic approach: flag common patterns
  (email-looking strings, things that match `AKIA...`/`sk-...`-style key
  prefixes) as a "possible sensitive content" warning at export time,
  but this will never be complete — the tool should set the expectation
  that the human/agent reviews before sharing publicly, not that
  detection is guaranteed.

## Capture reliability

- **Click on a fast-changing UI** (screenshot fires a moment after a
  modal/toast already closed) — timing race. Catchable early with a small
  deliberate delay (e.g. 100–200ms) before the screenshot, tunable, at the
  cost of slightly-after-the-fact screenshots.
- **Multi-monitor setups** — which screen to capture. Needs to capture the
  monitor containing the click, not always the primary display.
  Solvable early with correct coordinate-to-display mapping.
- **Rapid clicking / double-clicks** — should these be one step or two?
  Needs a debounce/coalescing rule (e.g. clicks within ~300ms of each
  other on the same element collapse into one step).
- **Non-click interactions that matter** — keyboard-only navigation, drag
  and drop, scrolling to reveal something important. Pure click-capture
  will miss these; worth deciding whether v1 explicitly scopes to "click +
  manual step insertion" and accepts this gap, or invests in
  keyboard-shortcut detection early.

## Terminal / agent capture

- **Very long terminal output** (a build log, a stack trace) — capturing
  it all makes for an unreadable step. Catchable early with a sensible
  truncation rule (show first/last N lines, link to full output) rather
  than dumping everything into the story.
- **Secrets in terminal output/env vars** — same masking approach as
  above, applied to captured stdout/stderr, not just UI screenshots.
- **Agent captures too many or too few steps** — if left entirely to the
  agent's judgment, step granularity could be wildly inconsistent
  (10 tiny steps for a trivial fix, 1 giant step for something complex).
  Worth giving the agent-facing tool description clear guidance on what
  counts as a "step worth capturing," rather than leaving it fully
  open-ended.

## Storage & performance

- **Screenshot volume** — a long recording session could produce hundreds
  of images. Needs a plan for local storage limits, cleanup of
  old/discarded sessions, and image compression, or this quietly eats
  disk space.
- **Recording left running accidentally** — a forgotten `loomless record
  start` running for hours captures irrelevant clicks and burns storage.
  Catchable early with a soft idle-timeout warning ("still recording —
  stop?") after a period of inactivity.

## Cross-platform quirks

- **Permissions**: macOS requires explicit Screen Recording and
  Accessibility permissions for global click/screenshot capture; Linux
  varies by display server (X11 vs Wayland — Wayland restricts global
  input hooks much more than X11, this is a real constraint, not a
  formality). Needs to be surfaced clearly at first-run, not discovered as
  a silent failure.
- **High-DPI / scaled displays** — click coordinates vs. screenshot pixel
  coordinates can mismatch if scaling isn't accounted for, causing the
  click-highlight annotation to land in the wrong place.
