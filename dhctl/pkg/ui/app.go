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
	pageRegistry          = "pageRegistry"
	pageCluster           = "pageCluster"
	pageCNI               = "pageCNI"
	pageDeckhouse         = "pageDeckhouse"
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
	var registryFocusable []tview.Primitive
	var staticMasterFocusable []tview.Primitive
	var clusterPageFocusable []tview.Primitive
	var deckhouseFocusable []tview.Primitive

	var providerCls *providerPage
	var staticMasterPage tview.Primitive
	var registryPage tview.Primitive
	var clusterPage tview.Primitive
	var deckhousePage tview.Primitive
	var cniP *cniPage

	cniP = newCNIPage(a.state, a.schemaStore, func() {
		addSwitchFocusEvent(a.app, a.pages, deckhouseFocusable)
		a.pages.SwitchToPage(pageDeckhouse)
	}, func() {
		addSwitchFocusEvent(a.app, a.pages, clusterPageFocusable)
		a.pages.SwitchToPage(pageCluster)
	})

	clusterPage, clusterPageFocusable = newClusterPage(a.state, a.schemaStore, func() {
		p, cniPageFocusable := cniP.Show()
		addSwitchFocusEvent(a.app, a.pages, cniPageFocusable)
		a.pages.AddPage(pageCNI, p, true, false)
		a.pages.SwitchToPage(pageCNI)
	}, func() {
		addSwitchFocusEvent(a.app, a.pages, registryFocusable)
		a.pages.SwitchToPage(pageRegistry)
	})

	providerCls = newProviderPage(a.state, a.schemaStore, func() {
		addSwitchFocusEvent(a.app, a.pages, registryFocusable)
		a.pages.SwitchToPage(pageRegistry)
	}, func() {
		addSwitchFocusEvent(a.app, a.pages, selectFocusables)
		a.pages.SwitchToPage(pageSelectClusterType)
	})

	staticMasterPage, staticMasterFocusable = newStaticMasterPage(a.state, func() {
		addSwitchFocusEvent(a.app, a.pages, registryFocusable)
		a.pages.SwitchToPage(pageRegistry)
	}, func() {
		addSwitchFocusEvent(a.app, a.pages, selectFocusables)
		a.pages.SwitchToPage(pageSelectClusterType)
	})

	deckhousePage, deckhouseFocusable = newDeckhousePage(a.state, a.schemaStore, func() {
		a.app.Stop()
	}, func() {
		p, cniPageFocusable := cniP.Show()
		addSwitchFocusEvent(a.app, a.pages, cniPageFocusable)
		a.pages.AddPage(pageCNI, p, true, false)
		a.pages.SwitchToPage(pageCNI)
	})

	registryPage, registryFocusable = newRegistryPage(a.state, a.schemaStore, func() {
		addSwitchFocusEvent(a.app, a.pages, clusterPageFocusable)
		a.pages.SwitchToPage(pageCluster)
	}, func() {
		if a.state.ClusterType == state.CloudCluster {
			p, focusable := providerCls.Show()
			a.pages.AddPage(pageProvider, p, true, false)
			addSwitchFocusEvent(a.app, a.pages, focusable)
			a.pages.SwitchToPage(pageProvider)
			return
		}

		addSwitchFocusEvent(a.app, a.pages, staticMasterFocusable)
		a.pages.SwitchToPage(pageStaticMaster)
	})

	selectCls, selectFocusables := newClusterTypePage(a.state, a.schemaStore, func() {
		if a.state.ClusterType == state.CloudCluster {
			p, focusable := providerCls.Show()
			a.pages.AddPage(pageProvider, p, true, false)
			addSwitchFocusEvent(a.app, a.pages, focusable)
			a.pages.SwitchToPage(pageProvider)
			return
		}

		addSwitchFocusEvent(a.app, a.pages, staticMasterFocusable)
		a.pages.SwitchToPage(pageStaticMaster)

	})

	mainForm, mainFocusables := welcomePage(func() {
		addSwitchFocusEvent(a.app, a.pages, selectFocusables)
		a.pages.SwitchToPage(pageSelectClusterType)
	})

	a.pages.AddPage(pageMain, mainForm, true, false).
		AddPage(pageSelectClusterType, selectCls, true, false).
		AddPage(pageRegistry, registryPage, true, false).
		AddPage(pageCluster, clusterPage, true, false).
		AddPage(pageDeckhouse, deckhousePage, true, false).
		AddPage(pageStaticMaster, staticMasterPage, true, false)

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
