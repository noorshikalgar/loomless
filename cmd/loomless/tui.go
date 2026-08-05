package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/noorshikalgar/loomless/internal/editor"
	"github.com/noorshikalgar/loomless/internal/engine"
	"github.com/noorshikalgar/loomless/internal/session"
)

// startRecording is a package-level var (not a direct call to
// engine.StartRecording), same reasoning as openBrowser: it's the one
// TUI action that spawns a real background OS process (the click
// daemon), which tests must be able to substitute away rather than
// actually forking one on every "create a new session" test run.
var startRecording = engine.StartRecording

// cmdTUI is the "loomless tui" entrypoint — a session-management dashboard
// (list sessions, start/stop recordings, jump into the local web editor,
// export, rename, refresh) alongside the existing scriptable CLI/MCP
// surface, not a replacement for either. Decision #14 chose a
// browser-based editor over a TUI specifically for the visual editing
// work (redaction, annotations — things that need a canvas); that
// reasoning doesn't apply to session *management*, which is exactly what
// a fast, keyboard-driven list is good at. See decisions #23/#24/#35.
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
	// allSessions is the unfiltered source of truth, refreshed from disk
	// by refresh() — m.list.Items() is always a view derived from this
	// (via applyFilter), never mutated directly, so a search query and a
	// disk refresh can never disagree about what the full session set is.
	allSessions   []session.Session
	list          list.Model
	editors       map[string]*runningEditor
	status        string
	statusIsError bool
	width, height int

	textInput    textinput.Model
	inputPurpose inputPurpose
	renameTarget string // session being renamed, captured when the prompt opens (the list's selection can't be trusted to still point at it once the name changes)

	// searchInput drives a persistent search bar above the list
	// (decision #35) — deliberately NOT bubbles/list's own built-in
	// filter (which lives inside the list widget and claims '/' itself),
	// since the layout this replaces asked for a distinct, always-visible
	// search bar rather than an overlay. searchFocused gates whether
	// keystrokes go to the search box (live-filtering as you type) or to
	// the normal single-key shortcuts — without that gate, typing "n" to
	// search for a session named "nginx-fix" would instead trigger "new
	// session" on the very first keystroke.
	searchInput   textinput.Model
	searchFocused bool
}

func newTUIModel() (tuiModel, error) {
	sessions, err := session.List()
	if err != nil {
		return tuiModel{}, err
	}
	l := list.New(sessionItems(sessions), list.NewDefaultDelegate(), 0, 0)
	l.Title = "Sessions"
	// This TUI's own search bar and status/legend lines replace list's
	// built-in equivalents — showing both would be redundant chrome
	// competing for the same narrow left column.
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)

	ti := textinput.New()
	ti.CharLimit = 64

	search := textinput.New()
	search.Placeholder = "Search sessions…"
	search.Prompt = "/ "
	search.CharLimit = 64

	m := tuiModel{
		allSessions: sessions,
		list:        l,
		editors:     make(map[string]*runningEditor),
		textInput:   ti,
		searchInput: search,
	}
	return m, nil
}

func sessionItems(sessions []session.Session) []list.Item {
	items := make([]list.Item, len(sessions))
	for i, s := range sessions {
		items[i] = sessionItem{s: s}
	}
	return items
}

// applyFilter rebuilds m.list's items from m.allSessions, restricted to
// sessions whose name contains the current search query
// (case-insensitive substring — this is a dashboard for a handful to a
// few dozen local sessions, not a corpus that needs fuzzy ranking). An
// empty query is the common case and just shows everything.
func (m *tuiModel) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(m.searchInput.Value()))
	if query == "" {
		m.list.SetItems(sessionItems(m.allSessions))
		return
	}
	filtered := make([]session.Session, 0, len(m.allSessions))
	for _, s := range m.allSessions {
		if strings.Contains(strings.ToLower(s.Name), query) {
			filtered = append(filtered, s)
		}
	}
	m.list.SetItems(sessionItems(filtered))
}

func (m tuiModel) Init() tea.Cmd { return nil }

