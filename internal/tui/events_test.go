package tui

import (
	"testing"
	"time"

	"radiocli/internal/ports"
)

func TestWaitForPlayerEvent(t *testing.T) {
	events := make(chan ports.Event, 1)
	events <- ports.Event{Property: "time-pos"}

	msg := waitForPlayerEvent(events)()

	event, ok := msg.(playerEventMsg)
	if !ok {
		t.Fatalf("message type = %T, want playerEventMsg", msg)
	}
	if event.event.Property != "time-pos" {
		t.Fatalf("property = %q, want time-pos", event.event.Property)
	}
}

func TestWaitForPlayerEventClosed(t *testing.T) {
	events := make(chan ports.Event)
	close(events)

	msg := waitForPlayerEvent(events)()

	if _, ok := msg.(playerExitedMsg); !ok {
		t.Fatalf("message type = %T, want playerExitedMsg", msg)
	}
}

func TestPlayerExitedFlashes(t *testing.T) {
	m := newTestModel(0)

	m.update(playerExitedMsg{})

	if !m.mpvExitNotified {
		t.Fatal("mpvExitNotified = false, want true")
	}
	if m.message != "mpv exited unexpectedly" {
		t.Fatalf("message = %q", m.message)
	}
}

func TestPlayerExitedFlashesOnlyOnce(t *testing.T) {
	m := newTestModel(0)
	m.update(playerExitedMsg{})
	first := m.flashID

	m.update(playerExitedMsg{})

	if m.flashID != first {
		t.Fatalf("flashID changed from %d to %d", first, m.flashID)
	}
}

func TestFlashExpiry(t *testing.T) {
	m := newTestModel(0)
	m.flash("hello", time.Minute)

	if m.flashTimer() == nil {
		t.Fatal("flashTimer returned nil")
	}

	m.update(flashExpiredMsg{id: m.flashID - 1})
	if m.message != "hello" {
		t.Fatalf("stale timer cleared the message")
	}

	m.update(flashExpiredMsg{id: m.flashID})
	if m.message != "" {
		t.Fatalf("message = %q, want empty", m.message)
	}
}

func TestFlashTimerIsScheduledOnce(t *testing.T) {
	m := newTestModel(0)
	m.flash("hello", time.Minute)

	if m.flashTimer() == nil {
		t.Fatal("first flashTimer returned nil")
	}
	if m.flashTimer() != nil {
		t.Fatal("second flashTimer should return nil")
	}
}
