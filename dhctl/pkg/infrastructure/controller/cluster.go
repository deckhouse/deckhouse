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

package controller

import (
	"context"

	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/directoryconfig"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

type StateLoader interface {
	PopulateMetaConfig(ctx context.Context, dc *directoryconfig.DirectoryConfig) (*config.MetaConfig, error)
	PopulateClusterState(ctx context.Context) ([]byte, map[string]state.NodeGroupInfrastructureState, error)
}

type NodeGroupController interface {
	DestroyNode(name string, nodeState []byte, sanityCheck bool) error
}

type BaseInfraControllerInterface interface {
	Destroy(clusterState []byte, sanityCheck bool) error
}

type ClusterInfra struct {
	stateLoader           StateLoader
	cache                 state.Cache
	infrastructureContext *infrastructure.Context

	tmpDir      string
	downloadDir string
	isDebug     bool
	logger      log.Logger
	dc          *directoryconfig.DirectoryConfig

	PhasedExecutionContext phases.DefaultPhasedExecutionContext
}

type ClusterInfraOptions struct {
	PhasedExecutionContext phases.DefaultPhasedExecutionContext
	TmpDir                 string
	DownloadDir            string
	IsDebug                bool
	Logger                 log.Logger
	DirectoryConfig        *directoryconfig.DirectoryConfig
}

func NewClusterInfraWithOptions(terraState StateLoader, cache state.Cache, infrastructureContext *infrastructure.Context, opts ClusterInfraOptions) *ClusterInfra {
	logger := opts.Logger
	if govalue.IsNil(logger) {
		logger = log.GetDefaultLogger()
	}

	return &ClusterInfra{
		stateLoader:           terraState,
		cache:                 cache,
		infrastructureContext: infrastructureContext,

		PhasedExecutionContext: opts.PhasedExecutionContext,
		tmpDir:                 opts.TmpDir,
		downloadDir:            opts.DownloadDir,
		isDebug:                opts.IsDebug,
		logger:                 logger,
		dc:                     opts.DirectoryConfig,
	}
}

func (r *ClusterInfra) DestroyCluster(ctx context.Context, autoApprove bool) error {
	metaConfig, err := r.stateLoader.PopulateMetaConfig(ctx, r.dc)
	if err != nil {
		return err
	}

	if r.infrastructureContext == nil {
		providerGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
			TmpDir:           r.tmpDir,
			DownloadDir:      r.downloadDir,
			AdditionalParams: cloud.ProviderAdditionalParams{},
			Logger:           r.logger,
			IsDebug:          r.isDebug,
		})

		r.infrastructureContext = infrastructure.NewContextWithProvider(providerGetter, r.logger)
	}

	provider, err := r.infrastructureContext.CloudProviderGetter()(ctx, metaConfig)
	if err != nil {
		return err
	}

	defer func() {
		err := provider.Cleanup()
		if err != nil {
			r.logger.LogErrorF("Failed to cleanup infrastructure cloud provider: %v\n", err)
		}
	}()

	clusterState, nodesState, err := r.stateLoader.PopulateClusterState(ctx)
	if err != nil {
		return err
	}

	if r.PhasedExecutionContext != nil {
		if shouldStop, err := r.PhasedExecutionContext.StartPhase(ctx, phases.AllNodesPhase, true, r.cache); err != nil {
			return err
		} else if shouldStop {
			return nil
		}
	}

	for nodeGroupName, nodeGroupStates := range nodesState {
		ngController, err := NewNodesController(ctx, metaConfig, r.cache, nodeGroupName, nodeGroupStates.Settings, r.infrastructureContext)
		if err != nil {
			return err
		}
		for name, ngState := range nodeGroupStates.State {
			err := ngController.DestroyNode(ctx, name, ngState, autoApprove)
			if err != nil {
				return err
			}
		}
	}

	if r.PhasedExecutionContext != nil {
		if shouldStop, err := r.PhasedExecutionContext.SwitchPhase(ctx, phases.BaseInfraPhase, true, r.cache, nil); err != nil {
			return err
		} else if shouldStop {
			return nil
		}
	}

	if err := NewBaseInfraController(metaConfig, r.cache, r.infrastructureContext).
		Destroy(ctx, clusterState, autoApprove); err != nil {
		return err
	}

	if r.PhasedExecutionContext != nil {
		return r.PhasedExecutionContext.CompletePhase(ctx, r.cache, nil)
	} else {
		return nil
	}
}
