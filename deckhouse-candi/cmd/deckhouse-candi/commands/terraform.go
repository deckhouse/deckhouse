package commands

import (
	"fmt"
	"os"

	"github.com/flant/logboek"
	"gopkg.in/alecthomas/kingpin.v2"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/deckhouse"
	"flant/deckhouse-candi/pkg/log"
	"flant/deckhouse-candi/pkg/ssh"
	"flant/deckhouse-candi/pkg/task"
	"flant/deckhouse-candi/pkg/terraform"
)

func DefineRunDestroyAllTerraformCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("destroy-all", " Destroy all terraform environment.")
	app.DefineSshFlags(cmd)
	app.DefineConfigFlags(cmd)
	app.DefineTerraformFlags(cmd)
	app.DefineSanityFlags(cmd)

	runFunc := func(sshClient *ssh.SshClient) error {
		metaConfig, err := config.ParseConfig(app.ConfigPath)
		if err != nil {
			return err
		}

		err = app.AskBecomePassword()
		if err != nil {
			return err
		}

		if err := task.WaitForSSHConnectionOnMaster(sshClient); err != nil {
			return err
		}

		kubeCl, err := task.StartKubernetesAPIProxy(sshClient)
		if err != nil {
			return err
		}

		nodesState, err := deckhouse.GetNodesStateFromCluster(kubeCl)
		if err != nil {
			return err
		}

		clusterState, err := deckhouse.GetClusterStateFromCluster(kubeCl)
		if err != nil {
			return err
		}

		for nodeGroupName, nodeGroupStates := range nodesState {
			step := "static-node"
			if nodeGroupName == "master" {
				step = "master-node"
			}

			for name, state := range nodeGroupStates {
				err := logboek.LogProcess(fmt.Sprintf("🔥 Destroy node %s 🔥", name), log.BoldOptions(), func() error {
					nodeRunner := terraform.NewRunnerFromMetaConfig(step, metaConfig).
						WithVariables(metaConfig.PrepareTerraformNodeGroupConfig(nodeGroupName, 0, "")).
						WithState(state).
						WithAutoApprove(app.SanityCheck)

					defer nodeRunner.Close()
					return terraform.DestroyPipeline(nodeRunner)
				})
				if err != nil {
					return err
				}
			}
		}

		return logboek.LogProcess("🔥 Destroy cluster infrastructure 🔥", log.BoldOptions(), func() error {
			baseRunner := terraform.NewRunnerFromMetaConfig("base-infrastructure", metaConfig).
				WithState(clusterState).
				WithAutoApprove(app.SanityCheck)

			defer baseRunner.Close()
			return terraform.DestroyPipeline(baseRunner)
		})
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		if !app.SanityCheck {
			logboek.LogWarnLn("NOTE: You will be asked for approve of every terraform destroy command.\n" +
				"If you understand what you are doing, you can use flag --yes-i-am-sane-and-i-understand-what-i-am-doing to skip approvals\n")
		}
		sshClient, err := ssh.NewClientFromFlags().Start()
		if err != nil {
			return err
		}

		err = logboek.LogProcess("💣 Run Terraform Destroy All 💣",
			log.MainProcessOptions(), func() error { return runFunc(sshClient) })

		if err != nil {
			logboek.LogErrorF("\nCritical Error: %s\n", err)
			os.Exit(1)
		}
		return nil
	})
	return cmd
}
