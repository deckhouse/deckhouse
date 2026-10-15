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
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	infraexec "github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/exec"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/plan"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

func metaConfigForTesting(provider, prefix, layout string) *config.MetaConfig {
	cfg := config.MetaConfig{}
	cfg.ProviderName = provider
	cfg.ClusterPrefix = prefix
	cfg.Layout = layout

	return &cfg
}

func newTestRunner(executor Executor) *Runner {
	return NewRunner(metaConfigForTesting("test-provider", "test-prefix", "test-layout"), &cache.DummyCache{}, executor)
}

func TestCheckPlanDestructiveChanges(t *testing.T) {
	tests := []struct {
		name    string
		plan    string
		changes *destructiveChangesReport
		err     error
		vm      string
	}{
		{
			name:    "Empty Changes",
			plan:    "./mocks/checkplan/empty.json",
			changes: &destructiveChangesReport{changes: (*plan.DestructiveChanges)(nil), hasVMChanges: false},
			err:     nil,
		},
		{
			name:    "Has destructive changes",
			plan:    "./mocks/checkplan/destructively_changed.json",
			changes: destructiveChangesReportVar,
			vm:      "yandex_compute_instance",
		},
		{
			name:    "Has destructive changes but without VM",
			plan:    "./mocks/checkplan/destructively_changed_without_vm.json",
			changes: destructiveChangesReportWithoutVM,
			err:     nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(tc.plan)
			require.NoError(t, err)

			runner := newTestRunner(&fakeExecutor{
				showResp: fakeResponse{
					resp: data,
					err:  nil,
					code: 0,
				},
				VMResource: tc.vm,
			})

			changes, err := runner.getPlanDestructiveChanges(t.Context(), "")
			if tc.err != nil {
				require.EqualError(t, err, tc.err.Error())
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.changes, changes)
		})
	}
}

func newTestRunnerWithChanges() *Runner {
	r := NewRunner(metaConfigForTesting("a", "b", "c"), &cache.DummyCache{}, &fakeExecutor{data: map[string]fakeResponse{}})
	r.changesInPlan = plan.HasChanges
	return r
}

func TestRunnerCreatesStateSaver(t *testing.T) {
	tests := []struct {
		name         string
		cache        state.Cache
		destinations int
	}{
		{
			name:         "Dummy cache does create saver with empty destinations",
			cache:        &cache.DummyCache{},
			destinations: 0,
		},

		{
			name:         "File cache does create saver with empty destinations",
			cache:        &cache.StateCache{},
			destinations: 0,
		},

		{
			name:         "K8s cache does create saver with one destination",
			cache:        &client.StateCache{},
			destinations: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := NewRunner(metaConfigForTesting("a", "b", "c"), tc.cache, &fakeExecutor{data: map[string]fakeResponse{}})
			require.NotNil(t, runner)
			require.NotNil(t, runner.stateSaver)
			require.Len(t, runner.stateSaver.saversDestinations, tc.destinations)
		})
	}
}

