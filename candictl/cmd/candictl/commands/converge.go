package commands

import (
	"fmt"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/candictl/pkg/app"
	"github.com/deckhouse/deckhouse/candictl/pkg/config"
	"github.com/deckhouse/deckhouse/candictl/pkg/kubernetes/actions/converge"
	"github.com/deckhouse/deckhouse/candictl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/candictl/pkg/log"
	"github.com/deckhouse/deckhouse/candictl/pkg/operations"
	"github.com/deckhouse/deckhouse/candictl/pkg/system/ssh"
)

func DefineConvergeCommand(kpApp *kingpin.Application) *kingpin.CmdClause {
	cmd := kpApp.Command("converge", "Converge kubernetes cluster.")
	app.DefineSSHFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineKubeFlags(cmd)

	runFunc := func(sshClient *ssh.Client) error {
		kubeCl, err := operations.ConnectToKubernetesAPI(sshClient)
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

		return runFunc(sshClient)
	})
	return cmd
}