// reservedRows is how much vertical space the chrome around the list
// takes: one line for the search bar, one for the nvim-style status
// bar, one for the shortcut legend beneath it.
const reservedRows = 3

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		leftWidth, _ := m.columnWidths()
		m.list.SetSize(leftWidth, msg.Height-reservedRows)
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

		if m.searchFocused {
			switch msg.String() {
			case "enter":
				// Keep whatever's typed, hand keyboard focus back to the
				// list — the filtered results stay filtered until the
				// query is cleared (Esc, or editing it again via '/').
				m.searchFocused = false
				m.searchInput.Blur()
				return m, nil
			case "esc":
				m.searchInput.SetValue("")
				m.searchFocused = false
				m.searchInput.Blur()
				m.applyFilter()
				return m, nil
			}
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			m.applyFilter()
			return m, cmd
		}

		switch msg.String() {
		case "ctrl+c", "q":
			m.closeAllEditors()
			return m, tea.Quit
		case "/":
			m.searchFocused = true
			return m, m.searchInput.Focus()
		case "esc":
			// Clear an already-applied search filter even when the bar
			// itself isn't focused right now — otherwise clearing it
			// again needs '/' then Esc instead of just Esc.
			if m.searchInput.Value() != "" {
				m.searchInput.SetValue("")
				m.applyFilter()
			}
			return m, nil
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

// Palette for the nvim-style status bar: solid background blocks, not
// just colored text on the terminal's default background — the mode
// segment's color changes with the current interaction mode (normal vs.
// search), the same way an editor statusline recolors its mode
// indicator between Normal/Insert/Visual.
var (
	modeNormalStyle = lipgloss.NewStyle().Bold(true).Padding(0, 1).
			Background(lipgloss.Color("62")).Foreground(lipgloss.Color("230"))
	modeSearchStyle = lipgloss.NewStyle().Bold(true).Padding(0, 1).
			Background(lipgloss.Color("214")).Foreground(lipgloss.Color("235"))
	statusInfoStyle = lipgloss.NewStyle().Padding(0, 1).
			Background(lipgloss.Color("236")).Foreground(lipgloss.Color("252"))
	statusErrStyle = lipgloss.NewStyle().Bold(true).Padding(0, 1).
			Background(lipgloss.Color("196")).Foreground(lipgloss.Color("230"))
	statusFillStyle = lipgloss.NewStyle().Background(lipgloss.Color("236"))

	legendStyle      = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("59"))
	promptLabelStyle = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("228")).Bold(true)

	searchBarStyle        = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("245"))
	searchBarFocusedStyle = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("230")).Bold(true)

	detailBorderStyle = lipgloss.NewStyle().Padding(0, 2).BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("59")).BorderLeft(true).
				BorderTop(false).BorderRight(false).BorderBottom(false)
	detailTitleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("228"))
	detailLabelStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	detailMutedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Italic(true)
	detailRecordingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	exportYesStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))
	exportNoStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
)

const listLegend = "/ search · enter/e edit · x stop editor · S stop rec · n new · c rename · m/p/s export · r refresh · q quit"

// columnWidths splits the terminal into a left session-list column and
// a right detail-panel column. The left column is capped rather than
// left to grow with the terminal — a session list is a list of short
// names, not prose, so a very wide left column mostly just wastes space
// the detail panel could use instead; the right column always gets
// whatever's left; a minimum on the right column stops the split from
// producing an unusably narrow detail panel on a small terminal.
func (m tuiModel) columnWidths() (left, right int) {
	left = m.width * 2 / 5
	if left > 42 {
		left = 42
	}
	if left < 20 {
		left = m.width
	}
	right = m.width - left
	if right < 24 {
		right = 0 // terminal too narrow for two columns at all
	}
	return left, right
}

func (m tuiModel) View() string {
	if m.inputPurpose != inputNone {
		label := "New session name:"
		if m.inputPurpose == inputRenameSession {
			label = "Rename " + m.renameTarget + " to:"
		}
		prompt := promptLabelStyle.Render(label) + " " + m.textInput.View()
		return m.list.View() + "\n" + prompt + "\n" + legendStyle.Render(inputLegend)
	}

	searchBar := m.renderSearchBar()
	body := m.renderBody()
	statusBar := m.renderStatusBar()
	legend := legendStyle.Render(listLegend)
	return searchBar + "\n" + body + "\n" + statusBar + "\n" + legend
}

const inputLegend = "enter confirm · esc cancel"

func (m tuiModel) renderSearchBar() string {
	style := searchBarStyle
	if m.searchFocused {
		style = searchBarFocusedStyle
	}
	return style.Width(m.width).Render(m.searchInput.View())
}

// renderBody lays out the session list and the detail panel side by
// side — falling back to just the list, full width, on a terminal too
// narrow for a usable two-column split (columnWidths' own right<24
// signal), rather than rendering a detail panel too cramped to read.
func (m tuiModel) renderBody() string {
	leftWidth, rightWidth := m.columnWidths()
	if rightWidth == 0 {
		return m.list.View()
	}
	listView := lipgloss.NewStyle().Width(leftWidth).Render(m.list.View())
	detail := detailBorderStyle.Width(rightWidth).Height(m.list.Height()).Render(m.renderDetail())
	return lipgloss.JoinHorizontal(lipgloss.Top, listView, detail)
}

