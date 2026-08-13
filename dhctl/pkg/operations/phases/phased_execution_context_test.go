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

package phases_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

// TestPhasedExecutionContext_NestedRunKeepsEnclosingPhase replays a successful static destroy:
// UpdateStaticDestroyerIPs is run inside AllNodes, once per processed host. The enclosing phase
// must survive the nested one, otherwise the run reports the inner phase as the last completed
// and the final progress collapses.
func TestPhasedExecutionContext_NestedRunKeepsEnclosingPhase(t *testing.T) {
	t.Parallel()

	var (
		phaseEvents    []phases.OnPhaseFuncData[phases.DefaultContextType]
		progressEvents []phases.Progress
	)

	pec := phases.NewDefaultPhasedExecutionContext(
		phases.OperationDestroy,
		func(data phases.OnPhaseFuncData[phases.DefaultContextType]) error {
			phaseEvents = append(phaseEvents, data)

			return nil
		},
		func(progress phases.Progress) error {
			progressEvents = append(progressEvents, progress)

			return nil
		},
	)
	// Static gates BaseInfra out, so AllNodes is the tail of the list.
	pec.SetClusterConfig(phases.ClusterConfig{ClusterType: "Static"})

	pipeline := phases.NewDefaultPipelineWithStateCache(pec, cache.NewTestCache(), phases.WithPipelineName("static destroy"))

	run := func(phase phases.OperationPhase, body func() error) error {
		return pipeline.ActionInPipeline().Run(t.Context(), phase, false, func() (phases.DefaultContextType, error) {
			if body == nil {
				return nil, nil
			}

			return nil, body()
		})
	}

	require.NoError(t, pipeline.Run(t.Context(), func(phases.DefaultPipelinePhaseSwitcher) error {
		for _, phase := range []phases.OperationPhase{
			phases.CreateStaticDestroyerNodeUserPhase,
			phases.WaitStaticDestroyerNodeUserPhase,
			phases.DeleteResourcesPhase,
			phases.SetDeckhouseResourcesDeletedPhase,
		} {
			if err := run(phase, nil); err != nil {
				return err
			}
		}

		return run(phases.AllNodesPhase, func() error {
			return run(phases.UpdateStaticDestroyerIPs, nil)
		})
	}))

	require.NotEmpty(t, phaseEvents)
	lastPhase := phaseEvents[len(phaseEvents)-1]

	assert.Equal(t, phases.OperationPhase(""), lastPhase.NextPhase, "last event must be the pipeline completion")
	assert.Equal(t, phases.AllNodesPhase, lastPhase.CompletedPhase,
		"the nested UpdateStaticDestroyerIPs must not become the completed phase of AllNodes",
	)

	require.NotEmpty(t, progressEvents)
	lastProgress := progressEvents[len(progressEvents)-1]

	assert.Equal(t, phases.AllNodesPhase, lastProgress.CompletedPhase)
	assert.Equal(t, 1.0, lastProgress.Progress)
}

// TestPhasedExecutionContext_OpenPhaseIsNotEnclosing replays commander converge, which starts
// Check and never completes it. A phase left open is not an enclosing phase: the phases that
// follow it are siblings, and the last one of them must be the one the pipeline reports.
func TestPhasedExecutionContext_OpenPhaseIsNotEnclosing(t *testing.T) {
	t.Parallel()

	var phaseEvents []phases.OnPhaseFuncData[phases.DefaultContextType]

	pec := phases.NewDefaultPhasedExecutionContext(
		phases.OperationConverge,
		func(data phases.OnPhaseFuncData[phases.DefaultContextType]) error {
			phaseEvents = append(phaseEvents, data)

			return nil
		},
		nil,
	)
	pec.SetClusterConfig(phases.ClusterConfig{ClusterType: "Cloud"})

	stateCache := cache.NewTestCache()
	require.NoError(t, pec.InitPipeline(t.Context(), stateCache))

	_, err := pec.StartPhase(t.Context(), phases.ConvergeCheckPhase, false, stateCache)
	require.NoError(t, err)

	for _, phase := range []phases.OperationPhase{phases.BaseInfraPhase, phases.AllNodesPhase} {
		_, err := pec.StartPhase(t.Context(), phase, false, stateCache)
		require.NoError(t, err)
		require.NoError(t, pec.CompletePhase(t.Context(), stateCache, nil))
	}

	require.NoError(t, pec.CompletePipeline(t.Context(), stateCache))

	require.NotEmpty(t, phaseEvents)
	lastPhase := phaseEvents[len(phaseEvents)-1]

	assert.Equal(t, phases.OperationPhase(""), lastPhase.NextPhase, "last event must be the pipeline completion")
	assert.Equal(t, phases.AllNodesPhase, lastPhase.CompletedPhase)
}
