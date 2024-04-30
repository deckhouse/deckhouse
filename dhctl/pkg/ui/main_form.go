package ui

import (
	"github.com/rivo/tview"
)

func MainFormPanel(onStart func()) (tview.Primitive, []tview.Primitive) {
	lbl1 := tview.NewTextView().SetText("Welcome to Deckhouse kubernetes platform bootstrap wizard!").SetTextAlign(tview.AlignCenter)
	btn1 := tview.NewButton("Shall we begin?").SetSelectedFunc(onStart)

	return tview.NewGrid().
			SetColumns(0, 24, 0).SetRows(0, 3, 3, 0).
			AddItem(box(), 0, 0, 1, 3, 0, 50, false).
			AddItem(lbl1, 1, 0, 1, 3, 0, 50, false).
			AddItem(box(), 2, 0, 1, 1, 0, 50, false).
			AddItem(btn1, 2, 1, 1, 1, 0, 50, true).
			AddItem(box(), 3, 0, 1, 1, 0, 50, false),
		[]tview.Primitive{btn1}
}
