/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package handler

import (
	"context"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"time"
)

type Resource interface {
	Filter(obj interface{}) bool
	OnAdd(obj interface{}, isInInitialList bool)
	OnUpdate(oldObj, newObj interface{})
	OnDelete(obj interface{})
	GetGroupVersionResourse() schema.GroupVersionResource
}

type KubernetesResourcesHandler struct {
	namespace             string
	clientSet             *kubernetes.Clientset
	sharedInformerFactory informers.SharedInformerFactory

	resources []Resource

	ctx       context.Context
	ctxCancel context.CancelFunc
}

// NewKubernetesResourcesHandler создает новый KubernetesResourcesHandler
func NewKubernetesResourcesHandler(
	ctx context.Context,
	clientSet *kubernetes.Clientset,
	defaultResync time.Duration,
	namespace string,
) *KubernetesResourcesHandler {

	ctx, cancel := context.WithCancel(ctx)
	return &KubernetesResourcesHandler{
		sharedInformerFactory: informers.NewSharedInformerFactoryWithOptions(clientSet, defaultResync, informers.WithNamespace(namespace)),
		namespace:             namespace,
		clientSet:             clientSet,
		ctx:                   ctx,
		ctxCancel:             cancel,
	}
}

// Subscribe добавляет новую функцию обновления и фильтрации для обработки событий
func (k *KubernetesResourcesHandler) Subscribe(resource Resource) error {
	k.sharedInformerFactory.Admissionregistration()

	genericInformer, err := k.sharedInformerFactory.ForResource(resource.GetGroupVersionResourse())
	if err != nil {
		return err
	}

	informer := genericInformer.Informer()
	informer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: resource.Filter,
		Handler:    resource,
	})
	informer.SetWatchErrorHandler(cache.DefaultWatchErrorHandler)

	k.resources = append(k.resources, resource)
	return nil
}

// Start запускает информатор и тикер
func (k *KubernetesResourcesHandler) Start() {
	go k.sharedInformerFactory.Start(k.ctx.Done())
}

// Stop останавливает информатор и тикер
func (k *KubernetesResourcesHandler) Stop() {
	k.sharedInformerFactory.Shutdown()
	defer k.ctxCancel()
}
