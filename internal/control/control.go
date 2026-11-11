// Package control implements the remote-control subcommands.
package control

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"

	"radiocli/internal/paths"
)

// Run handles the remote-control subcommands. It forwards them to the running
// TUI over its control socket; the TUI then drives mpv. Success is silent.
func Run(args []string) error {
	switch args[0] {
	case "pause":
		return sendAction(ActionPause)
	case "unpause":
		return sendAction(ActionUnpause)
	case "next":
		return sendAction(ActionNext)
	case "previous":
		return sendAction(ActionPrevious)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

// sendAction forwards an action to the TUI control socket. Success is silent.
func sendAction(action Action) error {
	conn, err := net.DialTimeout("unix", paths.TUIControlSocketPath(), 2*time.Second)
	if err != nil {
		return fmt.Errorf("radiocli is not running")
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

	if _, err := fmt.Fprintf(conn, "%s\n", action); err != nil {
		return err
	}

	reply, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return err
	}

	reply = strings.TrimSpace(reply)
	if reply == "ok" {
		return nil
	}

	return fmt.Errorf("%s", strings.TrimPrefix(reply, "error: "))
}
