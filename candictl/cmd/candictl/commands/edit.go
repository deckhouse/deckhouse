package commands

import (
	"encoding/json"
	"fmt"

	sh_app "github.com/flant/shell-operator/pkg/app"
	"gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	"flant/candictl/pkg/app"
	"flant/candictl/pkg/config"
	"flant/candictl/pkg/kubernetes/actions/manifests"
	"flant/candictl/pkg/log"
	"flant/candictl/pkg/operations"
	"flant/candictl/pkg/system/ssh"
	"flant/candictl/pkg/util/retry"
)

func DefineEditClusterConfigurationCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("cluster-configuration", "Edit cluster configuration in Kubernetes cluster.")
	sh_app.DefineKubeClientFlags(cmd)
	app.DefineOutputFlag(cmd)
	app.DefineSSHFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineEditorConfigFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		schemaStore := config.NewSchemaStore()

		sshClient, err := ssh.NewClientFromFlags().Start()
		if err != nil {
			return err
		}

		if err := operations.AskBecomePassword(); err != nil {
			return err
		}

		kubeCl, err := operations.StartKubernetesAPIProxy(sshClient)
		if err != nil {
			return err
		}

		configData, err := config.GetClusterConfigData(kubeCl, schemaStore)
		if err != nil {
			return err
		}

		configData, err = yaml.JSONToYAML(configData)
		if err != nil {
			return err
		}

		modifiedData, err := operations.Edit(configData)
		if err != nil {
			return err
		}

		doc := manifests.SecretWithClusterConfig(modifiedData)
		content, err := json.Marshal(doc)
		if err != nil {
			return err
		}

		return log.Process("common", "Save cluster configuration back to the Kubernetes cluster", func() error {
			if string(configData) == string(modifiedData) {
				log.InfoLn("Configurations are equal. Nothing to update.")
				return nil
			}
			return retry.StartLoop("Update cluster configuration secret", 5, 5, func() error {
				_, err = kubeCl.CoreV1().
					Secrets("kube-system").
					Patch("d8-cluster-configuration", types.MergePatchType, content)
				return err
			})
		})
	})
	return cmd
}

func DefineEditProviderClusterConfigurationCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("provider-cluster-configuration", "Edit provider cluster configuration in Kubernetes cluster.")
	sh_app.DefineKubeClientFlags(cmd)
	app.DefineOutputFlag(cmd)
	app.DefineSSHFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineEditorConfigFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshClient, err := ssh.NewClientFromFlags().Start()
		if err != nil {
			return err
		}

		if err := operations.AskBecomePassword(); err != nil {
			return err
		}

		kubeCl, err := operations.StartKubernetesAPIProxy(sshClient)
		if err != nil {
			return err
		}

		metaConfig, err := config.ParseConfigFromCluster(kubeCl)
		if err != nil {
			return err
		}

		if metaConfig.ClusterType != config.CloudClusterType {
			return fmt.Errorf("It is not possible to edit provider cluster configuration for Static cluster type.")
		}

		providerConfigData, err := metaConfig.ProviderClusterConfigYAML()
		if err != nil {
			return err
		}

		modifiedData, err := operations.Edit(providerConfigData)
		if err != nil {
			return err
		}

		doc := manifests.SecretWithProviderClusterConfig(modifiedData, nil)
		content, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		return log.Process("common", "Save provider cluster configuration back to the Kubernetes cluster", func() error {
			if string(providerConfigData) == string(modifiedData) {
				log.InfoLn("Configurations are equal. Nothing to update.")
				return nil
			}
			return retry.StartLoop("Update provider cluster configuration secret", 5, 5, func() error {
				_, err = kubeCl.CoreV1().
					Secrets("kube-system").
					Patch("d8-provider-cluster-configuration", types.MergePatchType, content)
				return err
			})
		})
	})
	return cmd
}
