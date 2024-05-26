package final

import (
	"fmt"

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

func (c *SshPage) MouseEnabled() bool {
	return false
}

func (c *SshPage) Show(onNext func(), onBack func()) (tview.Primitive, []tview.Primitive) {
	outStr := fmt.Sprintf(`Please add public key to /home/%s/.ssh/authorized_keys (Use clipboard for switch page):
%s`, c.st.GetUser(), c.st.PublicSSHKey())

	// non-static cluster
	if c.st.GetProvider() != "" {
		outStr = "Save and use private key for access to control-plane node:\n" + c.st.PrivateSSHKey()
	}

	view := tview.NewTextArea().SetText(outStr, true)

	p, focusable := widget.OptionsPage("SSH key", view, onNext, onBack)

	return p, append([]tview.Primitive{view}, focusable...)
}
