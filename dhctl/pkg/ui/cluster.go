package ui

import (
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

func selectClusterForm(st clusterTypeState, schema clusterTypesSchema, onNext func()) (tview.Primitive, []tview.Primitive) {
	const constInputsWidth = 30

	lblType := tview.NewTextView().
		SetText("Type")

	lblK8sVer := tview.NewTextView().
		SetText("Kubernetes version")

	lblProvider := tview.NewTextView().
		SetText("Provider")

	lblPrefix := tview.NewTextView().
		SetText("Prefix")

	providers := tview.NewDropDown().SetOptions(schema.CloudProviders(), func(text string, _ int) {
		st.SetProvider(text)
	}).SetFieldWidth(constInputsWidth).SetCurrentOption(0)

	versions := schema.K8sVersions()
	k8sVersions := tview.NewDropDown().SetOptions(versions, func(text string, _ int) {
		st.SetK8sVersion(text)
	}).SetFieldWidth(constInputsWidth).SetCurrentOption(len(versions) - 1)

	prefix := tview.NewInputField().SetFieldWidth(constInputsWidth)

	types := tview.NewDropDown().
		SetOptions([]string{state.CloudCluster, state.StaticCluster}, func(text string, _ int) {
			switch text {
			case state.StaticCluster:
				providers.SetCurrentOption(0)
				providers.SetDisabled(true)
				prefix.SetDisabled(true)
				prefix.SetText("")
			case state.CloudCluster:
				providers.SetDisabled(false)
				prefix.SetDisabled(false)
			}
			st.SetClusterType(text)
		}).SetFieldWidth(30).SetCurrentOption(0)

	optionsGrid := tview.NewGrid().
		SetColumns(constInputsWidth, 0).SetRows(2, 2, 2, 2).
		//----------------------------------------------------------------------------------------------
		AddItem(lblType, 0, 0, 1, 1, 0, 0, false).
		AddItem(types, 0, 1, 1, 1, 0, 0, false).
		//----------------------------------------------------------------------------------------------
		AddItem(lblK8sVer, 1, 0, 1, 1, 0, 0, false).
		AddItem(k8sVersions, 1, 1, 1, 1, 0, 0, false).
		//----------------------------------------------------------------------------------------------
		AddItem(lblProvider, 2, 0, 1, 1, 0, 0, false).
		AddItem(providers, 2, 1, 1, 1, 0, 0, false).
		//----------------------------------------------------------------------------------------------
		AddItem(lblPrefix, 3, 0, 1, 1, 0, 0, false).
		AddItem(prefix, 3, 1, 1, 1, 0, 0, false)

	p, focusable := widget.OptionsPage("Cluster settings", optionsGrid, func() {
		st.SetClusterPrefix(prefix.GetText())
		onNext()
	}, nil)

	return p, append([]tview.Primitive{types, providers, prefix}, focusable...)
}
