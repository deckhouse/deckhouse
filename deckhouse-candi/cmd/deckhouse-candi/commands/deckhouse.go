package commands

import (
	"fmt"

	"github.com/flant/logboek"
	sh_app "github.com/flant/shell-operator/pkg/app"
	"gopkg.in/alecthomas/kingpin.v2"
	"sigs.k8s.io/yaml"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/deckhouse"
	"flant/deckhouse-candi/pkg/kube"
	"flant/deckhouse-candi/pkg/log"
	"flant/deckhouse-candi/pkg/ssh"
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

		err = logboek.LogProcess("☠️ Remove Deckhouse ☠️", log.TaskOptions(), func() error {
			kubeCl := kube.NewKubernetesClient().WithSshClient(sshCl)
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

		clusterConfig, err := metaConfig.MarshalClusterConfigYAML()
		if err != nil {
			return fmt.Errorf("marshal cluster config: %v", err)
		}

		providerClusterConfig, err := metaConfig.MarshalProviderClusterConfigYAML()
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

		err = logboek.LogProcess("🛥️ Create Deckhouse Deployment 🛥️", log.TaskOptions(), func() error {
			kubeCl := kube.NewKubernetesClient().WithSshClient(sshClient)
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

func DefineDeckhouseInstall(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("install", "Install Deckhouse.")
	app.DefineSshFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineConfigFlags(cmd)
	sh_app.DefineKubeClientFlags(cmd)

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

		installConfig := deckhouse.Config{
			Registry:       metaConfig.DeckhouseConfig.ImagesRepo,
			DockerCfg:      metaConfig.DeckhouseConfig.RegistryDockerCfg,
			DevBranch:      metaConfig.DeckhouseConfig.DevBranch,
			ReleaseChannel: metaConfig.DeckhouseConfig.ReleaseChannel,
			Bundle:         metaConfig.DeckhouseConfig.Bundle,
			LogLevel:       metaConfig.DeckhouseConfig.LogLevel,
		}

		err = logboek.LogProcess("🛥️ Install Deckhouse 🛥️", log.TaskOptions(), func() error {
			kubeCl := kube.NewKubernetesClient().WithSshClient(sshClient)
			if err := kubeCl.Init(""); err != nil {
				return fmt.Errorf("open kubernetes connection: %v", err)
			}

			err = deckhouse.CreateDeckhouseManifests(kubeCl, &installConfig)
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
