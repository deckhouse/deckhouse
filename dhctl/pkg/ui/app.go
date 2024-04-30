package ui

import (
	"github.com/go-openapi/spec"
	"github.com/rivo/tview"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/state"
)

const (
	FormMain              = "FormMain"
	FormSelectClusterType = "FormSelectClusterType"
)

type Schemas interface {
	ClusterConfig() *spec.Schema
}

type App struct {
	app         *tview.Application
	state       *state.State
	pages       *tview.Pages
	schemaStore *state.Schema
}

func NewApp() *App {
	schemaStore := state.NewSchema(config.NewSchemaStore())

	res := &App{
		app:         tview.NewApplication(),
		pages:       tview.NewPages(),
		state:       state.NewState(schemaStore),
		schemaStore: schemaStore,
	}

	buildPages(res)

	return res
}

func buildPages(a *App) {
	selectCls, selectFocusables := selectClusterForm(a.state, a.schemaStore, func() {
		a.app.Stop()
	})

	mainForm, mainFocusables := welcomePage(func() {
		addSwitchFocusEvent(a.app, a.pages, selectFocusables)
		a.pages.SwitchToPage(FormSelectClusterType)
	})

	a.pages.AddPage(FormMain, mainForm, true, false).
		AddPage(FormSelectClusterType, selectCls, true, false)

	addSwitchFocusEvent(a.app, a.pages, mainFocusables)
	a.pages.SwitchToPage(FormMain)
}

func (a *App) Start() error {
	if err := a.app.SetRoot(a.pages, true).EnableMouse(true).EnablePaste(true).Run(); err != nil {
		return err
	}
	return nil
}

func (a *App) State() *state.State {
	return a.state
}
