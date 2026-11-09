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

package layout

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func testNode(name string, master bool) *corev1.Node {
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{}}}
	if master {
		node.Labels[ControlPlaneLabel] = ""
	}
	return node
}

func TestListStorageNodesListsMastersOnly(t *testing.T) {
	reader := fake.NewClientBuilder().WithObjects(
		testNode("master-0", true), testNode("worker-0", false), testNode("master-1", true),
	).Build()

	nodes, err := ListStorageNodes(context.Background(), reader)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"master-0", "master-1"}, nodes)
}

// A kubelet's status update on any node is not a master coming or going; only those are let through.
func TestStorageNodesChangedLetsThroughMastersComingAndGoing(t *testing.T) {
	changed := StorageNodesChanged()

	assert.True(t, changed.Create(event.CreateEvent{Object: testNode("master-3", true)}))
	assert.False(t, changed.Create(event.CreateEvent{Object: testNode("worker-3", false)}))
	assert.True(t, changed.Delete(event.DeleteEvent{Object: testNode("master-0", true)}))
	assert.False(t, changed.Delete(event.DeleteEvent{Object: testNode("worker-0", false)}))

	assert.False(t, changed.Update(event.UpdateEvent{
		ObjectOld: testNode("master-0", true), ObjectNew: testNode("master-0", true),
	}), "a master's status update")
	assert.False(t, changed.Update(event.UpdateEvent{
		ObjectOld: testNode("worker-0", false), ObjectNew: testNode("worker-0", false),
	}), "a worker's status update")
	assert.True(t, changed.Update(event.UpdateEvent{
		ObjectOld: testNode("node-0", false), ObjectNew: testNode("node-0", true),
	}), "a node becoming a master")
	assert.True(t, changed.Update(event.UpdateEvent{
		ObjectOld: testNode("node-0", true), ObjectNew: testNode("node-0", false),
	}), "a master losing the label")
}
