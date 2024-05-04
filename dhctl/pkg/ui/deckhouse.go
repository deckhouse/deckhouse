package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/hashicorp/go-multierror"
	"github.com/rivo/tview"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/widget"
)

type deckhouseState interface {
	SetReleaseChannel(string) error
	SetPublicDomainTemplate(string) error
	EnablePublishK8sAPI(bool)
}

type deckhouseSchema interface {
	ReleaseChannels() []string
}

func newDeckhousePage(st deckhouseState, schema deckhouseSchema, onNext func(), onBack func()) (tview.Primitive, []tview.Primitive) {
	const (
		constInputsWidth = 30

		releaseChannelLabel       = "Release channel"
		publicDomainTemplateLabel = "Public domain"
		enablePublishAPILabel     = "Publish k8s API"
	)

	form := tview.NewForm()

	channels := schema.ReleaseChannels()
	form.AddDropDown(releaseChannelLabel, channels, len(channels)-1, nil)

	form.AddInputField(publicDomainTemplateLabel, "%s.example.com", constInputsWidth, nil, nil)
	form.AddCheckbox(enablePublishAPILabel, true, nil)

	errorLbl := tview.NewTextView().SetTextColor(tcell.ColorRed)

	optionsGrid := tview.NewGrid().
		SetColumns(0).SetRows(0, 5).
		AddItem(form, 0, 0, 1, 1, 0, 0, true).
		AddItem(errorLbl, 1, 0, 1, 1, 0, 0, false)

	p, focusable := widget.OptionsPage("Deckhouse settings", optionsGrid, func() {
		_, s := form.GetFormItem(form.GetFormItemIndex(releaseChannelLabel)).(*tview.DropDown).GetCurrentOption()
		var allErrs *multierror.Error

		if err := st.SetReleaseChannel(s); err != nil {
			allErrs = multierror.Append(allErrs, err)
		}

		publicDomain := form.GetFormItemByLabel(publicDomainTemplateLabel).(*tview.InputField).GetText()
		if err := st.SetPublicDomainTemplate(publicDomain); err != nil {
			allErrs = multierror.Append(allErrs, err)
		}

		enablePublishAPI := form.GetFormItemByLabel(enablePublishAPILabel).(*tview.Checkbox).IsChecked()
		st.EnablePublishK8sAPI(enablePublishAPI)

		if err := allErrs.ErrorOrNil(); err != nil {
			errorLbl.SetText(err.Error())
			return
		}

		errorLbl.SetText("")

		onNext()
	}, onBack)

	return p, append([]tview.Primitive{optionsGrid}, focusable...)
}
