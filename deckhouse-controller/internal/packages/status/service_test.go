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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestSetMaintenanceMode(t *testing.T) {
	const name = "ns.app"

	maintenanceMode := func(s *Service) Condition {
		for _, cond := range s.GetStatus(name).Conditions {
			if cond.Type == ConditionMaintenanceMode {
				return cond
			}
		}

		return Condition{}
	}

	drain := func(s *Service) {
		for s.AppQueue().Len() > 0 {
			item, _ := s.AppQueue().Get()
			s.AppQueue().Done(item)
		}
	}

	newService := func(t *testing.T) *Service {
		s := NewService()
		s.NewStatus(name)
		t.Cleanup(func() {
			s.AppQueue().ShutDown()
			s.ModuleQueue().ShutDown()
		})

		return s
	}

	t.Run("new status seeds unknown", func(t *testing.T) {
		s := newService(t)

		assert.Equal(t, metav1.ConditionUnknown, maintenanceMode(s).Status)
	})

	t.Run("enabled marks no resource reconciliation", func(t *testing.T) {
		s := newService(t)

		s.SetMaintenanceMode(name, true)

		cond := maintenanceMode(s)
		assert.Equal(t, metav1.ConditionTrue, cond.Status)
		assert.Equal(t, ConditionReasonNoResourceReconciliation, cond.Reason)
		assert.NotEmpty(t, cond.Message)
	})

	t.Run("disabled clears reason and message", func(t *testing.T) {
		s := newService(t)

		s.SetMaintenanceMode(name, true)
		s.SetMaintenanceMode(name, false)

		cond := maintenanceMode(s)
		assert.Equal(t, metav1.ConditionFalse, cond.Status)
		assert.Empty(t, cond.Reason)
		assert.Empty(t, cond.Message)
	})

	t.Run("notifies only on change", func(t *testing.T) {
		s := newService(t)

		s.SetMaintenanceMode(name, true)
		assert.Equal(t, 1, s.AppQueue().Len())

		drain(s)
		s.SetMaintenanceMode(name, true)
		assert.Zero(t, s.AppQueue().Len())
	})

	t.Run("ignores untracked package", func(t *testing.T) {
		s := newService(t)

		s.SetMaintenanceMode("ns.other", true)

		assert.Empty(t, s.GetStatus("ns.other").Conditions)
		assert.Zero(t, s.AppQueue().Len())
	})
}
