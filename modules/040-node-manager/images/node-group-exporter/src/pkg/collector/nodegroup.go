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
	"log"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	k8s "node-group-exporter/pkg/kubernetes"
)

// NodeGroupCollector implements prometheus.Collector interface
type NodeGroupCollector struct {
	clientset    kubernetes.Interface
	watcher      *k8s.Watcher
	nodeGroups   map[string]*k8s.NodeGroupWrapper
	nodes        map[string]*k8s.Node
	nodesByGroup map[string][]*k8s.Node // Cached index: nodeGroup -> nodes
	mutex        sync.RWMutex

	// Metrics
	nodeGroupCountNodesTotal *prometheus.GaugeVec
	nodeGroupCountReadyTotal *prometheus.GaugeVec
	nodeGroupCountMaxTotal   *prometheus.GaugeVec
	nodeGroupNode            *prometheus.GaugeVec
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

	watcher, err := k8s.NewWatcher(clientset, restConfig, collector)
	if err != nil {
		return nil, err
	}
	collector.watcher = watcher

	return collector, nil
}

func (c *NodeGroupCollector) Start(ctx context.Context) error {
	log.Println("Starting NodeGroupCollector...")

	if err := c.watcher.Start(ctx); err != nil {
		return err
	}

	// Initial sync
	if err := c.syncResources(ctx); err != nil {
		log.Printf("Error during initial sync: %v", err)
	}

	return nil
}

func (c *NodeGroupCollector) Stop() {
	log.Println("Stopping NodeGroupCollector...")
	c.watcher.Stop()
}

func (c *NodeGroupCollector) Describe(ch chan<- *prometheus.Desc) {
	c.nodeGroupCountNodesTotal.Describe(ch)
	c.nodeGroupCountReadyTotal.Describe(ch)
	c.nodeGroupCountMaxTotal.Describe(ch)
	c.nodeGroupNode.Describe(ch)
}

// Collect implements prometheus.Collector
func (c *NodeGroupCollector) Collect(ch chan<- prometheus.Metric) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	c.nodeGroupCountNodesTotal.Collect(ch)
	c.nodeGroupCountReadyTotal.Collect(ch)
	c.nodeGroupCountMaxTotal.Collect(ch)
	c.nodeGroupNode.Collect(ch)
}

