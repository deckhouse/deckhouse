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
	"fmt"
	"node-group-exporter/pkg/entity"
	"sync"

	"github.com/deckhouse/deckhouse/pkg/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	dynamicInformers "k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// EventHandler defines interface for handling resource events
type EventHandler interface {
	OnNodeGroupAddOrUpdate(nodegroup *entity.NodeGroupData)
	OnNodeGroupDelete(nodegroup *entity.NodeGroupData)
	OnNodeAddOrUpdate(node *entity.NodeData)
	OnNodeDelete(node *entity.NodeData)
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
	logger            *log.Logger
}

// NewWatcher creates a new resource watcher
func NewWatcher(clientset kubernetes.Interface, restConfig *rest.Config, eventHandler EventHandler, logger *log.Logger) (*Watcher, error) {
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
		logger:        logger,
	}

	// Create informers
	watcher.nodeInformer = nodeFactory.Core().V1().Nodes().Informer()
	watcher.nodeGroupInformer = dynamicFactory.ForResource(NodeGroupGVR).Informer()

	return watcher, nil
}

// Start begins watching for resource changes
func (w *Watcher) Start(ctx context.Context) error {
	w.logger.Debug("Starting resource watchers...")

	// Add event handlers to Node informer
	w.nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			node, err := ConvertToNode(obj)
			if err != nil {
				w.logger.Debug("Error convert Node", log.Err(err))
				return
			}
			w.eventHandler.OnNodeAddOrUpdate(node)
		},
		UpdateFunc: func(_, newObj any) {
			newNode, err := ConvertToNode(newObj)
			if err != nil {
				w.logger.Debug("Error convert Node", log.Err(err))
				return
			}
			w.eventHandler.OnNodeAddOrUpdate(newNode)
		},
		DeleteFunc: func(obj any) {
			node, err := ConvertToNode(obj)
			if err != nil {
				w.logger.Debug("Error convert Node", log.Err(err))
				return
			}
			w.eventHandler.OnNodeDelete(node)
		},
	})

	// Add event handlers to NodeGroup informer
	w.nodeGroupInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			nodeGroup, err := ConvertToNodeGroup(obj)
			if err != nil {
				w.logger.Debug("Error convert NodeGroup", log.Err(err))
				return
			}
			w.eventHandler.OnNodeGroupAddOrUpdate(nodeGroup)
		},
		UpdateFunc: func(_, newObj any) {
			newNodeGroup, err := ConvertToNodeGroup(newObj)
			if err != nil {
				w.logger.Debug("Error convert NodeGroup", log.Err(err))
				return
			}
			w.eventHandler.OnNodeGroupAddOrUpdate(newNodeGroup)
		},
		DeleteFunc: func(obj any) {
			nodeGroup, err := ConvertToNodeGroup(obj)
			if err != nil {
				w.logger.Debug("Error convert NodeGroup", log.Err(err))
				return
			}
			w.eventHandler.OnNodeGroupDelete(nodeGroup)
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
	w.logger.Debug("Waiting for informer caches to sync...")
	if !cache.WaitForCacheSync(ctx.Done(), w.nodeInformer.HasSynced, w.nodeGroupInformer.HasSynced) {
		return fmt.Errorf("failed to sync informer caches")
	}
	w.logger.Info("Informer caches synced successfully")

	return nil
}

// Stop stops all watchers
func (w *Watcher) Stop() {
	close(w.stopCh)
	w.wg.Wait()
	w.logger.Debug("All watchers stopped")
}

// ListNodeGroups lists all NodeGroups using the dynamic client
func (w *Watcher) ListNodeGroups(ctx context.Context) ([]*entity.NodeGroupData, error) {
	nodeGroupList, err := w.dynamicClient.Resource(NodeGroupGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]*entity.NodeGroupData, 0, len(nodeGroupList.Items))
	for _, item := range nodeGroupList.Items {
		nodeGroup, err := ConvertToNodeGroup(&item)
		if err != nil {
			w.logger.Error("Error Convert NodeGroup", log.Err(err))
			continue
		}
		result = append(result, nodeGroup)
	}

	return result, nil
}
func (w *Watcher) ListNodes(ctx context.Context) ([]*entity.NodeData, error) {
	nodeList, err := w.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]*entity.NodeData, 0, len(nodeList.Items))
	for _, item := range nodeList.Items {
		node, err := ConvertToNode(&item)
		if err != nil {
			w.logger.Error("Error Convert Node", log.Err(err))
			continue
		}
		result = append(result, node)
	}

	return result, nil
}
