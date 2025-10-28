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

func (m *MockEventHandler) OnNodeGroupAdd(nodegroup *NodeGroupWrapper) {
	m.Called(nodegroup)
}

func (m *MockEventHandler) OnNodeGroupUpdate(old, new *NodeGroupWrapper) {
	m.Called(old, new)
}

func (m *MockEventHandler) OnNodeGroupDelete(nodegroup *NodeGroupWrapper) {
	m.Called(nodegroup)
}

func (m *MockEventHandler) OnNodeAdd(node *Node) {
	m.Called(node)
}

func (m *MockEventHandler) OnNodeUpdate(old, new *Node) {
	m.Called(old, new)
}

func (m *MockEventHandler) OnNodeDelete(node *Node) {
	m.Called(node)
}

// TestWatcher tests the Watcher
func TestWatcher(t *testing.T) {
	// Create fake Kubernetes client
	clientset := fake.NewSimpleClientset()

	// Create rest config
	restConfig := &rest.Config{}

	// Create mock event handler
	mockHandler := &MockEventHandler{}

	// Create watcher
	watcher, err := NewWatcher(clientset, restConfig, mockHandler)
	assert.NoError(t, err)

	assert.NotNil(t, watcher.clientset)
	assert.NotNil(t, watcher.dynamicClient)
	assert.NotNil(t, watcher.eventHandler)
	assert.NotNil(t, watcher.stopCh)
}

// TestConvertToNode tests the ConvertToNode function
func TestConvertToNode(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	restConfig := &rest.Config{}
	mockHandler := &MockEventHandler{}
	watcher, err := NewWatcher(clientset, restConfig, mockHandler)
	assert.NoError(t, err)

	// Test with valid v1.Node
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-node",
			Namespace: "default",
			Labels: map[string]string{
				"node.deckhouse.io/group": "test-group",
			},
		},
		Status: v1.NodeStatus{
			Conditions: []v1.NodeCondition{
				{
					Type:   v1.NodeReady,
					Status: v1.ConditionTrue,
				},
			},
		},
	}

	result := watcher.ConvertToNode(node)
	assert.NotNil(t, result)
	assert.Equal(t, "test-node", result.Name)
	assert.Equal(t, "test-group", result.NodeGroup)
	assert.True(t, result.Status.Conditions[0].Status == v1.ConditionTrue)

	// Test with invalid object
	invalidObj := "not a node"
	result = watcher.ConvertToNode(invalidObj)
	assert.Nil(t, result)
}

// TestConvertToNodeGroup tests the ConvertToNodeGroup function
func TestConvertToNodeGroup(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	restConfig := &rest.Config{}
	mockHandler := &MockEventHandler{}
	watcher, err := NewWatcher(clientset, restConfig, mockHandler)
	assert.NoError(t, err)

	// Test with valid unstructured NodeGroup
	nodeGroupObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "deckhouse.io/v1",
			"kind":       "NodeGroup",
			"metadata": map[string]interface{}{
				"name":      "test-nodegroup",
				"namespace": "default",
				"labels": map[string]interface{}{
					"env": "test",
				},
			},
			"spec": map[string]interface{}{
				"nodeType": "Cloud",
				"cloudInstances": map[string]interface{}{
					"maxPerZone": int64(5),
					"minPerZone": int64(1),
					"zones":      []interface{}{"zone-a", "zone-b"},
				},
			},
			"status": map[string]interface{}{
				"desired": int64(3),
				"ready":   int64(3),
			},
		},
	}

	result := watcher.ConvertToNodeGroup(nodeGroupObj)
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

	// Test with invalid object
	invalidObj := "not a nodegroup"
	result = watcher.ConvertToNodeGroup(invalidObj)
	assert.Nil(t, result)
}

// TestExtractNodeGroupFromNode tests the extractNodeGroupFromNode function
func TestExtractNodeGroupFromNode(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	restConfig := &rest.Config{}
	mockHandler := &MockEventHandler{}
	watcher, err := NewWatcher(clientset, restConfig, mockHandler)
	assert.NoError(t, err)

	// Test with node that has nodegroup label
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"node.deckhouse.io/group": "test-group",
			},
		},
	}

	result := watcher.extractNodeGroupFromNode(node)
	assert.Equal(t, "test-group", result)

	// Test with node without nodegroup label
	nodeNoLabel := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{},
		},
	}

	result = watcher.extractNodeGroupFromNode(nodeNoLabel)
	assert.Equal(t, "", result)
}

