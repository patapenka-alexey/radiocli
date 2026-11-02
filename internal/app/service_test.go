package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"radiocli/internal/equalizer"
	"radiocli/internal/ports"
	"radiocli/internal/station"
)

type fakeStore struct {
	stations  []station.Station
	addErr    error
	saveErr   error
	saveCalls int
	removed   []string

	eqEnabled bool
	eqPreset  string
	eqMixin   float64
	eqSaves   int
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

func (f *fakeStore) EqualizerEnabled() bool { return f.eqEnabled }

func (f *fakeStore) EqualizerPreset() string { return f.eqPreset }

func (f *fakeStore) EqualizerMixin() float64 { return f.eqMixin }

func (f *fakeStore) SaveEqualizer(enabled bool, preset string) error {
	f.eqEnabled = enabled
	f.eqPreset = preset
	f.eqSaves++

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
	filter  string
}

func (f *fakePlayer) Play(url string) { f.played = append(f.played, url) }

func (f *fakePlayer) TogglePause() { f.toggles++ }

func (f *fakePlayer) Stop() { f.stops++ }

func (f *fakePlayer) SetAudioFilter(filter string) { f.filter = filter }

func (f *fakePlayer) Status() ports.PlayerStatus { return f.status }

func (f *fakePlayer) Alive() bool { return f.alive }

func (f *fakePlayer) Quit() {}

func (f *fakePlayer) Events() <-chan ports.Event { return f.events }

func newTestService(store ports.StationStore, catalog ports.Catalog, player ports.Player) *Service {
	return New(store, catalog, player, nil, nil)
}

func TestPlaySetsCurrentStation(t *testing.T) {
	pl := &fakePlayer{alive: true}
	svc := newTestService(&fakeStore{}, &fakeCatalog{}, pl)

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
	svc := newTestService(&fakeStore{}, &fakeCatalog{}, pl)

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
	svc := newTestService(&fakeStore{}, &fakeCatalog{}, pl)

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
	svc := newTestService(store, &fakeCatalog{}, &fakePlayer{})

	if err := svc.AddStation("Jazz", "https://example.com/jazz"); err != nil {
		t.Fatalf("AddStation: %v", err)
	}

	if store.saveCalls != 1 {
		t.Fatalf("saveCalls = %d, want 1", store.saveCalls)
	}
}

func TestAddStationDoesNotSaveOnValidationError(t *testing.T) {
	store := &fakeStore{addErr: errors.New("duplicate")}
	svc := newTestService(store, &fakeCatalog{}, &fakePlayer{})

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
	svc := newTestService(store, &fakeCatalog{}, pl)

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
	svc := newTestService(&fakeStore{}, catalog, &fakePlayer{})

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
	svc := newTestService(&fakeStore{}, &fakeCatalog{}, pl)

	if got := svc.Events(); got != events {
		t.Fatal("Events() did not return the player event channel")
	}
}

func testPresets() *equalizer.Library {
	return equalizer.NewLibrary([]equalizer.Preset{
		{Name: "a", Bands: []equalizer.Band{{Frequency: 100, Width: 1.5, Gain: -4}}},
		{Name: "b", Bands: []equalizer.Band{{Frequency: 1000, Width: 1.5, Gain: 2}}},
	})
}

func TestEqualizerToggleAppliesFilterAndPersists(t *testing.T) {
	store := &fakeStore{eqMixin: equalizer.DefaultMixin}
	pl := &fakePlayer{}
	svc := New(store, &fakeCatalog{}, pl, store, testPresets())

	if pl.filter != "" {
		t.Fatalf("initial filter = %q, want empty", pl.filter)
	}

	if err := svc.ToggleEqualizer(); err != nil {
		t.Fatalf("ToggleEqualizer: %v", err)
	}

	want := equalizer.Preset{Name: "a", Bands: []equalizer.Band{{Frequency: 100, Width: 1.5, Gain: -4}}}.Filter()
	if pl.filter != want {
		t.Fatalf("filter = %q, want %q", pl.filter, want)
	}
	if !store.eqEnabled || store.eqPreset != "a" {
		t.Fatalf("saved equalizer = %v, %q", store.eqEnabled, store.eqPreset)
	}

	if err := svc.ToggleEqualizer(); err != nil {
		t.Fatalf("ToggleEqualizer off: %v", err)
	}
	if pl.filter != "" {
		t.Fatalf("filter after disable = %q, want empty", pl.filter)
	}
	if store.eqEnabled {
		t.Fatal("equalizer still enabled after second toggle")
	}
}

func TestEqualizerRestoredFromSettings(t *testing.T) {
	store := &fakeStore{eqEnabled: true, eqPreset: "b", eqMixin: equalizer.DefaultMixin}
	pl := &fakePlayer{}

	svc := New(store, &fakeCatalog{}, pl, store, testPresets())

	want := equalizer.Preset{Name: "b", Bands: []equalizer.Band{{Frequency: 1000, Width: 1.5, Gain: 2}}}.Filter()
	if pl.filter != want {
		t.Fatalf("restored filter = %q, want %q", pl.filter, want)
	}

	status := svc.EqualizerStatus()
	if !status.Enabled || status.Preset != "b" || status.Index != 1 || status.Count != 2 {
		t.Fatalf("status = %+v", status)
	}
	if status.Mixin != equalizer.DefaultMixin {
		t.Fatalf("status mixin = %v, want %v", status.Mixin, equalizer.DefaultMixin)
	}
}

func TestNextEqualizerCyclesAndPersists(t *testing.T) {
	store := &fakeStore{eqEnabled: true, eqPreset: "a"}
	pl := &fakePlayer{}
	svc := New(store, &fakeCatalog{}, pl, store, testPresets())

	if err := svc.NextEqualizer(); err != nil {
		t.Fatalf("NextEqualizer: %v", err)
	}

	if store.eqPreset != "b" {
		t.Fatalf("saved preset = %q, want b", store.eqPreset)
	}
	if status := svc.EqualizerStatus(); status.Preset != "b" {
		t.Fatalf("status preset = %q, want b", status.Preset)
	}
	if pl.filter == "" {
		t.Fatal("filter was not re-applied")
	}
}

func TestRefreshEqualizerReloadsPresetDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x"), []byte("EQSET\n100 1.5 -4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "y"), []byte("EQSET\n200 1.5 -2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RADIOCLI_EQSETS", dir)

	store := &fakeStore{}
	svc := New(store, &fakeCatalog{}, &fakePlayer{}, store, testPresets())

	if err := svc.RefreshEqualizer(); err != nil {
		t.Fatalf("RefreshEqualizer: %v", err)
	}

	status := svc.EqualizerStatus()
	if status.Count != 2 || status.Preset != "x" {
		t.Fatalf("status = %+v", status)
	}
	if store.eqPreset != "x" {
		t.Fatalf("saved preset = %q, want x", store.eqPreset)
	}
}

func TestToggleEqualizerWithoutPresets(t *testing.T) {
	svc := New(&fakeStore{}, &fakeCatalog{}, &fakePlayer{}, nil, nil)

	if err := svc.ToggleEqualizer(); !errors.Is(err, ErrNoEqualizerPresets) {
		t.Fatalf("ToggleEqualizer error = %v, want ErrNoEqualizerPresets", err)
	}
}
