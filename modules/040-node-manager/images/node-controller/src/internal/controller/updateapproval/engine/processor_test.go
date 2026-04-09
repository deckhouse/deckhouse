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

package engine

import (
	"context"
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

func withDisruptions(mode string, drainBefore *bool) func(*v1.NodeGroup) {
	return func(ng *v1.NodeGroup) {
		if ng.Spec.Disruptions == nil {
			ng.Spec.Disruptions = &v1.DisruptionsSpec{}
		}
		ng.Spec.Disruptions.ApprovalMode = v1.DisruptionApprovalMode(mode)
		if drainBefore != nil {
			if ng.Spec.Disruptions.Automatic == nil {
				ng.Spec.Disruptions.Automatic = &v1.AutomaticDisruptionSpec{}
			}
			ng.Spec.Disruptions.Automatic.DrainBeforeApproval = drainBefore
		}
	}
}

func withStatus(desired, ready, nodes int32) func(*v1.NodeGroup) {
	return func(ng *v1.NodeGroup) {
		ng.Status.Desired = desired
		ng.Status.Ready = ready
		ng.Status.Nodes = nodes
	}
}

func TestNeedDrainNode(t *testing.T) {
	ctx := context.Background()
	needDrain := func(deckhouseNodeName string, node *ua.NodeInfo, ng *v1.NodeGroup) bool {
		return Processor{DeckhouseNodeName: deckhouseNodeName}.NeedDrainNode(ctx, node, ng)
	}

	t.Run("single master node should not be drained", func(t *testing.T) {
		ng := newNodeGroup("master", v1.NodeTypeStatic, withStatus(1, 1, 1))
		node := &ua.NodeInfo{Name: "master-0", NodeGroup: "master"}
		assert.False(t, needDrain("", node, ng))
	})

	t.Run("deckhouse node should not be drained when only ready node", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeStatic, withStatus(2, 1, 2))
		node := &ua.NodeInfo{Name: "worker-1", NodeGroup: "worker"}
		assert.False(t, needDrain("worker-1", node, ng))
	})

	t.Run("deckhouse node can be drained when multiple ready nodes", func(t *testing.T) {
		drainBefore := true
		ng := newNodeGroup("worker", v1.NodeTypeStatic, withStatus(3, 3, 3), withDisruptions("Automatic", &drainBefore))
		node := &ua.NodeInfo{Name: "worker-1", NodeGroup: "worker"}
		assert.True(t, needDrain("worker-1", node, ng))
	})

	t.Run("respects DrainBeforeApproval=false", func(t *testing.T) {
		drainBefore := false
		ng := newNodeGroup("worker", v1.NodeTypeStatic, withDisruptions("Automatic", &drainBefore))
		node := &ua.NodeInfo{Name: "worker-1", NodeGroup: "worker"}
		assert.False(t, needDrain("", node, ng))
	})

	t.Run("defaults to true when no disruptions spec", func(t *testing.T) {
		ng := newNodeGroup("worker", v1.NodeTypeStatic, withStatus(3, 3, 3))
		node := &ua.NodeInfo{Name: "worker-1", NodeGroup: "worker"}
		assert.True(t, needDrain("", node, ng))
	})

	t.Run("multi-master can be drained", func(t *testing.T) {
		drainBefore := true
		ng := newNodeGroup("master", v1.NodeTypeStatic, withStatus(3, 3, 3), withDisruptions("Automatic", &drainBefore))
		node := &ua.NodeInfo{Name: "master-0", NodeGroup: "master"}
		assert.True(t, needDrain("", node, ng))
	})
}
