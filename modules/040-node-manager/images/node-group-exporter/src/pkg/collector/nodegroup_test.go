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
	"fmt"
	"testing"

	"github.com/aws/smithy-go/ptr"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	ngv1 "node-group-exporter/internal/v1"

	dto "github.com/prometheus/client_model/go"
)

func newTestNode(name, nodeGroup string, ready bool) *v1.Node {
	status := v1.ConditionFalse
	if ready {
		status = v1.ConditionTrue
	}

	labels := make(map[string]string)
	if nodeGroup != "" {
		labels["node.deckhouse.io/group"] = nodeGroup
	}

	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels:    labels,
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
}

func newTestNodeGroup(name string, nodeType ngv1.NodeType, status ngv1.NodeGroupStatus) *ngv1.NodeGroup {
	nodeGroup := &ngv1.NodeGroup{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "deckhouse.io/v1",
			Kind:       "NodeGroup",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: ngv1.NodeGroupSpec{
			NodeType: nodeType,
		},
		Status: status,
	}

	// Add CloudInstances for Cloud node types
	if nodeType == "Cloud" {
		nodeGroup.Spec.CloudInstances = ngv1.CloudInstances{
			MaxPerZone: ptr.Int32(5),
			MinPerZone: ptr.Int32(1),
			Zones:      []string{"zone-a", "zone-b"},
		}
	}

	return nodeGroup
}

// labelsMatch checks if all labels in the map match the metric labels
func labelsMatch(metricLabels []*dto.LabelPair, expectedLabels map[string]string) bool {
	if len(expectedLabels) == 0 {
		return true
	}
	labelMap := make(map[string]string, len(metricLabels))
	for _, l := range metricLabels {
		labelMap[l.GetName()] = l.GetValue()
	}
	for name, value := range expectedLabels {
		if labelMap[name] != value {
			return false
		}
	}
	return true
}

// getMetricValueByLabels returns the value of a metric by its name and multiple label values from the collected metrics.
func getMetricValueByLabels(metrics []*dto.MetricFamily, metricName string, labels map[string]string) (float64, bool) {
	for _, mf := range metrics {
		if mf.GetName() == metricName {
			for _, m := range mf.GetMetric() {
				if labelsMatch(m.GetLabel(), labels) {
					return m.GetGauge().GetValue(), true
				}
			}
		}
	}
	return 0.0, false
}

// getMetricValue returns the value of a metric by its name from the collected metrics.
func getMetricValue(metrics []*dto.MetricFamily, metricName string) (float64, bool) {
	return getMetricValueByLabels(metrics, metricName, nil)
}

// getMetricValueByLabel returns the value of a metric by its name and label value from the collected metrics.
func getMetricValueByLabel(metrics []*dto.MetricFamily, metricName, labelName, labelValue string) (float64, bool) {
	return getMetricValueByLabels(metrics, metricName, map[string]string{labelName: labelValue})
}

// TestNodeGroupCollectorMetrics tests the metrics collection
func TestNodeGroupCollectorMetrics(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	collector, err := NewNodeGroupCollector(clientset, &rest.Config{}, log.Default())
	assert.NoError(t, err)

	testNodeGroup := newTestNodeGroup("test-worker", "Cloud", ngv1.NodeGroupStatus{
		Desired: 3,
		Ready:   1,
		Nodes:   2,
		Max:     10,
	})
	testNodeGroup.Labels = map[string]string{"env": "test"}

	testNode := newTestNode("test-node-1", "test-worker", true)

	collector.OnNodeGroupAddOrUpdate(testNodeGroup)
	collector.OnNodeAddOrUpdate(testNode)

	count := testutil.ToFloat64(collector.nodeGroupCountNodesTotal.WithLabelValues("test-worker", "Cloud"))
	assert.Equal(t, float64(2), count)

	ready := testutil.ToFloat64(collector.nodeGroupCountReadyTotal.WithLabelValues("test-worker", "Cloud"))
	assert.Equal(t, float64(1), ready)

	max := testutil.ToFloat64(collector.nodeGroupCountMaxTotal.WithLabelValues("test-worker", "Cloud"))
	assert.Equal(t, float64(10), max) // 5 * 2 zones

	nodeMetric := testutil.ToFloat64(collector.nodeGroupNode.WithLabelValues("test-worker", "Cloud", "test-node-1"))
	assert.Equal(t, float64(1), nodeMetric)
}

