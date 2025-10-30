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
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	k8s "node-group-exporter/pkg/kubernetes"
	"node-group-exporter/pkg/logger"
)

// NodeGroupCollector implements prometheus.Collector interface
type NodeGroupCollector struct {
	clientset    kubernetes.Interface
	watcher      *k8s.Watcher
	nodeGroups   map[string]*k8s.NodeGroupWrapper
	nodes        map[string]*k8s.Node
	nodesByGroup map[string][]*k8s.Node // Cached index: nodeGroup -> nodes
	mutex        sync.RWMutex

	// Metrics (existing)
	nodeGroupCountNodesTotal *prometheus.GaugeVec
	nodeGroupCountReadyTotal *prometheus.GaugeVec
	nodeGroupCountMaxTotal   *prometheus.GaugeVec
	nodeGroupNode            *prometheus.GaugeVec

	// Metrics (compatible with hook/node_group_metrics.go)
	d8NodeGroupReady     *prometheus.GaugeVec
	d8NodeGroupNodes     *prometheus.GaugeVec
	d8NodeGroupInstances *prometheus.GaugeVec
	d8NodeGroupDesired   *prometheus.GaugeVec
	d8NodeGroupMin       *prometheus.GaugeVec
	d8NodeGroupMax       *prometheus.GaugeVec
	d8NodeGroupUpToDate  *prometheus.GaugeVec
	d8NodeGroupStandby   *prometheus.GaugeVec
	d8NodeGroupHasErrors *prometheus.GaugeVec
}

func NewNodeGroupCollector(clientset kubernetes.Interface, restConfig *rest.Config) (*NodeGroupCollector, error) {
	collector := &NodeGroupCollector{
		clientset:    clientset,
		nodeGroups:   make(map[string]*k8s.NodeGroupWrapper),
		nodes:        make(map[string]*k8s.Node),
		nodesByGroup: make(map[string][]*k8s.Node),
	}

	// Initialize metrics
	collector.nodeGroupCountNodesTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_count_nodes_total",
			Help: "Total number of nodes in node group",
		},
		[]string{"node_group", "node_type"},
	)
	collector.nodeGroupCountReadyTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_count_ready_total",
			Help: "Number of ready nodes in node group",
		},
		[]string{"node_group", "node_type"},
	)
	collector.nodeGroupCountMaxTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_count_max_total",
			Help: "Maximum number of nodes in node group",
		},
		[]string{"node_group", "node_type"},
	)
	collector.nodeGroupNode = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_node",
			Help: "List of nodes in node group. Status Ready (1) NotReady(0)",
		},
		[]string{"node_group", "node_type", "node"},
	)

	// Initialize metrics compatible with hook/node_group_metrics.go
	collector.d8NodeGroupReady = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "d8_node_group_ready",
			Help: "Number of ready nodes in node group",
		},
		[]string{"node_group_name"},
	)

	collector.d8NodeGroupNodes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "d8_node_group_nodes",
			Help: "Number of Kubernetes nodes (in any state) in the group",
		},
		[]string{"node_group_name"},
	)

	collector.d8NodeGroupInstances = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "d8_node_group_instances",
			Help: "Number of instances (in any state) in the group",
		},
		[]string{"node_group_name"},
	)

	collector.d8NodeGroupDesired = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "d8_node_group_desired",
			Help: "Number of desired machines in the group",
		},
		[]string{"node_group_name"},
	)

	collector.d8NodeGroupMin = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "d8_node_group_min",
			Help: "Minimal amount of instances in the group",
		},
		[]string{"node_group_name"},
	)

	collector.d8NodeGroupMax = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "d8_node_group_max",
			Help: "Maximum amount of instances in the group",
		},
		[]string{"node_group_name"},
	)

	collector.d8NodeGroupUpToDate = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "d8_node_group_up_to_date",
			Help: "Number of up-to-date nodes in the group",
		},
		[]string{"node_group_name"},
	)

	collector.d8NodeGroupStandby = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "d8_node_group_standby",
			Help: "Number of overprovisioned instances in the group",
		},
		[]string{"node_group_name"},
	)

	collector.d8NodeGroupHasErrors = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "d8_node_group_has_errors",
			Help: "Whether the node group has errors (1 if error condition is True, 0 otherwise)",
		},
		[]string{"node_group_name"},
	)

	watcher, err := k8s.NewWatcher(clientset, restConfig, collector)
	if err != nil {
		return nil, err
	}
	collector.watcher = watcher

	return collector, nil
}

