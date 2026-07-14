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
	"log/slog"

	"github.com/name212/govalue"

	libcon "github.com/deckhouse/lib-connection/pkg"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/controller"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/kube"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/static"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	dhctlstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

type infraDestroyerProvider struct {
	stateCache           dhctlstate.Cache
	logger               *slog.Logger
	kubeProvider         kube.ClientProviderWithCleanup
	phasesActionProvider phases.DefaultActionProvider

	commanderMode      bool
	skipResources      bool
	cloudStateProvider func() (controller.StateLoader, cloud.ClusterInfraDestroyer, error)

	sshClientProvider libcon.SSHProvider
	sshUser           string
	tmpDir            string
	staticLoopsParams static.LoopsParams
}

func (f *infraDestroyerProvider) Cloud(context.Context, *config.MetaConfig) (infraDestroyer, error) {
	if err := f.checkGeneralParams(); err != nil {
		return nil, err
	}

	if govalue.IsNil(f.cloudStateProvider) {
		return nil, fmt.Errorf("Cloud state provider should be provided to infraDestroyerProvider")
	}

	stateLoader, clusterInfra, err := f.cloudStateProvider()
	if err != nil {
		return nil, err
	}

	if govalue.IsNil(stateLoader) {
		return nil, fmt.Errorf("Cloud state loader should be provided by cloudStateProvider")
	}

	if govalue.IsNil(clusterInfra) {
		return nil, fmt.Errorf("Cluster infrastructure should be provided by cloudStateProvider")
	}

	return cloud.NewDestroyer(&cloud.DestroyerParams{
		Logger:       f.logger,
		KubeProvider: f.kubeProvider,
		State:        cloud.NewDestroyState(f.stateCache),

		ClusterInfra: clusterInfra,
		StateLoader:  stateLoader,

		CommanderMode: f.commanderMode,
		SkipResources: f.skipResources,
		SSHUser:       f.sshUser,
	}), nil
}

func (f *infraDestroyerProvider) Static(context.Context, *config.MetaConfig) (infraDestroyer, error) {
	if err := f.checkGeneralParams(); err != nil {
		return nil, err
	}

	if govalue.IsNil(f.sshClientProvider) {
		return nil, fmt.Errorf("SSH client provider should be provided to infraDestroyerProvider")
	}

	return static.NewDestroyer(&static.DestroyerParams{
		SSHClientProvider:    f.sshClientProvider,
		KubeProvider:         f.kubeProvider,
		State:                static.NewDestroyState(f.stateCache),
		Logger:               f.logger,
		PhasedActionProvider: f.phasesActionProvider,

		TmpDir: f.tmpDir,

		Loops: f.staticLoopsParams,
	}), nil
}

func (f *infraDestroyerProvider) Incorrect(_ context.Context, metaConfig *config.MetaConfig) (infraDestroyer, error) {
	return nil, config.UnsupportedClusterTypeErr(metaConfig)
}

func (f *infraDestroyerProvider) checkGeneralParams() error {
	if govalue.IsNil(f.stateCache) {
		return fmt.Errorf("State cache should be provided to infraDestroyerProvider")
	}

	if govalue.IsNil(f.kubeProvider) {
		return fmt.Errorf("Kubernetes provider should be provided to infraDestroyerProvider")
	}

	if govalue.IsNil(f.phasesActionProvider) {
		return fmt.Errorf("Phases action provider should be provided to infraDestroyerProvider")
	}

	if f.tmpDir == "" {
		return fmt.Errorf("Temp directory should be provided to infraDestroyerProvider")
	}

	// wait params can be nil

	return nil
}
