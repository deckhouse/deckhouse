// Copyright 2025 Flant JSC
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

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/controller"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/kube"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/static"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	dhctlstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/sshclient"
	"github.com/name212/govalue"
)

type GetAbortDestroyerParams struct {
	MetaConfig             *config.MetaConfig
	StateCache             dhctlstate.Cache
	InfrastructureContext  *infrastructure.Context
	PhasedExecutionContext phases.DefaultPhasedExecutionContext

	SSHClientProvider sshclient.SSHProvider
	LoggerProvider    log.LoggerProvider

	TmpDir        string
	IsDebug       bool
	CommanderMode bool
}

func GetAbortDestroyer(ctx context.Context, params *GetAbortDestroyerParams) (Destroyer, error) {
	if govalue.IsNil(params) {
		return nil, fmt.Errorf("GetAbortDestroyer: params are required")
	}

	if govalue.IsNil(params.MetaConfig) {
		return nil, fmt.Errorf("GetAbortDestroyer: meta config is required")
	}

	if govalue.IsNil(params.PhasedExecutionContext) {
		return nil, fmt.Errorf("GetAbortDestroyer: phase execution context is required")
	}

	if govalue.IsNil(params.SSHClientProvider) {
		return nil, fmt.Errorf("GetAbortDestroyer: ssh client provider is required")
	}

	if govalue.IsNil(params.StateCache) {
		return nil, fmt.Errorf("GetAbortDestroyer: state cache is required")
	}

	if params.TmpDir == "" {
		return nil, fmt.Errorf("GetAbortDestroyer: tmp dir is required")
	}

	return config.DoByClusterType(ctx, params.MetaConfig, newAbortDestroyerProvider(params))
}

type abortDestroyerProvider struct {
	params *GetAbortDestroyerParams
}

func newAbortDestroyerProvider(params *GetAbortDestroyerParams) *abortDestroyerProvider {
	return &abortDestroyerProvider{
		params: params,
	}
}

func (a *abortDestroyerProvider) Cloud(_ context.Context, metaConfig *config.MetaConfig) (Destroyer, error) {
	if govalue.IsNil(a.params.InfrastructureContext) {
		return nil, fmt.Errorf("GetAbortDestroyer: infrastructure context is required for cloud clusters")
	}

	logger := log.SafeProvideLogger(a.params.LoggerProvider)

	terraStateLoader := infrastructurestate.NewFileTerraStateLoader(a.params.StateCache, metaConfig)
	clusterInfra := controller.NewClusterInfraWithOptions(
		terraStateLoader, a.params.StateCache, a.params.InfrastructureContext,
		controller.ClusterInfraOptions{
			PhasedExecutionContext: a.params.PhasedExecutionContext,
			TmpDir:                 a.params.TmpDir,
			Logger:                 logger,
			IsDebug:                a.params.IsDebug,
		},
	)

	return cloud.NewDestroyer(&cloud.DestroyerParams{
		LoggerProvider: a.params.LoggerProvider,
		KubeProvider:   a.kubeProvider(),
		State:          cloud.NewDestroyState(a.params.StateCache),

		ClusterInfra: clusterInfra,
		StateLoader:  terraStateLoader,

		CommanderMode: a.params.CommanderMode,
	}), nil
}

func (a *abortDestroyerProvider) Static(context.Context, *config.MetaConfig) (Destroyer, error) {
	phaseProvider := phases.NewDefaultPhaseActionProviderWithStateCache(a.params.PhasedExecutionContext, a.params.StateCache)

	return static.NewDestroyer(&static.DestroyerParams{
		SSHClientProvider:    a.params.SSHClientProvider,
		State:                static.NewDestroyState(a.params.StateCache),
		KubeProvider:         a.kubeProvider(),
		LoggerProvider:       a.params.LoggerProvider,
		PhasedActionProvider: phaseProvider,
		TmpDir:               a.params.TmpDir,
	}), nil
}

func (a *abortDestroyerProvider) Incorrect(_ context.Context, metaConfig *config.MetaConfig) (Destroyer, error) {
	return nil, config.UnsupportedClusterTypeErr(metaConfig)
}

func (a *abortDestroyerProvider) kubeProvider() kube.ClientProviderWithCleanup {
	// for abort we cannot use kube api, returns provider which return error
	return newKubeClientErrorProvider("Abort command does not support Kube API")
}
