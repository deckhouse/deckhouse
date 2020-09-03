package bootstrap

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/commands"
	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/kubernetes/actions/converge"
	"flant/deckhouse-candi/pkg/kubernetes/actions/deckhouse"
	"flant/deckhouse-candi/pkg/kubernetes/actions/resources"
	"flant/deckhouse-candi/pkg/log"
	"flant/deckhouse-candi/pkg/system/ssh"
	"flant/deckhouse-candi/pkg/terraform"
	"flant/deckhouse-candi/pkg/util/cache"
)

const banner = "" +
	`========================================================================================
 _____             _     _                                ______                _ _____
(____ \           | |   | |                              / _____)              | (_____)
 _   \ \ ____ ____| |  _| | _   ___  _   _  ___  ____   | /      ____ ____   _ | |  _
| |   | / _  ) ___) | / ) || \ / _ \| | | |/___)/ _  )  | |     / _  |  _ \ / || | | |
| |__/ ( (/ ( (___| |< (| | | | |_| | |_| |___ ( (/ /   | \____( ( | | | | ( (_| |_| |_
|_____/ \____)____)_| \_)_| |_|\___/ \____(___/ \____)   \______)_||_|_| |_|\____(_____)
========================================================================================`

func printBanner() {
	_ = log.Process("bootstrap", "Banner", func() error {
		log.InfoLn(banner)
		return nil
	})
}

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

		cachePath := metaConfig.CachePath()
		if err = cache.Init(cachePath); err != nil {
			// TODO: it's better to ask for confirmation here
			return fmt.Errorf(
				"Create cache %s:\n\tError: %v\n\n"+
					"\tProbably that Kubernetes cluster was successfully bootstrapped.\n"+
					"\tIf you want to continue, please delete the cache folder manually.",
				cachePath, err,
			)
		}

		var resourcesToCreate *config.Resources
		if app.ResourcesPath != "" {
			parsedResources, err := config.ParseResources(app.ResourcesPath)
			if err != nil {
				return err
			}

			resourcesToCreate = parsedResources
		}

		var clusterUUID string
		err = log.Process("bootstrap", "Cluster UUID", func() error {
			if !cache.Global().InCache("uuid") {
				genClusterUUID, err := uuid.NewRandom()
				if err != nil {
					return fmt.Errorf("can't create cluster UUID: %v", err)
				}

				clusterUUID = genClusterUUID.String()
				cache.Global().Save("uuid", []byte(clusterUUID))
				log.InfoF("Generated cluster UUID: %s\n", clusterUUID)
			} else {
				clusterUUID = string(cache.Global().Load("uuid"))
				log.InfoF("Cluster UUID from cache: %s\n", clusterUUID)
			}
			return nil
		})
		if err != nil {
			return nil
		}
		cache.Global().AddToClean("uuid")
		metaConfig.UUID = clusterUUID

		deckhouseInstallConfig, err := deckhouse.PrepareDeckhouseInstallConfig(metaConfig)
		if err != nil {
			return err
		}

		var nodeIP string
		var devicePath string
		if metaConfig.ClusterType == config.CloudClusterType {
			err = log.Process("bootstrap", "Cloud infrastructure", func() error {
				baseRunner := terraform.NewRunnerFromConfig(metaConfig, "base-infrastructure").
					WithVariables(metaConfig.MarshalConfig()).
					WithAutoApprove(true)

				baseOutputs, err := terraform.ApplyPipeline(baseRunner, "Kubernetes cluster", terraform.GetBaseInfraResult)
				if err != nil {
					return err
				}

				masterNodeName := fmt.Sprintf("%s-master-0", metaConfig.ClusterPrefix)
				masterRunner := terraform.NewRunnerFromConfig(metaConfig, "master-node").
					WithVariables(metaConfig.NodeGroupConfig("master", 0, "")).
					WithName(masterNodeName).
					WithAutoApprove(true)

				masterOutputs, err := terraform.ApplyPipeline(masterRunner, masterNodeName, terraform.GetMasterNodeResult)
				if err != nil {
					return err
				}

				deckhouseInstallConfig.CloudDiscovery = baseOutputs.CloudDiscovery
				deckhouseInstallConfig.TerraformState = baseOutputs.TerraformState

				app.SshHost = masterOutputs.MasterIPForSSH
				sshClient.Settings.Host = masterOutputs.MasterIPForSSH

				nodeIP = masterOutputs.NodeInternalIP
				devicePath = masterOutputs.KubeDataDevicePath

				deckhouseInstallConfig.NodesTerraformState = make(map[string][]byte)
				deckhouseInstallConfig.NodesTerraformState[masterNodeName] = masterOutputs.TerraformState

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

		if err := commands.WaitForSSHConnectionOnMaster(sshClient); err != nil {
			return err
		}
		if err := commands.RunBashiblePipeline(sshClient, metaConfig, nodeIP, devicePath); err != nil {
			return err
		}
		kubeCl, err := commands.StartKubernetesAPIProxy(sshClient)
		if err != nil {
			return err
		}
		if err := commands.InstallDeckhouse(kubeCl, deckhouseInstallConfig, metaConfig.MasterNodeGroupManifest()); err != nil {
			return err
		}

		if metaConfig.ClusterType != config.CloudClusterType {
			// The rest of pipeline is additional master and static nodes creating process
			return nil
		}

		if err := commands.BootstrapAdditionalMasterNodes(kubeCl, metaConfig, metaConfig.MasterNodeGroupSpec.Replicas); err != nil {
			return err
		}

		staticNodeGroups := metaConfig.GetStaticNodeGroups()
		if err := commands.BootstrapStaticNodes(kubeCl, metaConfig, staticNodeGroups); err != nil {
			return err
		}

		err = log.Process("bootstrap", "Waiting for additional Nodes", func() error {
			if err := converge.WaitForNodesBecomeReady(kubeCl, "master", metaConfig.MasterNodeGroupSpec.Replicas); err != nil {
				return err
			}
			for _, staticNodeGroup := range staticNodeGroups {
				if err := converge.WaitForNodesBecomeReady(kubeCl, staticNodeGroup.Name, staticNodeGroup.Replicas); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return nil
		}

		if resourcesToCreate != nil {
			err = log.Process("bootstrap", "Create Resources", func() error {
				return resources.CreateResourcesLoop(kubeCl, resourcesToCreate)
			})
			if err != nil {
				return err
			}
		}

		_ = log.Process("bootstrap", "Clean cache", func() error {
			cache.Global().Clean()
			log.Warning(`Next run of "deckhouse-candi bootstrap" will create a new Kubernetes cluster.\n`)
			return nil
		})
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

		printBanner()
		err = runFunc(sshClient)

		if err != nil {
			log.ErrorLn(err.Error())
			os.Exit(1)
		}
		return nil
	})

	return cmd
}
