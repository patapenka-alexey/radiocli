package main

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

type mode int

const (
	modeNavigate mode = iota
	modeSearch
)

type tickMsg time.Time

type stationsRefreshedMsg struct {
	err error
}

type model struct {
	cfg *config
	db  *stationDB
	pl  *player

	width  int
	height int

	mode mode

	stationIndex   int
	stationOffset  int
	currentStation string

	searchQuery   string
	searchResults []Station
	searchIndex   int
	searchOffset  int

	message         string
	messageUntil    time.Time
	pendingDelete   string
	mpvExitNotified bool
	refreshing      bool
}

func newModel(cfg *config, db *stationDB, pl *player) *model {
	return &model{cfg: cfg, db: db, pl: pl}
}

func tickCmd() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func fetchStationsCmd(db *stationDB) tea.Cmd {
	return func() tea.Msg {
		return stationsRefreshedMsg{err: db.refresh()}
	}
}

func (m *model) startRefresh() tea.Cmd {
	if m.refreshing {
		return nil
	}
	m.refreshing = true
	m.flash("Updating station list...", 30*time.Second)
	return fetchStationsCmd(m.db)
}

func (m *model) Init() tea.Cmd {
	cmds := []tea.Cmd{tickCmd()}
	if !m.db.available() {
		cmds = append(cmds, m.startRefresh())
	}
	return tea.Batch(cmds...)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyPressMsg:
		return m, m.handleKey(msg)

	case stationsRefreshedMsg:
		m.refreshing = false
		if msg.err != nil {
			m.flash("Station list update failed", 5*time.Second)
		} else {
			m.flash(fmt.Sprintf("Station list updated (%d stations)", m.db.count()), 3*time.Second)
			if m.mode == modeSearch {
				m.updateSearch()
			}
		}
		return m, nil

	case tickMsg:
		if !m.pl.isAlive() && !m.mpvExitNotified {
			m.mpvExitNotified = true
			m.flash("mpv exited unexpectedly", 10*time.Second)
		}
		return m, tickCmd()
	}
	return m, nil
}

func (m *model) View() tea.View {
	content, cursor := m.render()
	v := tea.NewView(content)
	v.AltScreen = true
	v.Cursor = cursor
	return v
}

// -- actions ----------------------------------------------------------------

func (m *model) flash(text string, d time.Duration) {
	m.message = text
	m.messageUntil = time.Now().Add(d)
}

func (m *model) activeMessage() string {
	if time.Now().Before(m.messageUntil) {
		return m.message
	}
	return ""
}

func (m *model) playStation(name, url string) {
	if !m.pl.isAlive() {
		m.flash("mpv is not running", 5*time.Second)
		return
	}
	m.currentStation = name
	m.pl.play(url)
}

func (m *model) playSelectedStation() {
	if len(m.cfg.Stations) == 0 {
		return
	}
	s := m.cfg.Stations[m.stationIndex]
	m.playStation(s.Name, s.URL)
}

func (m *model) playSelectedSearch() {
	if len(m.searchResults) == 0 {
		return
	}
	s := m.searchResults[m.searchIndex]
	m.playStation(s.Name, s.URL)
}

func (m *model) addSelectedSearch() {
	if len(m.searchResults) == 0 {
		return
	}
	s := m.searchResults[m.searchIndex]
	if err := m.cfg.addStation(s.Name, s.URL); err != nil {
		m.flash(err.Error(), 5*time.Second)
		return
	}
	_ = m.cfg.save()
	m.flash(fmt.Sprintf("Added '%s' to My Stations", s.Name), 3*time.Second)
}

func (m *model) deleteSelected() {
	if len(m.cfg.Stations) == 0 {
		return
	}
	m.pendingDelete = m.cfg.Stations[m.stationIndex].Name
}

func (m *model) confirmDelete(yes bool) {
	name := m.pendingDelete
	m.pendingDelete = ""
	if name == "" || !yes {
		return
	}
	m.cfg.removeStation(name)
	_ = m.cfg.save()
	if name == m.currentStation {
		m.currentStation = ""
	}
	if m.stationIndex > len(m.cfg.Stations)-1 {
		m.stationIndex = max(0, len(m.cfg.Stations)-1)
	}
	m.flash(fmt.Sprintf("Removed '%s'", name), 3*time.Second)
}

func (m *model) moveStation(delta int) {
	last := len(m.cfg.Stations) - 1
	m.stationIndex = min(max(0, last), max(0, m.stationIndex+delta))
}

func (m *model) updateSearch() {
	m.searchResults = m.db.search(m.searchQuery, 200)
	m.searchIndex = 0
	m.searchOffset = 0
}

// -- input ------------------------------------------------------------------

func (m *model) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	if msg.String() == "ctrl+c" {
		return tea.Quit
	}
	key := msg.Key()

	if m.pendingDelete != "" {
		switch key.Code {
		case 'y', 'Y':
			m.confirmDelete(true)
		case 'n', 'N', tea.KeyEscape:
			m.confirmDelete(false)
		}
		return nil
	}

	if m.mode == modeSearch {
		return m.handleSearchKey(key)
	}
	return m.handleNavKey(key)
}

