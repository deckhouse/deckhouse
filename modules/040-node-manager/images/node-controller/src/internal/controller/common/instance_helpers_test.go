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

package common

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
)

func TestGetInstanceRebootingState(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 to scheme: %v", err)
	}
	if err := deckhousev1alpha2.AddToScheme(scheme); err != nil {
		t.Fatalf("add deckhousev1alpha2 to scheme: %v", err)
	}

	tests := []struct {
		name          string
		node          *corev1.Node
		instance      *deckhousev1alpha2.Instance
		expectActive  bool
		expectMessage string
	}{
		{
			name: "active while node is not ready",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "worker-0"},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionFalse, Reason: "KubeletReady"},
					},
				},
			},
			instance: &deckhousev1alpha2.Instance{
				ObjectMeta: metav1.ObjectMeta{Name: "worker-0"},
				Spec:       deckhousev1alpha2.InstanceSpec{NodeRef: deckhousev1alpha2.NodeRef{Name: "worker-0"}},
				Status: deckhousev1alpha2.InstanceStatus{
					Conditions: []deckhousev1alpha2.InstanceCondition{
						{
							Type:    deckhousev1alpha2.InstanceConditionTypeBashibleReady,
							Status:  metav1.ConditionUnknown,
							Reason:  deckhousev1alpha2.InstanceConditionReasonMachineReboot,
							Message: "Machine reboot requested by bashible reboot step",
						},
					},
				},
			},
			expectActive:  true,
			expectMessage: "Machine reboot requested by bashible reboot step",
		},
		{
			name: "inactive when node is ready again",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "worker-1"},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionTrue, Reason: "KubeletReady"},
					},
				},
			},
			instance: &deckhousev1alpha2.Instance{
				ObjectMeta: metav1.ObjectMeta{Name: "worker-1"},
				Spec:       deckhousev1alpha2.InstanceSpec{NodeRef: deckhousev1alpha2.NodeRef{Name: "worker-1"}},
				Status: deckhousev1alpha2.InstanceStatus{
					Conditions: []deckhousev1alpha2.InstanceCondition{
						{
							Type:    deckhousev1alpha2.InstanceConditionTypeBashibleReady,
							Status:  metav1.ConditionUnknown,
							Reason:  deckhousev1alpha2.InstanceConditionReasonMachineReboot,
							Message: "Machine reboot requested by bashible reboot step",
						},
					},
				},
			},
			expectActive:  false,
			expectMessage: "",
		},
		{
			name: "inactive when node ref is empty",
			instance: &deckhousev1alpha2.Instance{
				ObjectMeta: metav1.ObjectMeta{Name: "worker-2"},
				Status: deckhousev1alpha2.InstanceStatus{
					Conditions: []deckhousev1alpha2.InstanceCondition{
						{
							Type:    deckhousev1alpha2.InstanceConditionTypeBashibleReady,
							Status:  metav1.ConditionUnknown,
							Reason:  deckhousev1alpha2.InstanceConditionReasonMachineReboot,
							Message: "Machine reboot requested by bashible reboot step",
						},
					},
				},
			},
			expectActive:  false,
			expectMessage: "",
		},
		{
			name: "inactive when node is not found",
			instance: &deckhousev1alpha2.Instance{
				ObjectMeta: metav1.ObjectMeta{Name: "worker-3"},
				Spec:       deckhousev1alpha2.InstanceSpec{NodeRef: deckhousev1alpha2.NodeRef{Name: "worker-3"}},
				Status: deckhousev1alpha2.InstanceStatus{
					Conditions: []deckhousev1alpha2.InstanceCondition{
						{
							Type:    deckhousev1alpha2.InstanceConditionTypeBashibleReady,
							Status:  metav1.ConditionUnknown,
							Reason:  deckhousev1alpha2.InstanceConditionReasonMachineReboot,
							Message: "Machine reboot requested by bashible reboot step",
						},
					},
				},
			},
			expectActive:  false,
			expectMessage: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			builder := fake.NewClientBuilder().WithScheme(scheme)
			if tt.node != nil {
				builder = builder.WithObjects(tt.node)
			}
			cl := builder.Build()

			active, message, err := GetInstanceRebootingState(context.Background(), cl, tt.instance)
			if err != nil {
				t.Fatalf("GetInstanceRebootingState returned error: %v", err)
			}
			if active != tt.expectActive {
				t.Fatalf("expected active=%v, got %v", tt.expectActive, active)
			}
			if message != tt.expectMessage {
				t.Fatalf("expected message %q, got %q", tt.expectMessage, message)
			}
		})
	}
}
