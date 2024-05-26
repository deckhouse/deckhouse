package deckhouse

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/state"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/internal/widget"
)

type registryState interface {
	SetRegistryRepo(string) error
	SetRegistryUser(string)
	SetRegistryPassword(string)
	SetRegistrySchema(string) error
	SetRegistryCA(string)

	GetRegistryRepo() string
	GetRegistryUser() string
	GetRegistryPassword() string
	GetRegistrySchema() string
	GetRegistryCA() string
}

type registrySchema interface {
	HasCreds() bool
}

type RegistryPage struct {
	st     registryState
	schema registrySchema
}

func NewRegistryPage(st registryState, schema registrySchema) *RegistryPage {
	return &RegistryPage{
		st:     st,
		schema: schema,
	}
}

func (p *RegistryPage) MouseEnabled() bool {
	return true
}

func (p *RegistryPage) Show(onNext func(), onBack func()) (tview.Primitive, []tview.Primitive) {
	const (
		constInputsWidth = 30

		repoLabel     = "Repository"
		userLabel     = "User"
		passwordLabel = "Password"
		schemaLabel   = "Schema"
		caLabel       = "CA"
	)

	form := tview.NewForm()

	form.AddInputField(repoLabel, p.st.GetRegistryRepo(), constInputsWidth, nil, nil)

	if p.schema.HasCreds() {
		form.AddInputField(userLabel, p.st.GetRegistryUser(), constInputsWidth, nil, nil)
		form.AddPasswordField(passwordLabel, p.st.GetRegistryPassword(), constInputsWidth, '*', nil)

		schemas := []string{state.RegistryHTTPS, state.RegistryHTTP}
		i := 0
		for indx, schema := range schemas {
			if schema == p.st.GetRegistrySchema() {
				i = indx
				break
			}
		}

		form.AddDropDown(schemaLabel, schemas, i, nil)
		form.AddTextArea(caLabel, p.st.GetRegistryCA(), constInputsWidth, 2, 0, nil)
	}

	errorLbl := tview.NewTextView().SetTextColor(tcell.ColorRed)

	optionsGrid := tview.NewGrid().
		SetColumns(0).SetRows(0, 5).
		AddItem(form, 0, 0, 1, 1, 0, 0, true).
		AddItem(errorLbl, 1, 0, 1, 1, 0, 0, false)

	pp, focusable := widget.OptionsPage("Registry settings", optionsGrid, func() {
		repo := form.GetFormItemByLabel(repoLabel).(*tview.InputField).GetText()
		if err := p.st.SetRegistryRepo(repo); err != nil {
			errorLbl.SetText(err.Error())
			return
		}

		if p.schema.HasCreds() {
			user := form.GetFormItemByLabel(userLabel).(*tview.InputField).GetText()
			p.st.SetRegistryUser(user)

			passwd := form.GetFormItemByLabel(passwordLabel).(*tview.InputField).GetText()
			p.st.SetRegistryPassword(passwd)

			_, s := form.GetFormItem(form.GetFormItemIndex(schemaLabel)).(*tview.DropDown).GetCurrentOption()
			if err := p.st.SetRegistrySchema(s); err != nil {
				errorLbl.SetText(err.Error())
				return
			}

			ca := form.GetFormItemByLabel(caLabel).(*tview.TextArea).GetText()
			p.st.SetRegistryCA(ca)
		}

		onNext()
	}, onBack)

	return pp, append([]tview.Primitive{optionsGrid}, focusable...)
}
