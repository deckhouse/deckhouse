package ui

import (
	"github.com/rivo/tview"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/widget"
)

func SelectClusterForm(b *configBuilder, onNext func()) (tview.Primitive, []tview.Primitive) {
	optionsGrid := tview.NewGrid().
		SetColumns(30, 0).SetRows(2, 2)

	lblType := tview.NewTextView().
		SetText("Choice cluster type:")

	lblProvider := tview.NewTextView().
		SetText("Provider:")

	providers := tview.NewDropDown().SetOptions([]string{"Yandex", "Openstack", "GCP"}, func(text string, _ int) {
	}).SetFieldWidth(20).SetCurrentOption(0)

	types := tview.NewDropDown().SetOptions([]string{staticCluster, cloudCluster}, func(text string, _ int) {
		switch text {
		case staticCluster:
			providers.SetCurrentOption(0)
			providers.SetDisabled(true)
		case cloudCluster:
			providers.SetDisabled(false)
		}
		b.setClusterType(text)
	}).SetFieldWidth(20).SetCurrentOption(0)

	optionsGrid.AddItem(lblType, 0, 0, 1, 1, 0, 0, false).
		AddItem(types, 0, 1, 1, 1, 0, 0, false).
		AddItem(lblProvider, 1, 0, 1, 1, 0, 0, false).
		AddItem(providers, 1, 1, 1, 1, 0, 0, false)

	p, focusable := widget.OptionsPage(optionsGrid, onNext, nil)

	return p, append([]tview.Primitive{types, providers}, focusable...)
}
