// Copyright 2026 Flant JSC
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
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/name212/govalue"
	otattribute "go.opentelemetry.io/otel/attribute"

	libcon "github.com/deckhouse/lib-connection/pkg"
	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	dhctllog "github.com/deckhouse/lib-dhctl/pkg/log"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/directoryconfig"
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
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/suites"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/helper"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
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

// Params carries everything ClusterBootstrapper needs that is not derived from
// the cluster configuration files themselves. The Options field replaces the
// previous package-level dhctl/pkg/app globals; callers must populate it with a
// fresh *options.Options per operation to avoid sharing state between
// concurrent requests.
type Params struct {
	SSHProviderInitializer     *providerinitializer.SSHProviderInitializer
	KubeProvider               libcon.KubeProvider
	InitialState               phases.DhctlState
	ResetInitialState          bool
	DisableBootstrapClearCache bool
	OnPhaseFunc                phases.DefaultOnPhaseFunc
	OnProgressFunc             phases.OnProgressFunc
	CommanderMode              bool
	CommanderUUID              uuid.UUID
	InfrastructureContext      *infrastructure.Context

	TmpDir string
	// todo refact to logger provider
	Logger  log.Logger
	IsDebug bool

	// Options is the per-operation parsed configuration. Required.
	Options *options.Options

	DirectoryConfig *directoryconfig.DirectoryConfig

	*client.KubernetesInitParams
}

type ClusterBootstrapper struct {
	*Params
	PhasedExecutionContext phases.DefaultPhasedExecutionContext
	// TODO(dhctl-for-commander): pass stateCache externally using params as in Destroyer, this variable will be unneeded then
	lastState      phases.DhctlState
	logger         log.Logger
	loggerProvider dhctllog.LoggerProvider
}

