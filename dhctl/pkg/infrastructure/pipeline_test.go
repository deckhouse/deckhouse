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
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/exec"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/plan"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

func TestGetMasterNodeResult(t *testing.T) {
	state, err := os.ReadFile("./mocks/pipeline/empty_state.json")
	require.NoError(t, err)

	tests := []struct {
		name        string
		outputResp  fakeResponse
		expectedRes *PipelineOutputs
		expectedErr error
	}{
		{
			name:       "Return values for master int",
			outputResp: fakeResponse{code: 0, resp: []byte(`1`)},
			expectedRes: &PipelineOutputs{
				InfrastructureState: state,
				MasterIPForSSH:      "1",
				NodeInternalIP:      "1",
				KubeDataDevicePath:  "1",
			},
			expectedErr: nil,
		},
		{
			name:       "Return values for master string",
			outputResp: fakeResponse{code: 0, resp: []byte(`"test-data"`)},
			expectedRes: &PipelineOutputs{
				InfrastructureState: state,
				MasterIPForSSH:      "test-data",
				NodeInternalIP:      "test-data",
				KubeDataDevicePath:  "test-data",
			},
			expectedErr: nil,
		},
		{
			name:        "With output error return err",
			outputResp:  fakeResponse{code: 1, err: fmt.Errorf("failed")},
			expectedRes: nil,
			expectedErr: fmt.Errorf("can't get infrastructure output for \"master_ip_address_for_ssh\"\nfailed"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := newTestRunner(&fakeExecutor{outputResp: tc.outputResp}).
				WithName("test").
				WithStatePath("./mocks/pipeline/empty_state.json")

			res, err := GetMasterNodeResult(context.Background(), runner)
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.expectedRes, res)
		})
	}
}

func TestCheckBaseInfrastructurePipeline(t *testing.T) {
	app.TmpDirName = "/tmp"

	okPlan, err := os.ReadFile("./mocks/pipeline/base_infra_ok_plan.json")
	require.NoError(t, err)

	discoveryData, err := os.ReadFile("./mocks/pipeline/cloud_discovery_data.json")
	require.NoError(t, err)

	discoveryDataWithNewZones, err := os.ReadFile("./mocks/pipeline/cloud_discovery_data_changed_zones.json")
	require.NoError(t, err)

	tests := []struct {
		name        string
		showResp    fakeResponse
		planResp    fakeResponse
		outputResp  fakeResponse
		expectedRes int
		expectedErr error
	}{
		{
			name:        "No changes",
			showResp:    fakeResponse{resp: okPlan},
			outputResp:  fakeResponse{resp: discoveryData},
			expectedRes: plan.HasNoChanges,
			expectedErr: nil,
		},
		{
			name:        "Changes exit code",
			planResp:    fakeResponse{code: exec.HasChangesExitCode},
			showResp:    fakeResponse{resp: okPlan},
			outputResp:  fakeResponse{resp: discoveryData},
			expectedRes: plan.HasChanges,
			expectedErr: nil,
		},
		{
			name:        "Changes exit code and changed zones",
			planResp:    fakeResponse{code: exec.HasChangesExitCode},
			showResp:    fakeResponse{resp: discoveryDataWithNewZones},
			expectedRes: plan.HasDestructiveChanges,
			expectedErr: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := newTestRunner(&fakeExecutor{
				showResp:   tc.showResp,
				outputResp: fakeResponse{resp: discoveryData},
				planResp:   tc.planResp,
			}).
				WithName("test").
				WithStatePath("./mocks/pipeline/empty_state.json")

			res, pl, _, err := CheckBaseInfrastructurePipeline(context.Background(), runner, "test")
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}

			var expectedPlan plan.Plan
			require.NoError(t, json.Unmarshal(tc.showResp.resp, &expectedPlan))

			require.Equal(t, tc.expectedRes, res)
			require.Equal(t, expectedPlan, pl)
		})
	}
}

func TestDestroyPipeline(t *testing.T) {
	tests := []struct {
		name        string
		stateFile   string
		destroyResp fakeResponse
		expectedErr error
	}{
		{
			name:        "Empty state runner ok is ok",
			stateFile:   "./mocks/pipeline/empty_state.json",
			destroyResp: fakeResponse{},
			expectedErr: nil,
		},
		{
			name:        "Empty state runner failed still ok",
			stateFile:   "./mocks/pipeline/empty_state.json",
			destroyResp: fakeResponse{err: fmt.Errorf("failed")},
			expectedErr: nil,
		},
		{
			name:        "Not empty state failed destroy returns error",
			stateFile:   "./mocks/pipeline/not_empty_state.json",
			destroyResp: fakeResponse{code: 1, err: fmt.Errorf("failed")},
			expectedErr: fmt.Errorf("failed"),
		},
		{
			name:        "Not empty state runner ok destroy is ok",
			stateFile:   "./mocks/pipeline/not_empty_state.json",
			destroyResp: fakeResponse{},
			expectedErr: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := newTestRunner(&fakeExecutor{destroyResp: tc.destroyResp}).
				WithName("test").
				WithConfirm(func() *input.Confirmation {
					return input.NewConfirmation().WithYesByDefault()
				}).
				WithStatePath(tc.stateFile)

			err := DestroyPipeline(context.Background(), runner, "test")
			if tc.expectedErr != nil {
				require.Contains(t, err.Error(), tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