func (m *model) handleSearchKey(key tea.Key) tea.Cmd {
	switch key.Code {
	case tea.KeyEscape:
		m.mode = modeNavigate
		return nil
	case tea.KeyEnter:
		m.playSelectedSearch()
		m.mode = modeNavigate
		return nil
	case tea.KeyUp:
		m.searchIndex = max(0, m.searchIndex-1)
		return nil
	case tea.KeyDown:
		m.searchIndex = min(max(0, len(m.searchResults)-1), m.searchIndex+1)
		return nil
	case tea.KeyBackspace:
		if m.searchQuery != "" {
			r := []rune(m.searchQuery)
			m.searchQuery = string(r[:len(r)-1])
			m.updateSearch()
		}
		return nil
	}

	text := key.Text
	if text == "a" {
		m.addSelectedSearch()
		return nil
	}
	if text != "" && isPrintable(text) {
		m.searchQuery += text
		m.updateSearch()
	}
	return nil
}

func (m *model) handleNavKey(key tea.Key) tea.Cmd {
	switch key.Code {
	case tea.KeyUp:
		m.moveStation(-1)
	case tea.KeyDown:
		m.moveStation(1)
	case tea.KeyEnter:
		m.playSelectedStation()
	case tea.KeySpace:
		if m.pl.snapshot().Idle {
			m.playSelectedStation()
		} else {
			m.pl.togglePause()
		}
	case tea.KeyLeft:
		m.seek(-10)
	case tea.KeyRight:
		m.seek(10)
	}

	text := key.Text
	if text == "" {
		text = string(key.Code)
	}

	switch strings.ToLower(text) {
	case "q":
		return tea.Quit
	case "k":
		m.moveStation(-1)
	case "j":
		m.moveStation(1)
	case "s":
		m.pl.stop()
		m.currentStation = ""
	case "/":
		m.mode = modeSearch
		m.searchQuery = ""
		m.updateSearch()
	case "u":
		return m.startRefresh()
	case "d":
		m.deleteSelected()
	}
	return nil
}

func (m *model) seek(seconds float64) {
	if m.pl.snapshot().Idle {
		return
	}
	m.pl.seek(seconds)
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

func (m *model) render() (string, *tea.Cursor) {
	w, h := m.width, m.height
	if w < 40 || h < 8 {
		return "Terminal too small", nil
	}

	leftWidth := max(30, w/2)
	footerRows := 1
	if m.pendingDelete != "" {
		footerRows = 2
	}
	bodyTop := 0
	bodyBottom := h - footerRows
	bodyHeight := bodyBottom - bodyTop
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	lines := make([]string, 0, h)
	lines = append(lines, m.bodyLines(bodyHeight, leftWidth, w)...)
	lines = append(lines, m.footerLines(w, footerRows)...)

	var cursor *tea.Cursor
	if m.mode == modeSearch && m.pendingDelete == "" {
		x := leftWidth + 3 + runeLen(searchPrompt(m.searchQuery))
		y := bodyTop + 2
		cursor = tea.NewCursor(min(x, w-1), y)
	}

	return strings.Join(lines, "\n"), cursor
}

func (m *model) bodyLines(rows, leftWidth, width int) []string {
	contentRows := rows - 1
	if contentRows < 1 {
		contentRows = 1
	}

	left := m.stationListLines(contentRows, leftWidth)

	var right []string
	if m.mode == modeSearch {
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
		return style(ansiDim, strings.Repeat("─", width))
	}
	return style(ansiDim, strings.Repeat("─", leftWidth)+"┼"+strings.Repeat("─", width-leftWidth-1))
}

func (m *model) stationListLines(rows, leftWidth int) []string {
	items := m.cfg.Stations
	index, offset := m.stationIndex, m.stationOffset

	visible := rows - 1
	if visible < 0 {
		visible = 0
	}
	if index < offset {
		offset = index
	} else if index >= offset+visible {
		offset = index - visible + 1
	}
	if offset < 0 {
		offset = 0
	}
	m.stationOffset = offset

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

		if name == m.currentStation {
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

func (m *model) searchPanelLines(rows, width int) []string {
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
	if index < offset {
		offset = index
	} else if index >= offset+visible {
		offset = index - visible + 1
	}
	if offset < 0 {
		offset = 0
	}
	m.searchOffset = offset

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

func (m *model) nowPlayingLines() []string {
	snap := m.pl.snapshot()
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

	station := m.currentStation
	if station == "" {
		station = "-"
	}
	title := snap.Title
	if title == "" {
		title = "-"
	}

	lines := []string{
		style(ansiCyan, "Now Playing"),
		"",
		labelValue("Station", station, ""),
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

func (m *model) footerLines(width, count int) []string {
	lines := make([]string, count)

	if m.pendingDelete != "" {
		prompt := fmt.Sprintf("Delete '%s'? (y/N)", m.pendingDelete)
		lines[0] = style(ansiCyan, padRight(prompt, width))
		return lines
	}

	if m.mode == modeSearch {
		hints := hotkeyHints([][2]string{
			{"Enter", "play"},
			{"a", "add"},
			{"u", "update"},
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
		{"↑/↓", "move"},
		{"Enter", "play"},
		{"Space", "pause"},
		{"s", "stop"},
		{"←/→", "seek"},
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
