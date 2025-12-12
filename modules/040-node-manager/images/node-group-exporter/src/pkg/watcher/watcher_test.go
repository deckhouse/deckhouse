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

package watcher

import (
	"context"
	"testing"
	"time"

	"node-group-exporter/pkg/entity"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicInformers "k8s.io/client-go/dynamic/dynamicinformer"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

// TestEventHandler collects received events for testing
type TestEventHandler struct {
	nodeGroups        map[string]*entity.NodeGroupData
	nodes             map[string]*entity.NodeData
	deletedNodeGroups map[string]*entity.NodeGroupData
	deletedNodes      map[string]*entity.NodeData
}

func newTestEventHandler() *TestEventHandler {
	return &TestEventHandler{
		nodeGroups:        make(map[string]*entity.NodeGroupData),
		nodes:             make(map[string]*entity.NodeData),
		deletedNodeGroups: make(map[string]*entity.NodeGroupData),
		deletedNodes:      make(map[string]*entity.NodeData),
	}
}

func (t *TestEventHandler) OnNodeGroupAddOrUpdate(nodegroup *entity.NodeGroupData) {
	t.nodeGroups[nodegroup.Name] = nodegroup
}

func (t *TestEventHandler) OnNodeGroupDelete(nodegroup *entity.NodeGroupData) {
	t.deletedNodeGroups[nodegroup.Name] = nodegroup
}

func (t *TestEventHandler) OnNodeAddOrUpdate(node *entity.NodeData) {
	t.nodes[node.Name] = node
}

func (t *TestEventHandler) OnNodeDelete(node *entity.NodeData) {
	t.deletedNodes[node.Name] = node
}

func newWatcherWithFakeClients(t *testing.T, nodes []runtime.Object, nodeGroups []runtime.Object) (*Watcher, *TestEventHandler, *k8sfake.Clientset, *dynamicfake.FakeDynamicClient) {
	t.Helper()
	clientset := k8sfake.NewSimpleClientset(nodes...)
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, nodeGroups...)
	testHandler := newTestEventHandler()

	watcher := &Watcher{
		clientset:     clientset,
		dynamicClient: dynamicClient,
		eventHandler:  testHandler,
		stopCh:        make(chan struct{}),
		logger:        log.Default(),
	}

	nodeFactory := informers.NewSharedInformerFactory(clientset, InformerResyncPeriod)
	dynamicFactory := dynamicInformers.NewDynamicSharedInformerFactory(dynamicClient, InformerResyncPeriod)
	watcher.nodeInformer = nodeFactory.Core().V1().Nodes().Informer()
	watcher.nodeGroupInformer = dynamicFactory.ForResource(NodeGroupGVR).Informer()

	return watcher, testHandler, clientset, dynamicClient
}

func newTestNodeGroupUnstructured(name, nodeType string, status map[string]any) *unstructured.Unstructured {
	obj := map[string]any{
		"apiVersion": "deckhouse.io/v1",
		"kind":       "NodeGroup",
		"metadata": map[string]any{
			"name": name,
		},
		"spec": map[string]any{
			"nodeType": nodeType,
		},
		"status": status,
	}
	return &unstructured.Unstructured{Object: obj}
}

func newTestNode(name, nodeGroup string, ready bool) *v1.Node {
	status := v1.ConditionFalse
	if ready {
		status = v1.ConditionTrue
	}

	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: v1.NodeStatus{
			Conditions: []v1.NodeCondition{
				{
					Type:   v1.NodeReady,
					Status: status,
				},
			},
		},
	}
	if nodeGroup != "" {
		node.Labels = map[string]string{"node.deckhouse.io/group": nodeGroup}
	}
	return node
}

