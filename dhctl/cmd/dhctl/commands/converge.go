package commands

import (
	"fmt"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/converge"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
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
		sshClient, err := ssh.NewInitClientFromFlags(true)
		if err != nil {
			return err
		}

		return runFunc(sshClient)
	})
	return cmd
}
