// Package bootstrap is the composition root: it wires the concrete
// infrastructure adapters into the application service and the TUI.
package bootstrap

import (
	"radiocli/internal/app"
	"radiocli/internal/catalog"
	"radiocli/internal/config"
	"radiocli/internal/player"
	"radiocli/internal/tui"
)

// Deps holds the constructed application graph.
type Deps struct {
	Config  *config.Store
	Catalog *catalog.Repository
	Player  *player.MPV
	Service *app.Service
	Model   *tui.Model
}

// Build constructs the application dependencies.
func Build() (*Deps, error) {
	config := config.Load()
	catalog := catalog.LoadWithOptions(catalog.Options{Timeout: config.UpdateTimeout()})

	player, err := player.New(config.MpvPath())
	if err != nil {
		return nil, err
	}

	service := app.New(config, catalog, player)

	return &Deps{
		Config:  config,
		Catalog: catalog,
		Player:  player,
		Service: service,
		Model:   tui.NewModel(service),
	}, nil
}

// Close releases the resources owned by the dependency graph.
func (d *Deps) Close() {
	if d.Player != nil {
		d.Player.Quit()
	}
}
