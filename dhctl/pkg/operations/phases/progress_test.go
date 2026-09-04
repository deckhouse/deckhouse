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
	"bufio"
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
)

var (
	opts     = phases.ProgressOpts{Action: phases.ProgressActionDefault}
	skipOpts = phases.ProgressOpts{Action: phases.ProgressActionSkip}
)

// nth returns the n-th element of s, or the zero value when s is shorter.
func nth[T any](s []T, n int) T {
	if n >= 0 && n < len(s) {
		return s[n]
	}

	var zero T

	return zero
}

// phaseIndex returns the position of the named phase, failing the test when the phase is not declared.
func phaseIndex(t *testing.T, list []phases.PhaseWithSubPhases, name phases.OperationPhase) int {
	t.Helper()

	i := slices.IndexFunc(list, func(p phases.PhaseWithSubPhases) bool { return p.Phase == name })
	require.GreaterOrEqualf(t, i, 0, "phase %q is not declared", name)

	return i
}

// progressAfter is the progress reported once the phase at position i is completed.
func progressAfter(list []phases.PhaseWithSubPhases, i int) float64 {
	return float64(i+1) / float64(len(list))
}

// subPhaseStep is the progress added by completing a single sub-phase of the phase at position i.
func subPhaseStep(list []phases.PhaseWithSubPhases, i int) float64 {
	return (1 / float64(len(list))) / float64(len(nth(list, i).SubPhases))
}

func TestProgressTracker_FindLastCompletedPhase(t *testing.T) {
	t.Parallel()

	list := phases.BootstrapPhases()
	require.GreaterOrEqual(t, len(list), 2)

	first := nth(list, 0).Phase
	second := nth(list, 1).Phase
	beforeLast := nth(list, len(list)-2).Phase
	last := nth(list, len(list)-1).Phase

	progressTracker := phases.NewProgressTracker(phases.OperationBootstrap, nil)

	phase, ok := progressTracker.FindLastCompletedPhase(first, second)
	assert.EqualValues(t, first, phase)
	assert.False(t, ok)

	phase, ok = progressTracker.FindLastCompletedPhase("", first)
	assert.EqualValues(t, "", phase)
	assert.True(t, ok)

	phase, ok = progressTracker.FindLastCompletedPhase("", second)
	assert.EqualValues(t, first, phase)
	assert.True(t, ok)

	phase, ok = progressTracker.FindLastCompletedPhase(beforeLast, "")
	assert.EqualValues(t, beforeLast, phase)
	assert.False(t, ok)

	phase, ok = progressTracker.FindLastCompletedPhase("", last)
	assert.EqualValues(t, beforeLast, phase)
	assert.True(t, ok)
}