func NewClusterBootstrapper(params *Params) *ClusterBootstrapper {
	if params.Options != nil && params.Options.Global.ProgressFilePath != "" {
		params.OnProgressFunc = phases.WriteProgress(params.Options.Global.ProgressFilePath)
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
		lastState:      params.InitialState,
		logger:         logger,
		loggerProvider: log.ExternalLoggerProvider(logger),
	}
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
	ctx, span := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap")
	defer span.End()

	masterAddressesForSSH := make(map[string]string)

	if b.Options.Bootstrap.PostBootstrapScriptPath != "" {
		log.DebugF("Have post bootstrap script: %s\n", b.Options.Bootstrap.PostBootstrapScriptPath)
		if err := ValidateScriptFile(ctx, b.Options.Bootstrap.PostBootstrapScriptPath); err != nil {
			return err
		}
	}

	if b.Options.Bootstrap.ResourcesPath != "" {
		log.WarnLn("--resources flag is deprecated. Please use --config flag multiple repeatedly for logical resources separation")
		b.Options.Global.ConfigPaths = append(b.Options.Global.ConfigPaths, b.Options.Bootstrap.ResourcesPath)
	}

	ctx, configSpan := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.LoadConfig")
	defer configSpan.End()

	// first, parse and check cluster config
	preparatorParams := infrastructureprovider.NewPreparatorProviderParams(b.logger)
	preparatorParams.WithPhaseBootstrap()
	preparatorParams.WithPreflightChecks(infrastructureprovider.PreflightChecks{
		DVPValidateKubeAPI: true,
	})
	metaConfig, err := config.LoadConfigFromFile(
		ctx,
		b.Options.Global.ConfigPaths,
		infrastructureprovider.MetaConfigPreparatorProvider(preparatorParams),
		b.DirectoryConfig,
		config.ValidateOptionValidateExtensions(true),
	)
	if err != nil {
		return err
	}

	log.DebugLn("MetaConfig was loaded")

	b.PhasedExecutionContext.SetClusterConfig(phases.ClusterConfig{ClusterType: metaConfig.ClusterType})

	// Check if static cluster without ssh-host
	if metaConfig.IsStatic() && !b.SSHProviderInitializer.CheckHosts() {
		if input.IsTerminal() {
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
		DownloadDir:      b.Options.Global.DownloadDir,
		AdditionalParams: cloud.ProviderAdditionalParams{},
		Logger:           b.logger,
		IsDebug:          b.IsDebug,
	})

	b.InfrastructureContext = infrastructure.NewContextWithProvider(providerGetter, b.logger).
		WithUseTfCache(b.Options.Cache.UseTfCache).
		WithDebug(b.Options.Global.IsDebug)

	// next init cache
	cachePath := metaConfig.CachePath()
	if err = cache.InitWithOptions(ctx, cachePath, cache.CacheOptions{InitialState: b.InitialState, ResetInitialState: b.ResetInitialState, Cache: b.Options.Cache}); err != nil {
		// TODO: it's better to ask for confirmation here
		return fmt.Errorf(cacheMessage, cachePath, err)
	}

	stateCache := cache.Global()
	configHash := state.ConfigHash(b.Options.Global.ConfigPaths)

	if b.Options.Cache.DropCache {
		stateCache.Clean(ctx)
		stateCache.Delete(ctx, state.TombstoneKey)
		log.DebugLn("Cache was dropped")
	}

	if err := b.PhasedExecutionContext.InitPipeline(ctx, stateCache); err != nil {
		return err
	}
	// TODO(dhctl-for-commander): pass stateCache externally using params as in Destroyer, this variable will be unneeded then
	b.lastState = nil
	defer func() {
		_ = b.PhasedExecutionContext.Finalize(ctx, stateCache)
	}()

	printBanner()

	clusterUUID, err := generateClusterUUID(ctx, stateCache)
	if err != nil {
		return err
	}
	metaConfig.UUID = clusterUUID

	metaConfig.ResourceManagementTimeout = b.Options.Cache.ResourceManagementTimeout

	deckhouseInstallConfig, err := config.PrepareDeckhouseInstallConfig(ctx, metaConfig)
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

	if shouldStop, err := b.PhasedExecutionContext.StartPhase(ctx, phases.BaseInfraPhase, true, stateCache); err != nil {
		return err
	} else if shouldStop {
		return nil
	}

	_, baseInfraSpan := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.BaseInfra")
	defer baseInfraSpan.End()

	var nodeIP string
	var devicePath string
	var resourcesTemplateData map[string]interface{}

	cleanup, err := b.getCleanupFunc(ctx, metaConfig)
	if err != nil {
		return err
	}

	defer cleanup()

	globalPreflightSuite := suites.NewGlobalSuite(suites.GlobalDeps{
		MetaConfig:    metaConfig,
		InstallConfig: deckhouseInstallConfig,
		BuildInfo:     b.Options.BuildInfo,
	})

	if metaConfig.ClusterType == config.CloudClusterType {
		ctx, cloudPreflightSpan := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.CloudPreflight")
		defer cloudPreflightSpan.End()

		sshProvider, err := b.SSHProviderInitializer.GetSSHProvider(ctx)
		if err != nil {
			if !strings.Contains(err.Error(), "failed to get hosts from cache") {
				return err
			}
		}

		cloudPreflightSuite := suites.NewCloudSuite(suites.CloudDeps{
			InstallConfig: deckhouseInstallConfig,
			MetaConfig:    metaConfig,
		})
		postCloudPreflightSuite := suites.NewPostCloudSuite(suites.PostCloudDeps{
			MetaConfig:  metaConfig,
			SSHProvider: sshProvider,
			LegacyMode:  b.SSHProviderInitializer.IsLegacyMode(),
		})

		preflightRunner := preflight.New(globalPreflightSuite, cloudPreflightSuite, postCloudPreflightSuite)
		preflightRunner.UseCache(bootstrapState)
		preflightRunner.SetCacheSalt(configHash)
		preflightRunner.DisableChecks(b.Options.Preflight.DisabledChecks()...)
		if err := preflightRunner.Run(ctx, preflight.PhasePreInfra); err != nil {
			return err
		}

		cloudPreflightSpan.End()

		err = log.ProcessCtx(ctx, "bootstrap", "Cloud infrastructure", func(ctx context.Context) error {
			ctx, span := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.CloudInfra")
			defer span.End()

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

			// providers should be reinitialized here
			baseSettings := b.SSHProviderInitializer.GetSettings()
			connectionConfig := b.SSHProviderInitializer.GetConfig()

			if baseOutputs.BastionHost != "" {
				connectionConfig.Config.BastionHost = baseOutputs.BastionHost
				SaveBastionHostToCache(ctx, baseOutputs.BastionHost)
			}

			connectionConfig.Hosts = append(connectionConfig.Hosts, sshconfig.Host{Host: masterOutputs.MasterIPForSSH})

			sshProviderInitializer := providerinitializer.NewSSHProviderInitializer(baseSettings, connectionConfig)
			b.SSHProviderInitializer = sshProviderInitializer
			b.KubeProvider = sshProviderInitializer.GetKubeProvider(ctx)

			nodeIP = masterOutputs.NodeInternalIP
			devicePath = masterOutputs.KubeDataDevicePath

			deckhouseInstallConfig.NodesInfrastructureState = make(map[string][]byte)
			deckhouseInstallConfig.NodesInfrastructureState[masterNodeName] = masterOutputs.InfrastructureState

			masterAddressesForSSH[masterNodeName] = masterOutputs.MasterIPForSSH
			state.SaveMasterHostsToCache(ctx, stateCache, masterAddressesForSSH)
			return nil
		})
		if err != nil {
			return err
		}

		if err := preflightRunner.Run(ctx, preflight.PhasePostInfra); err != nil {
			return err
		}
	} else {
		ctx, staticPreflightSpan := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.StaticPreflight")
		defer staticPreflightSpan.End()

		staticPreflightSuite, err := suites.NewStaticSuite(suites.StaticDeps{
			SSHProviderInitializer: b.SSHProviderInitializer,
			MetaConfig:             metaConfig,
			LegacyMode:             b.SSHProviderInitializer.IsLegacyMode(),
		}, ctx)
		if err != nil {
			return err
		}

		preflightRunner := preflight.New(globalPreflightSuite, staticPreflightSuite)
		preflightRunner.UseCache(bootstrapState)
		preflightRunner.SetCacheSalt(configHash)
		preflightRunner.DisableChecks(b.Options.Preflight.DisabledChecks()...)

		if err := preflightRunner.Run(ctx, preflight.PhasePreInfra); err != nil {
			return err
		}

		if err = preflightRunner.Run(ctx, preflight.PhasePostInfra); err != nil {
			return err
		}

		var static struct {
			NodeIP string `json:"nodeIP"`
		}
		_ = json.Unmarshal(metaConfig.ClusterConfig["static"], &static)
		nodeIP = static.NodeIP

		if b.SSHProviderInitializer.CheckHosts() {
			connectionConfig := b.SSHProviderInitializer.GetConfig()
			if connectionConfig.Config.BastionHost != "" {
				SaveBastionHostToCache(ctx, connectionConfig.Config.BastionHost)
			}

			state.SaveMasterHostsToCache(ctx, stateCache, map[string]string{
				"first-master": connectionConfig.Hosts[0].Host,
			})
		}

		staticPreflightSpan.End()
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

	baseInfraSpan.End()

	if shouldStop, err := b.PhasedExecutionContext.SwitchPhase(ctx, phases.RegistryPackagesProxyPhase, false, stateCache, nil); err != nil {
		return err
	} else if shouldStop {
		return nil
	}
	ctx, registryPackagesProxySpan := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.RegistryPackagesProxy")
	defer registryPackagesProxySpan.End()

	if b.SSHProviderInitializer.CheckHosts() {
		sshProvider, err := b.SSHProviderInitializer.GetSSHProvider(ctx)
		if err != nil {
			return err
		}

		sshClient, err := sshProvider.Client(ctx)
		if err != nil {
			return err
		}

		if err := WaitForSSHConnectionOnMaster(ctx, sshClient); err != nil {
			return fmt.Errorf("failed to wait for SSH connection on master: %v", err)
		}
	}

	registryPackagesProxySpan.End()

	if shouldStop, err := b.PhasedExecutionContext.SwitchPhase(ctx, phases.ExecuteBashibleBundlePhase, false, stateCache, nil); err != nil {
		return err
	} else if shouldStop {
		return nil
	}

	ctx, bashibleBundleSpan := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.BashibleBundle")
	defer bashibleBundleSpan.End()

	nodeInterface, err := helper.GetNodeInterface(ctx, b.SSHProviderInitializer, b.SSHProviderInitializer.GetSettings())
	if err != nil {
		return fmt.Errorf("Could not get NodeInterface: %w", err)
	}

	err = RunBashiblePipeline(ctx, &BashiblePipelineParams{
		Node:           nodeInterface,
		NodeIP:         nodeIP,
		DevicePath:     devicePath,
		MetaConfig:     metaConfig,
		CommanderMode:  b.CommanderMode,
		DirsConfig:     b.DirectoryConfig,
		LoggerProvider: b.loggerProvider,
	})

	if err != nil {
		return err
	}

	bashibleBundleSpan.End()

	if shouldStop, err := b.PhasedExecutionContext.SwitchPhase(ctx, phases.InstallDeckhousePhase, false, stateCache, nil); err != nil {
		return err
	} else if shouldStop {
		return nil
	}

	ctx, installDeckhouseSpan := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.InstallDeckhouse")
	defer installDeckhouseSpan.End()

	kubeCl, err := b.KubeProvider.Client(ctx)
	if err != nil {
		return err
	}

	b.PhasedExecutionContext.CompleteSubPhase(phases.InstallDeckhouseSubPhaseConnect)

	installParams := InstallDeckhouseParams{
		BeforeDeckhouseTask: func() error {
			return createResources(
				ctx,
				&client.KubernetesClient{KubeClient: kubeCl},
				resourcesToCreateBeforeDeckhouseBootstrap,
				nil,
				true,
				b.Options.Bootstrap.ResourcesTimeout,
			)
		},
		State:            bootstrapState,
		DeckhouseTimeout: b.Options.Bootstrap.DeckhouseTimeout,
	}

	installDeckhouseResult, err := InstallDeckhouse(ctx, &client.KubernetesClient{KubeClient: kubeCl}, deckhouseInstallConfig, installParams)
	if err != nil {
		return err
	}

	b.PhasedExecutionContext.CompleteSubPhase(phases.InstallDeckhouseSubPhaseInstall)

	err = WaitForFirstMasterNodeBecomeReady(ctx, &client.KubernetesClient{KubeClient: kubeCl})
	if err != nil {
		return err
	}

	b.PhasedExecutionContext.CompleteSubPhase(phases.InstallDeckhouseSubPhaseWait)

	installDeckhouseSpan.End()

	if metaConfig.ClusterType == config.CloudClusterType {
		if shouldStop, err := b.PhasedExecutionContext.SwitchPhase(ctx, phases.InstallAdditionalMastersAndStaticNodes, true, stateCache, nil); err != nil {
			return err
		} else if shouldStop {
			return nil
		}

		ctx, additionalNodesSpan := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.AdditionalNodes")
		defer additionalNodesSpan.End()

		localBootstraper := func(action func() error) error {
			if b.CommanderMode {
				return action()
			}

			return lock.NewInLockLocalRunner(
				ctx,
				kubernetes.NewSimpleKubeClientGetter(&client.KubernetesClient{KubeClient: kubeCl}),
				"local-bootstraper",
				b.Options.SSH.User,
			).Run(ctx, action)
		}

		err := localBootstraper(func() error {
			return bootstrapAdditionalNodesForCloudCluster(
				ctx,
				&client.KubernetesClient{KubeClient: kubeCl},
				metaConfig,
				masterAddressesForSSH,
				b.InfrastructureContext,
			)
		})
		if err != nil {
			return err
		}

		additionalNodesSpan.End()
	}

	if shouldStop, err := b.PhasedExecutionContext.SwitchPhase(ctx, phases.CreateResourcesPhase, false, stateCache, nil); err != nil {
		return err
	} else if shouldStop {
		return nil
	}

	if err := controlplane.NewManagerReadinessChecker(kubernetes.NewSimpleKubeClientGetter(&client.KubernetesClient{KubeClient: kubeCl})).IsReadyAll(ctx); err != nil {
		return err
	}

	err = createResources(
		ctx,
		&client.KubernetesClient{KubeClient: kubeCl},
		resourcesToCreateAfterDeckhouseBootstrap,
		installDeckhouseResult,
		false,
		b.Options.Bootstrap.ResourcesTimeout,
	)
	if err != nil {
		return err
	}

	if shouldStop, err := b.PhasedExecutionContext.SwitchPhase(ctx, phases.ExecPostBootstrapPhase, false, stateCache, nil); err != nil {
		return err
	} else if shouldStop {
		return nil
	}

	if b.SSHProviderInitializer.CheckHosts() && b.Options.Bootstrap.PostBootstrapScriptPath != "" {
		ctx, postBootstrapSpan := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.PostBootstrap")
		defer postBootstrapSpan.End()

		postScriptExecutor := NewPostBootstrapScriptExecutor(b.SSHProviderInitializer, b.Options.Bootstrap.PostBootstrapScriptPath, bootstrapState).
			WithTimeout(b.Options.Bootstrap.PostBootstrapScriptTimeout)

		if err := postScriptExecutor.Execute(ctx); err != nil {
			return err
		}

		postBootstrapSpan.End()
	}

	if shouldStop, err := b.PhasedExecutionContext.SwitchPhase(ctx, phases.FinalizationPhase, false, stateCache, nil); err != nil {
		return err
	} else if shouldStop {
		return nil
	}

	if err := RunPostInstallTasks(ctx, &client.KubernetesClient{KubeClient: kubeCl}, installDeckhouseResult); err != nil {
		return err
	}

	if !b.DisableBootstrapClearCache {
		_ = log.ProcessCtx(ctx, "bootstrap", "Clear cache", func(ctx context.Context) error {
			cache.Global().CleanWithExceptions(
				ctx,
				state.MasterHostsCacheKey,
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
			sshProvider, err := b.SSHProviderInitializer.GetSSHProvider(ctx)
			if err != nil {
				return err
			}

			sshClient, err := sshProvider.Client(ctx)
			if err != nil {
				return err
			}
			for nodeName, address := range masterAddressesForSSH {
				fakeSession := sshClient.Session().Copy()
				fakeSession.SetAvailableHosts([]session.Host{{Host: address, Name: nodeName}})
				log.InfoF("%s | %s\n", nodeName, fakeSession.String())
			}

			return nil
		})
	}

	return b.PhasedExecutionContext.CompletePhaseAndPipeline(ctx, stateCache, nil)
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

func generateClusterUUID(ctx context.Context, stateCache state.Cache) (string, error) {
	var clusterUUID string

	return clusterUUID, log.ProcessCtx(ctx, "bootstrap", "Cluster UUID", func(ctx context.Context) error {
		ok, err := stateCache.InCache(ctx, "uuid")
		if err != nil {
			return err
		}

		if !ok {
			genClusterUUID, err := uuid.NewRandom()
			if err != nil {
				return fmt.Errorf("can't create cluster UUID: %v", err)
			}

			clusterUUID = genClusterUUID.String()
			err = stateCache.Save(ctx, "uuid", []byte(clusterUUID))
			if err != nil {
				return err
			}
			log.InfoF("Generated cluster UUID: %s\n", clusterUUID)
		} else {
			clusterUUIDBytes, err := stateCache.Load(ctx, "uuid")
			if err != nil {
				return err
			}
			clusterUUID = string(clusterUUIDBytes)
			log.InfoF("Cluster UUID from cache: %s\n", clusterUUID)
		}
		return nil
	})
}

func bootstrapAdditionalNodesForCloudCluster(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	metaConfig *config.MetaConfig,
	masterAddressesForSSH map[string]string,
	infrastructureContext *infrastructure.Context,
) error {
	if err := BootstrapAdditionalMasterNodes(ctx, kubeCl, metaConfig, masterAddressesForSSH, infrastructureContext, cache.Global()); err != nil {
		return err
	}

	terraNodeGroups := metaConfig.GetTerraNodeGroups()
	bootstrapAdditionalTerraNodeGroups := BootstrapTerraNodes
	if operations.IsSequentialNodesBootstrap(metaConfig) {
		bootstrapAdditionalTerraNodeGroups = operations.BootstrapSequentialTerraNodes
	}

	if err := bootstrapAdditionalTerraNodeGroups(ctx, kubeCl, metaConfig, terraNodeGroups, infrastructureContext); err != nil {
		return err
	}

	return log.ProcessCtx(ctx, "bootstrap", "Waiting for Node Groups are ready", func(ctx context.Context) error {
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

func splitResourcesOnPreAndPostDeckhouseInstall(resourcesToCreate template.Resources) (template.Resources, template.Resources) {
	before := make(template.Resources, 0, len(resourcesToCreate))
	after := make(template.Resources, 0, len(resourcesToCreate))

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

func createResources(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	resourcesToCreate template.Resources,
	result *InstallDeckhouseResult,
	skipChecks bool,
	timeout time.Duration,
) error {
	ctx, span := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.createResources")
	defer span.End()

	tasks := make([]actions.ModuleConfigTask, 0)
	if result != nil {
		log.WarnLn("\nThe installation has completed successfully.\nTo finalize bootstraping please add at least one non-master node or remove taints from your master node (if a single node installation).\n")

		tasks = result.ManifestResult.WithResourcesMCTasks

		span.SetAttributes(otattribute.Int("tasks_count", len(tasks)))

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

	span.SetAttributes(otattribute.Int("resources_count", len(resourcesToCreate)))

	return log.ProcessCtx(ctx, "bootstrap", "Create Resources", func(ctx context.Context) error {
		var err error
		checkers := make([]resources.Checker, 0)
		if !skipChecks {
			checkers, err = resources.GetCheckers(kubeCl, resourcesToCreate, nil)
			if err != nil {
				return err
			}
		}

		return resources.CreateResourcesLoop(ctx, kubeCl, resourcesToCreate, checkers, tasks, timeout)
	})
}
