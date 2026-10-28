package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"radiocli/internal/app"
	"radiocli/internal/catalog"
	"radiocli/internal/config"
	"radiocli/internal/control"
	"radiocli/internal/player"
	"radiocli/internal/tui"
)

func main() {
	if args := os.Args[1:]; len(args) > 0 {
		if err := control.Run(args); err != nil {
			fmt.Fprintf(os.Stderr, "radiocli: %v\n", err)

			os.Exit(1)
		}

		return
	}

	cfg := config.Load()
	db := catalog.Load()

	pl, err := player.New(cfg.MpvPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "radiocli: %v\n", err)

		os.Exit(1)
	}

	svc := app.New(cfg, db, pl)
	prog := tea.NewProgram(tui.NewModel(svc))
	if _, err := prog.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "radiocli: %v\n", err)
	}

	pl.Quit()
}
