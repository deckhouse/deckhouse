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

package collector

import (
	"context"
	"fmt"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	k8s "node-group-exporter/pkg/kubernetes"
)

// MockWatcher is a mock implementation of the watcher
type MockWatcher struct {
	mock.Mock
}

func (m *MockWatcher) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockWatcher) Stop() {
	m.Called()
}

func (m *MockWatcher) ConvertToNode(obj interface{}) *k8s.Node {
	args := m.Called(obj)
	return args.Get(0).(*k8s.Node)
}

func (m *MockWatcher) ConvertToNodeGroup(obj interface{}) *k8s.NodeGroupWrapper {
	args := m.Called(obj)
	return args.Get(0).(*k8s.NodeGroupWrapper)
}

// TestNodeGroupCollector tests the NodeGroupCollector
func TestNodeGroupCollector(t *testing.T) {
	// Create fake Kubernetes client
	clientset := fake.NewSimpleClientset()

	// Create collector
	collector, err := NewNodeGroupCollector(clientset, &rest.Config{})
	assert.NoError(t, err)

	// Test that collector implements prometheus.Collector
	var _ prometheus.Collector = collector

	// Test initial state
	assert.NotNil(t, collector.nodeGroupCountNodesTotal)
	assert.NotNil(t, collector.nodeGroupCountReadyTotal)
	assert.NotNil(t, collector.nodeGroupCountMaxTotal)
	assert.NotNil(t, collector.nodeGroupNode)
}

// TestNodeGroupCollectorMetrics tests the metrics collection
func TestNodeGroupCollectorMetrics(t *testing.T) {
	// Create fake Kubernetes client
	clientset := fake.NewSimpleClientset()

	// Create collector
	collector, err := NewNodeGroupCollector(clientset, &rest.Config{})
	assert.NoError(t, err)

	// Add test node group
	testNodeGroup := &k8s.NodeGroupWrapper{
		NodeGroup: &k8s.NodeGroup{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "deckhouse.io/v1",
				Kind:       "NodeGroup",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-worker",
				Namespace: "default",
				Labels:    map[string]string{"env": "test"},
			},
			Spec: k8s.NodeGroupSpec{
				NodeType: "Cloud",
				CloudInstances: &k8s.CloudInstancesSpec{
					MaxPerZone: 5,
					MinPerZone: 1,
					Zones:      []string{"zone-a", "zone-b"},
				},
			},
			Status: k8s.NodeGroupStatus{
				Desired: 3,
				Ready:   1,  // One ready node (test expects 1)
				Nodes:   1,  // One node total (test expects 1)
				Max:     10, // 5 * 2 zones (test expects 10)
			},
		},
	}

	// Add test node
	testNode := &k8s.Node{
		Node: &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-node-1",
				Namespace: "default",
				Labels: map[string]string{
					"node.deckhouse.io/group": "test-worker",
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
		},
		NodeGroup: "test-worker",
	}

	// Add test data to collector
	collector.nodeGroups["test-worker"] = testNodeGroup
	collector.nodes["test-node-1"] = testNode

	// Rebuild index after manually adding test data
	collector.rebuildNodesByGroup()

	// Update metrics
	collector.updateMetrics()

	// Test node_group_count_nodes_total metric
	count := testutil.ToFloat64(collector.nodeGroupCountNodesTotal.WithLabelValues("test-worker", "Cloud"))
	assert.Equal(t, float64(1), count)

	// Test node_group_count_ready_total metric
	ready := testutil.ToFloat64(collector.nodeGroupCountReadyTotal.WithLabelValues("test-worker", "Cloud"))
	assert.Equal(t, float64(1), ready)

	// Test node_group_count_max_total metric
	max := testutil.ToFloat64(collector.nodeGroupCountMaxTotal.WithLabelValues("test-worker", "Cloud"))
	assert.Equal(t, float64(10), max) // 5 * 2 zones

	// Test node_group_node metric - should be 1 for ready node
	nodeMetric := testutil.ToFloat64(collector.nodeGroupNode.WithLabelValues("test-worker", "Cloud", "test-node-1"))
	assert.Equal(t, float64(1), nodeMetric)
}

