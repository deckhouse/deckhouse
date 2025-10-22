// Copyright 2021 Flant JSC
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

package client

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	crdinformer "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	infappsv1 "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/tools/cache"
)

type DeploymentInformer struct {
	KubeClient *KubernetesClient
	ctx        context.Context
	cancel     context.CancelFunc

	// Filter by namespace
	Namespace string
	// Filter by object name
	Name string
	// filter labels
	LabelSelector *metav1.LabelSelector
	// filter by fields
	FieldSelector string

	SharedInformer cache.SharedInformer

	ListOptions metav1.ListOptions

	EventCb func(obj *appsv1.Deployment, event string)
}

func NewDeploymentInformer(parentCtx context.Context, client *KubernetesClient) *DeploymentInformer {
	ctx, cancel := context.WithCancel(parentCtx)
	informer := &DeploymentInformer{
		KubeClient: client,
		ctx:        ctx,
		cancel:     cancel,
	}
	return informer
}

func (p *DeploymentInformer) WithKubeEventCb(eventCb func(obj *appsv1.Deployment, event string)) {
	p.EventCb = eventCb
}

func (p *DeploymentInformer) CreateSharedInformer() (err error) {
	// define resyncPeriod for informer
	resyncPeriod := time.Duration(2) * time.Hour

	// define indexers for informer
	indexers := cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}

	// define tweakListOptions for informer
	labelSelector, err := metav1.LabelSelectorAsSelector(p.LabelSelector)
	if err != nil {
		return err
	}

	tweakListOptions := func(options *metav1.ListOptions) {
		if p.FieldSelector != "" {
			options.FieldSelector = p.FieldSelector
		}
		if labelSelector.String() != "" {
			options.LabelSelector = labelSelector.String()
		}
	}
	// p.ListOptions = metav1.ListOptions{}
	// tweakListOptions(&p.ListOptions)

	// create informer with add, update, delete callbacks
	informer := infappsv1.NewFilteredDeploymentInformer(p.KubeClient, p.Namespace, resyncPeriod, indexers, tweakListOptions)
	informer.AddEventHandler(p)
	p.SharedInformer = informer

	return nil
}

func (p *DeploymentInformer) OnAdd(obj interface{}, _ bool) {
	p.HandleWatchEvent(obj, "Added")
}

func (p *DeploymentInformer) OnUpdate(oldObj, newObj interface{}) {
	p.HandleWatchEvent(newObj, "Modified")
}

func (p *DeploymentInformer) OnDelete(obj interface{}) {
	p.HandleWatchEvent(obj, "Deleted")
}

func (p *DeploymentInformer) HandleWatchEvent(object interface{}, eventType string) {
	if staleObj, stale := object.(cache.DeletedFinalStateUnknown); stale {
		object = staleObj.Obj
	}
	obj := object.(*appsv1.Deployment)

	p.EventCb(obj, eventType)
}

func (p *DeploymentInformer) Run() {
	p.SharedInformer.Run(p.ctx.Done())
}

func (p *DeploymentInformer) Stop() {
	p.cancel()
}

type CRDInformer struct {
	KubeClient *KubernetesClient
	ctx        context.Context
	cancel     context.CancelFunc

	SharedInformer cache.SharedInformer

	ListOptions metav1.ListOptions

	EventCb func(obj *apiextensionsv1beta1.CustomResourceDefinition, event string)
}

func NewCRDInformer(parentCtx context.Context, client *KubernetesClient) *CRDInformer {
	ctx, cancel := context.WithCancel(parentCtx)
	informer := &CRDInformer{
		KubeClient: client,
		ctx:        ctx,
		cancel:     cancel,
	}
	return informer
}

func (p *CRDInformer) WithKubeEventCb(eventCb func(obj *apiextensionsv1beta1.CustomResourceDefinition, event string)) {
	p.EventCb = eventCb
}

func (p *CRDInformer) CreateSharedInformer() (err error) {
	// define resyncPeriod for informer
	resyncPeriod := time.Duration(2) * time.Hour

	// define indexers for informer
	indexers := cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}

	// Make kubeClient working with crd subscription, now client is replaced with nil value
	informer := crdinformer.NewCustomResourceDefinitionInformer(nil, resyncPeriod, indexers)
	informer.AddEventHandler(p)
	p.SharedInformer = informer

	return nil
}

func (p *CRDInformer) OnAdd(obj interface{}, _ bool) {
	p.HandleWatchEvent(obj, "Added")
}

func (p *CRDInformer) OnUpdate(oldObj, newObj interface{}) {
	p.HandleWatchEvent(newObj, "Modified")
}

func (p *CRDInformer) OnDelete(obj interface{}) {
	p.HandleWatchEvent(obj, "Deleted")
}

func (p *CRDInformer) HandleWatchEvent(object interface{}, eventType string) {
	if staleObj, stale := object.(cache.DeletedFinalStateUnknown); stale {
		object = staleObj.Obj
	}
	obj := object.(*apiextensionsv1beta1.CustomResourceDefinition)

	p.EventCb(obj, eventType)
}

func (p *CRDInformer) Run() {
	p.SharedInformer.Run(p.ctx.Done())
}

func (p *CRDInformer) Stop() {
	p.cancel()
}
