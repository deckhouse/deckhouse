package k8s

import (
	"fmt"
	"slices"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

func (c *Client) WatchEndpoints(namespace string, endpointName string, portNames []string, onChange func([]string)) error {
	factory := informers.NewSharedInformerFactoryWithOptions(c.client, 0, informers.WithNamespace(namespace))
	informer := factory.Core().V1().Endpoints().Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.processEndpoints(obj, endpointName, portNames, onChange)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.processEndpoints(newObj, endpointName, portNames, onChange)
		},
		DeleteFunc: func(obj interface{}) {
			onChange([]string{})
		},
	})

	stopCh := make(chan struct{})
	go informer.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, informer.HasSynced) {
		return fmt.Errorf("failed to sync informer cache")
	}

	return nil
}

func (c *Client) processEndpoints(obj interface{}, endpointName string, portNames []string, onChange func([]string)) {
	var addresses []string
	var ports []int32
	var addressPortList []string

	ep, ok := obj.(*v1.Endpoints)
	if !ok || ep.Name != endpointName {
		onChange(addressPortList) // empty
		return
	}

	for _, subset := range ep.Subsets {
		for _, p := range subset.Ports {
			if slices.Contains(portNames, p.Name) {
				ports = append(ports, p.Port)
			}
		}
	}
	if len(ports) < 1 {
		onChange(addressPortList) // empty
		return
	}

	for _, subset := range ep.Subsets {
		for _, a := range subset.Addresses {
			addresses = append(addresses, a.IP)
		}
	}

	for _, address := range addresses {
		for _, port := range ports {
			addressPortList = append(addressPortList, fmt.Sprintf("%s:%d", address, port))
		}
	}

	onChange(addressPortList)
}
