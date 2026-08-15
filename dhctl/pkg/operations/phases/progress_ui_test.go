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
	"context"
	"log/slog"
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

func TestConsumeProgress_BarIsFullWhenTheDeclaredTailDidNotRun(t *testing.T) {
	t.Parallel()

	var events []Progress
	tracker := NewProgressTracker(OperationConverge, func(progress Progress) error {
		events = append(events, progress)

		return nil
	})
	tracker.SetClusterConfig(ClusterConfig{ClusterType: "Static", HasClusterConfiguration: true})

	// CLI converge never announces DeckhouseConfiguration - converger.go puts it into skipPhases
	// for every non-commander run - so the run ends on AllNodes and Complete names AllNodes too.
	require.NoError(t, tracker.Progress("", ConvergeCheckPhase, "", ProgressOpts{}))
	require.NoError(t, tracker.Progress(ConvergeCheckPhase, InstallDeckhousePhase, "", ProgressOpts{}))
	require.NoError(t, tracker.Progress(InstallDeckhousePhase, AllNodesPhase, "", ProgressOpts{}))
	require.NoError(t, tracker.Progress(AllNodesPhase, "", "", ProgressOpts{}))
	require.NoError(t, tracker.Complete(AllNodesPhase))

	recorder := replay(t, events)

	assert.Equal(t, 1.0, recorder.bar())
	// The last phase that ran is named twice, by its own transition and by Complete. Only the
	// first of those is a phase transition; the second exists to carry the bar to the end.
	assert.Equal(t, []string{"Check converge", "Install Deckhouse", "Process all nodes"}, recorder.titles)
}

// replay drives consumeProgress with the recorded events and returns what it logged.
func replay(t *testing.T, events []Progress) *progressRecorder {
	t.Helper()

	recorder := &progressRecorder{}
	progressCh := make(chan Progress)
	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)

		consumeProgress(t.Context(), slog.New(recorder), progressCh, stop)
	}()

	for _, event := range events {
		progressCh <- event
	}

	close(stop)
	<-done

	require.NotEmpty(t, recorder.values, "the bar was never advanced")

	return recorder
}

// progressRecorder collects the bar fractions, which dhlog.Progress puts on the record under the
// progress_value attribute, and the phase titles logged beside them.
type progressRecorder struct {
	values []float64
	titles []string
}

func (r *progressRecorder) bar() float64 { return r.values[len(r.values)-1] }

func (r *progressRecorder) Enabled(context.Context, slog.Level) bool { return true }

func (r *progressRecorder) Handle(_ context.Context, record slog.Record) error {
	isBar := false

	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == "progress_value" {
			r.values = append(r.values, attr.Value.Float64())
			isBar = true
		}

		return true
	})

	if !isBar && record.Message != "progress:end" {
		r.titles = append(r.titles, record.Message)
	}

	return nil
}

func (r *progressRecorder) WithAttrs([]slog.Attr) slog.Handler { return r }

func (r *progressRecorder) WithGroup(string) slog.Handler { return r }

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
