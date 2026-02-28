/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package capi

import (
	"context"
	"testing"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	"github.com/deckhouse/node-controller/internal/controller/machine"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestMapInstanceToCAPIMachine(t *testing.T) {
	t.Parallel()

	mapFn := mapInstanceToCAPIMachine()

	tests := []struct {
		name        string
		instance    *deckhousev1alpha2.Instance
		expectedLen int
		expectNS    string
		expectName  string
	}{
		{
			name:        "no machine ref",
			instance:    &deckhousev1alpha2.Instance{},
			expectedLen: 0,
		},
		{
			name: "wrong api version",
			instance: &deckhousev1alpha2.Instance{
				Spec: deckhousev1alpha2.InstanceSpec{
					MachineRef: &deckhousev1alpha2.MachineRef{
						Kind:       "Machine",
						APIVersion: "machine.sapcloud.io/v1alpha1",
						Name:       "m1",
						Namespace:  "ns1",
					},
				},
			},
			expectedLen: 0,
		},
		{
			name: "wrong kind",
			instance: &deckhousev1alpha2.Instance{
				Spec: deckhousev1alpha2.InstanceSpec{
					MachineRef: &deckhousev1alpha2.MachineRef{
						Kind:       "MachineSet",
						APIVersion: capiv1beta2.GroupVersion.String(),
						Name:       "m1",
						Namespace:  "ns1",
					},
				},
			},
			expectedLen: 0,
		},
		{
			name: "valid machine ref",
			instance: &deckhousev1alpha2.Instance{
				Spec: deckhousev1alpha2.InstanceSpec{
					MachineRef: &deckhousev1alpha2.MachineRef{
						Kind:       "Machine",
						APIVersion: capiv1beta2.GroupVersion.String(),
						Name:       "m1",
						Namespace:  "ns1",
					},
				},
			},
			expectedLen: 1,
			expectNS:    "ns1",
			expectName:  "m1",
		},
		{
			name: "namespace fallback",
			instance: &deckhousev1alpha2.Instance{
				Spec: deckhousev1alpha2.InstanceSpec{
					MachineRef: &deckhousev1alpha2.MachineRef{
						Kind:       "Machine",
						APIVersion: capiv1beta2.GroupVersion.String(),
						Name:       "m2",
					},
				},
			},
			expectedLen: 1,
			expectNS:    machine.MachineNamespace,
			expectName:  "m2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requests := mapFn(context.Background(), tt.instance)
			if len(requests) != tt.expectedLen {
				t.Fatalf("expected %d requests, got %d", tt.expectedLen, len(requests))
			}
			if tt.expectedLen == 0 {
				return
			}
			if requests[0].Namespace != tt.expectNS || requests[0].Name != tt.expectName {
				t.Fatalf(
					"unexpected request: got %s/%s, want %s/%s",
					requests[0].Namespace,
					requests[0].Name,
					tt.expectNS,
					tt.expectName,
				)
			}
		})
	}
}

func TestCAPIInstanceWatchPredicate_Update(t *testing.T) {
	t.Parallel()

	pred := capiInstanceWatchPredicate()

	base := &deckhousev1alpha2.Instance{
		Spec: deckhousev1alpha2.InstanceSpec{
			MachineRef: &deckhousev1alpha2.MachineRef{
				Kind:       "Machine",
				APIVersion: capiv1beta2.GroupVersion.String(),
				Name:       "m1",
				Namespace:  "ns1",
			},
		},
		Status: deckhousev1alpha2.InstanceStatus{
			Phase:         deckhousev1alpha2.InstancePhaseRunning,
			MachineStatus: machine.MachineStatusReady,
			Conditions: []deckhousev1alpha2.InstanceCondition{
				{
					Type:               deckhousev1alpha2.InstanceConditionTypeMachineReady,
					Status:             metav1.ConditionTrue,
					Reason:             "Ready",
					LastTransitionTime: metav1.Now(),
				},
			},
		},
	}

	t.Run("ignore bashible-only updates", func(t *testing.T) {
		oldObj := base.DeepCopy()
		newObj := base.DeepCopy()
		newObj.Status.Conditions = append(newObj.Status.Conditions, deckhousev1alpha2.InstanceCondition{
			Type:               deckhousev1alpha2.InstanceConditionTypeBashibleReady,
			Status:             metav1.ConditionTrue,
			Reason:             "StepsCompleted",
			LastTransitionTime: metav1.Now(),
		})

		if pred.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}) {
			t.Fatal("expected predicate to ignore bashible-only status change")
		}
	})

	t.Run("ignore capi-owned field value changes when fields are present", func(t *testing.T) {
		oldObj := base.DeepCopy()
		newObj := base.DeepCopy()
		newObj.Status.Phase = deckhousev1alpha2.InstancePhaseProvisioned

		if pred.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}) {
			t.Fatal("expected predicate to ignore updates when owned fields are present")
		}
	})

	t.Run("react when phase is missing", func(t *testing.T) {
		oldObj := base.DeepCopy()
		newObj := base.DeepCopy()
		newObj.Status.Phase = ""

		if !pred.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}) {
			t.Fatal("expected predicate to react when phase is missing")
		}
	})

	t.Run("react when machineStatus is missing", func(t *testing.T) {
		oldObj := base.DeepCopy()
		newObj := base.DeepCopy()
		newObj.Status.MachineStatus = ""

		if !pred.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}) {
			t.Fatal("expected predicate to react when machineStatus is missing")
		}
	})

	t.Run("react when MachineReady condition is missing", func(t *testing.T) {
		oldObj := base.DeepCopy()
		newObj := base.DeepCopy()
		newObj.Status.Conditions = nil

		if !pred.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}) {
			t.Fatal("expected predicate to react when MachineReady condition is missing")
		}
	})
}
