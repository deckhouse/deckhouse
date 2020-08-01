package commands

import (
	"fmt"

	"github.com/flant/logboek"

	"gopkg.in/alecthomas/kingpin.v2"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/deckhouse"
	"flant/deckhouse-candi/pkg/log"
	"flant/deckhouse-candi/pkg/ssh"
	"flant/deckhouse-candi/pkg/task"
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
		return logboek.LogProcess("🐟 ~ Start Deckhouse cluster Converge", log.MainProcessOptions(), func() error {
			if err := task.WaitForSSHConnectionOnMaster(sshClient); err != nil {
				return err
			}

			kubeCl, err := task.StartKubernetesAPIProxy(sshClient)
			if err != nil {
				return err
			}

			metaConfig, err := config.ParseConfigFromCluster(kubeCl)
			if err != nil {
				return err
			}

			metaConfig.Prepare()

			err = deckhouse.RunConverge(kubeCl, metaConfig)
			if err != nil {
				return fmt.Errorf("converge problem: %v", err)
			}

			return nil
		})
	})
	return cmd
}