func TestProgressTracker(t *testing.T) {
	t.Parallel()

	var result []phases.Progress

	list := phases.BootstrapPhases()
	progressTracker := phases.NewProgressTracker(phases.OperationBootstrap, func(progress phases.Progress) error {
		result = append(result, progress)

		return nil
	})
	// Cloud is what makes the tracker's list the full one: the cloud-only nodes are gated on a
	// positive test, so an unset cluster type keeps BaseInfra and the additional-nodes phase out.
	progressTracker.SetClusterConfig(phases.ClusterConfig{ClusterType: "Cloud", HasClusterConfiguration: true})

	require.NoError(t, progressTracker.Progress("", "", "", opts))
	require.NoError(t, progressTracker.Progress(phases.BaseInfraPhase, "", "", opts))
	require.NoError(t, progressTracker.Progress(phases.InstallKubernetesPhase, "", "", opts))
	require.NoError(t, progressTracker.Progress("", "", phases.InstallDeckhouseSubPhaseConnect, opts))
	require.NoError(t, progressTracker.Progress("", "", phases.InstallDeckhouseSubPhaseInstall, opts))
	require.NoError(t, progressTracker.Progress("", "", phases.InstallDeckhouseSubPhaseWait, opts))
	require.NoError(t, progressTracker.Progress(phases.InstallDeckhousePhase, "", "", opts))
	require.NoError(t, progressTracker.Progress(phases.InstallAdditionalMastersAndStaticNodes, "", "", opts))
	require.NoError(t, progressTracker.Progress(phases.CreateResourcesPhase, "", "", opts))
	require.NoError(t, progressTracker.Progress(phases.ExecPostBootstrapPhase, "", "", opts))
	require.NoError(t, progressTracker.Progress(phases.FinalizationPhase, "", "", opts))

	// do nothing because progress is already 1
	require.NoError(t, progressTracker.Complete(phases.FinalizationPhase))

	var (
		idxBaseInfra        = phaseIndex(t, list, phases.BaseInfraPhase)
		idxInstallK8s       = phaseIndex(t, list, phases.InstallKubernetesPhase)
		idxInstallDeckhouse = phaseIndex(t, list, phases.InstallDeckhousePhase)
		idxAdditionalNodes  = phaseIndex(t, list, phases.InstallAdditionalMastersAndStaticNodes)
		idxCreateResources  = phaseIndex(t, list, phases.CreateResourcesPhase)
		idxPostBootstrap    = phaseIndex(t, list, phases.ExecPostBootstrapPhase)
		idxFinalization     = phaseIndex(t, list, phases.FinalizationPhase)
	)

	deckhouseSubPhases := nth(list, idxInstallDeckhouse).SubPhases
	nodesSubPhases := nth(list, idxAdditionalNodes).SubPhases

	step := subPhaseStep(list, idxInstallDeckhouse)
	afterConnect := progressAfter(list, idxInstallK8s) + step
	afterInstall := afterConnect + step
	afterWait := afterInstall + step

	expected := []phases.Progress{
		{
			Operation:       phases.OperationBootstrap,
			Phases:          list,
			Progress:        0,
			CompletedPhase:  "",
			CurrentPhase:    nth(list, 0).Phase,
			NextPhase:       nth(list, 1).Phase,
			CurrentSubPhase: nth(nth(list, 0).SubPhases, 0),
		},
		{
			Operation:       phases.OperationBootstrap,
			Phases:          list,
			Progress:        progressAfter(list, idxBaseInfra),
			CompletedPhase:  phases.BaseInfraPhase,
			CurrentPhase:    nth(list, idxBaseInfra+1).Phase,
			NextPhase:       nth(list, idxBaseInfra+2).Phase,
			CurrentSubPhase: nth(nth(list, idxBaseInfra+1).SubPhases, 0),
			NextSubPhase:    nth(nth(list, idxBaseInfra+1).SubPhases, 1),
		},
		{
			Operation:       phases.OperationBootstrap,
			Phases:          list,
			Progress:        progressAfter(list, idxInstallK8s),
			CompletedPhase:  phases.InstallKubernetesPhase,
			CurrentPhase:    nth(list, idxInstallK8s+1).Phase,
			NextPhase:       nth(list, idxInstallK8s+2).Phase,
			CurrentSubPhase: nth(deckhouseSubPhases, 0),
			NextSubPhase:    nth(deckhouseSubPhases, 1),
		},
		{
			Operation:         phases.OperationBootstrap,
			Phases:            list,
			Progress:          afterConnect,
			CompletedPhase:    phases.InstallKubernetesPhase,
			CurrentPhase:      nth(list, idxInstallDeckhouse).Phase,
			NextPhase:         nth(list, idxInstallDeckhouse+1).Phase,
			CompletedSubPhase: nth(deckhouseSubPhases, 0),
			CurrentSubPhase:   nth(deckhouseSubPhases, 1),
			NextSubPhase:      nth(deckhouseSubPhases, 2),
		},
		{
			Operation:         phases.OperationBootstrap,
			Phases:            list,
			Progress:          afterInstall,
			CompletedPhase:    phases.InstallKubernetesPhase,
			CurrentPhase:      nth(list, idxInstallDeckhouse).Phase,
			NextPhase:         nth(list, idxInstallDeckhouse+1).Phase,
			CompletedSubPhase: nth(deckhouseSubPhases, 1),
			CurrentSubPhase:   nth(deckhouseSubPhases, 2),
			NextSubPhase:      nth(deckhouseSubPhases, 3),
		},
		{
			Operation:         phases.OperationBootstrap,
			Phases:            list,
			Progress:          afterWait,
			CompletedPhase:    phases.InstallKubernetesPhase,
			CurrentPhase:      nth(list, idxInstallDeckhouse).Phase,
			NextPhase:         nth(list, idxInstallDeckhouse+1).Phase,
			CompletedSubPhase: nth(deckhouseSubPhases, 2),
			CurrentSubPhase:   nth(deckhouseSubPhases, 3),
			NextSubPhase:      nth(deckhouseSubPhases, 4),
		},
		{
			Operation:       phases.OperationBootstrap,
			Phases:          list,
			Progress:        max(progressAfter(list, idxInstallDeckhouse), afterWait),
			CompletedPhase:  phases.InstallDeckhousePhase,
			CurrentPhase:    nth(list, idxInstallDeckhouse+1).Phase,
			NextPhase:       nth(list, idxInstallDeckhouse+2).Phase,
			CurrentSubPhase: nth(nodesSubPhases, 0),
			NextSubPhase:    nth(nodesSubPhases, 1),
		},
		{
			Operation:       phases.OperationBootstrap,
			Phases:          list,
			Progress:        progressAfter(list, idxAdditionalNodes),
			CompletedPhase:  phases.InstallAdditionalMastersAndStaticNodes,
			CurrentPhase:    nth(list, idxAdditionalNodes+1).Phase,
			NextPhase:       nth(list, idxAdditionalNodes+2).Phase,
			CurrentSubPhase: nth(nth(list, idxAdditionalNodes+1).SubPhases, 0),
			NextSubPhase:    nth(nth(list, idxAdditionalNodes+1).SubPhases, 1),
		},
		{
			Operation:       phases.OperationBootstrap,
			Phases:          list,
			Progress:        progressAfter(list, idxCreateResources),
			CompletedPhase:  phases.CreateResourcesPhase,
			CurrentPhase:    nth(list, idxCreateResources+1).Phase,
			NextPhase:       nth(list, idxCreateResources+2).Phase,
			CurrentSubPhase: nth(nth(list, idxCreateResources+1).SubPhases, 0),
			NextSubPhase:    nth(nth(list, idxCreateResources+1).SubPhases, 1),
		},
		{
			Operation:       phases.OperationBootstrap,
			Phases:          list,
			Progress:        progressAfter(list, idxPostBootstrap),
			CompletedPhase:  phases.ExecPostBootstrapPhase,
			CurrentPhase:    nth(list, idxPostBootstrap+1).Phase,
			NextPhase:       nth(list, idxPostBootstrap+2).Phase,
			CurrentSubPhase: nth(nth(list, idxPostBootstrap+1).SubPhases, 0),
			NextSubPhase:    nth(nth(list, idxPostBootstrap+1).SubPhases, 1),
		},
		{
			Operation:      phases.OperationBootstrap,
			Phases:         list,
			Progress:       progressAfter(list, idxFinalization),
			CompletedPhase: phases.FinalizationPhase,
			CurrentPhase:   "",
			NextPhase:      "",
		},
	}

	if !cmp.Equal(expected, result, cmpOpts) {
		t.Errorf("Diff: %v", cmp.Diff(expected, result, cmpOpts))
	}
}

