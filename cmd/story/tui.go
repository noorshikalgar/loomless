package main

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/noorshikalgar/story/internal/editor"
	"github.com/noorshikalgar/story/internal/engine"
	"github.com/noorshikalgar/story/internal/session"
)

// startRecording is a package-level var (not a direct call to
// engine.StartRecording), same reasoning as openBrowser: it's the one
// TUI action that spawns a real background OS process (the click
// daemon), which tests must be able to substitute away rather than
// actually forking one on every "create a new session" test run.
var startRecording = engine.StartRecording

// cmdTUI is the "story tui" entrypoint — a session-management dashboard
// (list sessions, start/stop recordings, jump into the local web editor,
// export, rename, refresh) alongside the existing scriptable CLI/MCP
// surface, not a replacement for either. Decision #14 chose a
// browser-based editor over a TUI specifically for the visual editing
// work (redaction, annotations — things that need a canvas); that
// reasoning doesn't apply to session *management*, which is exactly what
// a fast, keyboard-driven list is good at. See decisions #23/#24.
func cmdTUI(args []string) error {
	m, err := newTUIModel()
	if err != nil {
		return err
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// sessionItem adapts session.Session to bubbles/list's DefaultItem
// interface — title + description strings, computed once per List()
// refresh rather than on every render.
type sessionItem struct {
	s session.Session
}

func (i sessionItem) Title() string {
	if i.s.StoppedAt == nil {
		return i.s.Name + "  ● recording"
	}
	return i.s.Name
}

func (i sessionItem) Description() string {
	return fmt.Sprintf("%d step(s) — started %s", len(i.s.Steps), i.s.StartedAt.Format("2006-01-02 15:04"))
}

func (i sessionItem) FilterValue() string { return i.s.Name }

// runningEditor tracks one session's already-open local editor server —
// pressing 'e' again on the same session reopens the browser tab instead
// of binding a second server to a second port.
type runningEditor struct {
	url    string
	server *http.Server
}

// inputPurpose distinguishes what a currently-open text prompt is for,
// since "new session" and "rename session" share the same one-line
// textinput UI but submit to different places.
type inputPurpose int

const (
	inputNone inputPurpose = iota
	inputNewSession
	inputRenameSession
)

type tuiModel struct {
	list          list.Model
	editors       map[string]*runningEditor
	status        string
	statusIsError bool
	width, height int

	textInput    textinput.Model
	inputPurpose inputPurpose
	renameTarget string // session being renamed, captured when the prompt opens (the list's selection can't be trusted to still point at it once the name changes)
}

func newTUIModel() (tuiModel, error) {
	sessions, err := session.List()
	if err != nil {
		return tuiModel{}, err
	}
	l := list.New(sessionItems(sessions), list.NewDefaultDelegate(), 0, 0)
	l.Title = "Story sessions"
	l.SetStatusBarItemName("session", "sessions")
	ti := textinput.New()
	ti.CharLimit = 64
	return tuiModel{list: l, editors: make(map[string]*runningEditor), textInput: ti}, nil
}

func sessionItems(sessions []session.Session) []list.Item {
	items := make([]list.Item, len(sessions))
	for i, s := range sessions {
		items[i] = sessionItem{s: s}
	}
	return items
}

func (m tuiModel) Init() tea.Cmd { return nil }

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		// Reserve two lines for the footer below the list: a status/
		// prompt line and a shortcut-legend line, always both present so
		// the list's height never has to shift between modes.
		m.list.SetSize(msg.Width, msg.Height-2)
		return m, nil

	case tea.KeyMsg:
		if m.inputPurpose != inputNone {
			switch msg.String() {
			case "enter":
				m.submitInput()
				return m, nil
			case "esc":
				m.cancelInput()
				return m, nil
			}
			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		}

		if m.list.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "ctrl+c", "q":
			m.closeAllEditors()
			return m, tea.Quit
		case "enter", "e":
			m.openEditor()
			return m, nil
		case "x":
			m.stopEditor()
			return m, nil
		case "S":
			m.stopRecording()
			return m, nil
		case "n":
			return m, m.startInput(inputNewSession)
		case "c":
			return m, m.startInput(inputRenameSession)
		case "m":
			m.export("md")
			return m, nil
		case "p":
			m.export("pdf")
			return m, nil
		case "s":
			m.export("slides")
			return m, nil
		case "r":
			m.refresh()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

var (
	statusOKStyle    = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("242"))
	statusErrStyle   = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("196"))
	legendStyle      = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("59"))
	promptLabelStyle = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("228")).Bold(true)
)

