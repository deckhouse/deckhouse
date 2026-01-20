// Copyright 2024 Flant JSC
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

package helper

import (
	"context"
	"fmt"
	"strings"

	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/util"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/pkg/util/callback"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/sshclient"
)

func CreateSSHClient(ctx context.Context, config *config.ConnectionConfig) (node.SSHClient, func() error, error) {
	cleanuper := callback.NewCallback()

	keysPaths := make([]string, 0, len(config.SSHConfig.SSHAgentPrivateKeys))
	for _, key := range config.SSHConfig.SSHAgentPrivateKeys {
		keyPath, cleanup, err := util.WriteDefaultTempFile([]byte(strings.TrimSpace(key.Key) + "\n"))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to write ssh private key: %w", err)
		}
		cleanuper.Add(cleanup)
		keysPaths = append(keysPaths, keyPath)
	}
	normalizedKeysPaths, err := app.ParseSSHPrivateKeyPaths(keysPaths)
	if err != nil {
		return nil, cleanuper.AsFunc(), fmt.Errorf("error parsing ssh agent private keys %v: %w", normalizedKeysPaths, err)
	}
	keys := make([]session.AgentPrivateKey, 0, len(normalizedKeysPaths))
	for i, key := range normalizedKeysPaths {
		keys = append(keys, session.AgentPrivateKey{
			Key:        key,
			Passphrase: config.SSHConfig.SSHAgentPrivateKeys[i].Passphrase,
		})
	}

	var sshHosts []session.Host
	if len(config.SSHHosts) > 0 {
		for _, h := range config.SSHHosts {
			sshHosts = append(sshHosts, session.Host{Host: h.Host, Name: h.Host})
		}
	} else {
		mastersIPs, err := state.GetMasterHostsIPs(cache.Global())
		if err != nil {
			return nil, cleanuper.AsFunc(), err
		}
		if len(mastersIPs) > 0 {
			sshHosts = mastersIPs
		}
	}

	sess := session.NewSession(session.Input{
		User:           config.SSHConfig.SSHUser,
		Port:           util.PortToString(config.SSHConfig.SSHPort),
		BastionHost:    config.SSHConfig.SSHBastionHost,
		BastionPort:    util.PortToString(config.SSHConfig.SSHBastionPort),
		BastionUser:    config.SSHConfig.SSHBastionUser,
		ExtraArgs:      config.SSHConfig.SSHExtraArgs,
		AvailableHosts: sshHosts,
	})

	app.SSHPrivateKeys = keysPaths
	app.SSHBastionHost = config.SSHConfig.SSHBastionHost
	app.SSHBastionPort = util.PortToString(config.SSHConfig.SSHBastionPort)
	app.SSHBastionUser = config.SSHConfig.SSHBastionUser
	app.SSHUser = config.SSHConfig.SSHUser
	app.BecomePass = config.SSHConfig.SudoPassword
	app.SSHHosts = sshHosts
	app.SSHPort = util.PortToString(config.SSHConfig.SSHPort)
	app.SSHExtraArgs = config.SSHConfig.SSHExtraArgs
	app.SSHLegacyMode = config.SSHConfig.LegacyMode
	app.SSHModernMode = config.SSHConfig.ModernMode

	sshClient := sshclient.NewClient(ctx, sess, keys)

	cleanuper.Add(func() error {
		if !govalue.IsNil(sshClient) {
			sshClient.Stop()
		}
		return nil
	})

	return sshClient, cleanuper.AsFunc(), nil
}