func TestProgressTracker_Complete(t *testing.T) {
	t.Parallel()

	var result []phases.Progress

	list := phases.BootstrapPhases()
	idxBaseInfra := phaseIndex(t, list, phases.BaseInfraPhase)

	progressTracker := phases.NewProgressTracker(phases.OperationBootstrap, func(progress phases.Progress) error {
		result = append(result, progress)

		return nil
	})
	// BaseInfra is a cloud-only node, so the tracker only declares it once the type is known.
	progressTracker.SetClusterConfig(phases.ClusterConfig{ClusterType: "Cloud", HasClusterConfiguration: true})

	require.NoError(t, progressTracker.Progress("", "", "", opts))
	require.NoError(t, progressTracker.Progress(phases.BaseInfraPhase, "", "", opts))
	require.NoError(t, progressTracker.Complete(phases.BaseInfraPhase))

	// everything after BaseInfra is skipped
	lastPhases := phases.BootstrapPhases()
	for i := idxBaseInfra + 1; i < len(lastPhases); i++ {
		lastPhases[i].Action = new(phases.ProgressActionSkip)
	}

	expected := []phases.Progress{
		{
			Operation:       phases.OperationBootstrap,
			Phases:          list,
			Progress:        0,
			CompletedPhase:  "",
			CurrentPhase:    nth(list, 0).Phase,
			NextPhase:       nth(list, 1).Phase,
			CurrentSubPhase: nth(nth(list, 0).SubPhases, 0),
		},
		{
			Operation:       phases.OperationBootstrap,
			Phases:          list,
			Progress:        progressAfter(list, idxBaseInfra),
			CompletedPhase:  phases.BaseInfraPhase,
			CurrentPhase:    nth(list, idxBaseInfra+1).Phase,
			NextPhase:       nth(list, idxBaseInfra+2).Phase,
			CurrentSubPhase: nth(nth(list, idxBaseInfra+1).SubPhases, 0),
			NextSubPhase:    nth(nth(list, idxBaseInfra+1).SubPhases, 1),
		},
		{
			Operation: phases.OperationBootstrap,
			Phases:    lastPhases,
			Progress:  1,
			// Not the tail of the declared list: everything past BaseInfra is marked skipped, and a
			// phase that never ran cannot be reported as the one the operation completed.
			CompletedPhase: phases.BaseInfraPhase,
			CurrentPhase:   "",
			NextPhase:      "",
		},
	}

	if !cmp.Equal(expected, result, cmpOpts) {
		t.Errorf("Diff: %v", cmp.Diff(expected, result, cmpOpts))
	}
}