// syncResources performs initial sync of resources
func (c *NodeGroupCollector) syncResources(ctx context.Context) error {
	log.Println("Performing initial sync of resources...")

	// Sync nodes
	if err := c.syncNodes(ctx); err != nil {
		log.Printf("Error syncing nodes: %v", err)
	}

	// Sync node groups
	if err := c.syncNodeGroups(ctx); err != nil {
		log.Printf("Error syncing node groups: %v", err)
	}

	// Build index after initial sync
	c.rebuildNodesByGroup()

	// Update metrics
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

// updateMetrics updates all metrics based on current state
// Uses cached nodesByGroup index for O(M) complexity where M is number of NodeGroups
func (c *NodeGroupCollector) updateMetrics() {
	// Reset all metrics
	c.nodeGroupCountNodesTotal.Reset()
	c.nodeGroupCountReadyTotal.Reset()
	c.nodeGroupCountMaxTotal.Reset()
	c.nodeGroupNode.Reset()

	log.Printf("updateMetrics called: NodeGroups=%d, TotalNodes=%d, nodesByGroup=%d", len(c.nodeGroups), len(c.nodes), len(c.nodesByGroup))

	// Log available node groups
	for name := range c.nodeGroups {
		log.Printf("NodeGroup in cache: %s", name)
	}

	// Log nodes by group
	for groupName, nodes := range c.nodesByGroup {
		log.Printf("nodesByGroup[%s] = %d nodes", groupName, len(nodes))
	}

	for _, nodeGroup := range c.nodeGroups {
		nodeType := nodeGroup.Spec.NodeType
		nodes := c.nodesByGroup[nodeGroup.Name]

		log.Printf("Processing NodeGroup '%s': type=%s, nodes=%d", nodeGroup.Name, nodeType, len(nodes))

		// Count total and ready nodes in this node group
		totalNodes := len(nodes)
		readyNodes := 0

		for _, node := range nodes {
			// Check node readiness and set metric - 1 if Ready, 0 if NotReady
			isReady := c.isNodeReady(node.Node)
			var nodeStatus float64
			if isReady {
				readyNodes++
				nodeStatus = 1.0
			}
			c.nodeGroupNode.WithLabelValues(nodeGroup.Name, nodeType, node.Name).Set(nodeStatus)
		}

		// Set node_group_count_nodes_total
		c.nodeGroupCountNodesTotal.WithLabelValues(nodeGroup.Name, nodeType).Set(float64(totalNodes))

		// Set node_group_count_ready_total
		c.nodeGroupCountReadyTotal.WithLabelValues(nodeGroup.Name, nodeType).Set(float64(readyNodes))

		// Set node_group_count_max_total
		maxNodes := c.calculateMaxNodes(nodeGroup, totalNodes)
		c.nodeGroupCountMaxTotal.WithLabelValues(nodeGroup.Name, nodeType).Set(float64(maxNodes))

		log.Printf("Metrics set for '%s': total=%d, ready=%d, max=%d", nodeGroup.Name, totalNodes, readyNodes, maxNodes)
	}

	if len(c.nodeGroups) == 0 {
		log.Printf("WARNING: updateMetrics called but no NodeGroups in cache!")
	}
}

// calculateMaxNodes calculates maximum number of nodes for a NodeGroup
// Uses totalNodes parameter to avoid recounting for Static groups
func (c *NodeGroupCollector) calculateMaxNodes(nodeGroup *k8s.NodeGroupWrapper, totalNodes int) int {
	// For Static node groups, max equals current node count
	if nodeGroup.Spec.NodeType == "Static" {
		return totalNodes
	}

	// For Cloud node groups, extract maxPerZone from spec.cloudInstances.maxPerZone
	if nodeGroup.Spec.CloudInstances != nil {
		maxPerZone := nodeGroup.Spec.CloudInstances.MaxPerZone
		zones := nodeGroup.Spec.CloudInstances.Zones
		if len(zones) > 0 {
			return int(maxPerZone) * len(zones)
		}
		// If no zones specified, assume 1 zone
		return int(maxPerZone)
	}

	// No cloud instances defined - return 0
	return 0
}

func (c *NodeGroupCollector) isNodeReady(node *v1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == v1.NodeReady {
			return condition.Status == v1.ConditionTrue
		}
	}
	return false
}

// EventHandler implementation

func (c *NodeGroupCollector) OnNodeGroupAdd(nodegroup *k8s.NodeGroupWrapper) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.nodeGroups[nodegroup.Name] = nodegroup
	log.Printf("Added NodeGroup: %s (type: %s), total nodegroups: %d", nodegroup.Name, nodegroup.Spec.NodeType, len(c.nodeGroups))
	c.updateMetrics()
}

func (c *NodeGroupCollector) OnNodeGroupUpdate(_, new *k8s.NodeGroupWrapper) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.nodeGroups[new.Name] = new
	c.updateMetrics()
	log.Printf("Updated NodeGroup: %s", new.Name)
}

func (c *NodeGroupCollector) OnNodeGroupDelete(nodegroup *k8s.NodeGroupWrapper) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.nodeGroups, nodegroup.Name)
	c.updateMetrics()
	log.Printf("Deleted NodeGroup: %s", nodegroup.Name)
}

func (c *NodeGroupCollector) OnNodeAdd(node *k8s.Node) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.nodes[node.Name] = node
	c.addNodeToIndex(node)
	log.Printf("Added Node: %s (NodeGroup: %s), total nodes: %d, nodeGroups: %d", node.Name, node.NodeGroup, len(c.nodes), len(c.nodeGroups))
	c.updateMetrics()
}

func (c *NodeGroupCollector) OnNodeUpdate(old, new *k8s.Node) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// If NodeGroup changed, need to update index
	if old != nil && old.NodeGroup != new.NodeGroup {
		c.removeNodeFromIndex(old)
		c.addNodeToIndex(new)
	}

	c.nodes[new.Name] = new
	c.updateMetrics()
	log.Printf("Updated Node: %s (NodeGroup: %s)", new.Name, new.NodeGroup)
}

func (c *NodeGroupCollector) OnNodeDelete(node *k8s.Node) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.nodes, node.Name)
	c.removeNodeFromIndex(node)
	c.updateMetrics()
	log.Printf("Deleted Node: %s (NodeGroup: %s)", node.Name, node.NodeGroup)
}