// TestWatcherStartWithKubernetesObjects tests Watcher.Start() by creating objects in fake Kubernetes
func TestWatcherStartWithKubernetesObjects(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	testNode := newTestNode("test-node-1", "test-group", true)
	testNodeGroup := newTestNodeGroupUnstructured("test-group", "Cloud", map[string]any{
		"desired":   int64(5),
		"ready":     int64(3),
		"nodes":     int64(4),
		"instances": int64(6),
		"min":       int64(1),
		"max":       int64(10),
		"upToDate":  int64(2),
		"standby":   int64(1),
	})

	w, testHandler, clientset, dynamicClient := newWatcherWithFakeClients(t, []runtime.Object{testNode}, []runtime.Object{testNodeGroup})

	err := w.Start(ctx)
	assert.NoError(t, err)
	defer w.Stop()

	assert.Eventually(t, func() bool {
		return len(testHandler.nodes) >= 1 && len(testHandler.nodeGroups) >= 1
	}, 5*time.Second, 10*time.Millisecond, "Should have received initial sync events")

	foundNode, exists := testHandler.nodes["test-node-1"]
	assert.True(t, exists, "Should have found test-node-1")
	assert.Equal(t, "test-node-1", foundNode.Name)
	assert.Equal(t, "test-group", foundNode.NodeGroup)
	assert.Equal(t, float64(1), foundNode.IsReady)

	foundNodeGroup, exists := testHandler.nodeGroups["test-group"]
	assert.True(t, exists, "Should have found test-group")
	assert.Equal(t, "test-group", foundNodeGroup.Name)
	assert.Equal(t, "Cloud", foundNodeGroup.NodeType)
	assert.Equal(t, int32(5), foundNodeGroup.Desired)
	assert.Equal(t, int32(3), foundNodeGroup.Ready)
	assert.Equal(t, int32(4), foundNodeGroup.Nodes)
	assert.Equal(t, int32(6), foundNodeGroup.Instances)
	assert.Equal(t, int32(1), foundNodeGroup.Min)
	assert.Equal(t, int32(10), foundNodeGroup.Max)
	assert.Equal(t, int32(2), foundNodeGroup.UpToDate)
	assert.Equal(t, int32(1), foundNodeGroup.Standby)
	assert.Equal(t, float64(0), foundNodeGroup.HasErrors)

	t.Run("create new node after start", func(t *testing.T) {
		initialNodeCount := len(testHandler.nodes)

		newNode := newTestNode("test-node-2", "test-group", false)
		_, err := clientset.CoreV1().Nodes().Create(ctx, newNode, metav1.CreateOptions{})
		assert.NoError(t, err)

		assert.Eventually(t, func() bool {
			return len(testHandler.nodes) > initialNodeCount
		}, 5*time.Second, 10*time.Millisecond, "Should have received new node event")

		foundNewNode, exists := testHandler.nodes["test-node-2"]
		assert.True(t, exists, "Should have found test-node-2")
		assert.Equal(t, "test-node-2", foundNewNode.Name)
		assert.Equal(t, "test-group", foundNewNode.NodeGroup)
		assert.Equal(t, float64(0), foundNewNode.IsReady)
	})

	t.Run("create new nodegroup after start", func(t *testing.T) {
		initialNodeGroupCount := len(testHandler.nodeGroups)

		newNodeGroup := newTestNodeGroupUnstructured("test-group-2", "Static", map[string]any{
			"desired": int64(2),
			"ready":   int64(1),
			"nodes":   int64(1),
		})

		_, err := dynamicClient.Resource(NodeGroupGVR).Create(ctx, newNodeGroup, metav1.CreateOptions{})
		assert.NoError(t, err)

		assert.Eventually(t, func() bool {
			return len(testHandler.nodeGroups) > initialNodeGroupCount
		}, 5*time.Second, 10*time.Millisecond, "Should have received new nodegroup event")

		foundNewNodeGroup, exists := testHandler.nodeGroups["test-group-2"]
		assert.True(t, exists, "Should have found test-group-2")
		assert.Equal(t, "test-group-2", foundNewNodeGroup.Name)
		assert.Equal(t, "Static", foundNewNodeGroup.NodeType)
	})

	t.Run("nodegroup with error condition", func(t *testing.T) {
		errorNodeGroup := newTestNodeGroupUnstructured("test-group-error", "Static", map[string]any{
			"desired": int64(2),
			"ready":   int64(1),
			"nodes":   int64(1),
			"conditions": []any{
				map[string]any{
					"type":   "Error",
					"status": "True",
				},
			},
		})

		_, err := dynamicClient.Resource(NodeGroupGVR).Create(ctx, errorNodeGroup, metav1.CreateOptions{})
		assert.NoError(t, err)

		var foundErrorNodeGroup *entity.NodeGroupData
		assert.Eventually(t, func() bool {
			var exists bool
			foundErrorNodeGroup, exists = testHandler.nodeGroups["test-group-error"]
			return exists
		}, 5*time.Second, 10*time.Millisecond, "Should have found test-group-error")
		assert.Equal(t, "test-group-error", foundErrorNodeGroup.Name)
		assert.Equal(t, float64(1), foundErrorNodeGroup.HasErrors)
	})
}
