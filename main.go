package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
)

func main() {
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
