package commands

import (
	"encoding/json"
	"fmt"

	sh_app "github.com/flant/shell-operator/pkg/app"
	"gopkg.in/alecthomas/kingpin.v2"
	"sigs.k8s.io/yaml"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/commands"
	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/kubernetes/actions/converge"
	"flant/deckhouse-candi/pkg/log"
	"flant/deckhouse-candi/pkg/system/ssh"
)

func DefineTerraformConvergeExporterCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("converge-exporter", "Run terraform converge exporter.")
	sh_app.DefineKubeClientFlags(cmd)
	app.DefineConvergeExporterFlags(cmd)
	app.DefineSshFlags(cmd)
	app.DefineBecomeFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
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
		log.InfoLn("Check started ...\n")

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