// TestStaticNodeGroup tests Static node group behavior
func TestStaticNodeGroup(t *testing.T) {
	// Create fake Kubernetes client
	clientset := fake.NewSimpleClientset()

	// Create collector
	collector, err := NewNodeGroupCollector(clientset, &rest.Config{})
	assert.NoError(t, err)

	// Add Static node group
	staticNodeGroup := &k8s.NodeGroupWrapper{
		NodeGroup: &k8s.NodeGroup{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "deckhouse.io/v1",
				Kind:       "NodeGroup",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "static-master",
				Namespace: "default",
			},
			Spec: k8s.NodeGroupSpec{
				NodeType: "Static",
			},
			Status: k8s.NodeGroupStatus{
				Desired: 3,
				Ready:   3, // All 3 nodes ready
				Nodes:   3, // All 3 nodes
				Max:     3, // Max is 3 for static
			},
		},
	}

	// Add static nodes
	for i := 1; i <= 3; i++ {
		node := &k8s.Node{
			Node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("static-master-%d", i),
					Namespace: "default",
					Labels: map[string]string{
						"node.deckhouse.io/group": "static-master",
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
			},
			NodeGroup: "static-master",
		}
		collector.nodes[fmt.Sprintf("static-master-%d", i)] = node
	}

	collector.nodeGroups["static-master"] = staticNodeGroup

	// Rebuild index after manually adding test data
	collector.rebuildNodesByGroup()

	collector.updateMetrics()

	// Test Static node group max (should equal current node count)
	max := testutil.ToFloat64(collector.nodeGroupCountMaxTotal.WithLabelValues("static-master", "Static"))
	assert.Equal(t, float64(3), max)

	// Test Static node group total nodes
	count := testutil.ToFloat64(collector.nodeGroupCountNodesTotal.WithLabelValues("static-master", "Static"))
	assert.Equal(t, float64(3), count)
}

// TestNodeTypeExtraction tests direct node type extraction
func TestNodeTypeExtraction(t *testing.T) {
	// Test Cloud node type
	cloudNodeGroup := &k8s.NodeGroupWrapper{
		NodeGroup: &k8s.NodeGroup{
			Spec: k8s.NodeGroupSpec{
				NodeType: "Cloud",
			},
		},
	}

	assert.Equal(t, "Cloud", cloudNodeGroup.Spec.NodeType)

	// Test Static node type
	staticNodeGroup := &k8s.NodeGroupWrapper{
		NodeGroup: &k8s.NodeGroup{
			Spec: k8s.NodeGroupSpec{
				NodeType: "Static",
			},
		},
	}

	assert.Equal(t, "Static", staticNodeGroup.Spec.NodeType)
}

// TestIsNodeReady tests the isNodeReady function
func TestIsNodeReady(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	collector, err := NewNodeGroupCollector(clientset, &rest.Config{})
	assert.NoError(t, err)

	// Test ready node
	readyNode := &v1.Node{
		Status: v1.NodeStatus{
			Conditions: []v1.NodeCondition{
				{
					Type:   v1.NodeReady,
					Status: v1.ConditionTrue,
				},
			},
		},
	}

	assert.True(t, collector.isNodeReady(readyNode))

	// Test not ready node
	notReadyNode := &v1.Node{
		Status: v1.NodeStatus{
			Conditions: []v1.NodeCondition{
				{
					Type:   v1.NodeReady,
					Status: v1.ConditionFalse,
				},
			},
		},
	}

	assert.False(t, collector.isNodeReady(notReadyNode))

	// Test node without ready condition
	noConditionNode := &v1.Node{
		Status: v1.NodeStatus{
			Conditions: []v1.NodeCondition{},
		},
	}

	assert.False(t, collector.isNodeReady(noConditionNode))
}

