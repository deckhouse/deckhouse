package deckhouse

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

	GetCNIType() string
	GetFlannelMode() string

	GetProvider() string
}

type cniSchema interface {
	GetCNIsForProvider(string) []string
	GetFlannelModes() []string
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

	initFlannelMode := false

	addFlannelMode := func() {
		if indx := form.GetFormItemIndex(flannelModeLabel); indx < 0 {
			flannelModes := c.schema.GetFlannelModes()
			i := 0
			for indx, flannelMode := range flannelModes {
				if flannelMode == c.st.GetFlannelMode() {
					i = indx
					break
				}
			}
			form.AddDropDown(flannelModeLabel, flannelModes, i, nil)
		}
	}

	cnis := c.schema.GetCNIsForProvider(c.st.GetProvider())
	i := 0
	for indx, cni := range cnis {
		if cni == c.st.GetCNIType() {
			i = indx
			break
		}
	}

	form.AddDropDown(cniLabel, cnis, i, func(option string, optionIndex int) {
		if initFlannelMode && option == state.CNIFlannel {
			addFlannelMode()
			return
		}

		if indx := form.GetFormItemIndex(flannelModeLabel); indx >= 0 {
			form.RemoveFormItem(indx)
		}
	})

	if c.st.GetCNIType() == state.CNIFlannel {
		addFlannelMode()
	}

	initFlannelMode = true

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
