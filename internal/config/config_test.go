package config

import (
	"os"
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

func TestEqualizerRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("RADIOCLI_CONFIG", path)

	cfg := Load()
	if cfg.EqualizerEnabled() {
		t.Fatal("equalizer enabled by default")
	}
	if cfg.EqualizerPreset() != "" {
		t.Fatalf("default preset = %q, want empty", cfg.EqualizerPreset())
	}
	if cfg.EqualizerMixin() != 0.25 {
		t.Fatalf("default mixin = %v, want 0.25", cfg.EqualizerMixin())
	}

	if err := cfg.SaveEqualizer(true, "wa_Rock"); err != nil {
		t.Fatalf("SaveEqualizer: %v", err)
	}

	reloaded := Load()
	if !reloaded.EqualizerEnabled() {
		t.Fatal("equalizer enabled state was not persisted")
	}
	if reloaded.EqualizerPreset() != "wa_Rock" {
		t.Fatalf("preset = %q, want wa_Rock", reloaded.EqualizerPreset())
	}
	if reloaded.EqualizerMixin() != 0.25 {
		t.Fatalf("mixin = %v, want 0.25", reloaded.EqualizerMixin())
	}
}

func TestEqualizerMixinLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("RADIOCLI_CONFIG", path)

	content := `{"equalizer_mixin": 0.4}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if got := Load().EqualizerMixin(); got != 0.4 {
		t.Fatalf("EqualizerMixin() = %v, want 0.4", got)
	}
}
