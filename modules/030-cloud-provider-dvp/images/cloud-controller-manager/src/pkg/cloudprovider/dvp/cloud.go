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

package dvp

import (
	"context"
	"dvp-common/api"
	"dvp-common/config"
	"io"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	discv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
)

const (
	providerName = "dvp"

	envDVPKubernetesConfigBase64 = "DVP_KUBERNETES_CONFIG_BASE64"

	svcNameLabel = "kubernetes.io/service-name"
)

type Cloud struct {
	dvpService *api.DVPCloudAPI
	config     config.CloudConfig
}

func init() {
	cloudprovider.RegisterCloudProvider(
		providerName,
		func(_ io.Reader) (cloudprovider.Interface, error) {
			cloudConfig, err := config.NewCloudConfig()
			if err != nil {
				return nil, err
			}

			cloudAPI, err := api.NewDVPCloudAPI(cloudConfig)
			if err != nil {
				return nil, err
			}

			return NewCloud(*cloudConfig, cloudAPI)
		},
	)
}

func NewCloud(config config.CloudConfig, api *api.DVPCloudAPI) (*Cloud, error) {
	cloud := &Cloud{
		dvpService: api,
		config:     config,
	}
	return cloud, nil
}

func (c *Cloud) Initialize(
	clientBuilder cloudprovider.ControllerClientBuilder,
	stop <-chan struct{},
) {
	clientSet := clientBuilder.ClientOrDie("cloud-controller-manager")

	informerFactory := informers.NewSharedInformerFactory(clientSet, time.Second*30)
	serviceInformer := informerFactory.Core().V1().Services()
	nodeInformer := informerFactory.Core().V1().Nodes()
	esInformer := informerFactory.Discovery().V1().EndpointSlices()

	esInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			c.onEndpointSliceEvent(obj, serviceInformer.Lister(), nodeInformer.Lister())
		},
		UpdateFunc: func(_, newObj any) {
			c.onEndpointSliceEvent(newObj, serviceInformer.Lister(), nodeInformer.Lister())
		},
		DeleteFunc: func(obj any) {
			c.onEndpointSliceEvent(obj, serviceInformer.Lister(), nodeInformer.Lister())
		},
	})

	go serviceInformer.Informer().Run(stop)
	go nodeInformer.Informer().Run(stop)
	go esInformer.Informer().Run(stop)

	if !cache.WaitForCacheSync(stop, serviceInformer.Informer().HasSynced) {
		log.Fatal("Timed out waiting for caches to sync")
	}
	if !cache.WaitForCacheSync(stop, nodeInformer.Informer().HasSynced) {
		log.Fatal("Timed out waiting for caches to sync")
	}
	if !cache.WaitForCacheSync(stop, esInformer.Informer().HasSynced) {
		log.Fatal("Timed out waiting for caches to sync")
	}
}

// LoadBalancer returns a balancer interface if supported.
func (c *Cloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return c, true
}

// Instances returns an instances interface if supported.
func (c *Cloud) Instances() (cloudprovider.Instances, bool) {
	return c, true
}

// Zones returns a zones interface if supported.
func (c *Cloud) Zones() (cloudprovider.Zones, bool) {
	return nil, false
}

// Clusters returns a clusters interface if supported.
func (c *Cloud) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

// Routes returns a routes interface if supported
func (c *Cloud) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

// ProviderName returns the cloud provider ID.
func (c *Cloud) ProviderName() string {
	return providerName
}

// HasClusterID returns true if the cluster has a clusterID
func (c *Cloud) HasClusterID() bool {
	return true
}

func (c *Cloud) InstancesV2() (cloudprovider.InstancesV2, bool) {
	return nil, false
}

func (c *Cloud) onEndpointSliceEvent(
	obj any,
	svcLister corelisters.ServiceLister,
	nodeLister corelisters.NodeLister,
) {
	es, ok := obj.(*discv1.EndpointSlice)
	if !ok {
		// tombstone case
		tomb, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		es, ok = tomb.Obj.(*discv1.EndpointSlice)
		if !ok {
			return
		}
	}

	svcName := es.GetLabels()[svcNameLabel]
	if svcName == "" {
		return
	}

	svc, err := svcLister.Services(es.Namespace).Get(svcName)
	if err != nil {
		klog.V(4).InfoS("onEndpointSliceEvent: service not found", "namespace", es.Namespace, "service", svcName, "err", err)
		return
	}
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return
	}
	if svc.Spec.ExternalTrafficPolicy != corev1.ServiceExternalTrafficPolicyTypeLocal {
		return
	}

	nodes, err := nodeLister.List(labels.Everything())
	if err != nil || len(nodes) == 0 {
		klog.V(4).InfoS("onEndpointSliceEvent: no nodes to process", "namespace", es.Namespace, "service", svc.Name, "err", err)
		return
	}

	klog.V(3).InfoS("onEndpointSliceEvent: triggering ensureLB",
		"namespace", svc.Namespace, "service", svc.Name, "slice", es.Name)

	if _, err = c.ensureLB(context.Background(), svc, nodes); err != nil {
		klog.ErrorS(err, "ensureLB failed", "namespace", svc.Namespace, "service", svc.Name)
	}
}
