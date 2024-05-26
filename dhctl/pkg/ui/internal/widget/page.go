package widget

import (
	"fmt"
	"reflect"

	"github.com/rivo/tview"
)

func OptionsPage(title string, child tview.Primitive, onNext func(), onBack func()) (tview.Primitive, []tview.Primitive) {
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
		SetColumns(0, 2, 0).
		AddItem(backBtn, 1, 0, 1, 1, 0, 0, false).
		AddItem(tview.NewBox(), 1, 1, 1, 1, 0, 0, false).
		AddItem(nextBtn, 1, 2, 1, 1, 0, 0, false)

	titleTxt := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetText(fmt.Sprintf("Deckhouse bootstrap wizard. %s", title))

	mainGrid := tview.NewGrid().
		SetColumns(10, 0, 10).SetRows(1, 0, 1).
		AddItem(titleTxt, 0, 0, 1, 3, 0, 0, false).
		AddItem(child, 1, 1, 1, 1, 0, 0, false).
		AddItem(nextBtnContainer, 2, 2, 1, 1, 0, 0, false)

	mainGrid.SetBorderPadding(1, 1, 1, 1)

	return mainGrid, focusable
}
