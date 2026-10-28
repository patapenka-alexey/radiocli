package paths

import (
	"path/filepath"
	"testing"
)

func TestConfigPathOverride(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, "custom.json")
	t.Setenv("RADIOCLI_CONFIG", want)

	if got := ConfigPath(); got != want {
		t.Fatalf("ConfigPath() = %q, want %q", got, want)
	}
}

func TestCatalogPathOverride(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, "stations.json")
	t.Setenv("RADIOCLI_DB", want)

	if got := CatalogPath(); got != want {
		t.Fatalf("CatalogPath() = %q, want %q", got, want)
	}
}

func TestDefaultPathsFollowConfigDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RADIOCLI_CONFIG", "")
	t.Setenv("RADIOCLI_DB", "")
	t.Setenv("XDG_CONFIG_HOME", dir)

	if got, want := ConfigPath(), filepath.Join(dir, "radiocli", "config.json"); got != want {
		t.Fatalf("ConfigPath() = %q, want %q", got, want)
	}
	if got, want := CatalogPath(), filepath.Join(dir, "radiocli", "allStations.json"); got != want {
		t.Fatalf("CatalogPath() = %q, want %q", got, want)
	}
}
