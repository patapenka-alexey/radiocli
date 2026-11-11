package control

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// Action is a remote-control command forwarded to the running TUI.
type Action string

const (
	// ActionPause pauses playback.
	ActionPause Action = "pause"
	// ActionUnpause resumes playback.
	ActionUnpause Action = "unpause"
	// ActionNext plays the next station.
	ActionNext Action = "next"
	// ActionPrevious plays the previous station.
	ActionPrevious Action = "previous"
)

func parseAction(value string) (Action, error) {
	switch action := Action(strings.TrimSpace(value)); action {
	case ActionPause, ActionUnpause, ActionNext, ActionPrevious:
		return action, nil
	default:
		return "", fmt.Errorf("unknown action %q", value)
	}
}

// Server accepts commands from the remote-control subcommands and hands them to
// the running TUI. It is the counterpart of sendAction.
type Server struct {
	listener net.Listener
}

// Listen binds socketPath and serves actions in the background. Each connection
// carries one action and is answered with "ok" or an "error: ..." line.
func Listen(socketPath string, handle func(Action)) (*Server, error) {
	_ = os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("listen on control socket: %w", err)
	}

	server := &Server{listener: listener}
	go server.serve(handle)

	return server, nil
}

func (s *Server) serve(handle func(Action)) {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}

		go s.handleConn(conn, handle)
	}
}

func (s *Server) handleConn(conn net.Conn, handle func(Action)) {
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return
	}

	action, err := parseAction(line)
	if err != nil {
		_, _ = fmt.Fprintf(conn, "error: %v\n", err)

		return
	}

	handle(action)

	_, _ = fmt.Fprintln(conn, "ok")
}

// Close stops accepting connections and removes the socket.
func (s *Server) Close() error {
	err := s.listener.Close()
	_ = os.Remove(s.listener.Addr().String())

	return err
}
