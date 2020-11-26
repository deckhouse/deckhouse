package commands

import (
	"fmt"

	"gopkg.in/alecthomas/kingpin.v2"

	"flant/candictl/pkg/app"
	"flant/candictl/pkg/config"
	"flant/candictl/pkg/kubernetes/actions/converge"
	"flant/candictl/pkg/kubernetes/actions/deckhouse"
	"flant/candictl/pkg/log"
	"flant/candictl/pkg/operations"
	"flant/candictl/pkg/system/ssh"
)

func DefineConvergeCommand(kpApp *kingpin.Application) *kingpin.CmdClause {
	cmd := kpApp.Command("converge", "Converge kubernetes cluster.")
	app.DefineSSHFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineTerraformFlags(cmd)

	runFunc := func(sshClient *ssh.Client) error {
		kubeCl, err := operations.StartKubernetesAPIProxy(sshClient)
		if err != nil {
			return err
		}

		if info := deckhouse.GetClusterInfo(kubeCl); info != "" {
			_ = log.Process("common", "Cluster Info", func() error { log.InfoF(info); return nil })
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
		if err := operations.AskBecomePassword(); err != nil {
			return err
		}

		return runFunc(sshClient)
	})
	return cmd
}
