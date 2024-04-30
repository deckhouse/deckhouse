package ui

import (
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

func selectClusterForm(st clusterTypeState, schema clusterTypesSchema, onNext func()) (tview.Primitive, []tview.Primitive) {

	const constInputsWidth = 30

	optionsGrid := tview.NewGrid().
		SetColumns(constInputsWidth, 0).SetRows(2, 2, 2)

	lblType := tview.NewTextView().
		SetText("Cluster type:")

	lblProvider := tview.NewTextView().
		SetText("Provider:")

	lblPrefix := tview.NewTextView().
		SetText("Prefix:")

	providers := tview.NewDropDown().SetOptions(schema.CloudProviders(), func(text string, _ int) {
		st.SetProvider(text)
	}).SetFieldWidth(constInputsWidth).SetCurrentOption(0)

	prefix := tview.NewInputField().SetFieldWidth(constInputsWidth)

	types := tview.NewDropDown().
		SetOptions([]string{state.StaticCluster, state.CloudCluster}, func(text string, _ int) {
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

	optionsGrid.AddItem(lblType, 0, 0, 1, 1, 0, 0, false).
		AddItem(types, 0, 1, 1, 1, 0, 0, false).
		AddItem(lblProvider, 1, 0, 1, 1, 0, 0, false).
		AddItem(providers, 1, 1, 1, 1, 0, 0, false).
		AddItem(lblPrefix, 2, 0, 1, 1, 0, 0, false).
		AddItem(prefix, 2, 1, 1, 1, 0, 0, false)

	p, focusable := widget.OptionsPage(optionsGrid, func() {
		st.SetClusterPrefix(prefix.GetText())
		onNext()
	}, nil)

	return p, append([]tview.Primitive{types, providers, prefix}, focusable...)
}
