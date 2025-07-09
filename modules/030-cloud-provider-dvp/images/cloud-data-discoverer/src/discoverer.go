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

	cloudDataV1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
	"github.com/deckhouse/deckhouse/pkg/log"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
)

type CloudConfig struct {
	*config.CloudConfig
}

type Discoverer struct {
	logger *log.Logger
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

func NewDiscoverer(logger *log.Logger) *Discoverer {
	config, err := newCloudConfig()
	if err != nil {
		logger.Fatal("Cannot get opts from env: %v", err)
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

	discoveryDataJSON, err := json.Marshal(discoveryData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal discovery data: %v", err)
	}

	d.logger.Debug("discovery data", "discoveryDataJSON", discoveryDataJSON)
	return discoveryDataJSON, nil
}

// getDVPStorageClass return storage classes list from DVP
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

// DisksMeta NotImplemented
func (d *Discoverer) DisksMeta(ctx context.Context) ([]v1alpha1.DiskMeta, error) {
	return []v1alpha1.DiskMeta{}, nil
}

// InstanceTypes NotImplemented
func (d *Discoverer) InstanceTypes(ctx context.Context) ([]v1alpha1.InstanceType, error) {
	return nil, nil
}

func mergeStorageDomains(
	sds []cloudDataV1.DVPStorageClass, // stored storage classes
	cloudSds []storagev1.StorageClass, // discovered storage classes
) []cloudDataV1.DVPStorageClass {
	result := []cloudDataV1.DVPStorageClass{}
	cloudSdsMap := make(map[string]cloudDataV1.DVPStorageClass)
	for _, sc := range cloudSds {

		volumeBindingMode := storagev1.VolumeBindingWaitForFirstConsumer
		if sc.VolumeBindingMode != nil {
			volumeBindingMode = *sc.VolumeBindingMode
		}

		reclaimPolicy := corev1.PersistentVolumeReclaimDelete
		if sc.ReclaimPolicy != nil {
			reclaimPolicy = *sc.ReclaimPolicy
		}

		allowVolumeExpansion := false
		if sc.AllowVolumeExpansion != nil {
			allowVolumeExpansion = *sc.AllowVolumeExpansion
		}

		cloudSdsMap[sc.Name] = cloudDataV1.DVPStorageClass{
			Name:                 sc.Name,
			VolumeBindingMode:    string(volumeBindingMode),
			ReclaimPolicy:        string(reclaimPolicy),
			AllowVolumeExpansion: allowVolumeExpansion,
			IsEnabled:            true,
			IsDefault:            false,
		}

		result = append(result, cloudSdsMap[sc.Name])
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
