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

package destroy

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/name212/govalue"

	libcon "github.com/deckhouse/lib-connection/pkg"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/directoryconfig"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/controller"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/kube"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	dhctlstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
)

type Destroyer interface {
	DestroyCluster(ctx context.Context, autoApprove bool) error
}

type infraDestroyer interface {
	Destroyer

	AfterResourcesDelete(ctx context.Context) error
	Prepare(ctx context.Context) error
	CleanupBeforeDestroy(ctx context.Context) error
}

type metaConfigPopulator interface {
	PopulateMetaConfig(ctx context.Context, dc *directoryconfig.DirectoryConfig) (*config.MetaConfig, error)
}

type Params struct {
	StateCache   dhctlstate.Cache
	SSHProvider  libcon.SSHProvider
	KubeProvider libcon.KubeProvider

	// todo pass pipeline provider here
	OnPhaseFunc            phases.DefaultOnPhaseFunc
	OnProgressFunc         phases.OnProgressFunc
	PhasedExecutionContext phases.DefaultPhasedExecutionContext

	SkipResources bool

	CommanderMode bool
	CommanderUUID uuid.UUID
	*commander.CommanderModeParams

	InfrastructureContext *infrastructure.Context

	TmpDir          string
	LoggerProvider  log.LoggerProvider
	IsDebug         bool
	DirectoryConfig *directoryconfig.DirectoryConfig

	// Options carries the per-operation parsed configuration. RPC handlers
	// must populate this with a fresh *options.Options to avoid sharing global
	// state between concurrent requests.
	Options *options.Options
}

func (p *Params) getExecutionContext() phases.DefaultPhasedExecutionContext {
	if p.PhasedExecutionContext != nil {
		return p.PhasedExecutionContext
	}

	return phases.NewDefaultPhasedExecutionContext(
		phases.OperationDestroy, p.OnPhaseFunc, p.OnProgressFunc,
	)
}

func (p *Params) getStateLoaderParams() *stateLoaderParams {
	return &stateLoaderParams{
		commanderMode:   p.CommanderMode,
		commanderParams: p.CommanderModeParams,

		stateCache: p.StateCache,
		logger:     log.SafeProvideLogger(p.LoggerProvider),

		skipResources: p.SkipResources,
		// from passed params always ask about load
		forceFromCache: false,
	}
}

type stateLoaderParams struct {
	commanderMode   bool
	commanderParams *commander.CommanderModeParams

	stateCache dhctlstate.Cache
	logger     log.Logger

	skipResources  bool
	forceFromCache bool
}

func initStateLoader(ctx context.Context, params *stateLoaderParams, kubeProvider kube.ClientProviderWithCleanup) (controller.StateLoader, kube.ClientProviderWithCleanup, error) {
	if params.commanderMode {
		// FIXME(dhctl-for-commander): commander uuid currently optional, make it required later
		// if params.CommanderUUID == uuid.Nil {
		//	panic("CommanderUUID required for destroy operation in commander mode!")
		// }

		metaConfig, err := commander.ParseMetaConfig(ctx, params.stateCache, params.commanderParams, params.logger)
		if err != nil {
			return nil, nil, fmt.Errorf("Unable to parse meta configuration: %w", err)
		}
		return infrastructurestate.NewFileTerraStateLoader(params.stateCache, metaConfig), kubeProvider, nil
	}

	stateLoaderKubeProvider := kubeProvider
	if params.skipResources {
		stateLoaderKubeProvider = newKubeClientErrorProvider("Skip resources flag was provided. State not found in cache")
	}

	cached := infrastructurestate.NewCachedTerraStateLoader(stateLoaderKubeProvider, params.stateCache, params.logger).
		WithForceFromCache(params.forceFromCache)
	return infrastructurestate.NewLazyTerraStateLoader(cached), stateLoaderKubeProvider, nil
}

// ClusterDestroyer orchestrates a three-phase destroy:
//  1. prepare           — load meta config, validate, pick the infra destroyer.
//  2. deleteResources   — drain deckhouse k8s resources; release k8s access at end.
//  3. destroyInfra      — tear down the underlying infrastructure.
//
// alwaysCleanup runs deferred on every exit (success or failure) and is
// where process-wide cleanup belongs (tmp dir removal). stateCache.Clean
// runs only on the success path so a failed destroy leaves the cache for
// the operator to inspect or resume.
type ClusterDestroyer struct {
	pipeline       phases.DefaultPipeline
	stateCache     dhctlstate.Cache
	tmpDir         string
	loggerProvider log.LoggerProvider

	prepare         *prepareDestroyPhase
	deleteResources *deleteResourcesPhase
	destroyInfra    destroyInfraPhase
}

