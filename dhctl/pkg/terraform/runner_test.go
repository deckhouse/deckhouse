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

package terraform

import (
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

func newTestRunner() *Runner {
	return NewRunner("test-provider", "test-prefix", "test-layout", "test-step", &cache.DummyCache{})
}

func TestCheckPlanDestructiveChanges(t *testing.T) {
	tests := []struct {
		name    string
		plan    string
		changes *PlanDestructiveChanges
		err     error
	}{
		{
			name:    "Empty Changes",
			plan:    "./mocks/checkplan/empty.json",
			changes: nil,
			err:     nil,
		},
		{
			name:    "Has destructive changes",
			plan:    "./mocks/checkplan/destructively_changed.json",
			changes: destructivelyChanged,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(tc.plan)
			require.NoError(t, err)

			executor := &fakeExecutor{data: map[string]fakeResponse{
				"show": {code: 0, resp: data},
			}}

			runner := newTestRunner().withTerraformExecutor(executor)

			changes, err := runner.getPlanDestructiveChanges("")
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
	r := NewRunner("a", "b", "c", "d", &cache.DummyCache{})
	r.changesInPlan = PlanHasChanges
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
			runner := NewRunner("a", "b", "c", "d", tc.cache)
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
			err:    ErrTerraformApplyAborted,
			runner: newTestRunnerWithChanges(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			skip, err := tc.runner.isSkipChanges()
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

type sleepExecutor struct {
	cancelCh chan struct{}
}

func (s *sleepExecutor) Output(_ ...string) ([]byte, error) {
	return nil, nil
}

func (s *sleepExecutor) Exec(_ ...string) (int, error) {
	ticker := time.NewTicker(time.Second)
loop:
	for {
		select {
		case <-ticker.C:
			continue
		case <-s.cancelCh:
			break loop
		}
	}
	return 0, nil
}

func (s *sleepExecutor) Stop() { close(s.cancelCh) }

func TestConcurrentExec(t *testing.T) {
	exec := sleepExecutor{cancelCh: make(chan struct{})}
	defer exec.Stop()

	runner := newTestRunner().withTerraformExecutor(&exec)

	go func() {
		_, _ = runner.execTerraform()
	}()

	runtime.Gosched()
	_, err := runner.execTerraform()

	require.Equal(t, "Terraform have been already executed.", err.Error())
}

var destructivelyChanged = &PlanDestructiveChanges{
	ResourcesDeleted: nil,
	ResourcesRecreated: []ValueChange{
		{
			CurrentValue: map[string]any{
				"allow_stopping_for_update": true,
				"boot_disk": []any{map[string]any{
					"auto_delete":       true,
					"device_name":       "test",
					"disk_id":           "test",
					"initialize_params": []any{map[string]any{"description": "", "image_id": "tests", "name": "kubernetes-data-root", "size": float64(35), "snapshot_id": "", "type": "network-ssd"}},
					"mode":              "READ_WRITE"},
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
		},
	},
}
