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
	SetK8sVersion(string)
	SetClusterPrefix(string)
}

type clusterTypesSchema interface {
	CloudProviders() []string
	K8sVersions() []string
}

func clusterPage(st clusterTypeState, schema clusterTypesSchema, onNext func()) (tview.Primitive, []tview.Primitive) {
	const (
		constInputsWidth = 20
		k8sVersionIndex  = 0
		typeIndex        = 1
		providerIndex    = 2
		prefixIndex      = 3
	)

	form := tview.NewForm()

	initAddProvider := false

	addProviderPrefixItems := func() {
		form.AddDropDown("Provider", schema.CloudProviders(), 0, func(option string, optionIndex int) {
			st.SetProvider(option)
		})

		form.AddInputField("Prefix", "", constInputsWidth, nil, func(text string) {
			st.SetClusterPrefix(text)
		})
	}

	versions := schema.K8sVersions()
	form.AddDropDown("Kubernetes version", versions, len(versions)-1, func(option string, optionIndex int) {
		st.SetK8sVersion(option)
	})

	form.AddDropDown("Type", []string{state.CloudCluster, state.StaticCluster}, 0, func(option string, optionIndex int) {
		switch option {
		case state.StaticCluster:
			if form.GetFormItemCount() > 2 {
				form.RemoveFormItem(2)
				form.RemoveFormItem(2)
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
		_, clType := form.GetFormItem(typeIndex).(*tview.DropDown).GetCurrentOption()
		st.SetClusterType(clType)

		_, k8sVersion := form.GetFormItem(k8sVersionIndex).(*tview.DropDown).GetCurrentOption()
		st.SetK8sVersion(k8sVersion)

		if form.GetFormItemCount() > 2 {
			_, provider := form.GetFormItem(providerIndex).(*tview.DropDown).GetCurrentOption()
			st.SetProvider(provider)

			prefix := form.GetFormItem(prefixIndex).(*tview.InputField).GetText()
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