const (
	listLegend  = "enter/e edit · x stop editor · S stop rec · n new · c rename · m/p/s export · r refresh · q quit"
	inputLegend = "enter confirm · esc cancel"
)

func (m tuiModel) View() string {
	if m.inputPurpose != inputNone {
		label := "New session name:"
		if m.inputPurpose == inputRenameSession {
			label = "Rename " + m.renameTarget + " to:"
		}
		prompt := promptLabelStyle.Render(label) + " " + m.textInput.View()
		return m.list.View() + "\n" + prompt + "\n" + legendStyle.Render(inputLegend)
	}

	status := m.status
	style := statusOKStyle
	if m.statusIsError {
		style = statusErrStyle
	}
	if status == "" {
		running := 0
		for _, i := range m.list.Items() {
			if it, ok := i.(sessionItem); ok && it.s.StoppedAt == nil {
				running++
			}
		}
		status = fmt.Sprintf("%d session(s) · %d editor(s) running · %d recording", len(m.list.Items()), len(m.editors), running)
	}
	return m.list.View() + "\n" + style.Render(status) + "\n" + legendStyle.Render(listLegend)
}

func (m *tuiModel) selected() (session.Session, bool) {
	item, ok := m.list.SelectedItem().(sessionItem)
	if !ok {
		return session.Session{}, false
	}
	return item.s, true
}

func (m *tuiModel) setStatus(text string, isError bool) {
	m.status = text
	m.statusIsError = isError
}

// startInput opens the one-line text prompt for either creating a new
// session or renaming the selected one — same widget, different
// pre-filled value and submit target (see submitInput). Renaming with
// nothing selected (an empty list) is simply a no-op; creating a session
// never needed a selection to begin with.
func (m *tuiModel) startInput(purpose inputPurpose) tea.Cmd {
	if purpose == inputRenameSession {
		sel, ok := m.selected()
		if !ok {
			return nil
		}
		m.renameTarget = sel.Name
		m.textInput.SetValue(sel.Name)
		m.textInput.CursorEnd()
	} else {
		m.textInput.SetValue("")
	}
	m.inputPurpose = purpose
	return m.textInput.Focus()
}

func (m *tuiModel) cancelInput() {
	m.inputPurpose = inputNone
	m.renameTarget = ""
	m.textInput.Blur()
}

func (m *tuiModel) submitInput() {
	name := strings.TrimSpace(m.textInput.Value())
	purpose := m.inputPurpose
	target := m.renameTarget
	m.cancelInput()

	if name == "" {
		m.setStatus("Session name cannot be empty", true)
		return
	}

	switch purpose {
	case inputNewSession:
		_, watching, warning, err := startRecording(name, false)
		if err != nil {
			m.setStatus(err.Error(), true)
			return
		}
		m.refresh()
		if !watching {
			m.setStatus("Created "+name+" (manual capture only: "+warning+")", true)
			return
		}
		m.setStatus("Recording "+name, false)

	case inputRenameSession:
		if _, err := session.Rename(target, name); err != nil {
			m.setStatus(err.Error(), true)
			return
		}
		m.refresh()
		m.setStatus("Renamed "+target+" to "+name, false)
	}
}