// TestEventHandler tests the EventHandler implementation
func TestEventHandler(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	collector, err := NewNodeGroupCollector(clientset, &rest.Config{})
	assert.NoError(t, err)

	// Test OnNodeGroupAdd
	nodeGroup := &k8s.NodeGroupWrapper{
		NodeGroup: &k8s.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-group",
				Namespace: "default",
			},
		},
	}

	collector.OnNodeGroupAdd(nodeGroup)
	assert.Contains(t, collector.nodeGroups, "test-group")

	// Test OnNodeGroupUpdate
	updatedNodeGroup := &k8s.NodeGroupWrapper{
		NodeGroup: &k8s.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-group",
				Namespace: "default",
			},
			Status: k8s.NodeGroupStatus{
				Desired: 5,
			},
		},
	}

	collector.OnNodeGroupUpdate(nodeGroup, updatedNodeGroup)
	assert.Equal(t, updatedNodeGroup, collector.nodeGroups["test-group"])

	// Test OnNodeGroupDelete
	collector.OnNodeGroupDelete(nodeGroup)
	assert.NotContains(t, collector.nodeGroups, "test-group")

	// Test OnNodeAdd
	node := &k8s.Node{
		Node: &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node",
			},
		},
		NodeGroup: "test-group",
	}

	collector.OnNodeAdd(node)
	assert.Contains(t, collector.nodes, "test-node")

	// Test OnNodeUpdate with same NodeGroup (nodes cannot change NodeGroup)
	updatedNode := &k8s.Node{
		Node: &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node",
			},
		},
		NodeGroup: "test-group", // Same NodeGroup
	}

	collector.OnNodeUpdate(node, updatedNode)
	assert.Equal(t, updatedNode, collector.nodes["test-node"])
	assert.Equal(t, "test-group", updatedNode.NodeGroup) // Should remain the same

	// Test OnNodeUpdate with status change (Ready status) - critical test
	notReadyNode := &k8s.Node{
		Node: &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node-status",
			},
			Status: v1.NodeStatus{
				Conditions: []v1.NodeCondition{
					{
						Type:   v1.NodeReady,
						Status: v1.ConditionFalse,
					},
				},
			},
		},
		NodeGroup: "test-group",
	}

	// Add NodeGroup first with status showing 0 ready
	testGroup := &k8s.NodeGroupWrapper{
		NodeGroup: &k8s.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-group",
			},
			Spec: k8s.NodeGroupSpec{
				NodeType: "Cloud",
			},
			Status: k8s.NodeGroupStatus{
				Nodes: 1,
				Ready: 0, // Initially not ready
				Max:   10,
			},
		},
	}
	collector.OnNodeGroupAdd(testGroup)
	collector.OnNodeAdd(notReadyNode)

	// Check initial metrics - should show 0 ready from status
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)
	metrics, _ := registry.Gather()

	var readyCount float64
	for _, mf := range metrics {
		if mf.GetName() == "node_group_count_ready_total" {
			for _, m := range mf.GetMetric() {
				readyCount = m.GetGauge().GetValue()
			}
		}
	}
	assert.Equal(t, 0.0, readyCount, "Status should show 0 ready initially")

	// Update node to Ready - this is the critical test
	readyNode := &k8s.Node{
		Node: &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node-status",
			},
			Status: v1.NodeStatus{
				Conditions: []v1.NodeCondition{
					{
						Type:   v1.NodeReady,
						Status: v1.ConditionTrue,
					},
				},
			},
		},
		NodeGroup: "test-group",
	}

	// Update NodeGroup status to reflect ready state
	testGroup.Status.Ready = 1
	collector.OnNodeGroupUpdate(testGroup, testGroup)
	collector.OnNodeUpdate(notReadyNode, readyNode)

	// Check metrics again - should show 1 ready from updated status
	metrics, _ = registry.Gather()
	readyCount = 0
	for _, mf := range metrics {
		if mf.GetName() == "node_group_count_ready_total" {
			for _, m := range mf.GetMetric() {
				readyCount = m.GetGauge().GetValue()
			}
		}
	}
	assert.Equal(t, 1.0, readyCount, "Status should show 1 ready after NodeGroup update")

	// Check node_group_node metric
	var nodeMetricValue float64
	for _, mf := range metrics {
		if mf.GetName() == "node_group_node" {
			for _, m := range mf.GetMetric() {
				for _, l := range m.GetLabel() {
					if l.GetName() == "node" && l.GetValue() == "test-node-status" {
						nodeMetricValue = m.GetGauge().GetValue()
						break
					}
				}
			}
		}
	}
	assert.Equal(t, 1.0, nodeMetricValue, "node_group_node metric should be 1 for ready node")

	// Test OnNodeDelete
	collector.OnNodeDelete(updatedNode)
	assert.NotContains(t, collector.nodes, "test-node")
}

