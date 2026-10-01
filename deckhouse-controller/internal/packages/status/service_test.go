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
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/werf/nelm/pkg/legacy/progrep"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/health"
)

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

func TestSetNoWorkloads(t *testing.T) {
	const name = "module"

	scaled := func(s *Service) Condition {
		cond, _ := s.statuses[name].condition(ConditionScaled)
		return cond
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

	t.Run("marks unobserved package scaled", func(t *testing.T) {
		s := newService(t)

		s.SetNoWorkloads(name, true)

		cond := scaled(s)
		assert.Equal(t, metav1.ConditionTrue, cond.Status)
		assert.Equal(t, ConditionReasonNoWorkloads, cond.Reason)
		assert.Equal(t, 1, s.ModuleQueue().Len())
	})

	t.Run("keeps monitor verdict", func(t *testing.T) {
		s := newService(t)

		s.UpdateHealth(name, health.Event{Health: health.Health{State: health.StateReconciling}})
		s.SetNoWorkloads(name, true)

		cond := scaled(s)
		assert.Equal(t, metav1.ConditionFalse, cond.Status)
		assert.Equal(t, ConditionReason(health.StateReconciling), cond.Reason)
	})

	t.Run("monitor overrides it once workloads appear", func(t *testing.T) {
		s := newService(t)

		s.SetNoWorkloads(name, true)
		s.UpdateHealth(name, health.Event{Health: health.Health{State: health.StateReconciling}})

		assert.Equal(t, metav1.ConditionFalse, scaled(s).Status)
	})

	t.Run("last workload removed maps unknown to no workloads", func(t *testing.T) {
		s := newService(t)

		s.UpdateHealth(name, health.Event{Health: health.Health{State: health.StateScaled}})
		s.SetNoWorkloads(name, true)
		s.UpdateHealth(name, health.Event{Health: health.Health{State: health.StateUnknown}})

		cond := scaled(s)
		assert.Equal(t, metav1.ConditionTrue, cond.Status)
		assert.Equal(t, ConditionReasonNoWorkloads, cond.Reason)
	})

	t.Run("workloads coming back reset it to unknown", func(t *testing.T) {
		s := newService(t)

		s.SetNoWorkloads(name, true)
		s.SetNoWorkloads(name, false)

		assert.Equal(t, metav1.ConditionUnknown, scaled(s).Status)
	})

	t.Run("survives version reset", func(t *testing.T) {
		s := newService(t)

		s.SetNoWorkloads(name, true)
		s.NewStatus(name)
		s.UpdateHealth(name, health.Event{Health: health.Health{State: health.StateReconciling}})
		s.UpdateHealth(name, health.Event{Health: health.Health{State: health.StateUnknown}})

		assert.Equal(t, ConditionReasonNoWorkloads, scaled(s).Reason)
	})
}

func TestHandleError(t *testing.T) {
	const name = "module"

	hooksProcessed := func(s *Service) Condition {
		cond, _ := s.statuses[name].condition(ConditionHooksProcessed)
		return cond
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

	err := NewError("HookFailed", errors.New("boom"))

	t.Run("records status error", func(t *testing.T) {
		s := newService(t)

		s.HandleError(context.Background(), name, ConditionHooksProcessed, err)

		cond := hooksProcessed(s)
		assert.Equal(t, metav1.ConditionFalse, cond.Status)
		assert.Equal(t, ConditionReason("HookFailed"), cond.Reason)
	})

	t.Run("skips cancelled context", func(t *testing.T) {
		s := newService(t)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		s.HandleError(ctx, name, ConditionHooksProcessed, err)

		assert.Equal(t, metav1.ConditionUnknown, hooksProcessed(s).Status)
		assert.Zero(t, s.ModuleQueue().Len())
	})
}
