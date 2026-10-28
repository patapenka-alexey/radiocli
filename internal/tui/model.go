package tui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"radiocli/internal/app"
	"radiocli/internal/ports"
	"radiocli/internal/station"
)

type state int

const (
	stateNavigate state = iota
	stateSearch
	stateConfirmDelete
	stateConfirmQuit
)

type stationsRefreshedMsg struct {
	err error
}

type playerEventMsg struct {
	event ports.Event
}

type playerExitedMsg struct{}

type flashExpiredMsg struct {
	id int
}

// Model is the Bubble Tea model for the radiocli TUI.
type Model struct {
	app *app.Service

	width  int
	height int

	state state

	stationIndex  int
	stationOffset int

	searchQuery   string
	searchResults []station.Station
	searchIndex   int
	searchOffset  int

	message          string
	messageUntil     time.Time
	flashID          int
	scheduledFlashID int
	confirmName      string
	quitResume       state
	mpvExitNotified  bool
	refreshing       bool
}

// NewModel creates the TUI model with the application service.
func NewModel(svc *app.Service) *Model {
	return &Model{app: svc}
}

func waitForPlayerEvent(events <-chan ports.Event) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-events
		if !ok {
			return playerExitedMsg{}
		}

		return playerEventMsg{event: event}
	}
}

func fetchStationsCmd(svc *app.Service) tea.Cmd {
	return func() tea.Msg {
		return stationsRefreshedMsg{err: svc.Refresh()}
	}
}

func (m *Model) startRefresh() tea.Cmd {
	if m.refreshing {
		return nil
	}

	m.refreshing = true
	m.flash("Updating station list...", 30*time.Second)
	return fetchStationsCmd(m.app)
}

func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{waitForPlayerEvent(m.app.Events())}

	if !m.app.CatalogAvailable() {
		cmds = append(cmds, m.startRefresh())
	}
	if timer := m.flashTimer(); timer != nil {
		cmds = append(cmds, timer)
	}

	return tea.Batch(cmds...)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := m.update(msg)
	m.ensureVisible()

	if timer := m.flashTimer(); timer != nil {
		cmd = tea.Batch(cmd, timer)
	}

	return m, cmd
}

func (m *Model) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case stationsRefreshedMsg:
		m.refreshing = false

		if msg.err != nil {
			m.flash("Station list update failed", 5*time.Second)
		} else {
			m.flash(fmt.Sprintf("Station list updated (%d stations)", m.app.StationCount()), 3*time.Second)
			if m.state == stateSearch {
				m.updateSearch()
			}
		}

		return nil

	case playerEventMsg:
		return waitForPlayerEvent(m.app.Events())

	case playerExitedMsg:
		if !m.mpvExitNotified {
			m.mpvExitNotified = true
			m.flash("mpv exited unexpectedly", 10*time.Second)
		}

		return nil

	case flashExpiredMsg:
		if msg.id == m.flashID {
			m.message = ""
			m.messageUntil = time.Time{}
		}

		return nil
	}

	return nil
}

func (m *Model) View() tea.View {
	content, cursor := m.render()

	v := tea.NewView(content)
	v.AltScreen = true
	v.Cursor = cursor

	return v
}

// -- actions ----------------------------------------------------------------

func (m *Model) flash(text string, d time.Duration) {
	m.message = text
	m.messageUntil = time.Now().Add(d)
	m.flashID++
}

func (m *Model) flashTimer() tea.Cmd {
	if m.flashID == m.scheduledFlashID {
		return nil
	}
	m.scheduledFlashID = m.flashID

	delay := time.Until(m.messageUntil)
	if delay <= 0 {
		return nil
	}

	id := m.flashID

	return tea.Tick(delay, func(time.Time) tea.Msg {
		return flashExpiredMsg{id: id}
	})
}

func (m *Model) activeMessage() string {
	if time.Now().Before(m.messageUntil) {
		return m.message
	}

	return ""
}

func (m *Model) playStation(s station.Station) {
	if err := m.app.Play(s); err != nil {
		m.flash(err.Error(), 5*time.Second)
	}
}

