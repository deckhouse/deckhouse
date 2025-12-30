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
	"os/exec"
	"syscall"

	"github.com/name212/govalue"

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
	VMChangeTester plan.VMChangeTester
}

func (p *ExecutorParams) validate() error {
	if err := p.RunExecutorParams.validateRunParams(); err != nil {
		return err
	}

	if p.PluginsDir == "" {
		return fmt.Errorf("PluginsDir is required for tofu executor")
	}

	if p.WorkingDir == "" {
		return fmt.Errorf("WorkingDir is required for tofu executor")
	}

	if p.Step == "" {
		return fmt.Errorf("Step is required for tofu executor")
	}

	return nil
}

type Executor struct {
	params ExecutorParams

	logger log.Logger
	cmd    *exec.Cmd
}

func NewExecutor(params ExecutorParams, logger log.Logger) (*Executor, error) {
	if err := params.validate(); err != nil {
		return nil, err
	}

	if govalue.IsNil(logger) {
		logger = log.GetDefaultLogger()
	}

	return &Executor{
		params: params,
		logger: logger,
	}, nil
}

func (e *Executor) IsVMChange(rc plan.ResourceChange) bool {
	if e.params.VMChangeTester == nil {
		return false
	}

	return e.params.VMChangeTester(rc)
}

func (e *Executor) GetStatesDir() string {
	return e.params.RootDir
}

func (e *Executor) Step() infrastructure.Step {
	return e.params.Step
}

func (e *Executor) Init(ctx context.Context) error {
	args := []string{
		"init",
		fmt.Sprintf("-plugin-dir=%s", e.params.PluginsDir),
		"-no-color",
		"-input=false",
	}

	e.cmd = tofuCmd(ctx, e.params.RunExecutorParams, e.params.WorkingDir, args...)
	_, err := infraexec.Exec(ctx, e.cmd, e.logger)

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
			e.params.WorkingDir,
		)
	}

	e.cmd = tofuCmd(ctx, e.params.RunExecutorParams, e.params.WorkingDir, args...)

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

	e.cmd = tofuCmd(ctx, e.params.RunExecutorParams, e.params.WorkingDir, args...)

	return infraexec.Exec(ctx, e.cmd, e.logger)
}

func (e *Executor) Output(ctx context.Context, statePath string, outFielda ...string) (result []byte, err error) {
	cmd, out, err := tofuOutputRun(ctx, e.params.RunExecutorParams, statePath, outFielda...)
	e.cmd = cmd
	return out, err
}

func (e *Executor) Destroy(ctx context.Context, opts infrastructure.DestroyOpts) error {
	args := []string{
		"destroy",
		"-no-color",
		"-auto-approve",
		fmt.Sprintf("-var-file=%s", opts.VariablesPath),
		fmt.Sprintf("-state=%s", opts.StatePath),
	}

	e.cmd = tofuCmd(ctx, e.params.RunExecutorParams, e.params.WorkingDir, args...)

	_, err := infraexec.Exec(ctx, e.cmd, e.logger)

	return err
}

func (e *Executor) Show(ctx context.Context, planPath string) (result []byte, err error) {
	args := []string{
		"show",
		"-json",
		planPath,
	}

	e.cmd = tofuCmd(ctx, e.params.RunExecutorParams, e.params.WorkingDir, args...)

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
