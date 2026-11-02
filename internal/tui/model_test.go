package tui

import (
	"strings"
	"testing"

	"radiocli/internal/app"
	"radiocli/internal/equalizer"
	"radiocli/internal/ports"
	"radiocli/internal/station"
)

type fakeStore struct {
	stations []station.Station

	eqEnabled bool
	eqPreset  string
	eqMixin   float64
}

func (f *fakeStore) Stations() []station.Station { return f.stations }

func (f *fakeStore) AddStation(name, url string) error {
	f.stations = append(f.stations, station.Station{Name: name, URL: url})

	return nil
}

func (f *fakeStore) RemoveStation(name string) {
	filtered := f.stations[:0]
	for _, s := range f.stations {
		if s.Name != name {
			filtered = append(filtered, s)
		}
	}
	f.stations = filtered
}

func (f *fakeStore) Save() error { return nil }

func (f *fakeStore) EqualizerEnabled() bool { return f.eqEnabled }

func (f *fakeStore) EqualizerPreset() string { return f.eqPreset }

func (f *fakeStore) EqualizerMixin() float64 { return f.eqMixin }

func (f *fakeStore) SaveEqualizer(enabled bool, preset string) error {
	f.eqEnabled = enabled
	f.eqPreset = preset

	return nil
}

type fakeCatalog struct{}

func (fakeCatalog) Search(string, int) []station.Station { return nil }

func (fakeCatalog) Refresh() error { return nil }

func (fakeCatalog) Count() int { return 0 }

func (fakeCatalog) Available() bool { return false }

type fakePlayer struct{}

func (fakePlayer) Play(string) {}

func (fakePlayer) TogglePause() {}

func (fakePlayer) Stop() {}

func (fakePlayer) SetAudioFilter(string) {}

func (fakePlayer) Status() ports.PlayerStatus { return ports.PlayerStatus{Alive: true} }

func (fakePlayer) Alive() bool { return true }

func (fakePlayer) Quit() {}

func (fakePlayer) Events() <-chan ports.Event { return nil }

func testLibrary() *equalizer.Library {
	return equalizer.NewLibrary([]equalizer.Preset{
		{Name: "a", Bands: []equalizer.Band{{Frequency: 100, Width: 1.5, Gain: -4}}},
		{Name: "b", Bands: []equalizer.Band{{Frequency: 1000, Width: 1.5, Gain: 2}}},
	})
}

func newTestModel(stations int) *Model {
	return newTestModelWith(make([]station.Station, stations)...)
}

func newTestModelWith(stations ...station.Station) *Model {
	store := &fakeStore{stations: stations, eqMixin: equalizer.DefaultMixin}

	return NewModel(app.New(store, fakeCatalog{}, fakePlayer{}, store, testLibrary()))
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

	if m.stationOffset > len(m.app.Stations())-1 {
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

func TestFooterSeparatorAboveHints(t *testing.T) {
	m := newTestModel(1)

	lines := m.footerLines(80, 40, 2)

	if lines[0] != footerSeparatorLine(40, 80) {
		t.Fatalf("separator = %q, want %q", lines[0], footerSeparatorLine(40, 80))
	}
	if !strings.Contains(lines[1], "move") {
		t.Fatalf("hint line = %q", lines[1])
	}
}

func TestEqualizerLabel(t *testing.T) {
	m := newTestModel(0)

	if label, _ := m.equalizerLabel(); label != "Off · a (1/2)" {
		t.Fatalf("initial label = %q, want Off · a (1/2)", label)
	}

	if err := m.app.ToggleEqualizer(); err != nil {
		t.Fatalf("ToggleEqualizer: %v", err)
	}

	if label, _ := m.equalizerLabel(); label != "a (1/2)" {
		t.Fatalf("enabled label = %q, want a (1/2)", label)
	}

	if err := m.app.NextEqualizer(); err != nil {
		t.Fatalf("NextEqualizer: %v", err)
	}

	if label, _ := m.equalizerLabel(); label != "b (2/2)" {
		t.Fatalf("next label = %q, want b (2/2)", label)
	}
}
