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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/name212/govalue"
	"golang.org/x/term"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/resources"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infrastructure/hook/controlplane"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/lock"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/local"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/sshclient"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
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
	OnProgressFunc             phases.OnProgressFunc
	CommanderMode              bool
	CommanderUUID              uuid.UUID
	InfrastructureContext      *infrastructure.Context

	ConfigPaths             []string
	ResourcesTimeout        time.Duration
	DeckhouseTimeout        time.Duration
	PostBootstrapScriptPath string
	UseTfCache              *bool
	AutoApprove             *bool

	TmpDir  string
	Logger  log.Logger
	IsDebug bool

	*client.KubernetesInitParams
}

type ClusterBootstrapper struct {
	*Params
	PhasedExecutionContext phases.DefaultPhasedExecutionContext
	initializeNewAgent     bool
	// TODO(dhctl-for-commander): pass stateCache externally using params as in Destroyer, this variable will be unneeded then
	lastState phases.DhctlState
	logger    log.Logger
}

func NewClusterBootstrapper(params *Params) *ClusterBootstrapper {
	if app.ProgressFilePath != "" {
		params.OnProgressFunc = phases.WriteProgress(app.ProgressFilePath)
	}

	logger := params.Logger
	if govalue.IsNil(logger) {
		logger = log.GetDefaultLogger()
	}

	return &ClusterBootstrapper{
		Params: params,
		PhasedExecutionContext: phases.NewDefaultPhasedExecutionContext(
			phases.OperationBootstrap, params.OnPhaseFunc, params.OnProgressFunc,
		),
		lastState: params.InitialState,
		logger:    logger,
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

func (b *ClusterBootstrapper) getCleanupFunc(ctx context.Context, metaConfig *config.MetaConfig) (func(), error) {
	if b.InfrastructureContext == nil {
		b.logger.LogDebugF("InfrastructureContext is nil. Skip cleanup.\n")
		return func() {}, nil
	}

	provider, err := b.InfrastructureContext.CloudProviderGetter()(ctx, metaConfig)
	if err != nil {
		return nil, err
	}

	return func() {
		err = provider.Cleanup()
		if err != nil {
			b.Logger.LogErrorF("Cannot cleanup provider: %v\n", err)
		}
	}, nil
}

func (b *ClusterBootstrapper) Bootstrap(ctx context.Context) error {
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
	metaConfig, err := config.LoadConfigFromFile(
		ctx,
		app.ConfigPaths,
		infrastructureprovider.MetaConfigPreparatorProvider(
			infrastructureprovider.NewPreparatorProviderParams(b.logger),
		),
	)
	if err != nil {
		return err
	}

	log.DebugLn("MetaConfig was loaded")

	// Check if static cluster without ssh-host
	if metaConfig.IsStatic() && len(app.SSHHosts) == 0 {
		fd := int(os.Stdin.Fd())
		isTerminal := term.IsTerminal(fd)

		if isTerminal {
			confirmation := input.NewConfirmation().
				WithMessage("Do you really want to bootstrap the cluster on the current host?")
			if !confirmation.Ask() {
				return fmt.Errorf("Bootstrap cancelled by user")
			}
		} else {
			return fmt.Errorf("Static cluster bootstrap requires --ssh-host option when not running in terminal. Please use --ssh-host option or pass --connection-config with SSHHost resource to bootstrap the cluster")
		}
	}

	providerGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
		TmpDir:           b.TmpDir,
		AdditionalParams: cloud.ProviderAdditionalParams{},
		Logger:           b.logger,
		IsDebug:          b.IsDebug,
	})

	b.InfrastructureContext = infrastructure.NewContextWithProvider(providerGetter, b.logger)

	if govalue.IsNil(b.Params.NodeInterface) {
		log.DebugLn("NodeInterface is nil")
		if len(app.SSHHosts) == 0 && metaConfig.IsStatic() {
			log.DebugLn("Hosts empty and static cluster. Use local interface")
			b.Params.NodeInterface = local.NewNodeInterface()
		} else {
			sshClient, err := sshclient.NewClientFromFlags()
			if err != nil {
				return err
			}

			// do it for get ssh
			if err := sshClient.OnlyPreparePrivateKeys(); err != nil {
				return err
			}

			if metaConfig.IsStatic() {
				// aks bastion pass for SSH Client Dial() with password auth
				if err := terminal.AskBastionPassword(); err != nil {
					return err
				}
				// ask become pass for SSH Client Dial() with password auth
				if err := terminal.AskBecomePassword(); err != nil {
					return err
				}
				if err := sshClient.Start(); err != nil {
					return fmt.Errorf("unable to start ssh client: %w", err)
				}
			}

			log.DebugF("Hosts is %v empty; static cluster is %v. Use ssh\n", len(app.SSHHosts), metaConfig.IsStatic())
			b.Params.NodeInterface = ssh.NewNodeInterfaceWrapper(sshClient)
		}
	}

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

	printBanner()

	clusterUUID, err := generateClusterUUID(stateCache)
	if err != nil {
		return err
	}
	metaConfig.UUID = clusterUUID

	metaConfig.ResourceManagementTimeout = app.ResourceManagementTimeout

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
	if err := preflightChecker.Global(ctx); err != nil {
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

	cleanup, err := b.getCleanupFunc(ctx, metaConfig)
	if err != nil {
		return err
	}

	defer cleanup()

	if metaConfig.ClusterType == config.CloudClusterType {
		err = preflightChecker.Cloud(ctx)
		if err != nil {
			return err
		}
		err = log.Process("bootstrap", "Cloud infrastructure", func() error {
			baseRunner, err := b.InfrastructureContext.GetBootstrapBaseInfraRunner(ctx, metaConfig, stateCache)
			if err != nil {
				return err
			}

			baseOutputs, err := infrastructure.ApplyPipeline(ctx, baseRunner, "Kubernetes cluster", infrastructure.GetBaseInfraResult)
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
			masterRunner, err := b.Params.InfrastructureContext.GetBootstrapNodeRunner(ctx, metaConfig, stateCache, infrastructure.BootstrapNodeRunnerOptions{
				NodeName:        masterNodeName,
				NodeGroupStep:   infrastructure.MasterNodeStep,
				NodeGroupName:   "master",
				NodeIndex:       0,
				NodeCloudConfig: "",
				RunnerLogger:    log.GetDefaultLogger(),
			})
			if err != nil {
				return err
			}

			masterOutputs, err := infrastructure.ApplyPipeline(ctx, masterRunner, masterNodeName, infrastructure.GetMasterNodeResult)
			if err != nil {
				return err
			}

			log.DebugLn("First control-plane node was created")

			deckhouseInstallConfig.CloudDiscovery = baseOutputs.CloudDiscovery
			deckhouseInstallConfig.InfrastructureState = baseOutputs.InfrastructureState

			if wrapper, ok := b.NodeInterface.(*ssh.NodeInterfaceWrapper); ok {
				sshClient := wrapper.Client()
				if baseOutputs.BastionHost != "" {
					sshClient.Session().BastionHost = baseOutputs.BastionHost
					SaveBastionHostToCache(baseOutputs.BastionHost)
				}
				sshClient.Session().SetAvailableHosts([]session.Host{{Host: masterOutputs.MasterIPForSSH, Name: masterNodeName}})
				// aks bastion pass for SSH Client Dial() with password auth
				if err := terminal.AskBastionPassword(); err != nil {
					return err
				}
				// ask become pass for SSH Client Dial() with password auth
				if err := terminal.AskBecomePassword(); err != nil {
					return err
				}
				if err := sshClient.Start(); err != nil {
					return fmt.Errorf("unable to start ssh client: %w", err)
				}
			}

			nodeIP = masterOutputs.NodeInternalIP
			devicePath = masterOutputs.KubeDataDevicePath

			deckhouseInstallConfig.NodesInfrastructureState = make(map[string][]byte)
			deckhouseInstallConfig.NodesInfrastructureState[masterNodeName] = masterOutputs.InfrastructureState

			masterAddressesForSSH[masterNodeName] = masterOutputs.MasterIPForSSH
			SaveMasterHostsToCache(masterAddressesForSSH)
			return nil
		})
		if err != nil {
			return err
		}
	} else {
		err = preflightChecker.Static(ctx)
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
			if sshClient.Session().BastionHost != "" {
				SaveBastionHostToCache(sshClient.Session().BastionHost)
			}

			SaveMasterHostsToCache(map[string]string{
				"first-master": sshClient.Session().Host(),
			})
		}
	}

	// next parse and check resources
	// do it after bootstrap cloud because resources can be template
	// and we want to fail immediately if template has errors
	var resourcesToCreateBeforeDeckhouseBootstrap template.Resources
	var resourcesToCreateAfterDeckhouseBootstrap template.Resources
	if metaConfig.ResourcesYAML != "" {
		parsedResources, err := template.ParseResourcesContent(metaConfig.ResourcesYAML, resourcesTemplateData)
		if err != nil {
			return err
		}

		before, after := splitResourcesOnPreAndPostDeckhouseInstall(parsedResources)

		resourcesToCreateBeforeDeckhouseBootstrap = before
		resourcesToCreateAfterDeckhouseBootstrap = after
	}

	if shouldStop, err := b.PhasedExecutionContext.SwitchPhase(phases.RegistryPackagesProxyPhase, false, stateCache, nil); err != nil {
		return err
	} else if shouldStop {
		return nil
	}

	if wrapper, ok := b.NodeInterface.(*ssh.NodeInterfaceWrapper); ok {
		if err := WaitForSSHConnectionOnMaster(ctx, wrapper.Client()); err != nil {
			return fmt.Errorf("failed to wait for SSH connection on master: %v", err)
		}
	}

	if metaConfig.ClusterType == config.CloudClusterType {
		err = preflightChecker.PostCloud(ctx)
		if err != nil {
			return err
		}
	}

	if shouldStop, err := b.PhasedExecutionContext.SwitchPhase(phases.ExecuteBashibleBundlePhase, false, stateCache, nil); err != nil {
		return err
	} else if shouldStop {
		return nil
	}

	if err := RunBashiblePipeline(ctx, b.NodeInterface, metaConfig, nodeIP, devicePath, b.CommanderMode); err != nil {
		return err
	}

	if shouldStop, err := b.PhasedExecutionContext.SwitchPhase(phases.InstallDeckhousePhase, false, stateCache, nil); err != nil {
		return err
	} else if shouldStop {
		return nil
	}

	kubeCl, err := kubernetes.ConnectToKubernetesAPI(ctx, b.NodeInterface)
	if err != nil {
		return err
	}

	b.PhasedExecutionContext.CompleteSubPhase(phases.InstallDeckhouseSubPhaseConnect)

	installDeckhouseResult, err := InstallDeckhouse(ctx, kubeCl, deckhouseInstallConfig, func() error {
		return createResources(ctx, kubeCl, resourcesToCreateBeforeDeckhouseBootstrap, metaConfig, nil, true)
	})
	if err != nil {
		return err
	}

	b.PhasedExecutionContext.CompleteSubPhase(phases.InstallDeckhouseSubPhaseInstall)

	err = WaitForFirstMasterNodeBecomeReady(ctx, kubeCl)
	if err != nil {
		return err
	}

	b.PhasedExecutionContext.CompleteSubPhase(phases.InstallDeckhouseSubPhaseWait)

	if metaConfig.ClusterType == config.CloudClusterType {
		if shouldStop, err := b.PhasedExecutionContext.SwitchPhase(phases.InstallAdditionalMastersAndStaticNodes, true, stateCache, nil); err != nil {
			return err
		} else if shouldStop {
			return nil
		}

		localBootstraper := func(action func() error) error {
			if b.CommanderMode {
				return action()
			}
			return lock.NewInLockLocalRunner(kubernetes.NewSimpleKubeClientGetter(kubeCl), "local-bootstraper").
				Run(ctx, action)
		}

		err := localBootstraper(func() error {
			return bootstrapAdditionalNodesForCloudCluster(ctx, kubeCl, metaConfig, masterAddressesForSSH, b.InfrastructureContext)
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

	if err := controlplane.NewManagerReadinessChecker(kubernetes.NewSimpleKubeClientGetter(kubeCl)).IsReadyAll(ctx); err != nil {
		return err
	}

	err = createResources(ctx, kubeCl, resourcesToCreateAfterDeckhouseBootstrap, metaConfig, installDeckhouseResult, false)
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

		if err := postScriptExecutor.Execute(ctx); err != nil {
			return err
		}
	}

	if shouldStop, err := b.PhasedExecutionContext.SwitchPhase(phases.FinalizationPhase, false, stateCache, nil); err != nil {
		return err
	} else if shouldStop {
		return nil
	}

	if err := RunPostInstallTasks(ctx, kubeCl, installDeckhouseResult); err != nil {
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
				fakeSession := wrapper.Client().Session().Copy()
				fakeSession.SetAvailableHosts([]session.Host{{Host: address, Name: nodeName}})
				log.InfoF("%s | %s\n", nodeName, fakeSession.String())
			}

			return nil
		})
	}

	return b.PhasedExecutionContext.CompletePhaseAndPipeline(stateCache, nil)
}

