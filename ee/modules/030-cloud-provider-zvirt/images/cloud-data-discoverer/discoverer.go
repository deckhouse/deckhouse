/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"

	"github.com/sirupsen/logrus"

	ovirtclientlog "github.com/ovirt/go-ovirt-client-log/v3"
	ovirtclient "github.com/ovirt/go-ovirt-client/v3"

	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
)

const (
	envZvirtAPIURL   = "ZVIRT_API_URL"
	envZvirtUsername = "ZVIRT_USERNAME"
	envZvirtPassword = "ZVIRT_PASSWORD"
	envZvirtInsecure = "ZVIRT_INSECURE"
)

type Discoverer struct {
	logger *logrus.Entry
	config *CloudConfig
}

type CloudConfig struct {
	APIURL   string `json:"serrver"`
	Username string `json:"user"`
	Password string `json:"password"`
	Insecure bool   `json:"insecure"`
}

func newCloudConfig() (*CloudConfig, error) {
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
	if insecure != "" {
		v, err := strconv.ParseBool(insecure)
		if err != nil {
			return nil, err
		}
		cloudConfig.Insecure = v
	}

	return cloudConfig, nil
}

// Client Creates a zvirt client
func (c *CloudConfig) client() (ovirtclient.ClientWithLegacySupport, error) {
	logger := ovirtclientlog.NewGoLogger(log.Default())

	tls := ovirtclient.TLS()

	if c.Insecure {
		tls.Insecure()
	} else {
		tls.CACertsFromSystem()
	}
	client, err := ovirtclient.New(
		c.APIURL,
		c.Username,
		c.Password,
		tls,
		logger,
		nil,
	)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func NewDiscoverer(logger *logrus.Entry) *Discoverer {
	config, err := newCloudConfig()
	if err != nil {
		logger.Fatalf("Cannot get opts from env: %v", err)
	}

	return &Discoverer{
		logger: logger,
		config: config,
	}
}

func (d *Discoverer) DiscoveryData(
	ctx context.Context,
	cloudProviderDiscoveryData []byte,
) ([]byte, error) {
	discoveryData := &v1alpha1.ZvirtCloudProviderDiscoveryData{}
	if len(cloudProviderDiscoveryData) > 0 {
		err := json.Unmarshal(cloudProviderDiscoveryData, &discoveryData)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal cloud provider discovery data: %v", err)
		}
	}

	zvirtClient, err := d.config.client()
	if err != nil {
		return nil, fmt.Errorf("failed to create zvirt client: %v", err)
	}

	sd, err := d.getStorageDomains(ctx, zvirtClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get sizing policies: %v", err)
	}

	discoveryData.StorageDomains = mergeStorageDomains(discoveryData.StorageDomains, sd)

	discoveryDataJson, err := json.Marshal(discoveryData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal discovery data: %v", err)
	}

	d.logger.Debugf("discovery data: %v", discoveryDataJson)
	return discoveryDataJson, nil
}

func (d *Discoverer) getStorageDomains(
	ctx context.Context,
	zvirtClient ovirtclient.ClientWithLegacySupport,
) ([]ovirtclient.StorageDomain, error) {
	sd, err := zvirtClient.ListStorageDomains(getRetryStrategy(ctx)...)
	if err != nil {
		return nil, err
	}
	return sd, nil
}

// NotImplemented
func (d *Discoverer) DisksMeta(ctx context.Context) ([]v1alpha1.DiskMeta, error) {
	return []v1alpha1.DiskMeta{}, nil
}

func (d *Discoverer) InstanceTypes(_ context.Context) ([]v1alpha1.InstanceType, error) {
	return []v1alpha1.InstanceType{}, nil
}

func mergeStorageDomains(
	sds []v1alpha1.ZvirtStorageDomain,
	cloudSds []ovirtclient.StorageDomain,
) []v1alpha1.ZvirtStorageDomain {
	result := []v1alpha1.ZvirtStorageDomain{}
	for _, sd := range cloudSds {
		logrus.Infof("%+v", sd)
		logrus.Infof(
			"StorageDomain: name=%s type=%s IsEnabled=%v status=%s\n",
			sd.Name(),
			string(sd.StorageType()),
			sd.Status() == ovirtclient.StorageDomainStatusActive,
			sd.Status(),
		)

		status := sd.Status() == ovirtclient.StorageDomainStatusActive
		if sd.Status() == ovirtclient.StorageDomainStatus("") && sd.ExternalStatus() == ovirtclient.StorageDomainExternalStatusOk {
			status = true
		}

		result = append(result, v1alpha1.ZvirtStorageDomain{
			Name:      sd.Name(),
			Type:      string(sd.StorageType()),
			IsEnabled: status,
			IsDefault: false,
		})
	}

	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	if len(result) > 0 {
		result[0].IsDefault = true
	}
	return result
}

func getRetryStrategy(ctx context.Context) []ovirtclient.RetryStrategy {
	return []ovirtclient.RetryStrategy{
		ovirtclient.AutoRetry(),
		ovirtclient.ContextStrategy(ctx),
	}
}
