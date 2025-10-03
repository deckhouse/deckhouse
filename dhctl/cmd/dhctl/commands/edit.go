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

package commands

import (
	"context"
	"fmt"

	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/sshclient"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
)

func connectionFlags(parent *kingpin.CmdClause) {
	app.DefineKubeFlags(parent)
	app.DefineSSHFlags(parent, config.ConnectionConfigParser{})
	app.DefineBecomeFlags(parent)
}

func baseEditConfigCMD(parent *kingpin.CmdClause, name, secret, dataKey string) *kingpin.CmdClause {
	cmd := parent.Command(name, fmt.Sprintf("Edit %s in Kubernetes cluster.", name))
	app.DefineEditorConfigFlags(cmd)
	app.DefineSanityFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		if err := terminal.AskBecomePassword(); err != nil {
			return err
		}
		if err := terminal.AskBastionPassword(); err != nil {
			return err
		}

		sshClient, err := sshclient.NewInitClientFromFlags(true)
		if err != nil {
			return err
		}

		kubeCl, err := kubernetes.ConnectToKubernetesAPI(context.Background(), ssh.NewNodeInterfaceWrapper(sshClient))
		if err != nil {
			return err
		}

		return operations.SecretEdit(kubeCl, name, "kube-system", secret, dataKey, map[string]string{
			"name": name,
		})
	})

	return cmd
}

func DefineEditCommands(parent *kingpin.CmdClause, wConnFlags bool) {
	clusterCmd := DefineEditClusterConfigurationCommand(parent)
	providerCmd := DefineEditProviderClusterConfigurationCommand(parent)
	staticCmd := DefineEditStaticClusterConfigurationCommand(parent)

	if wConnFlags {
		connectionFlags(clusterCmd)
		connectionFlags(providerCmd)
		connectionFlags(staticCmd)
	}
}

func DefineEditClusterConfigurationCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	return baseEditConfigCMD(
		parent,
		"cluster-configuration",
		"d8-cluster-configuration",
		"cluster-configuration.yaml",
	)
}

func DefineEditProviderClusterConfigurationCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	return baseEditConfigCMD(
		parent,
		"provider-cluster-configuration",
		"d8-provider-cluster-configuration",
		"cloud-provider-cluster-configuration.yaml",
	)
}

func DefineEditStaticClusterConfigurationCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	return baseEditConfigCMD(
		parent,
		"static-cluster-configuration",
		"d8-static-cluster-configuration",
		"static-cluster-configuration.yaml",
	)
}