func TestProgressTracker_Complete_ZeroProgress(t *testing.T) {
	t.Parallel()

	var result []phases.Progress

	progressTracker := phases.NewProgressTracker(phases.OperationBootstrap, func(progress phases.Progress) error {
		result = append(result, progress)
		return nil
	})

	require.NoError(t, progressTracker.Progress("", "", "", skipOpts))
	require.NoError(t, progressTracker.Complete(""))

	assert.EqualValues(t, 0, result[len(result)-1].Progress)
}

func TestProgressTracker_NilCallback(t *testing.T) {
	t.Parallel()

	list := phases.BootstrapPhases()
	progressTracker := phases.NewProgressTracker(phases.OperationBootstrap, nil)

	require.NoError(t, progressTracker.Progress("", "", "", opts))
	require.NoError(t, progressTracker.Progress(nth(list, len(list)-1).Phase, "", "", opts))
}

func TestProgressTracker_Skip(t *testing.T) {
	t.Parallel()

	var result []phases.Progress

	progressTracker := phases.NewProgressTracker(phases.OperationDestroy, func(progress phases.Progress) error {
		result = append(result, progress)

		return nil
	})
	progressTracker.SetClusterConfig(phases.ClusterConfig{ClusterType: "Cloud", HasClusterConfiguration: true})

	require.NoError(t, progressTracker.Progress("", "", "", opts))
	require.NoError(t, progressTracker.Progress(phases.AllNodesPhase, "", "", skipOpts))
	require.NoError(t, progressTracker.Progress(phases.BaseInfraPhase, "", "", opts))

	// Cloud destroy: the three static-only phases are gated out.
	destroyPhases := []phases.PhaseWithSubPhases{
		{Phase: phases.DeleteResourcesPhase},
		{Phase: phases.SetDeckhouseResourcesDeletedPhase},
		{Phase: phases.AllNodesPhase},
		{Phase: phases.BaseInfraPhase},
	}
	skippedDestroyPhases := []phases.PhaseWithSubPhases{
		{Phase: phases.DeleteResourcesPhase, Action: &skipOpts.Action},
		{Phase: phases.SetDeckhouseResourcesDeletedPhase, Action: &skipOpts.Action},
		{Phase: phases.AllNodesPhase, Action: &skipOpts.Action},
		{Phase: phases.BaseInfraPhase},
	}

	idxAllNodes := phaseIndex(t, destroyPhases, phases.AllNodesPhase)
	idxBaseInfra := phaseIndex(t, destroyPhases, phases.BaseInfraPhase)

	expected := []phases.Progress{
		{
			Operation:    phases.OperationDestroy,
			Progress:     0,
			CurrentPhase: nth(destroyPhases, 0).Phase,
			NextPhase:    nth(destroyPhases, 1).Phase,
			Phases:       destroyPhases,
		},
		{
			Operation:      phases.OperationDestroy,
			Progress:       progressAfter(destroyPhases, idxAllNodes),
			CompletedPhase: phases.AllNodesPhase,
			CurrentPhase:   phases.BaseInfraPhase,
			Phases:         skippedDestroyPhases,
		},
		{
			Operation:      phases.OperationDestroy,
			Progress:       progressAfter(destroyPhases, idxBaseInfra),
			CompletedPhase: phases.BaseInfraPhase,
			Phases:         skippedDestroyPhases,
		},
	}

	if !cmp.Equal(expected, result, cmpOpts) {
		t.Errorf("Diff: %v", cmp.Diff(expected, result, cmpOpts))
	}
}