func TestCheckRunnerHandleChanges(t *testing.T) {
	tests := []struct {
		name   string
		runner *Runner
		skip   bool
		err    error
	}{
		{
			name: "Yes and skip must not skip",
			skip: false,
			err:  nil,
			runner: newTestRunnerWithChanges().
				WithSkipChangesOnDeny(true).
				WithConfirm(func() *input.Confirmation {
					return input.NewConfirmation().WithYesByDefault()
				}),
		},
		{
			name: "Yes without skip must not skip",
			skip: false,
			err:  nil,
			runner: newTestRunnerWithChanges().
				WithConfirm(func() *input.Confirmation {
					return input.NewConfirmation().WithYesByDefault()
				}),
		},
		{
			name: "No and skip must skip",
			skip: true,
			err:  nil,
			runner: newTestRunnerWithChanges().
				WithSkipChangesOnDeny(true),
		},
		{
			name:   "No without skip must throw an error",
			skip:   false,
			err:    ErrInfrastructureApplyAborted,
			runner: newTestRunnerWithChanges(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			skip, err := tc.runner.isSkipChanges(t.Context())
			require.Equal(t, tc.skip, skip)
			if tc.err != nil {
				require.Error(t, err)
				require.EqualError(t, tc.err, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRunnerPlan(t *testing.T) {
	tests := []struct {
		name                       string
		showResp                   fakeResponse
		planResp                   fakeResponse
		vmResource                 string
		expectedChangesInPlan      int
		expectedDestructiveChanges *plan.DestructiveChanges
		expectedHasVMDestruction   bool
		expectedErrSubstring       string
	}{
		{
			name:                       "non-destructive changes",
			showResp:                   fakeResponse{resp: []byte("{\"format_version\":\"0.1\",\"resource_changes\":[]}")},
			planResp:                   fakeResponse{code: infraexec.HasChangesExitCode},
			expectedChangesInPlan:      plan.HasChanges,
			expectedDestructiveChanges: nil,
			expectedHasVMDestruction:   false,
		},
		{
			name:                       "destructive changes",
			showResp:                   fakeResponse{resp: mustReadFile(t, "./mocks/checkplan/destructively_changed.json")},
			planResp:                   fakeResponse{code: infraexec.HasChangesExitCode},
			vmResource:                 "yandex_compute_instance",
			expectedChangesInPlan:      plan.HasDestructiveChanges,
			expectedDestructiveChanges: destructivelyChanged,
			expectedHasVMDestruction:   true,
		},
		{
			name:                  "show error returns without panic",
			showResp:              fakeResponse{err: errors.New("show failed")},
			planResp:              fakeResponse{code: infraexec.HasChangesExitCode},
			expectedChangesInPlan: plan.HasChanges,
			expectedErrSubstring:  "show failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := newTestRunner(&fakeExecutor{
				showResp:   tc.showResp,
				planResp:   tc.planResp,
				VMResource: tc.vmResource,
			})

			err := runner.Plan(t.Context(), false, false)
			if tc.expectedErrSubstring != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.expectedErrSubstring)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.expectedChangesInPlan, runner.GetChangesInPlan())
			require.Equal(t, tc.expectedDestructiveChanges, runner.GetPlanDestructiveChanges())
			require.Equal(t, tc.expectedHasVMDestruction, runner.HasVMDestruction())
		})
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	return data
}

type sleepExecutor struct {
	cancelCh chan struct{}
}

func (e *sleepExecutor) IsVMChange(rc plan.ResourceChange) bool {
	return false
}

func (e *sleepExecutor) GetStatesDir() string {
	return ""
}

func (e *sleepExecutor) Step() Step {
	return ""
}

func (e *sleepExecutor) Init(ctx context.Context) error {
	return nil
}

func (e *sleepExecutor) Apply(ctx context.Context, opts ApplyOpts) error {
	return nil
}

func (e *sleepExecutor) Plan(ctx context.Context, opts PlanOpts) (exitCode int, err error) {
	ticker := time.NewTicker(time.Second)
loop:
	for {
		select {
		case <-ticker.C:
			continue
		case <-e.cancelCh:
			break loop
		}
	}
	return 0, nil
}

func (e *sleepExecutor) Output(ctx context.Context, opts OutputOpts) (result []byte, err error) {
	return nil, nil
}

func (e *sleepExecutor) Destroy(ctx context.Context, opts DestroyOpts) error {
	return nil
}

func (e *sleepExecutor) Show(ctx context.Context, opts ShowOpts) (result []byte, err error) {
	return nil, nil
}

func (e *sleepExecutor) GetActions(ctx context.Context, planPath string) (action []string, err error) {
	return nil, nil
}

func (e *sleepExecutor) Stop() {
	close(e.cancelCh)
}

func TestConcurrentExec(t *testing.T) {
	ctx := t.Context()

	exec := sleepExecutor{cancelCh: make(chan struct{})}
	defer exec.Stop()

	runner := newTestRunner(&exec)

	go func() {
		_, _ = runner.execInfrastructureUtility(ctx, func(ctx context.Context) (int, error) {
			return exec.Plan(ctx, PlanOpts{})
		})
	}()

	time.Sleep(500 * time.Millisecond)
	_, err := runner.execInfrastructureUtility(ctx, func(ctx context.Context) (int, error) {
		return exec.Plan(ctx, PlanOpts{})
	})

	require.Equal(t, "Infrastructure utility has already been executed.", err.Error())
}

var destructivelyChanged = &plan.DestructiveChanges{
	ResourcesDeleted: nil,
	ResourcesRecreated: []plan.ValueChange{
		{
			CurrentValue: map[string]any{
				"allow_stopping_for_update": true,
				"boot_disk": []any{
					map[string]any{
						"auto_delete":       true,
						"device_name":       "test",
						"disk_id":           "test",
						"initialize_params": []any{map[string]any{"description": "", "image_id": "tests", "name": "kubernetes-data-root", "size": float64(35), "snapshot_id": "", "type": "network-ssd"}},
						"mode":              "READ_WRITE",
					},
				},
				"created_at":                "2021-02-26T09:41:42Z",
				"description":               "",
				"folder_id":                 "test",
				"fqdn":                      "kube-master",
				"hostname":                  "kube-master",
				"id":                        "test",
				"labels":                    map[string]any{},
				"metadata":                  map[string]any{"ssh-keys": "", "user-data": ""},
				"name":                      "kube-master",
				"network_acceleration_type": "standard",
				"network_interface":         []any{map[string]any{"index": float64(0), "ip_address": "10.233.2.21", "ipv4": true, "ipv6": false, "ipv6_address": "", "mac_address": "test", "nat": false, "nat_ip_address": "", "nat_ip_version": "", "security_group_ids": []any{}, "subnet_id": "test"}},
				"platform_id":               "standard-v2",
				"resources":                 []any{map[string]any{"core_fraction": float64(100), "cores": float64(4), "gpus": float64(0), "memory": float64(8)}},
				"scheduling_policy":         []any{map[string]any{"preemptible": false}},
				"secondary_disk":            []any{map[string]any{"auto_delete": false, "device_name": "kubernetes-data", "disk_id": "test", "mode": "READ_WRITE"}},
				"service_account_id":        "",
				"status":                    "running",
				"timeouts":                  nil,
				"zone":                      "ru-central1-c",
			},
			NextValue: map[string]any{
				"allow_stopping_for_update": true,
				"boot_disk": []any{map[string]any{
					"auto_delete":       true,
					"initialize_params": []any{map[string]any{"image_id": "test", "size": float64(45), "type": "network-ssd"}},
				}},
				"description":               nil,
				"hostname":                  "kube-master",
				"labels":                    nil,
				"metadata":                  map[string]any{"node-network-cidr": "10.233.0.0/22", "ssh-keys": "test", "user-data": ""},
				"name":                      "kube-master",
				"network_acceleration_type": "standard",
				"network_interface":         []any{map[string]any{"ipv4": true, "nat": false, "subnet_id": "test"}},
				"platform_id":               "standard-v2",
				"resources":                 []any{map[string]any{"core_fraction": float64(100), "cores": float64(4), "gpus": nil, "memory": float64(8)}},
				"secondary_disk":            []any{map[string]any{"auto_delete": false, "device_name": "kubernetes-data", "disk_id": "test", "mode": "READ_WRITE"}},
				"timeouts":                  nil,
				"zone":                      "ru-central1-c",
			},
			Type: "yandex_compute_instance",
		},
	},
}

var destructiveChangesReportVar = &destructiveChangesReport{
	changes:      destructivelyChanged,
	hasVMChanges: true,
}

var destructivelyChangedWithoutVM = &plan.DestructiveChanges{
	ResourcesDeleted: nil,
	ResourcesRecreated: []plan.ValueChange{
		{
			CurrentValue: map[string]any{
				"created_at":  "2021-02-26T09:41:43Z",
				"description": "",
				"external_ipv4_address": []any{
					map[string]any{
						"address": "203.0.113.10",
						"zone_id": "ru-central1-c",
					},
				},
				"folder_id": "test",
				"id":        "addr-old",
				"labels":    map[string]any{},
				"name":      "kube-master-eip",
				"timeouts":  nil,
			},
			NextValue: map[string]any{
				"description": "",
				"external_ipv4_address": []any{
					map[string]any{
						"zone_id": "ru-central1-c",
					},
				},
				"labels":   map[string]any{},
				"name":     "kube-master-eip",
				"timeouts": nil,
			},
			Type: "yandex_vpc_address",
		},
	},
	Provider: "",
}

var destructiveChangesReportWithoutVM = &destructiveChangesReport{
	changes:      destructivelyChangedWithoutVM,
	hasVMChanges: false,
}

func newVMDestructionTestRunner(t *testing.T) (*Runner, *fakeExecutor) {
	t.Helper()

	executor := &fakeExecutor{
		showResp:   fakeResponse{resp: mustReadFile(t, "./mocks/checkplan/destructively_changed.json")},
		planResp:   fakeResponse{code: infraexec.HasChangesExitCode},
		VMResource: "yandex_compute_instance",
	}

	runner := newTestRunner(executor).
		WithVariables([]byte("old variables")).
		WithHook(&recordingHook{executor: executor})

	// A test may clear variablesPath, so remove the file created here. planPath is
	// read at cleanup time: a successful replan removes the previous plan itself.
	variablesPath := runner.variablesPath
	t.Cleanup(func() {
		_ = os.Remove(variablesPath)
		_ = os.Remove(runner.planPath)
	})

	require.NoError(t, runner.Plan(t.Context(), false, false))
	require.True(t, runner.HasVMDestruction())

	return runner, executor
}

type recordingHook struct {
	executor *fakeExecutor
}

func (h *recordingHook) BeforeAction(context.Context, RunnerInterface) (bool, error) {
	h.executor.calls = append(h.executor.calls, "before")
	return false, nil
}

func (h *recordingHook) IsReady() error { return nil }

func (h *recordingHook) AfterAction(context.Context, RunnerInterface, error) error {
	h.executor.calls = append(h.executor.calls, "after")
	return nil
}

func TestRunnerRefreshVariablesAndReplanBeforeVMDestruction(t *testing.T) {
	runner, executor := newVMDestructionTestRunner(t)
	oldPlanPath := runner.GetPlanPath()

	runner.WithVariablesRefresher(func(context.Context) ([]byte, error) {
		executor.planResp = fakeResponse{code: 0}
		return []byte("new variables"), nil
	})

	require.NoError(t, runner.refreshVariablesAndReplanBeforeVMDestruction(t.Context()))

	require.Equal(t, []string{"plan:old variables", "plan:new variables"}, executor.calls)
	require.Equal(t, []byte("new variables"), runner.variablesData)
	require.Equal(t, plan.HasNoChanges, runner.GetChangesInPlan())
	require.False(t, runner.HasVMDestruction())
	require.Nil(t, runner.GetPlanDestructiveChanges())
	require.NotEqual(t, oldPlanPath, runner.GetPlanPath())
	require.NoFileExists(t, oldPlanPath)
}

func TestRunnerRefreshVariablesFailures(t *testing.T) {
	tests := []struct {
		name          string
		refresher     VariablesRefresher
		emptyVarsPath bool
		planErr       error
		errContains   string
		variables     string
	}{
		{
			name:        "refresher error",
			refresher:   func(context.Context) ([]byte, error) { return nil, errors.New("boom") },
			errContains: "failed to refresh infrastructure variables before VM destruction: boom",
			variables:   "old variables",
		},
		{
			name:        "empty refreshed variables",
			refresher:   func(context.Context) ([]byte, error) { return nil, nil },
			errContains: "refreshed infrastructure variables are empty",
			variables:   "old variables",
		},
		{
			name:          "empty variables path",
			refresher:     func(context.Context) ([]byte, error) { return []byte("new variables"), nil },
			emptyVarsPath: true,
			errContains:   "variables path is empty",
			variables:     "old variables",
		},
		{
			// The file on disk already holds the new variables, and so does the runner.
			name:        "plan error",
			refresher:   func(context.Context) ([]byte, error) { return []byte("new variables"), nil },
			planErr:     errors.New("plan failed"),
			errContains: "failed to rebuild infrastructure plan with refreshed variables: plan failed",
			variables:   "new variables",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner, executor := newVMDestructionTestRunner(t)
			refreshed := false
			runner.WithVariablesRefresher(func(ctx context.Context) ([]byte, error) {
				refreshed = true
				return tt.refresher(ctx)
			})
			executor.planResp = fakeResponse{err: tt.planErr}
			if tt.emptyVarsPath {
				runner.variablesPath = ""
			}

			err := runner.refreshVariablesAndReplanBeforeVMDestruction(t.Context())
			require.ErrorContains(t, err, tt.errContains)
			require.Equal(t, []byte(tt.variables), runner.variablesData)
			// Without a variables path the refresher must not even be called.
			require.Equal(t, !tt.emptyVarsPath, refreshed)
		})
	}
}

func TestRunnerSkipsVariablesRefreshWithoutVMDestruction(t *testing.T) {
	executor := &fakeExecutor{
		showResp:   fakeResponse{resp: mustReadFile(t, "./mocks/checkplan/destructively_changed_without_vm.json")},
		planResp:   fakeResponse{code: infraexec.HasChangesExitCode},
		VMResource: "yandex_compute_instance",
	}

	refreshCalls := 0
	runner := newTestRunner(executor).
		WithVariables([]byte("old variables")).
		WithVariablesRefresher(func(context.Context) ([]byte, error) {
			refreshCalls++
			return []byte("new variables"), nil
		})

	variablesPath := runner.variablesPath
	t.Cleanup(func() {
		_ = os.Remove(variablesPath)
		_ = os.Remove(runner.planPath)
	})

	require.NoError(t, runner.Plan(t.Context(), false, false))
	require.Equal(t, plan.HasDestructiveChanges, runner.GetChangesInPlan())
	require.False(t, runner.HasVMDestruction())

	require.NoError(t, runner.refreshVariablesAndReplanBeforeVMDestruction(t.Context()))
	require.Zero(t, refreshCalls)
}

func TestRunnerApplyRefreshesVariablesBeforeConfirm(t *testing.T) {
	refreshErr := errors.New("boom")

	tests := []struct {
		name             string
		confirm          bool
		dismissDestroy   bool
		dismissChanges   bool
		refreshErr       error
		expectedErr      error
		expectedCalls    []string
		expectedConfirms int
	}{
		{
			name:             "confirmed",
			confirm:          true,
			expectedCalls:    []string{"plan:old variables", "refresh", "plan:new variables", "before", "apply:new variables", "after"},
			expectedConfirms: 1,
		},
		{
			name:             "denied",
			expectedCalls:    []string{"plan:old variables", "refresh", "plan:new variables"},
			expectedConfirms: 1,
		},
		{
			name:           "destructive changes dismissed",
			dismissDestroy: true,
			expectedCalls:  []string{"plan:old variables"},
		},
		{
			name:           "changes dismissed",
			dismissChanges: true,
			expectedErr:    ErrInfrastructureApplyAborted,
			expectedCalls:  []string{"plan:old variables"},
		},
		{
			name:          "refresh error",
			refreshErr:    refreshErr,
			expectedErr:   refreshErr,
			expectedCalls: []string{"plan:old variables", "refresh"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner, executor := newVMDestructionTestRunner(t)

			confirms := 0
			runner.
				WithSkipChangesOnDeny(true).
				WithAutoDismissDestructiveChanges(tt.dismissDestroy).
				WithAutoDismissChanges(tt.dismissChanges).
				WithConfirm(func() *input.Confirmation {
					confirms++
					if tt.confirm {
						return input.NewConfirmation().WithYesByDefault()
					}
					return input.NewConfirmation()
				}).
				WithVariablesRefresher(func(context.Context) ([]byte, error) {
					executor.calls = append(executor.calls, "refresh")
					return []byte("new variables"), tt.refreshErr
				})

			err := runner.Apply(t.Context())
			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tt.expectedCalls, executor.calls)
			require.Equal(t, tt.expectedConfirms, confirms)
		})
	}
}
