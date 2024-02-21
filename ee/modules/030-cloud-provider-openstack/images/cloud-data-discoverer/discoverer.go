/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumes"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumetypes"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/groups"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
)

type Discoverer struct {
	logger       *log.Entry
	authOpts     gophercloud.AuthOptions
	region       string
	moduleConfig []byte
	clusterUUID  string
}

func NewDiscoverer(logger *log.Entry) *Discoverer {
	authOpts, err := openstack.AuthOptionsFromEnv()
	if err != nil {
		logger.Fatalf("Cannot get opts from env: %v", err)
	}

	region := os.Getenv("OS_REGION")
	if region == "" {
		logger.Fatalf("Cannot get OS_REGION env")
	}

	clusterUUID := os.Getenv("CLUSTER_UUID")

	moduleConfig := os.Getenv("MODULE_CONFIG")

	return &Discoverer{
		logger:       logger,
		region:       region,
		authOpts:     authOpts,
		moduleConfig: []byte(moduleConfig),
		clusterUUID:  clusterUUID,
	}
}

func (d *Discoverer) InstanceTypes(ctx context.Context) ([]v1alpha1.InstanceType, error) {
	provider, err := newProvider(d.authOpts, d.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenStack provider: %v", err)
	}

	flavors, err := d.getFlavors(ctx, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to get flavors: %v", err)
	}

	res := make([]v1alpha1.InstanceType, 0, len(flavors))
	for _, f := range flavors {
		diskSize := f.Disk
		if diskSize == 0 {
			diskSize = f.Ephemeral
		}
		res = append(res, v1alpha1.InstanceType{
			Name:     f.Name,
			CPU:      resource.MustParse(strconv.FormatInt(int64(f.VCPUs), 10)),
			Memory:   resource.MustParse(strconv.FormatInt(int64(f.RAM), 10) + "Mi"),
			RootDisk: resource.MustParse(strconv.FormatInt(int64(diskSize), 10) + "Gi"),
		})
	}

	return res, nil
}

func (d *Discoverer) DiscoveryData(ctx context.Context, cloudProviderDiscoveryData []byte) ([]byte, error) {
	var discoveryData OpenstackCloudDiscoveryData

	if len(cloudProviderDiscoveryData) == 0 {
		cloudProviderDiscoveryData = d.moduleConfig
	}

	if len(cloudProviderDiscoveryData) > 0 {
		err := json.Unmarshal(cloudProviderDiscoveryData, &discoveryData)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal cloud provider discovery data: %v", err)
		}
	}

	provider, err := newProvider(d.authOpts, d.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenStack provider: %v", err)
	}

	flavors, err := d.getFlavors(ctx, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to get flavors: %v", err)
	}

	flavorNames := make([]string, 0, len(flavors))

	for _, flavor := range flavors {
		flavorNames = append(flavorNames, flavor.Name)
	}

	additionalSecurityGroups, err := d.getAdditionalSecurityGroups(ctx, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to get additional security groups: %v", err)
	}

	additionalNetworks, err := d.getAdditionalNetworks(ctx, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to get additional networks: %v", err)
	}

	images, err := d.getImages(ctx, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to get images: %v", err)
	}

	volumeTypes, err := d.getVolumeTypes(ctx, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to get volume types: %v", err)
	}

	discoveryDataJson, err := json.Marshal(v1alpha1.OpenStackCloudProviderDiscoveryData{
		APIVersion:               "deckhouse.io/v1alpha1",
		Kind:                     "OpenStackCloudProviderDiscoveryData",
		Flavors:                  flavorNames,
		AdditionalNetworks:       additionalNetworks,
		AdditionalSecurityGroups: additionalSecurityGroups,
		DefaultImageName:         discoveryData.Instances.ImageName,
		Images:                   images,
		MainNetwork:              discoveryData.Instances.MainNetwork,
		Zones:                    discoveryData.Zones,
		VolumeTypes:              volumeTypes,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal discovery data: %v", err)
	}

	return discoveryDataJson, nil
}

