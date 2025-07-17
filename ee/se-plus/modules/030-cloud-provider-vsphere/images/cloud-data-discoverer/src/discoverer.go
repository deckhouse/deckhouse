/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	v1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
	"github.com/deckhouse/deckhouse/go_lib/dependency/vsphere"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/cns"
	"github.com/vmware/govmomi/cns/types"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"

	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
)

type Discoverer struct {
	logger               *log.Logger
	clusterUUID          string
	csiCompatibilityFlag string
	govmomiClient        *govmomi.Client
	cnsClient            *cns.Client
	vsphereClient        vsphere.Client
}

func NewDiscoverer(logger *log.Logger) *Discoverer {
	clusterUUID := os.Getenv("CLUSTER_UUID")
	if clusterUUID == "" {
		logger.Fatalf("Cannot get CLUSTER_UUID env")
	}
	csiCompatibilityFlag := os.Getenv("CSI_COMPATIBILITY_FLAG")
	if csiCompatibilityFlag == "" {
		logger.Fatalf("Cannot get CSI_COMPATIBILITY_FLAG env")
	}

	host := os.Getenv("GOVMOMI_HOST")
	if host == "" {
		logger.Fatalf("Cannot get GOVMOMI_HOST env")
	}
	username := os.Getenv("GOVMOMI_USERNAME")
	if username == "" {
		logger.Fatalf("Cannot get GOVMOMI_USERNAME env")
	}
	password := os.Getenv("GOVMOMI_PASSWORD")
	if password == "" {
		logger.Fatalf("Cannot get GOVMOMI_PASSWORD env")
	}

	insecure := os.Getenv("GOVMOMI_INSECURE")
	if insecure == "" {
		logger.Fatalf("Cannot get GOVMOMI_INSECURE env")
	}
	insecureFlag, err := strconv.ParseBool(insecure)
	if err != nil {
		logger.Fatalf("Failed to parse GOVMOMI_INSECURE env as bool: %v", err)
	}

	parsedURL, err := url.Parse(fmt.Sprintf("https://%s:%s@%s/sdk", url.PathEscape(strings.TrimSpace(username)), url.PathEscape(strings.TrimSpace(password)), url.PathEscape(strings.TrimSpace(host))))
	if err != nil {
		logger.Fatalf("Failed to build connection url: %v", err)
	}

	soapClient := soap.NewClient(parsedURL, insecureFlag)
	vimClient, err := vim25.NewClient(context.TODO(), soapClient)
	if err != nil {
		logger.Fatalf("Failed to create vimClient client: %v", err)
	}

	if !vimClient.IsVC() {
		logger.Fatalf("Created client not connected to vCenter")
	}

	// vSphere connection is timed out after 30 minutes of inactivity.
	vimClient.RoundTripper = session.KeepAlive(vimClient.RoundTripper, 10*time.Minute)
	govmomiClient := &govmomi.Client{
		Client:         vimClient,
		SessionManager: session.NewManager(vimClient),
	}

	err = govmomiClient.SessionManager.Login(context.TODO(), parsedURL.User)
	if err != nil {
		logger.Fatalf("Failed to login with provided credentials: %v", err)
	}

	cnsClient, err := cns.NewClient(context.TODO(), govmomiClient.Client)
	if err != nil {
		logger.Fatalf("Failed to create CNS client: %v", err)
	}

	region := os.Getenv("REGION")
	if region == "" {
		logger.Fatalf("Cannot get REGION env")
	}

	regionTagCategory := os.Getenv("REGION_TAG_CATEGORY")
	if regionTagCategory == "" {
		logger.Fatalf("Cannot get REGION_TAG_CATEGORY env")
	}

	zoneTagCategory := os.Getenv("ZONE_TAG_CATEGORY")
	if zoneTagCategory == "" {
		logger.Fatalf("Cannot get ZONE_TAG_CATEGORY env")
	}

	config := &vsphere.ProviderClusterConfiguration{
		Region:            region,
		RegionTagCategory: regionTagCategory,
		ZoneTagCategory:   zoneTagCategory,
		Provider: vsphere.Provider{
			Server:   host,
			Username: username,
			Password: password,
			Insecure: insecureFlag,
		},
	}

	vc, err := vsphere.NewClient(config)
	if err != nil {
		logger.Fatalf("Failed to create vSphere client: %v", err)
	}

	return &Discoverer{
		logger:               logger,
		clusterUUID:          clusterUUID,
		csiCompatibilityFlag: csiCompatibilityFlag,
		govmomiClient:        govmomiClient,
		cnsClient:            cnsClient,
		vsphereClient:        vc,
	}
}

