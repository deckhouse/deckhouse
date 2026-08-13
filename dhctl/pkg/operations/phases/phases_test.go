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

package phases

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProjection pins what leaves dhctl for Commander and for the user's --progress-log-file-path
// JSONL: the gated tree, projected. The reference strings were captured before the tree type
// existed, from json.Marshal(operationPhases(...)) of the flat list, so a diff here means the
// wire moved. Phase names are frozen (they travel as raw strings over gRPC), so a task that
// changes the declared tree on purpose updates these strings in its own step.
func TestProjection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		operation   Operation
		clusterType string
		expected    string
	}{
		{
			operation:   OperationBootstrap,
			clusterType: "Cloud",
			expected:    `[{"phase":"Preparation","subPhases":["ImagesDownload","ConfigValidation","CachePreparation","StatePreparation"]},{"phase":"PreInfraPreflights"},{"phase":"BaseInfra","subPhases":["BaseInfra"]},{"phase":"FirstMaster"},{"phase":"PostInfraPreflights"},{"phase":"InstallKubernetes","subPhases":["BashibleBundlePrepartion","RegistryPackagesProxy","NodePreparation","ModulesPreparation","ExecuteBashibleBundle"]},{"phase":"InstallDeckhouse","subPhases":["ConnectToMaster","InstallDeckhouse","WaitForFirstMasterReady"]},{"phase":"InstallAdditionalMastersAndStaticNodes","subPhases":["AdditionalMasters","StaticNodes"]},{"phase":"WaitForControlPlaneManagerReadiness"},{"phase":"CreateResources"},{"phase":"ExecPostBootstrap"},{"phase":"Finalization"}]`,
		},
		{
			operation:   OperationBootstrap,
			clusterType: "Static",
			expected:    `[{"phase":"Preparation","subPhases":["ImagesDownload","ConfigValidation","CachePreparation","StatePreparation"]},{"phase":"PreInfraPreflights"},{"phase":"PostInfraPreflights"},{"phase":"InstallKubernetes","subPhases":["BashibleBundlePrepartion","RegistryPackagesProxy","NodePreparation","ModulesPreparation","ExecuteBashibleBundle"]},{"phase":"InstallDeckhouse","subPhases":["ConnectToMaster","InstallDeckhouse","WaitForFirstMasterReady"]},{"phase":"WaitForControlPlaneManagerReadiness"},{"phase":"CreateResources"},{"phase":"ExecPostBootstrap"},{"phase":"Finalization"}]`,
		},
		{
			operation:   OperationConverge,
			clusterType: "Cloud",
			expected:    `[{"phase":"Check","subPhases":["CheckInfra","CheckConfiguration"]},{"phase":"BaseInfra"},{"phase":"InstallDeckhouse"},{"phase":"AllNodes"},{"phase":"DeckhouseConfiguration"}]`,
		},
		{
			operation:   OperationConverge,
			clusterType: "Static",
			expected:    `[{"phase":"Check","subPhases":["CheckConfiguration"]},{"phase":"InstallDeckhouse"},{"phase":"AllNodes"},{"phase":"DeckhouseConfiguration"}]`,
		},
		{
			operation:   OperationCheck,
			clusterType: "Cloud",
			expected:    `[{"phase":"CheckInfra"},{"phase":"CheckConfiguration"}]`,
		},
		{
			operation:   OperationCheck,
			clusterType: "Static",
			expected:    `[{"phase":"CheckInfra"},{"phase":"CheckConfiguration"}]`,
		},
		{
			operation:   OperationDestroy,
			clusterType: "Cloud",
			expected:    `[{"phase":"DeleteResources"},{"phase":"SetDeckhouseResourcesDelete"},{"phase":"AllNodes"},{"phase":"BaseInfra"}]`,
		},
		{
			operation:   OperationDestroy,
			clusterType: "Static",
			expected:    `[{"phase":"CreateStaticDestroyerNodeUser"},{"phase":"WaitStaticDestroyerNodeUser"},{"phase":"DeleteResources"},{"phase":"SetDeckhouseResourcesDelete"},{"phase":"UpdateStaticDestroyerIPs"},{"phase":"AllNodes"}]`,
		},
		{
			operation:   OperationCommanderAttach,
			clusterType: "Cloud",
			expected:    `[{"phase":"Scan"},{"phase":"Capture"},{"phase":"Check","subPhases":["CheckInfra","CheckConfiguration"]}]`,
		},
		{
			operation:   OperationCommanderAttach,
			clusterType: "Static",
			expected:    `[{"phase":"Scan"},{"phase":"Capture"},{"phase":"Check","subPhases":["CheckInfra","CheckConfiguration"]}]`,
		},
		{
			operation:   OperationCommanderDetach,
			clusterType: "Cloud",
			expected:    `[{"phase":"Check","subPhases":["CheckInfra","CheckConfiguration"]},{"phase":"Detach"}]`,
		},
		{
			operation:   OperationCommanderDetach,
			clusterType: "Static",
			expected:    `[{"phase":"Check","subPhases":["CheckInfra","CheckConfiguration"]},{"phase":"Detach"}]`,
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.operation)+"/"+tt.clusterType, func(t *testing.T) {
			t.Parallel()

			opts := phasesOpts{clusterConfig: ClusterConfig{ClusterType: tt.clusterType}}

			actual, err := json.Marshal(operationPhases(tt.operation, opts))
			require.NoError(t, err)

			assert.Equal(t, tt.expected, string(actual), "field order and omitted fields are part of the wire format")
		})
	}
}

