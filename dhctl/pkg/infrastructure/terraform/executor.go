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
	"io"
	"os/exec"
	"syscall"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	infraexec "github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/exec"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/plan"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type ExecutorParams struct {
	RunExecutorParams

	WorkingDir     string
	PluginsDir     string
	Step           infrastructure.Step
	VmChangeTester plan.VMChangeTester
}

type Executor struct {
	params ExecutorParams

	logger log.Logger
	cmd    *exec.Cmd
}

func NewExecutor(params ExecutorParams, logger log.Logger) *Executor {
	return &Executor{
		params: params,
		logger: logger,
	}
}

func (e *Executor) IsVMChange(rc plan.ResourceChange) bool {
	return e.params.VmChangeTester(rc)
}

func (e *Executor) Step() infrastructure.Step {
	return e.params.Step
}

func (e *Executor) GetStatesDir() string {
	return e.params.RootDir
}

func (e *Executor) Init(ctx context.Context) error {
	args := []string{
		"init",
		fmt.Sprintf("-plugin-dir=%s", e.params.PluginsDir),
		"-get-plugins=false",
		"-no-color",
		"-input=false",
		e.params.WorkingDir,
	}

	e.cmd = terraformCmd(ctx, e.params.RunExecutorParams, args...)

	_, err := infraexec.Exec(ctx, e.cmd, e.logger)

	return err
}

func (e *Executor) Apply(ctx context.Context, opts infrastructure.ApplyOpts) error {
	args := []string{
		"apply",
		"-input=false",
		"-no-color",
		"-auto-approve",
		fmt.Sprintf("-state=%s", opts.StatePath),
		fmt.Sprintf("-state-out=%s", opts.StatePath),
	}

	if opts.PlanPath != "" {
		args = append(args, opts.PlanPath)
	} else {
		args = append(args,
			fmt.Sprintf("-var-file=%s", opts.VariablesPath),
			e.params.WorkingDir,
		)
	}

	e.cmd = terraformCmd(ctx, e.params.RunExecutorParams, args...)

	_, err := infraexec.Exec(ctx, e.cmd, e.logger)

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

	args = append(args, e.params.WorkingDir)

	e.cmd = terraformCmd(ctx, e.params.RunExecutorParams, args...)

	if opts.NoOutput {
		e.cmd.Stdout = io.Discard
		e.cmd.Stderr = io.Discard
	}
	return infraexec.Exec(ctx, e.cmd, e.logger)
}

func (e *Executor) Output(ctx context.Context, statePath string, outFielda ...string) (result []byte, err error) {
	cmd, output, err := terraformOutputRun(ctx, e.params.RunExecutorParams, statePath, outFielda...)
	e.cmd = cmd
	return output, err
}

func (e *Executor) Destroy(ctx context.Context, opts infrastructure.DestroyOpts) error {
	args := []string{
		"destroy",
		"-no-color",
		"-auto-approve",
		fmt.Sprintf("-var-file=%s", opts.VariablesPath),
		fmt.Sprintf("-state=%s", opts.StatePath),
		e.params.WorkingDir,
	}

	e.cmd = terraformCmd(ctx, e.params.RunExecutorParams, args...)

	_, err := infraexec.Exec(ctx, e.cmd, e.logger)

	return err
}

func (e *Executor) Show(ctx context.Context, planPath string) (result []byte, err error) {
	args := []string{
		"show",
		"-json",
		planPath,
	}

	e.cmd = terraformCmd(ctx, e.params.RunExecutorParams, args...)

	return e.cmd.Output()
}

func (e *Executor) SetExecutorLogger(logger log.Logger) {
	e.logger = logger
}

func (e *Executor) Stop() {
	log.DebugF("Interrupt terraform process by pid: %d\n", e.cmd.Process.Pid)

	// 1. Terraform exits immediately on SIGTERM, so SIGINT is used here
	//    to interrupt it gracefully even when main process caught the SIGTERM.
	// 2. Negative pid is used to send signal to the process group
	//    started by "Setpgid: true" to prevent double signaling
	//    from shell and from us.
	//    See also pkg/system/ssh/cmd/ssh.go
	_ = syscall.Kill(-e.cmd.Process.Pid, syscall.SIGINT)
}

func (e *Executor) GetActions(ctx context.Context, planPath string) (actions []string, err error) {
	args := []string{
		"show",
		"-json",
		planPath,
	}

	cmd := terraformCmd(ctx, e.params.RunExecutorParams, args...)
	return infrastructure.GetActions(ctx, cmd)
}
