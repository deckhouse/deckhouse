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

// Mirror of dhctl/cmd/dhctl/commands/edit.go.
// Drift is enforced by tools/check-dhctl-cmd-drift.sh.

package dhctlcli

import (
	"fmt"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kpcontext"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

func connectionFlags(parent *kingpin.CmdClause, opts *options.Options) {
	app.DefineKubeFlags(parent, &opts.Kube)
	app.DefineSSHFlags(parent, &opts.SSH, nil)
	app.DefineBecomeFlags(parent, &opts.Become)
}

func baseEditConfigCMD(parent *kingpin.CmdClause, opts *options.Options, name, secret, dataKey string) *kingpin.CmdClause {
	cmd := parent.Command(name, fmt.Sprintf("Edit %s in Kubernetes cluster.", name))
	app.DefineEditorConfigFlags(cmd, &opts.Render)
	app.DefineSanityFlags(cmd, &opts.Global)

	return cmd.Action(func(c *kingpin.ParseContext) error {
		ctx := kpcontext.ExtractContext(c)

		params, err := app.DefaultProviderParams(&opts.Global)
		if err != nil {
			return err
		}
		sshProviderInitializer, kubeProvider, err := providerinitializer.GetProviders(
			ctx,
			params,
			providerinitializer.WithKubeFlagsDefined(opts.Kube.IsDefined()),
			providerinitializer.WithRequiredKubeProvider(),
		)
		if err != nil {
			return err
		}
		if kubeProvider == nil {
			return fmt.Errorf("kubernetes provider is not initialized")
		}
		if sshProviderInitializer != nil {
			defer sshProviderInitializer.Cleanup(ctx) //nolint:errcheck // best-effort cleanup, mirrors dhctl source
		}

		kube, err := kubeProvider.Client(ctx)
		if err != nil {
			return err
		}
		kubeCl := &client.KubernetesClient{KubeClient: kube}

		return operations.SecretEdit(
			ctx,
			kubeCl,
			name, "kube-system", secret, dataKey, map[string]string{
				"name": name,
			},
			opts.Global.DirConfig(),
		)
	})
}

func DefineEditCommands(parent *kingpin.CmdClause, opts *options.Options, wConnFlags bool) {
	clusterCmd := DefineEditClusterConfigurationCommand(parent, opts)
	providerCmd := DefineEditProviderClusterConfigurationCommand(parent, opts)
	staticCmd := DefineEditStaticClusterConfigurationCommand(parent, opts)

	if wConnFlags {
		connectionFlags(clusterCmd, opts)
		connectionFlags(providerCmd, opts)
		connectionFlags(staticCmd, opts)
	}
}

func DefineEditClusterConfigurationCommand(parent *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	return baseEditConfigCMD(
		parent,
		opts,
		"cluster-configuration",
		"d8-cluster-configuration",
		"cluster-configuration.yaml",
	)
}

func DefineEditProviderClusterConfigurationCommand(parent *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	return baseEditConfigCMD(
		parent,
		opts,
		"provider-cluster-configuration",
		"d8-provider-cluster-configuration",
		"cloud-provider-cluster-configuration.yaml",
	)
}

func DefineEditStaticClusterConfigurationCommand(parent *kingpin.CmdClause, opts *options.Options) *kingpin.CmdClause {
	return baseEditConfigCMD(
		parent,
		opts,
		"static-cluster-configuration",
		"d8-static-cluster-configuration",
		"static-cluster-configuration.yaml",
	)
}
