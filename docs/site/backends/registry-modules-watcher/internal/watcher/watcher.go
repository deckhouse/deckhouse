package watcher

import (
	"context"
	"time"

	v1 "k8s.io/api/coordination/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const leaseLabel = "deckhouse.io/documentation-builder-sync"
const resyncTimeout = time.Minute

// var (
// 	DefaultChanSize int32 = 100
// )

type watcher struct {
	// ch chan string
	// kClient       *kubernetes.Clientset
	dynamicClient *dynamic.DynamicClient
	namespace     string
}

func New(kClient *kubernetes.Clientset, dynamicClient *dynamic.DynamicClient, namespace string) *watcher {
	return &watcher{
		// kClient:       kClient,
		dynamicClient: dynamicClient,
		// ch:            make(chan string, DefaultChanSize),
		namespace: namespace,
	}
}

// ActiveBackends read once on start service. Then use state in service.
// func (w *watcher) ActiveBackends() ([]string, error) {
// 	var activeBackends = []string{}

// 	leaseList, err := w.kClient.CoordinationV1().Leases(w.namespace).List(context.TODO(), metav1.ListOptions{
// 		LabelSelector: leaseLabel,
// 	})
// 	if err != nil {
// 		return nil, err
// 	}

// 	for _, lease := range leaseList.Items {
// 		if addr := lease.Spec.HolderIdentity; addr != nil {

// 			fmt.Println("addr: ", addr) //TODO
// 			activeBackends = append(activeBackends, *addr)
// 		}
// 	}

// 	return activeBackends, nil
// }

// func (w *watcher) Watch(ctx context.Context) (chan string, error) {
// 	// https://dev.to/davidsbond/go-creating-dynamic-kubernetes-informers-1npi
// 	// 040 basheble api server
// 	// deckhouse/modules/040-node-manager/images/bashible-apiserver/pkg/template/context.go
// 	events, err := w.kClient.CoordinationV1().Leases(w.namespace).Watch(ctx, metav1.ListOptions{
// 		LabelSelector: leaseLabel,
// 	})
// 	if err != nil {
// 		return nil, err
// 	}

// 	go w.listen(events)

// 	return w.ch, nil
// }

// func (w *watcher) listen(events watch.Interface) {
// 	for event := range events.ResultChan() {
// 		if event.Type == watch.Added {
// 			lease, ok := event.Object.(*v1.Lease)
// 			if !ok {
// 				log.Fatal("error: cast object to lease error")
// 			}

// 			if addr := lease.Spec.HolderIdentity; addr != nil {
// 				fmt.Println("addr: ", *addr) // TODO
// 				w.ch <- *addr
// 			}
// 		}
// 	}
// }

func (w *watcher) Watch(ctx context.Context, addHandler, deleteHandler func(backend string)) {
	tweakListOptions := func(options *metav1.ListOptions) {
		options.LabelSelector = leaseLabel
	}
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(w.dynamicClient, resyncTimeout, w.namespace, tweakListOptions)

	resource := schema.GroupVersionResource{Group: "coordination.k8s.io", Version: "v1", Resource: "lease"}
	informer := factory.ForResource(resource).Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			lease, ok := obj.(*v1.Lease)
			if !ok {
				klog.Error("cast object to lease error")
			}

			holderIdentity := lease.Spec.HolderIdentity
			if holderIdentity != nil {
				klog.Infof("add backend event. holderIdentity: %v", holderIdentity)
				addHandler(*holderIdentity)
			}

			klog.Error(`lease "holderIdentity" is empty`)
		},
		DeleteFunc: func(obj interface{}) {
			lease, ok := obj.(*v1.Lease)
			if !ok {
				klog.Error("cast object to lease error")
				return
			}

			holderIdentity := lease.Spec.HolderIdentity
			if holderIdentity != nil {
				klog.Infof("delete backend event. holderIdentity: %v", holderIdentity)
				deleteHandler(*holderIdentity)
			}

			klog.Error(`lease "holderIdentity" is empty`)
		},
	})

	go informer.Run(ctx.Done())

	// Wait for the first sync of the informer cache, should not take long
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		klog.Fatalf("unable to sync caches: %v", ctx.Err())
	}
}
