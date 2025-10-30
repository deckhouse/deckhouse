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
	"fmt"
	"sync"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	dynamicInformers "k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"node-group-exporter/pkg/logger"
)

// NodeGroupWrapper wraps the NodeGroup for compatibility
type NodeGroupWrapper struct {
	*NodeGroup
}

// Node represents a Kubernetes Node with additional fields
type Node struct {
	*v1.Node
	NodeGroup string
}

// EventHandler defines interface for handling resource events
type EventHandler interface {
	OnNodeGroupAdd(nodegroup *NodeGroupWrapper)
	OnNodeGroupUpdate(old, new *NodeGroupWrapper)
	OnNodeGroupDelete(nodegroup *NodeGroupWrapper)
	OnNodeAdd(node *Node)
	OnNodeUpdate(old, new *Node)
	OnNodeDelete(node *Node)
}

// Watcher watches for changes in Node and NodeGroup resources
type Watcher struct {
	clientset         kubernetes.Interface
	dynamicClient     dynamic.Interface
	eventHandler      EventHandler
	nodeInformer      cache.SharedIndexInformer
	nodeGroupInformer cache.SharedIndexInformer
	stopCh            chan struct{}
	wg                sync.WaitGroup
}

// NewWatcher creates a new resource watcher
func NewWatcher(clientset kubernetes.Interface, restConfig *rest.Config, eventHandler EventHandler) (*Watcher, error) {
	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	// Create informer factories
	nodeFactory := informers.NewSharedInformerFactory(clientset, InformerResyncPeriod)
	dynamicFactory := dynamicInformers.NewDynamicSharedInformerFactory(dynamicClient, InformerResyncPeriod)

	watcher := &Watcher{
		clientset:     clientset,
		dynamicClient: dynamicClient,
		eventHandler:  eventHandler,
		stopCh:        make(chan struct{}),
	}

	// Create informers
	watcher.nodeInformer = nodeFactory.Core().V1().Nodes().Informer()
	watcher.nodeGroupInformer = dynamicFactory.ForResource(NodeGroupGVR).Informer()

	return watcher, nil
}

// Start begins watching for resource changes
func (w *Watcher) Start(ctx context.Context) error {
	logger.Debug("Starting resource watchers...")

	// Add event handlers to Node informer
	w.nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			node := w.ConvertToNode(obj)
			if node != nil {
				w.eventHandler.OnNodeAdd(node)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldNode := w.ConvertToNode(oldObj)
			newNode := w.ConvertToNode(newObj)
			if oldNode != nil && newNode != nil {
				w.eventHandler.OnNodeUpdate(oldNode, newNode)
			}
		},
		DeleteFunc: func(obj interface{}) {
			node := w.ConvertToNode(obj)
			if node != nil {
				w.eventHandler.OnNodeDelete(node)
			}
		},
	})

	// Add event handlers to NodeGroup informer
	w.nodeGroupInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			nodeGroup := w.ConvertToNodeGroup(obj)
			if nodeGroup != nil {
				w.eventHandler.OnNodeGroupAdd(nodeGroup)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldNodeGroup := w.ConvertToNodeGroup(oldObj)
			newNodeGroup := w.ConvertToNodeGroup(newObj)
			if oldNodeGroup != nil && newNodeGroup != nil {
				w.eventHandler.OnNodeGroupUpdate(oldNodeGroup, newNodeGroup)
			}
		},
		DeleteFunc: func(obj interface{}) {
			nodeGroup := w.ConvertToNodeGroup(obj)
			if nodeGroup != nil {
				w.eventHandler.OnNodeGroupDelete(nodeGroup)
			}
		},
	})

	// Start the informers
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.nodeInformer.Run(w.stopCh)
	}()

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.nodeGroupInformer.Run(w.stopCh)
	}()

	// Wait for cache sync before proceeding
	logger.Debug("Waiting for informer caches to sync...")
	if !cache.WaitForCacheSync(ctx.Done(), w.nodeInformer.HasSynced, w.nodeGroupInformer.HasSynced) {
		return fmt.Errorf("failed to sync informer caches")
	}
	logger.Info("Informer caches synced successfully")

	return nil
}

// Stop stops all watchers
func (w *Watcher) Stop() {
	close(w.stopCh)
	w.wg.Wait()
	logger.Debug("All watchers stopped")
}

// ConvertToNodeGroup converts a runtime.Object to NodeGroup
func (w *Watcher) ConvertToNodeGroup(obj interface{}) *NodeGroupWrapper {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		logger.Debugf("Failed to convert obj to unstructured: %T", obj)
		return nil
	}

	nodeGroup := convertUnstructuredToNodeGroup(unstructuredObj)
	if nodeGroup == nil {
		return nil
	}

	return &NodeGroupWrapper{
		NodeGroup: nodeGroup,
	}
}

