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

	log "github.com/sirupsen/logrus"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	ycsdk "github.com/yandex-cloud/go-sdk"
	"github.com/yandex-cloud/go-sdk/iamkey"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type DiskMigrator struct {
	logger      *log.Entry
	folderID    string
	sdk         *ycsdk.SDK
	clusterName string
	client      *kubernetes.Clientset
}

func NewDiskMigrator(logger *log.Entry, kubeClient *kubernetes.Clientset, folderID, saKeyJSON, clusterName string) *DiskMigrator {
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

	return &DiskMigrator{
		logger:      logger,
		folderID:    folderID,
		sdk:         sdk,
		client:      kubeClient,
		clusterName: clusterName,
	}
}

func (d *DiskMigrator) MigrateDisks(ctx context.Context) error {

	disks, err := d.getDisksCreatedByCSIDriver(ctx)
	if err != nil {
		return fmt.Errorf("failed to get disks: %v", err)
	}

	pvs, err := d.getPVFromCluster(ctx)

	pvMap := make(map[string]struct{}, len(pvs))
	for _, pv := range pvs {
		// if not yandex csi PV, skip it
		if pv.Annotations[" pv.kubernetes.io/provisioned-by"] != "yandex.csi.flant.com" {
			continue
		}
		pvMap[pv.Name] = struct{}{}
	}

	for _, disk := range disks {
		if _, ok := disk.Labels["cluster"]; ok {
			d.logger.Warnf("disk %s already has 'cluster' label, skipping", disk.Name)
			continue
		}

		if _, ok := pvMap[disk.Name]; !ok {
			d.logger.Warnf("disk %s is not present in the cluster, skipping", disk.Name)
			continue
		}

		err := d.migrateDisk(ctx, disk)
		if err != nil {
			d.logger.Errorf("failed to migrate disk: %v", err)
		}
	}

	return nil
}

func (d *DiskMigrator) getDisksCreatedByCSIDriver(ctx context.Context) ([]*compute.Disk, error) {
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

func (d *DiskMigrator) migrateDisk(ctx context.Context, disk *compute.Disk) error {
	disk.Labels["cluster"] = d.clusterName
	_, err := d.sdk.Compute().Disk().Update(ctx, &compute.UpdateDiskRequest{DiskId: disk.Id, Labels: disk.Labels})
	return err
}

func (d *DiskMigrator) getPVFromCluster(ctx context.Context) ([]v1.PersistentVolume, error) {
	pvs, err := d.client.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return pvs.Items, nil
}
