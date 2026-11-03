# Installing Loomless

## Prerequisites

- Go 1.26+ (see `go.mod`)
- **macOS:** nothing else — screenshotting and window info use tools
  already on the system (`screencapture`, `osascript`).
- **Linux (X11 only, no Wayland):** `xdotool`, `wmctrl`, `xprop` (usually
  preinstalled) and [`maim`](https://github.com/naelstrof/maim) for
  screenshots — `apt install xdotool wmctrl x11-utils maim`. Linux has no
  OS permission prompts at all (see below) — it just needs these tools on
  `PATH`. Automatic click-capture isn't implemented on Linux yet; use
  `loomless step`/`loomless run` there.

## Build

```bash
go build -o loomless ./cmd/loomless
# optional: put it on your PATH
sudo mv loomless /usr/local/bin/
```

## Desktop app (macOS)

For people who'd rather not touch a terminal, build `Loomless.app` — a
double-clickable app that opens the dashboard in its own window:

```bash
packaging/macos/build-app.sh             # → dist/Loomless.app
UNIVERSAL=1 packaging/macos/build-app.sh # Apple Silicon + Intel in one app
```

Drag `dist/Loomless.app` into `/Applications` and open it. The same
binary still works as the CLI (`Loomless.app/Contents/MacOS/loomless
record start …`), and `loomless app` opens the window from a terminal.

Because the app itself runs the click capture, macOS asks for the
permissions below **for Loomless**, not for Terminal. The build is
ad-hoc signed: fine on the Mac that built it, but on any other Mac
Gatekeeper blocks it until it's Developer ID signed and notarized (see
PUBLISHING.md). Rebuilding changes the ad-hoc signature, so macOS may
ask for the permissions again after each rebuild.

Anything the app would have printed goes to `~/.loomless/app.log`.

## Desktop app (Windows)

Built from any OS — the Windows build is pure Go, no Windows machine or
C compiler needed:

```bash
packaging/windows/build.sh              # → dist/windows-amd64/
ARCH=arm64 packaging/windows/build.sh   # Windows on ARM
```

Copy the folder to the PC and double-click **`Loomless.exe`**: the
dashboard opens in its own window. `loomless-cli.exe` is the same
program for the command line (`loomless-cli record start …`). Every CI
run also attaches both, ready to download, under the run's Artifacts.

- No permission prompts — Windows doesn't gate click capture or
  screenshots. Clicks inside apps running **as administrator** aren't
  seen unless Loomless runs as administrator too.
- The window uses Edge WebView2, built into Windows 11 and most updated
  Windows 10 PCs. Without it, Loomless opens in the default browser.
- The `.exe` isn't code-signed yet, so SmartScreen shows "Windows
  protected your PC" on first launch: **More info → Run anyway**.
- Recordings live in `%USERPROFILE%\.loomless`; the app log is
  `%USERPROFILE%\.loomless\app.log`.

## macOS permissions — what each one is for, and what happens without it

Loomless asks macOS for two permissions the first time it needs them.
**Both are optional** — deny either (or never grant it) and the tool
keeps working, just with less automatically filled in. Nothing here is a
one-time irreversible grant either; toggle them anytime in System
Settings.

### Accessibility

**Why:** two separate things sit behind this one permission —
1. Knowing which app/window you're in. `loomless step` and every window
   capture ask macOS's System Events for the frontmost window's app name
   and title, and System Events scripting is gated behind Accessibility
   regardless of what you're actually asking it for.
2. Automatic click-capture (`--highlight`, `--auto-caption`, or just
   `record start` watching your clicks). This uses a macOS **event tap**
   in **listen-only mode** — it observes clicks, never intercepts or
   modifies them — but macOS gates event taps behind Accessibility even
   in listen-only mode, since the API *could* be used to intercept input.

**Without it:** steps still save — just without an app/window header, and
automatic click-capture won't start (you'll see a one-line warning, not a
failure). Use `loomless step "note"` / `loomless run -- <cmd>` for manual
capture instead.

**Grant it:** System Settings → Privacy & Security → Accessibility →
enable the terminal app you run `loomless` from (Terminal, iTerm2, etc.).

### Screen Recording

**Why:** the actual screenshot for a step is taken with macOS's own
`screencapture` tool. Since macOS Catalina, `screencapture` requires
Screen Recording permission for *any* capture — including a single
window's region, not just full-screen.

**Without it:** steps still save with their note and window info, just no
screenshot image.

**Grant it:** System Settings → Privacy & Security → Screen Recording →
enable the terminal app you run `loomless` from.

### The fully permission-free path

`loomless run -- <command>` (terminal capture) needs **neither**
permission — it just runs the command and captures its stdout/stderr as
a text step. If you'd rather not grant anything, you can still produce a
complete story from `loomless run` + `loomless step` alone.

## Agent-driven capture (MCP)

```bash
claude mcp add loomless -- /path/to/loomless mcp
```

Same permission rules apply — macOS gates the *process*, so whichever app
launches the MCP server (e.g. your terminal, or Claude Code itself) is
the one that needs Accessibility/Screen Recording granted, same as above.

## Verify it works

```bash
./loomless record start smoke-test
./loomless step "hello"
./loomless record stop
./loomless export smoke-test
cat ~/.loomless/sessions/smoke-test/smoke-test.md
```
