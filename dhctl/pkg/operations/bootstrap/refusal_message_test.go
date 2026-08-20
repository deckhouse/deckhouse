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

package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
)

// TestRefusalNamesTheClusterItResolvedTheTreeFor pins the text of both refusals, because the text
// is the whole of what they do. A bootstrap of a cluster dhctl did not create has no cluster type
// to name - the type comes from the ClusterConfiguration such a run has none of - and the refusal
// used to print it anyway, asking the user which "" cluster they meant.
func TestRefusalNamesTheClusterItResolvedTheTreeFor(t *testing.T) {
	t.Run("a cluster with no ClusterConfiguration is named without inventing a type", func(t *testing.T) {
		cfg := phases.ClusterConfig{}

		require.EqualError(t,
			refuseIfSkipExcluded([]phases.OperationPhase{phases.BaseInfraPhase}, cfg),
			`--skip-phase "BaseInfra" names a phase that is not part of the bootstrap of this cluster`,
		)

		require.EqualError(t,
			refuseIfExcluded(phases.InstallKubernetesPhase, cfg),
			`phase "InstallKubernetes" was requested explicitly, but it is excluded from the bootstrap of this cluster`,
		)
	})

	t.Run("a cluster that has one keeps being named by its type", func(t *testing.T) {
		cfg := phases.ClusterConfig{ClusterType: config.StaticClusterType, HasClusterConfiguration: true}

		require.EqualError(t,
			refuseIfSkipExcluded([]phases.OperationPhase{phases.BaseInfraPhase}, cfg),
			`--skip-phase "BaseInfra" names a phase that is not part of the bootstrap of a "Static" cluster`,
		)

		require.EqualError(t,
			refuseIfExcluded(phases.BaseInfraPhase, cfg),
			`phase "BaseInfra" was requested explicitly, but it is excluded from the bootstrap of a "Static" cluster`,
		)
	})
}
