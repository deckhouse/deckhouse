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

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/controller"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/kube"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/static"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	dhctlstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/sshclient"
)

type infraDestroyerProvider struct {
	stateCache     dhctlstate.Cache
	loggerProvider log.LoggerProvider
	kubeProvider   kube.ClientProviderWithCleanup

	commanderMode bool
	skipResources bool

	cloudStateProvider func() (controller.StateLoader, *controller.ClusterInfra, error)

	sshClientProvider    sshclient.SSHProvider
	phasesActionProvider phases.DefaultActionProvider
	tmpDir               string
}

func (f *infraDestroyerProvider) Cloud(context.Context, *config.MetaConfig) (infraDestroyer, error) {
	stateLoader, clusterInfra, err := f.cloudStateProvider()
	if err != nil {
		return nil, err
	}

	return cloud.NewDestroyer(&cloud.DestroyerParams{
		LoggerProvider: f.loggerProvider,
		KubeProvider:   f.kubeProvider,
		State:          cloud.NewDestroyState(f.stateCache),

		ClusterInfra: clusterInfra,
		StateLoader:  stateLoader,

		CommanderMode: f.commanderMode,
		SkipResources: f.skipResources,
	}), nil
}

func (f *infraDestroyerProvider) Static(context.Context, *config.MetaConfig) (infraDestroyer, error) {
	return static.NewDestroyer(&static.DestroyerParams{
		SSHClientProvider:    f.sshClientProvider,
		State:                static.NewDestroyState(f.stateCache),
		KubeProvider:         f.kubeProvider,
		LoggerProvider:       f.loggerProvider,
		PhasedActionProvider: f.phasesActionProvider,
		TmpDir:               f.tmpDir,
	}), nil
}

func (f *infraDestroyerProvider) Incorrect(_ context.Context, metaConfig *config.MetaConfig) (infraDestroyer, error) {
	return nil, config.UnsupportedClusterTypeErr(metaConfig)
}