// TestProjection_UnknownOperation guards the empty case: an unknown operation must project to an
// empty list and not to nil, because Progress.phases carries no omitempty and Commander would
// see null instead of [].
func TestProjection_UnknownOperation(t *testing.T) {
	t.Parallel()

	actual, err := json.Marshal(operationPhases(Operation("Nonexistent"), phasesOpts{}))
	require.NoError(t, err)

	assert.Equal(t, "[]", string(actual))
}

// TestCheckInfra_DeclaredWhereTheClusterTypeIsNeverResolved is the other half of the positive
// Cloud gate. Check, commander-attach and commander-detach never call SetClusterConfig, so their
// options stay empty for the whole run: a gate on CheckInfra there would drop it from every list
// while Checker.checkInfra kept announcing it, which buys an undeclared announcement instead of an
// honest one. Only converge, which resolves the cluster type before anything is reported, gates it.
func TestCheckInfra_DeclaredWhereTheClusterTypeIsNeverResolved(t *testing.T) {
	t.Parallel()

	for _, operation := range []Operation{OperationCheck, OperationCommanderAttach, OperationCommanderDetach} {
		t.Run(string(operation), func(t *testing.T) {
			t.Parallel()

			assert.Contains(t, declaredNames(operationPhases(operation, phasesOpts{})), CheckInfra)
		})
	}
}

// declaredNames flattens a projection into the names it declares, phases and sub-phases alike.
func declaredNames(list []PhaseWithSubPhases) []OperationPhase {
	names := make([]OperationPhase, 0, len(list))
	for _, phase := range list {
		names = append(names, phase.Phase)
		names = append(names, phase.SubPhases...)
	}

	return names
}

// TestPhaseTreeDepth is the precondition that makes project complete rather than lossy: the wire
// format holds two levels, so a node with grandchildren would silently lose them.
func TestPhaseTreeDepth(t *testing.T) {
	t.Parallel()

	for name, nodes := range map[string][]node{
		"bootstrap":       bootstrapNodes(),
		"converge":        convergeNodes(),
		"check":           checkNodes(),
		"destroy":         destroyNodes(),
		"commanderAttach": commanderAttachNodes(),
		"commanderDetach": commanderDetachNodes(),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			for _, phase := range nodes {
				for _, subPhase := range phase.Children {
					assert.Emptyf(t, subPhase.Children,
						"%s/%s has children: the wire format is two levels deep, a third one needs Commander's agreement first",
						phase.Name, subPhase.Name,
					)
				}
			}
		})
	}
}

// TestGateRecursesIntoChildren pins the recursion, which no tree exercises today: no sub-phase
// node carries a gate yet, and the assertion would pass vacuously against a top-level-only gate.
func TestGateRecursesIntoChildren(t *testing.T) {
	t.Parallel()

	tree := []node{{
		Name: InstallKubernetesPhase,
		Children: []node{
			{Name: InstallKubernetesSubPhaseNodePreparation},
			{Name: InstallKubernetesSubPhaseModulesPreparation, includeIf: ifStatic},
		},
	}}

	cloud := project(gate(tree, phasesOpts{clusterConfig: ClusterConfig{ClusterType: "Cloud"}}))
	require.Len(t, cloud, 1)
	assert.Equal(t, []OperationSubPhase{InstallKubernetesSubPhaseNodePreparation}, cloud[0].SubPhases)

	static := project(gate(tree, phasesOpts{clusterConfig: ClusterConfig{ClusterType: "Static"}}))
	require.Len(t, static, 1)
	assert.Len(t, static[0].SubPhases, 2)
}
