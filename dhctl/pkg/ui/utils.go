package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func addSwitchFocusEvent(app *tview.Application, parent *tview.Pages, forFocus []tview.Primitive) {
	curIndex := 0

	parent.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			curIndex = (curIndex + 1) % len(forFocus)
			app.SetFocus(forFocus[curIndex])
		case tcell.KeyEscape:
			app.Stop()
		}
		return event
	})
}
