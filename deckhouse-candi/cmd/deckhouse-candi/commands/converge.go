package commands

import (
	"fmt"

	"github.com/flant/logboek"

	"gopkg.in/alecthomas/kingpin.v2"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/commands"
	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/kubernetes/actions/converge"
	"flant/deckhouse-candi/pkg/log"
	"flant/deckhouse-candi/pkg/system/ssh"
)

func DefineConvergeCommand(kpApp *kingpin.Application) *kingpin.CmdClause {
	cmd := kpApp.Command("converge", "Converge kubernetes cluster.")
	app.DefineSshFlags(cmd)
	app.DefineBecomeFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshClient, err := ssh.NewClientFromFlags().Start()
		if err != nil {
			return err
		}
		return logboek.LogProcess("⛵ ~ Converge: Start converge ", log.MainProcessOptions(), func() error {
			if err := commands.WaitForSSHConnectionOnMaster(sshClient); err != nil {
				return err
			}

			kubeCl, err := commands.StartKubernetesAPIProxy(sshClient)
			if err != nil {
				return err
			}

			metaConfig, err := config.ParseConfigFromCluster(kubeCl)
			if err != nil {
				return err
			}

			metaConfig.Prepare()

			err = converge.RunConverge(kubeCl, metaConfig)
			if err != nil {
				return fmt.Errorf("converge problem: %v", err)
			}

			return nil
		})
	})
	return cmd
}
