package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"radiocli/internal/station"
)

func TestKeyMapsPerState(t *testing.T) {
	tests := []struct {
		state state
		key   string
		want  command
	}{
		{stateNavigate, "q", cmdRequestQuit},
		{stateNavigate, "/", cmdOpenSearch},
		{stateNavigate, "enter", cmdPlayStation},
		{stateSearch, "up", cmdMoveSearchUp},
		{stateSearch, "down", cmdMoveSearchDown},
		{stateSearch, "ctrl+a", cmdAddSearch},
		{stateSearch, "ctrl+u", cmdRefresh},
		{stateSearch, "esc", cmdCancelSearch},
		{stateSearch, "backspace", cmdBackspace},
		{stateConfirmDelete, "y", cmdConfirmDelete},
		{stateConfirmDelete, "n", cmdCancelDelete},
		{stateConfirmQuit, "y", cmdConfirmQuit},
		{stateConfirmQuit, "esc", cmdCancelQuit},
	}

	for _, tt := range tests {
		m := newTestModel(1)
		m.state = tt.state

		cmd, ok := lookupKey(m.keyMap(), tt.key)
		if !ok {
			t.Fatalf("state %v: no binding for %q", tt.state, tt.key)
		}
		if cmd != tt.want {
			t.Fatalf("state %v, key %q: command = %v, want %v", tt.state, tt.key, cmd, tt.want)
		}
	}
}

func TestUppercaseNavigationKeys(t *testing.T) {
	m := newTestModel(3)

	m.handleKey(press("J"))
	if m.stationIndex != 1 {
		t.Fatalf("stationIndex = %d, want 1", m.stationIndex)
	}

	m.handleKey(press("K"))
	if m.stationIndex != 0 {
		t.Fatalf("stationIndex = %d, want 0", m.stationIndex)
	}

	m.handleKey(press("Q"))
	if m.state != stateConfirmQuit {
		t.Fatalf("state = %v, want stateConfirmQuit", m.state)
	}
}

func TestSearchLettersAreTyped(t *testing.T) {
	for _, text := range []string{"a", "A", "u", "s", "q"} {
		t.Run(text, func(t *testing.T) {
			m := newTestModelWith()
			m.state = stateSearch

			m.handleKey(press(text))

			if m.searchQuery != text {
				t.Fatalf("searchQuery = %q, want %q", m.searchQuery, text)
			}
			if len(m.app.Stations()) != 0 {
				t.Fatalf("letter %q must not add a station", text)
			}
			if m.state != stateSearch {
				t.Fatalf("state = %v, want stateSearch", m.state)
			}
		})
	}
}

func TestSearchAddCommand(t *testing.T) {
	m := newTestModelWith()
	m.state = stateSearch
	m.searchResults = []station.Station{{Name: "Jazz", URL: "https://example.com/jazz"}}

	m.handleKey(pressCtrl('a'))

	stations := m.app.Stations()
	if len(stations) != 1 || stations[0].Name != "Jazz" {
		t.Fatalf("stations = %+v, want Jazz", stations)
	}
	if m.searchQuery != "" {
		t.Fatalf("searchQuery = %q, want empty", m.searchQuery)
	}
}

func TestRefreshNotBoundInNavigate(t *testing.T) {
	m := newTestModel(1)

	cmd := m.handleKey(press("u"))

	if cmd != nil {
		t.Fatal("u should not trigger refresh in navigate mode")
	}
	if m.refreshing {
		t.Fatal("refreshing = true, want false")
	}
}

func TestRefreshCommandInSearch(t *testing.T) {
	m := newTestModel(1)
	m.state = stateSearch

	cmd := m.handleKey(pressCtrl('u'))

	if cmd == nil {
		t.Fatal("refresh command returned nil tea.Cmd")
	}
	if !m.refreshing {
		t.Fatal("refreshing = false, want true")
	}
	if m.searchQuery != "" {
		t.Fatalf("searchQuery = %q, want empty", m.searchQuery)
	}
}

