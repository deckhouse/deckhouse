package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/flant/logboek"
	sh_app "github.com/flant/shell-operator/pkg/app"
	"gopkg.in/alecthomas/kingpin.v2"
	"sigs.k8s.io/yaml"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/commands"
	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/kubernetes/actions/converge"
	"flant/deckhouse-candi/pkg/system/ssh"
	"flant/deckhouse-candi/pkg/terraform"
)

func DefineRunDestroyAllTerraformCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("destroy-all", " Destroy all terraform environment.")
	app.DefineSshFlags(cmd)
	app.DefineTerraformFlags(cmd)
	app.DefineSanityFlags(cmd)

	runFunc := func(sshClient *ssh.SshClient) error {
		err := app.AskBecomePassword()
		if err != nil {
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

		nodesState, err := converge.GetNodesStateFromCluster(kubeCl)
		if err != nil {
			return err
		}

		clusterState, err := converge.GetClusterStateFromCluster(kubeCl)
		if err != nil {
			return err
		}

		for nodeGroupName, nodeGroupStates := range nodesState {
			step := "static-node"
			if nodeGroupName == "master" {
				step = "master-node"
			}

			for name, state := range nodeGroupStates {
				nodeRunner := terraform.NewRunnerFromConfig(metaConfig, step).
					WithVariables(metaConfig.PrepareTerraformNodeGroupConfig(nodeGroupName, 0, "")).
					WithState(state).
					WithAutoApprove(app.SanityCheck)

				err := terraform.DestroyPipeline(nodeRunner, fmt.Sprintf("Node %s", name))
				if err != nil {
					return err
				}

				nodeRunner.Close()
			}
		}

		baseRunner := terraform.NewRunnerFromConfig(metaConfig, "base-infrastructure").
			WithVariables(metaConfig.MarshalConfig()).
			WithState(clusterState).
			WithAutoApprove(app.SanityCheck)

		defer baseRunner.Close()
		return terraform.DestroyPipeline(baseRunner, "Kubernetes cluster")
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

		err = runFunc(sshClient)
		if err != nil {
			logboek.LogErrorLn(err.Error())
			os.Exit(1)
		}
		return nil
	})
	return cmd
}

func DefineTerraformConvergeExporterCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("converge-exporter", "Run terraform converge exporter.")
	sh_app.DefineKubeClientFlags(cmd)
	app.DefineConvergeExporterFlags(cmd)
	app.DefineSshFlags(cmd)
	app.DefineBecomeFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		logboek.SetLevel(logboek.Error)

		exporter := commands.NewConvergeExporter(app.ListenAddress, app.MetricsPath, app.CheckInterval)
		exporter.Start()
		return nil
	})
	return cmd
}

func DefineTerraformCheckCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("check", "Check differences between state of Kubernetes cluster and Terraform state.")
	sh_app.DefineKubeClientFlags(cmd)
	app.DefineOutputFlag(cmd)
	app.DefineSshFlags(cmd)
	app.DefineBecomeFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		logboek.SetLevel(logboek.Error)
		logboek.LogInfoLn("Check started...\n")

		sshClient, err := ssh.NewClientFromFlags().Start()
		if err != nil {
			return err
		}

		kubeCl, err := commands.StartKubernetesAPIProxy(sshClient)
		if err != nil {
			return err
		}

		metaConfig, err := config.ParseConfigInCluster(kubeCl)
		if err != nil {
			return err
		}

		metaConfig.Prepare()

		statistic, err := converge.CheckState(kubeCl, metaConfig)
		if err != nil {
			return err
		}

		var data []byte
		switch app.OutputFormat {
		case "yaml":
			data, err = yaml.Marshal(statistic)
			if err != nil {
				return err
			}
		case "json":
			data, err = json.Marshal(statistic)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("Unknown output format %s", app.OutputFormat)
		}

		fmt.Println(string(data))
		return nil
	})
	return cmd
}