// convertUnstructuredToNodeGroup converts unstructured data to NodeGroup
// This is a package-level function to avoid duplication
func convertUnstructuredToNodeGroup(obj *unstructured.Unstructured) *NodeGroup {
	nodeGroup := &NodeGroup{
		TypeMeta: metav1.TypeMeta{
			APIVersion: obj.GetAPIVersion(),
			Kind:       obj.GetKind(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        obj.GetName(),
			Namespace:   obj.GetNamespace(),
			Labels:      obj.GetLabels(),
			Annotations: obj.GetAnnotations(),
		},
	}

	// Extract spec.nodeType
	if nodeType, found, _ := unstructured.NestedString(obj.Object, "spec", "nodeType"); found {
		nodeGroup.Spec.NodeType = nodeType
	}

	// Extract spec.cloudInstances (check once and create if any field exists)
	hasCloudInstances := false
	var cloudInstances CloudInstancesSpec

	if maxPerZone, found, _ := unstructured.NestedInt64(obj.Object, "spec", "cloudInstances", "maxPerZone"); found {
		cloudInstances.MaxPerZone = int32(maxPerZone)
		hasCloudInstances = true
	}

	if minPerZone, found, _ := unstructured.NestedInt64(obj.Object, "spec", "cloudInstances", "minPerZone"); found {
		cloudInstances.MinPerZone = int32(minPerZone)
		hasCloudInstances = true
	}

	if zones, found, _ := unstructured.NestedStringSlice(obj.Object, "spec", "cloudInstances", "zones"); found {
		cloudInstances.Zones = zones
		hasCloudInstances = true
	}

	if hasCloudInstances {
		nodeGroup.Spec.CloudInstances = &cloudInstances
	}

	// Extract status.desired
	if desired, found, _ := unstructured.NestedInt64(obj.Object, "status", "desired"); found {
		nodeGroup.Status.Desired = int32(desired)
	}

	// Extract status.ready
	if ready, found, _ := unstructured.NestedInt64(obj.Object, "status", "ready"); found {
		nodeGroup.Status.Ready = int32(ready)
	}

	// Extract status.nodes
	if nodes, found, _ := unstructured.NestedInt64(obj.Object, "status", "nodes"); found {
		nodeGroup.Status.Nodes = int32(nodes)
	}

	// Extract status.instances
	if instances, found, _ := unstructured.NestedInt64(obj.Object, "status", "instances"); found {
		nodeGroup.Status.Instances = int32(instances)
	}

	// Extract status.min
	if min, found, _ := unstructured.NestedInt64(obj.Object, "status", "min"); found {
		nodeGroup.Status.Min = int32(min)
	}

	// Extract status.max
	if max, found, _ := unstructured.NestedInt64(obj.Object, "status", "max"); found {
		nodeGroup.Status.Max = int32(max)
	}

	// Extract status.upToDate
	if upToDate, found, _ := unstructured.NestedInt64(obj.Object, "status", "upToDate"); found {
		nodeGroup.Status.UpToDate = int32(upToDate)
	}

	// Extract status.standby
	if standby, found, _ := unstructured.NestedInt64(obj.Object, "status", "standby"); found {
		nodeGroup.Status.Standby = int32(standby)
	}

	// Extract status.conditions
	if conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions"); found {
		nodeGroup.Status.Conditions = make([]NodeGroupCondition, 0, len(conditions))
		for _, cond := range conditions {
			condMap, ok := cond.(map[string]interface{})
			if !ok {
				continue
			}
			condition := NodeGroupCondition{}
			if condType, found, _ := unstructured.NestedString(condMap, "type"); found {
				condition.Type = condType
			}
			if condStatus, found, _ := unstructured.NestedString(condMap, "status"); found {
				condition.Status = condStatus
			}
			if reason, found, _ := unstructured.NestedString(condMap, "reason"); found {
				condition.Reason = reason
			}
			if message, found, _ := unstructured.NestedString(condMap, "message"); found {
				condition.Message = message
			}
			nodeGroup.Status.Conditions = append(nodeGroup.Status.Conditions, condition)
		}
	}

	return nodeGroup
}

// ConvertToNode converts a runtime.Object to Node
func (w *Watcher) ConvertToNode(obj interface{}) *Node {
	nodeObj, ok := obj.(*v1.Node)
	if !ok {
		logger.Debugf("Failed to convert obj to v1.Node: %T", obj)
		return nil
	}

	return &Node{
		Node:      nodeObj,
		NodeGroup: w.extractNodeGroupFromNode(nodeObj),
	}
}

// ListNodeGroups lists all NodeGroups using the dynamic client
func (w *Watcher) ListNodeGroups(ctx context.Context) ([]*NodeGroupWrapper, error) {
	nodeGroupList, err := w.dynamicClient.Resource(NodeGroupGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]*NodeGroupWrapper, 0, len(nodeGroupList.Items))
	for _, item := range nodeGroupList.Items {
		nodeGroup := w.ConvertToNodeGroup(&item)
		if nodeGroup != nil {
			result = append(result, nodeGroup)
		}
	}

	return result, nil
}

// extractNodeGroupFromNode extracts node group name from node labels
func (w *Watcher) extractNodeGroupFromNode(node *v1.Node) string {
	if nodeGroup, exists := node.Labels[NodeGroupLabelKey]; exists {
		return nodeGroup
	}
	return ""
}
