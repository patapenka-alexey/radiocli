// Package control implements the remote-control subcommands.
package control

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"radiocli/internal/paths"
)

// Run handles the remote-control subcommands. It talks to the mpv instance
// started by the running TUI, so the TUI reflects the change via its property
// observers. Success is silent.
func Run(args []string) error {
	switch args[0] {
	case "pause":
		return sendCommand([]any{"set_property", "pause", true})
	case "unpause":
		return sendCommand([]any{"set_property", "pause", false})
	default:
		return fmt.Errorf("unknown command %q (available: pause, unpause)", args[0])
	}
}

func sendCommand(command []any) error {
	conn, err := net.DialTimeout("unix", paths.ControlSocketPath(), 2*time.Second)
	if err != nil {
		return fmt.Errorf("radiocli is not running")
	}
	defer conn.Close()

	payload := map[string]any{"command": command, "request_id": 1}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write(data); err != nil {
		return err
	}

	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return err
		}

		var msg map[string]any
		if json.Unmarshal(line, &msg) != nil {
			continue
		}

		id, ok := msg["request_id"].(float64)
		if !ok || int(id) != 1 {
			continue
		}

		if msg["error"] == "success" {
			return nil
		}

		return fmt.Errorf("mpv: %v", msg["error"])
	}
}
