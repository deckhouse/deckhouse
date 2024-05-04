package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/go-openapi/spec"
	"github.com/rivo/tview"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/widget"
)

type providerSchema interface {
	ProviderSchema(string) (*spec.Schema, error)
}

type providerState interface {
	SetProviderData(map[string]interface{})
	GetProvider() string
}

type providerPage struct {
	st     providerState
	schema providerSchema
	onNext func()
	onBack func()
}

func newProviderPage(st providerState, schema providerSchema, onNext func(), onBack func()) *providerPage {
	return &providerPage{
		st:     st,
		schema: schema,
		onBack: onBack,
		onNext: onNext,
	}

}

func (p *providerPage) Show() (tview.Primitive, []tview.Primitive) {
	const inputsWidth = 30

	providerName := p.st.GetProvider()
	providerS, _ := p.schema.ProviderSchema(providerName)

	form := widget.NewOpenapiForm(providerS, inputsWidth)

	errorLbl := tview.NewTextView().SetTextColor(tcell.ColorRed)

	optionsGrid := tview.NewGrid().
		SetColumns(0).SetRows(0, 5).
		AddItem(form, 0, 0, 1, 1, 0, 0, true).
		AddItem(errorLbl, 1, 0, 1, 1, 0, 0, false)

	pp, focusable := widget.OptionsPage("Provider settings", optionsGrid, func() {
		if err := form.Validate(); err != nil {
			errorLbl.SetText(err.Error())
			return
		}

		errorLbl.SetText("")
		p.st.SetProviderData(form.Data())

		p.onNext()
	}, p.onBack)

	return pp, append([]tview.Primitive{form}, focusable...)
}
