package ui

import (
	"github.com/go-openapi/spec"
	"github.com/rivo/tview"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/state"
)

const (
	pageMain              = "pageMain"
	pageSelectClusterType = "pageSelectClusterType"
	pageProvider          = "pageProvider"
	pageStaticMaster      = "pageStaticMaster"
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
	var selectFocusables []tview.Primitive
	var selectStaticMaster []tview.Primitive

	providerCls := newProviderPage(a.state, a.schemaStore, func() {
		a.app.Stop()
	}, func() {
		addSwitchFocusEvent(a.app, a.pages, selectFocusables)
		a.pages.SwitchToPage(pageSelectClusterType)
	})

	selectCls, selectFocusables := clusterPage(a.state, a.schemaStore, func() {
		if a.state.ClusterType == state.CloudCluster {
			p, focusable := providerCls.Show()
			a.pages.AddPage(pageProvider, p, true, false)
			addSwitchFocusEvent(a.app, a.pages, focusable)
			a.pages.SwitchToPage(pageProvider)
			return
		}

		var p tview.Primitive
		p, selectStaticMaster = staticMasterPage(a.state, func() {
			a.app.Stop()
		}, func() {
			addSwitchFocusEvent(a.app, a.pages, selectFocusables)
			a.pages.SwitchToPage(pageSelectClusterType)
		})

		addSwitchFocusEvent(a.app, a.pages, selectStaticMaster)
		a.pages.AddPage(pageStaticMaster, p, true, false)
		a.pages.SwitchToPage(pageStaticMaster)

	})

	mainForm, mainFocusables := welcomePage(func() {
		addSwitchFocusEvent(a.app, a.pages, selectFocusables)
		a.pages.SwitchToPage(pageSelectClusterType)
	})

	a.pages.AddPage(pageMain, mainForm, true, false).
		AddPage(pageSelectClusterType, selectCls, true, false)

	addSwitchFocusEvent(a.app, a.pages, mainFocusables)
	a.pages.SwitchToPage(pageMain)
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
