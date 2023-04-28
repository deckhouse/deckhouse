/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumetypes"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/utils/openstack/clientconfig"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/resource"

	v1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
)

type Discoverer struct {
	logger   *log.Entry
	authOpts gophercloud.AuthOptions
	region   string
}

func NewDiscoverer(logger *log.Entry) *Discoverer {
	authOpts, err := openstack.AuthOptionsFromEnv()
	if err != nil {
		logger.Fatalf("Cannnot get opts from env: %v", err)
	}

	region := os.Getenv("OS_REGION")
	if region == "" {
		logger.Fatalf("Cannnot get OS_REGION env")
	}

	return &Discoverer{
		logger:   logger,
		region:   region,
		authOpts: authOpts,
	}
}

func (d *Discoverer) InstanceTypes(_ context.Context) ([]v1alpha1.InstanceType, error) {
	provider, err := openstack.AuthenticatedClient(d.authOpts)
	if err != nil {
		return nil, fmt.Errorf("cannot create AuthenticatedClient: %v", err)
	}

	client, err := openstack.NewComputeV2(provider, gophercloud.EndpointOpts{
		Region: d.region,
	})

	if err != nil {
		return nil, fmt.Errorf("cannot create ComputeV2 client: %v", err)
	}

	pages, err := flavors.ListDetail(client, nil).AllPages()
	if err != nil {
		return nil, err
	}

	flvs, err := flavors.ExtractFlavors(pages)
	if err != nil {
		return nil, err
	}

	res := make([]v1alpha1.InstanceType, 0, len(flvs))
	for _, f := range flvs {
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

func (d *Discoverer) VolumeTypes(ctx context.Context) ([]v1.VolumeType, error) {
	client, err := clientconfig.NewServiceClient("volume", nil)
	if err != nil {
		return nil, err
	}

	allPages, err := volumetypes.List(client, volumetypes.ListOpts{}).AllPages()
	if err != nil {
		return nil, err
	}

	volumeTypes, err := volumetypes.ExtractVolumeTypes(allPages)
	if err != nil || len(volumeTypes) == 0 {
		return nil, fmt.Errorf("list of volume types is empty is empty, or an error was returned: %v", err)
	}

	var volumeTypesList []v1.VolumeType

	for _, volumeType := range volumeTypes {
		volumeTypesList = append(volumeTypesList, v1.VolumeType{
			Name: getStorageClassName(volumeType.Name),
			Type: volumeType.Name,
			Parameters: map[string]any{
				"ID":          volumeType.ID,
				"Name":        volumeType.Name,
				"Description": volumeType.Description,
				"ExtraSpecs":  volumeType.ExtraSpecs,
				"IsPublic":    volumeType.IsPublic,
				"QosSpecID":   volumeType.QosSpecID,
			},
		})
	}

	return volumeTypesList, nil
}

// Get StorageClass name from Volume type name to match Kubernetes restrictions from https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names
func getStorageClassName(value string) string {
	mapFn := func(r rune) rune {
		if r >= 'a' && r <= 'z' ||
			r >= 'A' && r <= 'Z' ||
			r >= '0' && r <= '9' ||
			r == '-' || r == '.' {
			return unicode.ToLower(r)
		} else if r == ' ' {
			return '-'
		}
		return rune(-1)
	}

	// a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.'
	value = strings.Map(mapFn, value)

	// must start and end with an alphanumeric character
	return strings.Trim(value, "-.")
}

func (d *Discoverer) DiscoveryData(ctx context.Context) (v1.DiscoveryData, error) {
	panic("not implemented")
}
