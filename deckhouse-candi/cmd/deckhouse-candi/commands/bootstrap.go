package commands

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/flant/logboek"
	"github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/deckhouse"
	"flant/deckhouse-candi/pkg/kube"
	"flant/deckhouse-candi/pkg/log"
	"flant/deckhouse-candi/pkg/ssh"
	"flant/deckhouse-candi/pkg/template"
	"flant/deckhouse-candi/pkg/terraform"
)

const banner = `
========================================================================================
 _____             _     _                                ______                _ _____
(____ \           | |   | |                              / _____)              | (_____)
 _   \ \ ____ ____| |  _| | _   ___  _   _  ___  ____   | /      ____ ____   _ | |  _
| |   | / _  ) ___) | / ) || \ / _ \| | | |/___)/ _  )  | |     / _  |  _ \ / || | | |
| |__/ ( (/ ( (___| |< (| | | | |_| | |_| |___ ( (/ /   | \____( ( | | | | ( (_| |_| |_
|_____/ \____)____)_| \_)_| |_|\___/ \____(___/ \____)   \______)_||_|_| |_|\____(_____)
========================================================================================
`

func DefineBootstrapCommand(kpApp *kingpin.Application) *kingpin.CmdClause {
	cmd := kpApp.Command("bootstrap", "Bootstrap cluster.")
	app.DefineSshFlags(cmd)
	app.DefineConfigFlags(cmd)
	app.DefineBecomeFlags(cmd)

	// Mute Shell-Operator logs
	logrus.SetLevel(logrus.PanicLevel)

	runFunc := func(sshClient *ssh.SshClient) error {
		metaConfig, err := config.ParseConfig(app.ConfigPath)
		if err != nil {
			return err
		}

		clusterConfig, err := metaConfig.MarshalClusterConfigYAML()
		if err != nil {
			return fmt.Errorf("marshal cluster config: %v", err)
		}

		providerClusterConfig, err := metaConfig.MarshalProviderClusterConfigYAML()
		if err != nil {
			return fmt.Errorf("marshal provider config: %v", err)
		}

		installConfig := deckhouse.Config{
			Registry:              metaConfig.DeckhouseConfig.ImagesRepo,
			DockerCfg:             metaConfig.DeckhouseConfig.RegistryDockerCfg,
			DevBranch:             metaConfig.DeckhouseConfig.DevBranch,
			ReleaseChannel:        metaConfig.DeckhouseConfig.ReleaseChannel,
			Bundle:                metaConfig.DeckhouseConfig.Bundle,
			LogLevel:              metaConfig.DeckhouseConfig.LogLevel,
			ClusterConfig:         clusterConfig,
			ProviderClusterConfig: providerClusterConfig,
		}

		var nodeIP string
		err = logboek.LogProcess("🌱 Run Terraform 🌱", log.TaskOptions(), func() error {
			if metaConfig.ClusterType == "Cloud" {
				basePipelineResult, err := terraform.NewPipeline(
					"base_infrastructure",
					metaConfig,
					terraform.GetBasePipelineResult,
				).Run()
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

				installConfig.DeckhouseConfig = metaConfig.MergeDeckhouseConfig(
					basePipelineResult["deckhouseConfig"],
					masterPipelineResult["deckhouseConfig"],
				)
				installConfig.CloudDiscovery = basePipelineResult["cloudDiscovery"]
				installConfig.TerraformState = basePipelineResult["terraformState"]

				_ = json.Unmarshal(masterPipelineResult["masterIP"], &app.SshHost)
				_ = json.Unmarshal(masterPipelineResult["nodeIP"], &nodeIP)

				sshClient.Session.Host = app.SshHost

				logboek.LogInfoF("Master IP: %s", masterPipelineResult["masterIP"])
			} else {
				installConfig.DeckhouseConfig = metaConfig.MergeDeckhouseConfig()
			}
			return nil
		})
		if err != nil {
			return err
		}
		// Generate bashible bundle

		// wait for ssh connection to master
		err = logboek.LogProcess("🚝 Establish SSH connection 🚝", log.TaskOptions(), func() error {
			err = sshClient.Check().AwaitAvailability()
			if err != nil {
				return fmt.Errorf("await master available: %v", err)
			}
			return nil
		})
		if err != nil {
			return err
		}

		var bundleName string
		err = logboek.LogProcess("🔍 Detect Bashible Bundle 🔍", log.TaskOptions(), func() error {
			// run detect bundle type
			detectCmd := sshClient.UploadScript("/deckhouse/candi/bashible/detect_bundle.sh")
			stdout, err := detectCmd.Execute()
			if err != nil {
				if ee, ok := err.(*exec.ExitError); ok {
					return fmt.Errorf("script '%s' error: %v\nstderr: %s", "detect_bundle.sh", err, string(ee.Stderr))
				}
				return fmt.Errorf("script '%s' error: %v", "detect_bundle.sh", err)
			}

			bundleName = strings.Trim(string(stdout), "\n ")
			logboek.LogInfoF("\nDetected bundle: %s\n\n", bundleName)

			return nil
		})
		if err != nil {
			return err
		}

		// Generate bootstrap scripts
		templateController := template.NewTemplateController("")
		logboek.LogInfoF("\nTemplates Dir: %q\n\n", templateController.TmpDir)

		err = logboek.LogProcess("🔨 Run Master Bootstrap 🔨", log.TaskOptions(), func() error {
			err = template.PrepareBootstrap(templateController, nodeIP, bundleName, metaConfig)
			for _, bootstrapScript := range []string{"bootstrap.sh", "bootstrap-networks.sh"} {
				logboek.LogInfoF("Execute bootstrap %s", bootstrapScript)

				cmd := sshClient.UploadScript(templateController.TmpDir + "/bootstrap/" + bootstrapScript).Sudo()

				stdout, err := cmd.Execute()
				if err != nil {
					if ee, ok := err.(*exec.ExitError); ok {
						return fmt.Errorf("script 'bootstrap/%s' error: %v\nstderr: %s", bootstrapScript, err, string(ee.Stderr))
					}
					return fmt.Errorf("script 'bootstrap/%s' error: %v", bootstrapScript, err)
				}

				if len(stdout) > 0 {
					logboek.LogInfoF("bootstrap/%s stdout: %v\n", bootstrapScript, string(stdout))
				}
			}
			return nil
		})
		// defer templateController.Close()

		err = logboek.LogProcess("📦 Prepare Bashible Bundle 📦", log.TaskOptions(), func() error {
			return template.PrepareBundle(templateController, nodeIP, bundleName, metaConfig)
		})
		if err != nil {
			return err
		}

		err = logboek.LogProcess("🚁 Run Bashible Bundle 🚁", log.TaskOptions(), func() error {
			bundleCmd := sshClient.UploadScript("bashible.sh", "--local").Sudo()
			parentDir := templateController.TmpDir + "/var/lib"
			bundleDir := "bashible"

			stdout, err := bundleCmd.ExecuteBundle(parentDir, bundleDir)
			if err != nil {
				if ee, ok := err.(*exec.ExitError); ok {
					return fmt.Errorf("bundle '%s' error: %v\nstderr: %s", bundleDir, err, string(ee.Stderr))
				}
				return fmt.Errorf("bundle '%s' error: %v", bundleDir, err)
			}
			logboek.LogInfoF("Got %d symbols\n", len(stdout))

			return nil
		})

		err = logboek.LogProcess("🛥️ Install Deckhouse 🛥️", log.TaskOptions(), func() error {
			kubeCl := kube.NewKubernetesClient().WithSshClient(sshClient)
			if err := kubeCl.Init(""); err != nil {
				return fmt.Errorf("open kubernetes connection: %v", err)
			}
			defer kubeCl.Stop()

			err = deckhouse.CreateDeckhouseManifests(kubeCl, &installConfig)
			if err != nil {
				return fmt.Errorf("deckhouse install: %v", err)
			}

			return nil
		})
		if err != nil {
			return err
		}

		return nil
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshClient, err := ssh.NewClientFromFlags().StartSession()
		if err != nil {
			return err
		}
		defer sshClient.StopSession()

		err = app.AskBecomePassword()
		if err != nil {
			return err
		}

		fmt.Print(banner)
		return logboek.LogProcess("🚀 Start Deckhouse CandI bootstrap 🚀",
			log.MainProcessOptions(), func() error { return runFunc(sshClient) })
	})

	return cmd
}
