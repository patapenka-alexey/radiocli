package main

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"

	tea "charm.land/bubbletea/v2"

	"radiocli/internal/bootstrap"
	"radiocli/internal/control"
)

const usage = `Usage:
  radiocli                 start the TUI
  radiocli <command>       remote control the running instance
  radiocli --help          show this help
  radiocli --version       show the version

Commands:
  pause                    pause the running instance
  unpause                  resume the running instance
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		return runCommand(args, stdout, stderr)
	}

	return runTUI(stderr)
}

func runCommand(args []string, stdout, stderr io.Writer) int {
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)

		return 0
	case "version", "-v", "--version":
		fmt.Fprintf(stdout, "radiocli %s\n", buildVersion())

		return 0
	}

	if err := control.Run(args); err != nil {
		fmt.Fprintf(stderr, "radiocli: %v\n", err)

		return 1
	}

	return 0
}

func runTUI(stderr io.Writer) int {
	deps, err := bootstrap.Build()
	if err != nil {
		fmt.Fprintf(stderr, "radiocli: %v\n", err)

		return 1
	}
	defer deps.Close()

	prog := tea.NewProgram(deps.Model)
	if _, err := prog.Run(); err != nil {
		fmt.Fprintf(stderr, "radiocli: %v\n", err)

		return 1
	}

	return 0
}

func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "dev"
	}

	return info.Main.Version
}
