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

package tofu

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	infraexec "github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/exec"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type RunExecutorParams struct {
	TofuBinPath string
	RootDir     string
	ExecutorID  string
	IsDebug     bool
}

func (p *RunExecutorParams) validateRunParams() error {
	if p.RootDir == "" {
		return fmt.Errorf("RootDir is required for tofu executor")
	}

	if p.TofuBinPath == "" {
		return fmt.Errorf("TofuBinPath is required for tofu executor")
	}

	if p.ExecutorID == "" {
		return fmt.Errorf("ExecutorID is required for tofu executor")
	}

	return nil
}

func tofuCmd(ctx context.Context, params RunExecutorParams, workingDir string, args ...string) *exec.Cmd {
	fullArgs := args
	if workingDir != "" {
		fullArgs = append([]string{fmt.Sprintf("-chdir=%s", workingDir)}, args...)
	}
	log.DebugF("Tofu Command:\n opentofu %s\n", strings.Join(fullArgs, " "))

	cmd := exec.CommandContext(ctx, params.TofuBinPath, fullArgs...)

	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGINT)
	}

	dataDir := filepath.Join(params.RootDir, fmt.Sprintf("tf_%s", params.ExecutorID))

	envs := append(
		os.Environ(),
		"TF_IN_AUTOMATION=yes",
		"TF_SKIP_CREATING_DEPS_LOCK_FILE=yes",
		fmt.Sprintf("TF_DATA_DIR=%s", dataDir),
	)

	envs = appendLogEnvs(envs)

	// this uses for skip destructive changes after migration to ready resource
	// in cloud provider dvp, because changing depends_on data source produce
	// destructive changes
	skipDataDeps := []string{
		"module.additional-disk.kubernetes_resource_ready_v1.additional_disk",
		"module.ipv4-address.kubernetes_resource_ready_v1.ipv4_address",
		"module.kubernetes-data-disk.kubernetes_resource_ready_v1.kubernetes-data-disk",
		"module.master.kubernetes_resource_ready_v1.vm",
		"module.static-node.kubernetes_resource_ready_v1.vm",
		// module.root-disk.kubernetes_resource_ready_v1.root-disk was not added because
		// it does not contain data source for disk
	}

	envs = append(
		envs,
		fmt.Sprintf("TF_SKIP_DEPS_FOR_DATA_SOURCES_PROVIDER=kubernetes"),
		fmt.Sprintf(
			"TF_SKIP_DEPS_FOR_DATA_SOURCES=%s",
			strings.Join(skipDataDeps, ";"),
		),
	)

	cmd.Env = infraexec.ReplaceHomeDirEnv(envs, params.RootDir)

	cmd.Env = append(
		cmd.Env,
		fmt.Sprintf("HTTP_PROXY=%s", os.Getenv("HTTP_PROXY")),
		fmt.Sprintf("HTTPS_PROXY=%s", os.Getenv("HTTPS_PROXY")),
		fmt.Sprintf("NO_PROXY=%s", os.Getenv("NO_PROXY")),
	)

	log.DebugF("Tofu Command envs:\n %s\n", strings.Join(cmd.Env, " "))

	return cmd
}

func appendLogEnvs(envs []string) []string {
	const (
		coreEnv     = "TF_LOG_CORE"
		providerEnv = "TF_LOG_PROVIDER"
	)

	coreVal := "INFO"
	providerVal := "INFO"

	for _, e := range envs {
		if strings.HasPrefix(e, coreEnv) {
			log.DebugF("Found opentofu core log env %s\n", e)
			coreVal = ""
			continue
		}

		if strings.HasPrefix(e, providerEnv) {
			log.DebugF("Found opentofu provider log env %s\n", e)
			providerVal = ""
			continue
		}
	}

	if coreVal != "" {
		envs = append(envs, fmt.Sprintf("%s=%s", coreEnv, coreVal))
	}

	if providerVal != "" {
		envs = append(envs, fmt.Sprintf("%s=%s", providerEnv, providerVal))
	}

	return envs
}
