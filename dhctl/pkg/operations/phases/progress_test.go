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
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
)

var (
	opts     = phases.ProgressOpts{Action: phases.ProgressActionDefault}
	skipOpts = phases.ProgressOpts{Action: phases.ProgressActionSkip}
)

func TestProgressTracker_FindLastCompletedPhase(t *testing.T) {
	t.Parallel()

	progressTracker := phases.NewProgressTracker(phases.OperationBootstrap, nil)

	phase, ok := progressTracker.FindLastCompletedPhase(phases.BootstrapPhases()[0].Phase, phases.BootstrapPhases()[1].Phase)
	assert.EqualValues(t, phases.BootstrapPhases()[0].Phase, phase)
	assert.False(t, ok)

	phase, ok = progressTracker.FindLastCompletedPhase("", phases.BootstrapPhases()[0].Phase)
	assert.EqualValues(t, "", phase)
	assert.True(t, ok)

	phase, ok = progressTracker.FindLastCompletedPhase("", phases.BootstrapPhases()[1].Phase)
	assert.EqualValues(t, phases.BootstrapPhases()[0].Phase, phase)
	assert.True(t, ok)

	phase, ok = progressTracker.FindLastCompletedPhase(phases.BootstrapPhases()[len(phases.BootstrapPhases())-2].Phase, "")
	assert.EqualValues(t, phases.BootstrapPhases()[len(phases.BootstrapPhases())-2].Phase, phase)
	assert.False(t, ok)

	phase, ok = progressTracker.FindLastCompletedPhase("", phases.BootstrapPhases()[len(phases.BootstrapPhases())-1].Phase)
	assert.EqualValues(t, phases.BootstrapPhases()[len(phases.BootstrapPhases())-2].Phase, phase)
	assert.True(t, ok)
}

