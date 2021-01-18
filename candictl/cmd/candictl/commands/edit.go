package commands

import (
	"encoding/json"
	"fmt"

	"gopkg.in/alecthomas/kingpin.v2"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/candictl/pkg/app"
	"github.com/deckhouse/deckhouse/candictl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/candictl/pkg/log"
	"github.com/deckhouse/deckhouse/candictl/pkg/operations"
	"github.com/deckhouse/deckhouse/candictl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/candictl/pkg/util/retry"
	"github.com/deckhouse/deckhouse/candictl/pkg/util/tomb"
)

func baseEditConfigCMD(parent *kingpin.CmdClause, name, secret, dataKey string, manifest func([]byte) *apiv1.Secret) *kingpin.CmdClause {
	cmd := parent.Command(name, fmt.Sprintf("Edit %s in Kubernetes cluster.", name))
	app.DefineKubeFlags(cmd)
	app.DefineOutputFlag(cmd)
	app.DefineSSHFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineEditorConfigFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		var sshClient *ssh.Client
		var err error
		if app.SSHHost != "" {
			sshClient, err = ssh.NewClientFromFlags().Start()
			if err != nil {
				return err
			}

			err = operations.AskBecomePassword()
			if err != nil {
				return err
			}
		}

		kubeCl, err := operations.ConnectToKubernetesAPI(sshClient)
		if err != nil {
			return err
		}

		config, err := kubeCl.CoreV1().Secrets("kube-system").Get(secret, metav1.GetOptions{})
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
				return retry.StartLoop(
					fmt.Sprintf("Update %s secret", name), 5, 5, func() error {
						_, err = kubeCl.CoreV1().
							Secrets("kube-system").
							Patch(secret, types.MergePatchType, content)
						return err
					})
			})
	})

	return cmd
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
