// Copyright 2023 Flant JSC
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
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"reflect"
	"time"

	"github.com/google/uuid"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/converge"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/resources"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infra/hook/controlplane"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/local"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
)

const (
	banner = "" +
		`========================================================================================
 _____             _     _                                ______                _ _____
(____ \           | |   | |                              / _____)              | (_____)
 _   \ \ ____ ____| |  _| | _   ___  _   _  ___  ____   | /      ____ ____   _ | |  _
| |   | / _  ) ___) | / ) || \ / _ \| | | |/___)/ _  )  | |     / _  |  _ \ / || | | |
| |__/ ( (/ ( (___| |< (| | | | |_| | |_| |___ ( (/ /   | \____( ( | | | | ( (_| |_| |_
|_____/ \____)____)_| \_)_| |_|\___/ \____(___/ \____)   \______)_||_|_| |_|\____(_____)
========================================================================================`

	bootstrapAbortInvalidCacheMessage = `Create cache %s:
	Error: %v
	Probably that Kubernetes cluster was successfully bootstrapped.
	Use "dhctl destroy" command to delete the cluster.
`
	bootstrapPhaseBaseInfraNonCloudMessage = `It is impossible to create base-infrastructure for non-cloud Kubernetes cluster.
You have to create it manually.
`
	bootstrapAbortCheckMessage = `You will be asked for approval multiple times.
If you are confident in your actions, you can use the flag "--yes-i-am-sane-and-i-understand-what-i-am-doing" to skip approvals.
`
	cacheMessage = `Create cache %s:
	Error: %v

	Probably that Kubernetes cluster was successfully bootstrapped.
	If you want to continue, please delete the cache folder manually.
`
)

// TODO(remove-global-app): Support all needed parameters in Params, remove usage of app.*
type Params struct {
	NodeInterface              node.Interface
	InitialState               phases.DhctlState
	ResetInitialState          bool
	DisableBootstrapClearCache bool
	OnPhaseFunc                phases.DefaultOnPhaseFunc
	CommanderMode              bool
	CommanderUUID              uuid.UUID
	TerraformContext           *terraform.TerraformContext

	ConfigPaths             []string
	ResourcesPath           string
	ResourcesTimeout        time.Duration
	DeckhouseTimeout        time.Duration
	PostBootstrapScriptPath string
	UseTfCache              *bool
	AutoApprove             *bool

	*client.KubernetesInitParams
}

type ClusterBootstrapper struct {
	*Params
	PhasedExecutionContext phases.DefaultPhasedExecutionContext

	initializeNewAgent bool
	// TODO(dhctl-for-commander): pass stateCache externally using params as in Destroyer, this variable will be unneeded then
	lastState phases.DhctlState
}

func NewClusterBootstrapper(params *Params) *ClusterBootstrapper {
	return &ClusterBootstrapper{
		Params:                 params,
		PhasedExecutionContext: phases.NewDefaultPhasedExecutionContext(params.OnPhaseFunc),
		lastState:              params.InitialState,
	}
}

