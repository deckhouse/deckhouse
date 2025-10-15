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
	"context"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/plan"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

type Step string

const (
	BaseInfraStep  Step = "base-infrastructure"
	MasterNodeStep Step = "master-node"
	StaticNodeStep Step = "static-node"
)

func GetStepByNodeGroupName(nodeGroupName string) Step {
	if nodeGroupName == global.MasterNodeGroupName {
		return MasterNodeStep
	}
	return StaticNodeStep
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

type OutputExecutor interface {
	Output(ctx context.Context, statePath string, outFields ...string) (result []byte, err error)
}

type Executor interface {
	OutputExecutor

	Init(ctx context.Context) error
	Apply(ctx context.Context, opts ApplyOpts) error
	Plan(ctx context.Context, opts PlanOpts) (exitCode int, err error)
	Destroy(ctx context.Context, opts DestroyOpts) error
	Show(ctx context.Context, statePath string) (result []byte, err error)

	// TODO need refactoring getting plan changes. I do not have more time for deep refactoring
	IsVMChange(rc plan.ResourceChange) bool

	GetStatesDir() string
	Step() Step

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
	VMResource  string
}

func (e *fakeExecutor) IsVMChange(rc plan.ResourceChange) bool {
	if e.VMResource == "" {
		return false
	}

	return e.VMResource == rc.Type
}

func (e *fakeExecutor) GetStatesDir() string {
	return ""
}

func (e *fakeExecutor) Step() Step {
	return ""
}

func (e *fakeExecutor) Init(ctx context.Context) error {
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

func (e *DummyExecutor) IsVMChange(rc plan.ResourceChange) bool {
	e.logger.LogWarnLn("Call IsVMChange on dummy executor")

	return false
}

func (e *DummyExecutor) GetStatesDir() string {
	e.logger.LogWarnLn("Call GetStatesDir on dummy executor")
	return ""
}

func (e *DummyExecutor) Step() Step {
	e.logger.LogWarnLn("Call Step on dummy executor")
	return ""
}

func (e *DummyExecutor) Init(ctx context.Context) error {
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

type DummyOutputExecutor struct {
	logger log.Logger
}

func NewDummyOutputExecutor(logger log.Logger) *DummyOutputExecutor {
	return &DummyOutputExecutor{
		logger: logger,
	}
}

func (e *DummyOutputExecutor) Output(ctx context.Context, statePath string, outFields ...string) (result []byte, err error) {
	e.logger.LogWarnLn("Call Output on dummy output executor")

	return nil, nil
}
