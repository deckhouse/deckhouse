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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/plan"
	dhlog "github.com/deckhouse/deckhouse/dhctl/pkg/logger"
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
	NoOutput         bool
	// only use for debug purposes
	Target string
}

type OutputOpts struct {
	StatePath     string
	OutFields     []string
	ShowSensitive bool
}

type ShowOpts struct {
	PlanPath      string
	ShowSensitive bool
}

type OutputExecutor interface {
	Output(ctx context.Context, opts OutputOpts) (result []byte, err error)
}

type Executor interface {
	OutputExecutor

	Init(ctx context.Context) error
	Apply(ctx context.Context, opts ApplyOpts) error
	Plan(ctx context.Context, opts PlanOpts) (exitCode int, err error)
	Destroy(ctx context.Context, opts DestroyOpts) error
	Show(ctx context.Context, opts ShowOpts) (result []byte, err error)
	GetActions(ctx context.Context, planPath string) (action []string, err error)

	// TODO need refactoring getting plan changes. I do not have more time for deep refactoring
	IsVMChange(rc plan.ResourceChange) bool

	GetStatesDir() string
	Step() Step

	Stop()
}

type fakeResponse struct {
	err  error
	code int
	resp []byte
}

type fakeExecutor struct {
	data        map[string]fakeResponse
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

func (e *fakeExecutor) Plan(ctx context.Context, opts PlanOpts) (int, error) {
	return e.planResp.code, e.planResp.err
}

func (e *fakeExecutor) Output(ctx context.Context, opts OutputOpts) ([]byte, error) {
	return e.outputResp.resp, e.outputResp.err
}

func (e *fakeExecutor) Destroy(ctx context.Context, opts DestroyOpts) error {
	return e.destroyResp.err
}

func (e *fakeExecutor) Show(ctx context.Context, opts ShowOpts) ([]byte, error) {
	return e.showResp.resp, e.showResp.err
}

func (e *fakeExecutor) GetActions(ctx context.Context, planPath string) ([]string, error) {
	return []string{}, nil
}

func (e *fakeExecutor) Stop() {}

type DummyExecutor struct{}

func NewDummyExecutor() *DummyExecutor {
	return &DummyExecutor{}
}

func (e *DummyExecutor) IsVMChange(rc plan.ResourceChange) bool {
	ctx := context.Background()
	dhlog.FromContext(ctx).WarnContext(ctx, "Call IsVMChange on dummy executor")

	return false
}

func (e *DummyExecutor) GetStatesDir() string {
	ctx := context.Background()
	dhlog.FromContext(ctx).WarnContext(ctx, "Call GetStatesDir on dummy executor")
	return ""
}

func (e *DummyExecutor) Step() Step {
	ctx := context.Background()
	dhlog.FromContext(ctx).WarnContext(ctx, "Call Step on dummy executor")
	return ""
}

func (e *DummyExecutor) Init(ctx context.Context) error {
	dhlog.FromContext(ctx).WarnContext(ctx, "Call Init on dummy executor")
	return nil
}

func (e *DummyExecutor) Apply(ctx context.Context, opts ApplyOpts) error {
	dhlog.FromContext(ctx).WarnContext(ctx, "Call Apply on dummy executor")

	return nil
}

func (e *DummyExecutor) Plan(ctx context.Context, opts PlanOpts) (int, error) {
	dhlog.FromContext(ctx).WarnContext(ctx, "Call Plan on dummy executor")

	return 0, nil
}

func (e *DummyExecutor) Output(ctx context.Context, opts OutputOpts) ([]byte, error) {
	dhlog.FromContext(ctx).WarnContext(ctx, "Call Output on dummy executor")

	return nil, nil
}

func (e *DummyExecutor) Destroy(ctx context.Context, opts DestroyOpts) error {
	dhlog.FromContext(ctx).WarnContext(ctx, "Call Destroy on dummy executor")

	return nil
}

func (e *DummyExecutor) Show(ctx context.Context, opts ShowOpts) ([]byte, error) {
	dhlog.FromContext(ctx).WarnContext(ctx, "Call Show on dummy executor")

	return nil, nil
}

func (e *DummyExecutor) GetActions(ctx context.Context, planPath string) ([]string, error) {
	dhlog.FromContext(ctx).WarnContext(ctx, "Call GetActions on dummy executor")

	return nil, nil
}

func (e *DummyExecutor) Stop() {
	ctx := context.Background()
	dhlog.FromContext(ctx).WarnContext(ctx, "Call Stop on dummy executor")
}

type DummyOutputExecutor struct{}

func NewDummyOutputExecutor() *DummyOutputExecutor {
	return &DummyOutputExecutor{}
}

func (e *DummyOutputExecutor) Output(ctx context.Context, opts OutputOpts) ([]byte, error) {
	dhlog.FromContext(ctx).WarnContext(ctx, "Call Output on dummy output executor")

	return nil, nil
}

//nolint:prealloc
func GetActions(ctx context.Context, cmd *exec.Cmd) ([]string, error) {
	type state struct {
		ResourceChanges []struct {
			Change struct {
				Actions []string `json:"actions"`
			} `json:"change"`
		} `json:"resource_changes"`
	}

	buf := bytes.NewBuffer(make([]byte, 0, 5000))
	cmd.Stdout = buf
	actions := make([]string, 0)

	if err := cmd.Run(); err != nil {
		return actions, fmt.Errorf("failed to start terraform: %w", err)
	}

	var res state
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		return actions, fmt.Errorf("failed to unmarshal json: %w", err)
	}
	for _, i := range res.ResourceChanges {
		act := i.Change.Actions
		actions = append(actions, act...)
	}

	return actions, nil
}
