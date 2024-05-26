package static

import (
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/discovery"
	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/state"

	"github.com/gdamore/tcell/v2"
	"github.com/hashicorp/go-multierror"
	"github.com/rivo/tview"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/internal/widget"
)

type staticNetworkState interface {
	SetInternalNetworkCIDR(string) error
	GetInternalNetworkCIDR() string
	GetSSHState() state.SSHState
}

type InternalNetworkPage struct {
	st staticNetworkState
}

func NewInternalNetworkPage(st staticNetworkState) *InternalNetworkPage {
	return &InternalNetworkPage{
		st: st,
	}
}

func (p *InternalNetworkPage) MouseEnabled() bool {
	return true
}

func (p *InternalNetworkPage) Show(onNext func(), onBack func()) (tview.Primitive, []tview.Primitive) {
	const (
		internalNetworkLabel = "Internal network CIDR"
	)

	d, err := discovery.NewStaticDiscoverer(p.st.GetSSHState())
	if err != nil {
		panic(err)
	}

	cidrs, err := d.GetInternalNetworkCIDR()
	if err != nil {
		panic(err)
	}

	form := tview.NewForm()

	indx := 0
	for i, cidr := range cidrs {
		if cidr == p.st.GetInternalNetworkCIDR() {
			indx = i
		}
	}

	form.AddDropDown(internalNetworkLabel, cidrs, indx, nil)

	errorLbl := tview.NewTextView().SetTextColor(tcell.ColorRed)

	optionsGrid := tview.NewGrid().
		SetColumns(0).SetRows(0, 2).
		AddItem(form, 0, 0, 1, 1, 0, 0, true).
		AddItem(errorLbl, 1, 0, 1, 1, 0, 0, false)

	pp, focusable := widget.OptionsPage("Internal network", optionsGrid, func() {
		var allErrs *multierror.Error

		_, sshInternalNetwork := form.GetFormItemByLabel(internalNetworkLabel).(*tview.DropDown).GetCurrentOption()
		if err := p.st.SetInternalNetworkCIDR(sshInternalNetwork); err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("Internal network CIDR %s", err))
		}

		if err := allErrs.ErrorOrNil(); err != nil {
			errorLbl.SetText(err.Error())
			return
		}

		errorLbl.SetText("")
		onNext()
	}, onBack)

	return pp, append([]tview.Primitive{optionsGrid}, focusable...)
}
