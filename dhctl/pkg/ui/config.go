package ui

import (
	"github.com/gdamore/tcell/v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/template"

	"github.com/f1bonacc1/glippy"
	"github.com/rivo/tview"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/widget"
)

type configState interface {
	template.State
	SetConfigYAML(data string) error
}

type configPage struct {
	st     configState
	onNext func()
	onBack func()
}

func newConfigPage(st configState, onNext func(), onBack func()) *configPage {
	return &configPage{
		st:     st,
		onBack: onBack,
		onNext: onNext,
	}
}

func (c *configPage) Show() (tview.Primitive, []tview.Primitive) {
	configYAML, err := template.RenderTemplate(c.st)
	if err != nil {
		panic(err)
	}

	view := tview.NewTextArea().SetText(configYAML, true).
		SetClipboard(func(s string) {
			glippy.Set(s)
		}, func() string {
			s, _ := glippy.Get()
			return s
		})

	view.SetBorder(true)

	errorLbl := tview.NewTextView().SetTextColor(tcell.ColorRed)

	optionsGrid := tview.NewGrid().
		SetColumns(0).SetRows(0, 4).
		AddItem(view, 0, 0, 1, 1, 0, 0, true).
		AddItem(errorLbl, 1, 0, 1, 1, 0, 0, false)

	p, focusable := widget.OptionsPage("Final configuration", optionsGrid, func() {
		if err := c.st.SetConfigYAML(view.GetText()); err != nil {
			errorLbl.SetText(err.Error())
			return
		}

		errorLbl.SetText("")
		c.onNext()

	}, c.onBack)

	return p, append([]tview.Primitive{view}, focusable...)
}
