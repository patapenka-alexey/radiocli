package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"radiocli/internal/bootstrap"
	"radiocli/internal/control"
)

func main() {
	if args := os.Args[1:]; len(args) > 0 {
		if err := control.Run(args); err != nil {
			fmt.Fprintf(os.Stderr, "radiocli: %v\n", err)

			os.Exit(1)
		}

		return
	}

	deps, err := bootstrap.Build()
	if err != nil {
		fmt.Fprintf(os.Stderr, "radiocli: %v\n", err)

		os.Exit(1)
	}
	defer deps.Close()

	prog := tea.NewProgram(deps.Model)
	if _, err := prog.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "radiocli: %v\n", err)
	}
}
