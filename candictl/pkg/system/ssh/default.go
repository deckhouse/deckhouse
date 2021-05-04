package ssh

import (
	"github.com/deckhouse/deckhouse/candictl/pkg/app"
	"github.com/deckhouse/deckhouse/candictl/pkg/system/ssh/session"
	"github.com/deckhouse/deckhouse/candictl/pkg/terminal"
)

func NewClientFromFlags() *Client {
	settings := session.NewSession(session.Input{
		PrivateKeys:    app.SSHPrivateKeys,
		AvailableHosts: app.SSHHosts,
		User:           app.SSHUser,
		Port:           app.SSHPort,
		BastionHost:    app.SSHBastionHost,
		BastionPort:    app.SSHBastionPort,
		BastionUser:    app.SSHBastionUser,
		ExtraArgs:      app.SSHExtraArgs,
	})

	return &Client{
		Settings: settings,
	}
}

func NewInitClientFromFlags(askPassword bool) (*Client, error) {
	if len(app.SSHHosts) == 0 {
		return nil, nil
	}

	var sshClient *Client
	var err error

	sshClient, err = NewClientFromFlags().Start()
	if err != nil {
		return nil, err
	}

	if askPassword {
		err = terminal.AskBecomePassword()
		if err != nil {
			return nil, err
		}
	}

	return sshClient, nil
}