// TestStaticNodeGroup tests Static node group behavior
func TestStaticNodeGroup(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	collector, err := NewNodeGroupCollector(clientset, &rest.Config{}, log.Default())
	assert.NoError(t, err)

	staticNodeGroup := newTestNodeGroup("static-master", "Static", ngv1.NodeGroupStatus{
		Ready: 3,
		Nodes: 5,
	})

	collector.OnNodeGroupAddOrUpdate(staticNodeGroup)

	for i := 1; i <= 3; i++ {
		node := newTestNode(fmt.Sprintf("static-master-%d", i), "static-master", true)
		collector.OnNodeAddOrUpdate(node)
	}

	max := testutil.ToFloat64(collector.nodeGroupCountMaxTotal.WithLabelValues("static-master", "Static"))
	assert.Equal(t, float64(5), max)

	count := testutil.ToFloat64(collector.nodeGroupCountNodesTotal.WithLabelValues("static-master", "Static"))
	assert.Equal(t, float64(5), count)

	// Verify ready nodes metric equals 3
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)
	metrics, err := registry.Gather()
	assert.NoError(t, err)

	readyCount, found := getMetricValue(metrics, "node_group_count_ready_total")
	assert.True(t, found, "node_group_count_ready_total metric should be found")
	assert.Equal(t, 3.0, readyCount, "node_group_count_ready_total should equal 3 ready nodes")

	// Verify that node metrics contain correct node_group label
	for i := 1; i <= 3; i++ {
		nodeName := fmt.Sprintf("static-master-%d", i)
		nodeMetricValue, found := getMetricValueByLabels(metrics, "node_group_node", map[string]string{
			"node":       nodeName,
			"node_group": "static-master",
		})
		assert.True(t, found, "node_group_node metric should be found for %s with correct node_group label", nodeName)
		assert.Equal(t, 1.0, nodeMetricValue, "node_group_node metric should be 1 for ready node %s", nodeName)
	}
}

// TestIsNodeReady tests the isNodeReady function
func TestIsNodeReady(t *testing.T) {
	readyNode := newTestNode("ready-node", "", true)
	nodeData := ToNodeMetricsData(readyNode)
	assert.Equal(t, 1.0, nodeData.IsReady, "ready node should have IsReady=1.0")

	notReadyNode := newTestNode("not-ready-node", "", false)
	nodeData = ToNodeMetricsData(notReadyNode)
	assert.Equal(t, 0.0, nodeData.IsReady, "not ready node should have IsReady=0.0")

	noConditionNode := &v1.Node{
		Status: v1.NodeStatus{
			Conditions: []v1.NodeCondition{},
		},
	}

	nodeData = ToNodeMetricsData(noConditionNode)
	assert.Equal(t, 0.0, nodeData.IsReady, "node without condition should have IsReady=0.0")
}

// TestEventHandler tests the EventHandler implementation
func TestEventHandler(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	collector, err := NewNodeGroupCollector(clientset, &rest.Config{}, log.Default())
	assert.NoError(t, err)

	nodeGroup := newTestNodeGroup("test-group", "Cloud", ngv1.NodeGroupStatus{})

	collector.OnNodeGroupAddOrUpdate(nodeGroup)
	assert.Contains(t, collector.nodeGroups, "test-group")

	updatedNodeGroup := newTestNodeGroup("test-group", "Cloud", ngv1.NodeGroupStatus{
		Desired: 5,
	})

	collector.OnNodeGroupAddOrUpdate(updatedNodeGroup)
	// Verify that NodeGroup was updated
	assert.Contains(t, collector.nodeGroups, "test-group")
	nodeGroupData := collector.nodeGroups["test-group"]
	assert.Equal(t, "test-group", nodeGroupData.Name)
	assert.Equal(t, int32(5), nodeGroupData.Desired)

	collector.OnNodeGroupDelete(nodeGroup)
	assert.NotContains(t, collector.nodeGroups, "test-group")

	node := newTestNode("test-node", "", false)

	collector.OnNodeAddOrUpdate(node)
	assert.Contains(t, collector.nodes, "test-node")

	updatedNode := newTestNode("test-node", "test-group", false)

	collector.OnNodeAddOrUpdate(updatedNode)
	// Verify that Node was updated
	assert.Contains(t, collector.nodes, "test-node")
	nodeData := collector.nodes["test-node"]
	assert.Equal(t, "test-node", nodeData.Name)
	assert.Equal(t, "test-group", nodeData.NodeGroup) // Should remain the same

	notReadyNode := newTestNode("test-node-status", "test-group", false)

	testGroup := newTestNodeGroup("test-group", "Cloud", ngv1.NodeGroupStatus{
		Nodes: 1,
		Ready: 0,
	})
	collector.OnNodeGroupAddOrUpdate(testGroup)
	collector.OnNodeAddOrUpdate(notReadyNode)

	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)
	metrics, _ := registry.Gather()

	readyCount, found := getMetricValue(metrics, "node_group_count_ready_total")
	assert.True(t, found, "node_group_count_ready_total metric should be found")
	assert.Equal(t, 0.0, readyCount, "Status should show 0 ready initially")

	readyNode := newTestNode("test-node-status", "test-group", true)

	testGroup.Status.Ready = 1
	collector.OnNodeGroupAddOrUpdate(testGroup)
	collector.OnNodeAddOrUpdate(readyNode)

	metrics, _ = registry.Gather()
	readyCount, found = getMetricValue(metrics, "node_group_count_ready_total")
	assert.True(t, found, "node_group_count_ready_total metric should be found")
	assert.Equal(t, 1.0, readyCount, "Status should show 1 ready after NodeGroup update")

	nodeMetricValueWithGroup, found := getMetricValueByLabels(metrics, "node_group_node", map[string]string{
		"node":       "test-node-status",
		"node_group": "test-group",
	})
	assert.True(t, found, "node_group_node metric should have correct node_group label")
	assert.Equal(t, 1.0, nodeMetricValueWithGroup, "node_group_node metric should be 1 for ready node with correct group")

	// Verify that ready metric equals one node
	readyCount, found = getMetricValue(metrics, "node_group_count_ready_total")
	assert.True(t, found, "node_group_count_ready_total metric should be found")
	assert.Equal(t, 1.0, readyCount, "node_group_count_ready_total should equal one ready node")

	maxCount, found := getMetricValue(metrics, "node_group_count_max_total")
	assert.True(t, found, "node_group_count_max_total metric should be found")
	assert.Equal(t, 1.0, maxCount, "node_group_count_max_total should equal 1")

	collector.OnNodeDelete(updatedNode)
	assert.NotContains(t, collector.nodes, "test-node")

	// Delete the ready node as well to test complete cleanup
	collector.OnNodeDelete(readyNode)
	assert.NotContains(t, collector.nodes, "test-node-status")

	// Update NodeGroup status to reflect all nodes deleted
	testGroup.Status.Nodes = 0
	testGroup.Status.Ready = 0
	collector.OnNodeGroupAddOrUpdate(testGroup)

	// Verify that metrics are reset after node deletion
	metrics, _ = registry.Gather()
	readyCountAfterDelete, found := getMetricValue(metrics, "node_group_count_ready_total")
	assert.True(t, found, "node_group_count_ready_total metric should be found after deletion")
	assert.Equal(t, 0.0, readyCountAfterDelete, "node_group_count_ready_total should be 0 after all nodes deletion")

	_, found = getMetricValueByLabel(metrics, "node_group_node", "node", "test-node")
	assert.False(t, found, "node_group_node metric should not be present after node deletion")

	_, found = getMetricValueByLabel(metrics, "node_group_node", "node", "test-node-status")
	assert.False(t, found, "node_group_node metric should not be present after node deletion")

	maxCountAfterDelete, found := getMetricValue(metrics, "node_group_count_max_total")
	assert.True(t, found, "node_group_count_max_total metric should be found")
	assert.Equal(t, 0.0, maxCountAfterDelete, "node_group_count_max_total should equal 0")
}

