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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
)

var opts = phases.ProgressOpts{Action: phases.PhaseActionDefault}

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

	require.NoError(t, progressTracker.Progress("", "", opts))
	require.NoError(t, progressTracker.Progress(phases.BaseInfraPhase, "", opts))
	require.NoError(t, progressTracker.Progress(phases.RegistryPackagesProxyPhase, "", opts))
	require.NoError(t, progressTracker.Progress(phases.ExecuteBashibleBundlePhase, "", opts))
	require.NoError(t, progressTracker.Progress("", phases.InstallDeckhouseSubPhaseConnect, opts))
	require.NoError(t, progressTracker.Progress("", phases.InstallDeckhouseSubPhaseInstall, opts))
	require.NoError(t, progressTracker.Progress("", phases.InstallDeckhouseSubPhaseWait, opts))
	require.NoError(t, progressTracker.Progress(phases.InstallDeckhousePhase, "", opts))
	require.NoError(t, progressTracker.Progress(phases.InstallAdditionalMastersAndStaticNodes, "", opts))
	require.NoError(t, progressTracker.Progress(phases.CreateResourcesPhase, "", opts))
	require.NoError(t, progressTracker.Progress(phases.ExecPostBootstrapPhase, "", opts))
	require.NoError(t, progressTracker.Progress(phases.FinalizationPhase, "", opts))

	// do nothing because progress is already 1
	require.NoError(t, progressTracker.Complete())

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
			Progress:       0.125,
			CompletedPhase: bootstrapPhases[0].Phase,
			CurrentPhase:   bootstrapPhases[1].Phase,
			NextPhase:      bootstrapPhases[2].Phase,
		},
		{
			Operation:      phases.OperationBootstrap,
			Phases:         bootstrapPhases,
			Progress:       0.25,
			CompletedPhase: bootstrapPhases[1].Phase,
			CurrentPhase:   bootstrapPhases[2].Phase,
			NextPhase:      bootstrapPhases[3].Phase,
		},
		{
			Operation:         phases.OperationBootstrap,
			Phases:            bootstrapPhases,
			Progress:          0.375,
			CompletedPhase:    bootstrapPhases[2].Phase,
			CurrentPhase:      bootstrapPhases[3].Phase,
			NextPhase:         bootstrapPhases[4].Phase,
			CompletedSubPhase: "",
			CurrentSubPhase:   bootstrapPhases[3].SubPhases[0],
			NextSubPhase:      bootstrapPhases[3].SubPhases[1],
		},
		{
			Operation:         phases.OperationBootstrap,
			Phases:            bootstrapPhases,
			Progress:          0.4166666666666667,
			CompletedPhase:    bootstrapPhases[2].Phase,
			CurrentPhase:      bootstrapPhases[3].Phase,
			NextPhase:         bootstrapPhases[4].Phase,
			CompletedSubPhase: bootstrapPhases[3].SubPhases[0],
			CurrentSubPhase:   bootstrapPhases[3].SubPhases[1],
			NextSubPhase:      bootstrapPhases[3].SubPhases[2],
		},
		{
			Operation:         phases.OperationBootstrap,
			Phases:            bootstrapPhases,
			Progress:          0.45833333333333337,
			CompletedPhase:    bootstrapPhases[2].Phase,
			CurrentPhase:      bootstrapPhases[3].Phase,
			NextPhase:         bootstrapPhases[4].Phase,
			CompletedSubPhase: bootstrapPhases[3].SubPhases[1],
			CurrentSubPhase:   bootstrapPhases[3].SubPhases[2],
			NextSubPhase:      "",
		},
		{
			Operation:         phases.OperationBootstrap,
			Phases:            bootstrapPhases,
			Progress:          0.5,
			CompletedPhase:    bootstrapPhases[2].Phase,
			CurrentPhase:      bootstrapPhases[3].Phase,
			NextPhase:         bootstrapPhases[4].Phase,
			CompletedSubPhase: bootstrapPhases[3].SubPhases[2],
		},
		{
			Operation:      phases.OperationBootstrap,
			Phases:         bootstrapPhases,
			Progress:       0.5,
			CompletedPhase: bootstrapPhases[3].Phase,
			CurrentPhase:   bootstrapPhases[4].Phase,
			NextPhase:      bootstrapPhases[5].Phase,
		},
		{
			Operation:      phases.OperationBootstrap,
			Phases:         bootstrapPhases,
			Progress:       0.625,
			CompletedPhase: bootstrapPhases[4].Phase,
			CurrentPhase:   bootstrapPhases[5].Phase,
			NextPhase:      bootstrapPhases[6].Phase,
		},
		{
			Operation:      phases.OperationBootstrap,
			Phases:         bootstrapPhases,
			Progress:       0.75,
			CompletedPhase: bootstrapPhases[5].Phase,
			CurrentPhase:   bootstrapPhases[6].Phase,
			NextPhase:      bootstrapPhases[7].Phase,
		},
		{
			Operation:      phases.OperationBootstrap,
			Phases:         bootstrapPhases,
			Progress:       0.875,
			CompletedPhase: bootstrapPhases[6].Phase,
			CurrentPhase:   bootstrapPhases[7].Phase,
		},
		{
			Operation:      phases.OperationBootstrap,
			Phases:         bootstrapPhases,
			Progress:       1,
			CompletedPhase: bootstrapPhases[7].Phase,
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

	require.NoError(t, progressTracker.Progress("", "", opts))
	require.NoError(t, progressTracker.Progress(phases.BaseInfraPhase, "", opts))
	require.NoError(t, progressTracker.Complete())

	lastPhases := phases.BootstrapPhases()
	for i := range lastPhases {
		// everything except BaseInfraPhase should be skipped
		if i == 0 {
			continue
		}
		lastPhases[i].Action = ptr.To(phases.PhaseActionSkip)
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
			Progress:       0.125,
			CompletedPhase: bootstrapPhases[0].Phase,
			CurrentPhase:   bootstrapPhases[1].Phase,
			NextPhase:      bootstrapPhases[2].Phase,
		},
		{
			Operation:      phases.OperationBootstrap,
			Phases:         lastPhases,
			Progress:       1,
			CompletedPhase: lastPhases[7].Phase,
			CurrentPhase:   "",
			NextPhase:      "",
		},
	}

	if !cmp.Equal(expected, result, cmpOpts) {
		t.Errorf("Diff: %v", cmp.Diff(expected, result, cmpOpts))
	}
}

