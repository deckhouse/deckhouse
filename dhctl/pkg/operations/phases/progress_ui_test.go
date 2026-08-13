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
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPhaseToString_DeclaredPhasesHaveTitles(t *testing.T) {
	t.Parallel()

	for operation, list := range map[Operation][]PhaseWithSubPhases{
		OperationBootstrap: BootstrapPhases(),
		OperationConverge:  ConvergePhases(),
		OperationDestroy:   DestroyPhases(),
	} {
		for _, phase := range list {
			phaseTitle := strings.TrimSpace(phaseToString(Progress{CurrentPhase: phase.Phase}, false))
			require.NotEmptyf(t, phaseTitle, "%s: phase %q has no title", operation, phase.Phase)

			if phase.Phase == FinalizationPhase {
				// The only phase whose title is literally its value; renaming it is out of scope.
				assert.Equal(t, string(FinalizationPhase), phaseTitle)
			} else {
				assert.NotEqualf(t, string(phase.Phase), phaseTitle,
					"%s: phase %q has no title and falls back to its raw name", operation, phase.Phase,
				)
			}

			for _, subPhase := range phase.SubPhases {
				line := strings.TrimSpace(phaseToString(Progress{CurrentPhase: phase.Phase, CurrentSubPhase: subPhase}, false))

				_, subPhaseTitle, ok := strings.Cut(line, ": ")
				require.Truef(t, ok, "%s: sub-phase %q renders as %q", operation, subPhase, line)
				assert.NotEqualf(t, string(subPhase), subPhaseTitle,
					"%s: sub-phase %q has no title and falls back to its raw name", operation, subPhase,
				)
			}
		}
	}
}

func TestPhaseToString_UnknownNameFallsBackToRawName(t *testing.T) {
	t.Parallel()

	const unknown OperationPhase = "PhaseAddedLater"

	assert.Equal(t, string(unknown), strings.TrimSpace(phaseToString(Progress{CurrentPhase: unknown}, false)))
}

func TestPhaseLists_MatchWhatIsAnnounced(t *testing.T) {
	t.Parallel()

	i := slices.IndexFunc(BootstrapPhases(), func(p PhaseWithSubPhases) bool { return p.Phase == InstallKubernetesPhase })
	require.GreaterOrEqual(t, i, 0, "InstallKubernetes is not declared")

	subPhases := BootstrapPhases()[i].SubPhases
	assert.Equal(t,
		slices.Index(subPhases, InstallKubernetesSubPhaseNodePreparation)+1,
		slices.Index(subPhases, InstallKubernetesSubPhaseModulesPreparation),
		"ModulesPreparation must be declared right after NodePreparation, as it is completed in steps_bashible.go",
	)

	for _, phase := range ConvergePhases() {
		assert.NotEqual(t, ScaleToMultiMasterPhase, phase.Phase, "ScaleToMultiMaster is a converge state value, not a phase")
	}
}