// TODO(remove-global-app): Eliminate usage of app.* global variables,
// TODO(remove-global-app):  use explicitly passed params everywhere instead,
// TODO(remove-global-app):  applyParams will not be needed anymore then.
//
// applyParams overrides app.* options that are explicitly passed using Params struct
func (b *ClusterBootstrapper) applyParams() (func(), error) {
	var restoreFuncs []func()
	restoreFunc := func() {
		for _, f := range restoreFuncs {
			f()
		}
	}

	if len(b.ConfigPaths) > 0 {
		restoreFuncs = append(restoreFuncs, setWithRestore(&app.ConfigPaths, b.ConfigPaths))
	}
	if b.ResourcesPath != "" {
		restoreFuncs = append(restoreFuncs, setWithRestore(&app.ResourcesPath, b.ResourcesPath))
	}
	if b.ResourcesTimeout != 0 {
		restoreFuncs = append(restoreFuncs, setWithRestore(&app.ResourcesTimeout, b.ResourcesTimeout))
	}
	if b.DeckhouseTimeout != 0 {
		restoreFuncs = append(restoreFuncs, setWithRestore(&app.DeckhouseTimeout, b.DeckhouseTimeout))
	}
	if b.PostBootstrapScriptPath != "" {
		restoreFuncs = append(restoreFuncs, setWithRestore(&app.PostBootstrapScriptPath, b.PostBootstrapScriptPath))
	}
	if b.UseTfCache != nil {
		var newValue string
		if *b.UseTfCache {
			newValue = app.UseStateCacheYes
		} else {
			newValue = app.UseStateCacheNo
		}
		restoreFuncs = append(restoreFuncs, setWithRestore(&app.UseTfCache, newValue))
	}
	if b.AutoApprove != nil {
		restoreFuncs = append(restoreFuncs, setWithRestore(&app.SanityCheck, *b.AutoApprove))
	}
	if b.KubernetesInitParams != nil {
		restoreFuncs = append(restoreFuncs, setWithRestore(&app.KubeConfigInCluster, b.KubernetesInitParams.KubeConfigInCluster))
		restoreFuncs = append(restoreFuncs, setWithRestore(&app.KubeConfig, b.KubernetesInitParams.KubeConfig))
		restoreFuncs = append(restoreFuncs, setWithRestore(&app.KubeConfigContext, b.KubernetesInitParams.KubeConfigContext))
	}
	return restoreFunc, nil
}