// openEditor starts (or reuses) the selected session's local web editor.
// net.Listen and editor.NewHandler are fast, local, non-blocking calls —
// this runs synchronously on the TUI's single event-loop goroutine
// rather than through a tea.Cmd, which keeps m.editors free of any
// concurrent-map-write hazard (a tea.Cmd's function body runs on its own
// goroutine; mutating shared model state from there without routing
// through a Msg would be a real data race, not just a style choice).
// Only the server's blocking Serve loop itself gets its own goroutine.
func (m *tuiModel) openEditor() {
	sel, ok := m.selected()
	if !ok {
		return
	}
	name := sel.Name

	if existing, ok := m.editors[name]; ok {
		openBrowser(existing.url)
		m.setStatus("Reopened editor for "+name+" at "+existing.url, false)
		return
	}

	handler, err := editor.NewHandler(name)
	if err != nil {
		m.setStatus(err.Error(), true)
		return
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		m.setStatus(err.Error(), true)
		return
	}
	url := fmt.Sprintf("http://%s/", ln.Addr().String())
	srv := &http.Server{Handler: handler}
	go srv.Serve(ln) //nolint:errcheck // srv.Close() on quit/stopEditor always returns http.ErrServerClosed, nothing actionable
	m.editors[name] = &runningEditor{url: url, server: srv}
	openBrowser(url)
	m.setStatus("Editor for "+name+" running at "+url, false)
}

// stopEditor closes the selected session's running editor server, if
// any — the counterpart to openEditor that was previously only possible
// by quitting the whole TUI.
func (m *tuiModel) stopEditor() {
	sel, ok := m.selected()
	if !ok {
		return
	}
	name := sel.Name
	e, ok := m.editors[name]
	if !ok {
		m.setStatus("No running editor for "+name, true)
		return
	}
	e.server.Close()
	delete(m.editors, name)
	m.setStatus("Stopped editor for "+name, false)
}

func (m *tuiModel) closeAllEditors() {
	for _, e := range m.editors {
		e.server.Close()
	}
}

// stopRecording ends the selected session's recording (click daemon +
// idle watchdog) regardless of whether it's the single global "current"
// session engine.StopRecording targets — see engine.StopRecordingNamed.
func (m *tuiModel) stopRecording() {
	sel, ok := m.selected()
	if !ok {
		return
	}
	if sel.StoppedAt != nil {
		m.setStatus(sel.Name+" is not recording", true)
		return
	}
	if _, err := engine.StopRecordingNamed(sel.Name); err != nil {
		m.setStatus(err.Error(), true)
		return
	}
	m.refresh()
	m.setStatus("Stopped recording "+sel.Name, false)
}

// export renders the selected session in the given format synchronously
// — export is normal disk I/O over a handful of already-captured PNGs,
// the same work "story export" already does synchronously on the plain
// CLI with no progress indicator, so doing it inline here (rather than
// building out an async progress-spinner Cmd) is consistent with that
// and keeps the TUI's concurrency model simple for this first pass.
func (m *tuiModel) export(format string) {
	sel, ok := m.selected()
	if !ok {
		return
	}
	var path string
	var err error
	switch format {
	case "md":
		path, _, err = engine.ExportMarkdown(sel.Name)
	case "pdf":
		path, err = engine.ExportPDF(sel.Name)
	case "slides":
		path, err = engine.ExportSlides(sel.Name)
	}
	if err != nil {
		m.setStatus(err.Error(), true)
		return
	}
	m.setStatus("Exported to "+path, false)
}

// refresh re-lists sessions from disk and clears any prior status
// message — the default (empty-status) view already shows live session/
// editor/recording counts, so a bare refresh needs no separate
// confirmation text of its own. Callers that follow refresh() with their
// own setStatus (export, rename, stop) simply overwrite this.
func (m *tuiModel) refresh() {
	sessions, err := session.List()
	if err != nil {
		m.setStatus(err.Error(), true)
		return
	}
	m.list.SetItems(sessionItems(sessions))
	m.setStatus("", false)
}
