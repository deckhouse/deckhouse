package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/state"
)

const (
	pageWelcome           = "pageWelcome"
	pageSelectClusterType = "pageSelectClusterType"
	pageProvider          = "pageProvider"
	pageStaticMaster      = "pageStaticMaster"
	pageRegistry          = "pageRegistry"
	pageCluster           = "pageCluster"
	pageCNI               = "pageCNI"
	pageDeckhouse         = "pageDeckhouse"
	pageSSH               = "pageSSH"
	pageConfig            = "pageConfig"
)

var (
	preClusterTypeSteps = []string{
		pageWelcome,
		pageSelectClusterType,
	}

	postClusterTypeSteps = []string{
		pageRegistry,
		pageCluster,
		pageCNI,
		pageDeckhouse,
		pageSSH,
		pageConfig,
	}

	staticPages = []string{
		pageStaticMaster,
	}

	staticPagesOrder = append(append(append([]string{}, preClusterTypeSteps...), staticPages...), postClusterTypeSteps...)

	orders = map[string][]string{
		// static
		"": staticPagesOrder,
	}
)

type Page interface {
	Show(onNext, onBack func()) (tview.Primitive, []tview.Primitive)
}

type Wizard struct {
	order            []string
	currentPageIndex int
	pagesView        *tview.Pages
	allPages         map[string]Page
	app              *tview.Application

	state       *state.State
	schemaStore *state.Schema
}

func NewWizard(app *tview.Application, st *state.State, schema *state.Schema) *Wizard {
	allPages := map[string]Page{
		pageWelcome:           NewWelcomePage(),
		pageSelectClusterType: NewClusterTypePage(st, schema),
		pageProvider:          NewProviderPage(st, schema),
		pageStaticMaster:      NewStaticMasterPage(st),
		pageRegistry:          NewRegistryPage(st, schema),
		pageCluster:           NewClusterPage(st, schema),
		pageCNI:               NewCNIPage(st, schema),
		pageDeckhouse:         NewDeckhousePage(st, schema),
		pageSSH:               NewSSHPage(st),
		pageConfig:            NewConfigPage(st),
	}

	// by default
	return &Wizard{
		order:            staticPagesOrder,
		currentPageIndex: 0,
		state:            st,
		schemaStore:      schema,
		allPages:         allPages,
		pagesView:        tview.NewPages(),
		app:              app,
	}
}

func (w *Wizard) Start() error {
	var onNext, onBack func()

	switchPage := func() {
		pageName := w.order[w.currentPageIndex]
		page := w.allPages[pageName]

		back := onBack
		if w.currentPageIndex == 0 {
			back = nil
		}

		view, focusables := page.Show(onNext, back)
		w.pagesView.AddPage(pageName, view, true, false)
		w.addSwitchFocusEvent(focusables)
		w.pagesView.SwitchToPage(pageName)
	}

	onNext = func() {
		pp := w.order[w.currentPageIndex]
		if pp == pageSelectClusterType {
			w.order = orders[w.state.GetProvider()]
		}

		w.currentPageIndex++
		if w.currentPageIndex >= len(w.order) {
			w.app.Stop()
			return
		}

		switchPage()
	}

	onBack = func() {
		w.currentPageIndex--
		if w.currentPageIndex < 0 {
			w.currentPageIndex = 0
		}

		switchPage()
	}

	switchPage()

	return w.app.SetRoot(w.pagesView, true).
		EnableMouse(true).
		EnablePaste(true).Run()

}

func (w *Wizard) addSwitchFocusEvent(forFocus []tview.Primitive) {
	curIndex := 0

	w.pagesView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			curIndex = (curIndex + 1) % len(forFocus)
			w.app.SetFocus(forFocus[curIndex])
		case tcell.KeyEscape:
			w.app.Stop()
		}
		return event
	})
}
