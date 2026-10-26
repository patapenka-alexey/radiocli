package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"
)

// controlSocketPath is the mpv JSON IPC socket shared by the running instance
// and the remote-control commands.
func controlSocketPath() string {
	return fmt.Sprintf("/tmp/radiocli-%d.sock", os.Getuid())
}

// runControl handles the remote-control subcommands. It talks to the mpv
// instance started by the running TUI, so the TUI reflects the change via its
// property observers. Success is silent.
func runControl(args []string) error {
	switch args[0] {
	case "pause":
		return sendControlCommand([]any{"set_property", "pause", true})
	case "unpause":
		return sendControlCommand([]any{"set_property", "pause", false})
	default:
		return fmt.Errorf("unknown command %q (available: pause, unpause)", args[0])
	}
}

func sendControlCommand(command []any) error {
	conn, err := net.DialTimeout("unix", controlSocketPath(), 2*time.Second)
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
