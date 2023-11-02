package wather

import (
	"context"
	"fmt"
	"log"
	"os"

	v1 "k8s.io/api/coordination/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const leaseLabel = "deckhouse.io/documentation-builder-sync"

var (
	DefaultChanSize int32 = 100
)

type watcher struct {
	ch        chan string
	kClient   *kubernetes.Clientset
	namespace string
}

func New(kClient *kubernetes.Clientset) *watcher {
	return &watcher{
		kClient:   kClient,
		ch:        make(chan string, DefaultChanSize),
		namespace: os.Getenv("POD_NAMESPACE"),
	}
}

// ActiveBackends read once on start service. Then use state in service.
func (w *watcher) ActiveBackends() ([]string, error) {
	var activeBackends = []string{}

	leaseList, err := w.kClient.CoordinationV1().Leases(w.namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: leaseLabel,
	})
	if err != nil {
		return nil, err
	}

	for _, lease := range leaseList.Items {
		if addr := lease.Spec.HolderIdentity; addr != nil {

			fmt.Println("addr: ", addr) //TODO
			activeBackends = append(activeBackends, *addr)
		}
	}

	return activeBackends, nil
}

func (w *watcher) Watch(ctx context.Context) (chan string, error) {
	// https://dev.to/davidsbond/go-creating-dynamic-kubernetes-informers-1npi
	// 040 basheble api server
	// deckhouse/modules/040-node-manager/images/bashible-apiserver/pkg/template/context.go
	events, err := w.kClient.CoordinationV1().Leases(w.namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector: leaseLabel,
	})
	if err != nil {
		return nil, err
	}

	go w.listen(events)

	return w.ch, nil
}

func (w *watcher) listen(events watch.Interface) {
	for event := range events.ResultChan() {
		if event.Type == watch.Added {
			lease, ok := event.Object.(*v1.Lease)
			if !ok {
				log.Fatal("error: cast object to lease error")
			}

			if addr := lease.Spec.HolderIdentity; addr != nil {
				fmt.Println("addr: ", *addr) // TODO
				w.ch <- *addr
			}
		}
	}
}