// TestNodeWithoutNodeGroup tests that nodes without existing NodeGroup are handled correctly
func TestNodeWithoutNodeGroup(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	collector, err := NewNodeGroupCollector(clientset, &rest.Config{}, log.Default())
	assert.NoError(t, err)

	orphanNode := newTestNode("orphan-node-1", "nonexistent-group", true)

	collector.OnNodeAddOrUpdate(orphanNode)

	assert.Contains(t, collector.nodes, "orphan-node-1")

	assert.Contains(t, collector.nodesByGroup, "nonexistent-group")
	assert.Len(t, collector.nodesByGroup["nonexistent-group"], 1)

	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)
	_, err = registry.Gather()
	assert.NoError(t, err)

	nodeGroup := newTestNodeGroup("nonexistent-group", "Cloud", ngv1.NodeGroupStatus{
		Desired: 1,
		Ready:   1,
	})
	collector.OnNodeGroupAddOrUpdate(nodeGroup)
	metrics, err := registry.Gather()

	assert.NoError(t, err)

	nodeMetricValue, foundNodeMetric := getMetricValueByLabel(metrics, "node_group_node", "node", "orphan-node-1")
	assert.True(t, foundNodeMetric, "node_group_node metric should be present after NodeGroup is added")
	assert.Equal(t, 1.0, nodeMetricValue, "node_group_node metric should be 1 for ready node")

	// Verify that nodeMetricValue contains correct node_group label
	nodeMetricValueWithGroup, found := getMetricValueByLabels(metrics, "node_group_node", map[string]string{
		"node":       "orphan-node-1",
		"node_group": "nonexistent-group",
	})
	assert.True(t, found, "node_group_node metric should have correct node_group label")
	assert.Equal(t, 1.0, nodeMetricValueWithGroup, "node_group_node metric should be 1 for ready node with correct group")

	collector.OnNodeDelete(orphanNode)
	assert.NotContains(t, collector.nodes, "orphan-node-1")
	assert.NotContains(t, collector.nodesByGroup, "nonexistent-group") // Should be cleaned up

	// Update NodeGroup status to reflect node deletion
	nodeGroup.Status.Nodes = 0
	nodeGroup.Status.Ready = 0
	collector.OnNodeGroupAddOrUpdate(nodeGroup)

	// Verify that metrics are reset after node deletion
	metrics, err = registry.Gather()
	assert.NoError(t, err)
	readyCount, found := getMetricValue(metrics, "node_group_count_ready_total")
	assert.True(t, found, "node_group_count_ready_total metric should be found")
	assert.Equal(t, 0.0, readyCount, "node_group_count_ready_total should be 0 after node deletion")

	_, found = getMetricValueByLabel(metrics, "node_group_node", "node", "orphan-node-1")
	assert.False(t, found, "node_group_node metric should not be present after node deletion")
}
