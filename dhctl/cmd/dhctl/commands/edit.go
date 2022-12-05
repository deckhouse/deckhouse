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
	"encoding/json"
	"fmt"
	"time"

	"gopkg.in/alecthomas/kingpin.v2"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

func connectionFlags(parent *kingpin.CmdClause) {
	app.DefineKubeFlags(parent)
	app.DefineSSHFlags(parent)
	app.DefineBecomeFlags(parent)
}

func baseEditConfigCMD(parent *kingpin.CmdClause, name, secret, dataKey string, manifest func([]byte) *apiv1.Secret) *kingpin.CmdClause {
	cmd := parent.Command(name, fmt.Sprintf("Edit %s in Kubernetes cluster.", name))
	app.DefineEditorConfigFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshClient, err := ssh.NewInitClientFromFlags(true)
		if err != nil {
			return err
		}

		kubeCl, err := operations.ConnectToKubernetesAPI(sshClient)
		if err != nil {
			return err
		}

		config, err := kubeCl.CoreV1().Secrets("kube-system").Get(context.TODO(), secret, metav1.GetOptions{})
		if err != nil {
			return err
		}

		configData := config.Data[dataKey]

		var modifiedData []byte
		tomb.WithoutInterruptions(func() { modifiedData, err = operations.Edit(configData) })
		if err != nil {
			return err
		}

		doc := manifest(modifiedData)
		content, err := json.Marshal(doc)
		if err != nil {
			return err
		}

		return log.Process(
			"common",
			fmt.Sprintf("Save %s back to the Kubernetes cluster", name), func() error {
				if string(configData) == string(modifiedData) {
					log.InfoLn("Configurations are equal. Nothing to update.")
					return nil
				}
				return retry.NewLoop(
					fmt.Sprintf("Update %s secret", name), 5, 5*time.Second).Run(func() error {
					_, err = kubeCl.CoreV1().
						Secrets("kube-system").
						Patch(context.TODO(), secret, types.MergePatchType, content, metav1.PatchOptions{})
					return err
				})
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
		manifests.SecretWithClusterConfig,
	)
}

func DefineEditProviderClusterConfigurationCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	return baseEditConfigCMD(
		parent,
		"provider-cluster-configuration",
		"d8-provider-cluster-configuration",
		"cloud-provider-cluster-configuration.yaml",
		func(data []byte) *apiv1.Secret {
			return manifests.SecretWithProviderClusterConfig(data, nil)
		},
	)
}

func DefineEditStaticClusterConfigurationCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	return baseEditConfigCMD(
		parent,
		"static-cluster-configuration",
		"d8-static-cluster-configuration",
		"static-cluster-configuration.yaml",
		manifests.SecretWithStaticClusterConfig,
	)
}
