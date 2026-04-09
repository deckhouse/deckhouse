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

package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	ua "github.com/deckhouse/node-controller/internal/controller/updateapproval/common"
)

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

func withDisruptions(mode string) func(*v1.NodeGroup) {
	return func(ng *v1.NodeGroup) {
		if ng.Spec.Disruptions == nil {
			ng.Spec.Disruptions = &v1.DisruptionsSpec{}
		}
		ng.Spec.Disruptions.ApprovalMode = v1.DisruptionApprovalMode(mode)
	}
}

func TestCalculateNodeStatus(t *testing.T) {
	ng := newNodeGroup("worker", v1.NodeTypeStatic)
	ngManual := newNodeGroup("worker", v1.NodeTypeStatic, withDisruptions("Manual"))

	tests := []struct {
		name     string
		node     ua.NodeInfo
		ng       *v1.NodeGroup
		checksum string
		expected string
	}{
		{"WaitingForApproval", ua.NodeInfo{IsWaitingForApproval: true}, ng, "abc", "WaitingForApproval"},
		{"DrainingForDisruption", ua.NodeInfo{IsApproved: true, IsDisruptionRequired: true, IsDraining: true}, ng, "abc", "DrainingForDisruption"},
		{"WaitingForManualDisruptionApproval", ua.NodeInfo{IsApproved: true, IsDisruptionRequired: true}, ngManual, "abc", "WaitingForManualDisruptionApproval"},
		{"UpToDate", ua.NodeInfo{ConfigurationChecksum: "abc"}, ng, "abc", "UpToDate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, CalculateNodeStatus(tt.node, tt.ng, tt.checksum))
		})
	}
}
