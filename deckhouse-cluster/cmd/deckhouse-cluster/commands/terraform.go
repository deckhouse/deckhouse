package commands

import (
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	"flant/deckhouse-cluster/pkg/app"
	"flant/deckhouse-cluster/pkg/config"
	"flant/deckhouse-cluster/pkg/terraform"
)

func GetRunBaseTerraformCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	return cmd.Command("run-base-terraform", "Run base terraform and save the state.").
		Action(func(c *kingpin.ParseContext) error {
			metaConfig, err := config.ParseConfig(app.ConfigPath)
			if err != nil {
				return err
			}

			basePipelineResult, err := terraform.NewPipeline("tf_base", metaConfig, terraform.GetBasePipelineResult).Run()
			if err != nil {
				return err
			}

			log.Infof("Deckhouse Config: %s", string(basePipelineResult["deckhouseConfig"]))
			log.Infof("Cloud Discovery Data: %s", string(basePipelineResult["cloudDiscovery"]))
			log.Infof("Terraform State: %s", string(basePipelineResult["terraformState"]))

			return nil
		})
}

func GetRunMasterTerraformCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	return cmd.Command("run-master-terraform", " Run master terraform and return the result.").
		Action(func(c *kingpin.ParseContext) error {
			metaConfig, err := config.ParseConfig(app.ConfigPath)
			if err != nil {
				return err
			}

			metaConfig.PrepareBootstrapSettings()

			masterPipelineResult, err := terraform.NewPipeline("tf_master", metaConfig, terraform.GetMasterPipelineResult).Run()
			if err != nil {
				return err
			}

			log.Infof("Master IP: %s", masterPipelineResult["masterIP"])
			log.Infof("Deckhouse Config: %s", string(masterPipelineResult["deckhouseConfig"]))
			log.Infof("Master Instance Group: %s", string(masterPipelineResult["masterInstanceClass"]))

			return nil
		})
}
