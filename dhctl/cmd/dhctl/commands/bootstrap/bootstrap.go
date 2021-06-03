package bootstrap

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/converge"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/resources"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
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

const cacheMessage = `Create cache %s:
	Error: %v

	Probably that Kubernetes cluster was successfully bootstrapped.
	If you want to continue, please delete the cache folder manually.
`

const versionMap = "/deckhouse/candi/version_map.yml"

func printBanner() {
	log.InfoLn(banner)
}

func generateClusterUUID() (string, error) {
	var clusterUUID string
	err := log.Process("bootstrap", "Cluster UUID", func() error {
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
	return clusterUUID, err
}

func loadConfigFromFile(path string) (*config.MetaConfig, error) {
	metaConfig, err := config.ParseConfig(path)
	if err != nil {
		return nil, err
	}

	if metaConfig.ClusterConfig == nil {
		return nil, fmt.Errorf("ClusterConfiguration must be provided")
	}

	err = metaConfig.LoadVersionMap(versionMap)
	if err != nil {
		return nil, err
	}
	if len(metaConfig.ProviderClusterConfig) == 0 && len(metaConfig.StaticClusterConfig) == 0 {
		return nil, fmt.Errorf("StaticClusterConfiguration must present for static-cluster bootstrap.")
	}
	return metaConfig, nil
}

func DefineBootstrapCommand(kpApp *kingpin.Application) *kingpin.CmdClause {
	cmd := kpApp.Command("bootstrap", "Bootstrap cluster.")
	app.DefineSSHFlags(cmd)
	app.DefineConfigFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineCacheFlags(cmd)
	app.DefineDropCacheFlags(cmd)
	app.DefineResourcesFlags(cmd, false)

	runFunc := func() error {
		masterAddressesForSSH := make(map[string]string)

		metaConfig, err := loadConfigFromFile(app.ConfigPath)
		if err != nil {
			return err
		}

		sshClient, err := ssh.NewClientFromFlags().Start()
		if err != nil {
			return err
		}

		err = terminal.AskBecomePassword()
		if err != nil {
			return err
		}

		printBanner()

		cachePath := metaConfig.CachePath()
		if err = cache.Init(cachePath); err != nil {
			// TODO: it's better to ask for confirmation here
			return fmt.Errorf(cacheMessage, cachePath, err)
		}

		if app.DropCache {
			cache.Global().Clean()
			cache.Global().Delete(".tombstone")
		}

		var resourcesToCreate *config.Resources
		if app.ResourcesPath != "" {
			parsedResources, err := config.ParseResources(app.ResourcesPath)
			if err != nil {
				return err
			}

			resourcesToCreate = parsedResources
		}

		clusterUUID, err := generateClusterUUID()
		if err != nil {
			return err
		}
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
				tomb.RegisterOnShutdown("base-infrastructure", baseRunner.Stop)

				baseOutputs, err := terraform.ApplyPipeline(baseRunner, "Kubernetes cluster", terraform.GetBaseInfraResult)
				if err != nil {
					return err
				}

				masterNodeName := fmt.Sprintf("%s-master-0", metaConfig.ClusterPrefix)
				masterRunner := terraform.NewRunnerFromConfig(metaConfig, "master-node").
					WithVariables(metaConfig.NodeGroupConfig("master", 0, "")).
					WithName(masterNodeName).
					WithAutoApprove(true)
				tomb.RegisterOnShutdown(masterNodeName, masterRunner.Stop)

				masterOutputs, err := terraform.ApplyPipeline(masterRunner, masterNodeName, terraform.GetMasterNodeResult)
				if err != nil {
					return err
				}

				deckhouseInstallConfig.CloudDiscovery = baseOutputs.CloudDiscovery
				deckhouseInstallConfig.TerraformState = baseOutputs.TerraformState

				app.SSHHosts = []string{masterOutputs.MasterIPForSSH}
				sshClient.Settings.SetAvailableHosts(app.SSHHosts)

				nodeIP = masterOutputs.NodeInternalIP
				devicePath = masterOutputs.KubeDataDevicePath

				deckhouseInstallConfig.NodesTerraformState = make(map[string][]byte)
				deckhouseInstallConfig.NodesTerraformState[masterNodeName] = masterOutputs.TerraformState

				masterAddressesForSSH[masterNodeName] = masterOutputs.MasterIPForSSH
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

		if err := operations.WaitForSSHConnectionOnMaster(sshClient); err != nil {
			return err
		}
		if err := operations.RunBashiblePipeline(sshClient, metaConfig, nodeIP, devicePath); err != nil {
			return err
		}
		kubeCl, err := operations.ConnectToKubernetesAPI(sshClient)
		if err != nil {
			return err
		}
		if err := operations.InstallDeckhouse(kubeCl, deckhouseInstallConfig, metaConfig.MasterNodeGroupManifest()); err != nil {
			return err
		}

		if metaConfig.ClusterType != config.CloudClusterType {
			// The rest of pipeline is additional master and static nodes creating process
			return nil
		}

		bootstrapIdentity := config.GetLocalConvergeLockIdentity("local-bootstraper")
		leaseLock := client.NewLeaseLock(kubeCl, config.GetConvergeLockLeaseConfig(bootstrapIdentity))
		err = leaseLock.Lock()
		if err != nil {
			return err
		}
		defer leaseLock.Unlock()

		if err := operations.BootstrapAdditionalMasterNodes(kubeCl, metaConfig, masterAddressesForSSH); err != nil {
			return err
		}

		terraNodeGroups := metaConfig.GetTerraNodeGroups()
		if err := operations.BootstrapTerraNodes(kubeCl, metaConfig, terraNodeGroups); err != nil {
			return err
		}

		err = log.Process("bootstrap", "Waiting for additional Nodes", func() error {
			if err := converge.WaitForNodesBecomeReady(kubeCl, "master", metaConfig.MasterNodeGroupSpec.Replicas); err != nil {
				return err
			}
			for _, terraNodeGroup := range terraNodeGroups {
				if err := converge.WaitForNodesBecomeReady(kubeCl, terraNodeGroup.Name, terraNodeGroup.Replicas); err != nil {
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

		_ = log.Process("bootstrap", "Clear cache", func() error {
			cache.Global().Clean()
			log.WarnLn(`Next run of "dhctl bootstrap" will create a new Kubernetes cluster.`)
			return nil
		})

		_ = log.Process("common", "Kubernetes Master Node addresses for SSH", func() error {
			for nodeName, address := range masterAddressesForSSH {
				fakeSession := sshClient.Settings.Copy()
				fakeSession.SetAvailableHosts([]string{address})
				log.InfoF("%s | %s\n", nodeName, fakeSession.String())
			}
			return nil
		})

		return nil
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		return runFunc()
	})

	return cmd
}