func (m *Model) playSelectedStation() {
	stations := m.app.Stations()
	if len(stations) == 0 {
		return
	}

	m.playStation(stations[m.stationIndex])
}

func (m *Model) playSelectedSearch() {
	if len(m.searchResults) == 0 {
		return
	}

	m.playStation(m.searchResults[m.searchIndex])
}

func (m *Model) addSelectedSearch() {
	if len(m.searchResults) == 0 {
		return
	}

	s := m.searchResults[m.searchIndex]
	if err := m.app.AddStation(s.Name, s.URL); err != nil {
		m.flash(err.Error(), 5*time.Second)

		return
	}

	m.flash(fmt.Sprintf("Added '%s' to My Stations", s.Name), 3*time.Second)
}

func (m *Model) requestDelete() {
	stations := m.app.Stations()
	if len(stations) == 0 {
		return
	}

	m.confirmName = stations[m.stationIndex].Name
	m.state = stateConfirmDelete
}

func (m *Model) requestQuit() {
	if m.state != stateConfirmQuit {
		m.quitResume = m.state
	}
	m.state = stateConfirmQuit
}

func (m *Model) confirmDelete() {
	name := m.confirmName
	m.confirmName = ""
	m.state = stateNavigate
	if name == "" {
		return
	}

	if err := m.app.RemoveStation(name); err != nil {
		m.flash(err.Error(), 5*time.Second)

		return
	}

	stations := m.app.Stations()
	if m.stationIndex > len(stations)-1 {
		m.stationIndex = max(0, len(stations)-1)
	}

	m.flash(fmt.Sprintf("Removed '%s'", name), 3*time.Second)
}

func (m *Model) cancelDelete() {
	m.confirmName = ""
	m.state = stateNavigate
}

func (m *Model) cancelQuit() {
	m.state = m.quitResume
}

func (m *Model) moveStation(delta int) {
	last := len(m.app.Stations()) - 1
	m.stationIndex = min(max(0, last), max(0, m.stationIndex+delta))
}

func (m *Model) updateSearch() {
	m.searchResults = m.app.Search(m.searchQuery, 200)
	m.searchIndex = 0
	m.searchOffset = 0
}

// -- input ------------------------------------------------------------------

func (m *Model) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	token := msg.String()

	if cmd, ok := globalKeyMap[token]; ok {
		return m.execute(cmd)
	}
	if cmd, ok := m.resolveCommand(token); ok {
		return m.execute(cmd)
	}

	if m.state == stateSearch {
		if text := msg.Key().Text; text != "" && isPrintable(text) {
			m.searchQuery += text
			m.updateSearch()
		}
	}

	return nil
}

func isPrintable(text string) bool {
	for _, r := range text {
		if r < 32 || r == 127 {
			return false
		}
	}
	return utf8.RuneCountInString(text) > 0
}

// -- rendering --------------------------------------------------------------

const (
	ansiReset   = "\x1b[0m"
	ansiDim     = "\x1b[2m"
	ansiReverse = "\x1b[7m"
	ansiGreen   = "\x1b[32m"
	ansiCyan    = "\x1b[36m"
)

func style(code, s string) string {
	return code + s + ansiReset
}

func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}

	return ansi.Truncate(s, width, "")
}

func padRight(s string, width int) string {
	s = truncate(s, width)
	return s + strings.Repeat(" ", max(0, width-runeLen(s)))
}

type layout struct {
	width          int
	height         int
	leftWidth      int
	footerRows     int
	bodyRows       int
	contentRows    int
	stationVisible int
	searchVisible  int
	confirming     bool
}

func (m *Model) layout() layout {
	w, h := m.width, m.height
	l := layout{width: w, height: h, leftWidth: max(30, w/2)}
	l.confirming = m.state == stateConfirmDelete || m.state == stateConfirmQuit
	l.footerRows = 1
	if l.confirming {
		l.footerRows = 2
	}

	l.bodyRows = h - l.footerRows
	if l.bodyRows < 1 {
		l.bodyRows = 1
	}

	l.contentRows = l.bodyRows - 1
	if l.contentRows < 1 {
		l.contentRows = 1
	}

	l.stationVisible = max(0, l.contentRows-1)
	l.searchVisible = max(0, l.contentRows-2)

	return l
}

