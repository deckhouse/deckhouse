package provider

import (
	"github.com/gdamore/tcell/v2"
	"github.com/go-openapi/spec"
	"github.com/rivo/tview"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/internal/widget"
)

type providerSchema interface {
	ProviderSchema(string) (*spec.Schema, error)
}

type providerState interface {
	SetProviderData(map[string]interface{})
	GetProvider() string
}

type ProviderPage struct {
	st     providerState
	schema providerSchema
}

func NewProviderPage(st providerState, schema providerSchema) *ProviderPage {
	return &ProviderPage{
		st:     st,
		schema: schema,
	}
}

func (p *ProviderPage) MouseEnabled() bool {
	return true
}

func (p *ProviderPage) Show(onNext func(), onBack func()) (tview.Primitive, []tview.Primitive) {
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

		onNext()
	}, onBack)

	return pp, append([]tview.Primitive{form}, focusable...)
}
