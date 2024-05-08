package ui

import (
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
		pageSelectClusterType,
	}

	staticPagesOrder = append(append(append([]string{}, preClusterTypeSteps...), staticPages...), postClusterTypeSteps...)
)

type Page interface {
	Show(onNext, onBack func()) (tview.Primitive, []tview.Primitive)
}

type Wizard struct {
	order            []string
	currentPageIndex int
	pagesView        *tview.Pages
	currentPage      Page
	allPages         map[string]Page

	state       *state.State
	schemaStore *state.Schema
}

func NewWizard(st *state.State, schema *state.Schema) *Wizard {
	// by default
	return &Wizard{
		order:            staticPagesOrder,
		currentPageIndex: 0,
		currentPage:      nil,
		state:            st,
		schemaStore:      schema,
	}
}

func (w *Wizard) Start() error {

}
