package ui

import (
	"github.com/go-openapi/spec"
	"github.com/rivo/tview"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/state"
)

type Schemas interface {
	ClusterConfig() *spec.Schema
}

type App struct {
	app    *tview.Application
	wizard *Wizard

	state       *state.State
	schemaStore *state.Schema
}

func NewApp() *App {
	schemaStore := state.NewSchema(config.NewSchemaStore())
	st := state.NewState(schemaStore)
	app := tview.NewApplication()

	return &App{
		app:         app,
		state:       st,
		schemaStore: schemaStore,
		wizard:      NewWizard(app, st, schemaStore),
	}
}

func (a *App) Start() (*state.State, error) {
	if err := a.wizard.Start(); err != nil {
		return nil, err
	}
	return a.state, nil
}