// TestProgressTracker_Progress_UndeclaredPhase pins the diagnostic for the case that used to pass
// in silence: a phase announcing itself without being declared for the operation moves nothing,
// and said nothing about it. Later tasks drop sub-phases whose producer is gone and rely on this
// warning to show up when a caller is left behind.
//
// Not parallel: it swaps the default logger, which is process-global.
func TestProgressTracker_Progress_UndeclaredPhase(t *testing.T) {
	var buf bytes.Buffer

	prev := slog.Default()

	slog.SetDefault(dhlog.NewBufferLogger(&buf))
	t.Cleanup(func() { slog.SetDefault(prev) })

	var result []phases.Progress

	progressTracker := phases.NewProgressTracker(phases.OperationBootstrap, func(progress phases.Progress) error {
		result = append(result, progress)

		return nil
	})
	// BaseInfra has to be a declared phase here: it is the one that must move the progress, so
	// that the two undeclared names below can be shown not to.
	progressTracker.SetClusterConfig(phases.ClusterConfig{ClusterType: "Cloud", HasClusterConfiguration: true})

	require.NoError(t, progressTracker.Progress(phases.BaseInfraPhase, "", "", opts))
	require.NoError(t, progressTracker.Progress("NoSuchPhase", "", "", opts))
	require.NoError(t, progressTracker.Progress("", "", "NoSuchSubPhase", opts))

	require.Len(t, result, 3)
	assert.Equal(t, result[0].Progress, result[1].Progress)
	assert.Equal(t, result[0].Progress, result[2].Progress)
	assert.EqualValues(t, phases.BaseInfraPhase, result[2].CompletedPhase)

	assert.Contains(t, buf.String(), "NoSuchPhase")
	assert.Contains(t, buf.String(), "NoSuchSubPhase")
}

func TestProgressTracker_WriteProgress(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	progressFile := "progress.jsonl"
	progressFilePath := filepath.Join(tmpDir, progressFile)

	list := phases.BootstrapPhases()
	progressTracker := phases.NewProgressTracker(
		phases.OperationBootstrap,
		phases.WriteProgress(progressFilePath),
	)
	// The expectation is built from the ungated BootstrapPhases, which is the Cloud list.
	progressTracker.SetClusterConfig(phases.ClusterConfig{ClusterType: "Cloud", HasClusterConfiguration: true})

	require.NoError(t, progressTracker.Progress("", "", "", opts))
	require.NoError(t, progressTracker.Progress(nth(list, len(list)-1).Phase, "", "", opts))

	result := readJSONLinesFromFile(t, progressFilePath)
	expected := []phases.Progress{
		{
			Operation:       phases.OperationBootstrap,
			Phases:          list,
			Progress:        0,
			CompletedPhase:  "",
			CurrentPhase:    nth(list, 0).Phase,
			NextPhase:       nth(list, 1).Phase,
			CurrentSubPhase: nth(nth(list, 0).SubPhases, 0),
		},
		{
			Operation:      phases.OperationBootstrap,
			Phases:         list,
			Progress:       progressAfter(list, len(list)-1),
			CompletedPhase: nth(list, len(list)-1).Phase,
			CurrentPhase:   "",
			NextPhase:      "",
		},
	}

	if !cmp.Equal(expected, result, cmpOpts) {
		t.Errorf("Diff: %v", cmp.Diff(expected, result, cmpOpts))
	}
}

