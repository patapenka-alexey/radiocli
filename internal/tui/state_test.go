package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"radiocli/internal/station"
)

func press(text string) tea.KeyPressMsg {
	r := []rune(text)

	return tea.KeyPressMsg{Code: r[0], Text: text}
}

func pressCtrl(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Mod: tea.ModCtrl}
}

func TestQuitConfirmationCancel(t *testing.T) {
	m := newTestModel(1)

	m.handleKey(press("q"))
	if m.state != stateConfirmQuit {
		t.Fatalf("state = %v, want stateConfirmQuit", m.state)
	}

	m.handleKey(press("n"))
	if m.state != stateNavigate {
		t.Fatalf("state = %v, want stateNavigate", m.state)
	}
}

func TestQuitConfirmationReturnsToPreviousState(t *testing.T) {
	m := newTestModel(1)
	m.state = stateSearch

	m.handleKey(pressCtrl('c'))
	if m.state != stateConfirmQuit {
		t.Fatalf("state = %v, want stateConfirmQuit", m.state)
	}

	m.handleKey(press("n"))
	if m.state != stateSearch {
		t.Fatalf("state = %v, want stateSearch", m.state)
	}
}

func TestDeleteConfirmation(t *testing.T) {
	m := newTestModelWith(
		station.Station{Name: "Jazz"},
		station.Station{Name: "Rock"},
	)

	m.handleKey(press("d"))
	if m.state != stateConfirmDelete {
		t.Fatalf("state = %v, want stateConfirmDelete", m.state)
	}
	if m.confirmName != "Jazz" {
		t.Fatalf("confirmName = %q, want Jazz", m.confirmName)
	}

	m.handleKey(press("n"))
	if m.state != stateNavigate {
		t.Fatalf("state = %v, want stateNavigate", m.state)
	}
	if m.confirmName != "" {
		t.Fatalf("confirmName = %q, want empty", m.confirmName)
	}
	if got := len(m.app.Stations()); got != 2 {
		t.Fatalf("stations = %d, want 2", got)
	}
}

func TestDeleteConfirmationRemovesStation(t *testing.T) {
	m := newTestModelWith(
		station.Station{Name: "Jazz"},
		station.Station{Name: "Rock"},
	)

	m.handleKey(press("d"))
	m.handleKey(press("y"))

	if m.state != stateNavigate {
		t.Fatalf("state = %v, want stateNavigate", m.state)
	}
	stations := m.app.Stations()
	if len(stations) != 1 || stations[0].Name != "Rock" {
		t.Fatalf("stations = %+v, want only Rock", stations)
	}
}

func TestSearchStateTransitions(t *testing.T) {
	m := newTestModel(1)

	m.handleKey(press("/"))
	if m.state != stateSearch {
		t.Fatalf("state = %v, want stateSearch", m.state)
	}

	m.handleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.state != stateNavigate {
		t.Fatalf("state = %v, want stateNavigate", m.state)
	}
}

func TestQuitFromDeleteConfirmationResumesDelete(t *testing.T) {
	m := newTestModelWith(station.Station{Name: "Jazz"})

	m.handleKey(press("d"))
	m.handleKey(pressCtrl('c'))
	if m.state != stateConfirmQuit {
		t.Fatalf("state = %v, want stateConfirmQuit", m.state)
	}

	m.handleKey(press("n"))
	if m.state != stateConfirmDelete {
		t.Fatalf("state = %v, want stateConfirmDelete", m.state)
	}
	if m.confirmName != "Jazz" {
		t.Fatalf("confirmName = %q, want Jazz", m.confirmName)
	}
}
