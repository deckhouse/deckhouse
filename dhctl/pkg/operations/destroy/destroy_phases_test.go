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
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

type staticMetaConfigPopulator struct{}

func (staticMetaConfigPopulator) PopulateMetaConfig(context.Context, *options.GlobalOptions) (*config.MetaConfig, error) {
	return &config.MetaConfig{ClusterType: config.StaticClusterType}, nil
}

// TestDestroy_CommanderModeAnnouncesStaticPhases pins the order inside ClusterDestroyer.destroy:
// the cluster type must reach the tracker before CheckCommanderUUID announces its phase, or the
// phase list built with an empty cluster type — and shipped in every gRPC frame — loses every
// static-only phase.
func TestDestroy_CommanderModeAnnouncesStaticPhases(t *testing.T) {
	stateCache := cache.NewTestCache()
	kubeProvider := newFakeKubeClientProvider(testCreateFakeKubeClient())

	var frames []phases.Progress

	pec := phases.NewDefaultPhasedExecutionContext(phases.OperationDestroy, nil, func(progress phases.Progress) error {
		frames = append(frames, progress)

		return nil
	})

	pipeline := phases.NewDefaultPipelineWithStateCache(pec, stateCache, phases.WithPipelineName("commander destroy"))
	phaseActionProvider := phases.NewDefaultPhaseActionProviderFromPipeline(pipeline)

	destroyer := &ClusterDestroyer{
		stateCache:       stateCache,
		configPreparator: staticMetaConfigPopulator{},

		pipeline: pipeline,

		d8Destroyer: deckhouse.NewDestroyer(deckhouse.DestroyerParams{
			CommanderMode: true,
			CommanderUUID: uuid.Must(uuid.NewRandom()),

			State: deckhouse.NewState(stateCache),

			Logger:               dhlog.Discard(),
			KubeProvider:         kubeProvider,
			PhasedActionProvider: phaseActionProvider,
		}),

		infraProvider: &infraDestroyerProvider{
			stateCache:           stateCache,
			logger:               dhlog.Discard(),
			kubeProvider:         kubeProvider,
			phasesActionProvider: phaseActionProvider,
			commanderMode:        true,
			sshClientProvider:    testCreateDefaultTestSSHProvider(session.Host{Host: "127.0.0.2", Name: "master-0"}, false),
			tmpDir:               t.TempDir(),
		},
	}

	// The fake cluster has no master nodes, so the run stops in Prepare, right after the phase
	// this test looks at.
	require.Error(t, destroyer.DestroyCluster(t.Context(), true))

	require.NotEmpty(t, frames, "CheckCommanderUUID must announce a phase in commander mode")

	announced := make([]phases.OperationPhase, 0, len(frames[0].Phases))
	for _, p := range frames[0].Phases {
		announced = append(announced, p.Phase)
	}

	assert.Equal(t, []phases.OperationPhase{
		phases.CreateStaticDestroyerNodeUserPhase,
		phases.WaitStaticDestroyerNodeUserPhase,
		phases.DeleteResourcesPhase,
		phases.SetDeckhouseResourcesDeletedPhase,
		phases.UpdateStaticDestroyerIPs,
		phases.AllNodesPhase,
	}, announced)
}