func TestProgressTracker_Progress_ExcludesPhase(t *testing.T) {
	t.Parallel()

	var result []phases.Progress
	progressTracker := phases.NewProgressTracker(phases.OperationBootstrap, func(progress phases.Progress) error {
		result = append(result, progress)
		return nil
	})
	progressTracker.SetClusterConfig(phases.ClusterConfig{ClusterType: "Static", HasClusterConfiguration: true})

	require.NoError(t, progressTracker.Progress("", "", "", opts))
	require.Len(t, result, 1)

	phaseNames := make([]string, 0, len(result[0].Phases))
	for _, p := range result[0].Phases {
		phaseNames = append(phaseNames, string(p.Phase))
	}

	assert.NotContains(t, phaseNames, string(phases.BaseInfraPhase),
		"BaseInfraPhase must not appear in progress for Bootstrap Static",
	)
}

func TestProgressTracker_Progress_CurrentPhase(t *testing.T) {
	t.Parallel()

	var result []phases.Progress
	progressTracker := phases.NewProgressTracker(phases.OperationBootstrap, func(progress phases.Progress) error {
		result = append(result, progress)
		return nil
	})
	progressTracker.SetClusterConfig(phases.ClusterConfig{ClusterType: "Static", HasClusterConfiguration: true})

	require.NoError(t, progressTracker.Progress(phases.InstallDeckhousePhase, phases.CreateResourcesPhase, "", opts))
	require.Len(t, result, 1)

	p := result[0]
	assert.Equal(t, string(phases.InstallDeckhousePhase), string(p.CompletedPhase))
	assert.Equal(t, string(phases.CreateResourcesPhase), string(p.CurrentPhase))
	assert.Equal(t, string(phases.ExecPostBootstrapPhase), string(p.NextPhase))

	// The phase jumped over must be marked as skipped. On a static cluster that is the
	// control-plane-manager wait: installing additional nodes is cloud-only work, and its node is
	// gated out here entirely - it cannot be skipped because it was never declared.
	names := make([]phases.OperationPhase, 0, len(p.Phases))
	for _, phase := range p.Phases {
		names = append(names, phase.Phase)
	}

	assert.NotContains(t, names, phases.InstallAdditionalMastersAndStaticNodes,
		"installing additional masters and CloudPermanent nodes is cloud-only and must not be announced on a static cluster",
	)

	waitPhase := nth(p.Phases, phaseIndex(t, p.Phases, phases.WaitForControlPlaneManagerReadinessPhase))

	require.NotNil(t, waitPhase.Action)
	assert.Equal(t, phases.ProgressActionSkip, *waitPhase.Action)
}

