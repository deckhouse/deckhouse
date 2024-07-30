// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ssh

import (
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
)

func NewClientFromFlags() *Client {
	settings := session.NewSession(session.Input{
		AvailableHosts: app.SSHHosts,
		User:           app.SSHUser,
		Port:           app.SSHPort,
		BastionHost:    app.SSHBastionHost,
		BastionPort:    app.SSHBastionPort,
		BastionUser:    app.SSHBastionUser,
		ExtraArgs:      app.SSHExtraArgs,
	})

	keys := make([]session.AgentPrivateKey, 0, len(app.SSHPrivateKeys))
	for _, key := range app.SSHPrivateKeys {
		keys = append(keys, session.AgentPrivateKey{Key: key})
	}

	return &Client{
		Settings:    settings,
		PrivateKeys: keys,
	}
}

func NewClientFromFlagsWithHosts() (*Client, error) {
	if len(app.SSHHosts) == 0 {
		return nil, fmt.Errorf("Hosts not passed")
	}

	return NewClientFromFlags(), nil
}

func NewInitClientFromFlagsWithHosts(askPassword bool) (*Client, error) {
	if len(app.SSHHosts) == 0 {
		return nil, fmt.Errorf("Hosts not passed")
	}

	return NewInitClientFromFlags(askPassword)
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
