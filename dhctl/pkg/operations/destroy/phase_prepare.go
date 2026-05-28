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

package destroy

import (
	"context"

	"github.com/google/uuid"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/directoryconfig"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/kube"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
)

// prepareDestroyPhase is the first phase of destroy: validate that we are
// allowed to destroy (commander UUID safety check), load the meta config,
// and pick the concrete infra destroyer based on the cluster type. The
// returned `prepared` flows through the rest of the pipeline.
type prepareDestroyPhase struct {
	configPreparator metaConfigPopulator
	directoryConfig  *directoryconfig.DirectoryConfig
	infraProvider    *infraDestroyerProvider
	kubeProvider     kube.ClientProviderWithCleanup

	deckhouseState       *deckhouse.State
	phasedActionProvider phases.DefaultActionProvider
	loggerProvider       log.LoggerProvider
	commanderMode        bool
	commanderUUID        uuid.UUID
	skipResources        bool
}

func (p *prepareDestroyPhase) run(ctx context.Context) (prepared, error) {
	if err := deckhouse.CheckCommanderUUID(ctx, deckhouse.Params{
		CommanderMode:        p.commanderMode,
		CommanderUUID:        p.commanderUUID,
		SkipResources:        p.skipResources,
		State:                p.deckhouseState,
		LoggerProvider:       p.loggerProvider,
		KubeProvider:         p.kubeProvider,
		PhasedActionProvider: p.phasedActionProvider,
	}); err != nil {
		return prepared{}, err
	}

	mc, err := p.configPreparator.PopulateMetaConfig(ctx, p.directoryConfig)
	if err != nil {
		return prepared{}, err
	}

	dst, err := config.DoByClusterType(ctx, mc, p.infraProvider)
	if err != nil {
		return prepared{}, err
	}

	return prepared{
		metaConfig:  mc,
		clusterType: mc.ClusterType,
		destroyer:   dst,
	}, nil
}