func (m *Model) ensureVisible() {
	if m.width < 40 || m.height < 8 {
		return
	}

	l := m.layout()
	m.stationOffset = clampOffset(m.stationIndex, m.stationOffset, l.stationVisible, len(m.app.Stations()))
	m.searchOffset = clampOffset(m.searchIndex, m.searchOffset, l.searchVisible, len(m.searchResults))
}

func clampOffset(index, offset, visible, total int) int {
	if visible <= 0 {
		return 0
	}

	if index < offset {
		offset = index
	} else if index >= offset+visible {
		offset = index - visible + 1
	}

	offset = min(offset, max(0, total-visible))

	return max(0, offset)
}

func (m *Model) render() (string, *tea.Cursor) {
	l := m.layout()
	if l.width < 40 || l.height < 8 {
		return "Terminal too small", nil
	}

	lines := make([]string, 0, l.height)
	lines = append(lines, m.bodyLines(l.bodyRows, l.leftWidth, l.width)...)
	lines = append(lines, m.footerLines(l.width, l.footerRows)...)

	var cursor *tea.Cursor
	if m.state == stateSearch && !l.confirming {
		x := l.leftWidth + 3 + runeLen(searchPrompt(m.searchQuery))
		y := 2
		cursor = tea.NewCursor(min(x, l.width-1), y)
	}

	return strings.Join(lines, "\n"), cursor
}

func (m *Model) bodyLines(rows, leftWidth, width int) []string {
	contentRows := rows - 1
	if contentRows < 1 {
		contentRows = 1
	}

	left := m.stationListLines(contentRows, leftWidth)

	var right []string
	if m.state == stateSearch {
		right = m.searchPanelLines(contentRows, width-leftWidth-3)
	} else {
		right = m.nowPlayingLines()
	}

	result := make([]string, 0, rows)
	result = append(result, composeRow(left, right, 0, width))
	result = append(result, separatorLine(leftWidth, width))
	for i := 1; i < contentRows; i++ {
		result = append(result, composeRow(left, right, i, width))
	}
	for len(result) < rows {
		result = append(result, strings.Repeat(" ", width))
	}
	return result
}

func composeRow(left, right []string, i, width int) string {
	leftCell := ""
	if i < len(left) {
		leftCell = left[i]
	}

	rightCell := ""
	if i < len(right) {
		rightCell = right[i]
	}

	return truncate(leftCell+"│  "+rightCell, width)
}

func separatorLine(leftWidth, width int) string {
	if width <= 0 {
		return ""
	}
	if leftWidth >= width {
		return strings.Repeat("─", width)
	}
	return strings.Repeat("─", leftWidth) + "┼" + strings.Repeat("─", width-leftWidth-1)
}

func (m *Model) stationListLines(rows, leftWidth int) []string {
	items := m.app.Stations()
	index, offset := m.stationIndex, m.stationOffset

	visible := rows - 1
	if visible < 0 {
		visible = 0
	}

	left := make([]string, rows)
	left[0] = style(ansiCyan, padRight(" Stations", leftWidth))

	for row := 0; row < visible && row < rows-1; row++ {
		itemIdx := offset + row
		if itemIdx >= len(items) {
			break
		}

		name := items[itemIdx].Name
		marker := "  "
		line := marker + name
		selected := itemIdx == index

		if name == m.app.CurrentStation() {
			marker = "> "
			line = marker + name
			if selected {
				line = style(ansiReverse, padRight(line, leftWidth))
			} else {
				line = style(ansiGreen, padRight(line, leftWidth))
			}
		} else if selected {
			line = style(ansiReverse, padRight(line, leftWidth))
		} else {
			line = padRight(line, leftWidth)
		}
		left[1+row] = line
	}

	for i := range left {
		if left[i] == "" {
			left[i] = strings.Repeat(" ", leftWidth)
		}
	}

	if len(items) == 0 && rows > 1 {
		left[1] = style(ansiDim, padRight(" No stations", leftWidth))
	}
	return left
}

func searchPrompt(query string) string {
	return " Search: " + query
}

