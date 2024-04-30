package ui

import (
	"github.com/rivo/tview"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/widget"
)

func SelectClusterForm(b *configBuilder, onNext func()) (tview.Primitive, []tview.Primitive) {
	lbl1 := tview.NewTextView().
		SetText("Choice cluster type:")

	tview.NewList()

	types := tview.NewDropDown().SetOptions([]string{cloudCluster, staticCluster}, func(text string, _ int) {
		b.setClusterType(text)
	}).SetFieldWidth(20).SetCurrentOption(0)

	optionsGrid := tview.NewGrid().
		SetColumns(30, 0).
		AddItem(lbl1, 0, 0, 1, 1, 0, 0, false).
		AddItem(types, 0, 1, 1, 1, 0, 0, false)

	p, focusable := widget.OptionsPage(optionsGrid, onNext, nil)

	return p, append([]tview.Primitive{types}, focusable...)
}
