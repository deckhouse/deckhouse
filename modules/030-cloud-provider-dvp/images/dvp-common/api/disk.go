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

package api

import (
	"context"
	"fmt"

	"github.com/deckhouse/virtualization/api/core/v1alpha2"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	diskNameLabel = "virtualDiskName"
)

type DiskService struct {
	*Service
}

func NewDiskService(service *Service) *DiskService {
	return &DiskService{service}
}

func (d *DiskService) ListDisksByName(ctx context.Context, diskName string) (*v1alpha2.VirtualDiskList, error) {
	var virtualDiskList v1alpha2.VirtualDiskList

	selector, err := labels.Parse(fmt.Sprintf("%s=%s", diskNameLabel, diskName))
	if err != nil {
		return nil, err
	}

	opts := &client.ListOptions{
		Namespace:     d.namespace,
		LabelSelector: selector,
	}

	if err := d.client.List(ctx, &virtualDiskList, opts); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, cloudprovider.DiskNotFound
		}
		return nil, err
	}

	return &virtualDiskList, nil
}

func (d *DiskService) CreateDisk(ctx context.Context, diskName string, diskSize int64, diskStorageClass string) (*v1alpha2.VirtualDisk, error) {
	vmd := v1alpha2.VirtualDisk{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha2.VirtualDiskKind,
			APIVersion: v1alpha2.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      diskName,
			Namespace: d.namespace,
			Labels: map[string]string{
				diskNameLabel: diskName,
			},
		},
		Spec: v1alpha2.VirtualDiskSpec{
			PersistentVolumeClaim: v1alpha2.VirtualDiskPersistentVolumeClaim{
				StorageClass: &diskStorageClass,
				Size:         resource.NewQuantity(diskSize, resource.BinarySI),
			},
		},
	}

	err := d.client.Create(ctx, &vmd)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return nil, err
	}

	newDisk, err := d.GetDiskByName(ctx, diskName)
	if err != nil {
		return nil, err
	}

	return newDisk, nil
}

func (d *DiskService) CreateDiskFromDataSource(
	ctx context.Context,
	diskName string,
	diskSize resource.Quantity,
	diskStorageClass string,
	imageDataSource *v1alpha2.VirtualDiskDataSource,
) (*v1alpha2.VirtualDisk, error) {
	vmd := v1alpha2.VirtualDisk{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha2.VirtualDiskKind,
			APIVersion: v1alpha2.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      diskName,
			Namespace: d.namespace,
			Labels: map[string]string{
				diskNameLabel: diskName,
			},
		},
		Spec: v1alpha2.VirtualDiskSpec{
			DataSource: imageDataSource,
			PersistentVolumeClaim: v1alpha2.VirtualDiskPersistentVolumeClaim{
				StorageClass: &diskStorageClass,
				Size:         &diskSize,
			},
		},
	}

	err := d.client.Create(ctx, &vmd)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return nil, err
	}

	newDisk, err := d.GetDiskByName(ctx, diskName)
	if err != nil {
		return nil, err
	}

	return newDisk, nil
}

func (d *DiskService) GetDiskByName(ctx context.Context, diskName string) (*v1alpha2.VirtualDisk, error) {
	disks, err := d.ListDisksByName(ctx, diskName)
	if err != nil {
		return nil, err
	}

	if len(disks.Items) > 1 {
		return nil, fmt.Errorf("found more then one disk with the name %s, please contanct the DVP admin to check the name duplication", diskName)
	}
	if len(disks.Items) == 0 {
		return nil, cloudprovider.DiskNotFound
	}

	return &disks.Items[0], nil
}

func (d *DiskService) RemoveDiskByName(ctx context.Context, diskName string) error {
	disk, err := d.GetDiskByName(ctx, diskName)
	if err != nil {
		return err
	}

	err = d.client.Delete(ctx, disk)
	if err != nil {
		return err
	}

	err = d.WaitDiskDeletion(ctx, diskName)
	if err != nil {
		return err
	}

	return nil
}

func (d *DiskService) ResizeDisk(ctx context.Context, diskName string, newSize string) error {
	var vmd v1alpha2.VirtualDisk

	err := d.client.Get(ctx, types.NamespacedName{
		Namespace: d.namespace,
		Name:      diskName,
	}, &vmd)
	if err != nil {
		return err
	}

	newSigeQuantity, err := resource.ParseQuantity(newSize)
	if err != nil {
		return err
	}
	vmd.Spec.PersistentVolumeClaim.Size = &newSigeQuantity

	err = d.client.Update(ctx, &vmd)
	if err != nil {
		return err
	}
	return nil
}

func (d *DiskService) GetStorageClassList(ctx context.Context) (*storagev1.StorageClassList, error) {
	storageClassList, err := d.clientset.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, cloudprovider.DiskNotFound
		}
		return nil, err
	}
	return storageClassList, nil
}

func (d *DiskService) WaitDiskCreation(ctx context.Context, vmdName string) error {
	return d.Wait(ctx, vmdName, &v1alpha2.VirtualDisk{}, func(obj client.Object) (bool, error) {
		vmd, ok := obj.(*v1alpha2.VirtualDisk)
		if !ok {
			return false, fmt.Errorf("expected a VirtualMachineDisk but got a %T", obj)
		}

		return vmd.Status.Phase == v1alpha2.DiskReady, nil
	})
}

func (c *DiskService) WaitDiskDeletion(ctx context.Context, vmdName string) error {
	return c.Wait(ctx, vmdName, &v1alpha2.VirtualDisk{}, func(obj client.Object) (bool, error) {
		return obj == nil, nil
	})
}
