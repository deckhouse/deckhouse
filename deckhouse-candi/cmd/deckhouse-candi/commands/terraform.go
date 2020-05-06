package commands

import (
	"bytes"
	"encoding/json"
	"os"

	"github.com/flant/logboek"
	"gopkg.in/alecthomas/kingpin.v2"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/log"
	"flant/deckhouse-candi/pkg/terraform"
)

func prettyPrintJSON(jsonData []byte) string {
	var data bytes.Buffer
	_ = json.Indent(&data, jsonData, "", "  ")
	return data.String()
}

func DefineRunBaseTerraformCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("base-infrastructure", "Run base terraform and save the state.")
	app.DefineConfigFlags(cmd)
	app.DefineTerraformFlags(cmd)

	runFunc := func() error {
		metaConfig, err := config.ParseConfig(app.ConfigPath)
		if err != nil {
			return err
		}

		basePipelineResult, err := terraform.NewPipeline(
			"base-infrastructure",
			app.TerraformStateDir,
			metaConfig,
			terraform.GetBasePipelineResult,
		).Run()
		if err != nil {
			return err
		}

		logboek.LogInfoF("Deckhouse Config: %s\n", prettyPrintJSON(basePipelineResult["deckhouseConfig"]))
		logboek.LogInfoF("Cloud Discovery Data: %s\n", prettyPrintJSON(basePipelineResult["cloudDiscovery"]))

		return nil
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		err := logboek.LogProcess("🌱 Run Terraform Base 🌱",
			log.MainProcessOptions(), func() error { return runFunc() })

		if err != nil {
			logboek.LogErrorF("\nCritical Error: %s\n", err)
			os.Exit(1)
		}
		return nil
	})

	return cmd
}

func DefineRunMasterTerraformCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("master-node-bootstrap", " Run master terraform and return the result.")
	app.DefineConfigFlags(cmd)
	app.DefineTerraformFlags(cmd)

	runFunc := func() error {
		metaConfig, err := config.ParseConfig(app.ConfigPath)
		if err != nil {
			return err
		}

		masterPipelineResult, err := terraform.NewPipeline(
			"master-node-bootstrap",
			app.TerraformStateDir,
			metaConfig,
			terraform.GetMasterPipelineResult,
		).Run()
		if err != nil {
			return err
		}

		logboek.LogInfoF("Master IP: %s\n", string(masterPipelineResult["masterIP"]))
		logboek.LogInfoF("Node IP: %s\n", string(masterPipelineResult["nodeIP"]))
		logboek.LogInfoF("Deckhouse Config: %s\n", prettyPrintJSON(masterPipelineResult["deckhouseConfig"]))
		logboek.LogInfoF("Master Instance Group: %s\n", prettyPrintJSON(masterPipelineResult["masterInstanceClass"]))

		return nil
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		err := logboek.LogProcess("🌱 Run Terraform Master Bootstrap 🌱",
			log.MainProcessOptions(), func() error { return runFunc() })

		if err != nil {
			logboek.LogErrorF("\nCritical Error: %s\n", err)
			os.Exit(1)
		}
		return nil
	})
	return cmd
}

func DefineRunDestroyAllTerraformCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("destroy-all", " Destroy all terraform environment.")
	app.DefineConfigFlags(cmd)
	app.DefineTerraformFlags(cmd)

	runFunc := func() error {
		metaConfig, err := config.ParseConfig(app.ConfigPath)
		if err != nil {
			logboek.LogErrorLn(err)
		}

		var masterState string
		err = logboek.LogProcess("Run Destroy for master-node-bootstrap", log.BoldOptions(), func() error {
			masterRunner := terraform.NewRunner("master-node-bootstrap", metaConfig)
			masterRunner.WithStateDir(app.TerraformStateDir)
			stdout, err := masterRunner.Init(false)
			if err != nil {
				logboek.LogInfoF(string(stdout))
				return err
			}

			stdout, err = masterRunner.Destroy(true)
			if err != nil {
				logboek.LogInfoF(string(stdout))
				return err
			}
			masterState = masterRunner.State
			return nil
		})
		if err != nil {
			logboek.LogErrorLn(err)
		}

		var baseState string
		err = logboek.LogProcess("Run Destroy for base-infrastructure", log.BoldOptions(), func() error {
			baseRunner := terraform.NewRunner("base-infrastructure", metaConfig)
			baseRunner.WithStateDir(app.TerraformStateDir)
			stdout, err := baseRunner.Init(false)
			if err != nil {
				logboek.LogInfoF(string(stdout))
				return err
			}

			stdout, err = baseRunner.Destroy(true)
			if err != nil {
				logboek.LogInfoF(string(stdout))
				return err
			}
			baseState = baseRunner.State
			return nil
		})
		if err != nil {
			logboek.LogErrorLn(err)
		}

		_ = os.Remove(masterState)
		_ = os.Remove(baseState)

		return nil
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		err := logboek.LogProcess("💣 Run Terraform Destroy All 💣",
			log.MainProcessOptions(), func() error { return runFunc() })

		if err != nil {
			logboek.LogErrorF("\nCritical Error: %s\n", err)
			os.Exit(1)
		}
		return nil
	})
	return cmd
}