// NewClusterDestroyer
// params.SSHClient should not START!
func NewClusterDestroyer(ctx context.Context, params *Params) (*ClusterDestroyer, error) {
	if govalue.IsNil(params.StateCache) {
		return nil, fmt.Errorf("State cache is required")
	}

	logger := log.SafeProvideLogger(params.LoggerProvider)

	if params.Options != nil && params.Options.Global.ProgressFilePath != "" {
		params.OnProgressFunc = phases.WriteProgress(params.Options.Global.ProgressFilePath)
	}

	pec := params.getExecutionContext()

	pipeline := phases.NewDefaultPipelineWithStateCacheProviderOpts(
		pec,
		params.StateCache,
		phases.WithPipelineLoggerProvider(params.LoggerProvider),
		phases.WithPipelineName("cluster-destroyer"),
	)()

	phaseActionProvider := phases.NewDefaultPhaseActionProviderFromPipeline(pipeline)

	var kubeProvider kube.ClientProviderWithCleanup = newKubeClientProvider(params.KubeProvider)

	terraStateLoader, kubeProvider, err := initStateLoader(ctx, params.getStateLoaderParams(), kubeProvider)
	if err != nil {
		return nil, err
	}

	deckhouseState := deckhouse.NewState(params.StateCache)

	infraProvider := &infraDestroyerProvider{
		stateCache:           params.StateCache,
		kubeProvider:         kubeProvider,
		loggerProvider:       params.LoggerProvider,
		phasesActionProvider: phaseActionProvider,

		commanderMode: params.CommanderMode,
		skipResources: params.SkipResources,
		cloudStateProvider: func() (controller.StateLoader, cloud.ClusterInfraDestroyer, error) {
			return terraStateLoader, controller.NewClusterInfraWithOptions(
				terraStateLoader,
				params.StateCache,
				params.InfrastructureContext,
				controller.ClusterInfraOptions{
					PhasedExecutionContext: pec,
					TmpDir:                 params.TmpDir,
					DownloadDir:            params.Options.Global.DownloadDir,
					IsDebug:                params.IsDebug,
					Logger:                 logger,
				},
			), nil
		},

		sshClientProvider: params.SSHProvider,
		sshUser:           params.Options.SSH.User,
		tmpDir:            params.TmpDir,
	}

	return &ClusterDestroyer{
		pipeline:       pipeline,
		stateCache:     params.StateCache,
		tmpDir:         params.TmpDir,
		loggerProvider: params.LoggerProvider,

		prepare: &prepareDestroyPhase{
			configPreparator:     terraStateLoader,
			directoryConfig:      params.DirectoryConfig,
			infraProvider:        infraProvider,
			kubeProvider:         kubeProvider,
			deckhouseState:       deckhouseState,
			phasedActionProvider: phaseActionProvider,
			loggerProvider:       params.LoggerProvider,
			commanderMode:        params.CommanderMode,
			commanderUUID:        params.CommanderUUID,
			skipResources:        params.SkipResources,
		},
		deleteResources: &deleteResourcesPhase{
			deckhouseState:       deckhouseState,
			kubeProvider:         kubeProvider,
			phasedActionProvider: phaseActionProvider,
			loggerProvider:       params.LoggerProvider,
			commanderMode:        params.CommanderMode,
			commanderUUID:        params.CommanderUUID,
			skipResources:        params.SkipResources,
		},
		destroyInfra: destroyInfraPhase{},
	}, nil
}

func (d *ClusterDestroyer) DestroyCluster(ctx context.Context, autoApprove bool) error {
	return d.pipeline.Run(ctx, func(_ phases.DefaultPipelinePhaseSwitcher) error {
		// Process-wide always-cleanup hook. The deleteResourcesPhase already
		// closes its own kube/SSH access via its internal defer, so on the
		// happy path there is nothing left to do here. The tmp dir is left
		// in place on purpose: cloud destroy keeps tofu state in it and the
		// operator may want to resume after a partial failure.
		defer d.alwaysCleanup(ctx)

		prep, err := d.prepare.run(ctx)
		if err != nil {
			return err
		}

		d.pipeline.SetClusterConfig(phases.ClusterConfig{ClusterType: prep.clusterType})

		if err := d.deleteResources.run(ctx, prep); err != nil {
			return err
		}

		if err := d.destroyInfra.run(ctx, prep, autoApprove); err != nil {
			return err
		}

		d.stateCache.Clean(ctx)
		return nil
	})
}

// alwaysCleanup is the home for process-wide cleanup that runs whether
// destroy succeeded or failed. Currently empty: the only resource that
// needs unconditional release (k8s API access) is owned by
// deleteResourcesPhase and freed via its own defer.
func (d *ClusterDestroyer) alwaysCleanup(_ context.Context) {}