func (c *NodeGroupCollector) Start(ctx context.Context) error {
	logger.Info("Starting NodeGroupCollector...")

	if err := c.watcher.Start(ctx); err != nil {
		return err
	}

	if err := c.syncResources(ctx); err != nil {
		logger.Errorf("Error during initial sync: %v", err)
	}

	return nil
}

func (c *NodeGroupCollector) Stop() {
	logger.Info("Stopping NodeGroupCollector...")
	c.watcher.Stop()
}

func (c *NodeGroupCollector) Describe(ch chan<- *prometheus.Desc) {
	c.nodeGroupCountNodesTotal.Describe(ch)
	c.nodeGroupCountReadyTotal.Describe(ch)
	c.nodeGroupCountMaxTotal.Describe(ch)
	c.nodeGroupNode.Describe(ch)

	// Describe hook-compatible metrics
	c.d8NodeGroupReady.Describe(ch)
	c.d8NodeGroupNodes.Describe(ch)
	c.d8NodeGroupInstances.Describe(ch)
	c.d8NodeGroupDesired.Describe(ch)
	c.d8NodeGroupMin.Describe(ch)
	c.d8NodeGroupMax.Describe(ch)
	c.d8NodeGroupUpToDate.Describe(ch)
	c.d8NodeGroupStandby.Describe(ch)
	c.d8NodeGroupHasErrors.Describe(ch)
}

// Collect implements prometheus.Collector
func (c *NodeGroupCollector) Collect(ch chan<- prometheus.Metric) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	c.nodeGroupCountNodesTotal.Collect(ch)
	c.nodeGroupCountReadyTotal.Collect(ch)
	c.nodeGroupCountMaxTotal.Collect(ch)
	c.nodeGroupNode.Collect(ch)

	// Collect hook-compatible metrics
	c.d8NodeGroupReady.Collect(ch)
	c.d8NodeGroupNodes.Collect(ch)
	c.d8NodeGroupInstances.Collect(ch)
	c.d8NodeGroupDesired.Collect(ch)
	c.d8NodeGroupMin.Collect(ch)
	c.d8NodeGroupMax.Collect(ch)
	c.d8NodeGroupUpToDate.Collect(ch)
	c.d8NodeGroupStandby.Collect(ch)
	c.d8NodeGroupHasErrors.Collect(ch)
}

// syncResources performs initial sync of resources
func (c *NodeGroupCollector) syncResources(ctx context.Context) error {
	logger.Debug("Performing initial sync of resources...")

	if err := c.syncNodes(ctx); err != nil {
		logger.Errorf("Error syncing nodes: %v", err)
	}

	if err := c.syncNodeGroups(ctx); err != nil {
		logger.Errorf("Error syncing node groups: %v", err)
	}

	c.rebuildNodesByGroup()

	c.updateMetrics()

	return nil
}

func (c *NodeGroupCollector) syncNodes(ctx context.Context) error {
	nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, node := range nodes.Items {
		convertedNode := c.watcher.ConvertToNode(&node)
		if convertedNode != nil {
			c.nodes[node.Name] = convertedNode
		}
	}

	return nil
}

