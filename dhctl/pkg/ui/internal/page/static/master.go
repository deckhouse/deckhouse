package static

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/hashicorp/go-multierror"
	"github.com/rivo/tview"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/internal/widget"
)

type staticMasterState interface {
	SetSSHUser(string) error
	SetSSHHost(string) error
	SetInternalNetworkCIDR(string) error
	SetUsePasswordForSudo(bool)

	SetBastionSSHUser(string)
	SetBastionSSHHost(string)

	GetSSHUser() string
	GetSSHHost() string
	GetInternalNetworkCIDR() string
	IsUsePasswordForSudo() bool

	GetBastionSSHUser() string
	GetBastionSSHHost() string
}

type MasterPage struct {
	st staticMasterState
}

func NewStaticMasterPage(st staticMasterState) *MasterPage {
	return &MasterPage{
		st: st,
	}
}

func (p *MasterPage) Show(onNext func(), onBack func()) (tview.Primitive, []tview.Primitive) {
	const (
		constInputsWidth = 30

		sshHostLabel         = "SSH host"
		sshUserLabel         = "SSH user name"
		internalNetworkLabel = "Internal network CIDR"
		askSudoPasswordLabel = "Ask sudo password"

		useBastionHostLabel  = "Use bastion host"
		bastionHostLabel     = "Bastion host"
		bastionHostUserLabel = "Bastion host user name"
	)

	form := tview.NewForm()
	form.AddInputField(sshHostLabel, p.st.GetSSHHost(), constInputsWidth, nil, nil)
	form.AddInputField(sshUserLabel, p.st.GetSSHUser(), constInputsWidth, nil, nil)

	form.AddInputField(internalNetworkLabel, p.st.GetInternalNetworkCIDR(), constInputsWidth, nil, nil)

	form.AddCheckbox(askSudoPasswordLabel, p.st.IsUsePasswordForSudo(), nil)

	checked := p.st.GetBastionSSHHost() != ""
	form.AddCheckbox(useBastionHostLabel, checked, func(check bool) {
		if check {
			if form.GetFormItemIndex(bastionHostLabel) < 0 {
				form.AddInputField(bastionHostLabel, p.st.GetBastionSSHHost(), constInputsWidth, nil, nil)
			}

			if form.GetFormItemIndex(bastionHostUserLabel) < 0 {
				form.AddInputField(bastionHostUserLabel, p.st.GetBastionSSHUser(), constInputsWidth, nil, nil)
			}

			return
		}

		if indx := form.GetFormItemIndex(bastionHostLabel); indx >= 0 {
			form.RemoveFormItem(indx)
		}

		if indx := form.GetFormItemIndex(bastionHostUserLabel); indx >= 0 {
			form.RemoveFormItem(indx)
		}
	})

	errorLbl := tview.NewTextView().SetTextColor(tcell.ColorRed)

	optionsGrid := tview.NewGrid().
		SetColumns(0).SetRows(0, 2).
		AddItem(form, 0, 0, 1, 1, 0, 0, true).
		AddItem(errorLbl, 1, 0, 1, 1, 0, 0, false)

	pp, focusable := widget.OptionsPage("First control-plane node settings", optionsGrid, func() {
		var allErrs *multierror.Error

		sshHost := form.GetFormItemByLabel(sshHostLabel).(*tview.InputField).GetText()
		if err := p.st.SetSSHHost(sshHost); err != nil {
			allErrs = multierror.Append(allErrs, err)
		}

		sshUser := form.GetFormItemByLabel(sshUserLabel).(*tview.InputField).GetText()
		if err := p.st.SetSSHUser(sshUser); err != nil {
			allErrs = multierror.Append(allErrs, err)
		}

		sshInternalNetwork := form.GetFormItemByLabel(internalNetworkLabel).(*tview.InputField).GetText()
		if err := p.st.SetInternalNetworkCIDR(sshInternalNetwork); err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("Internal network CIDR %s", err))
		}

		askSudoPassword := form.GetFormItemByLabel(askSudoPasswordLabel).(*tview.Checkbox).IsChecked()
		p.st.SetUsePasswordForSudo(askSudoPassword)

		useBastion := form.GetFormItemByLabel(useBastionHostLabel).(*tview.Checkbox).IsChecked()
		if useBastion {
			sshHost := form.GetFormItemByLabel(bastionHostLabel).(*tview.InputField).GetText()
			if sshHost != "" {
				p.st.SetBastionSSHHost(sshHost)
			} else {
				allErrs = multierror.Append(allErrs, fmt.Errorf("Bastion SSH host cannot be empty"))
			}

			sshUser := form.GetFormItemByLabel(bastionHostUserLabel).(*tview.InputField).GetText()
			if sshUser != "" {
				p.st.SetBastionSSHUser(sshUser)
			} else {
				allErrs = multierror.Append(allErrs, fmt.Errorf("Bastion SSH user cannot be empty"))
			}
		} else {
			p.st.SetBastionSSHHost("")
			p.st.SetBastionSSHUser("")
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
