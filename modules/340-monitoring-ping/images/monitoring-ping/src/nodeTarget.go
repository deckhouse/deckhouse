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
	"encoding/json"
	"fmt"
	"sync"

	"github.com/deckhouse/deckhouse/pkg/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type NodeTracker struct {
	sync.RWMutex
	nodes           []NodeTarget
	externalTargets []ExternalTarget
}

func NewNodeTracker() *NodeTracker {
	return &NodeTracker{
		nodes:           make([]NodeTarget, 0),
		externalTargets: make([]ExternalTarget, 0),
	}
}

func (nt *NodeTracker) Start(ctx context.Context, internalCM, externalCM, namespace string) error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	internalInformer := createInformer(internalCM, namespace, nt.onUpdate, true, clientset)
	externalInformer := createInformer(externalCM, namespace, nt.onUpdate, false, clientset)

	go internalInformer.Run(ctx.Done())
	go externalInformer.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), internalInformer.HasSynced, externalInformer.HasSynced) {
		return fmt.Errorf("failed to sync caches")
	}

	return nil
}

func (t *InternalTargets) Update(nt *NodeTracker) {
	nt.nodes = t.Cluster
	log.Info(fmt.Sprintf("Updated internal targets: %d nodes", len(t.Cluster)))
}

func (t *ExternalTargets) Update(nt *NodeTracker) {
	nt.externalTargets = t.Targets
	log.Info(fmt.Sprintf("Updated external targets: %d hosts", len(t.Targets)))
}

func (nt *NodeTracker) updateTargets(jsonData string, target Targets) {
	if err := json.Unmarshal([]byte(jsonData), target); err != nil {
		log.Error("Failed to unmarshal targets.json: %v", err)
		return
	}

	nt.Lock()
	defer nt.Unlock()
	target.Update(nt)
}

func (nt *NodeTracker) onUpdate(obj interface{}, internal bool) {
	cfgMap, ok := obj.(*v1.ConfigMap)
	if !ok {
		log.Warn("unexpected object type in onUpdate")
		return
	}

	jsonData, exists := cfgMap.Data["targets.json"]
	if !exists {
		log.Warn("targets.json not found in ConfigMap")
		return
	}

	if internal {
		nt.updateTargets(jsonData, &InternalTargets{})
	} else {
		nt.updateTargets(jsonData, &ExternalTargets{})
	}
}

func (nt *NodeTracker) ListClusterTargets() []NodeTarget {
	nt.RLock()
	defer nt.RUnlock()
	list := make([]NodeTarget, len(nt.nodes))
	copy(list, nt.nodes)
	return list
}

func (nt *NodeTracker) ListExternalTargets() []ExternalTarget {
	nt.RLock()
	defer nt.RUnlock()
	list := make([]ExternalTarget, len(nt.externalTargets))
	copy(list, nt.externalTargets)
	return list
}

func createInformer(cfgMapName, namespace string, handler func(interface{}, bool), internal bool, clientset *kubernetes.Clientset) cache.SharedInformer {
	lw := cache.NewFilteredListWatchFromClient(
		clientset.CoreV1().RESTClient(),
		"configmaps",
		namespace,
		func(options *metav1.ListOptions) {
			options.FieldSelector = fmt.Sprintf("metadata.name=%s", cfgMapName)
		},
	)

	informer := cache.NewSharedInformer(lw, &v1.ConfigMap{}, 0)
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { handler(obj, internal) },
		UpdateFunc: func(_, obj interface{}) { handler(obj, internal) },
		DeleteFunc: func(obj interface{}) {
			log.Warn("%s deleted — list will not be updated until recreated", cfgMapName)
		},
	})

	return informer
}
