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
	"dvp-common/api"
	"dvp-common/config"
	"io"
	"log"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	cloudprovider "k8s.io/cloud-provider"
)

const (
	providerName = "dvp"

	envDVPKubernetesConfigBase64 = "DVP_KUBERNETES_CONFIG_BASE64"
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

	go serviceInformer.Informer().Run(stop)
	go nodeInformer.Informer().Run(stop)

	if !cache.WaitForCacheSync(stop, serviceInformer.Informer().HasSynced) {
		log.Fatal("Timed out waiting for caches to sync")
	}
	if !cache.WaitForCacheSync(stop, nodeInformer.Informer().HasSynced) {
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
