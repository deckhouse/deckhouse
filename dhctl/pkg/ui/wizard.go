package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/internal/page/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/internal/page/final"
	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/internal/page/provider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/internal/page/static"
	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/internal/page/welcome"
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

	providerGenericPages = []string{
		pageProvider,
	}

	staticPagesOrder = append(append(append([]string{}, preClusterTypeSteps...), staticPages...), postClusterTypeSteps...)
	awsOrder         = append(append(append([]string{}, preClusterTypeSteps...), providerGenericPages...), postClusterTypeSteps...)
	gcpOrder         = append(append(append([]string{}, preClusterTypeSteps...), providerGenericPages...), postClusterTypeSteps...)
	azureOrder       = append(append(append([]string{}, preClusterTypeSteps...), providerGenericPages...), postClusterTypeSteps...)
	yandexOrder      = append(append(append([]string{}, preClusterTypeSteps...), providerGenericPages...), postClusterTypeSteps...)
	openstackOrder   = append(append(append([]string{}, preClusterTypeSteps...), providerGenericPages...), postClusterTypeSteps...)
	vsphereOrder     = append(append(append([]string{}, preClusterTypeSteps...), providerGenericPages...), postClusterTypeSteps...)
	vcdOrder         = append(append(append([]string{}, preClusterTypeSteps...), providerGenericPages...), postClusterTypeSteps...)
	zvirtOrder       = append(append(append([]string{}, preClusterTypeSteps...), providerGenericPages...), postClusterTypeSteps...)

	orders = map[string][]string{
		// static
		"":          staticPagesOrder,
		"AWS":       awsOrder,
		"GCP":       gcpOrder,
		"Azure":     azureOrder,
		"Yandex":    yandexOrder,
		"Openstack": openstackOrder,
		"VSphere":   vsphereOrder,
		"VCD":       vcdOrder,
		"ZVirt":     zvirtOrder,
	}
)

type Page interface {
	Show(onNext, onBack func()) (tview.Primitive, []tview.Primitive)
}

type wizard struct {
	order            []string
	currentPageIndex int
	pagesView        *tview.Pages
	allPages         map[string]Page
	app              *tview.Application

	state       *state.State
	schemaStore *state.Schema
}

func newWizard(app *tview.Application, st *state.State, schema *state.Schema) *wizard {
	allPages := map[string]Page{
		pageWelcome:           welcome.NewWelcomePage(),
		pageSelectClusterType: welcome.NewClusterTypePage(st, schema),
		pageProvider:          provider.NewProviderPage(st, schema),
		pageStaticMaster:      static.NewStaticMasterPage(st),
		pageRegistry:          deckhouse.NewRegistryPage(st, schema),
		pageCluster:           deckhouse.NewClusterPage(st, schema),
		pageCNI:               deckhouse.NewCNIPage(st, schema),
		pageDeckhouse:         deckhouse.NewDeckhousePage(st, schema),
		pageSSH:               final.NewSSHPage(st),
		pageConfig:            final.NewConfigPage(st),
	}

	// by default
	return &wizard{
		order:            staticPagesOrder,
		currentPageIndex: 0,
		state:            st,
		schemaStore:      schema,
		allPages:         allPages,
		pagesView:        tview.NewPages(),
		app:              app,
	}
}

func (w *wizard) Start() error {
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

func (w *wizard) addSwitchFocusEvent(forFocus []tview.Primitive) {
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
