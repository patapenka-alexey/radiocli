// Package equalizer parses MOC equalizer presets and translates them into
// mpv audio filters.
package equalizer

import (
	"bufio"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strconv"
	"strings"
)

//go:embed eqsets
var embedded embed.FS

// Band is a single peaking equalizer band.
type Band struct {
	Frequency float64
	Width     float64
	Gain      float64
}

// Preset is a named MOC equalizer preset.
type Preset struct {
	Name   string
	Preamp float64
	Bands  []Band
}

// Parse parses a MOC EQSET file.
func Parse(name string, r io.Reader) (Preset, error) {
	p := Preset{Name: name}
	scanner := bufio.NewScanner(r)
	seenHeader := false
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if !seenHeader {
			if !strings.EqualFold(line, "EQSET") {
				return Preset{}, fmt.Errorf("line %d: expected EQSET header", lineNo)
			}
			seenHeader = true

			continue
		}

		fields := strings.Fields(line)
		switch {
		case len(fields) == 2 && fields[0] == "0":
			preamp, err := parseFloat(fields[1])
			if err != nil {
				return Preset{}, fmt.Errorf("line %d: invalid preamp: %w", lineNo, err)
			}
			p.Preamp = preamp

		case len(fields) == 3:
			freq, err := parseFloat(fields[0])
			if err != nil {
				return Preset{}, fmt.Errorf("line %d: invalid frequency: %w", lineNo, err)
			}
			width, err := parseFloat(fields[1])
			if err != nil {
				return Preset{}, fmt.Errorf("line %d: invalid width: %w", lineNo, err)
			}
			gain, err := parseFloat(fields[2])
			if err != nil {
				return Preset{}, fmt.Errorf("line %d: invalid gain: %w", lineNo, err)
			}
			if freq <= 0 {
				return Preset{}, fmt.Errorf("line %d: frequency must be positive", lineNo)
			}
			if width <= 0 {
				return Preset{}, fmt.Errorf("line %d: width must be positive", lineNo)
			}
			p.Bands = append(p.Bands, Band{Frequency: freq, Width: width, Gain: gain})

		default:
			return Preset{}, fmt.Errorf("line %d: expected '<freq> <width> <gain>' or '0 <preamp>'", lineNo)
		}
	}

	if err := scanner.Err(); err != nil {
		return Preset{}, err
	}
	if !seenHeader {
		return Preset{}, errors.New("missing EQSET header")
	}
	if len(p.Bands) == 0 {
		return Preset{}, errors.New("preset has no bands")
	}

	return p, nil
}

func parseFloat(s string) (float64, error) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}

	return v, nil
}

// DefaultMixin is MOC's default equalizer dry/wet mix.
const DefaultMixin = 0.25

// Filter returns the mpv lavfi audio filter for the preset using the default
// MOC mix. It returns an empty string when the preset has no bands.
func (p Preset) Filter() string {
	return p.FilterWithMixin(DefaultMixin)
}

// FilterWithMixin returns the mpv lavfi audio filter for the preset. Mixin is
// the amount of the original signal mixed back in: 0 applies only the
// equalized signal, 1 disables equalization, and values in between blend the
// two. Values outside 0..1 are clamped.
func (p Preset) FilterWithMixin(mixin float64) string {
	mixin = clamp01(mixin)
	wet := p.filterChain()
	if wet == "" || mixin >= 1 {
		return ""
	}
	if mixin <= 0 {
		return "lavfi=[" + wet + "]"
	}

	return fmt.Sprintf(
		"lavfi=[asplit=2[dry][wet];[wet]%s[eq];[dry][eq]amix=inputs=2:weights=%s %s:normalize=0]",
		wet, formatFloat(mixin), formatFloat(1-mixin),
	)
}

func (p Preset) filterChain() string {
	parts := make([]string, 0, len(p.Bands)+1)
	if p.Preamp != 0 {
		parts = append(parts, "volume="+formatFloat(p.Preamp)+"dB")
	}
	for _, b := range p.Bands {
		parts = append(parts, fmt.Sprintf(
			"equalizer=f=%s:width_type=o:width=%s:g=%s",
			formatFloat(b.Frequency), formatFloat(b.Width), formatFloat(b.Gain),
		))
	}

	return strings.Join(parts, ",")
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}

	return v
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// LoadDir loads every valid preset from dir. Non-preset and malformed files
// are skipped.
func LoadDir(dir string) ([]Preset, error) {
	return loadFS(os.DirFS(dir))
}

// LoadEmbedded loads the presets bundled with radiocli.
func LoadEmbedded() ([]Preset, error) {
	return loadFS(mustSub(embedded, "eqsets"))
}

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}

	return sub
}

func loadFS(fsys fs.FS) ([]Preset, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}

	presets := make([]Preset, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		f, err := fsys.Open(entry.Name())
		if err != nil {
			continue
		}
		preset, err := Parse(entry.Name(), f)
		f.Close()
		if err != nil {
			continue
		}
		presets = append(presets, preset)
	}

	sort.Slice(presets, func(i, j int) bool {
		return presets[i].Name < presets[j].Name
	})

	return presets, nil
}

// Load loads presets from the first directory that contains valid presets.
// Directories are tried in order; when none has presets, it falls back to the
// embedded presets.
func Load(dirs ...string) []Preset {
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		if presets, err := LoadDir(dir); err == nil && len(presets) > 0 {
			return presets
		}
	}

	presets, err := LoadEmbedded()
	if err != nil {
		return nil
	}

	return presets
}

// Library tracks the available presets and the selected one.
type Library struct {
	presets []Preset
	index   int
}

// NewLibrary creates a preset library positioned at the first preset.
func NewLibrary(presets []Preset) *Library {
	return &Library{presets: presets}
}

// Len returns the number of presets.
func (l *Library) Len() int {
	return len(l.presets)
}

// Index returns the zero-based index of the selected preset.
func (l *Library) Index() int {
	return l.index
}

// Current returns the selected preset.
func (l *Library) Current() (Preset, bool) {
	if len(l.presets) == 0 {
		return Preset{}, false
	}

	return l.presets[l.index], true
}

// Select selects the preset with the given name, preserving the index when it
// is not found.
func (l *Library) Select(name string) bool {
	for i, preset := range l.presets {
		if preset.Name == name {
			l.index = i

			return true
		}
	}

	return false
}

// Next selects the following preset, wrapping around.
func (l *Library) Next() (Preset, bool) {
	if len(l.presets) == 0 {
		return Preset{}, false
	}

	l.index = (l.index + 1) % len(l.presets)

	return l.presets[l.index], true
}

// Reload replaces the presets while keeping the selected preset if it still
// exists.
func (l *Library) Reload(presets []Preset) {
	current, hadCurrent := l.Current()
	l.presets = presets
	l.index = 0

	if hadCurrent {
		l.Select(current.Name)
	}
}

// Presets returns a copy of the preset list.
func (l *Library) Presets() []Preset {
	return append([]Preset(nil), l.presets...)
}
