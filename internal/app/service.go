// Package app contains the application service that coordinates the station
// store, catalog and player ports.
package app

import (
	"errors"

	"radiocli/internal/equalizer"
	"radiocli/internal/paths"
	"radiocli/internal/ports"
	"radiocli/internal/station"
)

// ErrPlayerNotRunning is returned when playback is requested while mpv is gone.
var ErrPlayerNotRunning = errors.New("mpv is not running")

// ErrNoEqualizerPresets is returned when no equalizer preset is available.
var ErrNoEqualizerPresets = errors.New("no equalizer presets available")

// Service is the application facade used by the TUI and other frontends.
type Service struct {
	store    ports.StationStore
	catalog  ports.Catalog
	player   ports.Player
	settings ports.EqualizerSettings

	equalizer        *equalizer.Library
	equalizerEnabled bool
	equalizerMixin   float64

	currentStation string
}

// New creates an application service.
func New(store ports.StationStore, catalog ports.Catalog, player ports.Player, settings ports.EqualizerSettings, presets *equalizer.Library) *Service {
	if presets == nil {
		presets = equalizer.NewLibrary(nil)
	}

	svc := &Service{
		store:          store,
		catalog:        catalog,
		player:         player,
		settings:       settings,
		equalizer:      presets,
		equalizerMixin: equalizer.DefaultMixin,
	}

	if settings != nil {
		if name := settings.EqualizerPreset(); name != "" {
			presets.Select(name)
		}
		svc.equalizerEnabled = settings.EqualizerEnabled()
		svc.equalizerMixin = settings.EqualizerMixin()
	}

	svc.applyEqualizer()

	return svc
}

// Stations returns the user's station list.
func (s *Service) Stations() []station.Station {
	return s.store.Stations()
}

// AddStation validates, appends and persists a station.
func (s *Service) AddStation(name, url string) error {
	if err := s.store.AddStation(name, url); err != nil {
		return err
	}

	return s.store.Save()
}

// RemoveStation removes a station and persists the list.
func (s *Service) RemoveStation(name string) error {
	s.store.RemoveStation(name)
	if s.currentStation == name {
		s.currentStation = ""
	}

	return s.store.Save()
}

// Searches returns matching stations from the catalog.
func (s *Service) Search(query string, limit int) []station.Station {
	return s.catalog.Search(query, limit)
}

// Refresh re-downloads the station catalog.
func (s *Service) Refresh() error {
	return s.catalog.Refresh()
}

// StationCount returns the number of cached catalog entries.
func (s *Service) StationCount() int {
	return s.catalog.Count()
}

// CatalogAvailable reports whether a station catalog is cached.
func (s *Service) CatalogAvailable() bool {
	return s.catalog.Available()
}

// Play starts playback of a station.
func (s *Service) Play(st station.Station) error {
	if !s.player.Alive() {
		return ErrPlayerNotRunning
	}

	s.currentStation = st.Name
	s.player.Play(st.URL)

	return nil
}

// TogglePause toggles playback pause.
func (s *Service) TogglePause() {
	s.player.TogglePause()
}

// Stop stops playback and clears the current station.
func (s *Service) Stop() {
	s.player.Stop()
	s.currentStation = ""
}

// Status returns the current player status.
func (s *Service) Status() ports.PlayerStatus {
	return s.player.Status()
}

// Alive reports whether the player process is running.
func (s *Service) Alive() bool {
	return s.player.Alive()
}

// CurrentStation returns the name of the station being played, if any.
func (s *Service) CurrentStation() string {
	return s.currentStation
}

// EqualizerStatus is a point-in-time view of the equalizer state.
type EqualizerStatus struct {
	Enabled bool
	Preset  string
	Index   int
	Count   int
	Mixin   float64
}

// EqualizerStatus returns the current equalizer state.
func (s *Service) EqualizerStatus() EqualizerStatus {
	status := EqualizerStatus{
		Enabled: s.equalizerEnabled,
		Index:   s.equalizer.Index(),
		Count:   s.equalizer.Len(),
		Mixin:   s.equalizerMixin,
	}
	if preset, ok := s.equalizer.Current(); ok {
		status.Preset = preset.Name
	}

	return status
}

// ToggleEqualizer enables or disables the equalizer.
func (s *Service) ToggleEqualizer() error {
	if s.equalizer.Len() == 0 {
		return ErrNoEqualizerPresets
	}

	s.equalizerEnabled = !s.equalizerEnabled
	s.applyEqualizer()

	return s.saveEqualizer()
}

// RefreshEqualizer reloads presets from the configured preset directories and
// re-applies the current filter.
func (s *Service) RefreshEqualizer() error {
	presets := equalizer.Load(paths.EqsetsPaths()...)
	if len(presets) == 0 {
		return ErrNoEqualizerPresets
	}

	s.equalizer.Reload(presets)
	s.applyEqualizer()

	return s.saveEqualizer()
}

// NextEqualizer selects the next preset and re-applies the filter.
func (s *Service) NextEqualizer() error {
	if _, ok := s.equalizer.Next(); !ok {
		return ErrNoEqualizerPresets
	}

	s.applyEqualizer()

	return s.saveEqualizer()
}

func (s *Service) applyEqualizer() {
	if s.equalizerEnabled {
		if preset, ok := s.equalizer.Current(); ok {
			s.player.SetAudioFilter(preset.FilterWithMixin(s.equalizerMixin))

			return
		}
	}

	s.player.SetAudioFilter("")
}

func (s *Service) saveEqualizer() error {
	if s.settings == nil {
		return nil
	}

	preset := ""
	if current, ok := s.equalizer.Current(); ok {
		preset = current.Name
	}

	return s.settings.SaveEqualizer(s.equalizerEnabled, preset)
}

// Events returns player state-change notifications. The channel is closed when
// the player exits.
func (s *Service) Events() <-chan ports.Event {
	return s.player.Events()
}
