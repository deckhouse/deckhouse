package commands

import (
	"github.com/flant/logboek"
	"gopkg.in/alecthomas/kingpin.v2"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/terraform"
)

func DefineRunBaseTerraformCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("run-base-terraform", "Run base terraform and save the state.")
	app.DefineConfigFlags(cmd)
	cmd.Action(func(c *kingpin.ParseContext) error {
		metaConfig, err := config.ParseConfig(app.ConfigPath)
		if err != nil {
			return err
		}

		basePipelineResult, err := terraform.NewPipeline(
			"base_infrastructure",
			metaConfig,
			terraform.GetBasePipelineResult,
		).Run()
		if err != nil {
			return err
		}

		logboek.LogInfoF("Deckhouse Config: %s", string(basePipelineResult["deckhouseConfig"]))
		logboek.LogInfoF("Cloud Discovery Data: %s", string(basePipelineResult["cloudDiscovery"]))
		logboek.LogInfoF("Terraform State: %s", string(basePipelineResult["terraformState"]))

		return nil
	})
	return cmd
}

func DefineRunMasterTerraformCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("run-master-terraform", " Run master terraform and return the result.")
	app.DefineConfigFlags(cmd)
	cmd.Action(func(c *kingpin.ParseContext) error {
		metaConfig, err := config.ParseConfig(app.ConfigPath)
		if err != nil {
			return err
		}

		masterPipelineResult, err := terraform.NewPipeline(
			"master_node_bootstrap",
			metaConfig,
			terraform.GetMasterPipelineResult,
		).Run()
		if err != nil {
			return err
		}

		logboek.LogInfoF("Master IP: %s", masterPipelineResult["masterIP"])
		logboek.LogInfoF("Node IP: %s", masterPipelineResult["nodeIP"])
		logboek.LogInfoF("Deckhouse Config: %s", string(masterPipelineResult["deckhouseConfig"]))
		logboek.LogInfoF("Master Instance Group: %s", string(masterPipelineResult["masterInstanceClass"]))

		return nil
	})
	return cmd
}

func DefineRunDestroyAllTerraformCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("run-terraform-destroy-all", " Destroy all terraform environment.")
	app.DefineConfigFlags(cmd)
	cmd.Action(func(c *kingpin.ParseContext) error {
		logboek.LogInfoF("Destroying environment...")

		metaConfig, err := config.ParseConfig(app.ConfigPath)
		if err != nil {
			logboek.LogErrorLn(err)
		}

		stdout, err := terraform.NewRunner("master_node_bootstrap", metaConfig).Destroy(true)
		if err != nil {
			logboek.LogErrorLn(err)
		}
		logboek.LogInfoF(string(stdout))

		stdout, err = terraform.NewRunner("base_infrastructure", metaConfig).Destroy(true)
		if err != nil {
			logboek.LogErrorLn(err)
		}
		logboek.LogInfoF(string(stdout))

		return nil
	})
	return cmd
}
