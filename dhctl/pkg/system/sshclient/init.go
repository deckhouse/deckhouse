// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package sshclient

import (
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/clissh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/gossh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
)

func NewInitClientFromFlags(ctx context.Context, askPassword bool) (node.SSHClient, error) {

	switch {
	case app.SSHLegacyMode:
		// if set --ssh-legacy-mode
		return clissh.NewInitClientFromFlags(askPassword)
	case app.SSHModernMode:
		// if set --ssh-modern-mode
		return gossh.NewInitClientFromFlags(ctx, askPassword)
	case len(app.SSHPrivateKeys) > 0:
		// if flags doesn't set, but we have private keys
		return clissh.NewInitClientFromFlags(askPassword)
	default:
		return gossh.NewInitClientFromFlags(ctx, askPassword)
	}

}

func NewInitClientFromFlagsWithHosts(ctx context.Context, askPassword bool) (node.SSHClient, error) {
	if len(app.SSHHosts) == 0 {
		return nil, fmt.Errorf("Hosts not passed")
	}

	return NewInitClientFromFlags(ctx, askPassword)
}

type ClienOptions struct {
	InitializeNewAgent bool
}

func NewClient(ctx context.Context, sess *session.Session, privateKeys []session.AgentPrivateKey) node.SSHClient {
	return NewClientWithOptions(ctx, sess, privateKeys, ClienOptions{})
}

func NewClientWithOptions(ctx context.Context, sess *session.Session, privateKeys []session.AgentPrivateKey, clientOptions ClienOptions) node.SSHClient {

	switch {
	case app.SSHLegacyMode:
		// if set --ssh-legacy-mode
		client := clissh.NewClient(sess, privateKeys, clientOptions.InitializeNewAgent)
		return client
	case app.SSHModernMode:
		// if set --ssh-modern-mode
		return gossh.NewClient(ctx, sess, privateKeys)
	case len(app.SSHPrivateKeys) > 0:
		// if flags doesn't set, but we have private keys
		client := clissh.NewClient(sess, privateKeys, clientOptions.InitializeNewAgent)
		return client
	default:
		return gossh.NewClient(ctx, sess, privateKeys)
	}
}

func NewClientFromFlags(ctx context.Context) (node.SSHClient, error) {
	switch {
	case app.SSHLegacyMode:
		return clissh.NewClientFromFlags(), nil
	case app.SSHModernMode:
		return gossh.NewClientFromFlags(ctx)
	case len(app.SSHPrivateKeys) > 0:
		return clissh.NewClientFromFlags(), nil
	default:
		return gossh.NewClientFromFlags(ctx)
	}
}

func NewClientFromFlagsWithHosts(ctx context.Context) (node.SSHClient, error) {
	if len(app.SSHHosts) == 0 {
		return nil, fmt.Errorf("Hosts not passed")
	}

	return NewClientFromFlags(ctx)
}

func IsModernMode() bool {
	return app.SSHModernMode || len(app.SSHPrivateKeys) == 0
}

func IsLegacyMode() bool {
	return app.SSHLegacyMode || (len(app.SSHPrivateKeys) > 0 && !app.SSHModernMode)
}
