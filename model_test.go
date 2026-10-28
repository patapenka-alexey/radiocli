package main

import "testing"

func TestClampOffset(t *testing.T) {
	tests := []struct {
		name    string
		index   int
		offset  int
		visible int
		total   int
		want    int
	}{
		{"fits without scrolling", 2, 0, 5, 10, 0},
		{"scrolls down", 7, 0, 5, 10, 3},
		{"keeps existing offset", 7, 5, 5, 10, 5},
		{"scrolls up", 2, 5, 5, 10, 2},
		{"clamps to last page", 9, 8, 5, 10, 5},
		{"zero visible rows", 3, 2, 0, 10, 0},
		{"empty list", 0, 0, 5, 0, 0},
		{"negative offset", 0, -4, 5, 10, 0},
		{"index past total", 20, 0, 5, 10, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampOffset(tt.index, tt.offset, tt.visible, tt.total); got != tt.want {
				t.Fatalf("clampOffset(%d, %d, %d, %d) = %d, want %d",
					tt.index, tt.offset, tt.visible, tt.total, got, tt.want)
			}
		})
	}
}

func TestEnsureVisibleKeepsOffsetsInRange(t *testing.T) {
	m := &model{
		cfg:          &config{Stations: make([]Station, 20)},
		width:        80,
		height:       20,
		stationIndex: 19,
	}

	m.ensureVisible()

	if m.stationOffset > len(m.cfg.Stations)-1 {
		t.Fatalf("station offset %d out of range", m.stationOffset)
	}
	if got := m.stationOffset; got != 19-m.layout().stationVisible+1 {
		t.Fatalf("station offset = %d, want %d", got, 19-m.layout().stationVisible+1)
	}
}

func TestEnsureVisibleIsNoopOnSmallTerminal(t *testing.T) {
	m := &model{
		cfg:           &config{Stations: make([]Station, 20)},
		width:         10,
		height:        5,
		stationIndex:  19,
		stationOffset: 7,
	}

	m.ensureVisible()

	if m.stationOffset != 7 {
		t.Fatalf("station offset = %d, want 7", m.stationOffset)
	}
}