func TestSearchArrowsMoveSearchSelection(t *testing.T) {
	m := newTestModelWith(
		station.Station{Name: "A"},
		station.Station{Name: "B"},
	)
	m.state = stateSearch
	m.searchResults = []station.Station{
		{Name: "Result 1"},
		{Name: "Result 2"},
	}
	m.searchIndex = 0
	m.stationIndex = 1

	m.handleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.searchIndex != 1 {
		t.Fatalf("searchIndex = %d, want 1", m.searchIndex)
	}
	if m.stationIndex != 1 {
		t.Fatalf("stationIndex = %d, want 1 (unchanged)", m.stationIndex)
	}

	m.handleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.searchIndex != 1 {
		t.Fatalf("searchIndex = %d, want 1 (clamped)", m.searchIndex)
	}

	m.handleKey(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.searchIndex != 0 {
		t.Fatalf("searchIndex = %d, want 0", m.searchIndex)
	}
}

func TestArrowAndSpaceBindings(t *testing.T) {
	m := newTestModel(2)

	m.handleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.stationIndex != 1 {
		t.Fatalf("stationIndex = %d, want 1", m.stationIndex)
	}

	m.handleKey(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.stationIndex != 0 {
		t.Fatalf("stationIndex = %d, want 0", m.stationIndex)
	}
}

func TestHotkeysBlockedWhileRefreshing(t *testing.T) {
	m := newTestModel(2)
	m.state = stateSearch
	m.startRefresh()

	m.handleKey(press("a"))
	if m.searchQuery != "" {
		t.Fatalf("searchQuery = %q, want empty while refreshing", m.searchQuery)
	}

	m.handleKey(press("q"))
	if m.state != stateSearch {
		t.Fatalf("state = %v, want stateSearch while refreshing", m.state)
	}
	if !m.refreshing {
		t.Fatal("refreshing = false, want true")
	}
}

func TestCtrlCCancelsRefresh(t *testing.T) {
	m := newTestModel(2)
	m.startRefresh()
	id := m.refreshID

	m.handleKey(pressCtrl('c'))

	if m.refreshing {
		t.Fatal("refreshing = true after Ctrl+C")
	}
	if m.refreshCancel != nil {
		t.Fatal("refreshCancel was not released")
	}
	if !strings.Contains(m.message, "Update cancelled") {
		t.Fatalf("message = %q, want cancellation notice", m.message)
	}

	m.update(stationsRefreshedMsg{id: id, err: errors.New("cancelled")})
	if strings.Contains(m.message, "failed") {
		t.Fatalf("stale result changed the message: %q", m.message)
	}

	m.handleKey(pressCtrl('c'))
	if m.state != stateConfirmQuit {
		t.Fatalf("state = %v, want stateConfirmQuit after refresh cancellation", m.state)
	}
}

func TestRefreshShowsProgressAndStopsTicking(t *testing.T) {
	m := newTestModel(1)
	m.startRefresh()

	if !m.refreshing {
		t.Fatal("refreshing = false, want true")
	}
	if !strings.Contains(m.footerLines(80, 40, 2)[1], "Updating station list") {
		t.Fatalf("footer = %q, want update progress", m.footerLines(80, 40, 2)[1])
	}

	first := m.refreshStatus()
	if cmd := m.update(refreshTickMsg{}); cmd == nil {
		t.Fatal("refresh tick returned nil while refreshing")
	}
	if m.refreshStatus() == first {
		t.Fatal("refresh progress did not advance")
	}

	m.update(stationsRefreshedMsg{})
	if m.refreshing {
		t.Fatal("refreshing = true after completion")
	}
	if cmd := m.update(refreshTickMsg{}); cmd != nil {
		t.Fatal("refresh tick still scheduled after completion")
	}
}
