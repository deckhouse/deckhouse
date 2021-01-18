package commands

import (
	"encoding/json"
	"fmt"

	"gopkg.in/alecthomas/kingpin.v2"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/candictl/pkg/app"
	"github.com/deckhouse/deckhouse/candictl/pkg/config"
	"github.com/deckhouse/deckhouse/candictl/pkg/kubernetes/actions/converge"
	"github.com/deckhouse/deckhouse/candictl/pkg/log"
	"github.com/deckhouse/deckhouse/candictl/pkg/operations"
	"github.com/deckhouse/deckhouse/candictl/pkg/system/ssh"
)

func DefineTerraformConvergeExporterCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("converge-exporter", "Run terraform converge exporter.")
	app.DefineKubeFlags(cmd)
	app.DefineConvergeExporterFlags(cmd)
	app.DefineSSHFlags(cmd)
	app.DefineBecomeFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		exporter := operations.NewConvergeExporter(app.ListenAddress, app.MetricsPath, app.CheckInterval)
		exporter.Start()
		return nil
	})
	return cmd
}

func DefineTerraformCheckCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("check", "Check differences between state of Kubernetes cluster and Terraform state.")
	app.DefineKubeFlags(cmd)
	app.DefineOutputFlag(cmd)
	app.DefineSSHFlags(cmd)
	app.DefineBecomeFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		log.InfoLn("Check started ...\n")

		var sshClient *ssh.Client
		var err error
		if app.SSHHost != "" {
			sshClient, err = ssh.NewClientFromFlags().Start()
			if err != nil {
				return err
			}

			if err := operations.AskBecomePassword(); err != nil {
				return err
			}
		}

		kubeCl, err := operations.ConnectToKubernetesAPI(sshClient)
		if err != nil {
			return err
		}

		metaConfig, err := config.ParseConfigInCluster(kubeCl)
		if err != nil {
			return err
		}

		metaConfig.UUID, err = converge.GetClusterUUID(kubeCl)
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

		fmt.Print(string(data))
		return nil
	})
	return cmd
}
