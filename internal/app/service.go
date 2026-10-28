// Package app contains the application service that coordinates the station
// store, catalog and player ports.
package app

import (
	"errors"

	"radiocli/internal/ports"
	"radiocli/internal/station"
)

// ErrPlayerNotRunning is returned when playback is requested while mpv is gone.
var ErrPlayerNotRunning = errors.New("mpv is not running")

// Service is the application facade used by the TUI and other frontends.
type Service struct {
	store   ports.StationStore
	catalog ports.Catalog
	player  ports.Player

	currentStation string
}

// New creates an application service.
func New(store ports.StationStore, catalog ports.Catalog, player ports.Player) *Service {
	return &Service{store: store, catalog: catalog, player: player}
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

// Events returns player state-change notifications. The channel is closed when
// the player exits.
func (s *Service) Events() <-chan ports.Event {
	return s.player.Events()
}
