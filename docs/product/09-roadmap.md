# Roadmap (draft — phases, not dates)

Sequencing reflects "prove the core loop before adding infrastructure."
Not committing to timeframes here since scope per phase depends on
decisions in `10-open-questions.md`.

## Phase 0 — Validate the core loop
- Capture engine: click + screenshot capture, working on one OS first.
- CLI: start/stop, dump raw JSON.
- Markdown export, no annotation, no auto-captions.
- Goal: does a human actually find this faster/better than a quick manual
  write-up? Test on real PRs before building anything else.

## Phase 1 — MVP (human path)
- Terminal capture alongside click capture.
- Click-highlight annotation on screenshots.
- Manual step insertion (notes without a click).
- Cross-platform capture engine (all 3 major OSes).
- Basic secret-masking at capture time.

## Phase 1.5 — MVP (agent path)
- MCP server: `start_recording`, `capture_step`, `stop_recording`,
  `generate_story`.
- Validate with a real agent (e.g. Claude Code) narrating a real debugging
  session end-to-end.
- Tune agent-facing tool descriptions for sensible step granularity (see
  edge cases doc).

## Phase 2 — Make it good
- Local editor (reorder/delete steps, edit captions, redact/crop images).
- Auto-captions for non-agent (pure click) recordings.
- PDF export, slide-deck export.
- Idle-timeout / forgotten-recording safeguard.

## Phase 3 — Collaboration (pending the accounts/sharing decision)
- Share links (private + public, per `05-collaboration-and-sharing.md`).
- Activity/edit history (per `06-editing-and-versioning.md`).
- Team workspace, if the product direction goes that way.

## Phase 4 — Expansion (speculative)
- Desktop app for native-app capture beyond browser/terminal.
- Org-wide searchable library.
- Video export as an alternate output.

## Explicit checkpoint before Phase 3

Phase 3 is where this stops being "a CLI tool" and starts requiring a
backend, auth, and hosting. Recommend treating the jump from Phase 2 to
Phase 3 as a deliberate go/no-go decision point, informed by real usage
from Phases 0–2, rather than assuming it's the natural next step.
