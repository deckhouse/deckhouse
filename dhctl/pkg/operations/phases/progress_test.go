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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
)

func TestProgressTracker_NilCallback(t *testing.T) {
	t.Parallel()

	progressTracker := phases.NewProgressTracker(phases.OperationBootstrap, nil)
	require.NoError(t, progressTracker.Progress("", ""))
	require.NoError(t, progressTracker.Progress(phases.BootstrapPhases[len(phases.BootstrapPhases)-1].Phase, ""))
}

func TestProgressTracker_LastCompletedPhase(t *testing.T) {
	t.Parallel()

	progressTracker := phases.NewProgressTracker(phases.OperationBootstrap, func(progress phases.Progress) error {
		return nil
	})

	assert.EqualValues(t,
		phases.BootstrapPhases[0].Phase,
		progressTracker.LastCompletedPhase(phases.BootstrapPhases[0].Phase, ""),
	)
	assert.EqualValues(t,
		"",
		progressTracker.LastCompletedPhase("", phases.BootstrapPhases[0].Phase),
	)
	assert.EqualValues(t,
		phases.BootstrapPhases[0].Phase,
		progressTracker.LastCompletedPhase("", phases.BootstrapPhases[1].Phase),
	)
	assert.EqualValues(t,
		phases.BootstrapPhases[len(phases.BootstrapPhases)-2].Phase,
		progressTracker.LastCompletedPhase("", phases.BootstrapPhases[len(phases.BootstrapPhases)-1].Phase),
	)
}

func TestProgressTracker(t *testing.T) {
	t.Parallel()

	var result []phases.Progress

	progressTracker := phases.NewProgressTracker(phases.OperationBootstrap, func(progress phases.Progress) error {
		result = append(result, progress)

		return nil
	})

	require.NoError(t, progressTracker.Progress("", ""))
	require.NoError(t, progressTracker.Progress(phases.BootstrapPhases[2].Phase, ""))
	require.NoError(t, progressTracker.Progress(phases.BootstrapPhases[3].Phase, ""))
	require.NoError(t, progressTracker.Progress(phases.BootstrapPhases[len(phases.BootstrapPhases)-1].Phase, ""))

	assert.Equal(t, []phases.Progress{
		{
			Operation:      phases.OperationBootstrap,
			Phases:         phases.BootstrapPhases,
			Progress:       0,
			CompletedPhase: "",
			CurrentPhase:   phases.BootstrapPhases[0].Phase,
			NextPhase:      phases.BootstrapPhases[1].Phase,
		},
		{
			Operation:       phases.OperationBootstrap,
			Phases:          phases.BootstrapPhases,
			Progress:        0.375,
			CompletedPhase:  phases.BootstrapPhases[2].Phase,
			CurrentPhase:    phases.BootstrapPhases[3].Phase,
			NextPhase:       phases.BootstrapPhases[4].Phase,
			CurrentSubPhase: phases.InstallDeckhouseSubPhaseConnect,
			NextSubPhase:    phases.InstallDeckhouseSubPhaseInstall,
		},
		{
			Operation:      phases.OperationBootstrap,
			Phases:         phases.BootstrapPhases,
			Progress:       0.5,
			CompletedPhase: phases.BootstrapPhases[3].Phase,
			CurrentPhase:   phases.BootstrapPhases[4].Phase,
			NextPhase:      phases.BootstrapPhases[5].Phase,
		},
		{
			Operation:      phases.OperationBootstrap,
			Phases:         phases.BootstrapPhases,
			Progress:       1,
			CompletedPhase: phases.BootstrapPhases[len(phases.BootstrapPhases)-1].Phase,
			CurrentPhase:   "",
			NextPhase:      "",
		},
	}, result)
}

