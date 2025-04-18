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

package main

import (
	"context"
	"dvp-common/api"
	"dvp-common/config"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/sirupsen/logrus"
	storagev1 "k8s.io/api/storage/v1"

	cloudDataV1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
)

type CloudConfig struct {
	*config.CloudConfig
}

type Discoverer struct {
	logger *logrus.Entry
	config *CloudConfig
}

func newCloudConfig() (*CloudConfig, error) {
	cloudConfig, err := config.NewCloudConfig()
	if err != nil {
		return nil, err
	}
	return &CloudConfig{CloudConfig: cloudConfig}, nil
}

// Client Creates a dvp client
func (c *CloudConfig) client() (*api.DVPCloudAPI, error) {
	cloudAPI, err := api.NewDVPCloudAPI(c.CloudConfig)
	if err != nil {
		return nil, err
	}
	return cloudAPI, nil
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

func (d *Discoverer) CheckCloudConditions(ctx context.Context) ([]v1alpha1.CloudCondition, error) {
	return nil, nil
}

func (d *Discoverer) DiscoveryData(
	ctx context.Context,
	cloudProviderDiscoveryData []byte,
) ([]byte, error) {
	discoveryData := &cloudDataV1.DVPCloudProviderDiscoveryData{}
	if len(cloudProviderDiscoveryData) > 0 {
		err := json.Unmarshal(cloudProviderDiscoveryData, &discoveryData)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal cloud provider discovery data: %v", err)
		}
	}

	dvpClient, err := d.config.client()
	if err != nil {
		return nil, fmt.Errorf("failed to create dvp client: %v", err)
	}

	sd, err := d.getDVPStorageClass(ctx, dvpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get DVP storage class: %v", err)
	}

	discoveryData.StorageClassList = mergeStorageDomains(discoveryData.StorageClassList, sd)

	discoveryDataJson, err := json.Marshal(discoveryData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal discovery data: %v", err)
	}

	d.logger.Debugf("discovery data: %v", discoveryDataJson)
	return discoveryDataJson, nil
}

func (d *Discoverer) getDVPStorageClass(
	ctx context.Context,
	dvpClient *api.DVPCloudAPI,
) ([]storagev1.StorageClass, error) {
	scl, err := dvpClient.DiskService.GetStorageClassList(ctx)
	if err != nil {
		return nil, err
	}
	return scl.Items, nil
}

// NotImplemented
func (d *Discoverer) DisksMeta(ctx context.Context) ([]v1alpha1.DiskMeta, error) {
	return []v1alpha1.DiskMeta{}, nil
}

// NotImplemented
func (d *Discoverer) InstanceTypes(ctx context.Context) ([]v1alpha1.InstanceType, error) {
	return nil, nil
}

func mergeStorageDomains(
	sds []cloudDataV1.DVPStorageClass,
	cloudSds []storagev1.StorageClass,
) []cloudDataV1.DVPStorageClass {
	result := []cloudDataV1.DVPStorageClass{}
	cloudSdsMap := make(map[string]cloudDataV1.DVPStorageClass)
	for _, sd := range cloudSds {

		cloudSdsMap[sd.Name] = cloudDataV1.DVPStorageClass{
			Name:      sd.Name,
			IsEnabled: true,
			IsDefault: false,
		}

		result = append(result, cloudSdsMap[sd.Name])
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
