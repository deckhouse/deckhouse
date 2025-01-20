package k8s

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"node-proxy-sidecar/internal/config"
	"node-proxy-sidecar/internal/haproxy"
)

func (c *Client) WatchEndpoints(backend config.Backend, onChange func(config.Backend, []haproxy.Server)) error {
	namespace := backend.K8S.Namespace
	factory := informers.NewSharedInformerFactoryWithOptions(c.client, 0, informers.WithNamespace(namespace))
	informer := factory.Core().V1().Endpoints().Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.processEndpoints(obj, backend, onChange)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.processEndpoints(newObj, backend, onChange)
		},
		DeleteFunc: func(obj interface{}) {
			onChange(backend, []haproxy.Server{})
		},
	})

	stopCh := make(chan struct{})
	go informer.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, informer.HasSynced) {
		return fmt.Errorf("failed to sync informer cache")
	}

	return nil
}

func (c *Client) processEndpoints(obj interface{}, backend config.Backend, onChange func(config.Backend, []haproxy.Server)) {
	servers := make([]haproxy.Server, 0, 1)
	endpointName := backend.K8S.EndpointName
	portName := backend.K8S.PortName
	var port int32

	ep, ok := obj.(*v1.Endpoints)
	if !ok || ep.Name != endpointName {
		onChange(backend, servers) // empty
		return
	}

	for _, subset := range ep.Subsets {
		for _, p := range subset.Ports {
			if p.Name == portName {
				port = p.Port
				break
			}
			continue
		}
	}
	if port == 0 {
		onChange(backend, servers) // empty
		return
	}

	for _, subset := range ep.Subsets {
		for _, a := range subset.Addresses {
			servers = append(servers, haproxy.Server{Address: a.IP, Port: int64(port)})
		}
	}

	onChange(backend, servers)
}
