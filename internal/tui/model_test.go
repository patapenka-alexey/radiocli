package tui

import (
	"testing"

	"radiocli/internal/ports"
	"radiocli/internal/station"
)

type fakeStore struct {
	stations []station.Station
}

func (f *fakeStore) Stations() []station.Station { return f.stations }

func (f *fakeStore) AddStation(string, string) error { return nil }

func (f *fakeStore) RemoveStation(string) {}

func (f *fakeStore) Save() error { return nil }

type fakeCatalog struct{}

func (fakeCatalog) Search(string, int) []station.Station { return nil }

func (fakeCatalog) Refresh() error { return nil }

func (fakeCatalog) Count() int { return 0 }

func (fakeCatalog) Available() bool { return false }

type fakePlayer struct{}

func (fakePlayer) Play(string) {}

func (fakePlayer) TogglePause() {}

func (fakePlayer) Stop() {}

func (fakePlayer) Status() ports.PlayerStatus { return ports.PlayerStatus{Alive: true} }

func (fakePlayer) Alive() bool { return true }

func (fakePlayer) Quit() {}

func newTestModel(stations int) *Model {
	return &Model{
		cfg: &fakeStore{stations: make([]station.Station, stations)},
		db:  fakeCatalog{},
		pl:  fakePlayer{},
	}
}

func TestClampOffset(t *testing.T) {
	tests := []struct {
		name    string
		index   int
		offset  int
		visible int
		total   int
		want    int
	}{
		{"fits without scrolling", 2, 0, 5, 10, 0},
		{"scrolls down", 7, 0, 5, 10, 3},
		{"keeps existing offset", 7, 5, 5, 10, 5},
		{"scrolls up", 2, 5, 5, 10, 2},
		{"clamps to last page", 9, 8, 5, 10, 5},
		{"zero visible rows", 3, 2, 0, 10, 0},
		{"empty list", 0, 0, 5, 0, 0},
		{"negative offset", 0, -4, 5, 10, 0},
		{"index past total", 20, 0, 5, 10, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampOffset(tt.index, tt.offset, tt.visible, tt.total); got != tt.want {
				t.Fatalf("clampOffset(%d, %d, %d, %d) = %d, want %d",
					tt.index, tt.offset, tt.visible, tt.total, got, tt.want)
			}
		})
	}
}

func TestEnsureVisibleKeepsOffsetsInRange(t *testing.T) {
	m := newTestModel(20)
	m.width = 80
	m.height = 20
	m.stationIndex = 19

	m.ensureVisible()

	if m.stationOffset > len(m.cfg.Stations())-1 {
		t.Fatalf("station offset %d out of range", m.stationOffset)
	}
	if got := m.stationOffset; got != 19-m.layout().stationVisible+1 {
		t.Fatalf("station offset = %d, want %d", got, 19-m.layout().stationVisible+1)
	}
}

func TestEnsureVisibleIsNoopOnSmallTerminal(t *testing.T) {
	m := newTestModel(20)
	m.width = 10
	m.height = 5
	m.stationIndex = 19
	m.stationOffset = 7

	m.ensureVisible()

	if m.stationOffset != 7 {
		t.Fatalf("station offset = %d, want 7", m.stationOffset)
	}
}
