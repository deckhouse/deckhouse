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

package terraform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	infraexec "github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/exec"
)

type RunExecutorParams struct {
	RootDir          string
	TerraformBinPath string
}

func terraformCmd(ctx context.Context, params RunExecutorParams, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, params.TerraformBinPath, args...)
	cmd.Dir = filepath.Dir(params.TerraformBinPath)
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGINT)
	}

	cmd.Env = append(
		os.Environ(),
		"TF_IN_AUTOMATION=yes",
		"TF_DATA_DIR="+filepath.Join(params.RootDir, "tf_dhctl"),
	)

	// always use dug log for write its to debug log file
	cmd.Env = append(cmd.Env, "TF_LOG=DEBUG")

	envs := append(
		cmd.Env,
		fmt.Sprintf("HTTP_PROXY=%s", os.Getenv("HTTP_PROXY")),
		fmt.Sprintf("HTTPS_PROXY=%s", os.Getenv("HTTPS_PROXY")),
		fmt.Sprintf("NO_PROXY=%s", os.Getenv("NO_PROXY")),
	)

	cmd.Env = infraexec.ReplaceHomeDirEnv(envs, params.RootDir)
	return cmd
}
