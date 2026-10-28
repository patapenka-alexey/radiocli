package app

import (
	"errors"
	"testing"

	"radiocli/internal/ports"
	"radiocli/internal/station"
)

type fakeStore struct {
	stations  []station.Station
	addErr    error
	saveErr   error
	saveCalls int
	removed   []string
}

func (f *fakeStore) Stations() []station.Station { return f.stations }

func (f *fakeStore) AddStation(name, url string) error {
	if f.addErr != nil {
		return f.addErr
	}
	f.stations = append(f.stations, station.Station{Name: name, URL: url})

	return nil
}

func (f *fakeStore) RemoveStation(name string) {
	f.removed = append(f.removed, name)
	filtered := f.stations[:0]
	for _, s := range f.stations {
		if s.Name != name {
			filtered = append(filtered, s)
		}
	}
	f.stations = filtered
}

func (f *fakeStore) Save() error {
	f.saveCalls++

	return f.saveErr
}

type fakeCatalog struct {
	results    []station.Station
	refreshErr error
	limited    int
}

func (f *fakeCatalog) Search(query string, limit int) []station.Station {
	f.limited = limit

	return f.results
}

func (f *fakeCatalog) Refresh() error { return f.refreshErr }

func (f *fakeCatalog) Count() int { return len(f.results) }

func (f *fakeCatalog) Available() bool { return len(f.results) > 0 }

type fakePlayer struct {
	alive   bool
	played  []string
	toggles int
	stops   int
	status  ports.PlayerStatus
	events  chan ports.Event
}

func (f *fakePlayer) Play(url string) { f.played = append(f.played, url) }

func (f *fakePlayer) TogglePause() { f.toggles++ }

func (f *fakePlayer) Stop() { f.stops++ }

func (f *fakePlayer) Status() ports.PlayerStatus { return f.status }

func (f *fakePlayer) Alive() bool { return f.alive }

func (f *fakePlayer) Quit() {}

func (f *fakePlayer) Events() <-chan ports.Event { return f.events }

func TestPlaySetsCurrentStation(t *testing.T) {
	pl := &fakePlayer{alive: true}
	svc := New(&fakeStore{}, &fakeCatalog{}, pl)

	if err := svc.Play(station.Station{Name: "Jazz", URL: "https://example.com/jazz"}); err != nil {
		t.Fatalf("Play: %v", err)
	}

	if svc.CurrentStation() != "Jazz" {
		t.Fatalf("CurrentStation() = %q, want Jazz", svc.CurrentStation())
	}
	if len(pl.played) != 1 || pl.played[0] != "https://example.com/jazz" {
		t.Fatalf("played = %v", pl.played)
	}
}

func TestPlayFailsWhenPlayerIsGone(t *testing.T) {
	pl := &fakePlayer{alive: false}
	svc := New(&fakeStore{}, &fakeCatalog{}, pl)

	err := svc.Play(station.Station{Name: "Jazz", URL: "https://example.com/jazz"})

	if !errors.Is(err, ErrPlayerNotRunning) {
		t.Fatalf("Play error = %v, want ErrPlayerNotRunning", err)
	}
	if svc.CurrentStation() != "" {
		t.Fatalf("CurrentStation() = %q, want empty", svc.CurrentStation())
	}
	if len(pl.played) != 0 {
		t.Fatalf("played = %v, want none", pl.played)
	}
}

func TestStopClearsCurrentStation(t *testing.T) {
	pl := &fakePlayer{alive: true}
	svc := New(&fakeStore{}, &fakeCatalog{}, pl)

	if err := svc.Play(station.Station{Name: "Jazz", URL: "https://example.com/jazz"}); err != nil {
		t.Fatalf("Play: %v", err)
	}
	svc.Stop()

	if svc.CurrentStation() != "" {
		t.Fatalf("CurrentStation() = %q, want empty", svc.CurrentStation())
	}
	if pl.stops != 1 {
		t.Fatalf("stops = %d, want 1", pl.stops)
	}
}

func TestAddStationPersists(t *testing.T) {
	store := &fakeStore{}
	svc := New(store, &fakeCatalog{}, &fakePlayer{})

	if err := svc.AddStation("Jazz", "https://example.com/jazz"); err != nil {
		t.Fatalf("AddStation: %v", err)
	}

	if store.saveCalls != 1 {
		t.Fatalf("saveCalls = %d, want 1", store.saveCalls)
	}
}

func TestAddStationDoesNotSaveOnValidationError(t *testing.T) {
	store := &fakeStore{addErr: errors.New("duplicate")}
	svc := New(store, &fakeCatalog{}, &fakePlayer{})

	if err := svc.AddStation("Jazz", "https://example.com/jazz"); err == nil {
		t.Fatal("expected AddStation error")
	}

	if store.saveCalls != 0 {
		t.Fatalf("saveCalls = %d, want 0", store.saveCalls)
	}
}

func TestRemoveStationClearsCurrentAndPersists(t *testing.T) {
	store := &fakeStore{stations: []station.Station{{Name: "Jazz", URL: "https://example.com/jazz"}}}
	pl := &fakePlayer{alive: true}
	svc := New(store, &fakeCatalog{}, pl)

	if err := svc.Play(store.stations[0]); err != nil {
		t.Fatalf("Play: %v", err)
	}

	if err := svc.RemoveStation("Jazz"); err != nil {
		t.Fatalf("RemoveStation: %v", err)
	}

	if svc.CurrentStation() != "" {
		t.Fatalf("CurrentStation() = %q, want empty", svc.CurrentStation())
	}
	if store.saveCalls != 1 {
		t.Fatalf("saveCalls = %d, want 1", store.saveCalls)
	}
}

func TestSearchDelegatesToCatalog(t *testing.T) {
	catalog := &fakeCatalog{results: []station.Station{{Name: "Jazz"}}}
	svc := New(&fakeStore{}, catalog, &fakePlayer{})

	got := svc.Search("jazz", 10)

	if len(got) != 1 || got[0].Name != "Jazz" {
		t.Fatalf("Search results = %v", got)
	}
	if catalog.limited != 10 {
		t.Fatalf("limit = %d, want 10", catalog.limited)
	}
}

func TestEventsDelegatesToPlayer(t *testing.T) {
	events := make(chan ports.Event)
	pl := &fakePlayer{events: events}
	svc := New(&fakeStore{}, &fakeCatalog{}, pl)

	if got := svc.Events(); got != events {
		t.Fatal("Events() did not return the player event channel")
	}
}
