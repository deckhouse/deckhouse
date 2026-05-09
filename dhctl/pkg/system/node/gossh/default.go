// Copyright 2025 Flant JSC
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

package gossh

import (
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	genssh "github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
)

func NewClientFromFlags(ctx context.Context) (*Client, error) {
	settings := session.NewSession(session.Input{
		AvailableHosts:  sshHosts,
		User:            sshUser,
		Port:            sshPort,
		BastionHost:     sshBastionHost,
		BastionPort:     sshBastionPort,
		BastionUser:     sshBastionUser,
		BastionPassword: sshBastionPass,
		ExtraArgs:       sshExtraArgs,
	})

	return NewClient(ctx, settings, genssh.CollectDHCTLPrivateKeysFromFlags()), nil
}

func NewClientFromFlagsWithHosts(ctx context.Context) (*Client, error) {
	if len(sshHosts) == 0 {
		return nil, fmt.Errorf("Hosts not passed")
	}

	sshCl, err := NewClientFromFlags(ctx)
	return sshCl, err
}

func NewInitClientFromFlagsWithHosts(ctx context.Context, askPassword bool) (*Client, error) {
	if len(sshHosts) == 0 {
		return nil, fmt.Errorf("Hosts not passed")
	}

	return NewInitClientFromFlags(ctx, askPassword)
}

func NewInitClientFromFlags(ctx context.Context, askPassword bool) (*Client, error) {
	if len(sshHosts) == 0 {
		return nil, nil
	}

	var sshClient *Client
	var err error

	sshClient, err = NewClientFromFlags(ctx)
	if err != nil {
		return nil, err
	}

	err = sshClient.Start()
	if err != nil {
		return nil, err
	}

	if askPassword {
		err = terminal.AskBecomePassword(&pkgBecomeOptions)
		if err != nil {
			return nil, err
		}
		// keep the locally-cached become password in sync after the prompt.
		// TODO(nabokikhms): fix package level setters in the following PRs.
		becomePass = pkgBecomeOptions.BecomePass
	}

	return sshClient, nil
}
