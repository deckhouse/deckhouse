package widget

import (
	"reflect"

	"github.com/rivo/tview"
)

func OptionsPage(child tview.Primitive, onNext func(), onBack func()) (tview.Primitive, []tview.Primitive) {
	v := reflect.ValueOf(child)
	m := v.MethodByName("SetBorderPadding")
	if m.IsValid() {
		m.Call([]reflect.Value{reflect.ValueOf(1), reflect.ValueOf(0), reflect.ValueOf(0), reflect.ValueOf(0)})
	}

	focusable := make([]tview.Primitive, 0, 2)

	var nextBtn tview.Primitive = tview.NewBox()
	if onNext != nil {
		nextBtn = tview.NewButton("Next").SetSelectedFunc(onNext)
		focusable = append(focusable, nextBtn)
	}

	var backBtn tview.Primitive = tview.NewBox()
	if onBack != nil {
		backBtn = tview.NewButton("Back").SetSelectedFunc(onBack)
		focusable = append(focusable, backBtn)
	}

	nextBtnContainer := tview.NewGrid().
		AddItem(backBtn, 1, 0, 1, 1, 0, 0, false).
		AddItem(nextBtn, 1, 1, 1, 1, 0, 0, false)

	title := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetText("DKP bootstrap wizard")

	mainGrid := tview.NewGrid().
		SetColumns(40, 0, 40).SetRows(1, 0, 1).
		AddItem(title, 0, 0, 1, 3, 0, 0, false).
		AddItem(child, 1, 1, 1, 1, 0, 0, false).
		AddItem(nextBtnContainer, 2, 2, 1, 1, 0, 0, false)

	mainGrid.SetBorderPadding(1, 1, 1, 1)

	return mainGrid, focusable
}
