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

package tofu

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func TestExecutorApply_WithoutPlanPath_DoesNotPassWorkingDirAsPositionalArg(t *testing.T) {
	t.Parallel()

	binPath, err := exec.LookPath("true")
	require.NoError(t, err)

	rootDir := t.TempDir()
	workingDir := filepath.Join(rootDir, "module")
	require.NoError(t, os.MkdirAll(workingDir, 0o755))

	executor, err := NewExecutor(ExecutorParams{
		RunExecutorParams: RunExecutorParams{
			TofuBinPath: binPath,
			RootDir:     rootDir,
			ExecutorID:  "test",
		},
		WorkingDir: workingDir,
		PluginsDir: rootDir,
		Step:       infrastructure.MasterNodeStep,
	}, log.GetDefaultLogger())
	require.NoError(t, err)

	varsPath := filepath.Join(rootDir, "vars.tfvars.json")
	err = executor.Apply(context.Background(), infrastructure.ApplyOpts{
		StatePath:     filepath.Join(rootDir, "state.tfstate"),
		VariablesPath: varsPath,
	})
	require.NoError(t, err)
	require.NotNil(t, executor.cmd)
	require.Contains(t, executor.cmd.Args, "-var-file="+varsPath)

	for _, arg := range executor.cmd.Args {
		require.NotEqual(t, workingDir, arg, "working dir must not be passed as positional plan argument")
	}
}

func TestExecutorApply_WithPlanPath_UsesProvidedPlanOnly(t *testing.T) {
	t.Parallel()

	binPath, err := exec.LookPath("true")
	require.NoError(t, err)

	rootDir := t.TempDir()
	workingDir := filepath.Join(rootDir, "module")
	require.NoError(t, os.MkdirAll(workingDir, 0o755))

	executor, err := NewExecutor(ExecutorParams{
		RunExecutorParams: RunExecutorParams{
			TofuBinPath: binPath,
			RootDir:     rootDir,
			ExecutorID:  "test",
		},
		WorkingDir: workingDir,
		PluginsDir: rootDir,
		Step:       infrastructure.MasterNodeStep,
	}, log.GetDefaultLogger())
	require.NoError(t, err)

	planPath := filepath.Join(rootDir, "plan.tfplan")
	err = executor.Apply(context.Background(), infrastructure.ApplyOpts{
		StatePath:     filepath.Join(rootDir, "state.tfstate"),
		VariablesPath: filepath.Join(rootDir, "vars.tfvars.json"),
		PlanPath:      planPath,
	})
	require.NoError(t, err)
	require.NotNil(t, executor.cmd)
	require.Contains(t, executor.cmd.Args, planPath)

	for _, arg := range executor.cmd.Args {
		require.NotEqual(t, "-var-file="+filepath.Join(rootDir, "vars.tfvars.json"), arg)
	}
}

