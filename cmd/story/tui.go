package main

import (
	"fmt"
	"net"
	"net/http"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/noorshikalgar/story/internal/editor"
	"github.com/noorshikalgar/story/internal/engine"
	"github.com/noorshikalgar/story/internal/session"
)

// cmdTUI is the "story tui" entrypoint — a session-management dashboard
// (list sessions, jump into the local web editor, export, refresh)
// alongside the existing scriptable CLI/MCP surface, not a replacement
// for either. Decision #14 chose a browser-based editor over a TUI
// specifically for the visual editing work (redaction, annotations —
// things that need a canvas); that reasoning doesn't apply to session
// *management*, which is exactly what a fast, keyboard-driven list is
// good at. See decision #23.
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

type tuiModel struct {
	list          list.Model
	editors       map[string]*runningEditor
	status        string
	statusIsError bool
	width, height int
}

func newTUIModel() (tuiModel, error) {
	sessions, err := session.List()
	if err != nil {
		return tuiModel{}, err
	}
	l := list.New(sessionItems(sessions), list.NewDefaultDelegate(), 0, 0)
	l.Title = "Story sessions"
	l.SetStatusBarItemName("session", "sessions")
	return tuiModel{list: l, editors: make(map[string]*runningEditor)}, nil
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
		// Reserve one line for the status bar below the list.
		m.list.SetSize(msg.Width, msg.Height-1)
		return m, nil

	case tea.KeyMsg:
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

func (m tuiModel) View() string {
	statusStyle := lipgloss.NewStyle().Padding(0, 1)
	if m.statusIsError {
		statusStyle = statusStyle.Foreground(lipgloss.Color("196"))
	} else {
		statusStyle = statusStyle.Foreground(lipgloss.Color("242"))
	}
	status := m.status
	if status == "" {
		status = "enter/e edit · m/p/s export md/pdf/slides · r refresh · q quit"
	}
	return m.list.View() + "\n" + statusStyle.Render(status)
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
	go srv.Serve(ln) //nolint:errcheck // srv.Close() on quit/exit always returns http.ErrServerClosed, nothing actionable
	m.editors[name] = &runningEditor{url: url, server: srv}
	openBrowser(url)
	m.setStatus("Editor for "+name+" running at "+url, false)
}

func (m *tuiModel) closeAllEditors() {
	for _, e := range m.editors {
		e.server.Close()
	}
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

func (m *tuiModel) refresh() {
	sessions, err := session.List()
	if err != nil {
		m.setStatus(err.Error(), true)
		return
	}
	m.list.SetItems(sessionItems(sessions))
	m.setStatus("Refreshed ("+fmt.Sprint(len(sessions))+" session(s))", false)
}
