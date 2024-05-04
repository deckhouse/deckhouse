package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/state"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/widget"
)

type registryState interface {
	SetRegistryRepo(string) error
	SetRegistryUser(string)
	SetRegistryPassword(string)
	SetRegistrySchema(string) error
	SetRegistryCA(string)
}

type registrySchema interface {
	DefaultRegistryRepo() string
	DefaultRegistryUser() string

	HasCreds() bool
}

func newRegistryPage(st registryState, schema registrySchema, onNext func(), onBack func()) (tview.Primitive, []tview.Primitive) {
	const (
		constInputsWidth = 30

		repoLabel     = "Repository"
		userLabel     = "User"
		passwordLabel = "Password"
		schemaLabel   = "Schema"
		caLabel       = "CA"
	)

	form := tview.NewForm()

	form.AddInputField(repoLabel, schema.DefaultRegistryRepo(), constInputsWidth, nil, nil)

	if schema.HasCreds() {
		form.AddInputField(userLabel, schema.DefaultRegistryUser(), constInputsWidth, nil, func(text string) {
			st.SetRegistryUser(text)
		})
		form.AddPasswordField(passwordLabel, "", constInputsWidth, '*', func(text string) {
			st.SetRegistryPassword(text)
		})

		form.AddDropDown(schemaLabel, []string{state.RegistryHTTPS, state.RegistryHTTPS}, 0, func(option string, optionIndex int) {
			st.SetRegistrySchema(option)
		})

		form.AddTextArea(caLabel, "", constInputsWidth, 2, 0, func(text string) {
			st.SetRegistryCA(text)
		})
	}

	errorLbl := tview.NewTextView().SetTextColor(tcell.ColorRed)

	optionsGrid := tview.NewGrid().
		SetColumns(0).SetRows(0, 5).
		AddItem(form, 0, 0, 1, 1, 0, 0, true).
		AddItem(errorLbl, 1, 0, 1, 1, 0, 0, false)

	p, focusable := widget.OptionsPage("Registry settings", optionsGrid, func() {
		repo := form.GetFormItemByLabel(repoLabel).(*tview.InputField).GetText()
		if err := st.SetRegistryRepo(repo); err != nil {
			errorLbl.SetText(err.Error())
			return
		}

		if schema.HasCreds() {
			user := form.GetFormItemByLabel(userLabel).(*tview.InputField).GetText()
			st.SetRegistryUser(user)

			passwd := form.GetFormItemByLabel(passwordLabel).(*tview.InputField).GetText()
			st.SetRegistryPassword(passwd)

			_, s := form.GetFormItem(form.GetFormItemIndex(schemaLabel)).(*tview.DropDown).GetCurrentOption()
			if err := st.SetRegistrySchema(s); err != nil {
				errorLbl.SetText(err.Error())
				return
			}

			ca := form.GetFormItemByLabel(caLabel).(*tview.TextArea).GetText()
			st.SetRegistryCA(ca)
		}

		onNext()
	}, onBack)

	return p, append([]tview.Primitive{optionsGrid}, focusable...)
}
