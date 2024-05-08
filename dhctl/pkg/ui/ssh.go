package ui

import (
	"fmt"

	"github.com/f1bonacc1/glippy"

	"github.com/rivo/tview"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/internal/widget"
)

type sshState interface {
	GetProvider() string
	GetUser() string
	PublicSSHKey() string
	PrivateSSHKey() string
}

type SshPage struct {
	st sshState
}

func NewSSHPage(st sshState) *SshPage {
	return &SshPage{
		st: st,
	}
}

func (c *SshPage) Show(onNext func(), onBack func()) (tview.Primitive, []tview.Primitive) {
	outStr := fmt.Sprintf(`Please add public key to /home/%s/.ssh/authorized_keys Public key already added to clipboard:
%s`, c.st.GetUser(), c.st.PublicSSHKey())

	glippy.Set(c.st.PublicSSHKey())

	// non-static cluster
	if c.st.GetProvider() != "" {
		outStr = "Save and use private key for access to control-plane node:\n" + c.st.PrivateSSHKey()
		glippy.Set(c.st.PrivateSSHKey())
	}

	view := tview.NewTextArea().SetText(outStr, true).
		SetClipboard(func(s string) {
			glippy.Set(s)
		}, func() string {
			s, _ := glippy.Get()
			return s
		})

	p, focusable := widget.OptionsPage("SSH key", view, onNext, onBack)

	return p, append([]tview.Primitive{view}, focusable...)
}
