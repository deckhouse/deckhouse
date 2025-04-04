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
	"github.com/deckhouse/deckhouse/pkg/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"sync"
)

type NodeTracker struct {
	sync.RWMutex
	nodes []NodeTarget
}

func NewNodeTracker() *NodeTracker {
	return &NodeTracker{
		nodes: make([]NodeTarget, 0),
	}
}

func (nt *NodeTracker) Start(ctx context.Context, nameConfigMap, namespace string) error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	lw := cache.NewFilteredListWatchFromClient(
		clientset.CoreV1().RESTClient(),
		"configmaps",
		namespace,
		func(options *metav1.ListOptions) {
			options.FieldSelector = fmt.Sprintf("metadata.name=%s", nameConfigMap)
		},
	)

	informer := cache.NewSharedInformer(
		lw,
		&v1.ConfigMap{},
		0,
	)

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    nt.onUpdate,
		UpdateFunc: func(_, newObj interface{}) { nt.onUpdate(newObj) },
		DeleteFunc: func(obj interface{}) {
			log.Warn("monitoring-ping-config deleted — node list will not be updated until recreated")
		},
	})

	go informer.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return err
	}

	return nil
}

func (nt *NodeTracker) onUpdate(obj interface{}) {
	cfgMap, ok := obj.(*v1.ConfigMap)
	if !ok {
		return
	}

	jsonData, exists := cfgMap.Data["targets.json"]
	if !exists {
		return
	}

	type targets struct {
		Cluster []NodeTarget `json:"cluster_targets"`
	}

	var t targets
	if err := json.Unmarshal([]byte(jsonData), &t); err != nil {
		log.Error("Failed to unmarshal targets.json: %v", err)
		return
	}

	nt.Lock()
	defer nt.Unlock()
	nt.nodes = t.Cluster
}

func (nt *NodeTracker) List() []NodeTarget {
	nt.RLock()
	defer nt.RUnlock()
	return append([]NodeTarget(nil), nt.nodes...) // return a copy
}
