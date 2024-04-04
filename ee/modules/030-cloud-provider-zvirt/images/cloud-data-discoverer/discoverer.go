/*
Copyright 2024 Flant JSC
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
	"strings"

	cloudDataV1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
	"github.com/sirupsen/logrus"

	ovirtclientlog "github.com/ovirt/go-ovirt-client-log/v3"
	ovirtclient "github.com/ovirt/go-ovirt-client/v3"
)

const (
	envZvirtAPIURL   = "ZVIRT_API_URL"
	envZvirtUsername = "ZVIRT_USERNAME"
	envZvirtPassword = "ZVIRT_PASSWORD"
	envZvirtInsecure = "ZVIRT_INSECURE"
	envZvirtCaBundle = "ZVIRT_CA_BUNDLE"
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
	CaBundle string `json:"caBundle"`
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

	cloudConfig.Insecure = strings.ToLower(os.Getenv(envZvirtInsecure)) == "true"
	cloudConfig.CaBundle = os.Getenv(envZvirtCaBundle)

	return cloudConfig, nil
}

// Client Creates a zvirt client
func (c *CloudConfig) client() (ovirtclient.ClientWithLegacySupport, error) {
	logger := ovirtclientlog.NewGoLogger(log.Default())

	tls := ovirtclient.TLS()

	if c.Insecure {
		tls.Insecure()
	} else if c.CaBundle != "" {
		tls.CACertsFromMemory([]byte(c.CaBundle))
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
	discoveryData := &cloudDataV1.ZvirtCloudProviderDiscoveryData{}
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
	sd, err := zvirtClient.WithContext(ctx).ListStorageDomains()
	if err != nil {
		return nil, err
	}
	return sd, nil
}

func (d *Discoverer) DisksMeta(ctx context.Context) ([]v1alpha1.DiskMeta, error) {
	zvirtClient, err := d.config.client()
	if err != nil {
		return nil, err
	}

	disks, err := zvirtClient.WithContext(ctx).ListDisks()
	if err != nil {
		return nil, err
	}

	if len(disks) == 0 {
		return []v1alpha1.DiskMeta{}, nil
	}

	diskMeta := make([]v1alpha1.DiskMeta, 0, len(disks))
	for _, disk := range disks {
		diskMeta = append(diskMeta, v1alpha1.DiskMeta{
			ID:   string(disk.ID()),
			Name: disk.Alias(),
		})
	}

	return diskMeta, nil
}

// NotImplemented
func (d *Discoverer) InstanceTypes(ctx context.Context) ([]v1alpha1.InstanceType, error) {
	return nil, nil
}

func mergeStorageDomains(
	sds []cloudDataV1.ZvirtStorageDomain,
	cloudSds []ovirtclient.StorageDomain,
) []cloudDataV1.ZvirtStorageDomain {
	result := []cloudDataV1.ZvirtStorageDomain{}
	cloudSdsMap := make(map[string]cloudDataV1.ZvirtStorageDomain)
	for _, sd := range cloudSds {
		// status may be unknown if external status has arrived
		status := sd.Status() == ovirtclient.StorageDomainStatusActive
		if sd.Status() == ovirtclient.StorageDomainStatus("") && sd.ExternalStatus() == ovirtclient.StorageDomainExternalStatusOk {
			status = true
		}

		cloudSdsMap[sd.Name()] = cloudDataV1.ZvirtStorageDomain{
			Name:      sd.Name(),
			IsEnabled: status,
			IsDefault: false,
		}

		result = append(result, cloudSdsMap[sd.Name()])
	}

	for _, sd := range sds {
		if _, ok := cloudSdsMap[sd.Name]; ok {
			continue
		}
		result = append(result, sd)
	}

	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	if len(result) > 0 {
		result[0].IsDefault = true
	}
	return result
}