func TestProgressTracker_NilCallback(t *testing.T) {
	t.Parallel()

	bootstrapPhases := phases.BootstrapPhases()
	progressTracker := phases.NewProgressTracker(phases.OperationBootstrap, nil)

	require.NoError(t, progressTracker.Progress("", "", opts))
	require.NoError(t, progressTracker.Progress(bootstrapPhases[len(bootstrapPhases)-1].Phase, "", opts))
}

func TestProgressTracker_Skip(t *testing.T) {
	t.Parallel()

	var result []phases.Progress

	progressTracker := phases.NewProgressTracker(phases.OperationDestroy, func(progress phases.Progress) error {
		result = append(result, progress)

		return nil
	})

	skipOpts := phases.ProgressOpts{Action: phases.PhaseActionSkip}

	require.NoError(t, progressTracker.Progress("", "", opts))
	require.NoError(t, progressTracker.Progress(phases.AllNodesPhase, "", skipOpts))
	require.NoError(t, progressTracker.Progress(phases.BaseInfraPhase, "", opts))

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

	require.NoError(t, progressTracker.Progress("", "", opts))
	require.NoError(t, progressTracker.Progress(bootstrapPhases[len(bootstrapPhases)-1].Phase, "", opts))

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
	cmp.Comparer(func(x, y *phases.PhaseAction) bool {
		if x == nil && (y != nil && *y == "") {
			return true
		}

		if y == nil && (x != nil && *x == "") {
			return true
		}

		return cmp.Equal(y, x)
	}),
}
