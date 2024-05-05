package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/hashicorp/go-multierror"
	"github.com/rivo/tview"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/widget"
)

type cniState interface {
	SetCNIType(string) error
	SetFlannelMode(string) error

	GetProvider() string
}

type cniSchema interface {
	GetCNIsForProvider(string) []string
}

type cniPage struct {
	st     cniState
	schema cniSchema
	onNext func()
	onBack func()
}

func newCNIPage(st cniState, schema cniSchema, onNext func(), onBack func()) *cniPage {
	return &cniPage{
		st:     st,
		schema: schema,
		onBack: onBack,
		onNext: onNext,
	}

}

func (c *cniPage) Show() (tview.Primitive, []tview.Primitive) {
	const (
		cniLabel         = "CNI"
		flannelModeLabel = "Flannel mode"
	)

	form := tview.NewForm()

	cnis := c.schema.GetCNIsForProvider(c.st.GetProvider())
	form.AddDropDown(cniLabel, cnis, 0, func(option string, optionIndex int) {
		if option == state.CNIFlannel {
			if indx := form.GetFormItemIndex(flannelModeLabel); indx < 0 {
				form.AddDropDown(flannelModeLabel, []string{state.FlannelVxLAN, state.FlannelHostGW}, 0, nil)
			}
			return
		}

		if indx := form.GetFormItemIndex(flannelModeLabel); indx >= 0 {
			form.RemoveFormItem(indx)
		}
	})

	errorLbl := tview.NewTextView().SetTextColor(tcell.ColorRed)

	optionsGrid := tview.NewGrid().
		SetColumns(0).SetRows(0, 5).
		AddItem(form, 0, 0, 1, 1, 0, 0, true).
		AddItem(errorLbl, 1, 0, 1, 1, 0, 0, false)

	p, focusable := widget.OptionsPage("Container network interface", optionsGrid, func() {
		var allErrs *multierror.Error

		_, cni := form.GetFormItemByLabel(cniLabel).(*tview.DropDown).GetCurrentOption()
		if err := c.st.SetCNIType(cni); err != nil {
			allErrs = multierror.Append(allErrs, err)
		}

		if cni == state.CNIFlannel {
			_, mode := form.GetFormItemByLabel(flannelModeLabel).(*tview.DropDown).GetCurrentOption()

			if err := c.st.SetFlannelMode(mode); err != nil {
				allErrs = multierror.Append(allErrs, err)
			}
		}

		if err := allErrs.ErrorOrNil(); err != nil {
			errorLbl.SetText(err.Error())
			return
		}

		errorLbl.SetText("")
		c.onNext()
	}, c.onBack)

	return p, append([]tview.Primitive{optionsGrid}, focusable...)
}
