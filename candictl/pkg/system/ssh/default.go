package ssh

import (
	"flant/candictl/pkg/app"
	"flant/candictl/pkg/system/ssh/session"
)

func NewClientFromFlags() *SshClient {
	return &SshClient{
		Settings: &session.Session{
			PrivateKeys: app.SshPrivateKeys,
			Host:        app.SshHost,
			User:        app.SshUser,
			Port:        app.SshPort,
			BastionHost: app.SshBastionHost,
			BastionPort: app.SshBastionPort,
			BastionUser: app.SshBastionUser,
			ExtraArgs:   app.SshExtraArgs,
		},
	}
}
