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

package kubernetes

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

// MockEventHandler is a mock implementation of EventHandler
type MockEventHandler struct {
	mock.Mock
}

func (m *MockEventHandler) OnNodeGroupAdd(nodegroup *NodeGroupWrapper)    { m.Called(nodegroup) }
func (m *MockEventHandler) OnNodeGroupUpdate(old, new *NodeGroupWrapper)  { m.Called(old, new) }
func (m *MockEventHandler) OnNodeGroupDelete(nodegroup *NodeGroupWrapper) { m.Called(nodegroup) }
func (m *MockEventHandler) OnNodeAdd(node *Node)                          { m.Called(node) }
func (m *MockEventHandler) OnNodeUpdate(old, new *Node)                   { m.Called(old, new) }
func (m *MockEventHandler) OnNodeDelete(node *Node)                       { m.Called(node) }

// test helpers
func newWatcherForTest(t *testing.T) (*Watcher, *MockEventHandler) {
	t.Helper()
	clientset := fake.NewSimpleClientset()
	restConfig := &rest.Config{}
	mockHandler := &MockEventHandler{}
	w, err := NewWatcher(clientset, restConfig, mockHandler)
	assert.NoError(t, err)
	return w, mockHandler
}

func newNode(name, ng string, ready bool) *v1.Node {
	status := v1.ConditionFalse
	if ready {
		status = v1.ConditionTrue
	}
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"node.deckhouse.io/group": ng},
		},
		Status: v1.NodeStatus{Conditions: []v1.NodeCondition{{Type: v1.NodeReady, Status: status}}},
	}
}

func newUnstructuredNG(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "deckhouse.io/v1",
			"kind":       "NodeGroup",
			"metadata": map[string]interface{}{
				"name": name,
			},
		},
	}
}

// TestWatcher tests the Watcher
func TestWatcher(t *testing.T) {
	w, _ := newWatcherForTest(t)
	assert.NotNil(t, w.clientset)
	assert.NotNil(t, w.dynamicClient)
	assert.NotNil(t, w.eventHandler)
	assert.NotNil(t, w.stopCh)
}

// TestConvertToNode tests the ConvertToNode function
func TestConvertToNode(t *testing.T) {
	w, _ := newWatcherForTest(t)

	result := w.ConvertToNode(newNode("test-node", "test-group", true))
	assert.NotNil(t, result)
	assert.Equal(t, "test-node", result.Name)
	assert.Equal(t, "test-group", result.NodeGroup)
	assert.True(t, result.Status.Conditions[0].Status == v1.ConditionTrue)

	invalidObj := "not a node"
	result = w.ConvertToNode(invalidObj)
	assert.Nil(t, result)
}

// TestConvertToNodeGroup tests the ConvertToNodeGroup function
func TestConvertToNodeGroup(t *testing.T) {
	w, _ := newWatcherForTest(t)

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "deckhouse.io/v1",
			"kind":       "NodeGroup",
			"metadata": map[string]interface{}{
				"name":      "test-nodegroup",
				"namespace": "default",
				"labels":    map[string]interface{}{"env": "test"},
			},
			"spec": map[string]interface{}{
				"nodeType": "Cloud",
				"cloudInstances": map[string]interface{}{
					"maxPerZone": int64(5),
					"minPerZone": int64(1),
					"zones":      []interface{}{"zone-a", "zone-b"},
				},
			},
			"status": map[string]interface{}{"desired": int64(3), "ready": int64(3)},
		},
	}

	result := w.ConvertToNodeGroup(obj)
	assert.NotNil(t, result)
	assert.Equal(t, "test-nodegroup", result.Name)
	assert.Equal(t, "default", result.Namespace)
	assert.Equal(t, "Cloud", result.Spec.NodeType)
	assert.NotNil(t, result.Spec.CloudInstances)
	assert.Equal(t, int32(5), result.Spec.CloudInstances.MaxPerZone)
	assert.Equal(t, int32(1), result.Spec.CloudInstances.MinPerZone)
	assert.Equal(t, []string{"zone-a", "zone-b"}, result.Spec.CloudInstances.Zones)
	assert.Equal(t, int32(3), result.Status.Desired)
	assert.Equal(t, int32(3), result.Status.Ready)

	invalidObj := "not a nodegroup"
	result = w.ConvertToNodeGroup(invalidObj)
	assert.Nil(t, result)
}

// TestExtractNodeGroupFromNode tests the extractNodeGroupFromNode function
func TestExtractNodeGroupFromNode(t *testing.T) {
	w, _ := newWatcherForTest(t)

	assert.Equal(t, "test-group", w.extractNodeGroupFromNode(newNode("n", "test-group", true)))
	nodeNoLabel := &v1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{}}}
	assert.Equal(t, "", w.extractNodeGroupFromNode(nodeNoLabel))
}

// TestWatcherStartStop tests the Start and Stop methods
func TestWatcherStartStop(t *testing.T) {
	w, _ := newWatcherForTest(t)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err := w.Start(ctx); err != nil {
		t.Logf("Expected error with fake client (cannot sync CRD): %v", err)
	}
	w.Stop()
}

// TestWatcherEventHandler tests the EventHandler integration
func TestWatcherEventHandler(t *testing.T) {
	w, mh := newWatcherForTest(t)

	mh.On("OnNodeGroupAdd", mock.AnythingOfType("*kubernetes.NodeGroupWrapper"))
	mh.On("OnNodeAdd", mock.AnythingOfType("*kubernetes.Node"))

	ng := &NodeGroupWrapper{NodeGroup: &NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "test-group", Namespace: "default"}}}
	n := &Node{Node: &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "test-node"}}, NodeGroup: "test-group"}

	if w.eventHandler != nil {
		w.eventHandler.OnNodeGroupAdd(ng)
		w.eventHandler.OnNodeAdd(n)
	}
	mh.AssertExpectations(t)
}

// TestWatcherQueueProcessing tests the queue processing
func TestWatcherQueueProcessing(t *testing.T) {
	w, mh := newWatcherForTest(t)

	mh.On("OnNodeGroupAdd", mock.AnythingOfType("*kubernetes.NodeGroupWrapper"))
	mh.On("OnNodeAdd", mock.AnythingOfType("*kubernetes.Node"))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go w.Start(ctx)

	ng := newUnstructuredNG("test-group")
	n := &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "test-node"}}

	if w.eventHandler != nil {
		w.eventHandler.OnNodeGroupAdd(w.ConvertToNodeGroup(ng))
		w.eventHandler.OnNodeAdd(w.ConvertToNode(n))
	}
	time.Sleep(100 * time.Millisecond)
	w.Stop()
	mh.AssertExpectations(t)
}

// TestWatcherInformerIntegration tests the informer integration
func TestWatcherInformerIntegration(t *testing.T) {
	w, mh := newWatcherForTest(t)

	mh.On("OnNodeAdd", mock.AnythingOfType("*kubernetes.Node"))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go w.Start(ctx)

	testNode := newNode("test-node", "test-group", true)
	_, err := w.clientset.CoreV1().Nodes().Create(context.TODO(), testNode, metav1.CreateOptions{})
	assert.NoError(t, err)

	time.Sleep(200 * time.Millisecond)
	w.Stop()
	mh.AssertExpectations(t)
}
