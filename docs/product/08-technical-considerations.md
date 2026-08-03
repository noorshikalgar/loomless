# Technical Considerations

Options laid out, not decided. Meant to be resolved alongside
`10-open-questions.md`.

## Core capture engine language: Go vs Rust vs other

| | Go | Rust | Node/Electron |
|---|---|---|---|
| Cross-platform global input hooks + screenshot | Good via libraries (e.g. `robotgo`), but some libs are less actively maintained | Good via `rdev`/`enigo` + platform screenshot crates (e.g. `xcap`); more fragmented ecosystem, more manual glue | Weakest fit — needs native modules (`robotjs`-style), historically flaky/unmaintained, packaging is heavier |
| Single static binary, easy distribution | Yes, native strength | Yes, native strength | No — ships a full Chromium/Node runtime, large install size |
| Build speed / iteration speed | Fast | Slower compile times, steeper learning curve | Fast |
| Performance / memory footprint | Good | Best | Weakest (Electron overhead) |
| MCP server implementation | SDKs exist, straightforward | SDKs exist, straightforward | Straightforward — most MCP examples are JS/TS first |
| Team familiarity (unknown — worth confirming) | ? | ? | ? |

Reasonable framing: **Go or Rust for the capture engine core** (binary,
performance, no-runtime-required all matter for a CLI/background tool),
**with a decision needed on team familiarity/preference** — this doc
intentionally doesn't pick one. Node/Electron is realistically only in
play if a rich local editor UI (Phase 2) ends up being the dominant
engineering surface, since Electron is a much better fit for that than for
the capture engine itself.

## Component-level stack notes

- **CLI**: whichever language the engine is written in, exposed as
  subcommands — no separate stack needed.
- **MCP server**: thin wrapper around the same engine's internal API
  (start/capture/stop/generate), exposed over stdio or SSE per the MCP
  spec. Doesn't need to be a different language than the engine.
- **Local editor (Phase 2)**: could be a local web UI (engine runs a
  localhost server, editor is just HTML/JS opened in the browser — avoids
  Electron entirely) or a terminal UI (TUI) for a lighter-weight option.
  Worth deciding based on how much visual editing (cropping, blurring
  screenshots) is actually needed — a browser-based local UI handles that
  far more easily than a TUI.
- **Renderer (Markdown/PDF/slides)**: independent of the capture engine's
  language — Markdown generation is trivial in any language; PDF export
  likely shells out to a headless renderer (e.g. a headless-Chromium-based
  tool) regardless of what the core is written in.
- **Storage format for raw sessions**: a self-contained folder or single
  archive file (step metadata as JSON/YAML + images) — keeps a session
  portable and inspectable without a database, at least until/unless a
  hosted backend (Phase 3 sharing) is built.

## Backend / hosting (only relevant if Phase 3 sharing is pursued)

- Needs: auth, storage for shared stories (images + metadata), a way to
  serve public/private share links, basic team/workspace data model.
- This is a meaningfully large scope jump from "local CLI tool" — worth
  treating as a separate build phase/decision, not something to
  half-build alongside the core engine.

## Security considerations

- Local secret-masking (see edge cases doc) should happen in the capture
  engine itself, before anything touches disk — not bolted on at export
  time.
- If any hosted/share-link feature exists, screenshots and terminal
  captures are now potentially sensitive data leaving the local machine —
  needs explicit user consent per share, not an automatic upload on
  capture.
- MCP tool surface should probably not allow an agent to silently exfiltrate
  a recording to a public link without an explicit human-confirmed step
  (aligns with general "confirm before external actions" practice for
  agent tools).

## Packaging / distribution (once a language is chosen)

- CLI: package via the ecosystem's normal channel (Homebrew, a install
  script, GitHub releases with prebuilt binaries per OS/arch).
- Desktop app (later phase): reuse the same engine binary, wrap with a
  minimal native UI shell rather than rebuilding capture logic in
  Electron.
