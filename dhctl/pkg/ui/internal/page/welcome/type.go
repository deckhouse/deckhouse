package welcome

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/internal/widget"
	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/state"
)

type clusterTypeState interface {
	SetClusterType(string)
	SetProvider(string)
	SetClusterPrefix(string)

	GetClusterType() string
	GetProvider() string
	GetClusterPrefix() string
}

type clusterTypesSchema interface {
	CloudProviders() []string
}

type ClusterTypePage struct {
	st     clusterTypeState
	schema clusterTypesSchema
}

func NewClusterTypePage(st clusterTypeState, schema clusterTypesSchema) *ClusterTypePage {
	return &ClusterTypePage{
		st:     st,
		schema: schema,
	}
}

func (p *ClusterTypePage) Show(onNext func(), onBack func()) (tview.Primitive, []tview.Primitive) {
	const (
		constInputsWidth = 30

		providerLabel = "Provider"
		prefixLabel   = "Prefix"
		typeLabel     = "Type"
	)

	form := tview.NewForm()

	initAddProvider := false

	addProviderPrefixItems := func() {
		if form.GetFormItemIndex(providerLabel) < 0 {
			providers := p.schema.CloudProviders()
			i := 0
			for indx, provider := range providers {
				if provider == p.st.GetProvider() {
					i = indx
					break
				}
			}
			form.AddDropDown(providerLabel, providers, i, func(option string, optionIndex int) {
				p.st.SetProvider(option)
			})
		}

		if form.GetFormItemIndex(prefixLabel) < 0 {
			form.AddInputField(prefixLabel, p.st.GetClusterPrefix(), constInputsWidth, nil, func(text string) {
				p.st.SetClusterPrefix(text)
			})
		}
	}

	types := []string{state.CloudCluster, state.StaticCluster}
	i := 0
	for indx, tp := range types {
		if tp == p.st.GetClusterType() {
			i = indx
			break
		}
	}
	form.AddDropDown(typeLabel, types, i, func(option string, optionIndex int) {
		switch option {
		case state.StaticCluster:
			if indx := form.GetFormItemIndex(prefixLabel); indx >= 0 {
				form.RemoveFormItem(indx)
			}
			if indx := form.GetFormItemIndex(providerLabel); indx >= 0 {
				form.RemoveFormItem(indx)
			}
		case state.CloudCluster:
			if initAddProvider && form.GetFormItemCount() < 4 {
				addProviderPrefixItems()
			}

			initAddProvider = true
		}

		p.st.SetClusterType(option)
	})

	if p.st.GetClusterType() == state.CloudCluster {
		addProviderPrefixItems()
	}

	errorLbl := tview.NewTextView().SetTextColor(tcell.ColorRed)

	optionsGrid := tview.NewGrid().
		SetColumns(0).SetRows(0, 5).
		AddItem(form, 0, 0, 1, 1, 0, 0, true).
		AddItem(errorLbl, 1, 0, 1, 1, 0, 0, false)

	pp, focusable := widget.OptionsPage("Cluster settings", optionsGrid, func() {
		_, clType := form.GetFormItemByLabel(typeLabel).(*tview.DropDown).GetCurrentOption()
		p.st.SetClusterType(clType)

		if clType == state.CloudCluster {
			_, provider := form.GetFormItemByLabel(providerLabel).(*tview.DropDown).GetCurrentOption()
			p.st.SetProvider(provider)

			prefix := form.GetFormItemByLabel(prefixLabel).(*tview.InputField).GetText()
			if len(prefix) < 1 {
				errorLbl.SetText("Prefix is required")
				return
			}
			p.st.SetClusterPrefix(prefix)
		} else {
			p.st.SetProvider("")
		}

		onNext()
	}, nil)

	return pp, append([]tview.Primitive{optionsGrid}, focusable...)
}
