/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"dynamix-common/config"
	"dynamix-common/entity"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	dynamixapi "dynamix-common/api"

	cloudDataV1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
	"github.com/sirupsen/logrus"
)

const (
	envDynamixLocation = "DYNAMIX_LOCATION"
	envDynamixAccount  = "DYNAMIX_ACCOUNT"
)

type Discoverer struct {
	logger *logrus.Entry
	config *CloudConfig
}

type CloudConfig struct {
	Location    string
	Account     string
	Credentials config.Credentials
}

func newCloudConfig() (*CloudConfig, error) {
	cloudConfig := &CloudConfig{}
	cloudConfig.Location = os.Getenv(envDynamixLocation)
	if cloudConfig.Location == "" {
		return nil, fmt.Errorf("environment variable %q is required", envDynamixLocation)
	}

	cloudConfig.Account = os.Getenv(envDynamixAccount)
	if cloudConfig.Account == "" {
		return nil, fmt.Errorf("environment variable %q is required", envDynamixAccount)
	}
	credentialsConfig, err := config.NewCredentials()
	if err != nil {
		return nil, err
	}

	cloudConfig.Credentials = *credentialsConfig

	return cloudConfig, nil
}

// Client Creates a dynamix client
func (c *CloudConfig) client() (*dynamixapi.DynamixCloudAPI, error) {

	client, err := dynamixapi.NewDynamixCloudAPI(c.Credentials)
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
	discoveryData := &cloudDataV1.DynamixCloudProviderDiscoveryData{}
	if len(cloudProviderDiscoveryData) > 0 {
		err := json.Unmarshal(cloudProviderDiscoveryData, &discoveryData)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal cloud provider discovery data: %w", err)
		}
	}

	dynamixCloidAPI, err := d.config.client()
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamix api client client: %w", err)
	}

	location, err := dynamixCloidAPI.LocationService.GetLocationByName(ctx, d.config.Location)
	if err != nil {
		return nil, fmt.Errorf("failed to get dynamix location: %w", err)
	}

	seps, err := dynamixCloidAPI.SEPService.ListSEPWithPoolsByGID(ctx, location.GID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sizing policies: %w", err)
	}

	discoveryData.SEPs = mergeSEPs(discoveryData.SEPs, seps)

	discoveryDataJSON, err := json.Marshal(discoveryData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal discovery data: %w", err)
	}

	d.logger.Debugf("discovery data: %v", discoveryDataJSON)
	return discoveryDataJSON, nil
}

func (d *Discoverer) DisksMeta(ctx context.Context) ([]v1alpha1.DiskMeta, error) {
	dynamixCloidAPI, err := d.config.client()
	if err != nil {
		return nil, err
	}

	disks, err := dynamixCloidAPI.DiskService.ListDisksByAccountName(ctx, d.config.Account)
	if err != nil {
		return nil, err
	}

	if len(disks) == 0 {
		return []v1alpha1.DiskMeta{}, nil
	}

	diskMeta := make([]v1alpha1.DiskMeta, 0, len(disks))
	for _, disk := range disks {
		diskMeta = append(diskMeta, v1alpha1.DiskMeta{
			ID:   string(disk.ID),
			Name: disk.Name,
		})
	}

	return diskMeta, nil
}

// NotImplemented
func (d *Discoverer) InstanceTypes(ctx context.Context) ([]v1alpha1.InstanceType, error) {
	return nil, nil
}

func extractPoolNames(pools []entity.Pool) []string {
	result := make([]string, 0, len(pools))
	for _, pool := range pools {
		result = append(result, pool.Name)
	}

	return result
}
func mergeSEPs(
	seps []cloudDataV1.DynamixSEP,
	cloudSeps []entity.SEP,
) []cloudDataV1.DynamixSEP {
	result := []cloudDataV1.DynamixSEP{}
	cloudSepsMap := make(map[string]cloudDataV1.DynamixSEP)
	for _, sep := range cloudSeps {

		cloudSepsMap[sep.Name] = cloudDataV1.DynamixSEP{
			Name:      sep.Name,
			Pools:     extractPoolNames(sep.Pools),
			IsEnabled: sep.IsActive && sep.IsCreated,
			IsDefault: false,
		}

		result = append(result, cloudSepsMap[sep.Name])
	}

	for _, sep := range seps {
		if _, ok := cloudSepsMap[sep.Name]; ok {
			continue
		}
		result = append(result, sep)
	}

	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	for i := range result {
		if result[i].IsEnabled {
			result[i].IsDefault = true
			break
		}
	}

	return result
}