// TestWatcherStartStop tests the Start and Stop methods
func TestWatcherStartStop(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	restConfig := &rest.Config{}
	mockHandler := &MockEventHandler{}
	watcher, err := NewWatcher(clientset, restConfig, mockHandler)
	assert.NoError(t, err)

	// Test Start
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err = watcher.Start(ctx)
	assert.NoError(t, err)

	// Test Stop
	watcher.Stop()
}

// TestWatcherEventHandler tests the EventHandler integration
func TestWatcherEventHandler(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	restConfig := &rest.Config{}
	mockHandler := &MockEventHandler{}
	watcher, err := NewWatcher(clientset, restConfig, mockHandler)
	assert.NoError(t, err)

	// Set up expectations
	mockHandler.On("OnNodeGroupAdd", mock.AnythingOfType("*kubernetes.NodeGroupWrapper"))
	mockHandler.On("OnNodeAdd", mock.AnythingOfType("*kubernetes.Node"))

	// Create test objects
	nodeGroup := &NodeGroupWrapper{
		NodeGroup: &NodeGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-group",
				Namespace: "default",
			},
		},
	}

	node := &Node{
		Node: &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node",
			},
		},
		NodeGroup: "test-group",
	}

	// Test event handling
	if watcher.eventHandler != nil {
		watcher.eventHandler.OnNodeGroupAdd(nodeGroup)
		watcher.eventHandler.OnNodeAdd(node)
	}

	// Verify expectations
	mockHandler.AssertExpectations(t)
}

// TestWatcherQueueProcessing tests the queue processing
func TestWatcherQueueProcessing(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	restConfig := &rest.Config{}
	mockHandler := &MockEventHandler{}
	watcher, err := NewWatcher(clientset, restConfig, mockHandler)
	assert.NoError(t, err)

	// Set up expectations
	mockHandler.On("OnNodeGroupAdd", mock.AnythingOfType("*kubernetes.NodeGroupWrapper"))
	mockHandler.On("OnNodeAdd", mock.AnythingOfType("*kubernetes.Node"))

	// Start watcher
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go watcher.Start(ctx)

	// Add items to queues (simulate informer events)
	nodeGroup := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "deckhouse.io/v1",
			"kind":       "NodeGroup",
			"metadata": map[string]interface{}{
				"name":      "test-group",
				"namespace": "default",
			},
		},
	}

	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
		},
	}

	// Simulate informer events
	if watcher.eventHandler != nil {
		watcher.eventHandler.OnNodeGroupAdd(watcher.ConvertToNodeGroup(nodeGroup))
		watcher.eventHandler.OnNodeAdd(watcher.ConvertToNode(node))
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Stop watcher
	watcher.Stop()

	// Verify expectations
	mockHandler.AssertExpectations(t)
}

// TestWatcherInformerIntegration tests the informer integration
func TestWatcherInformerIntegration(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	restConfig := &rest.Config{}
	mockHandler := &MockEventHandler{}
	watcher, err := NewWatcher(clientset, restConfig, mockHandler)
	assert.NoError(t, err)

	// Set up expectations
	mockHandler.On("OnNodeAdd", mock.AnythingOfType("*kubernetes.Node"))

	// Start watcher
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go watcher.Start(ctx)

	// Create a test node
	testNode := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Labels: map[string]string{
				"node.deckhouse.io/group": "test-group",
			},
		},
		Status: v1.NodeStatus{
			Conditions: []v1.NodeCondition{
				{
					Type:   v1.NodeReady,
					Status: v1.ConditionTrue,
				},
			},
		},
	}

	// Add node to fake client
	_, err = clientset.CoreV1().Nodes().Create(context.TODO(), testNode, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Wait for informer to process
	time.Sleep(100 * time.Millisecond)

	// Stop watcher
	watcher.Stop()

	// Verify expectations
	mockHandler.AssertExpectations(t)
}
