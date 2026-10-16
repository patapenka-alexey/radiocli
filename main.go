package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
)

func main() {
	if args := os.Args[1:]; len(args) > 0 {
		if err := runControl(args); err != nil {
			fmt.Fprintf(os.Stderr, "radio-cli: %v\n", err)
			os.Exit(1)
		}
		return
	}

	cfg := loadConfig()
	db := loadStationDB()

	pl, err := newPlayer(cfg.MpvPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "radio-cli: %v\n", err)
		os.Exit(1)
	}

	prog := tea.NewProgram(newModel(cfg, db, pl))
	if _, err := prog.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "radio-cli: %v\n", err)
	}

	pl.quit()
}
