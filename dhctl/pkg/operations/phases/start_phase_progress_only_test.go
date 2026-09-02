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

package phases_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	utilcache "github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

// TestStartPhaseProgressOnly_FirstReportedStateCarriesTheClusterUUID replays what bootstrap does
// around the Preparation node: announce it while no state cache exists yet, let its body create
// the cache and mint the cluster uuid into it, then announce the next node.
//
// Every state reported through onPhaseFunc must carry the uuid. Commander reads it on every tick
// and fails the whole operation with "got unexpected empty deckhouse uuid from the cluster state"
// when it is missing - which is what announcing Preparation with StartPhase used to produce, since
// the only cache available at that point is the DummyCache that returns nothing.
func TestStartPhaseProgressOnly_FirstReportedStateCarriesTheClusterUUID(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	var reported []phases.OnPhaseFuncData[phases.DefaultContextType]

	pec := phases.NewDefaultPhasedExecutionContext(
		phases.OperationBootstrap,
		func(data phases.OnPhaseFuncData[phases.DefaultContextType]) error {
			reported = append(reported, data)

			return nil
		},
		nil,
	)
	pec.SetClusterConfig(phases.ClusterConfig{ClusterType: "Cloud", HasClusterConfiguration: true})

	// Preparation is announced before anything has built a state cache.
	pec.StartPhaseProgressOnly(ctx, phases.PreparationPhase)
	require.Empty(t, reported, "Preparation is announced progress-only, so nothing is reported for it")

	// The Preparation body: the cache exists from here on, and the uuid is in it.
	stateCache := utilcache.NewTestCache()
	require.NoError(t, stateCache.Save(ctx, "uuid", []byte("cluster-uuid-1234")))
	require.NoError(t, pec.InitPipeline(ctx, stateCache))

	// The walk announces every remaining node with SwitchPhase.
	shouldStop, err := pec.SwitchPhase(ctx, phases.PreInfraPreflightsPhase, true, stateCache, nil)
	require.NoError(t, err)
	require.False(t, shouldStop)

	require.Len(t, reported, 1, "the first node reported is the one after Preparation")
	require.Equal(t, []byte("cluster-uuid-1234"), reported[0].CompletedPhaseState["uuid"],
		"the first state reported must carry the cluster uuid",
	)
	// Preparation ran, so it must be reported as completed and not left to be marked skipped.
	require.Equal(t, phases.PreparationPhase, reported[0].CompletedPhase)
	require.Equal(t, phases.PreInfraPreflightsPhase, reported[0].NextPhase)
}
