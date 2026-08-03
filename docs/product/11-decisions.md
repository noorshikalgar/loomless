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
Apply. Added `story edit`-scoped Undo/Redo/Reset per step, backed by
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
   fields committed together in one call. Ran `story export --format
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
possible" — `story record start <name> --highlight` (was
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

## 23. A real TUI (`story tui`) for session management

Decided 2026-08-03, direct request: *"make cli a proper TUI kind app,
where it can show list session shortcuts to edit etc fully functional
TUI app."* Decision #14 chose a browser-based editor over a TUI, but
that decision was specifically about the *visual editing* work
(redaction, annotations) needing a canvas a terminal can't give you —
it never claimed a TUI has no place in this project. Session
*management* (which sessions exist, how big are they, jump into
editing/exporting one) is exactly the kind of fast, keyboard-driven
overview a TUI is good at, so `story tui` is additive: a third way to
use Story alongside the scriptable CLI/MCP surface and the existing
`story edit` web UI, not a replacement for either.

**Stack**: `github.com/charmbracelet/bubbletea` + `bubbles/list` +
`lipgloss` — the de facto standard Go TUI toolkit (pure Go, no cgo,
actively maintained, used by a huge fraction of the Go TUI ecosystem),
chosen over hand-rolling raw ANSI/terminal handling for the same reason
`goldmark`/`fpdf` were chosen over hand-rolled alternatives elsewhere in
this project: a well-solved problem beats a partially-solved one.

**New primitive**: `session.List()` — reads every directory under
`~/.story/sessions`, loads each `session.json`, sorts by `StartedAt`
descending. A session directory that fails to load (corrupted or
mid-write `session.json`) is skipped rather than failing the whole
listing — one bad session shouldn't hide every other one.

**Scope of this first pass**: list sessions (name, step count, started
time, a "● recording" marker for any session whose `StoppedAt` is still
nil), open the selected session's existing local web editor (`e`/enter —
reuses `internal/editor.NewHandler` exactly as `story edit` does,
listening on `127.0.0.1:0` and opening the system browser), export to
any format (`m`/`p`/`s` — reuses `internal/engine`'s existing
`ExportMarkdown`/`ExportPDF`/`ExportSlides`, the same functions the
plain CLI calls), and refresh (`r`, since sessions can be created by a
concurrently-running `story record start` or MCP client). Deliberately
**not** in this pass: starting/stopping a recording from the TUI —
`engine.StopRecording()` only operates on the single process-wide
"current" session pointer `story record start` maintains, not an
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
do the same synchronous disk I/O the plain `story export` CLI already
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

`openBrowser` (previously a plain top-level func in `cmd/story/main.go`,
shared with the existing `story edit` command) became a package-level
`var` purely so tests could swap in a no-op recorder — otherwise every
test run of the "open editor" action would pop open a real browser
window.

**Verified:**

1. Unit tests (`cmd/story/tui_test.go`): session listing count and
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

## GitHub remote

Decided 2026-08-03: staying local-only for now. `git init` is done, commits
are landing on `main` per [`../../CONTRIBUTING.md`](../../CONTRIBUTING.md).
Create the actual `github.com` remote (public, once there's real code)
when ready — `gh repo create` from an authenticated session.
