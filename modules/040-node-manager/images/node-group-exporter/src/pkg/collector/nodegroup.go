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
	"log/slog"
	"sync"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"node-group-exporter/pkg/entity"
	k8s "node-group-exporter/pkg/kubernetes"
)

// NodeGroupCollector implements prometheus.Collector interface
type NodeGroupCollector struct {
	clientset    kubernetes.Interface
	watcher      *k8s.Watcher
	nodeGroups   map[string]*entity.NodeGroupData
	nodesByGroup map[string][]*entity.NodeData
	mutex        sync.RWMutex
	logger       *log.Logger

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

func NewNodeGroupCollector(clientset kubernetes.Interface, restConfig *rest.Config, logger *log.Logger) (*NodeGroupCollector, error) {
	collector := &NodeGroupCollector{
		clientset:    clientset,
		nodeGroups:   make(map[string]*entity.NodeGroupData),
		nodesByGroup: make(map[string][]*entity.NodeData),
		logger:       logger,
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

	watcher, err := k8s.NewWatcher(clientset, restConfig, collector, logger)
	if err != nil {
		return nil, err
	}
	collector.watcher = watcher

	return collector, nil
}

func (c *NodeGroupCollector) Start(ctx context.Context) error {
	c.logger.Info("Starting NodeGroupCollector...")

	if err := c.watcher.Start(ctx); err != nil {
		return err
	}

	if err := c.syncResources(ctx); err != nil {
		c.logger.Error("Error during initial sync", log.Err(err))
		return err
	}

	return nil
}

func (c *NodeGroupCollector) Stop() {
	c.logger.Info("Stopping NodeGroupCollector...")
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
	c.logger.Debug("Performing initial sync of resources...")

	if err := c.syncNodes(ctx); err != nil {
		c.logger.Error("Error syncing nodes: ", log.Err(err))
		return err
	}

	if err := c.syncNodeGroups(ctx); err != nil {
		c.logger.Error("Error syncing node groups: ", log.Err(err))
		return err
	}

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

	c.nodesByGroup = make(map[string][]*entity.NodeData)
	for _, node := range nodes.Items {
		nodeData := k8s.ToNodeMetricsData(&node)
		if nodeData.NodeGroup != "" {
			c.nodesByGroup[nodeData.NodeGroup] = append(c.nodesByGroup[nodeData.NodeGroup], &nodeData)
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

func (c *NodeGroupCollector) removeNodeFromIndex(node *entity.NodeData) {
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

func (c *NodeGroupCollector) ensureNodeInIndex(node *entity.NodeData) bool {
	if node.NodeGroup == "" {
		return false
	}

	nodes := c.nodesByGroup[node.NodeGroup]
	for i := range nodes {
		if nodes[i].Name == node.Name {
			// Only Ready status can change
			if nodes[i].IsReady != node.IsReady {
				c.nodesByGroup[node.NodeGroup][i] = node
				return true
			}
			return false
		}
	}

	// Node not in index, add it
	c.nodesByGroup[node.NodeGroup] = append(c.nodesByGroup[node.NodeGroup], node)
	return true
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
		// Use values from NodeGroupData
		totalNodes := int(nodeGroup.Nodes)
		readyNodes := int(nodeGroup.Ready)
		maxNodes := int(nodeGroup.Max)

		// Get nodes from index for node_group_node metric only
		indexedNodes := c.nodesByGroup[nodeGroup.Name]

		// Set per-node metrics (only metric that requires node iteration)
		for _, indexedNode := range indexedNodes {
			c.nodeGroupNode.WithLabelValues(nodeGroup.Name, nodeGroup.NodeType, indexedNode.Name).Set(indexedNode.IsReady)
		}
		countNodes := len(indexedNodes)
		// Fallback for totalNodes if status is not available
		if totalNodes == 0 && countNodes > 0 {
			totalNodes = countNodes
			c.logger.Warn("NodeGroup status.nodes is 0, using index count",
				slog.String("NodeGroup", nodeGroup.Name),
				slog.Int("Count", countNodes))
		}

		if maxNodes == 0 {
			maxNodes = totalNodes
		}

		// Set aggregated metrics from status (existing metrics)
		c.nodeGroupCountNodesTotal.WithLabelValues(nodeGroup.Name, nodeGroup.NodeType).Set(float64(totalNodes))
		c.nodeGroupCountReadyTotal.WithLabelValues(nodeGroup.Name, nodeGroup.NodeType).Set(float64(readyNodes))
		c.nodeGroupCountMaxTotal.WithLabelValues(nodeGroup.Name, nodeGroup.NodeType).Set(float64(maxNodes))

		// Set hook-compatible metrics (same as in hook/node_group_metrics.go)
		c.d8NodeGroupReady.WithLabelValues(nodeGroup.Name).Set(float64(nodeGroup.Ready))
		c.d8NodeGroupNodes.WithLabelValues(nodeGroup.Name).Set(float64(nodeGroup.Nodes))
		c.d8NodeGroupInstances.WithLabelValues(nodeGroup.Name).Set(float64(nodeGroup.Instances))
		c.d8NodeGroupDesired.WithLabelValues(nodeGroup.Name).Set(float64(nodeGroup.Desired))
		c.d8NodeGroupMin.WithLabelValues(nodeGroup.Name).Set(float64(nodeGroup.Min))
		c.d8NodeGroupMax.WithLabelValues(nodeGroup.Name).Set(float64(nodeGroup.Max))
		c.d8NodeGroupUpToDate.WithLabelValues(nodeGroup.Name).Set(float64(nodeGroup.UpToDate))
		c.d8NodeGroupStandby.WithLabelValues(nodeGroup.Name).Set(float64(nodeGroup.Standby))
		c.d8NodeGroupHasErrors.WithLabelValues(nodeGroup.Name).Set(nodeGroup.HasErrors)

		c.logger.Debug("Metrics set for",
			slog.String("NodeGroup", nodeGroup.Name),
			slog.Int("TotalNodes", totalNodes),
			slog.Int("ReadyNodes", readyNodes),
			slog.Int("maxNodes", maxNodes),
			slog.Int("CountNodes", countNodes))
	}
}

// EventHandler implementation

func (c *NodeGroupCollector) OnNodeGroupAddOrUpdate(nodegroup *entity.NodeGroupData) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Update if NodeGroup is different
	if c.nodeGroups[nodegroup.Name] == nil || *c.nodeGroups[nodegroup.Name] != *nodegroup {
		c.nodeGroups[nodegroup.Name] = nodegroup
		c.updateMetrics()
	}
	c.logger.Debug("Add or Update NodeGroup",
		slog.String("NodeGroup", nodegroup.Name),
		slog.String("Type", nodegroup.NodeType),
		slog.Int("Nodes", len(c.nodeGroups)))
}

func (c *NodeGroupCollector) OnNodeGroupDelete(nodegroup *entity.NodeGroupData) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.nodeGroups, nodegroup.Name)
	c.updateMetrics()
	c.logger.Debug("Deleted NodeGroup",
		slog.String("NodeGroup", nodegroup.Name))
}

func (c *NodeGroupCollector) OnNodeAddOrUpdate(node *entity.NodeData) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	updated := c.ensureNodeInIndex(node)
	if updated {
		c.updateMetrics()
	}
	c.logger.Debug("Add or Updated Node",
		slog.String("Node", node.Name),
		slog.String("NodeGroup", node.NodeGroup),
		slog.Float64("Ready", node.IsReady),
		slog.Bool("Updated", updated))
}

func (c *NodeGroupCollector) OnNodeDelete(node *entity.NodeData) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.removeNodeFromIndex(node)
	c.updateMetrics()
	c.logger.Debug("Deleted Node",
		slog.String("Node", node.Name),
		slog.String("NodeGroup", node.NodeGroup))
}
