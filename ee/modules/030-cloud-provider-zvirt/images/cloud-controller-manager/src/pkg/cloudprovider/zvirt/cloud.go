/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package zvirt

import (
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/deckhouse/zvirt-cloud-controller-manager/pkg/zvirtapi"
	"k8s.io/client-go/informers"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"

	"k8s.io/client-go/tools/cache"
)

const (
	providerName = "zvirt"

	envZvirtAPIURL   = "ZVIRT_API_URL"
	envZvirtUsername = "ZVIRT_USERNAME"
	envZvirtPassword = "ZVIRT_PASSWORD"
	envZvirtInsecure = "ZVIRT_INSECURE"
	envZvirtCaBundle = "ZVIRT_CA_BUNDLE"
)

type CloudConfig struct {
	APIURL   string
	Username string
	Password string
	Insecure bool
	CaBundle string
}

type Cloud struct {
	zvirtService *zvirtapi.ZvirtCloudAPI
	config       CloudConfig
}

func init() {
	cloudprovider.RegisterCloudProvider(
		providerName,
		func(_ io.Reader) (cloudprovider.Interface, error) {
			config, err := NewCloudConfig()
			if err != nil {
				return nil, err
			}

			api, err := zvirtapi.NewZvirtCloudAPI(
				config.APIURL,
				config.Username,
				config.Password,
				config.Insecure,
				config.CaBundle,
			)
			if err != nil {
				return nil, err
			}

			return NewCloud(*config, api), nil
		},
	)
}

func NewCloud(config CloudConfig, api *zvirtapi.ZvirtCloudAPI) *Cloud {
	return &Cloud{
		zvirtService: api,
		config:       config,
	}
}

func NewCloudConfig() (*CloudConfig, error) {
	cloudConfig := &CloudConfig{}

	apiURL := os.Getenv(envZvirtAPIURL)
	if apiURL == "" {
		return nil, fmt.Errorf("environment variable %q is required", envZvirtAPIURL)
	}
	cloudConfig.APIURL = apiURL

	username := os.Getenv(envZvirtUsername)
	if username == "" {
		return nil, fmt.Errorf("environment variable %q is required", envZvirtUsername)
	}
	cloudConfig.Username = username

	password := os.Getenv(envZvirtPassword)
	if password == "" {
		return nil, fmt.Errorf("environment variable %q is required", envZvirtPassword)
	}
	cloudConfig.Password = password

	insecure := os.Getenv(envZvirtInsecure)
	cloudConfig.Insecure = false
	klog.V(4).Infof("init CloudConfig: %s=%s", envZvirtInsecure, insecure)
	if insecure != "" {
		v, err := strconv.ParseBool(insecure)
		if err != nil {
			return nil, err
		}
		cloudConfig.Insecure = v
	}
	cloudConfig.CaBundle = os.Getenv(envZvirtCaBundle)

	return cloudConfig, nil
}

func (zc *Cloud) Initialize(
	clientBuilder cloudprovider.ControllerClientBuilder,
	stop <-chan struct{},
) {
	clientset := clientBuilder.ClientOrDie("cloud-controller-manager")

	informerFactory := informers.NewSharedInformerFactory(clientset, time.Second*30)
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
func (zc *Cloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return nil, false
}

// Instances returns an instances interface if supported.
func (zc *Cloud) Instances() (cloudprovider.Instances, bool) {
	return zc, true
}

// Zones returns a zones interface if supported.
func (zc *Cloud) Zones() (cloudprovider.Zones, bool) {
	return nil, false
}

// Clusters returns a clusters interface if supported.
func (zc *Cloud) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

// Routes returns a routes interface if supported
func (zc *Cloud) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

// ProviderName returns the cloud provider ID.
func (zc *Cloud) ProviderName() string {
	return providerName
}

// HasClusterID returns true if the cluster has a clusterID
func (zc *Cloud) HasClusterID() bool {
	return true
}

func (zc *Cloud) InstancesV2() (cloudprovider.InstancesV2, bool) {
	return nil, false
}
