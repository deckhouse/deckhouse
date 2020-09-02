package commands

import (
	"fmt"

	sh_app "github.com/flant/shell-operator/pkg/app"
	"gopkg.in/alecthomas/kingpin.v2"
	"sigs.k8s.io/yaml"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/kubernetes/actions/deckhouse"
	"flant/deckhouse-candi/pkg/kubernetes/client"
	"flant/deckhouse-candi/pkg/log"
	"flant/deckhouse-candi/pkg/system/ssh"
)

func DefineDeckhouseRemoveDeployment(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("remove-deployment", "Delete deckhouse deployment.")
	app.DefineSshFlags(cmd)
	app.DefineBecomeFlags(cmd)
	sh_app.DefineKubeClientFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshCl, err := ssh.NewClientFromFlags().Start()
		if err != nil {
			return err
		}

		err = app.AskBecomePassword()
		if err != nil {
			return err
		}

		err = log.Process("default", "Remove Deckhouse️", func() error {
			kubeCl := client.NewKubernetesClient().WithSSHClient(sshCl)
			// auto init
			err = kubeCl.Init("")
			if err != nil {
				return fmt.Errorf("open kubernetes connection: %v", err)
			}

			err = deckhouse.DeleteDeckhouseDeployment(kubeCl)
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}

		return nil
	})

	return cmd
}

func DefineDeckhouseCreateDeployment(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("create-deployment", "Install deckhouse after terraform is applied successful.")
	app.DefineSshFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineConfigFlags(cmd)
	sh_app.DefineKubeClientFlags(cmd)

	var DryRun bool
	cmd.Flag("dry-run", "Output deployment yaml").
		BoolVar(&DryRun)

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshClient, err := ssh.NewClientFromFlags().Start()
		if err != nil {
			return err
		}

		err = app.AskBecomePassword()
		if err != nil {
			return err
		}

		// Load deckhouse config
		metaConfig, err := config.ParseConfig(app.ConfigPath)
		if err != nil {
			return err
		}

		clusterConfig, err := metaConfig.ClusterConfigYAML()
		if err != nil {
			return fmt.Errorf("marshal cluster config: %v", err)
		}

		providerClusterConfig, err := metaConfig.ProviderClusterConfigYAML()
		if err != nil {
			return fmt.Errorf("marshal provider config: %v", err)
		}

		installConfig := deckhouse.Config{
			Registry:              metaConfig.DeckhouseConfig.ImagesRepo,
			DockerCfg:             metaConfig.DeckhouseConfig.RegistryDockerCfg,
			DevBranch:             metaConfig.DeckhouseConfig.DevBranch,
			ReleaseChannel:        metaConfig.DeckhouseConfig.ReleaseChannel,
			Bundle:                metaConfig.DeckhouseConfig.Bundle,
			LogLevel:              metaConfig.DeckhouseConfig.LogLevel,
			ClusterConfig:         clusterConfig,
			ProviderClusterConfig: providerClusterConfig,
		}

		if DryRun {
			manifest := deckhouse.CreateDeckhouseDeploymentManifest(&installConfig)
			out, err := yaml.Marshal(manifest)
			if err != nil {
				return err
			}
			fmt.Println(string(out))
			return nil
		}

		err = log.Process("bootstrap", "Create Deckhouse Deployment", func() error {
			kubeCl := client.NewKubernetesClient().WithSSHClient(sshClient)
			if err := kubeCl.Init(""); err != nil {
				return fmt.Errorf("open kubernetes connection: %v", err)
			}

			err = deckhouse.CreateDeckhouseDeployment(kubeCl, &installConfig)
			if err != nil {
				return fmt.Errorf("deckhouse install: %v", err)
			}

			err = deckhouse.WaitForReadiness(kubeCl, &installConfig)
			if err != nil {
				return fmt.Errorf("deckhouse install: %v", err)
			}
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	return cmd
}