func (m *Model) searchPanelLines(rows, width int) []string {
	items := m.searchResults

	lines := make([]string, 0, rows)
	lines = append(lines, padRight(fmt.Sprintf(" Search results (%d)", len(items)), width))
	lines = append(lines, padRight(searchPrompt(m.searchQuery), width))

	start := len(lines)
	visible := rows - start
	if visible < 0 {
		visible = 0
	}

	index, offset := m.searchIndex, m.searchOffset

	for row := 0; row < visible && row < rows-start; row++ {
		itemIdx := offset + row
		if itemIdx >= len(items) {
			break
		}

		line := "  " + items[itemIdx].Name
		if itemIdx == index {
			line = style(ansiReverse, padRight(line, width))
		} else {
			line = padRight(line, width)
		}
		lines = append(lines, line)
	}

	for len(lines) < rows {
		lines = append(lines, strings.Repeat(" ", width))
	}

	if len(items) == 0 && rows > start {
		lines[start] = style(ansiDim, padRight(" No matches", width))
	}
	return lines
}

func (m *Model) nowPlayingLines() []string {
	snap := m.app.Status()
	playing := !snap.Idle && snap.Alive

	status := "Stopped"
	statusStyle := ansiDim
	if playing && !snap.Paused {
		status = "Playing"
		statusStyle = ansiGreen
	} else if playing && snap.Paused {
		status = "Paused"
		statusStyle = ansiCyan
	}

	stationName := m.app.CurrentStation()
	if stationName == "" {
		stationName = "-"
	}

	title := snap.Title
	if title == "" {
		title = "-"
	}

	lines := []string{
		style(ansiCyan, "Now Playing"),
		"",
		labelValue("Station", stationName, ""),
		labelValue("Title", title, ""),
		labelValue("Status", status, statusStyle),
	}

	pos := "--:--"
	if snap.HasTimePos {
		pos = fmtTime(snap.TimePos)
	}

	timeText := "--:--"
	switch {
	case snap.HasDuration:
		timeText = fmt.Sprintf("%s / %s", pos, fmtTime(snap.Duration))
	case playing:
		timeText = pos + " (live)"
	}
	lines = append(lines, labelValue("Time", timeText, ""))

	if !snap.Alive {
		lines = append(lines, "")
		lines = append(lines, style(ansiCyan, "mpv is not running"))
	}
	return lines
}

func labelValue(label, value, valueStyle string) string {
	return style(ansiDim, fmt.Sprintf("%-8s", label)) + " " + style(valueStyle, value)
}

func fmtTime(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	total := int(seconds)
	minutes, secs := total/60, total%60
	hours, minutes := minutes/60, minutes%60
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, secs)
	}
	return fmt.Sprintf("%02d:%02d", minutes, secs)
}

func (m *Model) footerLines(width, count int) []string {
	lines := make([]string, count)

	switch m.state {
	case stateConfirmQuit:
		lines[0] = style(ansiCyan, padRight("Quit radiocli? (y/N)", width))
		return lines
	case stateConfirmDelete:
		prompt := fmt.Sprintf("Delete '%s'? (y/N)", m.confirmName)
		lines[0] = style(ansiCyan, padRight(prompt, width))
		return lines
	case stateSearch:
		hints := hotkeyHints([][2]string{
			{"Enter", "play"},
			{"Ctrl+A", "add"},
			{"Ctrl+U", "update"},
			{"Esc", "cancel"},
		})
		lines[0] = truncate(hints, width)
		return lines
	}

	if msg := m.activeMessage(); msg != "" {
		lines[0] = style(ansiCyan, padRight(msg, width))
		return lines
	}

	hints := hotkeyHints([][2]string{
		{"↑/↓ j/k", "move"},
		{"Enter", "play"},
		{"Space", "pause"},
		{"s", "stop"},
		{"/", "search"},
		{"u", "update"},
		{"d", "delete"},
		{"q", "quit"},
	})
	lines[0] = truncate(hints, width)
	return lines
}

func hotkeyHints(keys [][2]string) string {
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = style(ansiCyan, k[0]) + style(ansiDim, " "+k[1])
	}
	return strings.Join(parts, style(ansiDim, " · "))
}
