# Decisions Log

Resolutions to the "must decide before any build starts" questions in
[`10-open-questions.md`](10-open-questions.md). Each entry: decision, date,
rationale, and what it unblocks/still leaves open.

## 1. Platform order

**Status: not revisited yet.** Standing assumption carried over from prior
discussion: desktop first, browser extension later. Still open whether the
MCP/agent angle changes this — revisit at Phase 0 kickoff since an
agent-driven session leans more on terminal + browser automation than
native-app capture.

## 2. Capture engine language — **Go**

Decided 2026-08-03. Single static binary fits the CLI-first distribution
model, iteration speed matters more than Rust's performance ceiling at this
stage, and the MCP SDK story is straightforward. Revisit only if a specific
`robotgo`-class library proves unmaintained during Phase 0 spike.

**Unblocks:** repo structure (`cmd/`, `internal/` Go layout), CI tooling
choice, packaging plan (Homebrew/GitHub Releases per
[`08-technical-considerations.md`](08-technical-considerations.md)).

## 3. Accounts/login — **Optional, local-first**

Decided 2026-08-03. The tool works fully with zero login for Phases 0-2
(record, export, paste into a PR/ticket). Login is only required to unlock
hosted share links / team workspace in Phase 3. Matches the
Scribe/Tango pattern per
[`05-collaboration-and-sharing.md`](05-collaboration-and-sharing.md).

**Unblocks:** confirms Phases 0-2 need no backend/auth work at all — local
CLI + local storage is sufficient until the Phase 3 go/no-go checkpoint in
[`09-roadmap.md`](09-roadmap.md).

## 4. Editing/versioning model — **Mutable with activity log**

Decided 2026-08-03. Editing is allowed (reorder, redact, edit captions) but
every change to a story is logged, and the rendered output can show an
"edited" indicator. Original raw capture is preserved separately so a
revert is always possible. Per
[`06-editing-and-versioning.md`](06-editing-and-versioning.md) Option 2.

**Unblocks:** data model needs to separate **capture-time data**
(screenshot, click coordinates, raw terminal output — immutable once
written) from **presentation-time data** (caption, order, redaction,
visibility) from day one, even before sharing/accounts exist in Phase 3 —
so the log has something to log retroactively instead of being bolted on
later.

## 5. Screenshot capture scope — **window-specific, not full-screen**

Decided 2026-08-03. Every capture is scoped to a single window (the
frontmost/active window, or an explicitly chosen one) rather than the whole
display. Narrows the surface for accidental sensitive-data capture (other
apps, other tabs, menu bar/notifications never appear), and makes the
resulting screenshots more relevant per step. Element/region-level capture
(cropping to a specific control, not just the window) is a stretch goal for
the same spike — attempt it, but window-level is the acceptance bar for
Phase 0, not element-level.

Implementation note (macOS first, per platform-order default): use the
built-in `screencapture -l <windowID>` rather than a full-display grab, with
window ID resolved via `osascript`/System Events for the frontmost window,
or from a `loomless windows list` picker for a specific one. No CGO/Accessibility
global-hook dependency needed for this — keeps the Phase 0 spike buildable
without extra OS permission prompts beyond Screen Recording.

**Unblocks:** Phase 0 capture engine scope — see
[`../../CONTRIBUTING.md`](../../CONTRIBUTING.md) for where this lands
(`feature/phase-0-capture-engine`).

## 6. Terminal capture scope — **explicit wrapper, not full shell session**