func (d *Discoverer) DisksMeta(ctx context.Context) ([]v1alpha1.DiskMeta, error) {
	provider, err := newProvider(d.authOpts, d.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenStack provider: %v", err)
	}

	disks, err := d.getVolumes(ctx, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to get volumes: %v", err)
	}

	disksMeta := make([]v1alpha1.DiskMeta, 0, len(disks))

	for _, volume := range disks {
		disksMeta = append(disksMeta, v1alpha1.DiskMeta{ID: volume.ID, Name: volume.Name})
	}

	return disksMeta, nil
}

func newProvider(authOpts gophercloud.AuthOptions, logger *log.Entry) (*gophercloud.ProviderClient, error) {
	provider, err := openstack.AuthenticatedClient(authOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenStack client: %v", err)
	}

	provider.MaxBackoffRetries = 3
	provider.RetryFunc = RetryFunc(logger)
	provider.RetryBackoffFunc = RetryBackoffFunc(logger)

	return provider, nil
}

func (d *Discoverer) getFlavors(ctx context.Context, provider *gophercloud.ProviderClient) ([]flavors.Flavor, error) {
	client, err := openstack.NewComputeV2(provider, gophercloud.EndpointOpts{
		Region: d.region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ComputeV2 client: %v", err)
	}

	client.Context = ctx

	allPages, err := flavors.ListDetail(client, flavors.ListOpts{}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list flavors: %v", err)
	}

	flavors, err := flavors.ExtractFlavors(allPages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract flavors: %v", err)
	}

	return flavors, nil
}

func (d *Discoverer) getVolumes(ctx context.Context, provider *gophercloud.ProviderClient) ([]volumes.Volume, error) {
	client, err := openstack.NewBlockStorageV3(provider, gophercloud.EndpointOpts{
		Region: d.region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create BlockStorageV3 client: %v", err)
	}

	client.Context = ctx

	allPages, err := volumes.List(client, volumes.ListOpts{
		Metadata: map[string]string{
			"cinder.csi.openstack.org/cluster": d.clusterUUID,
		},
	}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %v", err)
	}

	cloudVolumes, err := volumes.ExtractVolumes(allPages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract volumes: %v", err)
	}

	return cloudVolumes, nil
}

func (d *Discoverer) getVolumeTypes(ctx context.Context, provider *gophercloud.ProviderClient) ([]v1alpha1.OpenStackCloudProviderDiscoveryDataVolumeType, error) {
	client, err := openstack.NewBlockStorageV3(provider, gophercloud.EndpointOpts{
		Region: d.region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create BlockStorageV3 client: %v", err)
	}

	client.Context = ctx

	allPages, err := volumetypes.List(client, volumetypes.ListOpts{}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list volume types: %v", err)
	}

	volumeTypes, err := volumetypes.ExtractVolumeTypes(allPages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract volume types: %v", err)
	}
	if len(volumeTypes) == 0 {
		return nil, errors.New("volume types list is empty")
	}

	var volumeTypesList []v1alpha1.OpenStackCloudProviderDiscoveryDataVolumeType

	for _, volumeType := range volumeTypes {
		volumeTypesList = append(volumeTypesList, v1alpha1.OpenStackCloudProviderDiscoveryDataVolumeType{
			Name:        volumeType.Name,
			ID:          volumeType.ID,
			Description: volumeType.Description,
			ExtraSpecs:  volumeType.ExtraSpecs,
			IsPublic:    volumeType.IsPublic,
			QosSpecID:   volumeType.QosSpecID,
		})
	}

	return volumeTypesList, nil
}

func (d *Discoverer) getAdditionalNetworks(ctx context.Context, provider *gophercloud.ProviderClient) ([]string, error) {
	client, err := openstack.NewNetworkV2(provider, gophercloud.EndpointOpts{
		Region: d.region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create NetworkV2 client: %v", err)
	}

	client.Context = ctx

	allPages, err := networks.List(client, networks.ListOpts{}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %v", err)
	}

	networks, err := networks.ExtractNetworks(allPages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract networks: %v", err)
	}

	networkNames := make([]string, 0, len(networks))

	for _, network := range networks {
		networkNames = append(networkNames, network.Name)
	}

	return removeDuplicates(networkNames), nil
}

func (d *Discoverer) getAdditionalSecurityGroups(ctx context.Context, provider *gophercloud.ProviderClient) ([]string, error) {
	client, err := openstack.NewNetworkV2(provider, gophercloud.EndpointOpts{
		Region: d.region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ComputeV2 client: %v", err)
	}

	client.Context = ctx

	allPages, err := groups.List(client, groups.ListOpts{}).AllPages()
	if err != nil {
		if _, ok := err.(gophercloud.ErrDefault404); ok {
			d.logger.Infoln("Cloud does not support security groups. Returns empty array")
			return make([]string, 0), nil
		}

		return nil, fmt.Errorf("failed to list security groups: %v", err)
	}

	groups, err := groups.ExtractGroups(allPages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract security groups: %v", err)
	}

	groupNames := make([]string, 0, len(groups))

	for _, group := range groups {
		groupNames = append(groupNames, group.Name)
	}

	return removeDuplicates(groupNames), nil
}

func (d *Discoverer) getImages(ctx context.Context, provider *gophercloud.ProviderClient) ([]string, error) {
	client, err := openstack.NewImageServiceV2(provider, gophercloud.EndpointOpts{
		Region: d.region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ImageServiceV2 client: %v", err)
	}

	client.Context = ctx

	allPages, err := images.List(client, images.ListOpts{}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %v", err)
	}

	images, err := images.ExtractImages(allPages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract images: %v", err)
	}

	imageNames := make([]string, 0, len(images))

	for _, image := range images {
		imageNames = append(imageNames, image.Name)
	}

	return removeDuplicates(imageNames), nil
}

type OpenstackCloudDiscoveryData struct {
	Zones     []string                             `json:"zones,omitempty" yaml:"zones,omitempty"`
	Instances OpenstackCloudDiscoveryDataInstances `json:"instances,omitempty" yaml:"instances,omitempty"`
}

type OpenstackCloudDiscoveryDataInstances struct {
	ImageName   string `json:"imageName,omitempty" yaml:"imageName,omitempty"`
	MainNetwork string `json:"mainNetwork,omitempty" yaml:"mainNetwork,omitempty"`
}

func RetryFunc(logger *log.Entry) gophercloud.RetryFunc {
	return func(ctx context.Context, method, url string, options *gophercloud.RequestOpts, err error, failCount uint) error {
		if failCount >= 3 {
			return err
		}

		select {
		case <-time.After(3 * time.Second):
		case <-ctx.Done():
			logger.Errorf("Sleeping aborted: %v", ctx.Err())

			return err
		}

		return nil
	}
}

func RetryBackoffFunc(logger *log.Entry) gophercloud.RetryBackoffFunc {
	return func(ctx context.Context, respErr *gophercloud.ErrUnexpectedResponseCode, err error, retries uint) error {
		retryAfter := respErr.ResponseHeader.Get("Retry-After")
		if retryAfter == "" {
			return err
		}

		var sleep time.Duration

		// Parse delay seconds or HTTP date
		if v, err := strconv.ParseUint(retryAfter, 10, 32); err == nil {
			sleep = time.Duration(v) * time.Second
		} else if v, err := time.Parse(http.TimeFormat, retryAfter); err == nil {
			sleep = time.Until(v)
		} else {
			return err
		}

		logger.Warnf("Received StatusTooManyRequests response code sleeping for %s", sleep)

		select {
		case <-time.After(sleep):
		case <-ctx.Done():
			logger.Errorf("Sleeping aborted: %v", ctx.Err())

			return err
		}

		return nil
	}
}

func removeDuplicates(list []string) []string {
	var (
		keys       = make(map[string]struct{})
		uniqueList []string
	)

	for _, elem := range list {
		if elem == "" {
			continue
		}

		if _, ok := keys[elem]; !ok {
			keys[elem] = struct{}{}
			uniqueList = append(uniqueList, elem)
		}
	}

	return uniqueList
}