func (d *Discoverer) CheckCloudConditions(ctx context.Context) ([]v1alpha1.CloudCondition, error) {
	return nil, nil
}

// NotImplemented
func (d *Discoverer) InstanceTypes(ctx context.Context) ([]v1alpha1.InstanceType, error) {
	return nil, nil
}

// NotImplemented
func (d *Discoverer) DiscoveryData(ctx context.Context, cloudProviderDiscoveryData []byte) ([]byte, error) {
	discoveryData := &v1.VsphereCloudProviderDiscoveryData{}
	if len(cloudProviderDiscoveryData) > 0 {
		err := json.Unmarshal(cloudProviderDiscoveryData, &discoveryData)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal cloud provider discovery data: %v", err)
		}
	}

	zonesDatastores, err := d.vsphereClient.GetZonesDatastores()
	if err != nil {
		return nil, fmt.Errorf("error on GetZonesDatastores: %v", err)
	}

	discoveryData.StorageProfiles = mergeZonedDataStores(
		discoveryData.StorageProfiles,
		zonesDatastores.ZonedDataStores,
	)

	discoveryDataJSON, err := json.Marshal(discoveryData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal discovery data: %w", err)
	}

	d.logger.Debugf("discovery data: %v", discoveryDataJSON)
	return discoveryDataJSON, nil
}

func (d *Discoverer) DisksMeta(ctx context.Context) ([]v1alpha1.DiskMeta, error) {
	if d.csiCompatibilityFlag != "none" {
		d.logger.Warn("Skipping orphaned disks discovery: \"legacy\" CSI driver in-use")
		return []v1alpha1.DiskMeta{}, nil
	}

	disks, err := d.getDisksCreatedByCSIDriver(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get disks: %v", err)
	}

	disksMeta := make([]v1alpha1.DiskMeta, 0, len(disks))

	for _, disk := range disks {
		disksMeta = append(disksMeta, v1alpha1.DiskMeta{ID: disk.VolumeId.Id, Name: disk.Name})
	}

	return disksMeta, nil
}

func (d *Discoverer) getDisksCreatedByCSIDriver(ctx context.Context) ([]types.CnsVolume, error) {
	diskList, err := d.cnsClient.QueryVolume(ctx, types.CnsQueryFilter{ContainerClusterIds: []string{d.clusterUUID}})
	if err != nil {
		return nil, fmt.Errorf("failed to list disks: %v", err)
	}

	return diskList.Volumes, nil
}

func mergeZonedDataStores(storageProfiles []v1.ZonedDataStore, discoveredZonedDataStores []vsphere.ZonedDataStore) []v1.ZonedDataStore {
	result := make([]v1.ZonedDataStore, 0, len(storageProfiles))

	discoveredZonedDataStoresMap := make(map[string]v1.ZonedDataStore)
	for i := range discoveredZonedDataStores {
		discoveredZonedDataStoresMap[discoveredZonedDataStores[i].Name] = v1.ZonedDataStore{
			Zones:         discoveredZonedDataStores[i].Zones,
			InventoryPath: discoveredZonedDataStores[i].InventoryPath,
			Name:          discoveredZonedDataStores[i].Name,
			DatastoreType: discoveredZonedDataStores[i].DatastoreType,
			DatastoreURL:  discoveredZonedDataStores[i].DatastoreURL,
		}
		result = append(result, discoveredZonedDataStoresMap[discoveredZonedDataStores[i].Name])
	}

	for i := range storageProfiles {
		if _, ok := discoveredZonedDataStoresMap[storageProfiles[i].Name]; !ok {
			result = append(result, storageProfiles[i])
		}
	}

	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}
