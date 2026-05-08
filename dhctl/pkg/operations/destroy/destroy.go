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

type ClusterDestroyer struct {
	stateCache       dhctlstate.Cache
	configPreparator metaConfigPopulator

	pipeline phases.DefaultPipeline

	d8Destroyer     *deckhouse.Destroyer
	infraProvider   *infraDestroyerProvider
	DirectoryConfig *directoryconfig.DirectoryConfig
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

	d8Destroyer := deckhouse.NewDestroyer(deckhouse.DestroyerParams{
		CommanderUUID: params.CommanderUUID,
		CommanderMode: params.CommanderMode,

		SkipResources: params.SkipResources,
		State:         deckhouse.NewState(params.StateCache),

		LoggerProvider:       params.LoggerProvider,
		KubeProvider:         kubeProvider,
		PhasedActionProvider: phaseActionProvider,
	})

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
					IsDebug:                params.IsDebug,
					Logger:                 logger,
				},
			), nil
		},

		sshClientProvider: params.SSHProvider,
		tmpDir:            params.TmpDir,
	}

	return &ClusterDestroyer{
		stateCache:       params.StateCache,
		configPreparator: terraStateLoader,

		pipeline: pipeline,

		d8Destroyer:     d8Destroyer,
		infraProvider:   infraProvider,
		DirectoryConfig: params.DirectoryConfig,
	}, nil
}

func (d *ClusterDestroyer) DestroyCluster(ctx context.Context, autoApprove bool) error {
	return d.pipeline.Run(ctx, func(switcher phases.DefaultPipelinePhaseSwitcher) error {
		return d.destroy(ctx, autoApprove)
	})
}

func (d *ClusterDestroyer) destroy(ctx context.Context, autoApprove bool) error {
	if err := d.d8Destroyer.CheckCommanderUUID(ctx); err != nil {
		return err
	}

	// populate cluster state in cache
	metaConfig, err := d.configPreparator.PopulateMetaConfig(ctx, d.DirectoryConfig)
	if err != nil {
		return err
	}

	destroyer, err := config.DoByClusterType(ctx, metaConfig, d.infraProvider)
	if err != nil {
		return err
	}

	d.pipeline.SetClusterConfig(phases.ClusterConfig{ClusterType: metaConfig.ClusterType})

	err = destroyer.Prepare(ctx)
	if err != nil {
		return err
	}

	if err := d.d8Destroyer.CheckAndDeleteResources(ctx); err != nil {
		return err
	}

	if err := destroyer.AfterResourcesDelete(ctx); err != nil {
		return err
	}

	// only after load and save all states into cache
	// set resources as deleted
	if err := d.d8Destroyer.Finalize(ctx); err != nil {
		return err
	}

	// Stop proxy because we have already got all info from kubernetes-api
	// also stop ssh client for cloud clusters
	if err := destroyer.CleanupBeforeDestroy(ctx); err != nil {
		return err
	}

	if err := destroyer.DestroyCluster(ctx, autoApprove); err != nil {
		return err
	}

	d.stateCache.Clean(ctx)

	return nil
}
