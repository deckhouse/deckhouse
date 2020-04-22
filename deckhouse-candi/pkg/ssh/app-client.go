package ssh

import (
	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/ssh/session"
)

func NewClientFromFlags() *SshClient {
	return &SshClient{
		Session: &session.Session{
			PrivateKeys: app.SshPrivateKeys,
			Host:        app.SshHost,
			User:        app.SshUser,
			Port:        app.SshPort,
			BastionHost: app.SshBastionHost,
			BastionUser: app.SshBastionUser,
			ExtraArgs:   app.SshExtraArgs,
		},
	}
}
