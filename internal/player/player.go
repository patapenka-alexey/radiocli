// Package player drives a long-lived mpv process over its JSON IPC socket.
package player

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"radiocli/internal/paths"
)

// MPV drives a long-lived `mpv` process over its JSON IPC socket. mpv is
// started once with --idle=yes and all playback is controlled through the IPC
// protocol; no mpv-mpris plugin or DBus is required.
type MPV struct {
	mpvPath    string
	socketPath string

	cmd      *exec.Cmd
	conn     net.Conn
	waitDone chan struct{}

	sendMu    sync.Mutex
	requestID int

	mu          sync.Mutex
	title       string
	paused      bool
	idle        bool
	timePos     float64
	hasTimePos  bool
	duration    float64
	hasDuration bool
	exited      bool
}

// Snapshot is a point-in-time view of the player state.
type Snapshot struct {
	Title       string
	Paused      bool
	Idle        bool
	TimePos     float64
	HasTimePos  bool
	Duration    float64
	HasDuration bool
	Alive       bool
}

type observedProperty struct {
	id   int
	name string
}

var observedProperties = []observedProperty{
	{1, "media-title"},
	{2, "pause"},
	{3, "time-pos"},
	{4, "duration"},
	{5, "idle-active"},
}

// New starts an mpv process and connects to its IPC socket.
func New(mpvPath string) (*MPV, error) {
	p := &MPV{
		mpvPath:    mpvPath,
		socketPath: paths.ControlSocketPath(),
		idle:       true,
		waitDone:   make(chan struct{}),
	}

	if err := p.start(); err != nil {
		return nil, err
	}

	return p, nil
}

func (p *MPV) start() error {
	p.removeStaleSocket()

	args := []string{
		"--config=no",
		"--no-video",
		"--idle=yes",
		"--no-terminal",
		"--force-window=no",
		fmt.Sprintf("--input-ipc-server=%s", p.socketPath),
	}

	cmd := exec.Command(p.mpvPath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("mpv executable not found at '%s'", p.mpvPath)
	}
	p.cmd = cmd

	go func() {
		_ = cmd.Wait()
		p.mu.Lock()
		p.exited = true
		p.mu.Unlock()
		close(p.waitDone)
	}()

	conn, err := p.connectWithRetry()
	if err != nil {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return err
	}
	p.conn = conn

	go p.readLoop()

	for _, obs := range observedProperties {
		p.send([]any{"observe_property", obs.id, obs.name})
	}

	return nil
}

func (p *MPV) removeStaleSocket() {
	if _, err := os.Stat(p.socketPath); err == nil {
		_ = os.Remove(p.socketPath)
	}
}

func (p *MPV) connectWithRetry() (net.Conn, error) {
	deadline := time.Now().Add(5 * time.Second)
	var lastErr error

	for time.Now().Before(deadline) {
		p.mu.Lock()
		exited := p.exited
		p.mu.Unlock()
		if exited {
			return nil, fmt.Errorf("mpv exited immediately after start")
		}

		conn, err := net.Dial("unix", p.socketPath)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("timed out")
	}

	return nil, fmt.Errorf("could not connect to mpv IPC socket: %w", lastErr)
}

func (p *MPV) readLoop() {
	reader := bufio.NewReader(p.conn)
	for {
		line, err := reader.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			p.handleLine(bytes.TrimSpace(line))
		}
		if err != nil {
			break
		}
	}

	p.mu.Lock()
	p.exited = true
	p.mu.Unlock()
}

func (p *MPV) handleLine(line []byte) {
	var msg map[string]any
	if err := json.Unmarshal(line, &msg); err != nil {
		return
	}

	if msg["event"] != "property-change" {
		return
	}

	name, _ := msg["name"].(string)
	p.applyProperty(name, msg["data"])
}

func (p *MPV) applyProperty(name string, value any) {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch name {
	case "media-title":
		if s, ok := value.(string); ok {
			p.title = s
		} else {
			p.title = ""
		}
	case "pause":
		p.paused = toBool(value)
	case "idle-active":
		p.idle = toBool(value)
		if p.idle {
			p.title = ""
			p.hasTimePos = false
		}
	case "time-pos":
		if f, ok := toFloat(value); ok {
			p.timePos = f
			p.hasTimePos = true
		}
	case "duration":
		if f, ok := toFloat(value); ok {
			p.duration = f
			p.hasDuration = true
		}
	}
}

func toBool(v any) bool {
	b, _ := v.(bool)
	return b
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func (p *MPV) send(command []any) {
	if p.conn == nil {
		return
	}
	p.sendMu.Lock()
	defer p.sendMu.Unlock()

	p.requestID++
	payload := map[string]any{"command": command, "request_id": p.requestID}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	data = append(data, '\n')
	_, _ = p.conn.Write(data)
}

// Play loads the given stream URL into mpv.
func (p *MPV) Play(url string) {
	p.send([]any{"loadfile", url, "replace"})

	p.mu.Lock()
	p.idle = false
	p.paused = false
	p.mu.Unlock()
}

// TogglePause cycles the paused state.
func (p *MPV) TogglePause() {
	p.send([]any{"cycle", "pause"})
}

// Stop stops playback.
func (p *MPV) Stop() {
	p.send([]any{"stop"})
}

// Snapshot returns a consistent copy of the observed player state.
func (p *MPV) Snapshot() Snapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	return Snapshot{
		Title:       p.title,
		Paused:      p.paused,
		Idle:        p.idle,
		TimePos:     p.timePos,
		HasTimePos:  p.hasTimePos,
		Duration:    p.duration,
		HasDuration: p.hasDuration,
		Alive:       !p.exited && p.cmd != nil,
	}
}

// Alive reports whether the mpv process is still running.
func (p *MPV) Alive() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return !p.exited && p.cmd != nil
}

// Quit asks mpv to exit and waits for the process to finish.
func (p *MPV) Quit() {
	if p.conn != nil {
		p.send([]any{"quit"})
		_ = p.conn.Close()
	}
	if p.waitDone != nil {
		select {
		case <-p.waitDone:
		case <-time.After(3 * time.Second):
			if p.cmd != nil && p.cmd.Process != nil {
				_ = p.cmd.Process.Kill()
			}
			<-p.waitDone
		}
	}

	p.removeStaleSocket()
}
