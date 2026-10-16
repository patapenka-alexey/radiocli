package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Station is a single named radio stream.
type Station struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

func defaultStations() []Station {
	return []Station{
		{"Synthwave City FM - New", "https://synthwave-rex.radioca.st/stream"},
		{"Smooth Chill", "https://media-ssl.musicradio.com/ChillMP3"},
		{"Radio Record Synthwave", "http://radiorecord.hostingradio.ru/synth96.aacp"},
		{"Synthwave Neon City laut.fm", "https://stream.laut.fm/synthwave"},
		{"Laut.fm House Beatz and Waves", "http://stream.laut.fm/house_beatz_and_waves"},
		{"Laut.fm Vaporwave", "http://stream.laut.fm/vaporwave.m3u"},
		{"ABC News Radio MP3", "http://www.abc.net.au/res/streaming/audio/mp3/news_radio.pls"},
		{"Радио Sputnik (новости) / Radio Sputnik (news)", "https://icecast-rian.cdnvideo.ru/voicerus"},
		{"Wired FM - FM 99.9 - Limerick, Ireland", "https://s2.radio.co/sf0986595a/listen"},
		{"KSKO-FM 89.5 McGrath, AK  (MP3 Stream)", "https://ksko.streamguys1.com/live"},
		{"790 AM - KCAM Glennallen Alaska", "https://streams.radiomast.io/kcam"},
		{"Flashback 60s", "https://ice-11.spilarinn.is/60flashback"},
		{"радио Минск 92.4", "https://radio.mk.by:8443/radiominsk"},
		{"Радио Минская Волна 97.4 FM", "https://stream2.datacenter.by/mvradio"},
		{"Русское радио Минск 98.9 FM", "https://stream.rusradio.by:8443/live128"},
	}
}

// config mirrors the JSON file stored at ~/.config/radiocli/config.json.
type config struct {
	Stations []Station `json:"stations"`
	MpvPath  string    `json:"mpv_path"`

	path string
}

// configPath returns the location of config.json. It defaults to
// ~/.config/radiocli/config.json, following $XDG_CONFIG_HOME; RADIOCLI_CONFIG
// overrides the location.
func configPath() string {
	if override := os.Getenv("RADIOCLI_CONFIG"); override != "" {
		if abs, err := filepath.Abs(override); err == nil {
			return abs
		}
		return override
	}
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "radiocli", "config.json")
	}
	return "config.json"
}

func loadConfig() *config {
	path := configPath()
	cfg := &config{MpvPath: "mpv", path: path}

	if data, err := os.ReadFile(path); err == nil {
		var raw struct {
			Stations []Station `json:"stations"`
			MpvPath  *string   `json:"mpv_path"`
		}
		if json.Unmarshal(data, &raw) == nil {
			cfg.Stations = raw.Stations
			if raw.MpvPath != nil && *raw.MpvPath != "" {
				cfg.MpvPath = *raw.MpvPath
			}
		}
	}

	if len(cfg.Stations) == 0 {
		cfg.Stations = defaultStations()
	}
	return cfg
}

// addStation appends a station, enforcing the same uniqueness rules as the
// applet: both the name and the url must be unique.
func (c *config) addStation(name, url string) error {
	name = strings.TrimSpace(name)
	url = strings.TrimSpace(url)
	if name == "" || url == "" {
		return errors.New("Station must have a name and a url")
	}
	for _, s := range c.Stations {
		if s.URL == url {
			return fmt.Errorf("Another station (%s) already uses this url", s.Name)
		}
		if s.Name == name {
			return fmt.Errorf("A station named '%s' already exists. Rename the station.", name)
		}
	}
	c.Stations = append(c.Stations, Station{Name: name, URL: url})
	return nil
}

func (c *config) removeStation(name string) {
	filtered := c.Stations[:0]
	for _, s := range c.Stations {
		if s.Name != name {
			filtered = append(filtered, s)
		}
	}
	c.Stations = filtered
}

func (c *config) save() error {
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
	if err := enc.Encode(c); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, c.path)
}
