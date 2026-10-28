package player

import (
	"net"
	"testing"
	"time"

	"radiocli/internal/ports"
)

func newTestPlayer() *MPV {
	return &MPV{events: make(chan ports.Event, 16)}
}

func TestHandleLinePublishesPropertyEvent(t *testing.T) {
	p := newTestPlayer()

	p.handleLine([]byte(`{"event":"property-change","name":"pause","data":true}`))

	select {
	case event := <-p.Events():
		if event.Property != "pause" {
			t.Fatalf("property = %q, want pause", event.Property)
		}
	case <-time.After(time.Second):
		t.Fatal("no event published")
	}

	if status := p.Status(); !status.Paused {
		t.Fatal("player status was not updated")
	}
}

func TestHandleLineIgnoresOtherEvents(t *testing.T) {
	p := newTestPlayer()

	p.handleLine([]byte(`{"event":"start-file"}`))

	select {
	case event := <-p.Events():
		t.Fatalf("unexpected event: %+v", event)
	default:
	}
}

func TestReadLoopClosesEventsOnExit(t *testing.T) {
	client, server := net.Pipe()

	p := newTestPlayer()
	p.conn = client

	done := make(chan struct{})
	go func() {
		p.readLoop()
		close(done)
	}()

	if _, err := server.Write([]byte(`{"event":"property-change","name":"pause","data":true}` + "\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	select {
	case <-p.Events():
	case <-time.After(time.Second):
		t.Fatal("no event published")
	}

	_ = server.Close()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("read loop did not stop")
	}

	if _, ok := <-p.Events(); ok {
		t.Fatal("events channel is still open")
	}
}