func TestProgressTracker_SubPhases(t *testing.T) {
	t.Parallel()

	var result []phases.Progress

	progressTracker := phases.NewProgressTracker(phases.OperationBootstrap, func(progress phases.Progress) error {
		result = append(result, progress)

		return nil
	})

	require.NoError(t, progressTracker.Progress("", ""))
	require.NoError(t, progressTracker.Progress(phases.ExecuteBashibleBundlePhase, ""))
	require.NoError(t, progressTracker.Progress("", phases.InstallDeckhouseSubPhaseConnect))
	require.NoError(t, progressTracker.Progress("", phases.InstallDeckhouseSubPhaseInstall))
	require.NoError(t, progressTracker.Progress("", phases.InstallDeckhouseSubPhaseWait))
	require.NoError(t, progressTracker.Progress(phases.InstallDeckhousePhase, ""))
	require.NoError(t, progressTracker.Progress(phases.BootstrapPhases[len(phases.BootstrapPhases)-1].Phase, ""))

	assert.Equal(t, []phases.Progress{
		{
			Operation:      phases.OperationBootstrap,
			Phases:         phases.BootstrapPhases,
			Progress:       0,
			CompletedPhase: "",
			CurrentPhase:   phases.BootstrapPhases[0].Phase,
			NextPhase:      phases.BootstrapPhases[1].Phase,
		},
		{
			Operation:         phases.OperationBootstrap,
			Phases:            phases.BootstrapPhases,
			Progress:          0.375,
			CompletedPhase:    phases.ExecuteBashibleBundlePhase,
			CurrentPhase:      phases.InstallDeckhousePhase,
			NextPhase:         phases.InstallAdditionalMastersAndStaticNodes,
			CompletedSubPhase: "",
			CurrentSubPhase:   phases.InstallDeckhouseSubPhaseConnect,
			NextSubPhase:      phases.InstallDeckhouseSubPhaseInstall,
		},
		{
			Operation:         phases.OperationBootstrap,
			Phases:            phases.BootstrapPhases,
			Progress:          0.4166666666666667,
			CompletedPhase:    phases.ExecuteBashibleBundlePhase,
			CurrentPhase:      phases.InstallDeckhousePhase,
			NextPhase:         phases.InstallAdditionalMastersAndStaticNodes,
			CompletedSubPhase: phases.InstallDeckhouseSubPhaseConnect,
			CurrentSubPhase:   phases.InstallDeckhouseSubPhaseInstall,
			NextSubPhase:      phases.InstallDeckhouseSubPhaseWait,
		},
		{
			Operation:         phases.OperationBootstrap,
			Phases:            phases.BootstrapPhases,
			Progress:          0.45833333333333337,
			CompletedPhase:    phases.ExecuteBashibleBundlePhase,
			CurrentPhase:      phases.InstallDeckhousePhase,
			NextPhase:         phases.InstallAdditionalMastersAndStaticNodes,
			CompletedSubPhase: phases.InstallDeckhouseSubPhaseInstall,
			CurrentSubPhase:   phases.InstallDeckhouseSubPhaseWait,
			NextSubPhase:      "",
		},
		{
			Operation:         phases.OperationBootstrap,
			Phases:            phases.BootstrapPhases,
			Progress:          0.5,
			CompletedPhase:    phases.ExecuteBashibleBundlePhase,
			CurrentPhase:      phases.InstallDeckhousePhase,
			NextPhase:         phases.InstallAdditionalMastersAndStaticNodes,
			CompletedSubPhase: phases.InstallDeckhouseSubPhaseWait,
		},
		{
			Operation:      phases.OperationBootstrap,
			Phases:         phases.BootstrapPhases,
			Progress:       0.5,
			CompletedPhase: phases.InstallDeckhousePhase,
			CurrentPhase:   phases.InstallAdditionalMastersAndStaticNodes,
			NextPhase:      phases.CreateResourcesPhase,
		},
		{
			Operation:      phases.OperationBootstrap,
			Phases:         phases.BootstrapPhases,
			Progress:       1,
			CompletedPhase: phases.BootstrapPhases[len(phases.BootstrapPhases)-1].Phase,
			CurrentPhase:   "",
			NextPhase:      "",
		},
	}, result)
}

func TestProgressTracker_WriteProgress(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	progressFile := "progress.jsonl"
	progressFilePath := filepath.Join(tmpDir, progressFile)

	progressTracker := phases.NewProgressTracker(
		phases.OperationBootstrap,
		phases.WriteProgress(progressFilePath),
	)

	require.NoError(t, progressTracker.Progress("", ""))
	require.NoError(t, progressTracker.Progress(phases.BootstrapPhases[len(phases.BootstrapPhases)-1].Phase, ""))

	result := readJSONLinesFromFile(t, progressFilePath)

	assert.Equal(t, []phases.Progress{
		{
			Operation:      phases.OperationBootstrap,
			Phases:         phases.BootstrapPhases,
			Progress:       0,
			CompletedPhase: "",
			CurrentPhase:   phases.BootstrapPhases[0].Phase,
			NextPhase:      phases.BootstrapPhases[1].Phase,
		},
		{
			Operation:      phases.OperationBootstrap,
			Phases:         phases.BootstrapPhases,
			Progress:       1,
			CompletedPhase: phases.BootstrapPhases[len(phases.BootstrapPhases)-1].Phase,
			CurrentPhase:   "",
			NextPhase:      "",
		},
	}, result)
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
