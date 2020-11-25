package ssh

import (
	"flant/candictl/pkg/app"
	"flant/candictl/pkg/system/ssh/session"
)

func NewClientFromFlags() *Client {
	return &Client{
		Settings: &session.Session{
			PrivateKeys: app.SSHPrivateKeys,
			Host:        app.SSHHost,
			User:        app.SSHUser,
			Port:        app.SSHPort,
			BastionHost: app.SSHBastionHost,
			BastionPort: app.SSHBastionPort,
			BastionUser: app.SSHBastionUser,
			ExtraArgs:   app.SSHExtraArgs,
		},
	}
}
