package ui

import (
	"fmt"
	"net"

	"github.com/gdamore/tcell/v2"
	"github.com/hashicorp/go-multierror"
	"github.com/rivo/tview"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/widget"
)

type staticMasterState interface {
	SetSSHUser(string)
	SetSSHHost(string)
	SetInternalNetworkCIDR(string)
	SetUsePasswordForSudo(bool)

	SetBastionSSHUser(string)
	SetBastionSSHHost(string)
}

func newStaticMasterPage(st staticMasterState, onNext func(), onBack func()) (tview.Primitive, []tview.Primitive) {
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
	form.AddInputField(sshHostLabel, "", constInputsWidth, nil, func(text string) {
		st.SetSSHHost(text)
	})

	form.AddInputField(sshUserLabel, "ubuntu", constInputsWidth, nil, func(text string) {
		st.SetSSHUser(text)
	})

	form.AddCheckbox(askSudoPasswordLabel, false, func(check bool) {
		st.SetUsePasswordForSudo(check)
	})

	form.AddInputField(internalNetworkLabel, "", constInputsWidth, nil, func(text string) {
		st.SetInternalNetworkCIDR(text)
	})

	form.AddCheckbox(useBastionHostLabel, false, func(check bool) {
		if check {
			if form.GetFormItemIndex(bastionHostLabel) < 0 {
				form.AddInputField(bastionHostLabel, "", constInputsWidth, nil, func(text string) {
					st.SetBastionSSHHost(text)
				})
			}

			if form.GetFormItemIndex(bastionHostUserLabel) < 0 {
				form.AddInputField(bastionHostUserLabel, "ubuntu", constInputsWidth, nil, func(text string) {
					st.SetBastionSSHUser(text)
				})
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
		//SetBorders(true).
		SetColumns(0).SetRows(0, 2).
		AddItem(form, 0, 0, 1, 1, 0, 0, true).
		AddItem(errorLbl, 1, 0, 1, 1, 0, 0, false)

	p, focusable := widget.OptionsPage("First control-plane node settings", optionsGrid, func() {
		var allErrs *multierror.Error

		sshHost := form.GetFormItemByLabel(sshHostLabel).(*tview.InputField).GetText()
		if sshHost != "" {
			st.SetSSHHost(sshHost)
		} else {
			allErrs = multierror.Append(allErrs, fmt.Errorf("SSH host cannot be empty"))
		}

		sshUser := form.GetFormItemByLabel(sshUserLabel).(*tview.InputField).GetText()
		if sshUser != "" {
			st.SetSSHUser(sshUser)
		} else {
			allErrs = multierror.Append(allErrs, fmt.Errorf("SSH user cannot be empty"))
		}

		sshInternalNetwork := form.GetFormItemByLabel(internalNetworkLabel).(*tview.InputField).GetText()
		if sshInternalNetwork != "" {
			if _, _, err := net.ParseCIDR(sshInternalNetwork); err != nil {
				allErrs = multierror.Append(allErrs, fmt.Errorf("Incorrect network CIDR"))
			} else {
				st.SetInternalNetworkCIDR(sshUser)
			}
		} else {
			allErrs = multierror.Append(allErrs, fmt.Errorf("Internal network CIDR user cannot be empty"))
		}

		askSudoPassword := form.GetFormItemByLabel(askSudoPasswordLabel).(*tview.Checkbox).IsChecked()
		st.SetUsePasswordForSudo(askSudoPassword)

		useBastion := form.GetFormItemByLabel(useBastionHostLabel).(*tview.Checkbox).IsChecked()
		if useBastion {
			sshHost := form.GetFormItemByLabel(bastionHostLabel).(*tview.InputField).GetText()
			if sshHost != "" {
				st.SetBastionSSHHost(sshHost)
			} else {
				allErrs = multierror.Append(allErrs, fmt.Errorf("Bastion SSH host cannot be empty"))
			}

			sshUser := form.GetFormItemByLabel(bastionHostUserLabel).(*tview.InputField).GetText()
			if sshUser != "" {
				st.SetBastionSSHUser(sshUser)
			} else {
				allErrs = multierror.Append(allErrs, fmt.Errorf("Bastion SSH user cannot be empty"))
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
