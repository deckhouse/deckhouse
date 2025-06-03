/*
Copyright 2024 Flant JSC

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

package yandexcloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"sort"
	"strings"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/vpc/v1"
	ycsdk "github.com/yandex-cloud/go-sdk"
	"github.com/yandex-cloud/go-sdk/iamkey"

	v1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/cloud-data/discovery/meta"
	"github.com/deckhouse/deckhouse/pkg/log"
)

var (
	ErrNoFolderID = errors.New("empty folder ID")
	ErrSAJSON     = errors.New("empty service account JSON")
)

type Discoverer struct {
	logger   *log.Logger
	folderID string
	sdk      *ycsdk.SDK
}

func NewDiscoverer(logger *log.Logger, folderID, saKeyJSON string) (*Discoverer, error) {
	if folderID == "" {
		return nil, ErrNoFolderID
	}

	if saKeyJSON == "" {
		return nil, ErrSAJSON
	}

	saKeyJSONBytes := []byte(saKeyJSON)

	key, err := iamkey.ReadFromJSONBytes(saKeyJSONBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse saKeyJSON: %w", err)
	}

	creds, err := ycsdk.ServiceAccountKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create credentials for the given IAM Key: %w", err)
	}

	sdk, err := ycsdk.Build(context.Background(), ycsdk.Config{
		Credentials: creds,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build YC SDK: %w", err)
	}

	return &Discoverer{
		logger:   logger,
		folderID: folderID,
		sdk:      sdk,
	}, nil
}

// NotImplemented
func (d *Discoverer) InstanceTypes(_ context.Context) ([]v1alpha1.InstanceType, error) {
	return nil, nil
}

func (d *Discoverer) DiscoveryData(ctx context.Context, _ meta.DiscoveryDataOptions) ([]byte, error) {
	var (
		discoveryData = v1.DiscoveryData{
			APIVersion: "deckhouse.io/v1",
			Kind:       "YandexCloudProviderDiscoveryData",
		}
		err error
	)

	discoveryData.Zones, err = d.getZones(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list zones: %w", err)
	}

	discoveryData.Images, err = d.getImages(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get images: %w", err)
	}

	discoveryData.DiskTypes, err = d.getDiskTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get disk types: %w", err)
	}

	discoveryData.ExternalAddresses, err = d.getAddrs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get external addresses: %w", err)
	}

	discoveryData.Networks, err = d.getNets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get networks: %w", err)
	}

	discoveryData.Subnets, err = d.getSubnets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get subnets: %w", err)
	}

	discoveryData.Platforms, err = d.getPlatforms(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get platform ids: %w", err)
	}

	data, err := json.Marshal(&discoveryData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal discovery data: %w", err)
	}

	return data, nil
}

func (d *Discoverer) getNets(ctx context.Context) ([]v1.DiscoveryDataNetwork, error) {
	netsAll, err := d.sdk.VPC().Network().NetworkIterator(ctx, &vpc.ListNetworksRequest{FolderId: d.folderID}).TakeAll()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch all vpc networks: %w", err)
	}

	nets := make([]v1.DiscoveryDataNetwork, 0, len(netsAll))
	for _, net := range netsAll {
		nets = append(nets, v1.DiscoveryDataNetwork{
			Name:      net.GetName(),
			ID:        net.GetId(),
			CreatedAt: net.GetCreatedAt().AsTime(),
		})
	}

	return nets, nil
}

func (d *Discoverer) getSubnets(ctx context.Context) ([]v1.DiscoveryDataSubnet, error) {
	subsAll, err := d.sdk.VPC().Subnet().SubnetIterator(ctx, &vpc.ListSubnetsRequest{FolderId: d.folderID}).TakeAll()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch all vpc subnets: %w", err)
	}

	subs := make([]v1.DiscoveryDataSubnet, 0, len(subsAll))
	for _, sub := range subsAll {
		subs = append(subs, v1.DiscoveryDataSubnet{
			Name:      sub.GetName(),
			ID:        sub.GetId(),
			V4cidr:    strings.Join(sub.GetV4CidrBlocks(), "."),
			NetworkId: sub.GetNetworkId(),
			Zone:      sub.GetZoneId(),
			CreatedAt: sub.GetCreatedAt().AsTime(),
		})
	}

	return subs, nil
}

func (d *Discoverer) getAddrs(ctx context.Context) ([]v1.DiscoveryDataExternalAddress, error) {
	addrsAll, err := d.sdk.VPC().Address().AddressIterator(ctx, &vpc.ListAddressesRequest{FolderId: d.folderID}).TakeAll()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch all vpc subnets: %w", err)
	}

	addrs := make([]v1.DiscoveryDataExternalAddress, 0, len(addrsAll))

	for _, a := range addrsAll {
		if !a.GetReserved() || a.GetType() != vpc.Address_EXTERNAL {
			continue
		}

		addr := a.GetExternalIpv4Address()

		ip, err := netip.ParseAddr(addr.GetAddress())
		if err != nil {
			d.logger.Warn("error while parsing vpc external ip",
				log.Err(err),
				slog.String("addr_id", a.GetId()),
			)

			continue
		}

		addrs = append(addrs, v1.DiscoveryDataExternalAddress{
			ID:   a.GetId(),
			Zone: addr.GetZoneId(),
			IP:   ip,
			Used: a.GetUsed(),
		})
	}

	return addrs, nil
}

func (d *Discoverer) getPlatforms(ctx context.Context) ([]string, error) {
	instances, err := d.sdk.Compute().Instance().
		InstanceIterator(ctx, &compute.ListInstancesRequest{FolderId: d.folderID}).
		TakeAll()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch all instance platforms: %w", err)
	}

	platforms := make(map[string]struct{})

	ids := make([]string, 0, len(instances))

	for _, inst := range instances {
		id := inst.GetPlatformId()

		if _, ok := platforms[id]; !ok {
			platforms[id] = struct{}{}

			ids = append(ids, id)
		}
	}

	return ids, nil
}

func (d *Discoverer) getZones(ctx context.Context) ([]string, error) {
	zonesAll, err := d.sdk.Compute().Zone().ZoneIterator(ctx, &compute.ListZonesRequest{}).TakeAll()
	if err != nil {
		return nil, fmt.Errorf("failed to take all zones: %w", err)
	}

	zones := make([]string, 0, len(zonesAll))

	for _, zone := range zonesAll {
		if zone.GetStatus() == compute.Zone_UP {
			zones = append(zones, zone.GetId())
		}
	}

	return zones, nil
}

func (d *Discoverer) getDiskTypes(ctx context.Context) ([]string, error) {
	diskTypesAll, err := d.sdk.Compute().DiskType().
		DiskTypeIterator(ctx, &compute.ListDiskTypesRequest{}).
		TakeAll()
	if err != nil {
		return nil, fmt.Errorf("failed to take all disk types: %w", err)
	}

	diskTypes := make([]string, 0, len(diskTypesAll))
	for _, diskType := range diskTypesAll {
		diskTypes = append(diskTypes, diskType.GetId())
	}

	return diskTypes, nil
}

func (d *Discoverer) getImages(ctx context.Context) ([]v1.DiscoveryDataImage, error) {
	standardImages, err := d.sdk.Compute().Image().
		ImageIterator(ctx, &compute.ListImagesRequest{FolderId: "standard-images"}).TakeAll()
	if err != nil {
		return nil, fmt.Errorf("failed to list standard images: %w", err)
	}

	folderImages, err := d.sdk.Compute().Image().
		ImageIterator(ctx, &compute.ListImagesRequest{FolderId: d.folderID}).TakeAll()
	if err != nil {
		return nil, fmt.Errorf("failed to list folder images: %w", err)
	}

	images := make([]v1.DiscoveryDataImage, 0, len(standardImages)+len(folderImages))

	for _, image := range standardImages {
		if !checkImageFamilySupported(image.GetFamily()) {
			continue
		}

		images = append(images, v1.DiscoveryDataImage{
			ImageID:   image.GetId(),
			Name:      image.GetName(),
			Family:    image.GetFamily(),
			CreatedAt: image.GetCreatedAt().AsTime(),
		})
	}

	for _, image := range folderImages {
		images = append(images, v1.DiscoveryDataImage{
			ImageID:   image.GetId(),
			Name:      image.GetName(),
			Family:    image.GetFamily(),
			CreatedAt: image.GetCreatedAt().AsTime(),
		})
	}

	// sort by family
	sort.Slice(images, func(i, j int) bool {
		return images[i].Family < images[j].Family
	})

	return images, nil
}

// NotImplemented
func (d *Discoverer) DisksMeta(_ context.Context) ([]v1alpha1.DiskMeta, error) {
	return nil, nil
}
