package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/widget"
)

type clusterTypeState interface {
	SetClusterType(string)
	SetProvider(string)
	SetClusterPrefix(string)
}

type clusterTypesSchema interface {
	CloudProviders() []string
}

func newClusterTypePage(st clusterTypeState, schema clusterTypesSchema, onNext func()) (tview.Primitive, []tview.Primitive) {
	const (
		constInputsWidth = 30

		providerLabel   = "Provider"
		prefixLabel     = "Prefix"
		k8sVersionLabel = "Kubernetes Version"
		typeLabel       = "Type"
	)

	form := tview.NewForm()

	initAddProvider := false

	addProviderPrefixItems := func() {
		if form.GetFormItemIndex(providerLabel) < 0 {
			form.AddDropDown(providerLabel, schema.CloudProviders(), 0, func(option string, optionIndex int) {
				st.SetProvider(option)
			})
		}

		if form.GetFormItemIndex(prefixLabel) < 0 {
			form.AddInputField(prefixLabel, "", constInputsWidth, nil, func(text string) {
				st.SetClusterPrefix(text)
			})
		}
	}

	form.AddDropDown(typeLabel, []string{state.CloudCluster, state.StaticCluster}, 0, func(option string, optionIndex int) {
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

		st.SetClusterType(option)
	})

	addProviderPrefixItems()

	errorLbl := tview.NewTextView().SetTextColor(tcell.ColorRed)

	optionsGrid := tview.NewGrid().
		SetColumns(0).SetRows(0, 5).
		AddItem(form, 0, 0, 1, 1, 0, 0, true).
		AddItem(errorLbl, 1, 0, 1, 1, 0, 0, false)

	p, focusable := widget.OptionsPage("Cluster settings", optionsGrid, func() {
		_, clType := form.GetFormItemByLabel(typeLabel).(*tview.DropDown).GetCurrentOption()
		st.SetClusterType(clType)

		if clType == state.CloudCluster {
			_, provider := form.GetFormItemByLabel(providerLabel).(*tview.DropDown).GetCurrentOption()
			st.SetProvider(provider)

			prefix := form.GetFormItemByLabel(prefixLabel).(*tview.InputField).GetText()
			if len(prefix) < 1 {
				errorLbl.SetText("Prefix is required")
				return
			}
			st.SetClusterPrefix(prefix)
		}

		onNext()
	}, nil)

	return p, append([]tview.Primitive{optionsGrid}, focusable...)
}