func (b *ClusterBootstrapper) Bootstrap() error {
	if restore, err := b.applyParams(); err != nil {
		return err
	} else {
		defer restore()
	}

	masterAddressesForSSH := make(map[string]string)

	if app.PostBootstrapScriptPath != "" {
		log.DebugF("Have post bootstrap script: %s\n", app.PostBootstrapScriptPath)
		if err := ValidateScriptFile(app.PostBootstrapScriptPath); err != nil {
			return err
		}
	}

	if app.ResourcesPath != "" {
		log.WarnLn("--resources flag is deprecated. Please use --config flag multiple repeatedly for logical resources separation")
		app.ConfigPaths = append(app.ConfigPaths, app.ResourcesPath)
	}

	// first, parse and check cluster config
	metaConfig, err := config.LoadConfigFromFile(app.ConfigPaths)
	if err != nil {
		return err
	}

	if b.Params.NodeInterface == nil || reflect.ValueOf(b.Params.NodeInterface).IsNil() {
		log.DebugLn("NodeInterface is nil")
		if len(app.SSHHosts) == 0 && metaConfig.IsStatic() {
			log.DebugLn("Hosts empty and static cluster. Use local interface")
			b.Params.NodeInterface = local.NewNodeInterface()
		} else {
			sshClient := ssh.NewClientFromFlags()
			if _, err := sshClient.Start(); err != nil {
				return fmt.Errorf("unable to start ssh client: %w", err)
			}
			log.DebugF("Hosts is %v empty; static cluster is %v. Use ssh", len(app.SSHHosts), metaConfig.IsStatic())
			b.Params.NodeInterface = ssh.NewNodeInterfaceWrapper(sshClient)
		}
	}

	log.DebugLn("MetaConfig was loaded")

	// next init cache
	cachePath := metaConfig.CachePath()
	if err = cache.InitWithOptions(cachePath, cache.CacheOptions{InitialState: b.InitialState, ResetInitialState: b.ResetInitialState}); err != nil {
		// TODO: it's better to ask for confirmation here
		return fmt.Errorf(cacheMessage, cachePath, err)
	}

	stateCache := cache.Global()

	log.InfoF("State directory: %s\n", stateCache.Dir())

	if app.DropCache {
		stateCache.Clean()
		stateCache.Delete(state.TombstoneKey)
		log.DebugLn("Cache was dropped")
	}

	if err := b.PhasedExecutionContext.InitPipeline(stateCache); err != nil {
		return err
	}
	// TODO(dhctl-for-commander): pass stateCache externally using params as in Destroyer, this variable will be unneeded then
	b.lastState = nil
	defer b.PhasedExecutionContext.Finalize(stateCache)

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

	deckhouseInstallConfig, err := config.PrepareDeckhouseInstallConfig(metaConfig)
	if err != nil {
		return err
	}

	if b.CommanderMode {
		// FIXME(dhctl-for-commander): commander uuid currently optional, make it required later
		// if b.CommanderUUID == uuid.Nil {
		//	panic("CommanderUUID required for bootstrap operation in commander mode!")
		// }
		deckhouseInstallConfig.CommanderMode = b.CommanderMode
		deckhouseInstallConfig.CommanderUUID = b.CommanderUUID
	}

	// During full bootstrap we use the "kubeadm and deckhouse on master nodes" hack
	deckhouseInstallConfig.KubeadmBootstrap = true
	deckhouseInstallConfig.MasterNodeSelector = true

	bootstrapState := NewBootstrapState(stateCache)

	preflightChecker := preflight.NewChecker(b.NodeInterface, deckhouseInstallConfig, metaConfig, bootstrapState)
	if err := preflightChecker.Global(); err != nil {
		return err
	}

	if shouldStop, err := b.PhasedExecutionContext.StartPhase(phases.BaseInfraPhase, true, stateCache); err != nil {
		return err
	} else if shouldStop {
		return nil
	}

	var nodeIP string
	var devicePath string
	var resourcesTemplateData map[string]interface{}

	if metaConfig.ClusterType == config.CloudClusterType {
		err = preflightChecker.Cloud()
		if err != nil {
			return err
		}
		err = log.Process("bootstrap", "Cloud infrastructure", func() error {
			baseRunner := b.TerraformContext.GetBootstrapBaseInfraRunner(metaConfig, stateCache)

			baseOutputs, err := terraform.ApplyPipeline(baseRunner, "Kubernetes cluster", terraform.GetBaseInfraResult)
			if err != nil {
				return err
			}

			log.DebugLn("Base infrastructure was created")

			var cloudDiscoveryData map[string]interface{}
			err = json.Unmarshal(baseOutputs.CloudDiscovery, &cloudDiscoveryData)
			if err != nil {
				return err
			}

			resourcesTemplateData = map[string]interface{}{
				"cloudDiscovery": cloudDiscoveryData,
			}

			masterNodeName := fmt.Sprintf("%s-master-0", metaConfig.ClusterPrefix)
			masterRunner := b.Params.TerraformContext.GetBootstrapNodeRunner(metaConfig, stateCache, terraform.BootstrapNodeRunnerOptions{
				AutoApprove:     true,
				NodeName:        masterNodeName,
				NodeGroupStep:   "master-node",
				NodeGroupName:   "master",
				NodeIndex:       0,
				NodeCloudConfig: "",
			})

			masterOutputs, err := terraform.ApplyPipeline(masterRunner, masterNodeName, terraform.GetMasterNodeResult)
			if err != nil {
				return err
			}

			log.DebugLn("First control-plane node was created")

			deckhouseInstallConfig.CloudDiscovery = baseOutputs.CloudDiscovery
			deckhouseInstallConfig.TerraformState = baseOutputs.TerraformState

			if wrapper, ok := b.NodeInterface.(*ssh.NodeInterfaceWrapper); ok {
				sshClient := wrapper.Client()
				if baseOutputs.BastionHost != "" {
					sshClient.Settings.BastionHost = baseOutputs.BastionHost
					SaveBastionHostToCache(baseOutputs.BastionHost)
				}
				sshClient.Settings.SetAvailableHosts([]string{masterOutputs.MasterIPForSSH})
			}

			nodeIP = masterOutputs.NodeInternalIP
			devicePath = masterOutputs.KubeDataDevicePath

			deckhouseInstallConfig.NodesTerraformState = make(map[string][]byte)
			deckhouseInstallConfig.NodesTerraformState[masterNodeName] = masterOutputs.TerraformState

			masterAddressesForSSH[masterNodeName] = masterOutputs.MasterIPForSSH
			SaveMasterHostsToCache(masterAddressesForSSH)
			return nil
		})
		if err != nil {
			return err
		}
	} else {
		err = preflightChecker.Static()
		if err != nil {
			return err
		}
		var static struct {
			NodeIP string `json:"nodeIP"`
		}
		_ = json.Unmarshal(metaConfig.ClusterConfig["static"], &static)
		nodeIP = static.NodeIP

		if wrapper, ok := b.NodeInterface.(*ssh.NodeInterfaceWrapper); ok {
			sshClient := wrapper.Client()
			if sshClient.Settings.BastionHost != "" {
				SaveBastionHostToCache(sshClient.Settings.BastionHost)
			}

			SaveMasterHostsToCache(map[string]string{
				"first-master": sshClient.Settings.Host(),
			})
		}
	}

	// next parse and check resources
	// do it after bootstrap cloud because resources can be template
	// and we want to fail immediately if template has errors
	var resourcesToCreate template.Resources
	if metaConfig.ResourcesYAML != "" {
		parsedResources, err := template.ParseResourcesContent(metaConfig.ResourcesYAML, resourcesTemplateData)
		if err != nil {
			return err
		}

		resourcesToCreate = parsedResources
	}

	if shouldStop, err := b.PhasedExecutionContext.SwitchPhase(phases.RegistryPackagesProxyPhase, false, stateCache, nil); err != nil {
		return err
	} else if shouldStop {
		return nil
	}

	if wrapper, ok := b.NodeInterface.(*ssh.NodeInterfaceWrapper); ok {
		if err := WaitForSSHConnectionOnMaster(wrapper.Client()); err != nil {
			return fmt.Errorf("failed to wait for SSH connection on master: %v", err)
		}
	}

	if shouldStop, err := b.PhasedExecutionContext.SwitchPhase(phases.ExecuteBashibleBundlePhase, false, stateCache, nil); err != nil {
		return err
	} else if shouldStop {
		return nil
	}

	if err := RunBashiblePipeline(b.NodeInterface, metaConfig, nodeIP, devicePath); err != nil {
		return err
	}

	if shouldStop, err := b.PhasedExecutionContext.SwitchPhase(phases.InstallDeckhousePhase, false, stateCache, nil); err != nil {
		return err
	} else if shouldStop {
		return nil
	}

	kubeCl, err := kubernetes.ConnectToKubernetesAPI(b.NodeInterface)
	if err != nil {
		return err
	}
	if err := InstallDeckhouse(kubeCl, deckhouseInstallConfig); err != nil {
		return err
	}

	if metaConfig.ClusterType == config.CloudClusterType {
		if shouldStop, err := b.PhasedExecutionContext.SwitchPhase(phases.InstallAdditionalMastersAndStaticNodes, true, stateCache, nil); err != nil {
			return err
		} else if shouldStop {
			return nil
		}

		localBootstraper := func(f func() error) error {
			if b.CommanderMode {
				return f()
			}
			return converge.NewInLockLocalRunner(kubeCl, "local-bootstraper").Run()
		}

		err := localBootstraper(func() error {
			return bootstrapAdditionalNodesForCloudCluster(kubeCl, metaConfig, masterAddressesForSSH, b.TerraformContext)
		})
		if err != nil {
			return err
		}
	}

	if shouldStop, err := b.PhasedExecutionContext.SwitchPhase(phases.CreateResourcesPhase, false, stateCache, nil); err != nil {
		return err
	} else if shouldStop {
		return nil
	}

	if err := controlplane.NewManagerReadinessChecker(kubeCl).IsReadyAll(); err != nil {
		return err
	}

	err = createResources(kubeCl, resourcesToCreate, metaConfig)
	if err != nil {
		return err
	}

	if shouldStop, err := b.PhasedExecutionContext.SwitchPhase(phases.ExecPostBootstrapPhase, false, stateCache, nil); err != nil {
		return err
	} else if shouldStop {
		return nil
	}

	sshNodeInterfaceWrapper, ok := b.NodeInterface.(*ssh.NodeInterfaceWrapper)
	if ok && app.PostBootstrapScriptPath != "" {
		postScriptExecutor := NewPostBootstrapScriptExecutor(sshNodeInterfaceWrapper.Client(), app.PostBootstrapScriptPath, bootstrapState).
			WithTimeout(app.PostBootstrapScriptTimeout)

		if err := postScriptExecutor.Execute(); err != nil {
			return err
		}
	}

	if shouldStop, err := b.PhasedExecutionContext.SwitchPhase(phases.FinalizationPhase, false, stateCache, nil); err != nil {
		return err
	} else if shouldStop {
		return nil
	}

	if err := deckhouse.ConfigureReleaseChannel(kubeCl, deckhouseInstallConfig); err != nil {
		return err
	}

	if !b.DisableBootstrapClearCache {
		_ = log.Process("bootstrap", "Clear cache", func() error {
			cache.Global().CleanWithExceptions(
				MasterHostsCacheKey,
				ManifestCreatedInClusterCacheKey,
				BastionHostCacheKey,
				PostBootstrapResultCacheKey,
			)
			log.WarnLn(`Next run of "dhctl bootstrap" will create a new Kubernetes cluster.`)
			return nil
		})
	}

	log.Success("Deckhouse cluster was created successfully!\n")

	if metaConfig.ClusterType == config.CloudClusterType {
		_ = log.Process("common", "Kubernetes Master Node addresses for SSH", func() error {
			wrapper := b.NodeInterface.(*ssh.NodeInterfaceWrapper)
			for nodeName, address := range masterAddressesForSSH {
				fakeSession := wrapper.Client().Settings.Copy()
				fakeSession.SetAvailableHosts([]string{address})
				log.InfoF("%s | %s\n", nodeName, fakeSession.String())
			}

			return nil
		})
	}

	return b.PhasedExecutionContext.CompletePhaseAndPipeline(stateCache, nil)
}

