// Copyright 2021 Flant JSC
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

package infrastructure

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sync"
	"syscall"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

// based on https://stackoverflow.com/a/43931246
// https://regex101.com/r/qtIrSj/1
var infrastructureLogsMatcher = regexp.MustCompile(`(\s+\[(TRACE|DEBUG|INFO|WARN|ERROR)\]\s+|Use TF_LOG=TRACE|there is no package|\-\-\-\-)`)

func Exec(ctx context.Context, cmd *exec.Cmd, logger log.Logger) (int, error) {
	// Start infrastructure utility as a leader of the new process group to prevent
	// os.Interrupt (SIGINT) signal from the shell when Ctrl-C is pressed.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 1, fmt.Errorf("stdout pipe: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return 1, fmt.Errorf("stderr pipe: %v", err)
	}

	log.DebugLn(cmd.String())
	err = cmd.Start()
	if err != nil {
		log.ErrorF("Cannot start cmd: %v\n", err)
		return cmd.ProcessState.ExitCode(), err
	}

	var (
		wg     sync.WaitGroup
		errBuf bytes.Buffer
	)

	wg.Add(2)

	go func() {
		defer wg.Done()

		e := bufio.NewScanner(stderr)
		for e.Scan() {
			txt := e.Text()
			log.DebugLn(txt)

			if !app.IsDebug {
				if !infrastructureLogsMatcher.MatchString(txt) {
					errBuf.WriteString(txt + "\n")
				}
			}
		}
	}()

	go func() {
		defer wg.Done()

		s := bufio.NewScanner(stdout)
		for s.Scan() {
			logger.LogInfoLn(s.Text())
		}
	}()

	wg.Wait()

	err = cmd.Wait()

	exitCode := cmd.ProcessState.ExitCode() // 2 = exit code, if infrastructure plan has diff
	if err != nil && exitCode != hasChangesExitCode {
		logger.LogErrorF("Error while process exit code: %v\n", err)
		err = fmt.Errorf(errBuf.String())
		if app.IsDebug {
			err = fmt.Errorf("infrastructure utility has failed in DEBUG mode, search in the output above for an error")
		}
	}

	if exitCode == 0 {
		err = nil
	}
	return exitCode, err
}

type ApplyOpts struct {
	StatePath, PlanPath, VariablesPath string
}

type DestroyOpts struct {
	StatePath     string
	VariablesPath string
}

type PlanOpts struct {
	Destroy          bool
	StatePath        string
	VariablesPath    string
	OutPath          string
	DetailedExitCode bool
}

type Executor interface {
	Init(ctx context.Context, pluginsDir string) error
	Apply(ctx context.Context, opts ApplyOpts) error
	Plan(ctx context.Context, opts PlanOpts) (exitCode int, err error)
	Destroy(ctx context.Context, opts DestroyOpts) error
	Output(ctx context.Context, statePath string, outFields ...string) (result []byte, err error)
	Show(ctx context.Context, statePath string) (result []byte, err error)

	SetExecutorLogger(logger log.Logger)
	Stop()
}

type fakeResponse struct {
	err  error
	code int
	resp []byte
}

type fakeExecutor struct {
	data        map[string]fakeResponse
	logger      log.Logger
	outputResp  fakeResponse
	showResp    fakeResponse
	planResp    fakeResponse
	destroyResp fakeResponse
}

func (e *fakeExecutor) Init(ctx context.Context, pluginsDir string) error {
	return nil
}

func (e *fakeExecutor) Apply(ctx context.Context, opts ApplyOpts) error {
	return nil
}

func (e *fakeExecutor) Plan(ctx context.Context, opts PlanOpts) (exitCode int, err error) {
	return e.planResp.code, e.planResp.err
}

func (e *fakeExecutor) Output(ctx context.Context, statePath string, outFields ...string) (result []byte, err error) {
	return e.outputResp.resp, e.outputResp.err
}

func (e *fakeExecutor) Destroy(ctx context.Context, opts DestroyOpts) error {
	return e.destroyResp.err
}

func (e *fakeExecutor) Show(ctx context.Context, planPath string) (result []byte, err error) {
	return e.showResp.resp, e.showResp.err
}

func (e *fakeExecutor) SetExecutorLogger(logger log.Logger) {
	e.logger = logger
}

func (e *fakeExecutor) Stop() {}

type DummyExecutor struct {
	logger log.Logger
}

func NewDummyExecutor(logger log.Logger) *DummyExecutor {
	return &DummyExecutor{
		logger: logger,
	}
}

func (e *DummyExecutor) Init(ctx context.Context, pluginsDir string) error {
	e.logger.LogWarnLn("Call Init on dummy executor")
	return nil
}

func (e *DummyExecutor) Apply(ctx context.Context, opts ApplyOpts) error {
	e.logger.LogWarnLn("Call Apply on dummy executor")

	return nil
}

func (e *DummyExecutor) Plan(ctx context.Context, opts PlanOpts) (exitCode int, err error) {
	e.logger.LogWarnLn("Call Plan on dummy executor")

	return 0, nil
}

func (e *DummyExecutor) Output(ctx context.Context, statePath string, outFields ...string) (result []byte, err error) {
	e.logger.LogWarnLn("Call Output on dummy executor")

	return nil, nil
}

func (e *DummyExecutor) Destroy(ctx context.Context, opts DestroyOpts) error {
	e.logger.LogWarnLn("Call Destroy on dummy executor")

	return nil
}

func (e *DummyExecutor) Show(ctx context.Context, planPath string) (result []byte, err error) {
	e.logger.LogWarnLn("Call Show on dummy executor")

	return nil, nil
}

func (e *DummyExecutor) SetExecutorLogger(logger log.Logger) {
	e.logger = logger
}

func (e *DummyExecutor) Stop() {
	e.logger.LogWarnLn("Call Stop on dummy executor")
}
