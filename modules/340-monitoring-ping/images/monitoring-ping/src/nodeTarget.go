// Package ping Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"github.com/deckhouse/deckhouse/pkg/log"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"sync"
)

type NodeTracker struct {
	sync.RWMutex
	nodes map[string]NodeTarget
}

func NewNodeTracker() *NodeTracker {
	return &NodeTracker{
		nodes: make(map[string]NodeTarget),
	}
}

func (nt *NodeTracker) Start(ctx context.Context) error {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Error("in-cluster config error: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Error("new clientset error: %w", err)
	}

	informerFactory := informers.NewSharedInformerFactory(clientset, 0)
	nodeInformer := informerFactory.Core().V1().Nodes().Informer()

	nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    nt.onAddOrUpdate,
		UpdateFunc: func(_, newObj interface{}) { nt.onAddOrUpdate(newObj) },
		DeleteFunc: nt.onDelete,
	})

	go informerFactory.Start(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), nodeInformer.HasSynced) {
		log.Error("failed to wait for cache sync")
	}

	return nil
}

func (nt *NodeTracker) onAddOrUpdate(obj interface{}) {
	node := obj.(*v1.Node)

	if node.Spec.Unschedulable {
		return
	}

	var internalIP string
	for _, addr := range node.Status.Addresses {
		if addr.Type == v1.NodeInternalIP {
			internalIP = addr.Address
			break
		}
	}

	if internalIP == "" {
		return
	}

	nt.Lock()
	defer nt.Unlock()
	nt.nodes[node.Name] = NodeTarget{Name: node.Name, IP: internalIP}
}

func (nt *NodeTracker) onDelete(obj interface{}) {
	node := obj.(*v1.Node)

	nt.Lock()
	defer nt.Unlock()
	delete(nt.nodes, node.Name)
}

func (nt *NodeTracker) List() []NodeTarget {
	nt.RLock()
	defer nt.RUnlock()

	result := make([]NodeTarget, 0, len(nt.nodes))
	for _, v := range nt.nodes {
		result = append(result, v)
	}
	return result
}
