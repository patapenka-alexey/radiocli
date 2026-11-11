package tui

import (
	"testing"

	"radiocli/internal/app"
	"radiocli/internal/station"
)

func newRemoteModel(t *testing.T, stations ...station.Station) (*Model, *fakePlayer) {
	t.Helper()

	pl := &fakePlayer{}
	m := NewModel(app.New(&fakeStore{stations: stations}, fakeCatalog{}, pl))

	return m, pl
}

func TestRemoteNextWrapsAround(t *testing.T) {
	m, pl := newRemoteModel(t,
		station.Station{Name: "A", URL: "https://example.com/a"},
		station.Station{Name: "B", URL: "https://example.com/b"},
	)

	m.update(RemoteCommandMsg{Command: "next"})
	if m.app.CurrentStation() != "B" {
		t.Fatalf("CurrentStation() = %q, want B", m.app.CurrentStation())
	}
	if m.stationIndex != 1 {
		t.Fatalf("stationIndex = %d, want 1", m.stationIndex)
	}

	m.update(RemoteCommandMsg{Command: "next"})
	if m.app.CurrentStation() != "A" {
		t.Fatalf("CurrentStation() = %q, want A", m.app.CurrentStation())
	}
	if m.stationIndex != 0 {
		t.Fatalf("stationIndex = %d, want 0", m.stationIndex)
	}

	if len(pl.played) != 2 {
		t.Fatalf("played = %v, want two stations", pl.played)
	}
}

func TestRemotePreviousWrapsAround(t *testing.T) {
	m, _ := newRemoteModel(t,
		station.Station{Name: "A", URL: "https://example.com/a"},
		station.Station{Name: "B", URL: "https://example.com/b"},
	)

	m.update(RemoteCommandMsg{Command: "previous"})
	if m.app.CurrentStation() != "B" {
		t.Fatalf("CurrentStation() = %q, want B", m.app.CurrentStation())
	}
	if m.stationIndex != 1 {
		t.Fatalf("stationIndex = %d, want 1", m.stationIndex)
	}
}

func TestRemoteSwitchFollowsPlayingStation(t *testing.T) {
	m, _ := newRemoteModel(t,
		station.Station{Name: "A", URL: "https://example.com/a"},
		station.Station{Name: "B", URL: "https://example.com/b"},
		station.Station{Name: "C", URL: "https://example.com/c"},
	)
	m.stationIndex = 0
	m.playStation(m.app.Stations()[2])

	m.update(RemoteCommandMsg{Command: "next"})

	if m.app.CurrentStation() != "A" {
		t.Fatalf("CurrentStation() = %q, want A", m.app.CurrentStation())
	}
}

func TestRemotePauseAndUnpause(t *testing.T) {
	m, pl := newRemoteModel(t, station.Station{Name: "A", URL: "https://example.com/a"})

	m.update(RemoteCommandMsg{Command: "pause"})
	if !pl.paused || pl.toggles != 1 {
		t.Fatalf("paused = %v, toggles = %d, want true, 1", pl.paused, pl.toggles)
	}

	m.update(RemoteCommandMsg{Command: "pause"})
	if pl.toggles != 1 {
		t.Fatalf("toggles = %d, want 1: pause must be idempotent", pl.toggles)
	}

	m.update(RemoteCommandMsg{Command: "unpause"})
	if pl.paused || pl.toggles != 2 {
		t.Fatalf("paused = %v, toggles = %d, want false, 2", pl.paused, pl.toggles)
	}

	if len(pl.played) != 0 {
		t.Fatalf("played = %v, want none", pl.played)
	}
}

func TestRemoteSwitchWithNoStations(t *testing.T) {
	m, pl := newRemoteModel(t)

	m.update(RemoteCommandMsg{Command: "next"})

	if len(pl.played) != 0 {
		t.Fatalf("played = %v, want none", pl.played)
	}
	if m.message != "No stations" {
		t.Fatalf("message = %q, want No stations", m.message)
	}
}

func TestRemoteUnknownCommandIsIgnored(t *testing.T) {
	m, pl := newRemoteModel(t,
		station.Station{Name: "A", URL: "https://example.com/a"},
	)

	m.update(RemoteCommandMsg{Command: "bogus"})

	if len(pl.played) != 0 {
		t.Fatalf("played = %v, want none", pl.played)
	}
}
