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
	"slices"
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

const (
	DiscoveryDataKind    = "VsphereCloudDiscoveryData"
	DiscoveryDataVersion = "deckhouse.io/v1"
)

type Discoverer struct {
	logger               *log.Logger
	clusterUUID          string
	csiCompatibilityFlag string
	govmomiClient        *govmomi.Client
	cnsClient            *cns.Client
	vsphereClient        vsphere.Client
	vmFolderPath         string
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

	vmFolderPath := os.Getenv("VM_FOLDER_PATH")
	if vmFolderPath == "" {
		logger.Fatal("Cannot get VM_FOLDER_PATH env")
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
		vmFolderPath:         vmFolderPath,
	}
}

func (d *Discoverer) CheckCloudConditions(ctx context.Context) ([]v1alpha1.CloudCondition, error) {
	return nil, nil
}

// NotImplemented
func (d *Discoverer) InstanceTypes(ctx context.Context) ([]v1alpha1.InstanceType, error) {
	return nil, nil
}

func (d *Discoverer) DiscoveryData(ctx context.Context, cloudProviderDiscoveryData []byte) ([]byte, error) {
	discoveryData := new(v1.VsphereCloudDiscoveryData)
	if len(cloudProviderDiscoveryData) > 0 {
		err := json.Unmarshal(cloudProviderDiscoveryData, &discoveryData)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal cloud provider discovery data: %v", err)
		}
	}

	err := d.vsphereClient.RefreshClient()
	if err != nil {
		return nil, fmt.Errorf("failed to login to vSphere: %v", err)
	}

	zonesDatastores, err := d.vsphereClient.GetZonesDatastores()
	if err != nil {
		return nil, fmt.Errorf("error on GetZonesDatastores: %v", err)
	}

	storagePolicies, err := d.vsphereClient.ListPolicies()
	if err != nil {
		return nil, fmt.Errorf("failed to list Storage Policies: %v", err)
	}

	discoveryData.Kind = DiscoveryDataKind
	discoveryData.APIVersion = DiscoveryDataVersion
	discoveryData.Datacenter = zonesDatastores.Datacenter
	discoveryData.Zones = mergeZones(discoveryData.Zones, zonesDatastores.Zones)
	discoveryData.Datastores = mergeDatastores(discoveryData.Datastores, zonesDatastores.ZonedDataStores)
	discoveryData.VMFolderPath = d.vmFolderPath

	for i := range storagePolicies {
		discoveryData.StoragePolicies = append(discoveryData.StoragePolicies, v1.VsphereStoragePolicy{
			Name: storagePolicies[i].Name,
			ID:   storagePolicies[i].ID,
		})
	}

	discoveryDataJSON, err := json.Marshal(discoveryData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal discovery data: %w", err)
	}

	d.logger.Debug("discovery data:", "discoveryDataJSON", discoveryDataJSON)
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

func mergeZones(discoveredZones, newZones []string) []string {
	zones := append(discoveredZones, newZones...)
	resMap := make(map[string]struct{}, len(zones))
	res := make([]string, 0, len(zones))

	for i := range zones {
		if _, found := resMap[zones[i]]; !found {
			resMap[zones[i]] = struct{}{}
			res = append(res, zones[i])
		}
	}

	slices.Sort(res)
	return res
}

func mergeDatastores(discoveredZonedDataStores []v1.VsphereDatastore, newZonedDataStores []vsphere.ZonedDataStore) []v1.VsphereDatastore {
	zonedDataStores := append(discoveredZonedDataStores, vsphereZonedDataStoresToV1(newZonedDataStores)...)
	res := make([]v1.VsphereDatastore, 0, len(zonedDataStores))
	resMap := make(map[string]struct{}, len(zonedDataStores))

	for i := range zonedDataStores {
		if _, found := resMap[zonedDataStores[i].Name]; !found {
			resMap[zonedDataStores[i].Name] = struct{}{}
			res = append(res, zonedDataStores[i])
		}
	}

	sort.SliceStable(res, func(i, j int) bool {
		return res[i].Name < res[j].Name
	})
	return res
}

func vsphereZonedDataStoresToV1(in []vsphere.ZonedDataStore) []v1.VsphereDatastore {
	result := make([]v1.VsphereDatastore, 0, len(in))
	for i := range in {
		result = append(result, v1.VsphereDatastore{
			Zones:         in[i].Zones,
			InventoryPath: in[i].InventoryPath,
			Name:          in[i].Name,
			DatastoreType: in[i].DatastoreType,
			DatastoreURL:  in[i].DatastoreURL,
		})
	}
	return result
}
