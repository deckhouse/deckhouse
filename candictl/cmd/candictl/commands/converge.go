package commands

import (
	"fmt"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"

	"flant/candictl/pkg/app"
	"flant/candictl/pkg/commands"
	"flant/candictl/pkg/config"
	"flant/candictl/pkg/kubernetes/actions/converge"
	"flant/candictl/pkg/log"
	"flant/candictl/pkg/system/ssh"
)

func DefineConvergeCommand(kpApp *kingpin.Application) *kingpin.CmdClause {
	cmd := kpApp.Command("converge", "Converge kubernetes cluster.")
	app.DefineSshFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineTerraformFlags(cmd)

	runFunc := func(sshClient *ssh.SshClient) error {
		kubeCl, err := commands.StartKubernetesAPIProxy(sshClient)
		if err != nil {
			return err
		}

		metaConfig, err := config.ParseConfigFromCluster(kubeCl)
		if err != nil {
			return err
		}

		metaConfig.UUID, err = converge.GetClusterUUID(kubeCl)
		if err != nil {
			return err
		}

		err = converge.RunConverge(kubeCl, metaConfig)
		if err != nil {
			return fmt.Errorf("converge problem: %v", err)
		}

		return nil
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshClient, err := ssh.NewClientFromFlags().Start()
		if err != nil {
			return err
		}
		if err := app.AskBecomePassword(); err != nil {
			return err
		}

		err = runFunc(sshClient)
		if err != nil {
			log.ErrorLn(err.Error())
			os.Exit(1)
		}
		return nil
	})
	return cmd
}
