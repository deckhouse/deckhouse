package ui

import (
	"github.com/gdamore/tcell/v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/internal/template"

	"github.com/f1bonacc1/glippy"
	"github.com/rivo/tview"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/internal/widget"
)

type configState interface {
	template.State
	SetConfigYAML(data string) error
}

type ConfigPage struct {
	st configState
}

func NewConfigPage(st configState) *ConfigPage {
	return &ConfigPage{
		st: st,
	}
}

func (c *ConfigPage) Show(onNext func(), onBack func()) (tview.Primitive, []tview.Primitive) {
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
		onNext()

	}, onBack)

	return p, append([]tview.Primitive{view}, focusable...)
}
