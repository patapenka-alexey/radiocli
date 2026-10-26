package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"
)

// player drives a long-lived `mpv` process over its JSON IPC socket. mpv is
// started once with --idle=yes and all playback is controlled through the IPC
// protocol; no mpv-mpris plugin or DBus is required.
type player struct {
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

type playerSnapshot struct {
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

func newPlayer(mpvPath string) (*player, error) {
	p := &player{
		mpvPath:    mpvPath,
		socketPath: controlSocketPath(),
		idle:       true,
		waitDone:   make(chan struct{}),
	}

	if err := p.start(); err != nil {
		return nil, err
	}

	return p, nil
}

func (p *player) start() error {
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

func (p *player) removeStaleSocket() {
	if _, err := os.Stat(p.socketPath); err == nil {
		_ = os.Remove(p.socketPath)
	}
}

func (p *player) connectWithRetry() (net.Conn, error) {
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

func (p *player) readLoop() {
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

func (p *player) handleLine(line []byte) {
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

func (p *player) applyProperty(name string, value any) {
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

func (p *player) send(command []any) {
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

func (p *player) play(url string) {
	p.send([]any{"loadfile", url, "replace"})

	p.mu.Lock()
	p.idle = false
	p.paused = false
	p.mu.Unlock()
}

func (p *player) togglePause() {
	p.send([]any{"cycle", "pause"})
}

func (p *player) stop() {
	p.send([]any{"stop"})
}

func (p *player) snapshot() playerSnapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	return playerSnapshot{
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

func (p *player) isAlive() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return !p.exited && p.cmd != nil
}

func (p *player) quit() {
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