Decided 2026-08-03 (resolves open question #8). `loomless run -- <command>`
runs one command as a subprocess and captures its output as a step —
it does not attach to or tee the user's actual interactive shell.

Consistent with every capture-surface decision so far (window-scoped not
full-screen, listen-only click tap, optional accounts): default to the
narrower, explicit surface. A full-session shell wrapper would capture
every command the user runs for the entire recording — including ones
that have nothing to do with the story and are more likely to contain
secrets never meant for the output. An explicit per-command wrapper only
captures what's deliberately routed through it, at the cost of the user
needing to type `loomless run --` in front of commands they want captured.

Long output is truncated to a head/tail preview in the rendered story
(per `07-edge-cases.md`); full raw output is always saved alongside the
session so nothing is silently lost, just not inlined.

Secret-masking (decision #7, below) now runs on this path before anything
is written to disk.

## 7. Secret-masking — **pattern-based, text fields only, unconditional**

Decided 2026-08-03 (resolves open question #5). `internal/mask` pattern-
matches common secret shapes (AWS keys, `sk-`/`gh_`/`xox`-prefixed API
tokens, JWTs, PEM private-key blocks, and `label=value`/`--flag value`
style assignments) and replaces matches with `«masked»` before any text
touches disk. Applied unconditionally inside `session.AddStep` /
`AddTerminalStep` — not something a caller opts into — so every current
and future capture path (manual step, click daemon, `loomless run`, and the
Phase 1.5 MCP server later) gets it automatically. Covers: step notes,
window titles, terminal commands, and terminal output.

**Scope, deliberately narrow:** text only. Screenshot pixels are not
scanned — per `07-edge-cases.md`, reliable secret detection in arbitrary
pixel content needs OCR, which is real Phase 2 "auto-blur" scope, not a
Phase 1 pattern-match. And masking only governs what Loomless *saves*: a
command run via `loomless run` still prints its real, unmasked output live to
your actual terminal exactly as it would without Loomless — masking can't
and shouldn't suppress that, only what lands in `session.json` and the
output log.

Default aggressiveness leans toward **fewer false positives over catching
everything** (the doc's stated trade-off, left undecided until now): an
early version of the `label=value` rule also matched on bare whitespace
(`password reset`, `a secret plan`) and flagged ordinary prose caught by
the test suite — tightened to require an explicit `:`/`=` separator, plus
a separate `--flag value` rule for CLI-style args, which are unambiguous
without a separator. This will still miss real secrets in unrecognized
shapes; per the edge-cases doc, that's expected — this is a safety net,
not a guarantee, and the human/agent reviewing before sharing is still
the real backstop.

Each step records *which* rules fired (`Masked: []string`, rule names
only, never matched content) — visible in the CLI output and in
`session.json` — so masking is never silent.

## 8. MCP tool surface — the doc's 4, plus 2

Decided 2026-08-03 (Phase 1.5, per `09-roadmap.md`). `loomless mcp` runs an
MCP server over stdio exposing `start_recording`, `capture_step`,
`stop_recording`, `generate_story` — the exact 4 named in
`03-features.md`/`04-usage-scenarios.md` Scenario B — plus two more:

- `run_command`: the agent-facing equivalent of `loomless run`. Scenario B's
  own example ("ran the failing test to see the exact error", "re-ran the
  test suite, all green") is literally an agent running commands and
  wanting them captured — without this tool the agent would have to use
  its own separate shell tool and then manually describe the command via
  `capture_step`'s note, losing the actual output. Included because the
  scenario the doc describes needs it, not because it was asked for by
  name.
- `list_windows`: lets an agent sanity-check what a capture would resolve
  as the frontmost window before calling it — cheap, read-only, avoids
  capturing the wrong window blind.

Implementation: `cmd/loomless/mcp.go` + a new `internal/engine` package.
Business logic (session lifecycle, click-daemon start/stop, step capture,
command execution, export) moved out of `cmd/loomless/main.go` into
`internal/engine`, shared unchanged between the CLI and the MCP server —
per `08-technical-considerations.md`'s "thin wrapper around the same
engine's internal API." One hard invariant enforced there: nothing in
`internal/engine` writes to `os.Stdout` — the MCP stdio transport uses
stdout as the protocol stream, so a stray print would corrupt it. The
CLI's live terminal echo for `loomless run` is a tee the CLI adds on top
(`engine.RunCommand` takes optional tee writers, nil from the MCP path).

Verified against the real SDK client (`github.com/modelcontextprotocol/
go-sdk`, not just manual JSON-RPC): connected, listed all 6 tools, drove a
full start_recording → capture_step → run_command → list_windows →
stop_recording → generate_story sequence, and confirmed a fake secret
in `run_command`'s output came back masked in both the tool result and
the final generated Markdown.

## 9. Multi-monitor / high-DPI — reasoned correct by design, not hardware-verified

Reviewed 2026-08-03 (open question #7). This resolves to "should already
work, not confirmed on real hardware" rather than a clean yes/no, because
the dev machine used for every capture test in this project has exactly
one external display (2560×1440, 1:1 scale — not even Retina). There's no
second monitor or HiDPI panel attached to actually exercise either path
against.

Why it should already be correct, by construction, without needing
monitor-specific code:

- `capture.WindowInfo.X/Y/W/H` (`internal/capture/window_darwin.go`) come
  straight from AppleScript/System Events' `position`/`size` of the
  window, which macOS reports in **global** screen-point coordinates —
  the same virtual coordinate space that spans every connected display,
  not display-relative coordinates. A window on a second monitor
  positioned to the right of the main one just has an X coordinate
  greater than the main display's width; one positioned above/left can
  have negative coordinates. Nothing in our code assumes a single
  display's bounds.
- `capture.Screenshot` (same file) calls `screencapture -x -o -R
  x,y,w,h` with those same global coordinates, and deliberately never
  passes `-m` ("only capture the main monitor") or `-D <n>` (a specific
  display index) — confirmed via `man screencapture` — so `-R`'s region
  resolution is left to macOS itself, which already has to solve
  "which physical display(s) does this global rectangle fall on, and at
  what backing scale factor" for its own window manager. We're not
  reimplementing any of that math — see decision #5's implementation
  note, which chose `-R` over manual pixel-cropping for exactly this
  reason.
- The one real gap this reasoning doesn't cover: a window straddling two
  displays with *different* backing scale factors (e.g. half on a Retina
  laptop panel, half on a non-Retina external one) would hand
  `screencapture` a region spanning two different pixel densities. What
  it does with that isn't something `man screencapture` documents, and
  it's a genuinely rare window position — not worth engineering around
  speculatively.

**Known verification gap, stated plainly:** this needs an actual
multi-monitor and/or Retina setup to hands-on confirm rather than reason
about. Flagging rather than claiming "done" — revisit with real hardware
before treating this as closed.

## 10. Linux capture backend — X11 only, window/screenshot done, click-capture out of scope

Decided 2026-08-03 (closes part of the Phase 1 "cross-platform" gap).
`internal/capture/window_linux.go` implements `Frontmost`/`List`/
`Screenshot` for X11 by shelling out to standard tools — `xdotool` for the
active window and geometry, `xprop` for `WM_CLASS` (the closest X11
equivalent of "app name"), `wmctrl -lxG` for listing, and `maim -g` for
region screenshots — mirroring the macOS backend's own approach (no cgo,
no platform SDK headers, just well-known CLI tools). Automatic
click-capture is explicitly **not** implemented (`clickhook_linux.go`
fails loudly with a clear message rather than silently no-op'ing): X11
needs the XTest/RECORD extension or a privileged input read, Wayland
restricts global input observation entirely by design, and neither has
the "shell out to a standard tool" escape hatch the rest of this backend
uses. `loomless step`/`capture_step` and `loomless run`/`run_command` (manual
capture) work today on Linux; only the automatic click daemon doesn't —
the existing graceful-degradation path (record start warns and falls back
to manual-only) already covers this correctly, unchanged.

**How this was actually verified — not just cross-compiled.** No Linux
machine is available in this environment, but a real X11 session is
buildable locally: a Debian container (via the Docker/OrbStack already on
this machine) with `Xvfb` (virtual framebuffer X server), `fluxbox` (a
minimal window manager — required for `wmctrl`/`xdotool getactivewindow`
to work at all, since bare Xvfb has no EWMH hints without one), `xdotool`,
`wmctrl`, `xprop`, and `maim` installed, running a real `xterm` window.
The actual cross-compiled Linux `loomless` binary was copied in and run
directly against that live session — not a simulation.

This caught two real bugs that cross-compilation alone would never have
surfaced:

1. `windowInfoForID` called `xdotool getwindowclassname`, which doesn't
   exist — confirmed against `xdotool help`'s actual command list. Fixed
   by using `xprop -id <id> WM_CLASS` instead (ICCCM's real mechanism for
   this) and parsing its `"instance", "Class"` output.
2. Independently, and worse: when window resolution failed (which it did,
   because of bug #1), `loomless step` reported success with **no
   indication anything had gone wrong** — no window, no screenshot, no
   warning. Traced to `engine.CaptureStep` (`internal/engine/engine.go`),
   which silently discarded the resolution/screenshot error
   (`werr`/`capture.Screenshot`'s error) instead of surfacing it, a
   regression introduced during the Phase 1.5 engine refactor (decision
   #8) — the pre-refactor CLI code had printed this warning; the
   extraction into `internal/engine` dropped it. Fixed by giving
   `CaptureStep` the same `(step, warning, err)` shape `StartRecording`
   already uses, threaded through both the CLI's `loomless step` and the MCP
   `capture_step` tool's output.

Bug #2 is the more important finding: it means a capture failure could
have been silently invisible on **any** platform, not just Linux — this
project's own dogfooding on macOS just hadn't hit a window-resolution
failure recently enough to notice, since Accessibility permission was
already granted from earlier testing. Re-verified end to end after both
fixes: `loomless windows list`, `record start` (correctly warns + falls back
to manual since click-capture isn't implemented), `loomless step` (now shows
`(XTerm — story-linux-test-window)` and saves a real screenshot),
`loomless run`, `record stop`, and `export` all confirmed working against
the live container session — screenshot bytes pulled out and visually
inspected,
not just checked for existence.

**Still unverified:** Wayland (X11-only tools, as documented, will not
work there at all — no attempt made to test or support it here), and this
was one specific X11/WM/tool-version combination (Debian bookworm's
xdotool/wmctrl/maim, fluxbox) — behavior on other distros or window
managers isn't guaranteed identical.

## 11. Idle-timeout: separate watchdog process, desktop notification, not auto-stop

Decided 2026-08-03 (first Phase 2 item — the "forgotten recording" edge
case in `07-edge-cases.md`). `record start` now also launches a second
background process (`internal/engine.RunIdleWatchdog`, same
relaunch-and-detach pattern as the click daemon, refactored into a shared
`startBackgroundProcess`/`stopBackgroundProcess` helper pair) that checks
the session's last-activity time and sends a desktop notification once it
exceeds a threshold (default 20 minutes, re-notifying at most once per
threshold window rather than spamming every check) — a **soft nudge**,
matching the edge-cases doc's own wording ("still recording — stop?"), not
an automatic stop. Auto-stopping was considered and rejected: silently
ending a recording out from under someone who stepped away mid-task (a
meeting, a phone call) would be a worse surprise than a forgotten
recording burning some extra disk space.

Deliberately its **own** process/PID, not folded into the click daemon:
the click daemon doesn't exist at all on platforms/permission-states where
click-capture isn't available (Linux today, or macOS without Accessibility
granted) — see decision #10 — and the idle nudge is exactly as useful
there, arguably more so, since manual-only capture is easier to forget
about mid-task than click-driven capture.

Notification delivery (`internal/notify`) is its own tiny package,
platform-specific and best-effort: `osascript ... display notification` on
macOS, `notify-send` on Linux, a no-op elsewhere. A failed/missing
notification backend is logged, not fatal — the watchdog keeps running
either way.

Both the warn-after threshold and the check interval are overridable via
`LOOMLESS_IDLE_WARN_AFTER`/`LOOMLESS_IDLE_CHECK_INTERVAL` env vars (Go duration
strings, e.g. `5m`) — added for testability (verifying a 20-minute default
without actually waiting 20 minutes) but just as legitimately useful for a
real user who wants a shorter or longer threshold than the default.

Verified end to end on **both** supported platforms, not just macOS:
locally with the env vars set to a few seconds — confirmed the watchdog
starts, notifies exactly once per idle window (not every check tick),
and shuts down cleanly (PID file removed, "stopped" logged) on `record
stop`; and inside the same Linux/X11 container used for decision #10 —
confirmed the same start/notify-attempt/stop lifecycle, including that a
missing `notify-send` (not installed in the minimal test container) fails
the notification attempt gracefully without taking down the watchdog or
the recording.

Caught during testing: the first version rounded the idle duration to the
nearest **minute** for the log/notification text, which made anything
under 30 seconds display as "0s" — obviously wrong, and only obvious
because the fast env-var-driven test used seconds-scale durations. Fixed
by rounding to the nearest second instead, which reads fine at both the
test scale and the real 20-minutes-plus scale.

## 12. PDF export — direct session-to-PDF, pure Go, ASCII-sanitized text

Decided 2026-08-03 (second Phase 2 item). `loomless export <name> --format
pdf` and the MCP `generate_story` tool's `format: "pdf"` render straight
from `session.Session` to PDF via `internal/export/pdf.go`, using
[`github.com/go-pdf/fpdf`](https://github.com/go-pdf/fpdf) — a pure-Go PDF
library, no cgo, no external binary dependency (unlike the "headless
Chromium" option `08-technical-considerations.md` floated). Chosen over
converting the rendered Markdown to PDF: converting Markdown would need
a Markdown-to-PDF pipeline of its own (and likely the same headless-browser
dependency this avoids), whereas rendering directly from the structured
`Session` data — the same source `export.Markdown` already reads — needed
nothing extra.

**Text encoding tradeoff, deliberate:** fpdf's core fonts (Helvetica,
Courier) aren't UTF-8 aware without either an embedded TTF file or a
bundled codepage descriptor (`.map` file) for its Unicode-translator path
— both are assets this project would have to ship. Rather than add a font
dependency for what's realistically a handful of characters (em dashes,
smart quotes, the masking placeholder's guillemets `«»`), `safe()` maps
the common ones to ASCII equivalents (`—`→`--`, `«»`→`<<>>`, etc.) and
replaces anything else non-ASCII with `?`. Good enough for this project's
own text (comments, notes, captions); would need revisiting if this ever
needs to render non-Latin scripts.

Screenshots are embedded directly (read from `dir/step.ImageFile`, not
just linked the way Markdown does it — PDF has no equivalent of a relative
link to another file), scaled to the page's usable width preserving aspect
ratio, downscaled further if that would exceed a full page's height.
Terminal steps get the same masked `Output`/`Command` fields the Markdown
exporter uses — masking (decision #7) runs once, at capture time, and
every export format inherits it for free.

Verified visually on both platforms, not just "the command exits 0": a
real session (a note-and-screenshot step plus a terminal step with a fake
secret) exported to PDF and read directly (this tool renders PDF pages)
on macOS — confirmed the title, wrapped note text, real embedded
screenshot at correct proportions, and `«masked»`→`<<masked>>` all
rendered correctly, not garbled. Repeated the terminal-step half of that
in the same Linux/X11 container used for decisions #10/#11 — confirmed an
identical result from the identical pure-Go code path (no OS-specific
code exists in `pdf.go`, so this was mostly confirming that expectation
rather than expecting a surprise, but confirmed rather than assumed).

## 13. Slide-deck export — self-contained HTML, no CDN dependency

Decided 2026-08-03 (third and final Phase 2 export format).
`loomless export <name> --format slides` and the MCP `generate_story` tool's
`format: "slides"` render a single self-contained `.html` file
(`internal/export/slides.go`) — one step per slide, step-through via
on-screen Prev/Next buttons or arrow keys, screenshots embedded as base64
data URIs so the file has no dependency on the `images/` folder traveling
with it (unlike Markdown's relative links).

**No CDN.** `08-technical-considerations.md` mentions reveal.js as a
possible approach; deliberately not used here — a slide deck that only
renders with an internet connection contradicts this project's
local-first stance (decision #3). The whole deck (CSS, JS, markup, images)
is under 400 own hand-written lines plus embedded image bytes; a full
presentation framework would be a lot of dependency for what a browser
already does natively.

**Bug found and fixed via actual interactive browser testing, not just
"the file was created":** the first version wired the Prev/Next buttons
via inline `onclick="..."` HTML attributes. Verified in this project's
Browser tool (served over local HTTP — `file://` navigation is blocked by
the tool) that clicking the buttons at their exact, confirmed-correct
on-screen coordinates did **nothing** — the slide counter never advanced.
Isolated the cause methodically: `elementFromPoint` at the click
coordinates confirmed the click was landing squarely on the right button
(not an overlap/z-index issue); calling `document.getElementById
('nextBtn').click()` from the console worked instantly; calling the
`storyGo()` function directly worked instantly. Only the *synthetic click
dispatched by the testing tool* failed to trigger the inline `onclick`
handler specifically — switching to `addEventListener('click', ...)`
(strictly better practice regardless) fixed it, re-verified with the same
real synthetic click producing the correct slide change afterward.

A related, narrower finding: the *keyboard* arrow-key path's listener
logic was independently confirmed correct (`document.dispatchEvent(new
KeyboardEvent('keydown', {key:'ArrowLeft', bubbles:true}))` from the
console advances the slide exactly as the code intends), but the testing
tool's own synthetic key-press action didn't produce a `KeyboardEvent`
the page could see — appears to be a limitation of that specific
automation tool's key-dispatch, not a defect in the shipped page (a real
physical key press in a real browser is standard, thoroughly-tested
platform behavior, not something this project's code could plausibly
break). Noted rather than silently assumed fine, since "didn't manage to
verify" and "verified working" are different claims and this is the
former for keyboard nav specifically, even though the underlying logic
was independently confirmed correct by other means.

## 14. Local editor — web UI, resolves open question #12, real edit-log implementation of decision #4

Decided 2026-08-03 (last Phase 2 item this pass — screenshot redaction is
a separate follow-up, not included). `loomless edit <name>` runs a
localhost-only (`127.0.0.1`, never `0.0.0.0` — this is a single-user local
tool, not something to expose on a network) HTTP server
(`internal/editor`) serving a single-page app: reorder steps, hide/show
them, edit captions. Resolves open question #12 in favor of a web UI, not
a TUI — `08-technical-considerations.md`'s own reasoning already pointed
here ("a browser-based local UI handles [visual editing] far more easily
than a TUI"), and this slice already needs to show screenshots inline
next to editable captions, which a TUI can't do at all.

**This is also the first real implementation of decision #4's data model**
(mutable-with-activity-log), not just the decision to build one later.
`Session` gained `DisplayOrder`/`Hidden`/`EditLog` — deliberately kept
separate from `Steps` rather than mutating step order/identity in place:
a `Step`'s `Index` (and everything derived from it — `image_file`,
`output_file`) is capture-time data that's never renumbered, exactly the
"capture-time data should never silently change" principle
`06-editing-and-versioning.md` states. Reordering/hiding/captioning are
presentation-time edits layered on top, each one appended to `EditLog`
with old/new values — the "visible activity log" that document requires.
Every exporter (Markdown/PDF/Slides) now goes through a new
`Session.VisibleSteps()` helper instead of ranging over `Steps` directly,
so a hide/reorder edit is guaranteed to show up in every export format,
not just whichever one someone remembered to update.

Edited captions run through the same `internal/mask` secret-masking
(decision #7) new captured text does — an edited caption is just as
capable of containing a pasted secret as an originally-typed one, and
masking being "unconditional across every capture path" should mean every
*text-entry* path too, not just the original ones.

**Verified with real unit tests, not just manual clicking**, for the
trickiest part (`VisibleSteps()`'s ordering/filtering logic — the exact
kind of easy-to-get-subtly-wrong logic the mask false-positive earlier in
this project was): natural order, explicit reorder, hidden exclusion,
both together, all-hidden (empty result, not a crash), and a stale
`DisplayOrder` entry referencing a step that no longer exists (skipped,
not a crash) — `internal/session/session_test.go`. All passed first try.

**Then verified interactively for real**, the same way the slide-deck
click bug was caught: started a real editor server against a real
session (3 steps: one window/screenshot step, one terminal step, one more
window/screenshot step), drove it through the actual Browser tool —
clicked "Hide" on step 3 (confirmed dimmed in the UI, confirmed
`hidden: [3]` + an `EditLog` entry on disk), edited step 3's caption and
saved (confirmed the new text persisted, confirmed the old and new values
both landed in `EditLog`), and reordered steps 1 and 2 (confirmed the
swap rendered correctly, confirmed `display_order: [2,1,3]` on disk). Then
re-exported to Markdown and Slides and confirmed both showed steps in the
edited order with step 3 excluded — the same `VisibleSteps()` call the
unit tests already covered, now confirmed wired correctly end to end
through the HTTP API and browser UI on top of it, not just correct in
isolation.

**Screenshot redaction/crop — the visual-editing capability that most
motivated choosing a web UI over a TUI in the first place — is
deliberately not included in this pass.** It's a distinct enough
sub-feature (client-side canvas drawing + server-side image
pixel-flattening, with its own correctness bar: a redaction has to
actually make the covered content unrecoverable, not just visually hide
it) that it deserves its own focused pass rather than being squeezed in
here. Flagged as a follow-up task, not silently dropped.

## 15. Click-highlight toggle

Decided 2026-08-03, in response to a direct user question ("can the click
highlight be configurable, if user wants it can else remove"). The
automatic red-ring annotation on click-captured screenshots (decision #3
in the original Phase 1 work) was previously always-on with no way to
turn it off. `loomless record start <name> --no-highlight` and the MCP
`start_recording` tool's `no_highlight: true` disable it — the daemon
still captures a screenshot per click, just without the ring drawn on it.
Default stays on (opt out, not opt in), since the annotation is more
often useful than not for the primary bug-repro/PR-review use cases.

Plumbing: the click daemon is a separate relaunched process (`loomless __click-daemon <name>`), so the flag has to travel as a real process
argument, not just an in-memory bool — `engine.startBackgroundProcess`
gained a variadic `extraArgs` parameter for this, reused by
`startClickDaemon` to append `--no-highlight` when needed (the idle
watchdog doesn't need any extra args, so its call site is unchanged).

**Verified the plumbing end-to-end, not just that it compiles:** started
one session with `--no-highlight` and one without, then inspected the
actual running daemon processes' command lines (`ps -ax -o
pid,command`) — confirmed the flag reaches the spawned process exactly
when expected and is absent otherwise. What's *not* verified end-to-end:
whether a real click, with the flag set, actually produces an
un-annotated screenshot — that needs a real OS-level click firing through
the event tap, which (same limitation noted when click-capture was first
built) can't be synthesized in this environment. The conditional itself
(`if highlightClicks { ...capture.Highlight... }`) is simple and was
reviewed, and `capture.Highlight` itself was already thoroughly verified
in the original click-capture work — but "the flag reaches the process"
and "a real click respects the flag" are different claims, and only the
first is confirmed here.

## 16. Screenshot redaction + manual highlight — combined into one image-editing pass

Decided 2026-08-03, following on from decision #15's user question about
configurability, and closing the follow-up flagged when the local editor
first shipped (decision #14). Redaction (hide something sensitive) and
manual highlighting (draw attention to something) are opposite intents
but identical mechanics — click-and-drag a rectangle on a screenshot in
`loomless edit` — so they shipped together rather than as separate passes.

- `capture.Redact(pngData, x, y, w, h)`: solid black fill. Deliberately
  solid, not blur — per `06-editing-and-versioning.md`'s redaction
  carve-out, covered content must be genuinely unrecoverable, and a blur
  is sometimes reversible with enough effort where an overwrite isn't.
- `capture.DrawBoxHighlight(pngData, x, y, w, h)`: outline only, reusing
  the same red (`highlightColor`) as the automatic click-ring marker —
  the manual, user-placed equivalent of that automatic one.
- `session.EditImage`: the one deliberate exception to "capture-time data
  is never mutated" (decision #4) — redaction only works if the original
  pixels are actually gone, so the image file is overwritten in place.
  Still logged to `EditLog` (what happened, the rectangle, never the
  covered content — which was never retained anywhere to leak).
- Editor UI: click "Draw Redaction" or "Draw Highlight" to arm that mode
  (crosshair cursor, button highlighted), then drag on the image. Single-
  shot and auto-disarming after one rectangle, rather than a persistent
  toggle — stays unambiguous which mode is armed with several steps'
  images on one page. Drag coordinates convert from displayed (CSS,
  often scaled down) pixels to the image's natural pixel space before
  hitting the API.

**Verified three ways, escalating rigor:**

1. Real unit tests (`internal/capture/annotate_test.go`) proving the
   *distinction* the two operations exist for: `Redact` fills the entire
   rectangle solid black and leaves everything outside it byte-for-byte
   unchanged; `DrawBoxHighlight` paints only a border and leaves the
   rectangle's interior untouched — if a highlight ever accidentally
   filled solid, it would be indistinguishable from a redaction, which
   would defeat the reason both exist. Both passed first try.
2. Interactively, through the actual Browser tool against a real editor
   server: armed redact, dragged a box, confirmed a solid black region
   rendered in place, confirmed `EditLog` recorded `image_redact` with
   the rectangle; armed highlight, dragged a larger box around the same
   area, confirmed a red outline rendered around (not over) the existing
   redaction — proving sequential edits to the same image compose
   correctly, since each `EditImage` call reads the current on-disk
   bytes rather than the original.
3. **Independent pixel verification**: read the actual saved PNG off
   disk with Pillow (a tool with no connection to this project's own
   code, so it can't share a bug with it) and confirmed the redacted
   region's pixels are genuinely `(0, 0, 0, 255)` — not just "looked
   black in a browser screenshot," which a CSS overlay or a rendering
   artifact could also produce. This is the actual bar redaction has to
   clear: real unrecoverable pixels, verified by something outside the
   system under test.

## 17. Editor image-editing fixes: real coordinate bug, adjust-before-apply, undo/redo/reset, and a click-highlight refinement

Decided/fixed 2026-08-03, all in response to direct user testing of
decision #16's redact/highlight feature — real bug reports from someone
actually using it, not self-identified issues.

**Bug: drag rectangles landed offset from where the user actually
dragged.** Root cause: the drag overlay's `position: absolute` was
positioned relative to `.img-wrap` (which also contained the toolbar
above the image), but the drag coordinates were computed relative to the
`<img>` element itself — two different origins that disagreed by roughly
the toolbar's height, so every rectangle rendered shifted up from where
it was drawn. Fixed by introducing `.img-frame`, a wrapper containing
*only* the image and its overlay/handles, so both the coordinate
computation and the overlay's positioning share the same origin by
construction rather than by a hand-tuned offset. Verified precisely (not
just "looks right"): dispatched synthetic `MouseEvent`s with exact
`clientX`/`clientY` values against the real running editor and compared
the resulting overlay position against the intended one — landed within
half a pixel (floating-point rounding from `getBoundingClientRect()`),
for the draw, move (drag the rectangle body), and resize (drag a corner
handle) interactions all separately. An earlier verification attempt
mixed two different coordinate spaces (the Browser tool's own screenshot-
pixel space, which is scaled relative to real CSS pixels, vs. real CSS
pixels from `getBoundingClientRect()`) and produced a scaled-looking
false result — worth naming so a future reviewer doesn't mistake that
methodology error for a real bug in the fix.

**No way to adjust or undo a placed rectangle.** Redesigned the
interaction from "drag commits immediately" to draw → adjust (move the
whole rectangle by dragging its body, resize from any of 4 corner
handles) → explicit Apply/Cancel. Nothing reaches the server until
Apply. Added `loomless edit`-scoped Undo/Redo/Reset per step, backed by
`internal/editor/history.go` — an in-memory (not persisted; lost if the
editor process exits) stack of previous image bytes, deliberately kept
out of `session.EditImage`/`Session` itself: undo/redo history is
ephemeral editing-session state, not part of the durable story the way
`EditLog` is. Every undo/redo/reset **is** still logged to `EditLog`
(via a new `session.LogEdit`, reusable for edits whose actual pixel
mutation happens outside the session package), so the permanent record
of "this image was touched, and how many times" survives even though the
byte-level undo buffer doesn't. Verified with a real multi-step sequence
against the live server: redact, highlight, undo (removes highlight),
undo again (removes redaction too — confirmed via independent pixel
read), redo, redo (restores both, in the correct order, confirmed
pixel-by-pixel each time), reset (reverts to the pre-any-edit original,
confirmed), and confirmed undo after reset correctly reports "nothing to
undo" (400) rather than silently no-op'ing.

**Click-highlight refinement:** per the same testing pass, a click that
itself brings a different app to the front (switching window focus) no
longer gets the highlight ring — the click is still captured (the window
switch is real context for a story), but ringing a point that's "wherever
the background window happened to be" rather than a deliberate target
was judged more misleading than useful. Implemented by tracking the
frontmost app as of the previous click in the daemon's closure state,
seeded from the actual frontmost app at daemon startup (not left empty,
which would have wrongly flagged the session's very first click as a
focus change). Logic reviewed, not click-verified end to end — same
limitation noted for decision #15, synthesizing a real OS-level click
isn't possible in this environment.

## 18. Generic, numbered, editable annotations — click-highlight and manual highlight unified

Decided 2026-08-03, directly requested by the user: numbered badges on
highlights, user-editable numbers so a caption can say "1: this, 2:
that," and — explicitly — "even the click highlight should be editable
later, keep generic." That last part is the architecturally significant
one: it means the automatic click-ring and manually-drawn highlights
can't stay two different mechanisms (one baked permanently into pixels at
capture time, one baked in by the editor) — they have to become the same
kind of object.

**The shift:** highlights are no longer baked into a screenshot's pixels
at all. `Step` gains `Annotations []Annotation` — `{ID, Number, X, Y, W,
H}` in image pixel coordinates. `ID` is assigned once and never changes
(how the API addresses a specific annotation); `Number` is the free-form,
user-editable display badge (defaults to creation order). The screenshot
file itself stays clean; annotations render as a live overlay in the
editor and are only flattened onto a *copy* of the image at export time
(`internal/export/image.go`'s `stepImageBytes`/`stepImagePath` — the
on-disk original is never touched, mirroring how `DisplayOrder`/`Hidden`
already work as a presentation-time layer over immutable `Steps`).
`session.AddAnnotation`/`UpdateAnnotation`/`DeleteAnnotation` follow the
same reload-mutate-log-save pattern as every other edit function, each
logged to `EditLog`.

**Redaction stays exactly as it was — pixel-destructive, immediate, not
part of this list.** The user's ask was specifically about highlights;
redaction's whole point is that the covered content is actually gone, so
making it a movable/undoable "annotation object" would work against the
reason it exists. `EditImage`'s `action` parameter is narrowed to
`"redact"` only now that highlights have their own path.

**Click daemon:** `engine.RunClickDaemon` no longer calls a
pixel-mutating `Highlight` function (removed from `internal/capture`
entirely, along with its `drawRing` helper — both fully superseded, not
left around unused). It saves a clean screenshot and, when a highlight is
warranted, computes the click's bounding box in image pixel space
(`capture.PixelSize` — decodes just the PNG header, not the whole image)
and calls `session.AddAnnotation` — the exact same function the editor's
"Draw Highlight" button calls. One mechanism, two entry points, which is
what "keep generic" meant.

**Numbered badge rendering** (`capture.RenderAnnotations`, used by all
three exporters via the new `stepImageBytes`/`stepImagePath`) draws a
rectangle outline plus a filled circular badge with the number drawn in
it, using `golang.org/x/image/font/basicfont` — a built-in bitmap font,
no font file to bundle, consistent with the PDF exporter's own
no-external-assets stance (decision #12).

**Editor UI:** existing annotations are always live and interactive —
drag the body to move, drag a corner handle to resize, edit the number
input directly, click × to delete — each committing immediately via the
API (no Apply step, since editing an annotation that already exists
isn't destructive the way redaction is). Creating a *new* highlight
reuses the same draw → adjust → Apply flow redaction already had; Apply
branches to `/api/annotation` (create) instead of `/api/edit-image`
depending on which was armed.

**Verified in layers, each with a real result to check against:**

1. Unit tests (`internal/capture/annotate_test.go`,
   `TestRenderAnnotationsDrawsBorderAndBadgeLeavesRestUntouched`) —
   confirmed the border renders in the highlight color, the interior
   away from the badge stays untouched, and the badge circle itself
   (sampled off the exact center, which can legitimately land on the
   digit glyph instead of the badge background — caught by the first
   version of this test) is the correct color. A second test confirmed
   `RenderAnnotations` with an empty list returns the input completely
   unchanged (no wasted decode/re-encode round-trip).
2. A standalone visual check (three annotations, including a two-digit
   number "12") confirmed the badges render legibly and correctly
   centered before wiring this into the exporters at all.
3. Full interactive verification against a live editor, using precise
   synthetic `MouseEvent`s (the same method that caught the coordinate
   bug in decision #17): created an annotation via the real draw →
   adjust → Apply UI flow, confirmed it persisted with the right ID and
   an incrementing `Number`; renumbered it (confirmed `ID` stayed fixed
   while `Number` changed — the whole point of separating the two);
   moved it (confirmed position changed, size didn't); resized it via a
   corner handle (confirmed the opposite corner stayed anchored); deleted
   it (confirmed it's gone from the step and logged). Created two fresh
   annotations and exported to Markdown/PDF/Slides — confirmed the
   Markdown file links to a freshly-generated `*.annotated.png`, not the
   original `.png` (timestamps differ, proving the original was never
   touched), and visually confirmed both numbered badges render
   correctly and distinctly in the flattened output. Re-tested redaction
   afterward as a regression check, since its pixel-drawing internals
   were refactored to share code with the new annotation renderer —
   still produces genuinely unrecoverable black pixels, confirmed the
   same way as decision #16 (independent read of the raw bytes).

**Not verified: the automatic click-daemon path creating an annotation
from a real physical click**, for the same reason noted in decisions #15
and #17 — synthesizing a real OS-level click isn't possible in this
environment. `AddAnnotation` itself (the function the click daemon calls)
*is* thoroughly verified above, exercised via the identical code path the
editor's "Draw Highlight" button uses; what's specifically unconfirmed is
the click daemon's own coordinate math (click position → window-relative
point → pixel-space bounding box) ahead of that call.

## 19. Unified annotation/redaction history, and blur as a second redaction style

Decided 2026-08-03. Prompted directly by real usage of decision #18's
system: adding a highlight, hitting Apply, and having it render as an
invisible box — reproducible every time the browser had the step's
screenshot cached (i.e. every time after the first page load). Root
cause: `buildImageEditor`'s `placeAnnotations()` ran synchronously
whenever `img.complete` was already true (the cached-image case),
*before* `imgFrame` had been appended into the document — so
`img.clientWidth`/`clientHeight`, which annotation placement divided by,
read as 0 for a still-detached element. The annotation was written to
disk correctly the whole time; only its on-screen box was zeroed.

A quick patch (defer placement a frame via `requestAnimationFrame` when
the image is already complete) was written, verified to fix the repro,
and then explicitly discarded at the user's direction in favor of a
foundational rebuild: *"think properly, and create a proper highlight
system... build a core foundation of highlight system first — history,
undo redo, moveables, annotations, stack of highlights and move the
order... live annotations update... build a core foundation strong and
error prone[sic, "-free"]. and then go to UI layer."* That's a fair
read of what shipped in #18: redaction had its own undo/redo/reset
(`internal/editor/history.go`'s `imageHistory`, image-bytes-only),
annotations had none, and the two could never be undone as a single
coherent sequence even though a user editing a screenshot experiences
them as one activity. Patching the placement bug alone would have left
that structural gap in place.

**What changed, bottom-up:**

1. **`internal/session`**: `AddAnnotation` now returns the created
   `Annotation` (was `(*Session, error)`, now `(Annotation, *Session,
   error)`) so the editor can hand it straight back to the frontend
   without a follow-up fetch. Two new low-level primitives:
   `SetAnnotations` (overwrite a step's whole annotation list —
   deliberately *unlogged*, since it's a restore operation the caller
   logs once as a semantic action, not a fresh edit) and
   `ReorderAnnotations` (validates the given ID list is exactly a
   permutation of the step's existing annotations, then replaces the
   slice in that order — array order *is* z-order/paint order, by
   construction, so there's no separate z-index field to ever drift out
   of sync with it).

2. **`internal/editor/history.go`**: replaced the old `imageHistory`
   (undo/redo stacks of `[]byte`, one per step) with `editHistory` —
   undo/redo/baseline stacks of `editSnapshot{Image []byte, Annotations
   []session.Annotation}`, one unit per step per point in time. Every
   mutating endpoint pushes the *before* state onto `undo` right before
   applying its change; `undoStep`/`redoStep` pop-and-swap between the
   two stacks the usual way. `resetStep` returns the first snapshot ever
   pushed for that step (the true pre-edit baseline) but — deliberately
   — pushes the current state onto `undo` first rather than clearing
   history outright, so Reset is itself one more undoable edit instead
   of a dead end. This is genuinely one history, not two coordinated
   ones: an Undo after a redact-then-highlight sequence removes the
   highlight first, then the redaction, in the exact order they
   happened, regardless of which kind of edit either one was.

3. **`internal/editor/editor.go`**: every mutating handler
   (`/api/edit-image`, `/api/annotation`, `/api/annotation/update`,
   `/api/annotation/delete`, and the new `/api/annotation/reorder`)
   snapshots the step via `stepSnapshot` immediately before calling the
   matching `session.*` function, then pushes that snapshot onto
   `editHistory`. `/api/image-undo`/`-redo`/`-reset` were replaced with
   unified `/api/undo`/`/api/redo`/`/api/reset` (body: `{index}` only —
   no notion of "image undo" vs "annotation undo" anymore, just "undo").
   `applySnapshot` restores both halves of a snapshot: image bytes
   written straight to the step's file (if the snapshot has any — a
   step's annotations can be undone independently of whether its image
   ever changed) and `session.SetAnnotations` for the annotation list.
   Every mutating endpoint now responds with the affected
   `session.Step` (or, for creation, the created `Annotation`) as JSON,
   instead of `204 No Content` — needed for the frontend change below.

4. **`internal/editor/html.go`**: the actual placement bug is fixed by
   removing its precondition, not by sequencing around it. Annotation
   position/size are now percentages of the image's *natural* (decoded)
   size — `ann.x / img.naturalWidth * 100 + '%'` — rather than pixels
   scaled by `img.clientWidth`. `naturalWidth`/`naturalHeight` are set as
   soon as the browser has decoded the image data, independent of
   whether that `<img>` is attached to the document or has been laid out
   at all; `clientWidth`/`clientHeight` need both. So placement can no
   longer race attachment, for any image, cached or not — this isn't a
   narrower version of the old bug, it's a different mechanism that
   doesn't have the bug's precondition. (One supporting fix `.img-frame`
   needed: `display: inline-block` instead of the default block, so the
   frame's box always shrink-wraps to the image's actual rendered size —
   otherwise percentages-of-frame and percentages-of-image silently
   diverge whenever a screenshot renders narrower than the page column.)

   Separately, and because the underlying data model now supports it
   cleanly: annotation create/update/delete/reorder and redact/undo/redo/
   reset no longer trigger a full `/api/session` refetch and page
   re-render. A small `stepEditors` registry keyed by step index holds
   live handles (`setStepData`, `addAnnotationLocal`, `bustImageCache`)
   into each already-rendered step's DOM, so a successful edit patches
   local state and re-renders just that step's annotation overlay (or,
   for image bytes, appends a cache-busting query param to force a
   refetch of the one image that actually changed — the filename itself
   never changes on redact/undo/redo/reset). Reorder/hide/caption still
   go through the original full-session `load()` path, since they affect
   step ordering across the whole page; that remains safe now regardless
   of image-cache state, for the same reason the placement bug itself is
   gone. Each annotation also gained `‹`/`›` stack controls (send
   backward / bring forward) wired to `/api/annotation/reorder`.

**Second, smaller change bundled into this pass**, from the same
message: redaction now supports two styles, `capture.RedactStyleSolid`
(the existing opaque-black fill — the only style safe for actual secrets,
genuinely unrecoverable) and `capture.RedactStyleBlur` (a new 3-pass box
blur, `internal/capture/annotate.go`'s `blurRect` — three passes of box
blur is a standard, cheap approximation of a Gaussian blur). Blur is
explicitly *not* claimed to be cryptographically irreversible — it exists
for obscuring something (a face in a demo, a distracting background
element) where pixel-level unrecoverability was never the requirement,
not as an alternative for secrets. `session.EditImage`'s signature
changed from an `action` parameter (redact was already its only valid
value) to `style`, defaulting to solid when empty. The editor toolbar
gained a Solid/Blur toggle next to Draw Redaction.

**Explicitly deferred** (from the same follow-up message, to keep this
pass foundation-focused rather than growing indefinitely): a dedicated
editor-modal UI, freehand/hand-drawn annotations, image cropping, and a
color picker for highlights. None of these are blocked by anything
built here — front/back stacking, undo/redo/reset, and live updates are
all generic over "what kind of annotation," not hardcoded to rectangles.

**Verified in layers:**

1. Unit tests: `TestRedactBlurObscuresRegionButLeavesRestUntouched`
   (new) — a hard two-color split image, since the existing solid-fill
   test's flat single-color fixture can't demonstrate blur at all
   (averaging a uniform color with itself is a no-op); confirms the
   boundary pixel inside the blurred rect is neither pure input color
   (i.e. blending actually happened) and that pixels just outside the
   rect on both sides are completely untouched (checks the box blur's
   read window doesn't leak past the intended region). Existing solid
   redact and annotation-rendering tests updated for the `Redact`
   signature change and re-passing.
2. `go build ./... && go vet ./... && gofmt -l .` clean throughout.
3. Full interactive re-verification against a live editor on a real
   session (`test3`, real captured screenshots), using the same precise
   synthetic-`MouseEvent` method as decisions #17/#18:
   - **The actual regression repro**: created a highlight, confirmed it
     rendered immediately (250×180px, matching the drawn rectangle,
     confirmed via `getBoundingClientRect` — not just visually). Then
     did a real full-page navigation (not the app's own `load()`) back
     to the same URL and confirmed `img.complete === true` immediately
     on the fresh load — the exact cached-image precondition that used
     to zero out placement — and confirmed the annotation still rendered
     at its correct size and position. This is the one check that
     actually matters for this decision; everything else is regression
     coverage.
   - Added a second annotation live (no reload) — both visible
     simultaneously with distinct badges.
   - Reordered via the new `‹`/`›` controls; confirmed via
     `GET /api/session` that stored array order actually swapped
     (`[1, 2]` → `[2, 1]`).
   - Renumbered a badge directly (`5`); confirmed via the API that `id`
     stayed fixed while `number` changed, and that it persisted.
   - Applied a **blur** redaction; confirmed `EditLog` recorded
     `"blur rect(...)"`; confirmed via `md5` of the on-disk PNG that the
     bytes actually changed and that the served `<img src>` carried a
     fresh cache-busting `?t=` query param.
   - **Undo**: confirmed the image reverted to its pre-blur checksum
     *and* both annotations (including the renumber and reorder from
     earlier) reappeared correctly — proof the history is genuinely
     unified, not "image undo happened to run right after annotation
     state was already there."
   - **Redo**: confirmed the blur checksum came back exactly.
   - **Reset**: confirmed the image returned to its original
     pre-session checksum and the annotation list emptied entirely.
   - **Undo after Reset**: confirmed the full pre-reset state (blurred
     image, both annotations) came back — proof Reset is itself
     undoable, per the `resetStep` design above, not a dead end.
   - **Delete**: removed one annotation live; confirmed only the other
     remained, both on screen and via the API.

**Not verified**: the automatic click-daemon's annotation creation from
a real physical click, for the same standing reason as decisions #15,
#17, and #18 — this environment can't synthesize a real OS-level click.
`AddAnnotation` itself is exercised thoroughly above via the identical
path the editor's "Draw Highlight" button uses.

## 20. Annotation toolbar redesign, and editable step title/window info

Decided 2026-08-03, same day as #19, from direct feedback on it: a
screenshot showing decision #19's per-annotation controls — a red numbered
badge, a "×" delete button, and "‹"/"›" stack buttons — all crammed into
the same top-right corner of a small annotation, overlapping each other
illegibly. Called out explicitly as "not proper," with a specific
reference model: *"have a toolbar kind thing similar to font styles in
word above the highlight not as a badge. once saved than we can show on
click or hover show toolbar."*

**Annotation UI** (`internal/editor/html.go`): the number badge is now a
small static `<span>` (not an `<input>`) shown at all times as the only
default-visible chrome — matching "once saved" from the request, an
annotation at rest just looks like a highlighted, numbered rectangle. All
interactive controls (renumber, send backward/bring forward, delete) moved
into a `.annotation-toolbar` — a floating pill positioned above the
annotation (or below, when the annotation sits near the top of the image
and a toolbar above it would clip off the frame — `toolbar-below`,
computed from the annotation's vertical position as a fraction of image
height), shown via a pure-CSS `:hover` rule so revealing it costs no JS.
Resize handles got the same treatment (`.annotation .resize-handle`
hidden by default, shown on hover/pin) — matching "once saved" was really
asking for a general at-rest-vs-editing distinction, not something
specific to the toolbar alone.

"Or click," per the request, is handled by a `pinned` CSS class: a plain
click on an annotation's body (tracked the same way setupDrawing already
distinguishes a drag from a click — a `moved` flag set once the pointer
travels more than a few pixels) toggles `.pinned`, which the same
`:hover`-style CSS rule also matches, so the toolbar stays open without
requiring the mouse to stay put — useful once the user is actually about
to type into the renumber field. A single `mousedown` listener registered
once at script scope (not per-annotation) un-pins any annotation the
click landed outside of; because it's stateless (queries
`.annotation.pinned` fresh every time it fires) it needs no cleanup
across `renderAnnotations()`'s full teardown-and-rebuild cycles, unlike a
per-annotation listener would.

One correctness fix fell out of tracking `moved` for the pin/click
distinction: previously, ending a plain click on an annotation still
called `commit()` and POSTed to `/api/annotation/update` even though
nothing had actually changed (position math on a 0px drag still produces
"new" coordinates equal to the old ones) — every click added a spurious,
identical entry to the undo history. Now a plain click skips the network
call entirely and only toggles `.pinned`; only an actual drag/resize
calls `commitPosition()`.

**Metadata editability**, from the same message (*"step change thing
works but step 1 title is not editable. Everything should be editable
step claude etc got it customization on full"*): the step heading (was
hardcoded `'Step ' + step.index`) and the captured window app/title line
(was static text) are now click-to-edit, the same interaction pattern as
the annotation toolbar's reveal-on-interact philosophy — plain text at
rest, an `<input>` in place of it on click, committing on blur/Enter,
Escape to cancel without saving.

- `session.Step` gained a `Title` field (empty means "no override, show
  the default 'Step N' heading" — typing the default text back into the
  editor, or clearing it, stores an empty string rather than the literal
  default, so "Step 1" only ever means "unset" on disk, never a stored
  string a future rename to "Step 2" would leave stale).
- New `session.EditTitle` and `session.EditWindowInfo`, following
  `EditNote`'s existing shape exactly (reload, mutate, mask free-text
  fields the same way capture-time text already is, log old/new to
  EditLog, save) — `EditWindowInfo` masks `WindowTitle` for the same
  reason `AddStep` already does at capture time: a corrected window
  title is still user-typed free text, just as capable of carrying a
  pasted secret as an originally-captured one.
- New `/api/title` and `/api/window` editor endpoints, and
  `internal/export`'s three exporters now call a shared `stepHeading()`
  helper (`step.Title` if set, else `"Step " + step.Index`) instead of
  hardcoding the numbered heading — so a renamed step's title actually
  shows up in the exported Markdown/PDF/Slides, not just the editor.

**Verified:**

1. `go build/vet/gofmt` clean, existing test suite still passing (no
   behavior under test changed).
2. Live re-verification in the same editor session used for decision
   #19's checks: confirmed the annotation renders as just a highlighted
   rectangle plus a small numbered badge with no overlapping controls at
   rest; hovering revealed the toolbar cleanly positioned above it;
   dispatching a real no-movement mousedown+mouseup on the annotation
   body toggled `pinned` to `true` (checked via `classList.contains`,
   not just visually) with the toolbar staying visible after moving the
   mouse away; a click elsewhere on the page un-pinned it (`pinned` back
   to `false`).
3. Clicked the step heading, confirmed it became a pre-filled, selected
   text input; retyped it and tabbed away; confirmed via `GET
   /api/session` the new title persisted. Clicked the window line,
   confirmed both app and title fields became independent inputs;
   edited the app field and tabbed to the title field — confirmed via
   the API that nothing had saved yet (proving the "still within the
   pair" focus check works, not just committing on every blur); edited
   the title field and clicked elsewhere on the page — confirmed both
   fields committed together in one call. Ran `loomless export --format
   md` and confirmed the rendered Markdown used the custom title as its
   `##` heading and the edited app/title as the window line, while an
   untouched step in the same session still fell back to its default
   "Step N" heading correctly.

## 21. Step headings number by display position; click-highlight ring off by default

Decided 2026-08-03. Two small, unrelated fixes from the same feedback
round as #20, bundled here since both are display/default changes with
no shared code.

**Step numbering follows display position, not capture index.** Reported
directly: reordering steps in the editor didn't renumber the "Step N"
headings — a step moved to the top of the list kept showing whatever
number it originally captured at. The default heading (used whenever
`Step.Title` is empty) is now computed from the step's 1-indexed position
in the *current* order, not `Step.Index` — `Index` stays exactly what
decision #4 established (a stable, never-reassigned capture-time
reference used for image/output filenames, EditLog targeting, and the
annotation API), this only changes what number gets displayed when no
custom title overrides it. Applied in both `internal/export`'s
`stepHeading(step, position)` (position = the step's index within
`VisibleSteps()`, +1) and the editor's `buildStepTitle`/`buildImageEditor`
(position = index within the visible steps of the current display order —
hidden steps don't consume a number, matching that they're excluded from
`VisibleSteps()` and therefore don't appear in any export at all).
Verified: reordered two steps in a live editor session, confirmed the
headings swapped to match new position; exported to Markdown and
confirmed the `##` headings matched the editor's numbers exactly, in the
new order.

**Click-highlight ring is now off by default.** Decision #15 added the
on/off toggle but defaulted it on ("more often useful than not"); direct
feedback reversed that assessment ("not useful at all"). Kept as an
explicit opt-in rather than removed outright, per "keep it optional if
possible" — `loomless record start <name> --highlight` (was
`--no-highlight` to opt *out*; now `--highlight` to opt *in*, same flag
name repurposed for the opposite polarity since the meaning "the flag
that's present when you want the ring" stays true either way) and the MCP
`start_recording` tool's `highlight_clicks: true` (renamed from
`no_highlight`, since a double-negative default is worse API ergonomics
than a plain opt-in bool). The actual ring-drawing logic
(`AddAnnotation` from the click daemon) didn't change at all — only the
default value threaded through `StartRecording`/`startClickDaemon` at
every call site. Verified: `go build/vet/gofmt` clean; confirmed by
reading through every call site that used to hardcode `true` (CLI
`recordStart`, the `__click-daemon` hidden subcommand, MCP's tool
handler) — all three now default `false` and only flip to `true` when the
respective flag/field is explicitly set. Not independently re-verified
via a live click (same standing limitation as decisions #15/#17/#18/#19 —
this environment can't synthesize a real OS-level click), but the code
path a real click goes through is unchanged from decision #15's own
verification, only the default feeding into it changed.

## 22. Captions support Markdown, rendered (not just escaped) in every export

Decided 2026-08-03, direct request ("captions should support md format").
Markdown export needed no code change — `Step.Note` was already written
into the `.md` file as raw, unescaped text, so any Markdown syntax a user
typed was already going to render correctly the moment someone opened
that file in a Markdown viewer. PDF and Slides previously ran `Note`
through plain-text escaping (`safe()`/`html.EscapeString`), so `**bold**`
would show up as the literal four asterisks — this decision is really
about those two.

**Dependency**: added `github.com/yuin/goldmark` (pure Go, no cgo,
CommonMark-compliant, BSD-licensed, the de facto standard Go Markdown
library) rather than hand-rolling a regex-based inline parser. A caption
is free-typed human text, and free-typed text reliably finds every edge
case a hand-rolled parser gets wrong — unmatched `*`, nested emphasis,
literal `<`/`>`, escaped characters — so this is exactly the situation a
well-tested, widely-used parser earns its dependency weight, consistent
with how `go-pdf/fpdf` and `golang.org/x/image` were already accepted
for the same reason (a correctly-solved problem beats a partially-solved
one written from scratch).

**Slides** (`internal/export/note.go`'s `noteHTML`): `goldmark.Convert`
straight to HTML, dropped into a `<div class="note">` (was `<p
class="note">` around escaped text). goldmark's default renderer
*escapes* raw HTML in the source rather than passing it through live (no
`html.WithUnsafe()`) — a caption containing something that looks like
`<script>` stays inert text, same safety property `html.EscapeString`
had, Markdown support just isn't traded away to get it. `.slide`'s
`text-align: center` doesn't suit multi-paragraph/list caption content,
so `.note` now sets `text-align: left` — the one visible layout change
beyond the caption itself, and a direct consequence of captions now
being able to contain structured content instead of a single line.

**PDF** (`internal/export/note.go`'s `writeNotePDF`) is the more involved
half, and deliberately does *not* go through fpdf's own `HTMLBasicType`
HTML-string writer: that writer re-tokenizes its input with a regex that
has no entity-decoding step, so a caption containing a literal `<` or `>`
(plausible in a dev tool — `"if x < 5"`) would either show up as literal
`&lt;` or, worse, get misread as the start of an unclosed tag and
silently swallow everything up to the next `>`. Instead, `writeNotePDF`
walks goldmark's parsed AST directly and calls fpdf's text primitives
(`Write`/`WriteLinkString`/`SetFont`/`SetTextColor`) itself — no HTML
string is ever constructed or re-parsed, so there's nothing for a stray
angle bracket to be misinterpreted as. Bold/italic map to `SetFont`
style flags; inline code switches the *font family* to Courier (a core
PDF font already used by the terminal-step renderer, not an embedded
asset) for a real monospace look rather than approximating it with
italics; links get `WriteLinkString` (a genuine clickable PDF link
annotation, confirmed via `page.get_links()` in a real generated PDF,
not just a plain glyph) plus a legible blue (`RGB(37,99,235)`) so a link
actually looks like one — an initial pass shipped the link as
functionally clickable but visually indistinguishable from body text,
caught by looking at the rendered output rather than trusting a green
test run. Font and text color are captured before any caption rendering
starts and explicitly restored afterward (and after every individual
link), since every caller renders a caption and then keeps going with
plain text right after it — nothing about a caption should leak into
whatever comes next.

**Verified:**

1. Unit tests (`internal/export/note_test.go`):
   `TestNoteHTMLRendersBasicMarkdown` (bold/italic/code/link/list all
   produce the expected HTML tags), `TestNoteHTMLEscapesRawHTML` (a
   caption containing `<script>` never becomes a live tag in the
   output), `TestWriteNotePDFDoesNotErrorOnEdgeCases` (a sweep of
   plausible captions — empty, plain, nested emphasis, an unmatched lone
   `*`, literal `<`/`>` both close together and far apart, a link with
   query-string special characters, lists, multiple paragraphs, Unicode,
   a long wrapping caption — none may leave the PDF in an error state or
   fail to produce output), and `TestWriteNotePDFRestoresFont` (asserts
   both the font size *and*, via `GetStringWidth` compared against a
   never-touched reference instance — since fpdf exposes no direct
   family/style getter, and bold/Courier glyphs really do measure
   differently — that the family and style are back to
   Helvetica/regular, plus that text color is restored to whatever it
   was before rendering started).
2. `go build/vet/gofmt` clean, full suite passing.
3. Generated real PDF and Slides output from a live session with a
   caption exercising every feature at once (bold, inline code, a link,
   a bullet list, and the literal-angle-bracket edge case in the same
   caption) and inspected the actual rendered result, not just that the
   code ran without error: the Slides HTML in a real browser showed
   correctly styled bold/code/link/list text with the angle-bracket text
   rendered literally (not broken, not swallowing content); the PDF,
   rasterized page-by-page with PyMuPDF and read as an image, showed the
   same — bold text, a genuine Courier-font code span, and (after the
   color fix above) a visibly blue link — and `page.get_links()` on the
   real PDF confirmed a working `https://` URI annotation positioned
   exactly under the link text, not just plain glyphs that merely looked
   like a link.

## 23. A real TUI (`loomless tui`) for session management

Decided 2026-08-03, direct request: *"make cli a proper TUI kind app,
where it can show list session shortcuts to edit etc fully functional
TUI app."* Decision #14 chose a browser-based editor over a TUI, but
that decision was specifically about the *visual editing* work
(redaction, annotations) needing a canvas a terminal can't give you —
it never claimed a TUI has no place in this project. Session
*management* (which sessions exist, how big are they, jump into
editing/exporting one) is exactly the kind of fast, keyboard-driven
overview a TUI is good at, so `loomless tui` is additive: a third way to
use Loomless alongside the scriptable CLI/MCP surface and the existing
`loomless edit` web UI, not a replacement for either.

**Stack**: `github.com/charmbracelet/bubbletea` + `bubbles/list` +
`lipgloss` — the de facto standard Go TUI toolkit (pure Go, no cgo,
actively maintained, used by a huge fraction of the Go TUI ecosystem),
chosen over hand-rolling raw ANSI/terminal handling for the same reason
`goldmark`/`fpdf` were chosen over hand-rolled alternatives elsewhere in
this project: a well-solved problem beats a partially-solved one.

**New primitive**: `session.List()` — reads every directory under
`~/.loomless/sessions`, loads each `session.json`, sorts by `StartedAt`
descending. A session directory that fails to load (corrupted or
mid-write `session.json`) is skipped rather than failing the whole
listing — one bad session shouldn't hide every other one.

**Scope of this first pass**: list sessions (name, step count, started
time, a "● recording" marker for any session whose `StoppedAt` is still
nil), open the selected session's existing local web editor (`e`/enter —
reuses `internal/editor.NewHandler` exactly as `loomless edit` does,
listening on `127.0.0.1:0` and opening the system browser), export to
any format (`m`/`p`/`s` — reuses `internal/engine`'s existing
`ExportMarkdown`/`ExportPDF`/`ExportSlides`, the same functions the
plain CLI calls), and refresh (`r`, since sessions can be created by a
concurrently-running `loomless record start` or MCP client). Deliberately
**not** in this pass: starting/stopping a recording from the TUI —
`engine.StopRecording()` only operates on the single process-wide
"current" session pointer `loomless record start` maintains, not an
arbitrary session picked from a list, so wiring that up correctly needs
new plumbing rather than just a new keybinding; and session deletion,
which wasn't asked for and deserves its own confirm-before-destroy
design rather than being bolted on. Both are natural follow-ups, not
blocked by anything built here.

**Concurrency design, stated explicitly because it's the part most
likely to be gotten wrong in a TUI**: every state mutation
(`m.editors[name] = ...`, `m.status = ...`) happens synchronously on
bubbletea's single Update-loop goroutine, never inside an async
`tea.Cmd` closure. `editor.NewHandler`, `net.Listen`, and the export
functions are all fast, local, non-blocking calls (the export functions
do the same synchronous disk I/O the plain `loomless export` CLI already
does with no progress indicator), so there was no real need to reach for
`tea.Cmd`'s async-command-then-Msg pattern at all for this pass — doing
so anyway would have meant either accepting a genuine data race on
`m.editors` (a Go map, mutated from a goroutine while Update might
concurrently read it) or building out a Msg-passing layer to avoid one,
for no actual benefit here. The one genuinely-async piece — the editor
HTTP server's blocking `Serve` loop — gets its own goroutine exactly
once per session, tracked in `m.editors` so a second "open editor" on
the same session reuses the running server instead of binding a second
port; all tracked servers are closed on quit (`ctrl+c`/`q`) before
`tea.Quit`.

`openBrowser` (previously a plain top-level func in `cmd/loomless/main.go`,
shared with the existing `loomless edit` command) became a package-level
`var` purely so tests could swap in a no-op recorder — otherwise every
test run of the "open editor" action would pop open a real browser
window.

**Verified:**

1. Unit tests (`cmd/loomless/tui_test.go`): session listing count and
   order, the recording-status marker on `Title()` for a stopped vs.
   unstopped session, that `export("md")` actually produces the file on
   disk (not just a non-error return), that `refresh()` picks up a
   session created after the model was constructed, and — the one most
   worth having — that opening the editor twice on the same session
   reuses the exact same `*http.Server` and only calls `openBrowser`
   with the (identical) URL both times rather than binding a second
   port.
2. `go build/vet/gofmt` clean, full suite passing.
3. **Full interactive verification against the real compiled binary**,
   driven through an actual pseudo-terminal (Python's `pty` module,
   explicit `TIOCSWINSZ` to give it a real size, and manually answering
   the OSC-11/cursor-position queries bubbletea's terminal-capability
   detection blocks on when nothing responds — without that, the
   program hangs indefinitely waiting for a reply no raw pty ever
   sends). Screen contents were reconstructed with `pyte` (a real VT100
   emulator) rather than grepping raw ANSI bytes, since bubbletea
   renders via incremental line diffs, not full-screen redraws.
   Confirmed, against real session data on disk: the session list
   renders with correct names/step-counts/timestamps in the right
   (most-recent-first) order; the down-arrow moves the selection cursor;
   pressing `m` actually exports and the status line updates to show the
   real output path (confirmed the file landed on disk, not just that
   the status text changed); pressing `e` starts a real server — an
   actual HTTP request to `<url>/api/session` got a `200` back — and
   after `q`, that same URL became unreachable (server closed) while the
   process itself exited cleanly (exit status 0, confirmed via
   `waitpid`, not just "still running" checks — which is meaningless for
   an unreaped child, a mistake the first pass at this test made and
   quietly reported a false hang until corrected).

## 24. TUI: stop editor, create/rename/stop-recording sessions, polished footer

Decided 2026-08-04, direct feedback on decision #23's first pass: *"no
way to stop the running server, no way to create a new session, no way
to edit the session name"* plus a request to *"add a bottom status bar
… show the status shortcuts running status etc … polish TUI."* All four
close real gaps in the initial session-management dashboard rather than
adding new surface area — the first pass could show sessions and jump
into them, but nothing to close what it opened, start what it managed,
or fix a session's own identity.

**Two backend correctness fixes, added first because everything else
depends on them being right:**

1. `session.Session.Stop` previously cleared the global "current session"
   pointer *unconditionally*. That was harmless while the only caller
   (`engine.StopRecording`) always resolved its target *through*
   `session.Current()` — target and current were the same session by
   construction. Once a session can be stopped *by name* (next point),
   that guarantee is gone: stopping session A while B is actually current
   would have silently clobbered B's pointer, breaking `loomless record
   stop`/`loomless step` for a session nobody asked to touch. Fixed to only
   clear when `Current().Name` matches the session being stopped.
2. New `engine.StopRecordingNamed(name)` — `StopRecording` restricted to
   the single global "current" session was never going to work for a TUI
   list, where the selected item and "whatever was most recently started"
   are frequently different sessions. Refactored `StopRecording` itself
   to share the same underlying by-name logic, rather than duplicating
   the stop-daemon/reload/mark-stopped sequence.

**New `session.Rename(oldName, newName)`.** Distinct from decision #20's
`EditTitle` (which only overrides one step's display heading) — this
changes the session's actual identity: the on-disk directory, `Session
.Name` inside `session.json`, and the "current" pointer if it was
pointing at the renamed session (checked *before* the directory move, not
after — `Current()` can't resolve a directory that's already gone,
caught by a test that initially failed for exactly this reason).
Deliberately rejects renaming a session that's still recording
(`StoppedAt == nil`): `RunClickDaemon` is a relaunched, detached process
that was started with — and holds in memory for its entire lifetime — the
name it was given, with no channel to learn a rename happened; every
subsequent click would try to load a directory that no longer exists at
that path. Rather than leave that as a silent landmine, it's a rejected
operation with a message telling the user to stop the recording first.
Renaming to the session's own current name is treated as a no-op success
(even while still recording, since nothing structurally changes), not an
error.

**TUI wiring** (`cmd/loomless/tui.go`): new keys `x` (stop the selected
session's running editor server — the literal gap reported), `S` (stop
recording, via `StopRecordingNamed`), `n` (new session), `c` (rename). `n`
and `c` share one `bubbles/textinput`-based prompt (`inputPurpose`
distinguishes the two — new session starts blank, rename starts
pre-filled with the current name and cursor at the end) with its own
small mode: while a prompt is open, keystrokes go to the text field
instead of being read as the list's global shortcuts, `Enter` submits,
`Esc` cancels without side effects.

**Footer, rebuilt as two always-present lines** rather than the one
sometimes-shown-sometimes-not line from decision #23 (list height is
reserved for exactly two footer lines now, so it never has to resize
between modes): a status/prompt line (defaults, when nothing more
specific just happened, to live counts — `N session(s) · M editor(s)
running · K recording` — satisfying "show the status … running status"
directly) and a persistent, contextual shortcut legend (the full action
set in list mode, just `enter confirm · esc cancel` while a prompt is
open).

**`engine.StartRecording` became a package-level `var startRecording`**
in `cmd/loomless/tui.go` (same reasoning as `openBrowser` in decision #23):
it's the one action that forks a real background OS process. A test
calling the real thing would relaunch `os.Executable()` — which, inside
`go test`, *is the compiled test binary*, not the `loomless` CLI — with
args like `__click-daemon`, an unpredictable thing to do in a unit test
(confirmed by first writing the test against the real function, watching
it actually spawn and querying `ps` for what showed up, then switching to
the injectable var once that risk was visible directly rather than
theoretically).

**Verified:**

1. Unit tests: `session_test.go` (`Stop` only clears a matching current
   pointer, not another session's — with an explicit two-session test
   proving the negative case; `Rename` moves the directory and updates
   `Name`, rejects a still-recording session, rejects an existing target
   name, treats same-name as a no-op, and follows the "current" pointer
   — this last case is what caught the check-after-move ordering bug
   above), `engine_test.go` (`StopRecordingNamed` stops session A without
   disturbing session B's "current" status, and confirms `StopRecording`
   still works normally afterward), `cmd/loomless/tui_test.go` (stop-editor
   actually closes the server — confirmed via a real HTTP request before
   and after, not just the map entry disappearing; the full new-session
   prompt-to-disk flow via the injected `startRecording` fake, including
   the `watching=false` warning path; the rename flow confirming the
   prompt pre-fills correctly and the rename actually lands on disk;
   stop-recording marking `StoppedAt` and erroring cleanly on an
   already-stopped session).
2. `go build/vet/gofmt` clean, full suite passing.
3. Full interactive re-verification against the real compiled binary
   through the same pty+pyte harness as decision #23, this time driving
   every new key: `e` then `x` (opened, confirmed a real `200` from the
   server, stopped it, confirmed the same URL now unreachable); `n` then
   typing a name then `Enter` (the new session appeared at the top of the
   list — sort is by `StartedAt` descending — marked `● recording`, footer
   showed `1 recording`); `S` on it (marker disappeared, footer recount
   updated); `c` (prompt pre-filled with the exact current name,
   confirmed character-for-character, not just non-empty), `ctrl+u` to
   clear (bubbles/textinput's own binding) then retyping and `Enter`
   (session renamed on disk, confirmed via a fresh `session.Load` on the
   new name and the absence of the old one). One real bug surfaced and
   fixed *in the test harness itself*, not the product, worth recording
   since it produced a very convincing false alarm: the first version of
   the pty-polling helper had a stray `else: break` that abandoned the
   read loop after a single 200ms `select()` timeout instead of polling
   for the full requested duration — since `StartRecording`'s real
   background-process spawn plus its internal 300ms daemon-health check
   routinely takes longer than 200ms, every "after Enter" screenshot was
   captured mid-flight, showing the prompt still open even though the
   session had, in fact, already been created correctly on disk. Fixing
   the test harness (not the product) made the real behavior visible and
   correct.

## 25. Per-annotation highlight color

Decided 2026-08-04 — the last of the deferred items from decision #19's
follow-up list ("color changing for highlights"), picked up now that the
foundation (unified history, live editing) it was explicitly deferred
behind is solid. Every annotation used to render in one fixed color
(macOS system red) with no way to change it.

**`session.Annotation` gained `Color string`** — a `"#rrggbb"` hex
string, `omitempty` so every annotation created before this field existed
(no `Color` key in old `session.json` files) keeps rendering exactly the
color it always did rather than an unset/invalid one. `AddAnnotation` and
`UpdateAnnotation` both take a `color` parameter now; the click daemon's
automatic ring passes `""` (default), matching its existing look
unchanged.

**`capture.Annotation` and `RenderAnnotations`** carry `Color` through to
actual pixels, via a new `parseAnnotationColor(hex string) color.RGBA`
that falls back to the default highlight color for anything empty or
malformed. This mattered more than a passing glance suggests: a stored
color is user-supplied free text that travels all the way from a
browser's `<input type="color">` through an HTTP request body to disk and
back — `""`, a 3-digit shorthand, garbage text, even something
injection-shaped like `"javascript:alert(1)"` all have to degrade to a
sane rendered color, never panic or corrupt the image. `RenderAnnotations`
now resolves each annotation's own color independently in the same loop
that already draws its border/badge — the loop was per-annotation before
this change too, so drawing a different actual color per iteration was a
small addition to an already-correct structure, not a redesign.

**Editor UI**: a native `<input type="color">` in the same floating
per-annotation toolbar decision #20 built (renumber/reorder/delete),
committing on `change` the same way renumbering already does. Two things
worth being explicit about, because they're exactly the kind of thing
that's easy to get subtly wrong:

- Every place that reads "this annotation's color" — the inline style
  applied to the border and badge, the color input's initial value, the
  value sent back to the API — goes through one small `color()` helper
  that resolves `ann.color || DEFAULT_ANNOTATION_COLOR` once, so display
  and persistence can never silently disagree about what "unset" means.
- `UpdateAnnotation` *replaces* an annotation's stored color with
  whatever the request sends — there's no "leave color alone if not
  specified" merge behavior. That means every commit path (drag/resize,
  renumber, and the new color change itself) has to explicitly resend
  `ann.color` every time, or a plain drag would silently reset a
  carefully chosen custom color back to the default. All three call
  sites in `html.go` were updated together for exactly this reason.

**Verified:**

1. Unit tests: `TestParseAnnotationColor` (with/without a leading `#`,
   uppercase hex, too-short/too-long strings, non-hex garbage, and an
   injection-shaped string — all fall back to the default except the two
   genuinely valid forms), `TestRenderAnnotationsUsesPerAnnotationColor`
   (two annotations with different colors on the same image actually
   render as two different colors, not one image-wide setting bleeding
   across both).
2. `go build/vet/gofmt` clean, full suite passing.
3. Live browser verification against a real session: confirmed the color
   input renders inside the hover/pinned toolbar at the correct default
   value; changed it and confirmed via `GET /api/session` that the new
   hex value actually persisted, and via `getComputedStyle` that the
   border and badge updated live in the DOM without a reload; renumbered
   the same annotation afterward and confirmed the color survived (the
   exact regression the "every commit path must resend color" note above
   exists to prevent); called `/api/undo` and confirmed the restored
   snapshot carried the correct color, proving color rides along
   correctly inside the unified history from decision #19 rather than
   needing its own separate tracking. Also flattened a real export to PDF
   and rasterized it — the annotation on the actual exported page
   genuinely renders blue, not just in the editor overlay. This step
   produced a real false alarm worth recording: an unrelated red circle
   that happened to be part of a *different* step's captured screenshot
   content (visible on a different page) was mistaken for the annotation
   badge on first look, since both are small red circles in a similar
   position — resolved by checking which heading was actually above the
   screenshot being inspected, not by trusting "it's red, that must be
   my bug."

## 26. Image cropping — non-destructive, presentation-time, in the unified history

Decided 2026-08-04, next in the deferred list from decision #19 (crop was
explicitly named there). Chosen deliberately over the alternatives with
a specific persona in mind, per the standing instruction to think about
how a feature actually gets used by developers and QA before building
it: a bug-report or PR-walkthrough screenshot is almost always noisier
than the thing it's actually about — other apps in the background, a
notification banner, unrelated sidebar chrome — and cropping that away
is a direct legibility win for whoever reads the story afterward. It's
also a quiet security assist secret-masking can't touch: masking
(`internal/mask`) only ever sees *text* fields, never screenshot pixels,
so a Slack notification with someone else's message, or a different
app's window peeking into frame, is exactly the kind of thing a crop
catches that the masking pipeline structurally can't.

**Non-destructive, like annotations, unlike redaction.** `session.Step`
gained `Crop *CropRect` (nil = uncropped — every step before this field
existed, and every step nobody has cropped). The stored screenshot's
pixels are never trimmed; a crop is a stored rectangle applied fresh at
export/flatten time (`internal/export`), same as annotations. This was
the one real design decision worth deliberating rather than defaulting
to "just crop the file": redaction is destructive *on purpose*, because
the whole point is that covered content is actually gone — but cropping
is a framing choice, not a security one, and nothing about it needs that
guarantee. Non-destructive means adjustable-forever without penalty,
which matters a lot in practice: a crop drawn slightly too tight is a
common mistake, and there's no reason "redo it" should mean "redraw from
scratch" when the original pixels were never lost in the first place.

**Crop happens before annotations at export time, with annotation
coordinates rebased to match.** An annotation is stored in the
*original* image's pixel space (it has to be — the editor's own overlay
always shows the full uncropped image, crop or no crop, so annotation
positions must stay meaningful independent of whatever crop is or isn't
set). `stepImageBytes` crops first, then shifts every annotation's (x,y)
by subtracting the crop's own origin, then renders — and deliberately
does *no* explicit filtering of annotations that land outside the newly
cropped canvas: `RenderAnnotations`' drawing primitives already clip to
the target image's bounds pixel-by-pixel (the same bounds-checking
`Redact`'s solid fill already relied on), so an annotation shifted
off-canvas just doesn't draw anything, correctly, by construction,
without needing a second piece of logic to get right.

**Joins the same unified undo/redo/reset history as everything else**
(decision #19), not a parallel mechanism — `editSnapshot` gained a `Crop
*session.CropRect` field alongside `Image`/`Annotations`, and the
low-level restore primitive (`SetAnnotations`, previously
annotations-only) was renamed to `SetStepPresentation` and extended to
restore both fields atomically. This was a deliberate refusal to build
crop's own separate history "just this once" — the entire point of the
decision #19 rebuild was that a single Undo walks true chronological
order across every kind of edit to a step, and a crop-specific
undo/redo would have quietly reintroduced the exact fragmentation that
rebuild existed to eliminate.

**Editor UI**: a third "Draw Crop" toolbar button sharing the *exact*
same draw → adjust → Apply flow redaction and highlights already use
(`setupDrawing` gained a third mode rather than a parallel
implementation). Once applied, the crop boundary renders as a dashed
outline plus four bands darkening everything *outside* the kept region
— deliberately four separate absolutely-positioned divs, not a single
`overflow:hidden` + giant `box-shadow` trick (the more common way to do
this), because `.img-frame` can't safely clip overflow: a resize handle
on an annotation near the image's edge is drawn a few px outside the
annotation's own box on purpose, and clipping the frame would cut those
off. A "Clear Crop" button (disabled when there's no crop) removes it.

**Verified:**

1. Unit tests: `internal/capture` (`Crop` returns exactly the requested
   region — checked against a four-quadrant test image so a wrong
   region is unambiguous, not just "some pixels changed"; clamps a
   rectangle that extends past an edge instead of erroring or padding;
   rejects a rectangle entirely outside the image). `internal/export`
   (crop applied before annotations with coordinates correctly
   rebased — checked by placing an annotation exactly at the crop's own
   origin and confirming it lands at (0,0) in the cropped output, not
   its original coordinates; crop with zero annotations still produces
   the right size; no crop and no annotations returns the original
   bytes completely unchanged, byte-for-byte; `shiftAnnotationsForCrop`
   doesn't mutate the caller's slice — the session's live in-memory
   annotation data must stay in original-image coordinates for the
   editor's own overlay to keep working; `stepImagePath` produces a
   flattened copy for a crop-only step with zero annotations, which the
   size check alone wouldn't have caught if the "has anything to
   flatten" condition had been left checking annotations only).
   `internal/session` (`SetCrop`/`ClearCrop` persist and reload
   correctly; `SetCrop` rejects non-positive dimensions; clearing an
   already-clear crop is a no-op, not an error).
2. `go build/vet/gofmt` clean, full suite passing.
3. Live browser verification against a real session: drew a crop via
   the actual UI (armed the button, dragged, Apply), confirmed the
   dashed-boundary-plus-dark-mask rendered exactly where drawn and
   "Clear Crop" became enabled; confirmed the rectangle persisted via
   `GET /api/session` (caught, in the process, the same "which step am
   I actually looking at" display-order mix-up as decision #22's
   verification — resolved the same way, by checking the response
   payload's own `index` field rather than assuming); clicked the real
   Undo button (not a raw API call — the point was verifying the live
   DOM update path, not just the backend) and confirmed the mask
   disappeared and "Clear Crop" re-disabled; Redo brought it back;
   exported to PDF and visually confirmed the actual embedded screenshot
   is genuinely cropped — no menu bar, no sidebar, no traffic-light
   dots that were plainly visible in the original — while a different,
   uncropped step's screenshot in the same document was untouched.

## 27. Freehand hand-drawn annotations

Decided 2026-08-04. Every annotation until now was a rectangle — fine for
"this button" or "this panel," but a rectangle can't point at a specific
line in a stack trace, trace a diagonal path from a button to the toast
it produced, or circle an irregularly-shaped region of a screenshot
without also boxing in a bunch of unrelated pixels around it. Thought
through from the dev/QA side specifically: the person writing a bug
report or a QA repro step reaches for a pen gesture instinctively —
"circle the thing," "draw an arrow from here to there" — and a rectangle
forces them to either approximate with a box or fall back to a caption
sentence describing what they'd rather have just drawn. This closes that
gap without asking them to learn a second annotation tool: freehand is a
draw mode alongside Redaction/Highlight/Crop in the same toolbar, using
the same draw → adjust → Apply flow.

**Extends the existing generic `Annotation` type rather than building a
parallel one** — the same "keep generic" principle decision #18
established when click-highlight and manual highlight were unified.
`session.Annotation` gained `Points []Point` (`omitempty`, nil for every
annotation before this field existed); `X/Y/W/H` remain the source of
truth for hit-testing, the numbered badge position, and the move/color
API, but for a freehand annotation they're *derived* — computed once via
`boundingBox(points)` at creation and recomputed on every move — rather
than independently settable. `AddAnnotation`/`UpdateAnnotation` both
take a `points []Point` parameter; when non-nil, the caller's `x,y,w,h`
arguments are ignored in favor of the computed bounding box. The nil-vs-
non-nil distinction (not an empty-slice check) is deliberate and rests on
Go's own JSON semantics: an omitted or `null` `points` field unmarshals
to a nil slice, while an explicit `"points":[]` unmarshals to a non-nil
empty one — which is how the backend tells "this is a rectangle
resize/renumber/recolor request, leave Points alone" apart from "this is
a freehand move, recompute the bounding box," without needing a separate
boolean flag on the wire.

**Rendering branches on `len(Points) > 0`, once, in the same per-
annotation loop `RenderAnnotations` already had** (the same loop decision
#25 extended for per-annotation color) — a freehand annotation draws its
stroke via a new `drawPolyline`, a rectangle draws its border via the
existing `drawRectBorder`; both still get the same numbered badge. Stroke
rendering deliberately reuses the already-tested `drawFilledCircle`
primitive — stamping overlapping circles along each segment at
`radius * 0.5` spacing — rather than writing a dedicated thick-line
rasterizer, since a stamped-circle stroke was already proven correct
pixel-by-pixel for the badge and needed no new antialiasing logic.

**Crop rebasing (decision #26) extends to every point, not just the
bounding box** — `shiftAnnotationsForCrop` now shifts each `Point` by the
crop's own origin alongside `X/Y`, otherwise a cropped export would show
the numbered badge in the right place while the actual stroke rendered
offset from it, silently wrong in a way that's easy to miss on a quick
look and only obvious once you trace the badge back to its stroke.

**Editor UI**: a "Draw Freehand" toolbar button following the same
pattern as Crop before it — a new `armedMode`, not a parallel input-
handling path. While drawing, a live SVG polyline preview traces the
cursor's path directly in natural pixel space (points are captured via
`framePosNatural`, the same helper used elsewhere, so there's no
screen-to-natural conversion needed at Apply time — the raw captured
points are exactly what gets POSTed). A persisted freehand annotation
renders as an inline SVG `<polyline>` sized to its own bounding box
(`viewBox="0 0 w h"`, points expressed relative to the box's own
origin) rather than a CSS-positioned border, so the annotation element
itself carries the actual stroke shape, not just a hit box around it.

**Move-only — no resize handles for freehand annotations**, a deliberate
scope decision rather than an oversight: `commitPosition` for a freehand
annotation translates every point by the drag delta and resends the
whole `points` array, which composes cleanly with drag-to-move exactly
like a rectangle does. Scaling a hand-drawn path from a corner handle,
by contrast, rarely preserves what made the gesture meaningful in the
first place — a circle drawn around one line of a stack trace, stretched
non-uniformly from a corner, stops looking like a circle around that
line — so rather than ship a resize interaction likely to make strokes
worse more often than better, this pass ships move-only and leaves
resize as a possible future addition if it turns out to be wanted.

**Verified:**

1. Unit tests: `internal/session` (`TestBoundingBox` — empty, single
   point, diagonal, and out-of-order point sets; freehand `Add`/`Update`
   correctly recompute the bounding box from points and ignore the
   caller's x/y/w/h; rectangle `Add`/`Update` are unaffected by the
   freehand code path and never touch `Points`). `internal/capture`
   (`TestRenderAnnotationsFreehandDrawsAlongThePathNotARectangle` —
   draws a straight horizontal stroke and checks both that its midpoint
   is painted *and* that a point below the line, where a rectangle's
   border would have been, is untouched — specifically distinguishing
   "drew a stroke" from "drew a rectangle and got lucky"; single-point
   stroke draws a dot instead of degenerating). `internal/export`
   (`TestShiftAnnotationsForCropShiftsFreehandPoints` — every point
   moves with the crop, not just the bounding box).
2. `go build/vet/gofmt` clean, full suite passing.
3. Live browser verification against a real session: armed "Draw
   Freehand," dispatched a synthetic 9-point mouse path, confirmed the
   live SVG preview traced the actual zigzag shape (not a straight line
   or a box) as points were added; clicked Apply and confirmed via
   `GET /api/session` that all 9 points persisted with a correctly
   computed bounding box; dragged the stroke and confirmed via the API
   response that the bounding box shifted by the same delta as the
   drag, and that the *first stored point* shifted by that exact same
   delta with width/height unchanged — confirming pure translation, not
   a resize-in-disguise. Exported to PDF and rasterized the page with
   PyMuPDF to look at actual pixels rather than trust the code path: the
   embedded image shows a genuinely irregular hand-drawn outline, not a
   rectangle, at the post-move position — the same independent-pixel
   verification already applied to crop (#26) and color (#25).

## 28. Focused "expand" view for the image editor

Decided 2026-08-04. Every step's screenshot editor lived inline in a
900px-wide reading column alongside every other step. That's fine for
skimming a story, but wrong for the actual editing work: a screenshot
captured at full display resolution — often 1900px+ wide, more on
HiDPI — was rendered down to a few hundred CSS pixels before a QA
person could even start drawing on it. Thought through from that
person's actual workflow: circling one specific line in a dense stack
trace, or drawing a redaction box tight around a credential without
also covering the label next to it, both need to see and click
individual pixels precisely — and a screenshot squeezed to a third of
its size makes exactly that kind of precision work needlessly fiddly,
which is the opposite of what an annotation tool should feel like.

**Reparents the existing DOM, doesn't build a second editor.**
`buildImageEditor` already constructs one self-contained `imgWrap`
subtree (toolbar + image + annotation/crop overlays) with every
interaction — drawing, dragging, undo/redo, the `stepEditors[...]`
live-update hooks — wired as closures directly over that subtree's own
nodes. A parallel "modal editor" would have meant either duplicating
all of that wiring or threading a size parameter through every drag/
resize calculation in it; instead, a small "⤢" button on each image
calls `openFocusView(imgWrap, label)`, which does nothing more than
`modalBody.appendChild(imgWrap)` — a real DOM move, not a clone, so
every existing listener and closure keeps working unchanged — and
records where it came from so `closeFocusView()` can put it straight
back. This works cleanly for a structural reason established earlier
in the project rather than by luck: every position in that subtree
(annotation placement, drag overlays, the crop mask) is already
computed either as a percentage of the image's *natural* size or as a
ratio measured fresh at drag time (see the `renderAnnotations` and
`setupDrawing` comments), never as a fixed pixel value baked in when
the element was first built — so rendering the exact same subtree at
several times the size, in a different part of the page, needed zero
changes to any interaction code.

**Fixed-position overlay, not another element inside the 900px
column** — the whole point was to escape that width cap, so the modal
renders relative to the viewport (`position: fixed; inset: 0`) with the
image capped at `max-height: 76vh; max-width: 88vw` instead of the
column's `max-width: 100%`, filling most of the actual screen rather
than a fraction of a reading-width block. Three equivalent ways to
close it (the "× Close" button, clicking the dark backdrop, or Escape)
all funnel into the same `closeFocusView`, so there's exactly one place
that owns "put the node back" — a step list re-render can't happen
while the modal is open (it covers and intercepts the whole viewport),
so there's no risk of `closeFocusView` trying to reinsert into a parent
`render()` already threw away.

**Verified:**

1. `go build/vet/gofmt` clean, full suite passing (this is a frontend-
   only, embedded-JS change — no new Go logic, so no new Go unit tests;
   the interaction logic being verified is entirely the *existing*,
   already-unit-tested backend endpoints exercised through the reused
   drawing code).
2. Live browser verification against a real session: clicked the expand
   button and confirmed the image rendered at roughly 2.4x its inline
   size (1054 CSS px vs. its ~440px inline width, for a 1904px-wide
   natural image) with the toolbar, crop mask, and existing freehand
   annotation all still correctly positioned at the new size; confirmed
   via the accessibility tree that a second step's own image editor
   (left in the background) kept its own visible expand button while
   the focused step's expand button correctly disappeared while inside
   the modal (the `.editor-modal-body .expand-btn { display: none }`
   rule doing its job); dispatched a synthetic redact drag inside the
   modal, applied it, confirmed the redaction rendered correctly at the
   enlarged scale, then used the real Undo button to revert it cleanly;
   closed via Escape and confirmed the image reappeared in its original
   step card with the DOM otherwise undisturbed; reopened and closed
   again via the "× Close" button as a second, independent path to the
   same `closeFocusView` function; confirmed via `GET /api/session`
   that no stray edit state was left behind by the whole sequence.

## 29. Freeform "content steps" — Markdown, images, and video/link embeds between captured steps

Decided 2026-08-04, scoped with the user up front (two direct
questions: what should v1 contain, and how should a content step be
created) rather than guessing at what "rich text editor" meant — the
previous note on this task flagged that ambiguity explicitly, and it
was real: "rich text" could have meant anything from a plain textarea
to a full WYSIWYG toolbar. Landed scope: Markdown text, uploaded
images, and YouTube/Vimeo embeds; created via a "+ Insert step" control
placed exactly where the user wants it in the story, not appended to
the end.

Thought through from the dev/QA side: a captured screenshot can't
explain *why* a step matters, link out to a design doc, or point at a
demo video — today the only place to add that is a step's caption,
squeezed in alongside a screenshot it's not really about. A content
step is for exactly that: an authored aside dropped between two real
captures, e.g. "why this refactor was necessary" before the steps that
show it, or a linked walkthrough video after a tricky one.

**Reuses the existing Note/Markdown pipeline end to end rather than
inventing a new content model.** A content step is still a `Step` (a
`Session.Steps` entry, `DisplayOrder`-positioned, reorderable/hideable
like any other) with no `Command`/`ImageFile`, its body living in the
same `Note` field manual capture-time steps already use, rendered by
the exact Markdown pipeline decision #22 built. The only new schema is
one boolean, `Step.IsContent` (`omitempty`, false for every step before
this existed) — it changes nothing about how a step *exports* (every
exporter already only shows a window line `if step.WindowApp != ""`,
which is already empty for an authored step), only how the *editor*
displays it: skipping the "+ add app" window-info affordance that would
otherwise invite attributing a paragraph of prose to an application.
Reusing Note/Markdown instead of a parallel "content block" type meant
"Markdown text" support needed zero new code — the entire feature
reduced to insertion UX, image upload, and closing two real gaps the
existing Markdown pipeline had never needed to cover before content
steps made them visible (below).

**`InsertContentStep(name, position)`** is the first place a step can
land anywhere but the end — every prior creator (`AddStep`,
`AddTerminalStep`) only ever appends, because live capture has no
concept of "insert before step 3." It materializes `DisplayOrder`
(synthesizing it from natural step order first, if never explicitly
set) and splices the new index in at `position`, which is clamped
rather than rejected — the frontend always derives it from its own
current rendering of the list, so there's no realistic invalid value,
only the two ends.

**Uploaded images: a small, separately-scoped endpoint, not a Step
field.** `/api/upload-image` writes PNG/JPEG bytes (base64 JSON body,
matching this file's existing all-JSON convention — no multipart
handling introduced for one endpoint) to the session's `images/`
folder and returns a filename; the frontend inserts a plain
`![](filename)` at the textarea's cursor and the *existing*
`/api/caption` save persists it. No new field tracks "this step has
these images" anywhere — exactly like a captured screenshot's presence
is implied only by a `Step.ImageFile` reference, an uploaded image's
presence is implied only by its Markdown reference inside `Note`.
PNG/JPEG only (sniffed via `http.DetectContentType`, not trusted from
the filename) is a deliberate, narrow v1 boundary, not a
half-finished one — documented here rather than silently discovered.

**Video/link embeds degrade by format, not by silently doing
nothing.** A bare YouTube/Vimeo URL alone on its own line (the same
"paste a link on its own line" convention Slack/Notion use) becomes a
real `<iframe>` in Slides — the one export format actually capable of
hosting one. PDF and Markdown structurally can't play a video, so the
same line there stays a clickable link instead — which meant fixing
two real, pre-existing gaps in `writeNotePDF` that this feature made
visible for the first time (a caption had never needed either before):
**(1)** a bare `https://…` URL (no `[text](url)` or `<url>` syntax)
rendered as inert black text, not a link — fixed by switching both PDF
and Slides' shared `mdParser` to goldmark's GFM `Linkify` extension.
**(2)** an image reference inside a caption's Markdown wasn't rendered
at all — `writeNotePDF`'s AST walker had no `*ast.Image` case, so it
would have silently vanished from the one export format that embeds
everything else as real content rather than links. Fixed with a
`writeNoteImagePDF` helper mirroring `renderWindowStep`'s own
screenshot-embedding logic (same width-scaling-to-fit-a-page approach)
— a missing file renders a visible `[image missing: ...]` note rather
than erroring, same failure-visibility choice the screenshot embed
already makes. **(3)**, caught only during live verification, not code
review: a Markdown heading (`## …`) had no `*ast.Heading` case either —
its text ran directly into the next paragraph's text on the same line,
with no space or break, because only `*ast.Paragraph` ever called
`pdf.Ln()`. Fixed by giving headings the same bold treatment as
`**bold**` text (reusing `boldDepth`) plus their own trailing break —
deliberately not scaling font size per heading level, since making a
heading merely *visible as a heading* was the actual defect; a full
per-level type scale is a nicety for later, not a fix required now.

**Slides' self-containment (screenshots embedded as data URIs, no
`images/` folder needs to travel with the file — see `Slides`' own doc
comment) extends to note-embedded images too.** `inlineLocalImages`
rewrites a relative `![](filename)` reference into a `data:` URI before
handing the note to goldmark, specifically for Slides — Markdown export
needs the opposite treatment (a relative link, matching exactly how it
already references every captured screenshot) and gets none, since
that's already correct as-is. Getting this backwards would have meant
an uploaded image looking fine in the editor and in Markdown export,
then silently breaking the moment the self-contained Slides `.html`
file was opened from anywhere other than right next to the session
folder — exactly the kind of failure that's invisible until someone
actually moves the file.

**Verified:**

1. Unit tests: `internal/session` (`InsertContentStep` lands the new
   index at the exact requested `DisplayOrder` position — not just
   appended, which is the specific mistake that would leave it
   invisible in the editor per `VisibleSteps`' own comment; clamps an
   out-of-range position instead of rejecting it; synthesizes
   `DisplayOrder` correctly on a session that never had one set
   explicitly). `internal/export` (`videoEmbedURL` recognizes YouTube
   watch/short/embed URLs and Vimeo, and explicitly does NOT recognize
   an unrelated or Loom link; a standalone video-URL line embeds while
   surrounding paragraphs stay intact and a non-video link stays a
   plain Linkify'd `<a>`; `inlineLocalImages` rewrites a genuinely local
   reference to a data URI, leaves remote/already-`data:` references
   untouched, and leaves a reference to a missing file as-is rather
   than dropping it; `writeNotePDF` actually embeds a local image
   (cursor advances by a block-sized amount, not a line's worth) and
   shows a placeholder instead of erroring for a missing one; the
   heading-separation regression test specifically pins "heading +
   paragraph takes strictly more vertical space than the same words as
   one flat paragraph," which is what the pre-fix run-together bug
   would have failed).
2. `go build/vet/gofmt` clean, full suite passing.
3. Live browser verification against a real session: clicked "+ Insert
   step," confirmed a new card appeared at exactly that position with
   the textarea auto-focused and no window-info line shown; typed a
   heading, bold text, a bare YouTube link on its own line, and a
   Markdown link; drove the real "Insert Image" file input with a
   synthetic `File`/`DataTransfer` (the same technique used for every
   other browser-input test this session where no OS-level widget can
   be driven) through the actual `FileReader` → base64 → `fetch` code
   path — not a mocked shortcut — confirmed the uploaded file landed on
   disk and a `![](...)` reference appeared at the cursor; saved and
   confirmed via `GET /api/session` the note persisted exactly as
   typed. Exported to all three formats and inspected each directly
   rather than trusting "no error": Markdown export's raw `.md` file
   contains the expected relative image link; the exported Slides
   `.html`, loaded in a real browser, showed a genuinely live YouTube
   `<iframe>` (the video's own "unavailable" message came from
   YouTube's servers refusing to embed that specific video — direct
   proof the iframe was actually querying YouTube, not a rendering
   bug), the Markdown link as clickable text, and the uploaded image
   rendered correctly; the exported PDF, rasterized with PyMuPDF (the
   same pixel-level verification method used for every export-facing
   feature this session), showed the heading properly bolded and
   separated, both links genuinely blue and clickable, and the
   uploaded image embedded as a real block image at the correct
   position and scale — this rasterized check is what caught the
   heading bug in the first place; it wasn't visible in the unit tests
   until a regression test was written specifically to pin it after.

## 30. Annotation redesign: a persistent shape-tool stack, decoupled from numbering

Decided 2026-08-04, in direct response to user feedback that named two
concrete problems and a design change, not just "improve annotations":
"I draw once and leave it. It does not give me to draw again, that
should not be the case... we also want tools like circle, rect, lines,
arrows... by default no shape will have annotations... when I select
them or hover on the shapes I can have option to add annotations."

**The bug: every draw tool disarmed itself after one shape.** Redact,
the old single "Draw Highlight," and freehand all called `setArmed(null)`
the instant Apply succeeded (or, for freehand, the instant a stroke was
created) — meaning drawing a second shape always required re-clicking
the tool button first. This matched how a *destructive* action like
redaction should behave (deliberate, one at a time), but it was the
wrong model entirely for markup: a real drawing tool (Excalidraw,
Figma) keeps a tool selected across many draws until you explicitly
switch away, and that's what "I draw once and it won't let me draw
again" was actually asking for.

**Two different interaction models now share one "armed tool" concept,
split by what each tool actually is.** Redact and Crop keep the
original draw -> adjust (resize via corner handles) -> Apply/Cancel
flow and disarm after Apply, because both are genuinely different from
markup: redaction is destructive and deliberately not "just try it,"
and crop replaces the single special region a step can have, not one
of many independent objects. The five shape tools — Rectangle, Circle,
Line, Arrow, Pen (freehand) — are a **persistent palette**: arming one
stays armed across draws (click the same tool again, press Escape, or
pick a different tool to stop — never after just one shape), and each
drawn shape POSTs to `/api/annotation` and becomes a real, immediately
draggable object the instant the mouse comes up, with **no separate
Apply step at all**. Post-creation dragging/resizing already existed
(`buildAnnotationElement`) for adjusting a shape that came out slightly
wrong, so a second adjust-before-commit phase in front of that would
have just been the same capability twice — removing it is a
simplification, not a missing feature.

**Circle/Line/Arrow reuse existing primitives rather than adding new
rasterization code.** A circle is an ellipse inscribed in the drawn
box, stroked via the exact same overlapping-stamped-circles technique
`drawPolyline` already used for a freehand stroke (`drawEllipseBorder`
walks the parametric ellipse and stamps `drawFilledCircle` at each
angle). A line is a two-point path — `drawPolyline` with two points
already draws a straight segment, so line needed **zero new Go
rendering code**. An arrow is a line plus `drawArrowhead`, which is
itself just two more two-point `drawPolyline` calls for the wings. On
the editor side, a box shape (rect/circle) renders via a CSS
border (`border-radius: 50%` for circle — one line of CSS approximates
the ellipse well enough for a live preview and the persisted shape
alike) and a path shape (line/arrow/freehand) renders via an inline SVG
polyline, generalizing the freehand-only SVG code from decision #27 to
all three; an arrow's polyline additionally references one shared,
lazily-created SVG `<marker>` (`fill="context-stroke"`, so it
auto-matches whatever color the shape is set to, with no per-color
marker variants needed).

**Every shape defaults to NOT annotated — numbering is now an explicit,
per-shape opt-in, not automatic.** Before this, every rectangle or
freehand stroke was unconditionally a numbered "annotation": drawing a
plain circle around something to just highlight it always claimed a
number in the sequence whether that made sense or not. `Annotation`
gained `Annotated *bool` — a pointer specifically so "the JSON key was
never written" (every annotation saved before this field existed, which
under the OLD rules was *always* shown numbered) can be told apart from
"explicitly set to false" (a freshly drawn shape, deliberately left as
plain markup): nil means "legacy, keep showing the badge exactly as it
always has," never "unset, default to hidden." `IsAnnotated()` is the
one place that resolves this; `capture.Annotation`'s own `Annotated`
field is a plain bool by contrast, because `toCaptureAnnotations`
already resolves the legacy convention via `IsAnnotated()` before values
ever reach the rendering package — by the time `RenderAnnotations` sees
`Annotated`, false always genuinely means "don't draw the badge," no
further nil-checking needed downstream. The click daemon's automatic
ring is the one shape creator that still explicitly passes
`Annotated: true`, since a sequential numbered marker showing click
order is its entire purpose and was never in question.

**A shape's own hover toolbar gained a "#" toggle** (next to the
existing renumber/recolor/reorder/delete controls, which already only
appear on hover/pin per decision #18's "chrome, not permanent clutter"
choice) that turns the numbered badge on or off. Turning it on assigns
the next number past the highest currently-*annotated* number on that
step (computed client-side from the same `step.annotations` array
already kept in sync, not a new backend endpoint — `/api/annotation/
update` already accepted a `Number`) rather than reusing whatever number
the shape happened to be created with; turning it off leaves the
stored number alone (harmless and unused while off, and re-toggling
later reassigns fresh rather than risking a stale, confusing number).
The number input itself is hidden alongside the badge when a shape
isn't annotated, so the toolbar doesn't show an editable field for
something that currently has no visible effect.

**`AddAnnotation`/`UpdateAnnotation` took a new `AnnotationShape`
struct instead of more positional parameters.** Both were already at
eight parameters before this feature; adding `Kind` and `Annotated`
would have pushed `UpdateAnnotation` to twelve, well past the point
Go's own conventions call for a struct instead of a longer positional
list — and since every call site (the click daemon, both editor
endpoints, every test) needed touching anyway for the new fields, the
incremental cost of doing this properly now was low. Every commit path
in the editor (drag, resize, renumber, recolor, and now toggling
annotated) resends the shape's FULL current state through this struct,
not a partial patch — the exact "a plain drag silently resets an
unrelated field" lesson decision #25 already learned for color, now
also true for `Kind` and `Annotated`.

**Verified:**

1. Unit tests: `internal/session` (`IsAnnotated()`'s nil-means-legacy-
   true resolution, explicitly for nil/&true/&false; a newly created
   shape's `Annotated` is exactly what the caller asked for and is
   never left as a nil pointer, which is reserved for pre-existing
   data; toggling `Annotated` via `UpdateAnnotation` round-trips through
   save/reload; `Kind` persists correctly for circle/line/arrow, with a
   line's bounding box correctly derived from its two points).
   `internal/capture` (a circle's rendered pixels form an ellipse, not
   a rectangle — checked by confirming a bbox corner a rectangle border
   would touch stays untouched while the ellipse's actual top-center
   boundary point is painted; a line's stroke follows only the
   diagonal, not its own bounding box; an arrow paints extra pixels
   near its endpoint beyond what the identical two-point line does,
   proving the arrowhead actually extends past the bare shaft;
   `Annotated: false` suppresses the badge while leaving the outline
   drawn; legacy data — `Kind == ""` with non-empty `Points` — still
   resolves to "freehand" via `shapeKind`, not the empty-Kind "rect"
   fallback).
2. `go build/vet/gofmt` clean, full suite passing.
3. Live browser verification against real sessions, specifically
   targeting the reported bug and the new behaviors: armed "Rectangle"
   and drew two separate shapes back to back with no re-click between
   them, confirming the tool stayed armed and both persisted (the exact
   bug report — "draw once and it won't let me draw again" — now
   doesn't reproduce); did the same for circle, line, arrow, and pen in
   turn, each confirmed via `GET /api/session` with the correct `kind`
   and `annotated: false`; a **first attempt** at testing all five
   shapes fired their draws synchronously with no gap between requests
   and produced corrupted, overwritten data — traced to `AddAnnotation`'s
   pre-existing (not introduced by this change) Load-mutate-Save
   pattern racing when multiple requests overlap in flight, a scenario
   only reachable by firing synthetic events faster than any real user
   can click; re-ran with each draw properly sequenced (matching real
   usage) and every shape persisted correctly, confirming this was a
   test-harness artifact, not a product regression — recorded here as
   the same kind of "verify the test before blaming the product" lesson
   decisions #24/#25 already logged, and as a known pre-existing
   concurrency characteristic worth hardening separately if it ever
   proves reachable by realistic use. Toggled a shape's "#" button on,
   confirmed the badge appeared with the correct next-sequential number
   and a **sibling shape stayed completely unannotated** (isolated,
   properly-sequenced repeat of the toggle test after the race above
   was understood); toggled back off, confirmed the badge disappeared.
   Dragged a rectangle and confirmed its new position persisted with
   `kind`/`annotated` unchanged; dragged an arrow and confirmed via the
   API response that every point translated by the same delta as the
   bounding box, not just the box itself. Confirmed Escape disarms an
   armed tool. Exported to PDF and rasterized with PyMuPDF: two plain
   rectangles and an arrow with a genuinely visible arrowhead all
   rendered with **no numbered badges at all**, matching the new
   default; a separate legacy session (`test4`, with annotations saved
   before this feature existed) still rendered its old badge correctly
   after reloading in the redesigned editor, confirming the nil-means-
   legacy-true compatibility path works end to end, not just in a unit
   test.

## 31. Configurable header display style (App/Title/Both/Hidden)

Decided 2026-08-04, prompted by the user asking directly why the
window line always shows as two segments and whether it could change —
answered with a scoping question (what should change: nothing, a
merged field, app-only, or something else?), and the answer was
"multiple templates... hide show... some where," which this turns into
four concrete, always-reversible display styles rather than a redesign
of the underlying data.

**Purely a display choice layered over data that was already there —
no new capture behavior, and the underlying fields are never
touched.** `Step.HeaderStyle` ("", "app", "title", or "hidden") only
changes which parts of the ALREADY-captured `WindowApp`/`WindowTitle`
get shown; switching styles back and forth always has the full
original captured info to show again, since nothing is ever cleared or
overwritten. This mirrors the exact same non-destructive-presentation-
layer pattern this project keeps reaching for — annotations over a
clean screenshot (#18), crop over an untrimmed original (#26) — applied
here to text instead of pixels.

**One control, always visible, never a dead end.** A "Header: Both /
App / Title / Hidden" button sits in the step-header row next to the
existing Up/Down/Hide controls — NOT tucked inside the window-line's
own click-to-edit mode, which was the first design considered and
rejected: if the cycle control lived only inside the edit view, setting
the style to "Hidden" would remove the window line's own text entirely,
and with it the only way back in (nothing left to click to reopen edit
mode). Putting it in the header row instead — a row of controls that's
already always rendered regardless of the window line's own content —
means "Hidden" is always one click from "Both" again, matching the "hide
show... somewhere" request literally: the somewhere is a fixed, stable
location, not conditional on current visibility.

**A shared `windowLineSegments` helper, not three copies of the same
branching logic.** Markdown, PDF, and Slides each already rendered the
window line with a slightly different separator style (an em-dash with
markdown bold, a plain hyphen for fpdf's ASCII-only core fonts, an
em-dash in raw HTML) — duplicating "if hidden show nothing, if app-only
show just the app..." three times, once per exporter, would have been
three places to keep in sync and three places a next edge case could
diverge. `windowLineSegments` returns the 0/1/2 strings to show, in
order; each exporter still owns its own separator and formatting,
joining those segments its own way — the actual style-selection
DECISION happens once.

**Verified:**

1. Unit tests: `internal/session` (`EditHeaderStyle` persists and
   round-trips through reload for every valid style, in sequence,
   confirming `WindowApp`/`WindowTitle` are byte-identical throughout —
   the specific claim "purely a display choice" makes; an unrecognized
   style value is rejected outright rather than silently stored).
   `internal/export` (`windowLineSegments` table-tested across both/
   app-only-captured/nothing-captured baseline cases AND all three
   non-default styles, including the edge case of requesting "app" on a
   step that only ever captured a title, or vice versa — correctly
   empty, not a panic or a blank segment).
2. `go build/vet/gofmt` clean, full suite passing.
3. Live browser verification against a real session: clicked the cycle
   button through all four states, confirming via direct DOM inspection
   (not just eyeballing rendered text, since this particular test
   fixture's WindowApp and WindowTitle happened to be identical
   strings, which would have made an app-only and title-only render
   visually indistinguishable) that "Title" mode's window-line `<div>`
   contains exactly one `<span>` — proving the app segment was actually
   omitted from the DOM, not just visually coincidental; confirmed via
   `GET /api/session` after every step that `WindowApp`/`WindowTitle`
   never changed while `header_style` did; confirmed "Hidden" removes
   the window line from the page entirely while the cycle button stays
   present and functional (clicked it again, confirmed it cycled back
   to "Both" and the line reappeared with its original text intact).
   Set a step's style to "hidden" via the live API and exported to
   Markdown, confirming the `**Window:**` line was genuinely absent
   from the output file for that step, proving the export path (not
   just the editor's own preview) honors the setting.

## 32. Icon toolbar, a self-explanatory header-style control, and a live Markdown preview

Decided 2026-08-04, from direct feedback on a screenshot of the just-
shipped toolbar and header control: "improve this toolbar properly with
icons," "improve this Header: Both what it means??? confused," and "for
markdown can we have preview as well right there?" Three small, focused
fixes rather than one redesign.

**Icon toolbar.** The image editor's toolbar had grown to thirteen
text-labeled buttons (decision #30 alone added five) — a wall of words
rather than a toolbar. Every tool button now shows a small inline SVG
icon instead (hand-drawn stroke/fill shapes, 24x24 viewBox, no icon
library or CDN dependency — this project stays local-first, see
decision #3), with the former label moved to `title`/`aria-label` so
it's still there as a hover tooltip and still the accessible name for
assistive tech, just not permanently taking up horizontal space.
Apply/Cancel deliberately KEPT their text labels — they're rare,
higher-stakes confirm/cancel actions (Apply commits a destructive
redaction or replaces the crop), and an icon-only checkmark/X felt like
the wrong place to trade a half-second of reading time for compactness.

**Header-style control: a `<select>`, not a cycle button.** The single
"Header: Both" button from decision #31 was accurate but genuinely
unclear out of context — "Both" only means something once you already
know what it's short for, and a cycling button hides the other three
options entirely until you've clicked through them once. Replaced with
a plain `<select>` listing all four choices by their own full label
("Window: App + Title", "Window: App only", "Window: Title only",
"Window: Hidden") — the options ARE the explanation, nothing left to
intuit from a terse abbreviation. Kept in the same always-visible
step-header spot (not the window-line's own edit mode) for the same
"Hidden must never be a dead end" reasoning decision #31 already
established.

**Live Markdown preview, rendered server-side, not a second parser.**
Every note-bearing textarea (captions and content steps alike) gets a
"Preview" toggle; when on, a debounced (400ms after typing stops) call
to a new `/api/render-preview` endpoint shows the actual rendered HTML
below the textarea. The endpoint is a one-line wrapper
(`export.RenderNotePreview`) around the exact same
`noteHTMLWithVideoEmbeds` pipeline Slides export already uses — the
deliberate alternative to writing a second Markdown-to-HTML converter
in JavaScript. A hand-rolled JS parser would have been faster per
keystroke (no network round trip) but would inevitably drift from the
real Go/goldmark rendering over time as either side picked up an edge
case the other didn't — video-embed detection and Linkify behavior
included for free, correctly, by construction, since it's the same
code path. Local image references (`![](content-....png)`) need no
special handling here the way Slides export's `inlineLocalImages` does:
the preview renders inside the still-running local editor server, so a
plain relative image src resolves correctly against that server's own
`/images/{file}` route — the data-URI inlining trick only exists for
the self-contained, server-less *exported* Slides file.

**Verified:**

1. Unit tests: `internal/export` (`RenderNotePreview` confirmed to use
   the video-embed-aware pipeline specifically, not a plain
   `noteHTML` call, by checking for the same `<div class="video-embed">`
   marker the Slides-export test already pins).
2. `go build/vet/gofmt` clean, full suite passing.
3. Live browser verification against a real session: confirmed every
   icon button's accessible name (via the accessibility tree, not just
   eyeballing) matches its former text label exactly. Clicked Preview,
   typed bold/link/inline-code Markdown, and confirmed the panel
   rendered real `<strong>`/`<a>`/`<code>` — then kept typing and
   confirmed the debounced live-update picked up the change without
   re-clicking Preview. Changed the header-style `<select>` and
   confirmed via `GET /api/session` it persisted correctly. Clicked an
   icon shape-tool button and confirmed the tool still armed/drew
   correctly — this surfaced a real testing-environment quirk worth
   recording: an early synthetic-`MouseEvent`-based check reported
   `window.innerWidth === 0` inside the browser-automation tool's own
   JS execution context, producing degenerate zero-size shapes, even
   though a screenshot taken at the exact same moment showed the page
   rendering completely normally — the tool's JS-eval context and its
   actual rendering surface had briefly disagreed about viewport size.
   Resolved by switching to the automation tool's real click-and-drag
   action (driving actual screen coordinates, the same input path a
   human's mouse would produce) rather than dispatching synthetic
   events computed from that same unreliable in-page measurement,
   which then produced a correctly-sized shape — a test-tooling
   artifact, not a product bug, but one worth writing down since it's
   the kind of false alarm that's easy to misdiagnose as a real
   regression in the coordinate-scaling logic.

## 33. Auto-captions for click recordings — local grounding, optional LLM polish

Decided 2026-08-04, the last unbuilt item from the original Phase 2
roadmap: a pure-click (non-agent) recording captured a sequence of
screenshots with every caption left empty, since nothing narrated what
each click was actually doing. `docs/product/03-features.md` had
already scoped the intended shape years earlier — "using the clicked
element's label/DOM text as grounding context, not raw pixel guessing"
— which meant this genuinely needed two separate pieces of new
capability, each with its own real tradeoff, not one self-contained
feature: (1) knowing *what* was clicked, beyond raw (x, y), and (2)
turning that into a natural sentence. Both were scoped explicitly with
the user before writing any code, rather than assuming the literal
"via an LLM call" wording from the old feature doc still reflected what
was wanted three phases later.

**Element grounding: real macOS Accessibility API, not a text
heuristic.** `capture.AXElementAt(x, y)` queries
`AXUIElementCopyElementAtPosition` for the actual UI control under a
click and pulls its `kAXRoleDescriptionAttribute` (the same human-
readable role VoiceOver itself uses — "button," "text field," not the
raw `AXButton` constant) plus the first non-empty of
`kAXTitleAttribute`/`kAXDescriptionAttribute`/`kAXValueAttribute` as a
label — different control types populate different attributes for
what a human would call their "label," so trying all three in priority
order is load-bearing, not defensive padding. New cgo (matching
`clickhook_darwin.go`'s existing precedent — this file isn't the first
cgo in this package), gated behind `//go:build darwin` with `_linux.go`
and `_other.go` stubs returning `ok=false`, the same three-file
per-platform pattern `clickhook_*.go`/`window_*.go` already established.
Requires its own Accessibility grant, separate from the click tap's
own — deliberately only queried when `--auto-caption` is actually
passed, so a user who never asked for this never sees the extra
permission prompt.

**Local template caption by default; LLM rewrite is opt-in and
asynchronous, never on the capture hot path.** `buildAutoCaption`
composes a caption purely from already-available data (no network, no
API key, no per-click cost) — "Clicked the "Export" button in
Finder.", degrading gracefully through "Clicked a button in Finder."
(no label), "Clicked in Finder." (grounding unavailable), down to ""
(nothing at all available) — never fabricating specificity the query
didn't actually provide. This alone satisfies the roadmap's original
intent for most uses and is what ships when `ANTHROPIC_API_KEY` isn't
set, which is the DEFAULT for every user who hasn't configured one —
consistent with every other decision this project has made toward
staying local-first (no CDN, no headless-browser export dependency,
self-contained Slides files). If a key IS configured,
`polishCaptionAsync` fires a **non-blocking goroutine**, after the step
is already saved with its complete local caption, that asks a small
model (`claude-haiku-4-5`, `max_tokens: 60` — a bounded one-sentence
rewrite, not an open-ended generation task) to phrase it more
naturally, then updates the step via the existing `session.EditNote`
if and when it succeeds. A slow network, a missing key, or an API
error just means the local caption stands — capture responsiveness
never depends on this succeeding, and neither does correctness.

**Only the already-masked caption ever leaves the process.**
`session.AppendStep` already runs every text field (note included)
through the same secret-masking pass every other capture path uses —
this matters more here than it ever has before, since an AX-queried
element label is real, unfiltered UI text (could be a filename, an
email address, anything visible on screen) flowing into a caption for
the first time. `polishCaptionAsync` is deliberately called with
`step.Note` (the value AppendStep already masked and persisted), never
the local `caption` variable computed before that pass — sending the
pre-masking text to a third-party API would silently defeat the
masking guarantee for exactly the case (an LLM call) where it matters
most.

**Verified:**

1. Unit tests: `internal/engine` (`buildAutoCaption` table-tested
   across every degradation tier — labeled element with/without an
   app, role-only, grounding entirely unavailable, and the
   focus-just-changed override, which takes priority over all of
   them, matching the existing annotation-suppression reasoning
   `RunClickDaemon` already had for the same "a focus-switch click
   isn't a deliberate target" case; `rephraseCaption` against a local
   `httptest.Server` confirming the request carries the right
   `x-api-key`/`anthropic-version` headers and the caption embedded
   in the prompt, correctly parses a real response shape, and
   surfaces a non-200 status as an error rather than silently
   returning garbage; `polishCaptionAsync` confirmed to be a
   genuine no-op — no goroutine, no session mutation — when
   `ANTHROPIC_API_KEY` is unset, and confirmed end-to-end (polling
   with a timeout, since it's genuinely async) to update the step's
   persisted note when a key is configured against a mocked
   endpoint).
2. `go build/vet/gofmt` clean on macOS; cross-compiled clean for
   Linux (the `axelement_linux.go` stub, matching `clickhook_linux.go`'s
   already-accepted gap: no click-capture on Linux yet means no click
   to ground an element query against there either).
3. A real, non-mocked end-to-end plumbing check: built the actual CLI
   binary and ran `loomless record start <name> --auto-caption` against
   the live daemon-relaunch path (flag parsing → `StartRecording` →
   `startClickDaemon`'s subprocess relaunch with `--auto-caption` →
   `RunClickDaemon`), confirming the daemon starts cleanly, logs
   "auto-captions enabled," and stops cleanly — the full plumbing
   this decision's code touches, exercised for real, not simulated.
4. **One thing this could NOT be verified live, worth recording
   honestly rather than glossing over**: `AXElementAt`'s actual
   *success* path — finding a real element with a real role/label
   from a real click. Confirmed `AXIsProcessTrusted()` correctly
   reports the built binary's permission state, and confirmed
   `AXElementAt` cleanly returns `ok=false` (no crash, no hang) across
   repeated calls at several coordinates when nothing is found —
   real evidence the cgo memory bridging (the malloc'd C strings,
   `CFRelease` calls) doesn't have an immediately-crashing bug. But
   every attempt to construct a live "found a real, named UI element"
   test in this environment — launching Calculator, an AppleScript
   `display dialog` — failed to produce a queryable window, traced to
   the automation shell here not attaching to the same interactive
   desktop session actually in use (confirmed via `System Events`
   reporting a completely different app as frontmost while a
   Bash-launched Calculator process ran with zero open windows).
   This is a genuine environment constraint, not a reasoning gap —
   recommend a quick manual smoke test on a real interactive session
   (`loomless record start test --auto-caption`, click a few labeled
   buttons in a real app, check the resulting captions) before
   relying on this in practice.

## 34. Multi-provider LLM caption polish (Anthropic/OpenAI/Gemini/Ollama)

Decided 2026-08-04, directly prompted by "do we only support Anthropic,
can't we have more options for llm" — a fair question, since decision
#33 hard-coded a single provider into `polishCaptionAsync` with no seam
for anything else. Scoped with the user first on two axes: *which*
providers (landed on all three cloud options plus a local one, not just
one more), and *how* a configured provider gets picked (landed on an
explicit env var, not scanning for whichever key happens to be set).

**`llmProvider` is a one-method interface** (`rephrase(caption string)
(string, error)`) with four implementations —
`anthropicProvider`/`openAIProvider`/`geminiProvider`/`ollamaProvider`
— because the four providers agree on almost nothing at the wire
level: Anthropic uses an `x-api-key` header, OpenAI uses `Authorization:
Bearer`, Gemini's key travels as a `?key=` query parameter (Google's
own documented convention, not something this file invented), and
Ollama has no key at all. Response shapes differ just as much
(`content[0].text` vs. `choices[0].message.content` vs.
`candidates[0].content.parts[0].text` vs. a flat `{"response": "..."}`)
— there was no honest shared HTTP helper to extract beyond the prompt
text itself (`rewritePromptFor`, one function, reused by all four so a
caption reads the same regardless of which backend answered it).

**`LOOMLESS_LLM_PROVIDER` is required and explicit — no auto-detection by
scanning for whichever API key happens to be present.** The tempting
alternative (if `ANTHROPIC_API_KEY` is set, just use it) has a real
failure mode: plenty of people already export `ANTHROPIC_API_KEY` for
unrelated reasons (Claude Code itself, another tool, a shell profile
default) with zero intention of Loomless spending it on caption rewrites.
Silently picking that up isn't a convenience, it's Loomless quietly making
an API call the user never asked it to make. Naming a provider that
turns out to be missing its own required key (e.g.
`LOOMLESS_LLM_PROVIDER=openai` with no `OPENAI_API_KEY`) is treated as a
real error worth logging once — almost certainly a typo or a forgotten
export, not a state worth silently tolerating the way "no
`LOOMLESS_LLM_PROVIDER` at all" is.

**Ollama is the one provider that needs no API key and sends nothing
over the network by default** (`http://localhost:11434` unless
`OLLAMA_HOST` says otherwise) — genuinely the closest fit to this
project's local-first stance of anything in this feature: a user who'd
rather run the rewrite through a model they've already pulled, with
zero data ever leaving the machine and zero cost, gets exactly that,
not a cloud call dressed up as optional.

**Each cloud provider also honors a `*_BASE_URL` override**
(`ANTHROPIC_BASE_URL`/`OPENAI_BASE_URL`/`GEMINI_BASE_URL`) — added for
a real reason (pointing at a self-hosted or OpenAI-compatible gateway,
which is common enough in practice) that happens to double as the only
seam this file's own tests need to exercise all four providers against
a local `httptest.Server` instead of spending real API credits or
requiring real keys for CI.

**Verified:**

1. Unit tests: one request/response test per provider, each pinned to
   that provider's own distinctive wire shape specifically (Anthropic's
   `x-api-key` + API-version headers; OpenAI's `Bearer` auth +
   `choices[].message.content`; Gemini's API key as a query parameter +
   `candidates[].content.parts[].text`; Ollama's no-auth request + flat
   `{"response": ...}`) — not just "some provider works," but that each
   one's actual, different shape is handled correctly. `selectLLMProvider`
   tested for: no env var → `(nil, nil)`; a real provider name with its
   key missing → an error; an unrecognized provider name → an error;
   and, for each of the four names, that the right concrete provider
   type comes back with the right config, including Gemini's
   `GOOGLE_API_KEY` fallback and Ollama's host-to-baseURL construction.
   `polishCaptionAsync` re-verified end to end through the real
   `selectLLMProvider` lookup (not a bypassed seam) via
   `LOOMLESS_LLM_PROVIDER=anthropic` + `ANTHROPIC_BASE_URL` pointed at a
   mock server, confirming the full chain from env-var selection through
   to `session.EditNote` still works after the refactor.
2. `go build/vet/gofmt` clean; cross-compiled clean for Linux.

## 35. TUI redesign: solid nvim-style status bar, live search, two-column detail view

Decided 2026-08-05. The TUI (`loomless tui`, decisions #23/#24) was a flat,
single-column session list with a plain-foreground status line and no
way to see anything about a session besides its name, step count, and
start time — reasonable for a first pass, but thin once someone
actually has more than a handful of sessions on disk (the developer
persona this whole project targets: someone who's recorded a dozen
half-finished debugging sessions and needs to tell them apart fast,
without opening each one in the browser editor just to check "did I
already export this one?").

**Dev/QA persona framing:** the person reaching for `loomless tui` instead
of `loomless export <name>` directly is specifically someone who *doesn't
remember the exact session name* or wants to browse — so the two things
worth optimizing for are (a) narrowing the list fast (search) and (b)
answering "what is this session, exactly" without leaving the list
(the detail panel: location, timing, step/screenshot counts, and
crucially per-format export status, so "have I already exported this"
is a glance instead of a `ls` in another terminal).

**Design decisions:**

- **A dedicated, always-visible search bar (`/` to focus), not
  bubbles/list's built-in filter overlay.** The built-in filter also
  binds `/` but renders as a transient overlay across the list itself;
  the user's ask was for a search box positioned *above* the list,
  visible at all times, which is a different UI, not a styling tweak —
  so `list.SetFilteringEnabled(false)` and a hand-rolled
  `textinput.Model` + `applyFilter()` (case-insensitive substring match
  against session names) replace it. `Esc` clears the query from
  anywhere in normal mode, not just while the search box itself is
  focused, since re-pressing `/` just to then press `Esc` to clear a
  filter that's visibly still active would be an odd extra hop.
- **Two-column layout via `lipgloss.JoinHorizontal`, with a real
  minimum-width fallback to single-column.** A fixed split would either
  waste space on a wide terminal or produce an unreadably thin detail
  panel on a narrow one; the left column is capped (not left to grow
  unbounded — it's a list of short names, not prose) and the whole
  two-column layout collapses back to the plain single-column list
  below a terminal-width threshold rather than rendering a panel too
  narrow to read anything in.
- **The detail panel is read-only context, not another editing
  surface.** Rename and the web editor already own "change this
  session"; the panel exists purely to answer "what is this session" —
  location (`session.Dir`), started/stopped timestamps, step count
  broken out by how many carry a screenshot, and export status per
  format checked directly against the filesystem (`os.Stat` on
  `<dir>/<name>.{md,pdf,html}`), not inferred from any in-memory flag,
  since the actual exported file on disk is the only thing that can't
  lie about whether an export happened.
- **An actual nvim-style status bar — solid `.Background()` fills, not
  just colored foreground text on the terminal default.** The previous
  status line was one `Foreground()`-only style; a real statusline look
  needs background-filled segments (a MODE block that recolors between
  NORMAL/SEARCH, a counts block, a background-filled spacer stretching
  to the right edge, and a right-aligned status/error segment) — this
  is what was explicitly asked for ("solid color proper structured"),
  and it's also just more scannable at a glance than a wall of
  same-colored text.
- **`allSessions` as the disk-backed source of truth, separate from
  what `list.Model` is currently displaying.** `refresh()` now updates
  `allSessions` and re-applies whatever search query is active, rather
  than calling `list.SetItems` directly — otherwise a background
  refresh (or the auto-refresh after create/rename/stop) would silently
  drop an in-progress search filter.
- **No changes to any existing method signature or behavior** —
  `export`, `openEditor`, `stopEditor`, `startInput`, `submitInput`,
  `stopRecording`, `refresh`, and the `status`/`statusIsError`/`list`/
  `editors`/`textInput`/`inputPurpose`/`renameTarget` fields are
  untouched, since `tui_test.go`'s existing 13 tests call these
  directly rather than driving the UI through simulated keypresses —
  the redesign is additive (new fields, new render methods, new key
  handling for search focus) specifically so none of that coverage
  needed to change.

**Verified:**

1. `go build/vet/gofmt` clean; all 13 pre-existing `tui_test.go` tests
   pass unmodified (confirming the redesign didn't change any tested
   method's behavior); full `go test ./...` clean.
2. Live verification in a real pty (`tmux`, since the TUI needs an
   actual terminal, not a piped one): built the binary, created three
   real sessions plus reused several pre-existing ones with real
   exported files on disk, then drove the running TUI via
   `tmux send-keys`/`capture-pane` — confirmed the two-column layout
   renders with real session data, `/` switches the status bar's mode
   block from NORMAL to SEARCH, typing a query live-filters the list
   and updates the counts segment (`7/7` → `1/7`) and the detail panel
   in the same keystroke, the detail panel's export checkmarks matched
   the real files present on disk (`test4` showed Markdown ✓/PDF ✓/
   Slides — matching what was actually exported earlier in this
   session), arrow-key navigation updates the detail panel to track
   the new selection, `Esc` clears an active filter and reverts the
   mode block to NORMAL, and resizing the pty down to 35 columns
   correctly collapsed to the single-column fallback without a crash
   or layout corruption.

## 36. Rename the product to Loomless

Decided 2026-08-05. Publishing this repo to GitHub surfaced that its
working name, "Story," was never actually the intended product name —
the user's real product name is **Loomless**. Renamed the whole
identity before publishing rather than after, since every public
surface (module path, GitHub repo, binary users type, MCP server
identity agents connect to) is far cheaper to get right before the
first external clone/star/import than to migrate later.

**What changed vs. what stayed on purpose:**

- `go.mod` module path: `github.com/noorshikalgar/story` →
  `github.com/noorshikalgar/loomless`, and every internal import updated
  to match.
- `cmd/story` → `cmd/loomless`; the binary/CLI command is now
  `loomless` (`go build -o loomless ./cmd/loomless`).
- The MCP server's own `Implementation.Name` is now `"loomless"`
  (was `"story"`).
- The local session-data root: `~/.story` → `~/.loomless`.
- Config env vars: `STORY_LLM_PROVIDER` → `LOOMLESS_LLM_PROVIDER`,
  `STORY_IDLE_WARN_AFTER`/`STORY_IDLE_CHECK_INTERVAL` →
  `LOOMLESS_IDLE_WARN_AFTER`/`LOOMLESS_IDLE_CHECK_INTERVAL` — these are
  part of the tool's public config surface (something a user or a CI
  config might set), not internal detail, so they needed to move too.
- Every doc (README, this decisions log, and `docs/product/*.md`),
  usage/help text, and error-message prefix that named the tool
  itself now says "Loomless"/`loomless`.
- **Deliberately NOT renamed:** the word "story" everywhere it's used
  as the generic noun for the *output artifact* the tool produces —
  "turn a working session into a readable story," the MCP tool named
  `generate_story`, doc comments like "the rendered story," "a repro
  story." The product is Loomless; what Loomless produces is still,
  correctly, called a story — that's the whole naming pun the tagline
  already relies on, and collapsing that distinction would have made
  the tagline read as a typo instead of a pun. Historical decision
  entries above this one were updated where they referenced now-renamed
  paths/commands (since e.g. `cmd/story/mcp.go` no longer exists and a
  stale reference would just be wrong), but arbitrary test-fixture
  strings from past verification runs (e.g. a Linux test window
  literally titled `story-linux-test-window`) were left as an accurate
  record of what was actually run at the time, not retroactively
  rebranded.
- **Also deliberately NOT renamed:** internal cgo C symbol names
  (`storyStartEventTap`, `storyElementAt`, etc. in the `_darwin.go`
  files) — these are private implementation-detail identifiers with
  zero user-visible surface, and touching cgo code without being able
  to re-verify the darwin build in this pass carried more risk than the
  purely cosmetic benefit was worth.

**Verified:**

1. `go build/vet/gofmt ./...` clean, full `go test ./...` clean,
   `GOOS=linux GOARCH=amd64` cross-compile clean.
2. Grepped the whole repo (`\bstory\b`/`\bStory\b`, case-sensitive and
   not) after the rename and manually classified every remaining hit —
   confirmed each one is the intentional generic-noun usage described
   above, not a missed identity reference.
3. Real end-to-end CLI smoke test against the rebuilt `loomless`
   binary: `loomless` with no args prints the renamed usage text;
   `loomless record start`/`loomless step`/`loomless record stop`/
   `loomless export` against a throwaway session confirmed data lands
   under `~/.loomless/sessions/<name>/` and exports correctly, then the
   throwaway session was deleted.

## GitHub remote

Decided 2026-08-03: staying local-only for now. `git init` is done, commits
are landing on `main` per [`../../CONTRIBUTING.md`](../../CONTRIBUTING.md).
Create the actual `github.com` remote (public, once there's real code)
when ready — `gh repo create` from an authenticated session.
