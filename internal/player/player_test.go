package player

import (
	"errors"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"radiocli/internal/ports"
)

func newTestPlayer() *MPV {
	return &MPV{events: make(chan ports.Event, 16)}
}

type recordingConn struct {
	net.Conn
	writes []string
}

func (c *recordingConn) Write(b []byte) (int, error) {
	c.writes = append(c.writes, string(b))

	return len(b), nil
}

func TestInstanceLockRejectsSecondInstance(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "radiocli.sock")
	first := &MPV{socketPath: socketPath}
	second := &MPV{socketPath: socketPath}

	if err := first.acquireLock(); err != nil {
		t.Fatalf("acquire first lock: %v", err)
	}
	defer first.releaseLock()

	if err := second.acquireLock(); err == nil {
		second.releaseLock()
		t.Fatal("second instance acquired an already-held lock")
	}

	first.releaseLock()

	if err := second.acquireLock(); err != nil {
		t.Fatalf("acquire lock after release: %v", err)
	}
	second.releaseLock()
}

func TestCommandsFailWithoutConnection(t *testing.T) {
	p := newTestPlayer()

	if err := p.Play("https://example.com/stream"); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("Play error = %v, want ErrNotRunning", err)
	}
	if err := p.TogglePause(); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("TogglePause error = %v, want ErrNotRunning", err)
	}
	if err := p.Stop(); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("Stop error = %v, want ErrNotRunning", err)
	}
}

func TestPlayLoadsAndUnpauses(t *testing.T) {
	conn := &recordingConn{}
	p := newTestPlayer()
	p.conn = conn

	if err := p.Play("https://example.com/stream"); err != nil {
		t.Fatalf("Play: %v", err)
	}

	if len(conn.writes) != 2 {
		t.Fatalf("writes = %d, want 2: %v", len(conn.writes), conn.writes)
	}
	if !strings.Contains(conn.writes[0], `"loadfile"`) || !strings.Contains(conn.writes[0], "https://example.com/stream") {
		t.Fatalf("first write = %q, want loadfile", conn.writes[0])
	}
	if !strings.Contains(conn.writes[1], `"set_property"`) || !strings.Contains(conn.writes[1], `"pause"`) || !strings.Contains(conn.writes[1], "false") {
		t.Fatalf("second write = %q, want pause=false", conn.writes[1])
	}

	if status := p.Status(); status.Paused || status.Idle {
		t.Fatalf("status = %+v, want playing", status)
	}
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
