// Copyright 2026 Flant JSC
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

package status

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/werf/nelm/pkg/legacy/progrep"
)

func TestCountProgress(t *testing.T) {
	operation := func(category progrep.OperationCategory, status progrep.OperationStatus) progrep.Operation {
		return progrep.Operation{Category: category, Status: status}
	}

	tests := []struct {
		name      string
		ops       []progrep.Operation
		completed int
		remaining int
	}{
		{
			name: "no operations",
		},
		{
			name: "stage and release operations are not counted",
			ops: []progrep.Operation{
				operation(progrep.OperationCategoryMeta, progrep.OperationStatusCompleted),
				operation(progrep.OperationCategoryRelease, progrep.OperationStatusCompleted),
				operation(progrep.OperationCategoryMeta, progrep.OperationStatusPending),
				operation(progrep.OperationCategoryRelease, progrep.OperationStatusPending),
			},
		},
		{
			name: "resource and track operations are counted",
			ops: []progrep.Operation{
				operation(progrep.OperationCategoryMeta, progrep.OperationStatusCompleted),
				operation(progrep.OperationCategoryResource, progrep.OperationStatusCompleted),
				operation(progrep.OperationCategoryTrack, progrep.OperationStatusCompleted),
				operation(progrep.OperationCategoryResource, progrep.OperationStatusProgressing),
				operation(progrep.OperationCategoryTrack, progrep.OperationStatusPending),
				operation(progrep.OperationCategoryMeta, progrep.OperationStatusPending),
			},
			completed: 2,
			remaining: 2,
		},
		{
			name: "failed and canceled operations remain",
			ops: []progrep.Operation{
				operation(progrep.OperationCategoryResource, progrep.OperationStatusCompleted),
				operation(progrep.OperationCategoryTrack, progrep.OperationStatusFailed),
				operation(progrep.OperationCategoryResource, progrep.OperationStatusCanceled),
				operation(progrep.OperationCategoryTrack, progrep.OperationStatusCanceled),
			},
			completed: 1,
			remaining: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			completed, remaining := countProgress(tt.ops)

			assert.Equal(t, tt.completed, completed)
			assert.Equal(t, tt.remaining, remaining)
		})
	}
}

func TestUpdateUninstallTrackingWritesThroughDeletionFreeze(t *testing.T) {
	const name = "ns.app"

	s := NewService()
	s.NewStatus(name)
	s.SetDeleting(name)

	report := progrep.ProgressReport{Operations: []progrep.Operation{
		{Category: progrep.OperationCategoryResource, Status: progrep.OperationStatusCompleted},
		{Category: progrep.OperationCategoryResource, Status: progrep.OperationStatusProgressing},
	}}

	// The install path is frozen along with the conditions it rewrites.
	s.UpdateTracking(name, report)
	assert.Empty(t, s.GetStatus(name).Tracking.Report.Operations)

	s.UpdateUninstallTracking(name, report)

	got := s.GetStatus(name)
	assert.Equal(t, Tracking{Completed: 1, Remaining: 1, Report: report}, got.Tracking)
	for _, cond := range got.Conditions {
		assert.Equal(t, ConditionReasonDeleting, cond.Reason, cond.Type)
	}

	// Trailing empty reports keep the last snapshot.
	s.UpdateUninstallTracking(name, progrep.ProgressReport{})
	assert.Equal(t, report, s.GetStatus(name).Tracking.Report)
}
