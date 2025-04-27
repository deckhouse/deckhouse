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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func tofuCmd(ctx context.Context, workingDir string, args ...string) *exec.Cmd {
	fullArgs := args
	if workingDir != "" {
		fullArgs = append([]string{fmt.Sprintf("-chdir=%s", workingDir)}, args...)
	}
	log.DebugF("Tofu Command:\n opentofu %s\n", strings.Join(fullArgs, " "))

	cmd := exec.CommandContext(ctx, "opentofu", fullArgs...)

	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGINT)
	}

	cmd.Env = append(
		os.Environ(),
		"TF_IN_AUTOMATION=yes",
		"TF_SKIP_CREATING_DEPS_LOCK_FILE=yes",
		"TF_DATA_DIR="+filepath.Join(app.TmpDirName, "tf_dhctl"),
	)

	// always use dug log for write its to debug log file
	cmd.Env = append(cmd.Env, "TF_LOG=DEBUG")

	cmd.Env = append(
		cmd.Env,
		fmt.Sprintf("HTTP_PROXY=%s", os.Getenv("HTTP_PROXY")),
		fmt.Sprintf("HTTPS_PROXY=%s", os.Getenv("HTTPS_PROXY")),
		fmt.Sprintf("NO_PROXY=%s", os.Getenv("NO_PROXY")),
	)

	log.DebugF("Tofu Command envs:\n %s\n", strings.Join(cmd.Env, " "))

	return cmd
}

type Executor struct {
	workingDir string
	logger     log.Logger
	cmd        *exec.Cmd
}

func NewExecutor(workingDir string, looger log.Logger) *Executor {
	return &Executor{
		workingDir: workingDir,
		logger:     looger,
	}
}

func (e *Executor) Init(ctx context.Context, pluginsDir string) error {
	args := []string{
		"init",
		fmt.Sprintf("-plugin-dir=%s", pluginsDir),
		"-no-color",
		"-input=false",
	}

	e.cmd = tofuCmd(ctx, e.workingDir, args...)
	_, err := infrastructure.Exec(ctx, e.cmd, e.logger)

	return err
}

func (e *Executor) Apply(ctx context.Context, opts infrastructure.ApplyOpts) error {
	args := []string{
		"apply",
		"-input=false",
		"-no-color",
		"-lock=false",
		"-auto-approve",
		fmt.Sprintf("-state=%s", opts.StatePath),
		fmt.Sprintf("-state-out=%s", opts.StatePath),
	}

	if opts.PlanPath != "" {
		args = append(args, opts.PlanPath)
	} else {
		args = append(args,
			fmt.Sprintf("-var-file=%s", opts.VariablesPath),
			e.workingDir,
		)
	}

	e.cmd = tofuCmd(ctx, e.workingDir, args...)

	_, err := infrastructure.Exec(ctx, e.cmd, e.logger)

	return err
}

func (e *Executor) Plan(ctx context.Context, opts infrastructure.PlanOpts) (exitCode int, err error) {
	args := []string{
		"plan",
		"-input=false",
		"-no-color",
		fmt.Sprintf("-var-file=%s", opts.VariablesPath),
		fmt.Sprintf("-state=%s", opts.StatePath),
	}

	if opts.DetailedExitCode {
		args = append(args, "-detailed-exitcode")
	}

	if opts.OutPath != "" {
		args = append(args, fmt.Sprintf("-out=%s", opts.OutPath))
	}

	if opts.Destroy {
		args = append(args, "-destroy")
	}

	e.cmd = tofuCmd(ctx, e.workingDir, args...)

	return infrastructure.Exec(ctx, e.cmd, e.logger)
}

func (e *Executor) Output(ctx context.Context, statePath string, outFielda ...string) (result []byte, err error) {
	args := []string{
		"output",
		"-no-color",
		"-json",
		fmt.Sprintf("-state=%s", statePath),
	}
	if len(outFielda) > 0 {
		args = append(args, outFielda...)
	}

	e.cmd = tofuCmd(ctx, e.workingDir, args...)

	return e.cmd.Output()
}

func (e *Executor) Destroy(ctx context.Context, opts infrastructure.DestroyOpts) error {
	args := []string{
		"destroy",
		"-no-color",
		"-auto-approve",
		fmt.Sprintf("-var-file=%s", opts.VariablesPath),
		fmt.Sprintf("-state=%s", opts.StatePath),
	}

	e.cmd = tofuCmd(ctx, e.workingDir, args...)

	_, err := infrastructure.Exec(ctx, e.cmd, e.logger)

	return err
}

func (e *Executor) Show(ctx context.Context, planPath string) (result []byte, err error) {
	args := []string{
		"show",
		"-json",
		planPath,
	}

	e.cmd = tofuCmd(ctx, e.workingDir, args...)

	return e.cmd.Output()
}

func (e *Executor) SetExecutorLogger(logger log.Logger) {
	e.logger = logger
}

func (e *Executor) Stop() {
	log.DebugF("Interrupt tofu process by pid: %d\n", e.cmd.Process.Pid)

	// 1. Tofu exits immediately on SIGTERM, so SIGINT is used here
	//    to interrupt it gracefully even when main process caught the SIGTERM.
	// 2. Negative pid is used to send signal to the process group
	//    started by "Setpgid: true" to prevent double signaling
	//    from shell and from us.
	//    See also pkg/system/ssh/cmd/ssh.go
	_ = syscall.Kill(-e.cmd.Process.Pid, syscall.SIGINT)
}
