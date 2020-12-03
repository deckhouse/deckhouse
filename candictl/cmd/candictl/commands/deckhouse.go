package commands

import (
	"fmt"

	"gopkg.in/alecthomas/kingpin.v2"
	"sigs.k8s.io/yaml"

	"flant/candictl/pkg/app"
	"flant/candictl/pkg/config"
	"flant/candictl/pkg/kubernetes/actions/deckhouse"
	"flant/candictl/pkg/kubernetes/client"
	"flant/candictl/pkg/log"
	"flant/candictl/pkg/operations"
	"flant/candictl/pkg/system/ssh"
)

func DefineDeckhouseRemoveDeployment(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("remove-deployment", "Delete deckhouse deployment.")
	app.DefineSSHFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineKubeFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshClient := ssh.NewClientFromFlags()
		if sshClient != nil {
			if _, err := sshClient.Start(); err != nil {
				return err
			}
		}

		err := operations.AskBecomePassword()
		if err != nil {
			return err
		}

		err = log.Process("default", "Remove Deckhouse️", func() error {
			kubeCl := client.NewKubernetesClient().WithSSHClient(sshClient)
			// auto init
			err = kubeCl.Init()
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
	app.DefineSSHFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineConfigFlags(cmd)
	app.DefineKubeFlags(cmd)

	var DryRun bool
	cmd.Flag("dry-run", "Output deployment yaml").
		BoolVar(&DryRun)

	cmd.Action(func(c *kingpin.ParseContext) error {
		// Load deckhouse config
		metaConfig, err := config.ParseConfig(app.ConfigPath)
		if err != nil {
			return err
		}

		var sshClient *ssh.Client
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

		installConfig, err := deckhouse.PrepareDeckhouseInstallConfig(metaConfig)
		if err != nil {
			return err
		}

		if DryRun {
			manifest := deckhouse.CreateDeckhouseDeploymentManifest(installConfig)
			out, err := yaml.Marshal(manifest)
			if err != nil {
				return err
			}

			fmt.Println(string(out))
			return nil
		}

		err = log.Process("bootstrap", "Create Deckhouse Deployment", func() error {
			kubeCl := client.NewKubernetesClient().WithSSHClient(sshClient)
			if err := kubeCl.Init(); err != nil {
				return fmt.Errorf("open kubernetes connection: %v", err)
			}

			err = deckhouse.CreateDeckhouseDeployment(kubeCl, installConfig)
			if err != nil {
				return fmt.Errorf("deckhouse install: %v", err)
			}

			err = deckhouse.WaitForReadiness(kubeCl)
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
