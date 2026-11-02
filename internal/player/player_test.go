package player

import (
	"encoding/json"
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

func TestSetAudioFilterSendsProperty(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()

	p := newTestPlayer()
	p.conn = client

	done := make(chan map[string]any, 1)
	go func() {
		var msg map[string]any
		if json.NewDecoder(server).Decode(&msg) == nil {
			done <- msg
		}
	}()

	p.SetAudioFilter("lavfi=[equalizer=f=100:width_type=o:width=1.5:g=-4]")

	select {
	case msg := <-done:
		command, ok := msg["command"].([]any)
		if !ok || len(command) != 3 {
			t.Fatalf("command = %#v", msg["command"])
		}
		if command[0] != "set_property" || command[1] != "af" {
			t.Fatalf("command = %#v", command)
		}
	case <-time.After(time.Second):
		t.Fatal("no command sent")
	}
}