func (c *NodeGroupCollector) syncNodeGroups(ctx context.Context) error {
	nodeGroupList, err := c.watcher.ListNodeGroups(ctx)
	if err != nil {
		return err
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, nodeGroup := range nodeGroupList {
		c.nodeGroups[nodeGroup.Name] = nodeGroup
	}

	return nil
}

// rebuildNodesByGroup rebuilds the nodesByGroup index from scratch
// Should only be called during initial sync
func (c *NodeGroupCollector) rebuildNodesByGroup() {
	c.nodesByGroup = make(map[string][]*k8s.Node)
	for _, node := range c.nodes {
		if node.NodeGroup != "" {
			c.nodesByGroup[node.NodeGroup] = append(c.nodesByGroup[node.NodeGroup], node)
		}
	}
}

func (c *NodeGroupCollector) addNodeToIndex(node *k8s.Node) {
	if node.NodeGroup != "" {
		c.nodesByGroup[node.NodeGroup] = append(c.nodesByGroup[node.NodeGroup], node)
	}
}

func (c *NodeGroupCollector) removeNodeFromIndex(node *k8s.Node) {
	if node.NodeGroup == "" {
		return
	}

	nodes := c.nodesByGroup[node.NodeGroup]
	for i, n := range nodes {
		if n.Name == node.Name {
			c.nodesByGroup[node.NodeGroup] = append(nodes[:i], nodes[i+1:]...)
			break
		}
	}

	// Clean up empty slices
	if len(c.nodesByGroup[node.NodeGroup]) == 0 {
		delete(c.nodesByGroup, node.NodeGroup)
	}
}

func (c *NodeGroupCollector) ensureNodeInIndex(node *k8s.Node) {
	if node.NodeGroup == "" {
		return
	}

	nodes := c.nodesByGroup[node.NodeGroup]
	for i := range nodes {
		if nodes[i].Name == node.Name {
			// Update the reference to point to latest node
			c.nodesByGroup[node.NodeGroup][i] = node
			return
		}
	}

	// Node not in index, add it
	c.nodesByGroup[node.NodeGroup] = append(c.nodesByGroup[node.NodeGroup], node)
}

func (c *NodeGroupCollector) updateMetrics() {
	c.nodeGroupCountNodesTotal.Reset()
	c.nodeGroupCountReadyTotal.Reset()
	c.nodeGroupCountMaxTotal.Reset()
	c.nodeGroupNode.Reset()

	// Reset hook-compatible metrics
	c.d8NodeGroupReady.Reset()
	c.d8NodeGroupNodes.Reset()
	c.d8NodeGroupInstances.Reset()
	c.d8NodeGroupDesired.Reset()
	c.d8NodeGroupMin.Reset()
	c.d8NodeGroupMax.Reset()
	c.d8NodeGroupUpToDate.Reset()
	c.d8NodeGroupStandby.Reset()
	c.d8NodeGroupHasErrors.Reset()

	for _, nodeGroup := range c.nodeGroups {
		nodeType := nodeGroup.Spec.NodeType

		// Use values from NodeGroup status (primary source)
		totalNodes := int(nodeGroup.Status.Nodes)
		readyNodes := int(nodeGroup.Status.Ready)
		maxNodes := int(nodeGroup.Status.Max)

		if maxNodes == 0 {
			maxNodes = totalNodes
		}

		// Get nodes from index for node_group_node metric only
		indexedNodes := c.nodesByGroup[nodeGroup.Name]
		var nodeCount int

		// Set per-node metrics (only metric that requires node iteration)
		for _, indexedNode := range indexedNodes {
			if freshNode, exists := c.nodes[indexedNode.Name]; exists {
				nodeCount++
				nodeStatus := 0.0
				if c.isNodeReady(freshNode.Node) {
					nodeStatus = 1.0
				}
				c.nodeGroupNode.WithLabelValues(nodeGroup.Name, nodeType, freshNode.Name).Set(nodeStatus)
			}
		}

		// Fallback for totalNodes if status is not available
		if totalNodes == 0 && nodeCount > 0 {
			totalNodes = nodeCount
			logger.Warnf("NodeGroup '%s' status.nodes is 0, using index count %d", nodeGroup.Name, nodeCount)
		}

		// Set aggregated metrics from status (existing metrics)
		c.nodeGroupCountNodesTotal.WithLabelValues(nodeGroup.Name, nodeType).Set(float64(totalNodes))
		c.nodeGroupCountReadyTotal.WithLabelValues(nodeGroup.Name, nodeType).Set(float64(readyNodes))
		c.nodeGroupCountMaxTotal.WithLabelValues(nodeGroup.Name, nodeType).Set(float64(maxNodes))

		// Set hook-compatible metrics (same as in hook/node_group_metrics.go)
		c.d8NodeGroupReady.WithLabelValues(nodeGroup.Name).Set(float64(nodeGroup.Status.Ready))
		c.d8NodeGroupNodes.WithLabelValues(nodeGroup.Name).Set(float64(nodeGroup.Status.Nodes))
		c.d8NodeGroupInstances.WithLabelValues(nodeGroup.Name).Set(float64(nodeGroup.Status.Instances))
		c.d8NodeGroupDesired.WithLabelValues(nodeGroup.Name).Set(float64(nodeGroup.Status.Desired))
		c.d8NodeGroupMin.WithLabelValues(nodeGroup.Name).Set(float64(nodeGroup.Status.Min))
		c.d8NodeGroupMax.WithLabelValues(nodeGroup.Name).Set(float64(nodeGroup.Status.Max))
		c.d8NodeGroupUpToDate.WithLabelValues(nodeGroup.Name).Set(float64(nodeGroup.Status.UpToDate))
		c.d8NodeGroupStandby.WithLabelValues(nodeGroup.Name).Set(float64(nodeGroup.Status.Standby))

		// Check for errors in conditions (same logic as in hook)
		hasErrors := 0.0
		for _, condition := range nodeGroup.Status.Conditions {
			if condition.Type == "Error" && condition.Status == "True" {
				hasErrors = 1.0
				break
			}
		}
		c.d8NodeGroupHasErrors.WithLabelValues(nodeGroup.Name).Set(hasErrors)

		logger.Debugf("Metrics set for '%s': total=%d, ready=%d, max=%d, node_metrics=%d", nodeGroup.Name, totalNodes, readyNodes, maxNodes, nodeCount)
	}
}

func (c *NodeGroupCollector) isNodeReady(node *v1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == v1.NodeReady && condition.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

// EventHandler implementation

func (c *NodeGroupCollector) OnNodeGroupAdd(nodegroup *k8s.NodeGroupWrapper) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.nodeGroups[nodegroup.Name] = nodegroup
	logger.Debugf("Added NodeGroup: %s (type: %s), total nodegroups: %d", nodegroup.Name, nodegroup.Spec.NodeType, len(c.nodeGroups))
	c.updateMetrics()
}

func (c *NodeGroupCollector) OnNodeGroupUpdate(_, new *k8s.NodeGroupWrapper) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.nodeGroups[new.Name] = new
	c.updateMetrics()
	logger.Debugf("Updated NodeGroup: %s", new.Name)
}

