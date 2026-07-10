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
	"io"
	"os/exec"
	"syscall"

	otattribute "go.opentelemetry.io/otel/attribute"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	infraexec "github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/exec"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/plan"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
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

	cmd *exec.Cmd
}

func NewExecutor(params ExecutorParams) (*Executor, error) {
	if err := params.validate(); err != nil {
		return nil, err
	}

	return &Executor{
		params: params,
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
	ctx, span := telemetry.StartSpan(ctx, "tofu.init")
	defer span.End()
	span.SetAttributes(
		otattribute.String("pipeline_step", string(e.params.Step)),
		otattribute.String("working_dir", e.params.WorkingDir),
	)

	// Start (or reuse) the persistent kubernetes provider daemon before the
	// first tofu invocation so plan/apply across this and later pipelines
	// share one warm provider process.
	EnableProviderDaemon(e.params.PluginsDir)

	args := []string{
		"init",
		fmt.Sprintf("-plugin-dir=%s", e.params.PluginsDir),
		"-no-color",
		"-input=false",
	}

	e.cmd = tofuCmd(ctx, e.params.RunExecutorParams, e.params.WorkingDir, args...)
	_, err := infraexec.Exec(ctx, e.cmd, e.params.IsDebug)

	return err
}

func (e *Executor) Apply(ctx context.Context, opts infrastructure.ApplyOpts) error {
	ctx, span := telemetry.StartSpan(ctx, "tofu.apply")
	defer span.End()
	span.SetAttributes(
		otattribute.String("pipeline_step", string(e.params.Step)),
		otattribute.String("working_dir", e.params.WorkingDir),
		otattribute.Bool("from_plan", opts.PlanPath != ""),
	)

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

	_, err := infraexec.Exec(ctx, e.cmd, e.params.IsDebug)

	return err
}

func (e *Executor) Plan(ctx context.Context, opts infrastructure.PlanOpts) (int, error) {
	ctx, span := telemetry.StartSpan(ctx, "tofu.plan")
	defer span.End()
	span.SetAttributes(
		otattribute.String("pipeline_step", string(e.params.Step)),
		otattribute.String("working_dir", e.params.WorkingDir),
		otattribute.Bool("destroy", opts.Destroy),
		otattribute.Bool("detailed_exitcode", opts.DetailedExitCode),
	)

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

	if opts.Target != "" {
		targetArgs := []string{
			fmt.Sprintf("-target=%s", opts.Target),
			"-show-sensitive",
		}
		args = append(args, targetArgs...)
	}

	e.cmd = tofuCmd(ctx, e.params.RunExecutorParams, e.params.WorkingDir, args...)
	if opts.NoOutput {
		e.cmd.Stdout = io.Discard
		e.cmd.Stderr = io.Discard
	}

	return infraexec.Exec(ctx, e.cmd, e.params.IsDebug)
}

func (e *Executor) Output(ctx context.Context, opts infrastructure.OutputOpts) ([]byte, error) {
	cmd, out, err := tofuOutputRun(ctx, e.params.RunExecutorParams, opts)
	e.cmd = cmd
	return out, err
}

func (e *Executor) Destroy(ctx context.Context, opts infrastructure.DestroyOpts) error {
	ctx, span := telemetry.StartSpan(ctx, "tofu.destroy")
	defer span.End()
	span.SetAttributes(
		otattribute.String("pipeline_step", string(e.params.Step)),
		otattribute.String("working_dir", e.params.WorkingDir),
	)

	args := []string{
		"destroy",
		"-no-color",
		"-auto-approve",
		fmt.Sprintf("-var-file=%s", opts.VariablesPath),
		fmt.Sprintf("-state=%s", opts.StatePath),
	}

	e.cmd = tofuCmd(ctx, e.params.RunExecutorParams, e.params.WorkingDir, args...)

	_, err := infraexec.Exec(ctx, e.cmd, e.params.IsDebug)

	return err
}

func (e *Executor) Show(ctx context.Context, opts infrastructure.ShowOpts) ([]byte, error) {
	args := []string{
		"show",
		"-json",
	}

	if opts.ShowSensitive {
		args = append(args, "-show-sensitive")
	}

	args = append(args, opts.PlanPath)

	e.cmd = tofuCmd(ctx, e.params.RunExecutorParams, e.params.WorkingDir, args...)

	return e.cmd.Output()
}

func (e *Executor) Stop() {
	ctx := context.Background()
	dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Interrupting tofu process with pid: %d", e.cmd.Process.Pid))

	// 1. Tofu exits immediately on SIGTERM, so SIGINT is used here
	//    to interrupt it gracefully even when main process caught the SIGTERM.
	// 2. Negative pid is used to send signal to the process group
	//    started by "Setpgid: true" to prevent double signaling
	//    from shell and from us.
	//    See also pkg/system/ssh/cmd/ssh.go
	_ = syscall.Kill(-e.cmd.Process.Pid, syscall.SIGINT)
}

func (e *Executor) GetActions(ctx context.Context, planPath string) ([]string, error) {
	args := []string{
		"show",
		"-json",
		planPath,
	}

	cmd := tofuCmd(ctx, e.params.RunExecutorParams, e.params.WorkingDir, args...)
	return infrastructure.GetActions(ctx, cmd)
}
