// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"time"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	leaseLabel    = "deckhouse.io/documentation-builder-sync"
	namespace     = "d8-system"
	resyncTimeout = time.Minute
)

func NewModuleDocsSyncer() (*ModuleDocsSyncer, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("get cluster config: %w", err)
	}

	kClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("get k8s client: %w", err)
	}

	factory := informers.NewSharedInformerFactoryWithOptions(
		kClient,
		resyncTimeout,
		informers.WithNamespace(namespace),
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = leaseLabel
		}),
	)

	informer := factory.Coordination().V1().Leases().Informer()

	dClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("get dynamic client: %w", err)
	}

	s := &ModuleDocsSyncer{dClient, informer}
	return s, nil
}

type ModuleDocsSyncer struct {
	dClient  dynamic.Interface
	informer cache.SharedIndexInformer
}

func (s *ModuleDocsSyncer) Run(ctx context.Context) {
	log.Error(s.onLease(ctx)) // TODO: remove this line

	s.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			err := s.onLease(ctx)
			if err != nil {
				log.Error("on lease: ", err)
			}
		},
	})
	s.informer.Run(ctx.Done())
}

func (s *ModuleDocsSyncer) onLease(ctx context.Context) error {
	msGVR := schema.ParseGroupResource("modulesources.deckhouse.io").WithVersion("v1alpha1")
	list, err := s.dClient.Resource(msGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}

	for _, item := range list.Items {
		log.Printf("TMP: %+v\n", item)
		log.Println(unstructured.NestedString(item.UnstructuredContent(), ".spec.registry.repo"))
		log.Println(unstructured.NestedString(item.UnstructuredContent(), ".spec.registry.dockerCfg"))
		log.Println(unstructured.NestedString(item.UnstructuredContent(), ".spec.registry.ca"))
	}

	//opts := make([]cr.Option, 0)
	//if ex.Spec.Registry.DockerCFG != "" {
	//	opts = append(opts, cr.WithAuth(ex.Spec.Registry.DockerCFG))
	//} else {
	//	opts = append(opts, cr.WithDisabledAuth())
	//}
	//
	//if ex.Spec.Registry.CA != "" {
	//	opts = append(opts, cr.WithCA(ex.Spec.Registry.CA))
	//}

	//regCli, err := cr.NewClient(path.Join(moduleSource.Spec.Registry.Repo, moduleName))
	//if err != nil {
	//	return fmt.Errorf("fetch module error: %v", err)
	//}
	//
	//img, err := regCli.Image(moduleVersion)
	//if err != nil {
	//	return fmt.Errorf("fetch module version error: %v", err)
	//}

	return nil
}
