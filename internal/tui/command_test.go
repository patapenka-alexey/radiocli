package tui

import (
	"os"
	"path/filepath"
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
	for _, text := range []string{"a", "A", "u", "s", "q", "e", "E"} {
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

func TestEqualizerKeyBindings(t *testing.T) {
	m := newTestModel(1)

	tests := []struct {
		key  string
		want command
	}{
		{"E", cmdToggleEqualizer},
		{"e", cmdRefreshEqualizer},
	}

	for _, tt := range tests {
		cmd, ok := lookupKey(m.keyMap(), tt.key)
		if !ok || cmd != tt.want {
			t.Fatalf("key %q: command = %v, ok = %v, want %v", tt.key, cmd, ok, tt.want)
		}
	}

	if _, ok := lookupKey(m.keyMap(), "ctrl+e"); ok {
		t.Fatal("ctrl+e should be a global binding, not a navigate binding")
	}
}

func TestToggleEqualizerKey(t *testing.T) {
	m := newTestModel(1)

	m.handleKey(press("E"))

	status := m.app.EqualizerStatus()
	if !status.Enabled || status.Preset != "a" {
		t.Fatalf("status = %+v", status)
	}

	m.handleKey(press("E"))
	if m.app.EqualizerStatus().Enabled {
		t.Fatal("equalizer still enabled")
	}
}

func TestCtrlENextEqualizerIsGlobal(t *testing.T) {
	m := newTestModel(1)
	m.state = stateSearch

	m.handleKey(pressCtrl('e'))

	status := m.app.EqualizerStatus()
	if status.Preset != "b" {
		t.Fatalf("preset = %q, want b", status.Preset)
	}
	if m.searchQuery != "" {
		t.Fatalf("searchQuery = %q, want empty", m.searchQuery)
	}
}

func TestRefreshEqualizerKey(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x"), []byte("EQSET\n100 1.5 -4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RADIOCLI_EQSETS", dir)

	m := newTestModel(1)

	m.handleKey(press("e"))

	status := m.app.EqualizerStatus()
	if status.Count != 1 || status.Preset != "x" {
		t.Fatalf("status = %+v", status)
	}
	if m.searchQuery != "" {
		t.Fatalf("searchQuery = %q, want empty", m.searchQuery)
	}
}
