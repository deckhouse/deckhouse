// k8s/client.go
package k8s

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"node-proxy-sidecar/internal/config"
	"node-proxy-sidecar/internal/haproxy"
)

type BackendUpdate struct {
	Backend config.Backend
	Servers []haproxy.Server
}

func (c *Client) ForceSync(backend config.Backend, updates chan<- BackendUpdate) error {
	log.Infof("ForceSync started for backend: %s ", backend.Name)

	namespace := backend.K8S.Namespace
	endpointName := backend.K8S.EndpointName

	ep, err := c.client.CoreV1().Endpoints(namespace).Get(context.TODO(), endpointName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		log.Errorf("Endpoint %s in namespace %s not found\n", endpointName, namespace)
		return err
	} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
		fmt.Printf("Error getting endpoint %s in namespace %s: %v\n", endpointName, namespace, statusError.ErrStatus.Message)
		return err
	} else if err != nil {
		log.Error(err)
		return err
	}
	c.processEndpoints(ep, backend, updates)
	return nil
}

func (c *Client) WatchEndpoints(backend config.Backend, updates chan<- BackendUpdate) error {
	log.Infof("WatchEndpoints started for backend: %s ", backend.Name)

	namespace := backend.K8S.Namespace
	endpointName := backend.K8S.EndpointName

	tweakListOpts := func(opts *metav1.ListOptions) {
		opts.FieldSelector = "metadata.name=" + endpointName
	}

	factory := informers.NewSharedInformerFactoryWithOptions(
		c.client,
		0,
		informers.WithNamespace(namespace),
		informers.WithTweakListOptions(tweakListOpts),
	)
	informer := factory.Core().V1().Endpoints().Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.processEndpoints(obj, backend, updates)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.processEndpoints(newObj, backend, updates)
		},
		DeleteFunc: func(obj interface{}) {
			updates <- BackendUpdate{backend, []haproxy.Server{}}
		},
	})

	stopCh := make(chan struct{})
	go informer.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, informer.HasSynced) {
		return fmt.Errorf("failed to sync informer cache")
	}

	return nil
}

func (c *Client) processEndpoints(obj interface{}, backend config.Backend, updates chan<- BackendUpdate) {
	servers := make([]haproxy.Server, 0)
	endpointName := backend.K8S.EndpointName
	portName := backend.K8S.PortName
	var port int32

	ep, ok := obj.(*v1.Endpoints)
	if !ok {
		log.Error("An invalid endpoint object was received")
		return
	}

	if ep.Name != endpointName {
		log.Infof("Enpoint with name %s was received,but we're expecting %s", ep.Name, endpointName)
		updates <- BackendUpdate{backend, servers}
		return
	}

	for _, subset := range ep.Subsets {
		for _, p := range subset.Ports {
			if p.Name == portName {
				port = p.Port
				break
			}
		}
	}

	if port == 0 {
		log.Errorf("Port with name %s not forund in endpoint %s", portName, ep.Name)
		updates <- BackendUpdate{backend, servers}
		return
	}

	for _, subset := range ep.Subsets {
		for _, a := range subset.Addresses {
			servers = append(servers, haproxy.Server{
				Address: a.IP,
				Port:    int64(port),
			})
		}
	}

	updates <- BackendUpdate{backend, servers}
}