// TODO(dhctl-for-commander): pass stateCache externally using params as in Destroyer, this method will be unneeded then
func (c *ClusterBootstrapper) GetLastState() phases.DhctlState {
	if c.lastState != nil {
		return c.lastState
	} else {
		return c.PhasedExecutionContext.GetLastState()
	}
}

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

func bootstrapAdditionalNodesForCloudCluster(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, masterAddressesForSSH map[string]string, terraformContext *terraform.TerraformContext) error {
	if err := BootstrapAdditionalMasterNodes(kubeCl, metaConfig, masterAddressesForSSH, terraformContext); err != nil {
		return err
	}

	terraNodeGroups := metaConfig.GetTerraNodeGroups()
	if err := BootstrapTerraNodes(kubeCl, metaConfig, terraNodeGroups, terraformContext); err != nil {
		return err
	}

	return log.Process("bootstrap", "Waiting for Node Groups are ready", func() error {
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

func createResources(kubeCl *client.KubernetesClient, resourcesToCreate template.Resources, metaConfig *config.MetaConfig) error {
	log.WarnLn("Some resources require at least one non-master node to be added to the cluster.")

	if resourcesToCreate == nil {
		return nil
	}

	return log.Process("bootstrap", "Create Resources", func() error {
		checkers, err := resources.GetCheckers(kubeCl, resourcesToCreate, metaConfig)
		if err != nil {
			return err
		}

		return resources.CreateResourcesLoop(kubeCl, resourcesToCreate, checkers)
	})
}

func setWithRestore[T any](target *T, newValue T) func() {
	oldValue := *target
	*target = newValue
	return func() {
		*target = oldValue
	}
}
