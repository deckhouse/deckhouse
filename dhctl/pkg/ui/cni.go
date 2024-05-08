package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/hashicorp/go-multierror"
	"github.com/rivo/tview"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/internal/widget"
	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/state"
)

type cniState interface {
	SetCNIType(string) error
	SetFlannelMode(string) error

	GetProvider() string
}

type cniSchema interface {
	GetCNIsForProvider(string) []string
}

type CniPage struct {
	st     cniState
	schema cniSchema
}

func NewCNIPage(st cniState, schema cniSchema) *CniPage {
	return &CniPage{
		st:     st,
		schema: schema,
	}
}

func (c *CniPage) Show(onNext func(), onBack func()) (tview.Primitive, []tview.Primitive) {
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
		onNext()
	}, onBack)

	return p, append([]tview.Primitive{optionsGrid}, focusable...)
}
