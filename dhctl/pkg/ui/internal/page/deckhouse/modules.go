package deckhouse

import (
	"github.com/gdamore/tcell/v2"
	"github.com/hashicorp/go-multierror"
	"github.com/rivo/tview"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/internal/widget"
)

type modulesState interface {
	SetReleaseChannel(string) error
	SetPublicDomainTemplate(string) error
	EnablePublishK8sAPI(bool)

	GetReleaseChannel() string
	GetPublicDomainTemplate() string
	IsEnablePublishK8sAPI() bool
}

type modulesSchema interface {
	ReleaseChannels() []string
}

type ModulesPage struct {
	st     modulesState
	schema modulesSchema
}

func NewDeckhousePage(st modulesState, schema modulesSchema) *ModulesPage {
	return &ModulesPage{
		st:     st,
		schema: schema,
	}
}

func (p *ModulesPage) MouseEnabled() bool {
	return true
}

func (p *ModulesPage) Show(onNext func(), onBack func()) (tview.Primitive, []tview.Primitive) {
	const (
		constInputsWidth = 30

		releaseChannelLabel       = "Release channel"
		publicDomainTemplateLabel = "Public domain"
		enablePublishAPILabel     = "Publish k8s API"
	)

	form := tview.NewForm()

	channels := p.schema.ReleaseChannels()
	i := 0
	for indx, channel := range channels {
		if channel == p.st.GetReleaseChannel() {
			i = indx
			break
		}
	}
	form.AddDropDown(releaseChannelLabel, channels, i, nil)

	form.AddInputField(publicDomainTemplateLabel, p.st.GetPublicDomainTemplate(), constInputsWidth, nil, nil)
	form.AddCheckbox(enablePublishAPILabel, p.st.IsEnablePublishK8sAPI(), nil)

	errorLbl := tview.NewTextView().SetTextColor(tcell.ColorRed)

	optionsGrid := tview.NewGrid().
		SetColumns(0).SetRows(0, 5).
		AddItem(form, 0, 0, 1, 1, 0, 0, true).
		AddItem(errorLbl, 1, 0, 1, 1, 0, 0, false)

	page, focusable := widget.OptionsPage("Deckhouse settings", optionsGrid, func() {
		_, s := form.GetFormItem(form.GetFormItemIndex(releaseChannelLabel)).(*tview.DropDown).GetCurrentOption()
		var allErrs *multierror.Error

		if err := p.st.SetReleaseChannel(s); err != nil {
			allErrs = multierror.Append(allErrs, err)
		}

		publicDomain := form.GetFormItemByLabel(publicDomainTemplateLabel).(*tview.InputField).GetText()
		if err := p.st.SetPublicDomainTemplate(publicDomain); err != nil {
			allErrs = multierror.Append(allErrs, err)
		}

		enablePublishAPI := form.GetFormItemByLabel(enablePublishAPILabel).(*tview.Checkbox).IsChecked()
		p.st.EnablePublishK8sAPI(enablePublishAPI)

		if err := allErrs.ErrorOrNil(); err != nil {
			errorLbl.SetText(err.Error())
			return
		}

		errorLbl.SetText("")

		onNext()
	}, onBack)

	return page, append([]tview.Primitive{optionsGrid}, focusable...)
}