// walkSkipping replays what --skip-phase does to the progress tracker. The bootstrap traverser
// announces every declared node but the named ones, and a skipped node is never mentioned to the
// phase context at all - that is the whole of the feature as far as this package is concerned, so
// it is the whole of what these tests drive.
func walkSkipping(t *testing.T, skipped ...phases.OperationPhase) []phases.Progress {
	t.Helper()

	var result []phases.Progress

	pec := phases.NewDefaultPhasedExecutionContext(phases.OperationBootstrap, nil, func(progress phases.Progress) error {
		result = append(result, progress)

		return nil
	})
	// BootstrapPhases below is the ungated list, and the ungated list is the cloud one.
	pec.SetClusterConfig(phases.ClusterConfig{ClusterType: "Cloud", HasClusterConfiguration: true})

	stateCache := cache.NewTestCache()
	require.NoError(t, pec.InitPipeline(t.Context(), stateCache))

	announced := false

	for _, declared := range phases.BootstrapPhases() {
		if slices.Contains(skipped, declared.Phase) {
			continue
		}

		var err error
		if announced {
			_, err = pec.SwitchPhase(t.Context(), declared.Phase, false, stateCache, nil)
		} else {
			_, err = pec.StartPhase(t.Context(), declared.Phase, false, stateCache)
			announced = true
		}
		require.NoError(t, err)
	}

	require.NoError(t, pec.CompletePhaseAndPipeline(t.Context(), stateCache, nil))
	require.NoError(t, pec.Finalize(t.Context(), stateCache))

	return result
}

// TestProgress_UserSkippedPhaseStaysDeclaredAndMarkedSkipped pins the representation a phase left
// out by --skip-phase gets on the wire, which is the one the walk gets for free: the node stays in
// the announced list - where a node the gates exclude is absent from it altogether - and carries
// ProgressActionSkip, deliberately the same string an early stop and a resume already use. A new
// action would render on an older Commander as "waiting".
func TestProgress_UserSkippedPhaseStaysDeclaredAndMarkedSkipped(t *testing.T) {
	t.Parallel()

	result := walkSkipping(t, phases.CreateResourcesPhase)
	final := nth(result, len(result)-1)

	assert.Equal(t, 1.0, final.Progress)
	phaseIndex(t, final.Phases, phases.CreateResourcesPhase)

	for _, phase := range final.Phases {
		require.NotNilf(t, phase.Action, "phase %q was announced without an action", phase.Phase)

		expected := phases.ProgressActionDefault
		if phase.Phase == phases.CreateResourcesPhase {
			expected = phases.ProgressActionSkip
		}

		assert.Equalf(t, expected, *phase.Action, "phase %q", phase.Phase)
	}
}

// TestProgress_CompletedPhaseIsTheLastPhaseThatRan covers the tail of the list, which is where
// skipping by not announcing loses its way. Complete named the last declared phase whatever had
// become of it, so a run with the tail skipped finished by reporting a phase that never ran as the
// one it had completed.
func TestProgress_CompletedPhaseIsTheLastPhaseThatRan(t *testing.T) {
	t.Parallel()

	result := walkSkipping(t, phases.ExecPostBootstrapPhase, phases.FinalizationPhase)
	final := nth(result, len(result)-1)

	assert.EqualValues(t, phases.CreateResourcesPhase, final.CompletedPhase)
	assert.Equal(t, 1.0, final.Progress)

	for _, name := range []phases.OperationPhase{phases.ExecPostBootstrapPhase, phases.FinalizationPhase} {
		phase := nth(final.Phases, phaseIndex(t, final.Phases, name))
		require.NotNilf(t, phase.Action, "phase %q was announced without an action", name)
		assert.Equalf(t, phases.ProgressActionSkip, *phase.Action, "phase %q", name)
	}
}

// TestProgressTracker_NoClusterConfigurationIsAnAnswerAndNotAnAbsence covers the one config that
// equals the zero value the tracker is built with: a cluster whose control plane dhctl did not
// create carries no ClusterConfiguration, so both fields stay empty. The list it announces must
// drop the phases that build a control plane and keep the ones dhctl still runs on such a
// cluster - and the call must resolve the list rather than recognise its own zero value and
// return, leaving whatever progress had already written on the wire.
func TestProgressTracker_NoClusterConfigurationIsAnAnswerAndNotAnAbsence(t *testing.T) {
	t.Parallel()

	declared := declaredPhases(t, phases.OperationBootstrap, phases.ClusterConfig{})

	assert.NotContains(t, declared, phases.InstallKubernetesPhase)
	assert.NotContains(t, declared, phases.PostInfraPreflightsPhase)
	assert.Contains(t, declared, phases.InstallDeckhousePhase)
	assert.Contains(t, declared, phases.CreateResourcesPhase)

	var reported phases.Progress

	tracker := phases.NewProgressTracker(phases.OperationBootstrap, func(progress phases.Progress) error {
		reported = progress

		return nil
	})

	require.NoError(t, tracker.Progress(phases.PreparationPhase, "", "", opts))
	require.NotNil(t, nth(reported.Phases, 0).Action, "the fixture: bootstrap announces Preparation before the config it loads is known")

	tracker.SetClusterConfig(phases.ClusterConfig{})
	require.NoError(t, tracker.Progress("", "", "", opts))

	assert.Nil(t, nth(reported.Phases, 0).Action, "the list was not resolved again, so the gates never ran")
}