// TODO(dhctl-for-commander): pass stateCache externally using params as in Destroyer, this method will be unneeded then
func (b *ClusterBootstrapper) GetLastState() phases.DhctlState {
	if b.lastState != nil {
		return b.lastState
	} else {
		return b.PhasedExecutionContext.GetLastState()
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

func bootstrapAdditionalNodesForCloudCluster(ctx context.Context, kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, masterAddressesForSSH map[string]string, infrastructureContext *infrastructure.Context) error {
	if err := BootstrapAdditionalMasterNodes(ctx, kubeCl, metaConfig, masterAddressesForSSH, infrastructureContext); err != nil {
		return err
	}

	terraNodeGroups := metaConfig.GetTerraNodeGroups()
	bootstrapAdditionalTerraNodeGroups := BootstrapTerraNodes
	if operations.IsSequentialNodesBootstrap() || metaConfig.ProviderName == "vcd" {
		// vcd doesn't support parrallel creating nodes in same vapp
		// https://github.com/vmware/terraform-provider-vcd/issues/530
		bootstrapAdditionalTerraNodeGroups = operations.BootstrapSequentialTerraNodes
	}

	if err := bootstrapAdditionalTerraNodeGroups(ctx, kubeCl, metaConfig, terraNodeGroups, infrastructureContext); err != nil {
		return err
	}

	return log.Process("bootstrap", "Waiting for Node Groups are ready", func() error {
		ngs := map[string]int{"master": metaConfig.MasterNodeGroupSpec.Replicas}
		for _, ng := range terraNodeGroups {
			if ng.Replicas > 0 {
				ngs[ng.Name] = ng.Replicas
			}
		}
		if err := entity.WaitForNodesBecomeReady(ctx, kubeCl, ngs); err != nil {
			return err
		}

		return nil
	})
}
func splitResourcesOnPreAndPostDeckhouseInstall(resourcesToCreate template.Resources) (before template.Resources, after template.Resources) {
	before = make(template.Resources, 0, len(resourcesToCreate))
	after = make(template.Resources, 0, len(resourcesToCreate))

	for _, resource := range resourcesToCreate {
		annotations := resource.Object.GetAnnotations()
		if annotations == nil || annotations["dhctl.deckhouse.io/bootstrap-resource-place"] != "before-deckhouse" {
			log.DebugF("Add resource %s - %s to after queue\n", resource.String(), resource.Object.GetName())
			after = append(after, resource)
			continue
		}

		log.DebugF("Add resource %s - %s to before queue\n", resource.String(), resource.Object.GetName())
		before = append(before, resource)
	}

	return before, after
}

func createResources(ctx context.Context, kubeCl *client.KubernetesClient, resourcesToCreate template.Resources, metaConfig *config.MetaConfig, result *InstallDeckhouseResult, skipChecks bool) error {
	tasks := make([]actions.ModuleConfigTask, 0)
	if result != nil {
		log.WarnLn("\nThe installation has completed successfully.\nTo finalize bootstraping please add at least one non-master node or remove taints from your master node (if a single node installation).\n")

		tasks = result.ManifestResult.WithResourcesMCTasks

		if len(resourcesToCreate) == 0 {
			for _, task := range tasks {
				return retry.NewLoop(task.Title, 60, 5*time.Second).RunContext(ctx, func() error {
					return task.Do(kubeCl)
				})
			}

			return nil
		}
	}

	if len(resourcesToCreate) == 0 {
		return nil
	}

	return log.Process("bootstrap", "Create Resources", func() error {
		checkers := make([]resources.Checker, 0)
		if !skipChecks {
			var err error
			checkers, err = resources.GetCheckers(kubeCl, resourcesToCreate, metaConfig)
			if err != nil {
				return err
			}

		}

		return resources.CreateResourcesLoop(ctx, kubeCl, resourcesToCreate, checkers, tasks)
	})
}

func setWithRestore[T any](target *T, newValue T) func() {
	oldValue := *target
	*target = newValue
	return func() {
		*target = oldValue
	}
}
