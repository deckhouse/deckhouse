// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
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

const (
	versionMap     = "/deckhouse/candi/version_map.yml"
	imagesTagsJSON = "/deckhouse/candi/images_tags.json"
)

func printBanner() {
	log.InfoLn(banner)
}

func generateClusterUUID(stateCache state.Cache) (string, error) {
	var clusterUUID string
	err := log.Process("bootstrap", "Cluster UUID", func() error {
		ok, err := stateCache.InCache("uuid")
		if err != nil {
			return err
		}

		if !ok {
			genClusterUUID, err := uuid.NewRandom()
			if err != nil {
				return fmt.Errorf("can't create cluster UUID: %v", err)
			}

			clusterUUID = genClusterUUID.String()
			err = stateCache.Save("uuid", []byte(clusterUUID))
			if err != nil {
				return err
			}
			log.InfoF("Generated cluster UUID: %s\n", clusterUUID)
		} else {
			clusterUUIDBytes, err := stateCache.Load("uuid")
			if err != nil {
				return err
			}
			clusterUUID = string(clusterUUIDBytes)
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

	err = metaConfig.LoadImagesTags(imagesTagsJSON)
	if err != nil {
		return nil, err
	}

	if len(metaConfig.ProviderClusterConfig) == 0 && len(metaConfig.StaticClusterConfig) == 0 {
		return nil, fmt.Errorf("StaticClusterConfiguration must present for static-cluster bootstrap.")
	}
	return metaConfig, nil
}

func bootstrapAdditionalNodesForCloudCluster(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, masterAddressesForSSH map[string]string) error {
	if err := operations.BootstrapAdditionalMasterNodes(kubeCl, metaConfig, masterAddressesForSSH); err != nil {
		return err
	}

	terraNodeGroups := metaConfig.GetTerraNodeGroups()
	if err := operations.BootstrapTerraNodes(kubeCl, metaConfig, terraNodeGroups); err != nil {
		return err
	}

	return log.Process("bootstrap", "Waiting for additional Nodes", func() error {
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
}

func setBastionHostFromCloudProvider(host string, sshClient *ssh.Client) {
	app.SSHBastionHost = host
	app.SSHBastionUser = app.SSHUser
	app.SSHBastionPort = app.SSHPort

	if sshClient != nil {
		sshClient.Settings.BastionHost = app.SSHBastionHost
		sshClient.Settings.BastionUser = app.SSHBastionUser
		sshClient.Settings.BastionPort = app.SSHBastionPort
	}
}

func DefineBootstrapCommand(kpApp *kingpin.Application) *kingpin.CmdClause {
	cmd := kpApp.Command("bootstrap", "Bootstrap cluster.")
	app.DefineSSHFlags(cmd)
	app.DefineConfigFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineCacheFlags(cmd)
	app.DefineDropCacheFlags(cmd)
	app.DefineResourcesFlags(cmd, false)
	app.DefineDeckhouseFlags(cmd)
	app.DefineDontUsePublicImagesFlags(cmd)

	runFunc := func() error {
		masterAddressesForSSH := make(map[string]string)

		// first, parse and check cluster config
		metaConfig, err := loadConfigFromFile(app.ConfigPath)
		if err != nil {
			return err
		}

		// next parse and check resources
		var resourcesToCreate *config.Resources
		if app.ResourcesPath != "" {
			parsedResources, err := config.ParseResources(app.ResourcesPath)
			if err != nil {
				return err
			}

			resourcesToCreate = parsedResources
		}

		// next init cache
		cachePath := metaConfig.CachePath()
		if err = cache.Init(cachePath); err != nil {
			// TODO: it's better to ask for confirmation here
			return fmt.Errorf(cacheMessage, cachePath, err)
		}

		stateCache := cache.Global()

		if app.DropCache {
			stateCache.Clean()
			stateCache.Delete(state.TombstoneKey)
		}

		// after verifying configs and cache ask password
		sshClient, err := ssh.NewClientFromFlags().Start()
		if err != nil {
			return err
		}

		err = terminal.AskBecomePassword()
		if err != nil {
			return err
		}

		printBanner()

		clusterUUID, err := generateClusterUUID(stateCache)
		if err != nil {
			return err
		}
		metaConfig.UUID = clusterUUID

		deckhouseInstallConfig, err := deckhouse.PrepareDeckhouseInstallConfig(metaConfig)
		if err != nil {
			return err
		}

		// During full bootstrap we use the "kubeadm and deckhouse on master nodes" hack
		deckhouseInstallConfig.KubeadmBootstrap = true
		deckhouseInstallConfig.MasterNodeSelector = true

		var nodeIP string
		var devicePath string
		if metaConfig.ClusterType == config.CloudClusterType {
			err = log.Process("bootstrap", "Cloud infrastructure", func() error {
				baseRunner := terraform.NewRunnerFromConfig(metaConfig, "base-infrastructure", stateCache).
					WithVariables(metaConfig.MarshalConfig()).
					WithAutoApprove(true)
				tomb.RegisterOnShutdown("base-infrastructure", baseRunner.Stop)

				baseOutputs, err := terraform.ApplyPipeline(baseRunner, "Kubernetes cluster", terraform.GetBaseInfraResult)
				if err != nil {
					return err
				}

				masterNodeName := fmt.Sprintf("%s-master-0", metaConfig.ClusterPrefix)
				masterRunner := terraform.NewRunnerFromConfig(metaConfig, "master-node", stateCache).
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

				if baseOutputs.BastionHost != "" {
					setBastionHostFromCloudProvider(baseOutputs.BastionHost, sshClient)
					operations.SaveBastionHostToCache(baseOutputs.BastionHost)
				}

				app.SSHHosts = []string{masterOutputs.MasterIPForSSH}
				sshClient.Settings.SetAvailableHosts(app.SSHHosts)

				nodeIP = masterOutputs.NodeInternalIP
				devicePath = masterOutputs.KubeDataDevicePath

				deckhouseInstallConfig.NodesTerraformState = make(map[string][]byte)
				deckhouseInstallConfig.NodesTerraformState[masterNodeName] = masterOutputs.TerraformState

				masterAddressesForSSH[masterNodeName] = masterOutputs.MasterIPForSSH
				operations.SaveMasterHostsToCache(masterAddressesForSSH)
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

		if metaConfig.ClusterType == config.CloudClusterType {
			err := converge.NewInLockLocalRunner(kubeCl, "local-bootstraper").Run(func() error {
				return bootstrapAdditionalNodesForCloudCluster(kubeCl, metaConfig, masterAddressesForSSH)
			})
			if err != nil {
				return err
			}
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
			cache.Global().CleanWithExceptions(
				operations.MasterHostsCacheKey,
				operations.ManifestCreatedInClusterCacheKey,
				operations.BastionHostCacheKey,
			)
			log.WarnLn(`Next run of "dhctl bootstrap" will create a new Kubernetes cluster.`)
			return nil
		})

		if metaConfig.ClusterType == config.CloudClusterType {
			_ = log.Process("common", "Kubernetes Master Node addresses for SSH", func() error {
				for nodeName, address := range masterAddressesForSSH {
					fakeSession := sshClient.Settings.Copy()
					fakeSession.SetAvailableHosts([]string{address})
					log.InfoF("%s | %s\n", nodeName, fakeSession.String())
				}

				return nil
			})
		}

		return nil
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		return runFunc()
	})

	return cmd
}
