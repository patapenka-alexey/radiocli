// Package ports defines the interfaces between the application core and the
// infrastructure adapters (filesystem, mpv, radio-browser.info).
package ports

import "radiocli/internal/station"

// StationStore is the persistence port for the user's station list.
type StationStore interface {
	Stations() []station.Station
	AddStation(name, url string) error
	RemoveStation(name string)
	Save() error
}

// Catalog is the read/refresh port for the searchable station database.
type Catalog interface {
	Search(query string, limit int) []station.Station
	Refresh() error
	Count() int
	Available() bool
}

// PlayerStatus is a point-in-time view of the playback state.
type PlayerStatus struct {
	Title       string
	Paused      bool
	Idle        bool
	TimePos     float64
	HasTimePos  bool
	Duration    float64
	HasDuration bool
	Alive       bool
}

// Player is the playback control port.
type Player interface {
	Play(url string)
	TogglePause()
	Stop()
	Status() PlayerStatus
	Alive() bool
	Quit()
}