func TestProgressTracker(t *testing.T) {
	t.Parallel()

	var result []phases.Progress

	bootstrapPhases := phases.BootstrapPhases()
	progressTracker := phases.NewProgressTracker(phases.OperationBootstrap, func(progress phases.Progress) error {
		result = append(result, progress)

		return nil
	})

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

	expected := []phases.Progress{
		{
			Operation:      phases.OperationBootstrap,
			Phases:         bootstrapPhases,
			Progress:       0,
			CompletedPhase: "",
			CurrentPhase:   bootstrapPhases[0].Phase,
			NextPhase:      bootstrapPhases[1].Phase,
		},
		{
			Operation:      phases.OperationBootstrap,
			Phases:         bootstrapPhases,
			Progress:       0.2222222222222222,
			CompletedPhase: bootstrapPhases[1].Phase,
			CurrentPhase:   bootstrapPhases[2].Phase,
			NextPhase:      bootstrapPhases[3].Phase,
		},
		{
			Operation:       phases.OperationBootstrap,
			Phases:          bootstrapPhases,
			Progress:        0.4444444444444444,
			CompletedPhase:  bootstrapPhases[3].Phase,
			CurrentPhase:    bootstrapPhases[4].Phase,
			NextPhase:       bootstrapPhases[5].Phase,
			CurrentSubPhase: bootstrapPhases[4].SubPhases[0],
			NextSubPhase:    bootstrapPhases[4].SubPhases[1],
		},
		{
			Operation:         phases.OperationBootstrap,
			Phases:            bootstrapPhases,
			Progress:          0.48148148148148145,
			CompletedPhase:    bootstrapPhases[3].Phase,
			CurrentPhase:      bootstrapPhases[4].Phase,
			NextPhase:         bootstrapPhases[5].Phase,
			CompletedSubPhase: bootstrapPhases[4].SubPhases[0],
			CurrentSubPhase:   bootstrapPhases[4].SubPhases[1],
			NextSubPhase:      bootstrapPhases[4].SubPhases[2],
		},
		{
			Operation:         phases.OperationBootstrap,
			Phases:            bootstrapPhases,
			Progress:          0.5185185185185185,
			CompletedPhase:    bootstrapPhases[3].Phase,
			CurrentPhase:      bootstrapPhases[4].Phase,
			NextPhase:         bootstrapPhases[5].Phase,
			CompletedSubPhase: bootstrapPhases[4].SubPhases[1],
			CurrentSubPhase:   bootstrapPhases[4].SubPhases[2],
			NextSubPhase:      "",
		},
		{
			Operation:         phases.OperationBootstrap,
			Phases:            bootstrapPhases,
			Progress:          0.5555555555555556,
			CompletedPhase:    bootstrapPhases[3].Phase,
			CurrentPhase:      bootstrapPhases[4].Phase,
			NextPhase:         bootstrapPhases[5].Phase,
			CompletedSubPhase: bootstrapPhases[4].SubPhases[2],
		},
		{
			Operation:       phases.OperationBootstrap,
			Phases:          bootstrapPhases,
			Progress:        0.5555555555555556,
			CompletedPhase:  bootstrapPhases[4].Phase,
			CurrentPhase:    bootstrapPhases[5].Phase,
			NextPhase:       bootstrapPhases[6].Phase,
			CurrentSubPhase: bootstrapPhases[5].SubPhases[0],
			NextSubPhase:    bootstrapPhases[5].SubPhases[1],
		},
		{
			Operation:      phases.OperationBootstrap,
			Phases:         bootstrapPhases,
			Progress:       0.6666666666666666,
			CompletedPhase: bootstrapPhases[5].Phase,
			CurrentPhase:   bootstrapPhases[6].Phase,
			NextPhase:      bootstrapPhases[7].Phase,
		},
		{
			Operation:      phases.OperationBootstrap,
			Phases:         bootstrapPhases,
			Progress:       0.7777777777777778,
			CompletedPhase: bootstrapPhases[6].Phase,
			CurrentPhase:   bootstrapPhases[7].Phase,
			NextPhase:      bootstrapPhases[8].Phase,
		},
		{
			Operation:      phases.OperationBootstrap,
			Phases:         bootstrapPhases,
			Progress:       0.8888888888888888,
			CompletedPhase: bootstrapPhases[7].Phase,
			CurrentPhase:   bootstrapPhases[8].Phase,
		},
		{
			Operation:      phases.OperationBootstrap,
			Phases:         bootstrapPhases,
			Progress:       1,
			CompletedPhase: bootstrapPhases[8].Phase,
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

	bootstrapPhases := phases.BootstrapPhases()

	progressTracker := phases.NewProgressTracker(phases.OperationBootstrap, func(progress phases.Progress) error {
		result = append(result, progress)

		return nil
	})

	require.NoError(t, progressTracker.Progress("", "", "", opts))
	require.NoError(t, progressTracker.Progress(phases.BaseInfraPhase, "", "", opts))
	require.NoError(t, progressTracker.Complete(phases.BaseInfraPhase))

	lastPhases := phases.BootstrapPhases()
	for i := range lastPhases {
		// only PreInfraPreflights and BaseInfra are completed; everything after is skipped
		if i <= 1 {
			continue
		}
		lastPhases[i].Action = ptr.To(phases.ProgressActionSkip)
	}

	expected := []phases.Progress{
		{
			Operation:      phases.OperationBootstrap,
			Phases:         bootstrapPhases,
			Progress:       0,
			CompletedPhase: "",
			CurrentPhase:   bootstrapPhases[0].Phase,
			NextPhase:      bootstrapPhases[1].Phase,
		},
		{
			Operation:      phases.OperationBootstrap,
			Phases:         bootstrapPhases,
			Progress:       float64(2) / float64(len(bootstrapPhases)),
			CompletedPhase: bootstrapPhases[1].Phase,
			CurrentPhase:   bootstrapPhases[2].Phase,
			NextPhase:      bootstrapPhases[3].Phase,
		},
		{
			Operation:      phases.OperationBootstrap,
			Phases:         lastPhases,
			Progress:       1,
			CompletedPhase: lastPhases[len(lastPhases)-1].Phase,
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

	bootstrapPhases := phases.BootstrapPhases()
	progressTracker := phases.NewProgressTracker(phases.OperationBootstrap, nil)

	require.NoError(t, progressTracker.Progress("", "", "", opts))
	require.NoError(t, progressTracker.Progress(bootstrapPhases[len(bootstrapPhases)-1].Phase, "", "", opts))
}

func TestProgressTracker_Skip(t *testing.T) {
	t.Parallel()

	var result []phases.Progress

	progressTracker := phases.NewProgressTracker(phases.OperationDestroy, func(progress phases.Progress) error {
		result = append(result, progress)

		return nil
	})

	require.NoError(t, progressTracker.Progress("", "", "", opts))
	require.NoError(t, progressTracker.Progress(phases.AllNodesPhase, "", "", skipOpts))
	require.NoError(t, progressTracker.Progress(phases.BaseInfraPhase, "", "", opts))

	expected := []phases.Progress{
		{
			Operation:    phases.OperationDestroy,
			Progress:     0,
			CurrentPhase: phases.DeleteResourcesPhase,
			NextPhase:    phases.AllNodesPhase,
			Phases: []phases.PhaseWithSubPhases{
				{Phase: phases.DeleteResourcesPhase},
				{Phase: phases.AllNodesPhase},
				{Phase: phases.BaseInfraPhase},
			},
		},
		{
			Operation:      phases.OperationDestroy,
			Progress:       0.6666666666666666,
			CompletedPhase: phases.AllNodesPhase,
			CurrentPhase:   phases.BaseInfraPhase,
			Phases: []phases.PhaseWithSubPhases{
				{Phase: phases.DeleteResourcesPhase, Action: &skipOpts.Action},
				{Phase: phases.AllNodesPhase, Action: &skipOpts.Action},
				{Phase: phases.BaseInfraPhase},
			},
		},
		{
			Operation:      phases.OperationDestroy,
			Progress:       1,
			CompletedPhase: phases.BaseInfraPhase,
			Phases: []phases.PhaseWithSubPhases{
				{Phase: phases.DeleteResourcesPhase, Action: &skipOpts.Action},
				{Phase: phases.AllNodesPhase, Action: &skipOpts.Action},
				{Phase: phases.BaseInfraPhase},
			},
		},
	}

	if !cmp.Equal(expected, result, cmpOpts) {
		t.Errorf("Diff: %v", cmp.Diff(expected, result, cmpOpts))
	}
}

func TestProgressTracker_WriteProgress(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	progressFile := "progress.jsonl"
	progressFilePath := filepath.Join(tmpDir, progressFile)

	bootstrapPhases := phases.BootstrapPhases()
	progressTracker := phases.NewProgressTracker(
		phases.OperationBootstrap,
		phases.WriteProgress(progressFilePath),
	)

	require.NoError(t, progressTracker.Progress("", "", "", opts))
	require.NoError(t, progressTracker.Progress(bootstrapPhases[len(bootstrapPhases)-1].Phase, "", "", opts))

	result := readJSONLinesFromFile(t, progressFilePath)
	expected := []phases.Progress{
		{
			Operation:      phases.OperationBootstrap,
			Phases:         bootstrapPhases,
			Progress:       0,
			CompletedPhase: "",
			CurrentPhase:   bootstrapPhases[0].Phase,
			NextPhase:      bootstrapPhases[1].Phase,
		},
		{
			Operation:      phases.OperationBootstrap,
			Phases:         bootstrapPhases,
			Progress:       1,
			CompletedPhase: bootstrapPhases[len(bootstrapPhases)-1].Phase,
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
	progressTracker.SetClusterConfig(phases.ClusterConfig{ClusterType: "Static"})

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
	progressTracker.SetClusterConfig(phases.ClusterConfig{ClusterType: "Static"})

	require.NoError(t, progressTracker.Progress(phases.InstallDeckhousePhase, phases.CreateResourcesPhase, "", opts))
	require.Len(t, result, 1)

	p := result[0]
	assert.Equal(t, string(phases.InstallDeckhousePhase), string(p.CompletedPhase))
	assert.Equal(t, string(phases.CreateResourcesPhase), string(p.CurrentPhase))
	assert.Equal(t, string(phases.ExecPostBootstrapPhase), string(p.NextPhase))

	// InstallAdditionalMastersAndStaticNodes must be marked as skipped
	var installAdditionalPhase *phases.PhaseWithSubPhases
	for i := range p.Phases {
		if p.Phases[i].Phase == phases.InstallAdditionalMastersAndStaticNodes {
			installAdditionalPhase = &p.Phases[i]
			break
		}
	}

	require.NotNil(t, installAdditionalPhase)
	assert.NotNil(t, installAdditionalPhase.Action)
	assert.Equal(t, phases.ProgressActionSkip, *installAdditionalPhase.Action)
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
	cmpopts.IgnoreFields(phases.PhaseWithSubPhases{}, "includeIf"),

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
