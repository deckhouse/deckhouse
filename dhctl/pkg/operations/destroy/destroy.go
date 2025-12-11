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
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/controller"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/kube"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	dhctlstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
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
	PopulateMetaConfig(ctx context.Context) (*config.MetaConfig, error)
}

type Params struct {
	NodeInterface node.Interface
	StateCache    dhctlstate.Cache

	OnPhaseFunc            phases.DefaultOnPhaseFunc
	OnProgressFunc         phases.OnProgressFunc
	PhasedExecutionContext phases.DefaultPhasedExecutionContext

	SkipResources bool

	CommanderMode bool
	CommanderUUID uuid.UUID
	*commander.CommanderModeParams

	InfrastructureContext *infrastructure.Context

	TmpDir         string
	LoggerProvider log.LoggerProvider
	IsDebug        bool
}

type ClusterDestroyer struct {
	stateCache       dhctlstate.Cache
	configPreparator metaConfigPopulator

	d8Destroyer   *deckhouse.Destroyer
	infraProvider *infraDestroyerProvider

	PhasedExecutionContext phases.DefaultPhasedExecutionContext

	loggerProvider log.LoggerProvider
}

// NewClusterDestroyer
// params.SSHClient should not START!
func NewClusterDestroyer(ctx context.Context, params *Params) (*ClusterDestroyer, error) {
	if govalue.IsNil(params.StateCache) {
		return nil, fmt.Errorf("State cache is required")
	}

	wrapper, ok := params.NodeInterface.(*ssh.NodeInterfaceWrapper)
	if !ok {
		return nil, fmt.Errorf("Cluster destruction requires usage of ssh node interface")
	}

	sshClientProvider := sync.OnceValues(func() (node.SSHClient, error) {
		sshClient := wrapper.Client()
		if err := sshClient.Start(); err != nil {
			return nil, err
		}

		return sshClient, nil
	})

	logger := log.SafeProvideLogger(params.LoggerProvider)

	if app.ProgressFilePath != "" {
		params.OnProgressFunc = phases.WriteProgress(app.ProgressFilePath)
	}

	var pec phases.DefaultPhasedExecutionContext
	if params.PhasedExecutionContext != nil {
		pec = params.PhasedExecutionContext
	} else {
		pec = phases.NewDefaultPhasedExecutionContext(
			phases.OperationDestroy, params.OnPhaseFunc, params.OnProgressFunc,
		)
	}

	phaseActionProvider := phases.NewDefaultPhaseActionProviderWithStateCache(pec, params.StateCache)

	var kubeProvider kube.ClientProviderWithCleanup = newKubeClientProvider(sshClientProvider)

	d8Destroyer := deckhouse.NewDestroyer(deckhouse.DestroyerParams{
		CommanderUUID: params.CommanderUUID,
		CommanderMode: params.CommanderMode,

		SkipResources: params.SkipResources,
		State:         deckhouse.NewState(params.StateCache),

		LoggerProvider:       params.LoggerProvider,
		KubeProvider:         kubeProvider,
		PhasedActionProvider: phaseActionProvider,
	})

	var terraStateLoader controller.StateLoader

	if params.CommanderMode {
		// FIXME(dhctl-for-commander): commander uuid currently optional, make it required later
		// if params.CommanderUUID == uuid.Nil {
		//	panic("CommanderUUID required for destroy operation in commander mode!")
		// }

		metaConfig, err := commander.ParseMetaConfig(ctx, params.StateCache, params.CommanderModeParams, logger)
		if err != nil {
			return nil, fmt.Errorf("Unable to parse meta configuration: %w", err)
		}
		terraStateLoader = infrastructurestate.NewFileTerraStateLoader(params.StateCache, metaConfig)
	} else {
		stateLoaderKubeProvider := kubeProvider
		if params.SkipResources {
			stateLoaderKubeProvider = newKubeClientErrorProvider("Skip resources flag was provided. State not found in cache")
		}

		terraStateLoader = infrastructurestate.NewLazyTerraStateLoader(
			infrastructurestate.NewCachedTerraStateLoader(stateLoaderKubeProvider, params.StateCache, logger),
		)
	}

	infraProvider := &infraDestroyerProvider{
		stateCache:     params.StateCache,
		kubeProvider:   kubeProvider,
		loggerProvider: params.LoggerProvider,
		commanderMode:  params.CommanderMode,
		skipResources:  params.SkipResources,
		cloudStateProvider: func() (controller.StateLoader, *controller.ClusterInfra, error) {
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
	}

	return &ClusterDestroyer{
		stateCache:       params.StateCache,
		configPreparator: terraStateLoader,

		d8Destroyer:   d8Destroyer,
		infraProvider: infraProvider,

		PhasedExecutionContext: pec,

		loggerProvider: params.LoggerProvider,
	}, nil
}

func (d *ClusterDestroyer) DestroyCluster(ctx context.Context, autoApprove bool) error {
	if err := d.PhasedExecutionContext.InitPipeline(d.stateCache); err != nil {
		return err
	}
	defer d.PhasedExecutionContext.Finalize(d.stateCache)

	err := d.destroyWithPhaseExecutor(ctx, autoApprove)

	if err == nil {
		return d.PhasedExecutionContext.CompletePipeline(d.stateCache)
	}

	if errors.Is(err, phases.ErrShouldStop) {
		log.SafeProvideLogger(d.loggerProvider).LogDebugLn("Destroy phase execution context: should stop")
		return nil
	}

	return err
}

func (d *ClusterDestroyer) destroyWithPhaseExecutor(ctx context.Context, autoApprove bool) error {
	if err := d.d8Destroyer.CheckCommanderUUID(ctx); err != nil {
		return err
	}

	// populate cluster state in cache
	metaConfig, err := d.configPreparator.PopulateMetaConfig(ctx)
	if err != nil {
		return err
	}

	destroyer, err := config.DoByClusterType(ctx, metaConfig, d.infraProvider)
	if err != nil {
		return err
	}

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

	log.SafeProvideLogger(d.loggerProvider).LogDebugF("Resources were destroyed set\n")

	// Stop proxy because we have already got all info from kubernetes-api
	// also stop ssh client for cloud clusters
	if err := destroyer.CleanupBeforeDestroy(ctx); err != nil {
		return err
	}

	if err := destroyer.DestroyCluster(ctx, autoApprove); err != nil {
		return err
	}

	d.stateCache.Clean()

	return nil
}
