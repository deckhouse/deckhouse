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

package main

import (
	"context"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	ycsdk "github.com/yandex-cloud/go-sdk"
	"github.com/yandex-cloud/go-sdk/iamkey"

	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
)

type Discoverer struct {
	logger   *log.Entry
	folderID string
	sdk      *ycsdk.SDK
}

func NewDiscoverer(logger *log.Entry) *Discoverer {
	folderID := os.Getenv("YC_FOLDER_ID")
	if folderID == "" {
		logger.Fatal("Cannot get YC_FOLDER_ID env")
	}

	saKeyJSON := os.Getenv("YC_SA_KEY_JSON")
	if saKeyJSON == "" {
		logger.Fatal("Cannot get YC_SA_KEY_JSON env")
	}

	saKeyJSONBytes := []byte(saKeyJSON)
	key, err := iamkey.ReadFromJSONBytes(saKeyJSONBytes)
	if err != nil {
		logger.Fatalf("Failed to parse YC_SA_KEY_JSON: %v", err)
	}

	creds, err := ycsdk.ServiceAccountKey(key)
	if err != nil {
		logger.Fatalf("Failed to create credentials for the given IAM Key: %v", err)
	}

	sdk, err := ycsdk.Build(context.Background(), ycsdk.Config{
		Credentials: creds,
	})
	if err != nil {
		log.Fatalf("Failed to build YC SDK: %v", err)
	}

	return &Discoverer{
		logger:   logger,
		folderID: folderID,
		sdk:      sdk,
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
		disksMeta = append(disksMeta, v1alpha1.DiskMeta{ID: disk.Id, Name: disk.Name})
	}

	return disksMeta, nil
}

func (d *Discoverer) getDisksCreatedByCSIDriver(ctx context.Context) ([]*compute.Disk, error) {
	diskList, err := d.sdk.Compute().Disk().List(ctx, &compute.ListDisksRequest{
		FolderId: d.folderID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list disks: %v", err)
	}

	disks := diskList.GetDisks()
	disksCreatedByCSIDriver := make([]*compute.Disk, 0, len(disks))

	for _, disk := range disks {
		if disk.Description == "Created by Yandex CSI driver" {
			disksCreatedByCSIDriver = append(disksCreatedByCSIDriver, disk)
		}
	}

	if len(disksCreatedByCSIDriver) == 0 {
		d.logger.Warnln("Unexpected behavior: no disks created by the CSI driver were found in the cloud. Should be checked manually")
	}

	return disksCreatedByCSIDriver, nil
}
