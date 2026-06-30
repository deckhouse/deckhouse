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
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	otattribute "go.opentelemetry.io/otel/attribute"

	libcon "github.com/deckhouse/lib-connection/pkg"
	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	dhctllog "github.com/deckhouse/lib-dhctl/pkg/log"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/resources"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap/registry"
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
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

const (
	bootstrapAbortInvalidCacheMessage = `Create cache %s:
	Error: %v
	The Kubernetes cluster was probably bootstrapped successfully.
	Use the "dhctl destroy" command to delete the cluster.
`
	bootstrapPhaseBaseInfraNonCloudMessage = `It is impossible to create base infrastructure for a non-cloud Kubernetes cluster.
You have to create it manually.
`
	bootstrapAbortCheckMessage = `You will be asked for approval multiple times.
If you are confident in your actions, you can use the flag "--yes-i-am-sane-and-i-understand-what-i-am-doing" to skip approvals.
`
	cacheMessage = `Create cache %s:
	Error: %v

	The Kubernetes cluster was probably bootstrapped successfully.
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

	TmpDir  string
	IsDebug bool

	// Options is the per-operation parsed configuration. Required.
	Options *options.Options

	*client.KubernetesInitParams
}

type ClusterBootstrapper struct {
	*Params
	PhasedExecutionContext phases.DefaultPhasedExecutionContext
	// TODO(dhctl-for-commander): pass stateCache externally using params as in Destroyer, this variable will be unneeded then
	lastState      phases.DhctlState
	loggerProvider dhctllog.LoggerProvider
}

func (b *ClusterBootstrapper) applyCommanderModeConfig(cfg *config.DeckhouseInstaller) {
	if b.CommanderMode {
		// FIXME(dhctl-for-commander): commander uuid currently optional, make it required later
		// if b.CommanderUUID == uuid.Nil {
		//	panic("CommanderUUID required for bootstrap operation in commander mode!")
		// }
		cfg.CommanderMode = b.CommanderMode
		cfg.CommanderUUID = b.CommanderUUID
	}
}

func (b *ClusterBootstrapper) commanderModeAction(action func() error, fallback func() error) error {
	if b.CommanderMode {
		if action != nil {
			return action()
		}
		return nil
	}
	if fallback != nil {
		return fallback()
	}
	return nil
}

func NewClusterBootstrapper(ctx context.Context, params *Params) *ClusterBootstrapper {
	if params.Options != nil && params.Options.Global.ProgressFilePath != "" {
		params.OnProgressFunc = phases.WriteProgress(params.Options.Global.ProgressFilePath)
	}

	return &ClusterBootstrapper{
		Params: params,
		PhasedExecutionContext: phases.NewDefaultPhasedExecutionContext(
			phases.OperationBootstrap, params.OnPhaseFunc, params.OnProgressFunc,
		),
		lastState:      params.InitialState,
		loggerProvider: dhctllog.SimpleLoggerProvider(dhlog.NewLibdhctlAdapter(ctx)),
	}
}

func (b *ClusterBootstrapper) getCleanupFunc(ctx context.Context, metaConfig *config.MetaConfig) (func(), error) {
	if b.InfrastructureContext == nil {
		dhlog.FromContext(ctx).DebugContext(ctx, "InfrastructureContext is nil. Skipping cleanup.")
		return func() {}, nil
	}

	provider, err := b.InfrastructureContext.CloudProviderGetter()(ctx, metaConfig)
	if err != nil {
		return nil, err
	}

	return func() {
		err = provider.Cleanup()
		if err != nil {
			dhlog.FromContext(ctx).ErrorContext(ctx, fmt.Sprintf("Cannot clean up provider: %v", err))
		}
	}, nil
}

type bootstrapContext struct {
	masterAddressesForSSH   map[string]string
	metaConfig              *config.MetaConfig
	stateCache              state.Cache
	configHash              string
	deckhouseInstallConfig  *config.DeckhouseInstaller
	bootstrapState          *State
	nodeIP                  string
	devicePath              string
	resourcesTemplateData   map[string]any
	resourcesToCreateBefore template.Resources
	resourcesToCreateAfter  template.Resources
	installDeckhouseResult  *InstallDeckhouseResult
	cleanup                 func()
	finishProgress          func()
	preflightRunner         *preflight.Preflight
}

func (b *ClusterBootstrapper) Bootstrap(ctx context.Context) error {
	ctx, span := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap")
	defer span.End()

	if b.Options.Bootstrap.PostBootstrapScriptPath != "" {
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Found post-bootstrap script: %s", b.Options.Bootstrap.PostBootstrapScriptPath))
		if err := ValidateScriptFile(ctx, b.Options.Bootstrap.PostBootstrapScriptPath); err != nil {
			return err
		}
	}

	if b.Options.Bootstrap.ResourcesPath != "" {
		dhlog.FromContext(ctx).WarnContext(ctx, "--resources flag is deprecated. Please use the --config flag multiple times for logical resource separation")
		b.Options.Global.ConfigPaths = append(b.Options.Global.ConfigPaths, b.Options.Bootstrap.ResourcesPath)
	}

	// Registry shoud run before LoadConfigFromFile
	registryStop, err := registry.InitFromConfig(
		ctx,
		b.loggerProvider(),
		b.Options.Global.ConfigPaths,
		b.Options.Registry.ImgBundlePath,
	)
	if err != nil {
		return err
	}
	defer registryStop()

	bctx := &bootstrapContext{
		masterAddressesForSSH: make(map[string]string),
	}

	if err := b.bootstrapLoadConfig(ctx, bctx); err != nil {
		return err
	}

	defer func() {
		if err := b.PhasedExecutionContext.Finalize(ctx, bctx.stateCache); err != nil {
			dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("failed to finalize phased execution context: %v", err))
		}
		if bctx.finishProgress != nil {
			bctx.finishProgress()
		}
		if bctx.cleanup != nil {
			bctx.cleanup()
		}
	}()

	phasesToRun := []func(context.Context, *bootstrapContext) error{
		b.bootstrapPreflight,
		b.bootstrapBaseInfra,
		b.bootstrapPostInfraPreflights,
		b.bootstrapKubernetes,
		b.bootstrapDeckhouse,
		b.bootstrapAdditionalNodes,
		b.bootstrapCreateResources,
		b.bootstrapPostBootstrap,
		b.bootstrapFinalize,
	}

	for _, p := range phasesToRun {
		err := p(ctx, bctx)
		if err != nil {
			if err.Error() == "stopped" {
				return nil
			}
			return err
		}
	}

	return nil
}
func (b *ClusterBootstrapper) bootstrapLoadConfig(ctx context.Context, bctx *bootstrapContext) error {
	ctx, configSpan := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.LoadConfig")
	defer configSpan.End()

	// first, parse and check cluster config
	preparatorParams := infrastructureprovider.NewPreparatorProviderParams()
	preparatorParams.WithPhaseBootstrap()
	preparatorParams.WithPreflightChecks(infrastructureprovider.PreflightChecks{
		DVPValidateKubeAPI: true,
	})
	metaConfig, err := config.LoadConfigFromFile(
		ctx,
		b.Options.Global.ConfigPaths,
		infrastructureprovider.MetaConfigPreparatorProvider(preparatorParams),
		&b.Options.Global,
		config.ValidateOptionValidateExtensions(true),
	)
	if err != nil {
		return err
	}

	dhlog.FromContext(ctx).DebugContext(ctx, "MetaConfig was loaded")

	interactive := input.IsTerminal() && !b.Options.Global.ShowProgress
	printBanner(ctx)

	if interactive {
		progressCh, finishProgress := phases.InitProgress(ctx, dhlog.FromContext(ctx), "Bootstrap cluster")
		bctx.finishProgress = finishProgress

		onUpdateFunc := func(progress phases.Progress) error {
			// Non-blocking: the pipeline's deferred Finalize can emit after the consumer has
			// stopped and the channel is no longer drained; never block or panic on it.
			select {
			case progressCh <- progress:
			default:
			}
			return nil
		}

		b.PhasedExecutionContext = phases.NewDefaultPhasedExecutionContext(phases.OperationBootstrap, b.OnPhaseFunc, onUpdateFunc)
	}

	if err := config.ApplyCNIBootstrap(ctx, metaConfig, &b.Options.Global); err != nil {
		return fmt.Errorf("apply cni bootstrap: %w", err)
	}

	b.PhasedExecutionContext.SetClusterConfig(phases.ClusterConfig{ClusterType: metaConfig.ClusterType})

	// Check if static cluster without ssh-host
	if metaConfig.IsStatic() && !b.SSHProviderInitializer.CheckHosts(ctx) {
		if input.IsTerminal() {
			confirmation := input.NewConfirmation().
				WithMessage("Do you really want to bootstrap the cluster on the current host?")
			if !confirmation.Ask() {
				return fmt.Errorf("Bootstrap canceled by user")
			}
		} else {
			return fmt.Errorf("Static cluster bootstrap requires --ssh-host option when not running in terminal. Please use --ssh-host option or pass --connection-config with SSHHost resource to bootstrap the cluster")
		}
	}

	providerGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
		TmpDir:           b.TmpDir,
		GlobalOptions:    &b.Options.Global,
		AdditionalParams: cloud.ProviderAdditionalParams{},
		IsDebug:          b.IsDebug,
	})

	b.InfrastructureContext = infrastructure.NewContextWithProvider(providerGetter).
		WithUseTfCache(b.Options.Cache.UseTfCache).
		WithDebug(b.Options.Global.IsDebug)

	// next init cache
	cachePath := metaConfig.CachePath()
	if err = cache.InitWithOptions(ctx, cachePath, cache.CacheOptions{InitialState: b.InitialState, ResetInitialState: b.ResetInitialState, Cache: b.Options.Cache}); err != nil {
		// TODO: it's better to ask for confirmation here
		return fmt.Errorf(cacheMessage, cachePath, err)
	}

	stateCache := cache.Global()

	if b.Options.Cache.DropCache {
		stateCache.Clean(ctx)
		stateCache.Delete(ctx, state.TombstoneKey)
		dhlog.FromContext(ctx).DebugContext(ctx, "Cache was dropped")
	}

	if err := b.PhasedExecutionContext.InitPipeline(ctx, stateCache); err != nil {
		return err
	}
	// TODO(dhctl-for-commander): pass stateCache externally using params as in Destroyer, this variable will be unneeded then
	b.lastState = nil

	// interactive := input.IsTerminal() && !b.Options.Global.ShowProgress
	// printBanner(ctx)

	// if interactive {
	// 	progressCh, finishProgress := phases.InitProgress(ctx, dhlog.FromContext(ctx), "Bootstrap cluster")
	// 	bctx.finishProgress = finishProgress

	// 	onUpdateFunc := func(progress phases.Progress) error {
	// 		// Non-blocking: the pipeline's deferred Finalize can emit after the consumer has
	// 		// stopped and the channel is no longer drained; never block or panic on it.
	// 		select {
	// 		case progressCh <- progress:
	// 		default:
	// 		}
	// 		return nil
	// 	}

	// 	b.PhasedExecutionContext = phases.NewDefaultPhasedExecutionContext(phases.OperationBootstrap, b.OnPhaseFunc, onUpdateFunc)
	// }

	configHash := state.ConfigHash(ctx, b.Options.Global.ConfigPaths)

	clusterUUID, err := generateClusterUUID(ctx, stateCache)
	if err != nil {
		return err
	}
	metaConfig.UUID = clusterUUID

	metaConfig.ResourceManagementTimeout = b.Options.Cache.ResourceManagementTimeout

	deckhouseInstallConfig, err := config.PrepareDeckhouseInstallConfig(ctx, metaConfig, &b.Options.Global)
	if err != nil {
		return err
	}

	b.applyCommanderModeConfig(deckhouseInstallConfig)

	// During full bootstrap we use the "kubeadm and deckhouse on master nodes" hack
	deckhouseInstallConfig.KubeadmBootstrap = true
	deckhouseInstallConfig.MasterNodeSelector = true

	bootstrapState := NewBootstrapState(stateCache)

	bctx.metaConfig = metaConfig
	bctx.stateCache = stateCache
	bctx.configHash = configHash
	bctx.deckhouseInstallConfig = deckhouseInstallConfig
	bctx.bootstrapState = bootstrapState

	return nil
}

func (b *ClusterBootstrapper) bootstrapPreflight(ctx context.Context, bctx *bootstrapContext) error {
	if shouldStop, err := b.PhasedExecutionContext.StartPhase(ctx, phases.PreInfraPreflightsPhase, true, bctx.stateCache); err != nil {
		return err
	} else if shouldStop {
		return nil
	}

	ctx, preflightSpan := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.PreInfraPreflights")
	defer preflightSpan.End()

	cleanup, err := b.getCleanupFunc(ctx, bctx.metaConfig)
	if err != nil {
		return err
	}
	bctx.cleanup = cleanup

	globalPreflightSuite := suites.NewGlobalSuite(suites.GlobalDeps{
		MetaConfig:    bctx.metaConfig,
		InstallConfig: bctx.deckhouseInstallConfig,
		BuildInfo:     b.Options.BuildInfo,
	})

	if bctx.metaConfig.ClusterType == config.CloudClusterType {
		sshProvider, err := b.SSHProviderInitializer.GetSSHProvider(ctx)
		if err != nil {
			if !errors.Is(err, providerinitializer.ErrHostsFromCacheNotFound) {
				return err
			}
		}

		cloudPreflightSuite := suites.NewCloudSuite(suites.CloudDeps{
			InstallConfig:          bctx.deckhouseInstallConfig,
			MetaConfig:             bctx.metaConfig,
			SSHProviderInitializer: b.SSHProviderInitializer,
		})
		postCloudPreflightSuite := suites.NewPostCloudSuite(suites.PostCloudDeps{
			MetaConfig:  bctx.metaConfig,
			SSHProvider: sshProvider,
			LegacyMode:  b.SSHProviderInitializer.IsLegacyMode(),
		})

		preflightRunner := preflight.New(globalPreflightSuite, cloudPreflightSuite, postCloudPreflightSuite)
		preflightRunner.UseCache(bctx.bootstrapState)
		preflightRunner.SetCacheSalt(bctx.configHash)
		preflightRunner.DisableChecks(b.Options.Preflight.DisabledChecks()...)
		bctx.preflightRunner = preflightRunner
		if err := preflightRunner.Run(ctx, preflight.PhasePreInfra); err != nil {
			return err
		}
	} else {
		staticPreflightSuite, err := suites.NewStaticSuite(suites.StaticDeps{
			SSHProviderInitializer: b.SSHProviderInitializer,
			MetaConfig:             bctx.metaConfig,
			LegacyMode:             b.SSHProviderInitializer.IsLegacyMode(),
			GlobalOpts:             &b.Options.Global,
		}, ctx)
		if err != nil {
			return err
		}

		preflightRunner := preflight.New(globalPreflightSuite, staticPreflightSuite)
		preflightRunner.UseCache(bctx.bootstrapState)
		preflightRunner.SetCacheSalt(bctx.configHash)
		preflightRunner.DisableChecks(b.Options.Preflight.DisabledChecks()...)
		bctx.preflightRunner = preflightRunner

		if err := preflightRunner.Run(ctx, preflight.PhasePreInfra); err != nil {
			return err
		}
	}
	return nil
}

func (b *ClusterBootstrapper) bootstrapBaseInfra(ctx context.Context, bctx *bootstrapContext) error {
	if shouldStop, err := b.PhasedExecutionContext.SwitchPhase(ctx, phases.BaseInfraPhase, false, bctx.stateCache, nil); err != nil {
		return err
	} else if shouldStop {
		return nil
	}

	ctx, baseInfraSpan := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.BaseInfra")
	defer baseInfraSpan.End()

	if bctx.metaConfig.ClusterType == config.CloudClusterType {
		err := dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Cloud infrastructure", func(ctx context.Context) error {
			ctx, span := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.CloudInfra")
			defer span.End()

			baseRunner, err := b.InfrastructureContext.GetBootstrapBaseInfraRunner(ctx, bctx.metaConfig, bctx.stateCache)
			if err != nil {
				return err
			}

			baseOutputs, err := infrastructure.ApplyPipeline(ctx, baseRunner, "Kubernetes cluster", &b.Options.Global, infrastructure.GetBaseInfraResult)
			if err != nil {
				return err
			}

			dhlog.FromContext(ctx).DebugContext(ctx, "Base infrastructure was created")
			b.PhasedExecutionContext.CompleteSubPhase(ctx, phases.BaseInfraSubPhaseBaseInfra)

			var cloudDiscoveryData map[string]any
			err = json.Unmarshal(baseOutputs.CloudDiscovery, &cloudDiscoveryData)
			if err != nil {
				return err
			}

			bctx.resourcesTemplateData = map[string]any{
				"cloudDiscovery": cloudDiscoveryData,
			}

			masterNodeName := fmt.Sprintf("%s-master-0", bctx.metaConfig.ClusterPrefix)
			masterRunner, err := b.Params.InfrastructureContext.GetBootstrapNodeRunner(ctx, bctx.metaConfig, bctx.stateCache, infrastructure.BootstrapNodeRunnerOptions{
				NodeName:        masterNodeName,
				NodeGroupStep:   infrastructure.MasterNodeStep,
				NodeGroupName:   "master",
				NodeIndex:       0,
				NodeCloudConfig: "",
			})
			if err != nil {
				return err
			}

			masterOutputs, err := infrastructure.ApplyPipeline(ctx, masterRunner, masterNodeName, &b.Options.Global, infrastructure.GetMasterNodeResult)
			if err != nil {
				return err
			}

			dhlog.FromContext(ctx).DebugContext(ctx, "First control-plane node was created")
			b.PhasedExecutionContext.CompleteSubPhase(ctx, phases.BaseInfraSubPhaseFirstMaster)

			bctx.deckhouseInstallConfig.CloudDiscovery = baseOutputs.CloudDiscovery
			bctx.deckhouseInstallConfig.InfrastructureState = baseOutputs.InfrastructureState

			// providers should be reinitialized here
			baseSettings := b.SSHProviderInitializer.GetSettings()
			connectionConfig := b.SSHProviderInitializer.GetConfig()

			if baseOutputs.BastionHost != "" {
				connectionConfig.Config.BastionHost = baseOutputs.BastionHost
				if err := SaveBastionHostToCache(ctx, bctx.stateCache, baseOutputs.BastionHost); err != nil {
					dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("Cannot save bastion host to cache %v", err))
				}
			}

			connectionConfig.Hosts = append(connectionConfig.Hosts, sshconfig.Host{Host: masterOutputs.MasterIPForSSH})

			b.SSHProviderInitializer.Reinitialize(
				ctx,
				baseSettings,
				connectionConfig,
			)
			b.KubeProvider = b.SSHProviderInitializer.GetKubeProvider(ctx)

			bctx.nodeIP = masterOutputs.NodeInternalIP
			bctx.devicePath = masterOutputs.KubeDataDevicePath

			bctx.deckhouseInstallConfig.NodesInfrastructureState = make(map[string][]byte)
			bctx.deckhouseInstallConfig.NodesInfrastructureState[masterNodeName] = masterOutputs.InfrastructureState

			bctx.masterAddressesForSSH[masterNodeName] = masterOutputs.MasterIPForSSH
			state.SaveMasterHostsToCache(ctx, bctx.stateCache, bctx.masterAddressesForSSH)

			interactive := input.IsTerminal() && !b.Options.Global.ShowProgress
			if interactive {
				sshProvider, err := b.SSHProviderInitializer.GetSSHProvider(ctx)
				if err != nil {
					return err
				}
				sshClient, err := sshProvider.Client(ctx)
				if err != nil {
					return err
				}
				sshString := sshClient.Session().String()
				dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("First master connection string: %s", sshString))
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *ClusterBootstrapper) bootstrapPostInfraPreflights(ctx context.Context, bctx *bootstrapContext) error {
	if shouldStop, err := b.PhasedExecutionContext.SwitchPhase(ctx, phases.PostInfraPreflightsPhase, false, bctx.stateCache, nil); err != nil {
		return err
	} else if shouldStop {
		return nil
	}

	if bctx.metaConfig.ClusterType == config.CloudClusterType {
		if err := bctx.preflightRunner.Run(ctx, preflight.PhasePostInfra); err != nil {
			return err
		}
	} else {
		if err := bctx.preflightRunner.Run(ctx, preflight.PhasePostInfra); err != nil {
			return err
		}

		var static struct {
			NodeIP string `json:"nodeIP"`
		}
		if err := json.Unmarshal(bctx.metaConfig.ClusterConfig["static"], &static); err != nil {
			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Static config is missing: %s", err.Error()))
		}
		bctx.nodeIP = static.NodeIP

		if b.SSHProviderInitializer.CheckHosts(ctx) {
			connectionConfig := b.SSHProviderInitializer.GetConfig()
			if connectionConfig.Config.BastionHost != "" {
				if err := SaveBastionHostToCache(ctx, bctx.stateCache, connectionConfig.Config.BastionHost); err != nil {
					dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("Cannot save bastion host to cache %v", err))
				}
			}

			state.SaveMasterHostsToCache(ctx, bctx.stateCache, map[string]string{
				"first-master": connectionConfig.Hosts[0].Host,
			})
		}
	}

	if bctx.metaConfig.ResourcesYAML != "" {
		parsedResources, err := template.ParseResourcesContent(ctx, bctx.metaConfig.ResourcesYAML, bctx.resourcesTemplateData)
		if err != nil {
			return err
		}

		before, after := splitResourcesOnPreAndPostDeckhouseInstall(ctx, parsedResources)

		bctx.resourcesToCreateBefore = before
		bctx.resourcesToCreateAfter = after
	}

	if b.SSHProviderInitializer.CheckHosts(ctx) {
		sshProvider, err := b.SSHProviderInitializer.GetSSHProvider(ctx)
		if err != nil {
			return err
		}

		sshClient, err := sshProvider.Client(ctx)
		if err != nil {
			return err
		}

		if err := WaitForSSHConnectionOnMaster(ctx, sshClient); err != nil {
			return fmt.Errorf("failed to wait for SSH connection on master: %w", err)
		}
	}

	return nil
}

func (b *ClusterBootstrapper) bootstrapKubernetes(ctx context.Context, bctx *bootstrapContext) error {
	if shouldStop, err := b.PhasedExecutionContext.SwitchPhase(ctx, phases.InstallKubernetesPhase, false, bctx.stateCache, nil); err != nil {
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
		Node:                   nodeInterface,
		NodeIP:                 bctx.nodeIP,
		DevicePath:             bctx.devicePath,
		MetaConfig:             bctx.metaConfig,
		CommanderMode:          b.CommanderMode,
		GlobalOpts:             &b.Options.Global,
		LoggerProvider:         b.loggerProvider,
		PhasedExecutionContext: b.PhasedExecutionContext,
	})

	if err != nil {
		return err
	}

	bashibleBundleSpan.End()

	return nil
}

func (b *ClusterBootstrapper) bootstrapDeckhouse(ctx context.Context, bctx *bootstrapContext) error {
	if shouldStop, err := b.PhasedExecutionContext.SwitchPhase(ctx, phases.InstallDeckhousePhase, false, bctx.stateCache, nil); err != nil {
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

	b.PhasedExecutionContext.CompleteSubPhase(ctx, phases.InstallDeckhouseSubPhaseConnect)

	installParams := InstallDeckhouseParams{
		BeforeDeckhouseTask: func() error {
			return createResources(
				ctx,
				&client.KubernetesClient{KubeClient: kubeCl},
				bctx.resourcesToCreateBefore,
				nil,
				true,
				b.Options.Bootstrap.ResourcesTimeout,
			)
		},
		State:            bctx.bootstrapState,
		DeckhouseTimeout: b.Options.Bootstrap.DeckhouseTimeout,
	}

	installDeckhouseResult, err := InstallDeckhouse(ctx, &client.KubernetesClient{KubeClient: kubeCl}, bctx.deckhouseInstallConfig, installParams)
	if err != nil {
		return err
	}
	bctx.installDeckhouseResult = installDeckhouseResult

	b.PhasedExecutionContext.CompleteSubPhase(ctx, phases.InstallDeckhouseSubPhaseInstall)

	err = WaitForFirstMasterNodeBecomeReady(ctx, &client.KubernetesClient{KubeClient: kubeCl})
	if err != nil {
		return err
	}

	b.PhasedExecutionContext.CompleteSubPhase(ctx, phases.InstallDeckhouseSubPhaseWait)
	return nil
}

func (b *ClusterBootstrapper) bootstrapAdditionalNodes(ctx context.Context, bctx *bootstrapContext) error {
	if bctx.metaConfig.ClusterType == config.CloudClusterType {
		if shouldStop, err := b.PhasedExecutionContext.SwitchPhase(ctx, phases.InstallAdditionalMastersAndStaticNodes, true, bctx.stateCache, nil); err != nil {
			return err
		} else if shouldStop {
			return nil
		}

		ctx, additionalNodesSpan := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.AdditionalNodes")
		defer additionalNodesSpan.End()

		kubeCl, err := b.KubeProvider.Client(ctx)
		if err != nil {
			return err
		}

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

		err = localBootstraper(func() error {
			return bootstrapAdditionalNodesForCloudCluster(
				ctx,
				&client.KubernetesClient{KubeClient: kubeCl},
				bctx.metaConfig,
				bctx.masterAddressesForSSH,
				b.InfrastructureContext,
				&b.Options.Global,
				b.PhasedExecutionContext,
			)
		})
		if err != nil {
			return err
		}

		additionalNodesSpan.End()
	}

	kubeCl, err := b.KubeProvider.Client(ctx)
	if err != nil {
		return err
	}

	if err := controlplane.NewManagerReadinessChecker(kubernetes.NewSimpleKubeClientGetter(&client.KubernetesClient{KubeClient: kubeCl})).IsReadyAll(ctx); err != nil {
		return err
	}
	b.PhasedExecutionContext.CompleteSubPhase(ctx, phases.InstallAdditionalMastersAndStaticNodesSubPhaseWait)

	return nil
}

func (b *ClusterBootstrapper) bootstrapCreateResources(ctx context.Context, bctx *bootstrapContext) error {
	if shouldStop, err := b.PhasedExecutionContext.SwitchPhase(ctx, phases.CreateResourcesPhase, false, bctx.stateCache, nil); err != nil {
		return err
	} else if shouldStop {
		return nil
	}

	kubeCl, err := b.KubeProvider.Client(ctx)
	if err != nil {
		return err
	}

	err = createResources(
		ctx,
		&client.KubernetesClient{KubeClient: kubeCl},
		bctx.resourcesToCreateAfter,
		bctx.installDeckhouseResult,
		false,
		b.Options.Bootstrap.ResourcesTimeout,
	)
	if err != nil {
		return err
	}

	return nil
}
func (b *ClusterBootstrapper) bootstrapPostBootstrap(ctx context.Context, bctx *bootstrapContext) error {
	if shouldStop, err := b.PhasedExecutionContext.SwitchPhase(ctx, phases.ExecPostBootstrapPhase, false, bctx.stateCache, nil); err != nil {
		return err
	} else if shouldStop {
		return nil
	}

	if b.SSHProviderInitializer.CheckHosts(ctx) && b.Options.Bootstrap.PostBootstrapScriptPath != "" {
		ctx, postBootstrapSpan := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.PostBootstrap")
		defer postBootstrapSpan.End()

		postScriptExecutor := NewPostBootstrapScriptExecutor(b.SSHProviderInitializer, b.Options.Bootstrap.PostBootstrapScriptPath, bctx.bootstrapState).
			WithTimeout(b.Options.Bootstrap.PostBootstrapScriptTimeout)

		if err := postScriptExecutor.Execute(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (b *ClusterBootstrapper) bootstrapFinalize(ctx context.Context, bctx *bootstrapContext) error {
	if shouldStop, err := b.PhasedExecutionContext.SwitchPhase(ctx, phases.FinalizationPhase, false, bctx.stateCache, nil); err != nil {
		return err
	} else if shouldStop {
		return nil
	}

	kubeCl, err := b.KubeProvider.Client(ctx)
	if err != nil {
		return err
	}

	if err := RunPostInstallTasks(ctx, &client.KubernetesClient{KubeClient: kubeCl}, bctx.installDeckhouseResult); err != nil {
		return err
	}

	if !b.DisableBootstrapClearCache {
		_ = dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Clear cache", func(ctx context.Context) error {
			ctx, span := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.ClearCache")
			defer span.End()

			cache.Global().CleanWithExceptions(
				ctx,
				state.MasterHostsCacheKey,
				ManifestCreatedInClusterCacheKey,
				BastionHostCacheKey,
				PostBootstrapResultCacheKey,
			)
			dhlog.FromContext(ctx).WarnContext(ctx, `Next run of "dhctl bootstrap" will create a new Kubernetes cluster.`)

			return nil
		})
	}

	dhlog.FromContext(ctx).InfoContext(ctx, "Deckhouse cluster created successfully!", dhlog.ShowInCompacted())

	if bctx.metaConfig.ClusterType == config.CloudClusterType {
		_ = dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Kubernetes Master Node addresses for SSH", func(ctx context.Context) error {
			ctx, span := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.KubernetesMasterNodeAddressesForSSH")
			defer span.End()

			sshProvider, err := b.SSHProviderInitializer.GetSSHProvider(ctx)
			if err != nil {
				return err
			}

			sshClient, err := sshProvider.Client(ctx)
			if err != nil {
				return err
			}
			for nodeName, address := range bctx.masterAddressesForSSH {
				fakeSession := sshClient.Session().Copy()
				fakeSession.SetAvailableHosts([]session.Host{{Host: address, Name: nodeName}})
				dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("%s | %s", nodeName, fakeSession.String()), dhlog.ShowInCompacted())
			}

			return nil
		})
	}

	return b.PhasedExecutionContext.CompletePhaseAndPipeline(ctx, bctx.stateCache, nil)
}

func (b *ClusterBootstrapper) GetLastState() phases.DhctlState {
	if b.lastState != nil {
		return b.lastState
	}

	return b.PhasedExecutionContext.GetLastState()
}

func printBanner(ctx context.Context) {
	dhlog.PrintBanner(ctx)
}

func generateClusterUUID(ctx context.Context, stateCache state.Cache) (string, error) {
	var clusterUUID string

	return clusterUUID, dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Cluster UUID", func(ctx context.Context) error {
		ok, err := stateCache.InCache(ctx, "uuid")
		if err != nil {
			return err
		}

		if !ok {
			genClusterUUID, err := uuid.NewRandom()
			if err != nil {
				return fmt.Errorf("can't create cluster UUID: %w", err)
			}

			clusterUUID = genClusterUUID.String()
			err = stateCache.Save(ctx, "uuid", []byte(clusterUUID))
			if err != nil {
				return err
			}
			dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("Generated cluster UUID: %s", clusterUUID))
		} else {
			clusterUUIDBytes, err := stateCache.Load(ctx, "uuid")
			if err != nil {
				return err
			}
			clusterUUID = string(clusterUUIDBytes)
			dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("Cluster UUID from cache: %s", clusterUUID))
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
	globalOptions *options.GlobalOptions,
	pec phases.DefaultPhasedExecutionContext,
) error {
	ctx, span := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.AdditionalNodesForCloudCluster")
	defer span.End()

	if err := BootstrapAdditionalMasterNodes(ctx, kubeCl, metaConfig, masterAddressesForSSH, infrastructureContext, cache.Global(), globalOptions); err != nil {
		return err
	}

	terraNodeGroups := metaConfig.GetTerraNodeGroups()
	bootstrapAdditionalTerraNodeGroups := BootstrapTerraNodes
	if operations.IsSequentialNodesBootstrap(metaConfig) {
		bootstrapAdditionalTerraNodeGroups = operations.BootstrapSequentialTerraNodes
	}

	pec.CompleteSubPhase(ctx, phases.InstallAdditionalMastersAndStaticNodesSubPhaseAdditionalMasters)

	if err := bootstrapAdditionalTerraNodeGroups(ctx, kubeCl, metaConfig, terraNodeGroups, infrastructureContext, globalOptions); err != nil {
		return err
	}

	return dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Waiting for node groups to become ready", func(ctx context.Context) error {
		ctx, span := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.AdditionalNodesForCloudCluster.WaitForNodesBecomeReady")
		defer span.End()

		ngs := map[string]int{"master": metaConfig.MasterNodeGroupSpec.Replicas}
		for _, ng := range terraNodeGroups {
			if ng.Replicas > 0 {
				ngs[ng.Name] = ng.Replicas
			}
		}
		if err := entity.WaitForNodesBecomeReady(ctx, kubeCl, ngs); err != nil {
			return err
		}

		pec.CompleteSubPhase(ctx, phases.InstallAdditionalMastersAndStaticNodeSubPhaseStaticNodes)

		return nil
	})
}

func splitResourcesOnPreAndPostDeckhouseInstall(ctx context.Context, resourcesToCreate template.Resources) (template.Resources, template.Resources) {
	before := make(template.Resources, 0, len(resourcesToCreate))
	after := make(template.Resources, 0, len(resourcesToCreate))

	for _, resource := range resourcesToCreate {
		annotations := resource.Object.GetAnnotations()
		if annotations == nil || annotations["dhctl.deckhouse.io/bootstrap-resource-place"] != "before-deckhouse" {
			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Add resource %s - %s to after queue", resource.String(), resource.Object.GetName()))
			after = append(after, resource)
			continue
		}

		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Add resource %s - %s to before queue", resource.String(), resource.Object.GetName()))
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
		dhlog.FromContext(ctx).WarnContext(
			ctx,
			"Core module deckhouse has been installed.\n"+
				"The resources provided by your configuration will now be applied.",
		)

		tasks = result.ManifestResult.WithResourcesMCTasks

		span.SetAttributes(otattribute.Int("tasks_count", len(tasks)))

		if len(resourcesToCreate) == 0 {
			for _, task := range tasks {
				return retry.NewLoop(task.Title, 300, 1*time.Second).RunContext(ctx, func() error {
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

	return dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Create Resources", func(ctx context.Context) error {
		var err error
		checkers := make([]resources.Checker, 0)
		if !skipChecks {
			checkers, err = resources.GetCheckers(ctx, kubeCl, resourcesToCreate, nil)
			if err != nil {
				return err
			}
		}

		return resources.CreateResourcesLoop(ctx, kubeCl, resourcesToCreate, checkers, tasks, timeout)
	})
}
