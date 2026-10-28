package tui

import (
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
		{stateNavigate, "u", cmdRefresh},
		{stateNavigate, "enter", cmdPlayStation},
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

func TestRefreshCommand(t *testing.T) {
	m := newTestModel(1)

	cmd := m.handleKey(press("u"))

	if cmd == nil {
		t.Fatal("refresh command returned nil tea.Cmd")
	}
	if !m.refreshing {
		t.Fatal("refreshing = false, want true")
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
