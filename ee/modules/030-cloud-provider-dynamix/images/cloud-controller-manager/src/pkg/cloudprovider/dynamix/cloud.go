/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package dynamix

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	cloudprovider "k8s.io/cloud-provider"

	"dynamix-common/api"
	"dynamix-common/config"
)

const (
	providerName = "dynamix"

	envDynamixAppID         = "DYNAMIX_APP_ID"
	envDynamixAppSecret     = "DYNAMIX_APP_SECRET"
	envDynamixOAuth2URL     = "DYNAMIX_OAUTH2_URL"
	envDynamixControllerURL = "DYNAMIX_CONTROLLER_URL"
	envDynamixInsecure      = "DYNAMIX_INSECURE"
)

type CloudConfig struct {
	config.Credentials
}

type Cloud struct {
	dynamixService *api.DynamixCloudAPI
	config         CloudConfig
}

func init() {
	cloudprovider.RegisterCloudProvider(
		providerName,
		func(_ io.Reader) (cloudprovider.Interface, error) {
			cloudConfig, err := NewCloudConfig()
			if err != nil {
				return nil, err
			}

			cloudAPI, err := api.NewDynamixCloudAPI(cloudConfig.Credentials)
			if err != nil {
				return nil, err
			}

			return NewCloud(*cloudConfig, cloudAPI), nil
		},
	)
}

func NewCloud(config CloudConfig, api *api.DynamixCloudAPI) *Cloud {
	return &Cloud{
		dynamixService: api,
		config:         config,
	}
}

func NewCloudConfig() (*CloudConfig, error) {
	cloudConfig := &CloudConfig{}

	appID := os.Getenv(envDynamixAppID)
	if appID == "" {
		return nil, fmt.Errorf("environment variable %q is required", envDynamixAppID)
	}
	cloudConfig.AppID = appID

	appSecret := os.Getenv(envDynamixAppSecret)
	if appSecret == "" {
		return nil, fmt.Errorf("environment variable %q is required", envDynamixAppSecret)
	}
	cloudConfig.AppSecret = appSecret

	oAuth2URL := os.Getenv(envDynamixOAuth2URL)
	if oAuth2URL == "" {
		return nil, fmt.Errorf("environment variable %q is required", envDynamixOAuth2URL)
	}
	cloudConfig.OAuth2URL = oAuth2URL

	controllerURL := os.Getenv(envDynamixControllerURL)
	if controllerURL == "" {
		return nil, fmt.Errorf("environment variable %q is required", envDynamixControllerURL)
	}
	cloudConfig.ControllerURL = controllerURL

	cloudConfig.Insecure = strings.ToLower(os.Getenv(envDynamixInsecure)) == "true"

	return cloudConfig, nil
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
	return nil, false
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
