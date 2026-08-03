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
or from a `story windows list` picker for a specific one. No CGO/Accessibility
global-hook dependency needed for this — keeps the Phase 0 spike buildable
without extra OS permission prompts beyond Screen Recording.

**Unblocks:** Phase 0 capture engine scope — see
[`../../CONTRIBUTING.md`](../../CONTRIBUTING.md) for where this lands
(`feature/phase-0-capture-engine`).

## 6. Terminal capture scope — **explicit wrapper, not full shell session**

Decided 2026-08-03 (resolves open question #8). `story run -- <command>`
runs one command as a subprocess and captures its output as a step —
it does not attach to or tee the user's actual interactive shell.

Consistent with every capture-surface decision so far (window-scoped not
full-screen, listen-only click tap, optional accounts): default to the
narrower, explicit surface. A full-session shell wrapper would capture
every command the user runs for the entire recording — including ones
that have nothing to do with the story and are more likely to contain
secrets never meant for the output. An explicit per-command wrapper only
captures what's deliberately routed through it, at the cost of the user
needing to type `story run --` in front of commands they want captured.

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
and future capture path (manual step, click daemon, `story run`, and the
Phase 1.5 MCP server later) gets it automatically. Covers: step notes,
window titles, terminal commands, and terminal output.

**Scope, deliberately narrow:** text only. Screenshot pixels are not
scanned — per `07-edge-cases.md`, reliable secret detection in arbitrary
pixel content needs OCR, which is real Phase 2 "auto-blur" scope, not a
Phase 1 pattern-match. And masking only governs what Story *saves*: a
command run via `story run` still prints its real, unmasked output live to
your actual terminal exactly as it would without Story — masking can't
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

Decided 2026-08-03 (Phase 1.5, per `09-roadmap.md`). `story mcp` runs an
MCP server over stdio exposing `start_recording`, `capture_step`,
`stop_recording`, `generate_story` — the exact 4 named in
`03-features.md`/`04-usage-scenarios.md` Scenario B — plus two more:

- `run_command`: the agent-facing equivalent of `story run`. Scenario B's
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

Implementation: `cmd/story/mcp.go` + a new `internal/engine` package.
Business logic (session lifecycle, click-daemon start/stop, step capture,
command execution, export) moved out of `cmd/story/main.go` into
`internal/engine`, shared unchanged between the CLI and the MCP server —
per `08-technical-considerations.md`'s "thin wrapper around the same
engine's internal API." One hard invariant enforced there: nothing in
`internal/engine` writes to `os.Stdout` — the MCP stdio transport uses
stdout as the protocol stream, so a stray print would corrupt it. The
CLI's live terminal echo for `story run` is a tee the CLI adds on top
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
uses. `story step`/`capture_step` and `story run`/`run_command` (manual
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
The actual cross-compiled Linux `story` binary was copied in and run
directly against that live session — not a simulation.

This caught two real bugs that cross-compilation alone would never have
surfaced:

1. `windowInfoForID` called `xdotool getwindowclassname`, which doesn't
   exist — confirmed against `xdotool help`'s actual command list. Fixed
   by using `xprop -id <id> WM_CLASS` instead (ICCCM's real mechanism for
   this) and parsing its `"instance", "Class"` output.
2. Independently, and worse: when window resolution failed (which it did,
   because of bug #1), `story step` reported success with **no
   indication anything had gone wrong** — no window, no screenshot, no
   warning. Traced to `engine.CaptureStep` (`internal/engine/engine.go`),
   which silently discarded the resolution/screenshot error
   (`werr`/`capture.Screenshot`'s error) instead of surfacing it, a
   regression introduced during the Phase 1.5 engine refactor (decision
   #8) — the pre-refactor CLI code had printed this warning; the
   extraction into `internal/engine` dropped it. Fixed by giving
   `CaptureStep` the same `(step, warning, err)` shape `StartRecording`
   already uses, threaded through both the CLI's `story step` and the MCP
   `capture_step` tool's output.

Bug #2 is the more important finding: it means a capture failure could
have been silently invisible on **any** platform, not just Linux — this
project's own dogfooding on macOS just hadn't hit a window-resolution
failure recently enough to notice, since Accessibility permission was
already granted from earlier testing. Re-verified end to end after both
fixes: `story windows list`, `record start` (correctly warns + falls back
to manual since click-capture isn't implemented), `story step` (now shows
`(XTerm — story-linux-test-window)` and saves a real screenshot), `story
run`, `record stop`, and `export` all confirmed working against the live
container session — screenshot bytes pulled out and visually inspected,
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
`STORY_IDLE_WARN_AFTER`/`STORY_IDLE_CHECK_INTERVAL` env vars (Go duration
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

Decided 2026-08-03 (second Phase 2 item). `story export <name> --format
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
`story export <name> --format slides` and the MCP `generate_story` tool's
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
a separate follow-up, not included). `story edit <name>` runs a
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
turn it off. `story record start <name> --no-highlight` and the MCP
`start_recording` tool's `no_highlight: true` disable it — the daemon
still captures a screenshot per click, just without the ring drawn on it.
Default stays on (opt out, not opt in), since the annotation is more
often useful than not for the primary bug-repro/PR-review use cases.

Plumbing: the click daemon is a separate relaunched process (`story
__click-daemon <name>`), so the flag has to travel as a real process
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
`story edit` — so they shipped together rather than as separate passes.

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

## GitHub remote

Decided 2026-08-03: staying local-only for now. `git init` is done, commits
are landing on `main` per [`../../CONTRIBUTING.md`](../../CONTRIBUTING.md).
Create the actual `github.com` remote (public, once there's real code)
when ready — `gh repo create` from an authenticated session.
