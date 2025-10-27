package kubernetes

import (
	"context"
	"log"
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
	log.Println("Starting resource watchers...")

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

	return nil
}

// Stop stops all watchers
func (w *Watcher) Stop() {
	close(w.stopCh)
	w.wg.Wait()
	log.Println("All watchers stopped")
}

// ConvertToNodeGroup converts a runtime.Object to NodeGroup
func (w *Watcher) ConvertToNodeGroup(obj interface{}) *NodeGroupWrapper {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		log.Printf("Failed to convert obj to unstructured: %T", obj)
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

	return nodeGroup
}

// ConvertToNode converts a runtime.Object to Node
func (w *Watcher) ConvertToNode(obj interface{}) *Node {
	nodeObj, ok := obj.(*v1.Node)
	if !ok {
		log.Printf("Failed to convert obj to v1.Node: %T", obj)
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
