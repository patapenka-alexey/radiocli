// Package config loads and persists the radiocli configuration.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"radiocli/internal/paths"
	"radiocli/internal/ports"
	"radiocli/internal/station"
)

var _ ports.StationStore = (*Store)(nil)

// DefaultUpdateTimeout is how long a station list update may take.
const DefaultUpdateTimeout = 30 * time.Second

func defaultStations() []station.Station {
	return []station.Station{
		{Name: "Synthwave City FM - New", URL: "https://synthwave-rex.radioca.st/stream"},
		{Name: "Smooth Chill", URL: "https://media-ssl.musicradio.com/ChillMP3"},
		{Name: "Radio Record Synthwave", URL: "http://radiorecord.hostingradio.ru/synth96.aacp"},
		{Name: "Synthwave Neon City laut.fm", URL: "https://stream.laut.fm/synthwave"},
		{Name: "Laut.fm House Beatz and Waves", URL: "http://stream.laut.fm/house_beatz_and_waves"},
		{Name: "Laut.fm Vaporwave", URL: "http://stream.laut.fm/vaporwave.m3u"},
		{Name: "ABC News Radio MP3", URL: "http://www.abc.net.au/res/streaming/audio/mp3/news_radio.pls"},
		{Name: "Радио Sputnik (новости) / Radio Sputnik (news)", URL: "https://icecast-rian.cdnvideo.ru/voicerus"},
		{Name: "Wired FM - FM 99.9 - Limerick, Ireland", URL: "https://s2.radio.co/sf0986595a/listen"},
		{Name: "KSKO-FM 89.5 McGrath, AK  (MP3 Stream)", URL: "https://ksko.streamguys1.com/live"},
		{Name: "790 AM - KCAM Glennallen Alaska", URL: "https://streams.radiomast.io/kcam"},
		{Name: "Flashback 60s", URL: "https://ice-11.spilarinn.is/60flashback"},
		{Name: "радио Минск 92.4", URL: "https://radio.mk.by:8443/radiominsk"},
		{Name: "Радио Минская Волна 97.4 FM", URL: "https://stream2.datacenter.by/mvradio"},
		{Name: "Русское радио Минск 98.9 FM", URL: "https://stream.rusradio.by:8443/live128"},
	}
}

// Store mirrors the JSON file stored at ~/.config/radiocli/config.json.
type Store struct {
	stations      []station.Station
	mpvPath       string
	updateTimeout time.Duration

	path string
}

// Load reads the configuration from disk, falling back to the built-in
// defaults only when the file is missing, unreadable or has no stations key.
func Load() *Store {
	path := paths.ConfigPath()
	cfg := &Store{mpvPath: "mpv", updateTimeout: DefaultUpdateTimeout, path: path}

	if data, err := os.ReadFile(path); err == nil {
		var raw struct {
			Stations      []station.Station `json:"stations"`
			MpvPath       *string           `json:"mpv_path"`
			UpdateTimeout json.RawMessage   `json:"update_timeout"`
			Timeout       json.RawMessage   `json:"timeout"`
		}

		if json.Unmarshal(data, &raw) == nil {
			cfg.stations = raw.Stations
			if raw.MpvPath != nil && *raw.MpvPath != "" {
				cfg.mpvPath = *raw.MpvPath
			}
			timeout := raw.UpdateTimeout
			if len(timeout) == 0 {
				timeout = raw.Timeout
			}
			if value, ok := parseTimeout(timeout); ok {
				cfg.updateTimeout = value
			}
		}
	}

	if cfg.stations == nil {
		cfg.stations = defaultStations()
	}

	return cfg
}

// parseTimeout accepts either a number of seconds or a duration string such as
// "30s" or "1m". Missing, invalid and non-positive values are ignored so the
// caller keeps the default.
func parseTimeout(raw json.RawMessage) (time.Duration, bool) {
	if len(raw) == 0 {
		return 0, false
	}

	var seconds float64
	if err := json.Unmarshal(raw, &seconds); err == nil {
		if seconds <= 0 {
			return 0, false
		}

		return time.Duration(seconds * float64(time.Second)), true
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		timeout, err := time.ParseDuration(strings.TrimSpace(text))
		if err != nil || timeout <= 0 {
			return 0, false
		}

		return timeout, true
	}

	return 0, false
}

// Stations returns a copy of the configured station list.
func (c *Store) Stations() []station.Station {
	return append([]station.Station(nil), c.stations...)
}

// MpvPath returns the configured mpv executable.
func (c *Store) MpvPath() string {
	return c.mpvPath
}

// UpdateTimeout returns the maximum duration of a station list update.
func (c *Store) UpdateTimeout() time.Duration {
	return c.updateTimeout
}

// AddStation appends a station, enforcing the same uniqueness rules as the
// applet: both the name and the url must be unique.
func (c *Store) AddStation(name, url string) error {
	name = strings.TrimSpace(name)
	url = strings.TrimSpace(url)
	if name == "" || url == "" {
		return errors.New("station must have a name and a url")
	}

	for _, s := range c.stations {
		if s.URL == url {
			return fmt.Errorf("another station (%s) already uses this url", s.Name)
		}

		if s.Name == name {
			return fmt.Errorf("a station named '%s' already exists; rename the station", name)
		}
	}

	c.stations = append(c.stations, station.Station{Name: name, URL: url})

	return nil
}

// RemoveStation removes every station with the given name.
func (c *Store) RemoveStation(name string) {
	filtered := c.stations[:0]

	for _, s := range c.stations {
		if s.Name != name {
			filtered = append(filtered, s)
		}
	}

	c.stations = filtered
}

// Save writes the configuration atomically to disk.
func (c *Store) Save() error {
	dir := filepath.Dir(c.path)
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	tmp := c.path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")

	stations := c.stations
	if stations == nil {
		stations = []station.Station{}
	}

	file := struct {
		Stations      []station.Station `json:"stations"`
		MpvPath       string            `json:"mpv_path"`
		UpdateTimeout float64           `json:"update_timeout"`
	}{
		Stations:      stations,
		MpvPath:       c.mpvPath,
		UpdateTimeout: c.updateTimeout.Seconds(),
	}

	if err := enc.Encode(file); err != nil {
		f.Close()

		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	return os.Rename(tmp, c.path)
}