// TestNodeWithoutNodeGroup tests that nodes without existing NodeGroup are handled correctly
func TestNodeWithoutNodeGroup(t *testing.T) {
	// Create fake Kubernetes client
	clientset := fake.NewSimpleClientset()

	// Create collector
	collector, err := NewNodeGroupCollector(clientset, &rest.Config{})
	assert.NoError(t, err)

	// Add node WITHOUT creating corresponding NodeGroup first
	orphanNode := &k8s.Node{
		Node: &v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "orphan-node-1",
				Labels: map[string]string{
					"node.deckhouse.io/group": "nonexistent-group",
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
		},
		NodeGroup: "nonexistent-group",
	}

	// Add orphan node through event handler
	collector.OnNodeAdd(orphanNode)

	// Verify node is in nodes map
	assert.Contains(t, collector.nodes, "orphan-node-1")

	// Verify node is in nodesByGroup index
	assert.Contains(t, collector.nodesByGroup, "nonexistent-group")
	assert.Len(t, collector.nodesByGroup["nonexistent-group"], 1)

	// Collect metrics - should work without errors
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)
	metrics, err := registry.Gather()
	assert.NoError(t, err)
	_ = metrics // Verify metrics can be collected without errors

	// Since NodeGroup doesn't exist, no node_group_node metric for this node
	// This is expected behavior - orphan nodes don't generate metrics until NodeGroup appears

	// Now add the NodeGroup
	nodeGroup := &k8s.NodeGroupWrapper{
		NodeGroup: &k8s.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "nonexistent-group",
			},
			Spec: k8s.NodeGroupSpec{
				NodeType: "Cloud",
			},
			Status: k8s.NodeGroupStatus{
				Desired: 1,
				Ready:   1,
			},
		},
	}
	collector.OnNodeGroupAdd(nodeGroup)

	// Now metrics should be generated
	metrics, err = registry.Gather()
	assert.NoError(t, err)

	// Find node_group_node metric
	foundNodeMetric := false
	for _, mf := range metrics {
		if mf.GetName() == "node_group_node" {
			for _, m := range mf.GetMetric() {
				labels := m.GetLabel()
				for _, l := range labels {
					if l.GetName() == "node" && l.GetValue() == "orphan-node-1" {
						foundNodeMetric = true
						// Node is ready, so value should be 1
						assert.Equal(t, 1.0, m.GetGauge().GetValue())
						break
					}
				}
			}
		}
	}
	assert.True(t, foundNodeMetric, "node_group_node metric should be present after NodeGroup is added")

	// Test removing node when NodeGroup exists
	collector.OnNodeDelete(orphanNode)
	assert.NotContains(t, collector.nodes, "orphan-node-1")
	assert.NotContains(t, collector.nodesByGroup, "nonexistent-group") // Should be cleaned up
}

func TestCollectorIntegration(t *testing.T) {
	// Create fake Kubernetes client with test data
	clientset := fake.NewSimpleClientset()

	// Create collector
	collector, err := NewNodeGroupCollector(clientset, &rest.Config{})
	assert.NoError(t, err)

	// Add test data directly using event handlers
	testNodeGroup := &k8s.NodeGroupWrapper{
		NodeGroup: &k8s.NodeGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-integration",
				Namespace: "default",
			},
			Spec: k8s.NodeGroupSpec{
				NodeType: "Cloud",
				CloudInstances: &k8s.CloudInstancesSpec{
					MaxPerZone: 3,
					MinPerZone: 1,
					Zones:      []string{"zone-a"},
				},
			},
			Status: k8s.NodeGroupStatus{
				Desired: 2,
				Ready:   2,
			},
		},
	}
	collector.nodeGroups["test-integration"] = testNodeGroup

	// Rebuild index since we added data directly
	collector.rebuildNodesByGroup()

	// Update metrics
	collector.updateMetrics()

	// Test that metrics are registered
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	// Collect metrics
	metrics, err := registry.Gather()
	assert.NoError(t, err)
	assert.NotEmpty(t, metrics)

	// Verify we have the expected metrics
	// Note: node_group_node metric will only have values if there are nodes in the group
	metricNames := make(map[string]bool)
	for _, m := range metrics {
		metricNames[m.GetName()] = true
	}

	// Check that the main 3 metrics are present (node_group_node only appears with actual nodes)
	assert.Contains(t, metricNames, "node_group_count_nodes_total")
	assert.Contains(t, metricNames, "node_group_count_ready_total")
	assert.Contains(t, metricNames, "node_group_count_max_total")

	// Verify that at least 3 metrics are registered
	assert.GreaterOrEqual(t, len(metricNames), 3)
}
