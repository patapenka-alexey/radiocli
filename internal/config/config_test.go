package config

import (
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("RADIOCLI_CONFIG", path)

	cfg := Load()
	if err := cfg.AddStation("Test Station", "https://example.com/stream"); err != nil {
		t.Fatalf("AddStation: %v", err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reloaded := Load()
	stations := reloaded.Stations()
	found := false
	for _, s := range stations {
		if s.Name == "Test Station" && s.URL == "https://example.com/stream" {
			found = true
		}
	}

	if !found {
		t.Fatalf("saved station not found after reload: %+v", stations)
	}
	if reloaded.MpvPath() != "mpv" {
		t.Fatalf("MpvPath() = %q, want %q", reloaded.MpvPath(), "mpv")
	}
}

func TestAddStationRejectsDuplicates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("RADIOCLI_CONFIG", path)

	cfg := Load()
	if err := cfg.AddStation("Same", "https://example.com/a"); err != nil {
		t.Fatalf("AddStation: %v", err)
	}
	if err := cfg.AddStation("Same", "https://example.com/b"); err == nil {
		t.Fatal("expected duplicate name error")
	}
	if err := cfg.AddStation("Other", "https://example.com/a"); err == nil {
		t.Fatal("expected duplicate url error")
	}
}

func TestRemoveStation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("RADIOCLI_CONFIG", path)

	cfg := Load()
	before := len(cfg.Stations())
	cfg.RemoveStation("Nonexistent Station")

	if got := len(cfg.Stations()); got != before {
		t.Fatalf("remove of nonexistent station changed count: %d -> %d", before, got)
	}

	name := cfg.Stations()[0].Name
	cfg.RemoveStation(name)

	for _, s := range cfg.Stations() {
		if s.Name == name {
			t.Fatalf("station %q still present after removal", name)
		}
	}
}
