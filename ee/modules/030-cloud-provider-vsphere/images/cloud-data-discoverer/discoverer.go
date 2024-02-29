/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/cns"
	"github.com/vmware/govmomi/cns/types"

	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
)

type Discoverer struct {
	logger        *log.Entry
	clusterUUID   string
	govmomiClient *govmomi.Client
	cnsClient     *cns.Client
}

func NewDiscoverer(logger *log.Entry) *Discoverer {
	clusterUUID := os.Getenv("CLUSTER_UUID")
	if clusterUUID == "" {
		logger.Fatalf("Cannot get CLUSTER_UUID env")
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

	govmomiClient, err := govmomi.NewClient(context.TODO(), parsedURL, insecureFlag)
	if err != nil {
		logger.Fatalf("Failed to create govmomi client: %v", err)
	}

	if !govmomiClient.IsVC() {
		logger.Fatalf("Created client not connected to vCenter")
	}

	cnsClient, err := cns.NewClient(context.TODO(), govmomiClient.Client)
	if err != nil {
		fmt.Printf("failed to create CNS client: %v", err)
	}

	return &Discoverer{
		logger:        logger,
		clusterUUID:   clusterUUID,
		govmomiClient: govmomiClient,
		cnsClient:     cnsClient,
	}
}

// NotImplemented
func (d *Discoverer) InstanceTypes(ctx context.Context) ([]v1alpha1.InstanceType, error) {
	return nil, nil
}

// NotImplemented
func (d *Discoverer) DiscoveryData(ctx context.Context, cloudProviderDiscoveryData []byte) ([]byte, error) {
	return nil, nil
}

func (d *Discoverer) DisksMeta(ctx context.Context) ([]v1alpha1.DiskMeta, error) {
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
