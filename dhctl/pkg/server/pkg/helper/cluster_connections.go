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

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
)

type ClusterConnectionsOptions struct {
	CommanderMode bool

	ApiServerUrl     string
	ApiServerOptions ApiServerOptions

	SchemaStore         *config.SchemaStore
	SSHConnectionConfig string
}

func InitializeClusterConnections(ctx context.Context, opts ClusterConnectionsOptions) (*client.KubernetesClient, node.SSHClient, func() error, error) {
	if opts.CommanderMode && opts.ApiServerUrl != "" {
		kubeCl, err := CreateKubeClient(ctx, opts.ApiServerUrl, opts.ApiServerOptions)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("error creating kubernetes client: %w", err)
		}
		return kubeCl, nil, func() error { return nil }, nil
	} else {
		var sshClient node.SSHClient
		var cleanup func() error

		err := log.Process("default", "Preparing SSH client", func() error {
			connectionConfig, err := config.ParseConnectionConfig(
				opts.SSHConnectionConfig,
				opts.SchemaStore,
				config.ValidateOptionCommanderMode(opts.CommanderMode),
				config.ValidateOptionStrictUnmarshal(opts.CommanderMode),
				config.ValidateOptionValidateExtensions(opts.CommanderMode),
			)
			if err != nil {
				return fmt.Errorf("parsing connection config: %w", err)
			}

			sshClient, cleanup, err = CreateSSHClient(ctx, connectionConfig)
			if err != nil {
				return fmt.Errorf("preparing ssh client: %w", err)
			}
			return nil
		})

		return nil, sshClient, cleanup, err
	}
}