// declaredPhases returns the phase list the tracker reports for the given cluster config.
// The gated list is not exported, so it is read back from a progress event, the same way it
// reaches Commander.
func declaredPhases(t *testing.T, operation phases.Operation, cfg phases.ClusterConfig) []phases.OperationPhase {
	t.Helper()

	var reported []phases.PhaseWithSubPhases

	progressTracker := phases.NewProgressTracker(operation, func(progress phases.Progress) error {
		reported = progress.Phases

		return nil
	})
	progressTracker.SetClusterConfig(cfg)
	require.NoError(t, progressTracker.Progress("", "", "", opts))

	names := make([]phases.OperationPhase, 0, len(reported))
	for _, p := range reported {
		names = append(names, p.Phase)
	}

	return names
}

func TestDestroyPhases_DeclaredPerClusterType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		clusterType string
		expected    []phases.OperationPhase
	}{
		{
			clusterType: "Static",
			// Announcement order, not execution order: both node-user phases are announced from
			// Prepare, ahead of DeleteResources, and UpdateStaticDestroyerIPs runs inside AllNodes.
			expected: []phases.OperationPhase{
				phases.CreateStaticDestroyerNodeUserPhase,
				phases.WaitStaticDestroyerNodeUserPhase,
				phases.DeleteResourcesPhase,
				phases.SetDeckhouseResourcesDeletedPhase,
				phases.UpdateStaticDestroyerIPs,
				phases.AllNodesPhase,
			},
		},
		{
			clusterType: "Cloud",
			expected: []phases.OperationPhase{
				phases.DeleteResourcesPhase,
				// Finalize is called for both cluster types, so this phase is never gated.
				phases.SetDeckhouseResourcesDeletedPhase,
				phases.AllNodesPhase,
				phases.BaseInfraPhase,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.clusterType, func(t *testing.T) {
			t.Parallel()

			declared := declaredPhases(t, phases.OperationDestroy, phases.ClusterConfig{ClusterType: tt.clusterType, HasClusterConfiguration: true})

			assert.Equal(t, tt.expected, declared)
			assert.NotContains(t, declared, phases.CommanderUUIDWasChecked,
				"CommanderUUIDWasChecked is announced only in commander mode and only on the first attempt",
			)
		})
	}
}

func readJSONLinesFromFile(t *testing.T, filename string) []phases.Progress {
	t.Helper()

	file, err := os.Open(filename)
	require.NoError(t, err)

	defer file.Close()

	var result []phases.Progress
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		var data phases.Progress
		line := scanner.Text()

		require.NoError(t, json.Unmarshal([]byte(line), &data))

		result = append(result, data)
	}

	require.NoError(t, scanner.Err())

	return result
}

var cmpOpts = cmp.Options{
	// cmp panics on any unexported field reached from this package. PhaseWithSubPhases has none:
	// it is only the projection of the phase tree onto the wire, and the gating (and, later, the
	// node callbacks) live on the unexported tree node instead. This keeps that a guard, not a
	// live exclusion, and it stays wildcard-free on purpose.
	cmpopts.IgnoreUnexported(phases.PhaseWithSubPhases{}),

	cmp.Comparer(func(x, y *phases.ProgressAction) bool {
		if x == nil && (y != nil && *y == "") {
			return true
		}

		if y == nil && (x != nil && *x == "") {
			return true
		}

		return cmp.Equal(y, x)
	}),
}
