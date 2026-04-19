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

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	ngv1 "node-group-exporter/internal/v1"
	"node-group-exporter/pkg/entity"

	dto "github.com/prometheus/client_model/go"
)

func newTestNodeData(name, nodeGroup string, ready bool) *entity.NodeData {
	isReady := 0.0
	if ready {
		isReady = 1.0
	}
	return &entity.NodeData{
		Name:      name,
		NodeGroup: nodeGroup,
		IsReady:   isReady,
	}
}

func newTestNodeGroupData(name string, nodeType string, status ngv1.NodeGroupStatus) *entity.NodeGroupData {
	hasErrors := 0.0
	return &entity.NodeGroupData{
		Name:      name,
		NodeType:  nodeType,
		HasErrors: hasErrors,
		Nodes:     status.Nodes,
		Ready:     status.Ready,
		Max:       status.Max,
		Instances: status.Instances,
		Desired:   status.Desired,
		Min:       status.Min,
		UpToDate:  status.UpToDate,
		Standby:   status.Standby,
	}
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

// findNodeInGroups searches for a node by name in all groups and returns the node data and group name if found
func findNodeInGroups(collector *NodeGroupCollector, nodeName string) (*entity.NodeData, string) {
	for groupName, nodes := range collector.nodesByGroup {
		for _, node := range nodes {
			if node.Name == nodeName {
				return node, groupName
			}
		}
	}
	return nil, ""
}

// TestNodeGroupCollectorMetrics tests the metrics collection
func TestNodeGroupCollectorMetrics(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	collector, err := NewNodeGroupCollector(clientset, &rest.Config{}, log.Default())
	assert.NoError(t, err)

	testNodeGroupData := newTestNodeGroupData("test-worker", "Cloud", ngv1.NodeGroupStatus{
		Desired: 3,
		Ready:   1,
		Nodes:   2,
		Max:     10,
	})

	testNodeData := newTestNodeData("test-node-1", "test-worker", true)

	collector.OnNodeGroupAddOrUpdate(testNodeGroupData)
	collector.OnNodeAddOrUpdate(testNodeData)

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

	staticNodeGroupData := newTestNodeGroupData("static-master", "Static", ngv1.NodeGroupStatus{
		Ready: 3,
		Nodes: 5,
	})

	collector.OnNodeGroupAddOrUpdate(staticNodeGroupData)

	for i := 1; i <= 3; i++ {
		nodeData := newTestNodeData(fmt.Sprintf("static-master-%d", i), "static-master", true)
		collector.OnNodeAddOrUpdate(nodeData)
	}

	max := testutil.ToFloat64(collector.nodeGroupCountMaxTotal.WithLabelValues("static-master", "Static"))
	assert.Equal(t, float64(5), max)

	count := testutil.ToFloat64(collector.nodeGroupCountNodesTotal.WithLabelValues("static-master", "Static"))
	assert.Equal(t, float64(5), count)

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

// TestEventHandler tests the EventHandler implementation
func TestEventHandler(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	collector, err := NewNodeGroupCollector(clientset, &rest.Config{}, log.Default())
	assert.NoError(t, err)

	nodeGroupData := newTestNodeGroupData("test-group", "Cloud", ngv1.NodeGroupStatus{})
	collector.OnNodeGroupAddOrUpdate(nodeGroupData)
	assert.Contains(t, collector.nodeGroups, "test-group")

	updatedNodeGroupData := newTestNodeGroupData("test-group", "Cloud", ngv1.NodeGroupStatus{
		Desired: 5,
	})

	collector.OnNodeGroupAddOrUpdate(updatedNodeGroupData)
	assert.Contains(t, collector.nodeGroups, "test-group")
	storedNodeGroupData := collector.nodeGroups["test-group"]
	assert.Equal(t, "test-group", storedNodeGroupData.Name)
	assert.Equal(t, int32(5), storedNodeGroupData.Desired)

	collector.OnNodeGroupDelete(nodeGroupData)
	assert.NotContains(t, collector.nodeGroups, "test-group")

	nodeData := newTestNodeData("test-node", "", false)
	collector.OnNodeAddOrUpdate(nodeData)
	// Node without group should not be in nodesByGroup
	_, groupName := findNodeInGroups(collector, "test-node")
	assert.Empty(t, groupName, "Node without group should not be in nodesByGroup")

	updatedNodeData := newTestNodeData("test-node", "test-group", false)
	collector.OnNodeAddOrUpdate(updatedNodeData)
	var foundNodeData *entity.NodeData
	foundNodeData, groupName = findNodeInGroups(collector, "test-node")
	assert.NotNil(t, foundNodeData, "Node should be found in nodesByGroup")
	assert.Equal(t, "test-node", foundNodeData.Name)
	assert.Equal(t, "test-group", foundNodeData.NodeGroup)
	assert.Equal(t, "test-group", groupName)

	notReadyNodeData := newTestNodeData("test-node-status", "test-group", false)

	testGroupData := newTestNodeGroupData("test-group", "Cloud", ngv1.NodeGroupStatus{
		Nodes: 1,
		Ready: 0,
	})
	collector.OnNodeGroupAddOrUpdate(testGroupData)
	collector.OnNodeAddOrUpdate(notReadyNodeData)

	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)
	metrics, _ := registry.Gather()

	readyCount, found := getMetricValue(metrics, "node_group_count_ready_total")
	assert.True(t, found, "node_group_count_ready_total metric should be found")
	assert.Equal(t, 0.0, readyCount, "Status should show 0 ready initially")

	readyNodeData := newTestNodeData("test-node-status", "test-group", true)

	updatedTestGroupData := newTestNodeGroupData("test-group", "Cloud", ngv1.NodeGroupStatus{
		Nodes: 1,
		Ready: 1,
	})
	collector.OnNodeGroupAddOrUpdate(updatedTestGroupData)
	collector.OnNodeAddOrUpdate(readyNodeData)

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

	readyCount, found = getMetricValue(metrics, "node_group_count_ready_total")
	assert.True(t, found, "node_group_count_ready_total metric should be found")
	assert.Equal(t, 1.0, readyCount, "node_group_count_ready_total should equal one ready node")

	maxCount, found := getMetricValue(metrics, "node_group_count_max_total")
	assert.True(t, found, "node_group_count_max_total metric should be found")
	assert.Equal(t, 1.0, maxCount, "node_group_count_max_total should equal 1")

	collector.OnNodeDelete(updatedNodeData)
	deletedNode, _ := findNodeInGroups(collector, "test-node")
	assert.Nil(t, deletedNode, "Node should be removed from nodesByGroup")

	// Delete the ready node as well to test complete cleanup
	collector.OnNodeDelete(readyNodeData)
	deletedReadyNode, _ := findNodeInGroups(collector, "test-node-status")
	assert.Nil(t, deletedReadyNode, "Node should be removed from nodesByGroup")

	// Update NodeGroup status to reflect all nodes deleted
	finalTestGroupData := newTestNodeGroupData("test-group", "Cloud", ngv1.NodeGroupStatus{
		Nodes: 0,
		Ready: 0,
	})
	collector.OnNodeGroupAddOrUpdate(finalTestGroupData)

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

	orphanNodeData := newTestNodeData("orphan-node-1", "nonexistent-group", true)
	collector.OnNodeAddOrUpdate(orphanNodeData)

	nodeData, groupName := findNodeInGroups(collector, "orphan-node-1")
	assert.Equal(t, "nonexistent-group", nodeData.NodeGroup)
	assert.Equal(t, "nonexistent-group", groupName)

	assert.Contains(t, collector.nodesByGroup, "nonexistent-group")
	assert.Len(t, collector.nodesByGroup["nonexistent-group"], 1)

	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)
	_, err = registry.Gather()
	assert.NoError(t, err)

	nodeGroupData := newTestNodeGroupData("nonexistent-group", "Cloud", ngv1.NodeGroupStatus{
		Desired: 1,
		Ready:   1,
	})
	collector.OnNodeGroupAddOrUpdate(nodeGroupData)
	metrics, err := registry.Gather()

	assert.NoError(t, err)

	nodeMetricValue, foundNodeMetric := getMetricValueByLabel(metrics, "node_group_node", "node", "orphan-node-1")
	assert.True(t, foundNodeMetric, "node_group_node metric should be present after NodeGroup is added")
	assert.Equal(t, 1.0, nodeMetricValue, "node_group_node metric should be 1 for ready node")

	nodeMetricValueWithGroup, found := getMetricValueByLabels(metrics, "node_group_node", map[string]string{
		"node":       "orphan-node-1",
		"node_group": "nonexistent-group",
	})
	assert.True(t, found, "node_group_node metric should have correct node_group label")
	assert.Equal(t, 1.0, nodeMetricValueWithGroup, "node_group_node metric should be 1 for ready node with correct group")

	collector.OnNodeDelete(orphanNodeData)
	deletedNodeData, _ := findNodeInGroups(collector, "orphan-node-1")
	assert.Nil(t, deletedNodeData, "Node should be removed from nodesByGroup")
	assert.NotContains(t, collector.nodesByGroup, "nonexistent-group")

	collector.OnNodeGroupDelete(nodeGroupData)

	metrics, err = registry.Gather()
	assert.NoError(t, err)
	_, found = getMetricValue(metrics, "node_group_count_ready_total")
	assert.False(t, found, "node_group_count_ready_total metric should be found")

	_, found = getMetricValueByLabel(metrics, "node_group_node", "node", "orphan-node-1")
	assert.False(t, found, "node_group_node metric should not be present after node deletion")
}
