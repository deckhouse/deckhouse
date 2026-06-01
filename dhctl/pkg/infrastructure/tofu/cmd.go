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
	dhlog "github.com/deckhouse/deckhouse/dhctl/pkg/logger"
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
	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Tofu Command:\n opentofu %s", strings.Join(fullArgs, " ")))

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

	envs = appendLogEnvs(ctx, envs)

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
		"TF_SKIP_DEPS_FOR_DATA_SOURCES_PROVIDER=kubernetes",
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

	// If dhctl has a persistent provider daemon running, tell every tofu
	// invocation to reuse it via go-plugin reattach. This trades 5-6 cold
	// plugin spawns per pipeline (~500ms each) for one warm gRPC connection.
	// EnsureProviderDaemon respawns the daemon on each call if the previous
	// instance died, so transient crashes don't cascade through the rest of
	// the bootstrap.
	if reattach := EnsureProviderDaemon(); reattach != "" {
		cmd.Env = append(cmd.Env, "TF_REATTACH_PROVIDERS="+reattach)
	}

	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Tofu Command envs:\n %s", strings.Join(cmd.Env, " ")))

	return cmd
}

func appendLogEnvs(ctx context.Context, envs []string) []string {
	const (
		coreEnv     = "TF_LOG_CORE"
		providerEnv = "TF_LOG_PROVIDER"
	)

	coreVal := "INFO"
	providerVal := "INFO"

	for _, e := range envs {
		if strings.HasPrefix(e, coreEnv) {
			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Found opentofu core log env %s", e))
			coreVal = ""
			continue
		}

		if strings.HasPrefix(e, providerEnv) {
			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Found opentofu provider log env %s", e))
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
