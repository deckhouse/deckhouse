package ui

import (
	"fmt"

	"github.com/f1bonacc1/glippy"

	"github.com/rivo/tview"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/widget"
)

type sshState interface {
	GetProvider() string
	GetUser() string
	PublicSSHKey() string
	PrivateSSHKey() string
}

type sshPage struct {
	st     sshState
	onNext func()
	onBack func()
}

func newSSHPage(st sshState, onNext func(), onBack func()) *sshPage {
	return &sshPage{
		st:     st,
		onBack: onBack,
		onNext: onNext,
	}
}

func (c *sshPage) Show() (tview.Primitive, []tview.Primitive) {
	outStr := fmt.Sprintf(`Please add public key to /home/%s/.ssh/authorized_keys Public key already added to clipboard:
%s`, c.st.GetUser(), c.st.PublicSSHKey())

	glippy.Set(c.st.PublicSSHKey())

	// non-static cluster
	if c.st.GetProvider() != "" {
		outStr = `Save and use private key for access to control-plane node:
%s` + c.st.PrivateSSHKey()
		glippy.Set(c.st.PrivateSSHKey())
	}

	view := tview.NewTextView().SetText(outStr).SetScrollable(true).SetRegions(true)

	p, focusable := widget.OptionsPage("SSH key", view, c.onNext, c.onBack)

	return p, append([]tview.Primitive{view}, focusable...)
}
