package control

import (
	"bufio"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseAction(t *testing.T) {
	tests := []struct {
		input string
		want  Action
		ok    bool
	}{
		{"pause\n", ActionPause, true},
		{"unpause\n", ActionUnpause, true},
		{"next\n", ActionNext, true},
		{"previous\n", ActionPrevious, true},
		{" next \n", ActionNext, true},
		{"bogus\n", "", false},
		{"\n", "", false},
	}

	for _, tt := range tests {
		action, err := parseAction(tt.input)
		if tt.ok && err != nil {
			t.Fatalf("parseAction(%q) error = %v", tt.input, err)
		}
		if !tt.ok && err == nil {
			t.Fatalf("parseAction(%q) expected an error", tt.input)
		}
		if tt.ok && action != tt.want {
			t.Fatalf("parseAction(%q) = %q, want %q", tt.input, action, tt.want)
		}
	}
}

func TestListenHandlesAction(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "control.sock")

	actions := make(chan Action, 1)
	server, err := Listen(socket, func(action Action) { actions <- action })
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer server.Close()

	conn, err := net.DialTimeout("unix", socket, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("next\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	reply, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("read reply: %v", err)
	}
	if strings.TrimSpace(reply) != "ok" {
		t.Fatalf("reply = %q, want ok", reply)
	}

	select {
	case action := <-actions:
		if action != ActionNext {
			t.Fatalf("action = %q, want %q", action, ActionNext)
		}
	case <-time.After(time.Second):
		t.Fatal("handler was not called")
	}
}

func TestListenRejectsUnknownAction(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "control.sock")

	called := false
	server, err := Listen(socket, func(Action) { called = true })
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer server.Close()

	conn, err := net.DialTimeout("unix", socket, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("bogus\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	reply, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("read reply: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(reply), "error:") {
		t.Fatalf("reply = %q, want an error", reply)
	}
	if called {
		t.Fatal("handler must not be called for an unknown action")
	}
}
