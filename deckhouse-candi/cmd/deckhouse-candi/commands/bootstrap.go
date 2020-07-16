package commands

import (
	"encoding/json"
	"fmt"
	"os"
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

const rebootExitCode = 255

func DefineBootstrapCommand(kpApp *kingpin.Application) *kingpin.CmdClause {
	cmd := kpApp.Command("bootstrap", "Bootstrap cluster.")
	app.DefineSshFlags(cmd)
	app.DefineConfigFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineTerraformFlags(cmd)

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
			DeckhouseConfig:       metaConfig.MergeDeckhouseConfig(),
			ClusterConfig:         clusterConfig,
			ProviderClusterConfig: providerClusterConfig,
		}

		var nodeIP string
		// var masterInstanceClass []byte
		if metaConfig.ClusterType == "Cloud" {
			err = logboek.LogProcess("🌱 Run Terraform 🌱", log.TaskOptions(), func() error {
				basePipelineResult, err := terraform.NewPipeline(&terraform.PipelineOptions{
					Provider:           metaConfig.ProviderName,
					Layout:             metaConfig.Layout,
					Step:               "base-infrastructure",
					TerraformVariables: metaConfig.MarshalConfig(),
					StateDir:           app.TerraformStateDir,
					GetResult:          terraform.GetBasePipelineResult,
				}).Run()
				if err != nil {
					return err
				}

				masterPipelineResult, err := terraform.NewPipeline(&terraform.PipelineOptions{
					Provider:           metaConfig.ProviderName,
					Layout:             metaConfig.Layout,
					Step:               "master-node",
					TerraformVariables: metaConfig.MarshalNodeGroupConfig("master", 0, ""),
					StateDir:           app.TerraformStateDir,
					GetResult:          terraform.GetMasterNodePipelineResult,
				}).Run()
				if err != nil {
					return err
				}

				installConfig.CloudDiscovery = basePipelineResult["cloudDiscovery"]
				installConfig.TerraformState = basePipelineResult["terraformState"]

				_ = json.Unmarshal(masterPipelineResult["masterIPForSSH"], &app.SshHost)
				_ = json.Unmarshal(masterPipelineResult["nodeInternalIP"], &nodeIP)

				// Add tf-node-state to store it in kubernetes in future
				installConfig.NodesTerraformState = make(map[string][]byte)
				installConfig.NodesTerraformState["master-0"] = masterPipelineResult["terraformState"]

				sshClient.Settings.Host = app.SshHost

				logboek.LogInfoF("Master Address: %s", masterPipelineResult["masterIPForSSH"])
				return nil
			})
			if err != nil {
				return err
			}
		} else {
			var static struct {
				NodeIP string `json:"nodeIP"`
			}
			_ = json.Unmarshal(metaConfig.ClusterConfig["static"], &static)
			nodeIP = static.NodeIP
		}

		// wait for ssh connection to master
		err = logboek.LogProcess("🚝 Wait for SSH on master become ready 🚝", log.TaskOptions(), func() error {
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
			logboek.LogInfoF("Detected bundle: %s\n", bundleName)

			return nil
		})
		if err != nil {
			return err
		}

		// Generate bootstrap scripts
		templateController := template.NewTemplateController("")
		logboek.LogInfoF("Templates Dir: %q\n\n", templateController.TmpDir)

		err = logboek.LogProcess("🔨 Run Master Bootstrap 🔨", log.TaskOptions(), func() error {
			if err = template.PrepareBootstrap(templateController, nodeIP, bundleName, metaConfig); err != nil {
				return fmt.Errorf("prepare bootstrap: %v", err)
			}

			for _, bootstrapScript := range []string{"bootstrap.sh", "bootstrap-networks.sh"} {
				scriptPath := templateController.TmpDir + "/bootstrap/" + bootstrapScript

				err = logboek.LogProcess("Run "+bootstrapScript, log.BoldOptions(), func() error {
					if _, err := os.Stat(scriptPath); err != nil {
						if os.IsNotExist(err) {
							logboek.LogInfoF("Script %s doesn't found\n", scriptPath)
							return nil
						}
						return fmt.Errorf("script path: %v", err)
					}
					cmd := sshClient.UploadScript(scriptPath).
						WithStdoutHandler(func(l string) { logboek.LogInfoLn(l) }).
						Sudo()

					_, err := cmd.Execute()
					if err != nil {
						return fmt.Errorf("run %s: %w", scriptPath, err)
					}
					return nil
				})
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
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
		if err != nil {
			return err
		}

		err = logboek.LogProcess("⛺ Reboot master ⛺", log.TaskOptions(), func() error {
			rebootCmd := sshClient.Command("sudo", "reboot").Sudo().WithSSHArgs("-o", "ServerAliveInterval=15")
			if err := rebootCmd.Run(); err != nil {
				if ee, ok := err.(*exec.ExitError); ok {
					if ee.ExitCode() == rebootExitCode {
						return nil
					}
				}
				return fmt.Errorf("shutdown error: stdout: %s stderr: %s %v", rebootCmd.StdoutBuffer.String(), rebootCmd.StderrBuffer.String(), err)
			}
			return nil
		})
		if err != nil {
			return err
		}

		err = logboek.LogProcess("🚝 Wait for SSH on master become ready again 🚝", log.TaskOptions(), func() error {
			err = sshClient.Check().AwaitAvailability()
			if err != nil {
				return fmt.Errorf("await master available: %v", err)
			}
			return nil
		})
		if err != nil {
			return err
		}

		var kubeCl *kube.KubernetesClient
		err = logboek.LogProcess("Start Proxy", log.BoldOptions(), func() error {
			kubeCl = kube.NewKubernetesClient().WithSshClient(sshClient)
			if err := kubeCl.Init(""); err != nil {
				return fmt.Errorf("open kubernetes connection: %v", err)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("start kubernetes proxy: %v", err)
		}

		err = logboek.LogProcess("🛥️ Install Deckhouse 🛥️", log.TaskOptions(), func() error {
			err = deckhouse.WaitForKubernetesAPI(kubeCl)
			if err != nil {
				return fmt.Errorf("deckhouse wait api: %v", err)
			}

			err = deckhouse.CreateDeckhouseManifests(kubeCl, &installConfig)
			if err != nil {
				return fmt.Errorf("deckhouse create manifests: %v", err)
			}

			err = deckhouse.WaitForReadiness(kubeCl, &installConfig)
			if err != nil {
				return fmt.Errorf("deckhouse install: %v", err)
			}

			err = deckhouse.CreateNodeGroup(kubeCl, "master", metaConfig.MergeMasterNodeGroupConfig())
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return err
		}

		if metaConfig.ClusterType != "Cloud" {
			return nil
		}

		masterCloudConfig, err := deckhouse.GetCloudConfig(kubeCl, "master")
		if err != nil {
			return err
		}

		if metaConfig.MasterNodeGroupSpec.Replicas > 1 {
			for i := 1; i < metaConfig.MasterNodeGroupSpec.Replicas; i++ {
				stateSuffix := fmt.Sprintf("-%v", i)
				nodeName := fmt.Sprintf("master-%v", i)
				nodeConfig := metaConfig.MarshalNodeGroupConfig("master", i, masterCloudConfig)

				err = logboek.LogProcess(fmt.Sprintf("🌿 Bootstrap additional Master Node %v 🌿", i), log.TaskOptions(), func() error {
					state, err := terraform.NewPipeline(&terraform.PipelineOptions{
						Provider:           metaConfig.ProviderName,
						Layout:             metaConfig.Layout,
						Step:               "master-node",
						TerraformVariables: nodeConfig,
						StateDir:           app.TerraformStateDir,
						StateSuffix:        stateSuffix,
						GetResult:          terraform.OnlyState,
					}).Run()
					if err != nil {
						return err
					}

					return deckhouse.SaveNodeTerraformState(kubeCl, nodeName, state["terraformState"])
				})
				if err != nil {
					return err
				}
			}
		}

		staticNodeGroups := metaConfig.GetStaticNodeGroups()
		for _, staticNodeGroup := range staticNodeGroups {
			err = deckhouse.CreateNodeGroup(kubeCl, staticNodeGroup.Name, metaConfig.MergeNodeGroupConfig(staticNodeGroup))
			if err != nil {
				return err
			}

			nodeCloudConfig, err := deckhouse.GetCloudConfig(kubeCl, staticNodeGroup.Name)
			if err != nil {
				return err
			}

			for i := 0; i < staticNodeGroup.Replicas; i++ {
				stateSuffix := fmt.Sprintf("-%s-%v", staticNodeGroup.Name, i)
				nodeName := fmt.Sprintf("%s-%v", staticNodeGroup.Name, i)
				nodeConfig := metaConfig.MarshalNodeGroupConfig(staticNodeGroup.Name, i, nodeCloudConfig)

				err = logboek.LogProcess(fmt.Sprintf("🌿 Bootstrap additional node %v 🌿", nodeName), log.TaskOptions(), func() error {
					state, err := terraform.NewPipeline(&terraform.PipelineOptions{
						Provider:           metaConfig.ProviderName,
						Layout:             metaConfig.Layout,
						Step:               "static-node",
						TerraformVariables: nodeConfig,
						StateDir:           app.TerraformStateDir,
						StateSuffix:        stateSuffix,
						GetResult:          terraform.OnlyState,
					}).Run()
					if err != nil {
						return err
					}

					return deckhouse.SaveNodeTerraformState(kubeCl, nodeName, state["terraformState"])
				})
				if err != nil {
					return err
				}
			}
		}

		err = deckhouse.WaitForNodesBecomeReady(kubeCl, "master", metaConfig.MasterNodeGroupSpec.Replicas)
		if err != nil {
			return err
		}

		for _, staticNodeGroup := range staticNodeGroups {
			err = deckhouse.WaitForNodesBecomeReady(kubeCl, staticNodeGroup.Name, staticNodeGroup.Replicas)
			if err != nil {
				return err
			}
		}

		return nil
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshClient, err := ssh.NewClientFromFlags().Start()
		if err != nil {
			return err
		}

		err = app.AskBecomePassword()
		if err != nil {
			return err
		}

		fmt.Print(banner)
		err = logboek.LogProcess("🐜 Start Deckhouse CandI bootstrap 🐜",
			log.MainProcessOptions(), func() error { return runFunc(sshClient) })

		if err != nil {
			logboek.LogErrorF("\nCritical Error: %s\n", err)
			os.Exit(1)
		}
		return nil
	})

	return cmd
}