// renderStatusBar builds the nvim-style line: a solid mode block, a
// solid counts block, a filled-background middle, and (when there's
// something to say) a right-aligned status/error block — replacing the
// old plain single-color text line with actual background-filled
// segments, the same visual language an editor statusline uses.
func (m tuiModel) renderStatusBar() string {
	mode, modeStyle := "NORMAL", modeNormalStyle
	if m.searchFocused {
		mode, modeStyle = "SEARCH", modeSearchStyle
	}
	modeSeg := modeStyle.Render(mode)

	running := 0
	for _, s := range m.allSessions {
		if s.StoppedAt == nil {
			running++
		}
	}
	shown := len(m.list.Items())
	countsText := fmt.Sprintf(" %d/%d sessions · %d editor(s) · %d recording ", shown, len(m.allSessions), len(m.editors), running)
	countsSeg := statusInfoStyle.Render(countsText)

	var rightSeg string
	if m.status != "" {
		style := statusInfoStyle
		if m.statusIsError {
			style = statusErrStyle
		}
		rightSeg = style.Render(" " + m.status + " ")
	}

	left := modeSeg + countsSeg
	fillWidth := m.width - lipgloss.Width(left) - lipgloss.Width(rightSeg)
	if fillWidth < 0 {
		fillWidth = 0
	}
	fill := statusFillStyle.Render(strings.Repeat(" ", fillWidth))
	return left + fill + rightSeg
}

// renderDetail describes the currently-highlighted session — everything
// here is read-only context (location, size, export status), not
// another place to edit anything; editing already has its own
// dedicated controls (rename, the web editor).
func (m tuiModel) renderDetail() string {
	sel, ok := m.selected()
	if !ok {
		return detailMutedStyle.Render("No session selected.")
	}

	dir, dirErr := session.Dir(sel.Name)
	imageCount := 0
	for _, step := range sel.Steps {
		if step.ImageFile != "" {
			imageCount++
		}
	}

	status := detailMutedStyle.Render("stopped")
	if sel.StoppedAt == nil {
		status = detailRecordingStyle.Render("● recording")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n%s\n\n", detailTitleStyle.Render(sel.Name), status)
	if dirErr == nil {
		fmt.Fprintf(&b, "%s\n%s\n\n", detailLabelStyle.Render("Location"), dir)
	}
	fmt.Fprintf(&b, "%s\n%s\n\n", detailLabelStyle.Render("Started"), sel.StartedAt.Format("2006-01-02 15:04"))
	if sel.StoppedAt != nil {
		fmt.Fprintf(&b, "%s\n%s\n\n", detailLabelStyle.Render("Stopped"), sel.StoppedAt.Format("2006-01-02 15:04"))
	}
	fmt.Fprintf(&b, "%s\n%d step(s), %d with a screenshot\n\n", detailLabelStyle.Render("Steps"), len(sel.Steps), imageCount)

	fmt.Fprintf(&b, "%s\n", detailLabelStyle.Render("Exported"))
	fmt.Fprintf(&b, "  Markdown  %s\n", exportMark(dirErr == nil && fileExists(filepath.Join(dir, sel.Name+".md"))))
	fmt.Fprintf(&b, "  PDF       %s\n", exportMark(dirErr == nil && fileExists(filepath.Join(dir, sel.Name+".pdf"))))
	fmt.Fprintf(&b, "  Slides    %s\n", exportMark(dirErr == nil && fileExists(filepath.Join(dir, sel.Name+".html"))))

	if ed, ok := m.editors[sel.Name]; ok {
		fmt.Fprintf(&b, "\n%s\n%s\n", detailLabelStyle.Render("Editor running"), ed.url)
	}
	return b.String()
}

func exportMark(exported bool) string {
	if exported {
		return exportYesStyle.Render("✓ yes")
	}
	return exportNoStyle.Render("— no")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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
		_, watching, warning, err := startRecording(name, false, false)
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
// the same work "loomless export" already does synchronously on the plain
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

// refresh re-lists sessions from disk, re-applies whatever search query
// is currently active (a refresh shouldn't silently drop a filter the
// user was mid-way through), and clears any prior status message — the
// default (empty-status) view already shows live session/editor/
// recording counts, so a bare refresh needs no separate confirmation
// text of its own. Callers that follow refresh() with their own
// setStatus (export, rename, stop) simply overwrite this.
func (m *tuiModel) refresh() {
	sessions, err := session.List()
	if err != nil {
		m.setStatus(err.Error(), true)
		return
	}
	m.allSessions = sessions
	m.applyFilter()
	m.setStatus("", false)
}
