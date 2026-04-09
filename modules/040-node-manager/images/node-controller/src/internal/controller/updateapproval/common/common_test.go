/*
Copyright 2025 Flant JSC

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

package common_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	ua "github.com/deckhouse/node-controller/internal/controller/updateapproval/common"
)

func intStrPtr(v intstr.IntOrString) *intstr.IntOrString { return &v }

func newNodeGroup(name string, nodeType v1.NodeType, opts ...func(*v1.NodeGroup)) *v1.NodeGroup {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       v1.NodeGroupSpec{NodeType: nodeType},
		Status:     v1.NodeGroupStatus{Desired: 3, Ready: 3, Nodes: 3},
	}
	for _, opt := range opts {
		opt(ng)
	}
	return ng
}

func fixedTime() time.Time {
	return time.Date(2021, 1, 13, 13, 30, 0, 0, time.UTC)
}

func TestCalculateConcurrency(t *testing.T) {
	tests := []struct {
		name       string
		maxConc    *intstr.IntOrString
		totalNodes int
		expected   int
	}{
		{name: "nil returns 1", maxConc: nil, totalNodes: 10, expected: 1},
		{name: "int value 3", maxConc: intStrPtr(intstr.FromInt(3)), totalNodes: 10, expected: 3},
		{name: "string value 5", maxConc: intStrPtr(intstr.FromString("5")), totalNodes: 10, expected: 5},
		{name: "percentage 25%", maxConc: intStrPtr(intstr.FromString("25%")), totalNodes: 10, expected: 2},
		{name: "percentage 50%", maxConc: intStrPtr(intstr.FromString("50%")), totalNodes: 10, expected: 5},
		{name: "percentage 5% rounds up to 1", maxConc: intStrPtr(intstr.FromString("5%")), totalNodes: 10, expected: 1},
		{name: "percentage 100%", maxConc: intStrPtr(intstr.FromString("100%")), totalNodes: 10, expected: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ua.CalculateConcurrency(tt.maxConc, tt.totalNodes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildNodeInfo(t *testing.T) {
	t.Run("extracts all annotations correctly", func(t *testing.T) {
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "worker-1",
				Labels: map[string]string{ua.NodeGroupLabel: "worker"},
				Annotations: map[string]string{
					ua.ConfigurationChecksumAnnotation: "abc123",
					ua.ApprovedAnnotation:              "",
					ua.WaitingForApprovalAnnotation:    "",
					ua.DisruptionRequiredAnnotation:    "",
					ua.DisruptionApprovedAnnotation:    "",
					ua.RollingUpdateAnnotation:         "",
					ua.DrainingAnnotation:              "bashible",
					ua.DrainedAnnotation:               "bashible",
				},
			},
			Spec:   corev1.NodeSpec{Unschedulable: true},
			Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}}},
		}

		info := ua.BuildNodeInfo(node)
		assert.Equal(t, "worker-1", info.Name)
		assert.Equal(t, "worker", info.NodeGroup)
		assert.Equal(t, "abc123", info.ConfigurationChecksum)
		assert.True(t, info.IsApproved)
		assert.True(t, info.IsWaitingForApproval)
		assert.True(t, info.IsDisruptionRequired)
		assert.True(t, info.IsDisruptionApproved)
		assert.True(t, info.IsRollingUpdate)
		assert.True(t, info.IsDraining)
		assert.True(t, info.IsDrained)
		assert.True(t, info.IsUnschedulable)
		assert.True(t, info.IsReady)
	})

	t.Run("handles missing annotations", func(t *testing.T) {
		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "worker-1",
				Labels: map[string]string{ua.NodeGroupLabel: "worker"},
			},
			Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionFalse}}},
		}

		info := ua.BuildNodeInfo(node)
		assert.False(t, info.IsApproved)
		assert.False(t, info.IsWaitingForApproval)
		assert.False(t, info.IsDraining)
		assert.False(t, info.IsDrained)
		assert.False(t, info.IsReady)
	})
}

func TestIsInAllowedWindow(t *testing.T) {
	t.Run("empty windows always allowed", func(t *testing.T) {
		assert.True(t, ua.IsInAllowedWindow(nil, fixedTime()))
		assert.True(t, ua.IsInAllowedWindow([]v1.DisruptionWindow{}, fixedTime()))
	})

	t.Run("within window", func(t *testing.T) {
		windows := []v1.DisruptionWindow{{From: "13:00", To: "14:00"}}
		assert.True(t, ua.IsInAllowedWindow(windows, fixedTime()))
	})
}
