package bootstrap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/flant/logboek"
	"github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/deckhouse"
	"flant/deckhouse-candi/pkg/log"
	"flant/deckhouse-candi/pkg/ssh"
	"flant/deckhouse-candi/pkg/task"
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
	app.DefineTerraformFlags(cmd)
	app.DefineResourcesFlags(cmd)

	// Mute Shell-Operator logs
	logrus.SetLevel(logrus.PanicLevel)

	runFunc := func(sshClient *ssh.SshClient) error {
		metaConfig, err := config.ParseConfig(app.ConfigPath)
		if err != nil {
			return err
		}

		var resources *config.Resources
		if app.ResourcesPath != "" {
			parsedResources, err := config.ParseResources(app.ResourcesPath)
			if err != nil {
				return err
			}

			resources = parsedResources
		}

		deckhouseInstallConfig, err := deckhouse.PrepareDeckhouseInstallConfig(metaConfig)
		if err != nil {
			return err
		}

		var nodeIP string
		// var masterInstanceClass []byte
		if metaConfig.ClusterType == config.CloudClusterType {
			err = logboek.LogProcess("🚢 ~ Create Kubernetes Master node", log.TaskOptions(), func() error {
				baseStateFilepath := filepath.Join(app.TerraformStateDir, fmt.Sprintf("%s-base-infra.tfstate", metaConfig.ClusterPrefix))
				baseRunner := terraform.NewRunnerFromMetaConfig("base-infrastructure", metaConfig).
					WithVariables(metaConfig.MarshalConfig()).
					WithStatePath(baseStateFilepath).
					WithAutoApprove(true)

				basePipelineResult, err := terraform.ApplyPipeline(baseRunner, terraform.GetBaseInfraResult)
				if err != nil {
					return err
				}

				masterStateFilepath := filepath.Join(app.TerraformStateDir, fmt.Sprintf("%s-first-master.tfstate", metaConfig.ClusterPrefix))
				masterRunner := terraform.NewRunnerFromMetaConfig("master-node", metaConfig).
					WithVariables(metaConfig.PrepareTerraformNodeGroupConfig("master", 0, "")).
					WithStatePath(masterStateFilepath).
					WithAutoApprove(true)

				masterPipelineResult, err := terraform.ApplyPipeline(masterRunner, terraform.GetMasterNodeResult)
				if err != nil {
					return err
				}

				deckhouseInstallConfig.CloudDiscovery = basePipelineResult["cloudDiscovery"]
				deckhouseInstallConfig.TerraformState = basePipelineResult["terraformState"]

				_ = json.Unmarshal(masterPipelineResult["masterIPForSSH"], &app.SshHost)
				_ = json.Unmarshal(masterPipelineResult["nodeInternalIP"], &nodeIP)

				// Add tf-node-state to store it in kubernetes in future
				deckhouseInstallConfig.NodesTerraformState = make(map[string][]byte)
				deckhouseInstallConfig.NodesTerraformState["master-0"] = masterPipelineResult["terraformState"]

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

		if err := task.WaitForSSHConnectionOnMaster(sshClient); err != nil {
			return err
		}
		bundleName, err := task.DetermineBundleName(sshClient)
		if err != nil {
			return err
		}

		templateController := template.NewTemplateController("")
		logboek.LogInfoF("Templates Dir: %q\n\n", templateController.TmpDir)

		if err := task.BootstrapMaster(sshClient, bundleName, nodeIP, metaConfig, templateController); err != nil {
			return err
		}
		if err = task.PrepareBashibleBundle(bundleName, nodeIP, metaConfig, templateController); err != nil {
			return err
		}
		if err := task.ExecuteBashibleBundle(sshClient, templateController.TmpDir); err != nil {
			return err
		}
		if err := task.RebootMaster(sshClient); err != nil {
			return err
		}
		if err := task.WaitForSSHConnectionOnMaster(sshClient); err != nil {
			return err
		}
		kubeCl, err := task.StartKubernetesAPIProxy(sshClient)
		if err != nil {
			return err
		}
		if err := task.InstallDeckhouse(kubeCl, deckhouseInstallConfig, metaConfig.MarshalMasterNodeGroupConfig()); err != nil {
			return err
		}
		if metaConfig.ClusterType != config.CloudClusterType {
			// The rest of pipeline is additional master and static nodes creating process
			return nil
		}

		if err := task.BootstrapAdditionalMasterNodes(kubeCl, metaConfig, metaConfig.MasterNodeGroupSpec.Replicas); err != nil {
			return err
		}
		staticNodeGroups := metaConfig.GetStaticNodeGroups()
		if err := task.BootstrapStaticNodes(kubeCl, metaConfig, staticNodeGroups); err != nil {
			return err
		}
		if err := deckhouse.WaitForNodesBecomeReady(kubeCl, "master", metaConfig.MasterNodeGroupSpec.Replicas); err != nil {
			return err
		}
		for _, staticNodeGroup := range staticNodeGroups {
			if err := deckhouse.WaitForNodesBecomeReady(kubeCl, staticNodeGroup.Name, staticNodeGroup.Replicas); err != nil {
				return err
			}
		}

		if resources != nil {
			err = logboek.LogProcess("⛴️ ~ Create Resources", log.TaskOptions(), func() error {
				return deckhouse.CreateResourcesLoop(kubeCl, resources)
			})
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
		err = logboek.LogProcess("⛵ ~ Bootstrap: Deckhouse Cluster and Infrastructure",
			log.MainProcessOptions(), func() error { return runFunc(sshClient) })

		if err != nil {
			logboek.LogErrorF("\nCritical Error: %s\n", err)
			os.Exit(1)
		}
		return nil
	})

	return cmd
}
