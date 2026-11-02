package equalizer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const samplePreset = `# comment
EQSET
0 1.4
100 1.5 -4
4000 1.5 2
`

func TestParse(t *testing.T) {
	preset, err := Parse("sample", strings.NewReader(samplePreset))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if preset.Name != "sample" {
		t.Fatalf("Name = %q, want sample", preset.Name)
	}
	if preset.Preamp != 1.4 {
		t.Fatalf("Preamp = %v, want 1.4", preset.Preamp)
	}
	if len(preset.Bands) != 2 {
		t.Fatalf("len(Bands) = %d, want 2", len(preset.Bands))
	}
	if preset.Bands[0] != (Band{Frequency: 100, Width: 1.5, Gain: -4}) {
		t.Fatalf("first band = %+v", preset.Bands[0])
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"missing header", "100 1.5 -4\n"},
		{"bad header", "NOT_EQSET\n100 1.5 -4\n"},
		{"no bands", "EQSET\n0 1.4\n"},
		{"bad frequency", "EQSET\nx 1.5 -4\n"},
		{"zero frequency", "EQSET\n0 1.5 -4\n"},
		{"zero width", "EQSET\n100 0 -4\n"},
		{"wrong field count", "EQSET\n100 1.5\n"},
		{"bad preamp", "EQSET\n0 loud\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Parse("bad", strings.NewReader(tt.input)); err == nil {
				t.Fatal("expected parse error")
			}
		})
	}
}

func TestFilter(t *testing.T) {
	preset, err := Parse("sample", strings.NewReader(samplePreset))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	want := "lavfi=[volume=1.4dB,equalizer=f=100:width_type=o:width=1.5:g=-4,equalizer=f=4000:width_type=o:width=1.5:g=2]"
	if got := preset.FilterWithMixin(0); got != want {
		t.Fatalf("FilterWithMixin(0) = %q, want %q", got, want)
	}
}

func TestFilterDefaultMixin(t *testing.T) {
	preset, err := Parse("sample", strings.NewReader(samplePreset))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	want := "lavfi=[asplit=2[dry][wet];[wet]volume=1.4dB,equalizer=f=100:width_type=o:width=1.5:g=-4,equalizer=f=4000:width_type=o:width=1.5:g=2[eq];[dry][eq]amix=inputs=2:weights=0.25 0.75:normalize=0]"
	if got := preset.Filter(); got != want {
		t.Fatalf("Filter() = %q, want %q", got, want)
	}
}

func TestFilterMixinDisabled(t *testing.T) {
	preset, err := Parse("sample", strings.NewReader(samplePreset))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got := preset.FilterWithMixin(1); got != "" {
		t.Fatalf("FilterWithMixin(1) = %q, want empty", got)
	}
}

func TestFilterWithoutPreamp(t *testing.T) {
	preset, err := Parse("sample", strings.NewReader("EQSET\n100 1.5 -4\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	want := "lavfi=[equalizer=f=100:width_type=o:width=1.5:g=-4]"
	if got := preset.FilterWithMixin(0); got != want {
		t.Fatalf("FilterWithMixin(0) = %q, want %q", got, want)
	}
}

func TestLoadDirSkipsInvalidFilesAndSorts(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"b_preset":  "EQSET\n100 1.5 -4\n",
		"a_preset":  "EQSET\n200 1.5 -2\n",
		"README.md": "# not a preset\n",
		"broken":    "EQSET\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	presets, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}

	if len(presets) != 2 {
		t.Fatalf("len(presets) = %d, want 2", len(presets))
	}
	if presets[0].Name != "a_preset" || presets[1].Name != "b_preset" {
		t.Fatalf("presets = %q, %q", presets[0].Name, presets[1].Name)
	}
}

func TestLoadFallsBackToEmbedded(t *testing.T) {
	presets := Load(filepath.Join(t.TempDir(), "missing"))
	if len(presets) == 0 {
		t.Fatal("Load returned no presets")
	}
	if presets[0].Name != "wa_Classical" {
		t.Fatalf("first preset = %q, want wa_Classical", presets[0].Name)
	}
}

func TestEmbeddedPresets(t *testing.T) {
	presets, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}
	if len(presets) < 17 {
		t.Fatalf("len(presets) = %d, want at least 17", len(presets))
	}
}

func TestLibraryNextAndReload(t *testing.T) {
	lib := NewLibrary([]Preset{{Name: "a"}, {Name: "b"}, {Name: "c"}})

	if preset, ok := lib.Next(); !ok || preset.Name != "b" {
		t.Fatalf("Next() = %q, %v", preset.Name, ok)
	}
	lib.Next()
	if preset, _ := lib.Next(); preset.Name != "a" {
		t.Fatalf("Next() did not wrap: %q", preset.Name)
	}

	lib.Select("c")
	lib.Reload([]Preset{{Name: "b"}, {Name: "c"}, {Name: "d"}})
	if preset, _ := lib.Current(); preset.Name != "c" {
		t.Fatalf("Reload lost selected preset: %q", preset.Name)
	}

	lib.Reload([]Preset{{Name: "x"}, {Name: "y"}})
	if preset, _ := lib.Current(); preset.Name != "x" {
		t.Fatalf("Reload fallback = %q, want x", preset.Name)
	}
}

func TestEmptyLibrary(t *testing.T) {
	lib := NewLibrary(nil)

	if _, ok := lib.Current(); ok {
		t.Fatal("Current() on empty library returned ok")
	}
	if _, ok := lib.Next(); ok {
		t.Fatal("Next() on empty library returned ok")
	}
}

func TestLoadUsesFirstDirWithPresets(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	if err := os.WriteFile(filepath.Join(first, "a_preset"), []byte("EQSET\n100 1.5 -4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(second, "b_preset"), []byte("EQSET\n200 1.5 -2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	presets := Load(first, second)
	if len(presets) != 1 || presets[0].Name != "a_preset" {
		t.Fatalf("presets = %+v, want only a_preset", presets)
	}

	empty := t.TempDir()
	presets = Load(empty, second)
	if len(presets) != 1 || presets[0].Name != "b_preset" {
		t.Fatalf("presets from fallback dir = %+v, want only b_preset", presets)
	}
}
