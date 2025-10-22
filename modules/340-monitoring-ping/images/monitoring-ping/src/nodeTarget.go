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

func (nt *NodeTracker) Start(ctx context.Context, configMapName, namespace string) error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	informer := createInformer(configMapName, namespace, nt.onUpdate, clientset)
	go informer.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return fmt.Errorf("failed to sync cache")
	}

	return nil
}

func (nt *NodeTracker) updateInternal(jsonData string) {
	var t []NodeTarget
	if err := json.Unmarshal([]byte(jsonData), &t); err != nil {
		log.Error("Failed to unmarshal targets.json: %v", err)
		return
	}
	nt.Lock()
	nt.nodes = t
	nt.Unlock()
	log.Info(fmt.Sprintf("Updated internal targets: %d nodes", len(t)))
}

func (nt *NodeTracker) updateExternal(jsonData string) {
	var t []ExternalTarget
	if err := json.Unmarshal([]byte(jsonData), &t); err != nil {
		log.Error("Failed to unmarshal external_targets.json: %v", err)
		return
	}
	nt.Lock()
	nt.externalTargets = t
	nt.Unlock()
	log.Info(fmt.Sprintf("Updated external targets: %d hosts", len(t)))
}

func (nt *NodeTracker) onUpdate(obj interface{}) {
	cfgMap, ok := obj.(*v1.ConfigMap)
	if !ok {
		log.Warn("unexpected object type in onUpdate")
		return
	}

	if jsonData, exists := cfgMap.Data["targets.json"]; exists {
		nt.updateInternal(jsonData)
	} else {
		log.Warn("targets.json not found in ConfigMap")
	}

	if jsonData, exists := cfgMap.Data["external_targets.json"]; exists {
		nt.updateExternal(jsonData)
	} else {
		log.Warn("external_targets.json not found in ConfigMap")
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

func createInformer(cfgMapName, namespace string, handler func(interface{}), clientset *kubernetes.Clientset) cache.SharedInformer {
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
		AddFunc:    handler,
		UpdateFunc: func(_, obj interface{}) { handler(obj) },
		DeleteFunc: func(obj interface{}) {
			log.Warn("%s deleted — list will not be updated until recreated", cfgMapName)
		},
	})

	return informer
}