func (c *NodeGroupCollector) OnNodeGroupDelete(nodegroup *k8s.NodeGroupWrapper) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.nodeGroups, nodegroup.Name)
	c.updateMetrics()
	logger.Debugf("Deleted NodeGroup: %s", nodegroup.Name)
}

func (c *NodeGroupCollector) OnNodeAdd(node *k8s.Node) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.nodes[node.Name] = node
	c.addNodeToIndex(node)
	logger.Debugf("Added Node: %s (NodeGroup: %s), total nodes: %d, nodeGroups: %d", node.Name, node.NodeGroup, len(c.nodes), len(c.nodeGroups))
	c.updateMetrics()
}

func (c *NodeGroupCollector) OnNodeUpdate(old, new *k8s.Node) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.nodes[new.Name] = new
	c.ensureNodeInIndex(new)
	c.updateMetrics()
	logger.Debug("Updated Node", zap.String("node", new.Name), zap.String("nodeGroup", new.NodeGroup), zap.Bool("ready", c.isNodeReady(new.Node)))
}

func (c *NodeGroupCollector) OnNodeDelete(node *k8s.Node) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.nodes, node.Name)
	c.removeNodeFromIndex(node)
	c.updateMetrics()
	logger.Debugf("Deleted Node: %s (NodeGroup: %s)", node.Name, node.NodeGroup)
}
